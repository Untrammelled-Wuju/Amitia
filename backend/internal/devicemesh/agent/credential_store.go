package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/secretstore"
)

type StoredCredential struct {
	CloudBaseUrl     string                    `json:"cloudBaseUrl"`
	CredentialID     string                    `json:"credentialId"`
	Credential       string                    `json:"credential"`
	SpaceID          runtimeidentity.SpaceID   `json:"spaceId"`
	DeviceID         runtimeidentity.DeviceID  `json:"deviceId"`
	RuntimeID        runtimeidentity.RuntimeID `json:"runtimeId"`
	ExpiresAt        time.Time                 `json:"expiresAt"`
	Protocol         string                    `json:"protocol"`
	Fingerprint      string                    `json:"fingerprint,omitempty"`
	ProviderPath     []string                  `json:"providerPath,omitempty"`
	PreviousCoreID   string                    `json:"previousCoreId,omitempty"`
	ProviderChangeID string                    `json:"providerChangeId,omitempty"`
}

type SessionCursor struct {
	RuntimeSessionID         runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`
	ConnectionGeneration     int64                            `json:"connectionGeneration"`
	LastAppliedStateRevision int64                            `json:"lastAppliedStateRevision"`
	LastProcessedCommandSeq  int64                            `json:"lastProcessedCommandSequence"`
	LastEventSequence        int64                            `json:"lastEventSequence"`
	ActualStateHash          string                           `json:"actualStateHash"`
}

type CredentialStore struct {
	mu       *sync.Mutex
	dirPath  string
	credFile string
	sessFile string
}

var credentialLocks sync.Map

func NewCredentialStore(dataDir string) *CredentialStore {
	dir := filepath.Join(dataDir, "device-mesh")
	key, err := filepath.Abs(dir)
	if err != nil {
		key = filepath.Clean(dir)
	}
	if runtime.GOOS == "windows" {
		key = strings.ToLower(key)
	}
	mutex, _ := credentialLocks.LoadOrStore(key, &sync.Mutex{})
	return &CredentialStore{
		mu:       mutex.(*sync.Mutex),
		dirPath:  dir,
		credFile: filepath.Join(dir, "credential.json"),
		sessFile: filepath.Join(dir, "session-state.json"),
	}
}

func (s *CredentialStore) LoadCredential() (*StoredCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadCredentialLocked()
}

func (s *CredentialStore) loadCredentialLocked() (*StoredCredential, error) {

	data, err := secretstore.Read(s.credFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cred StoredCredential
	if err := json.Unmarshal(data, &cred); err != nil {
		return nil, err
	}

	return &cred, nil
}

func (s *CredentialStore) WithActiveCredential(ctx context.Context, expected *StoredCredential, write func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.withActiveCredentialLocked(ctx, expected, write)
}

func (s *CredentialStore) WithActiveSessionCredential(ctx context.Context, expected *StoredCredential, session runtimeidentity.RuntimeSessionID, generation int64, write func(context.Context) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cursor, err := s.loadCursorLocked()
	if err != nil {
		return err
	}
	if cursor == nil || session == "" || generation < 1 || cursor.RuntimeSessionID != session || cursor.ConnectionGeneration != generation {
		return errors.Join(errors.New("设备执行连接已变化，旧审批已拦截"), coordination.ErrScopeExpired)
	}
	return s.withActiveCredentialLocked(ctx, expected, write)
}

func (s *CredentialStore) withActiveCredentialLocked(ctx context.Context, expected *StoredCredential, write func(context.Context) error) error {
	credential, err := s.loadCredentialLocked()
	if err != nil {
		return err
	}
	if credential == nil || expected == nil || credential.CredentialID != expected.CredentialID || credential.Credential != expected.Credential || credential.SpaceID != expected.SpaceID || credential.DeviceID != expected.DeviceID || credential.RuntimeID != expected.RuntimeID || !credential.ExpiresAt.After(time.Now()) {
		return errors.New("设备绑定已失效，旧 Core 写入已拦截")
	}
	for _, name := range []string{"unpair-intent.json", "provider-transition.json", "candidate-credential.json"} {
		_, err := secretstore.Read(filepath.Join(s.dirPath, name))
		if !os.IsNotExist(err) {
			if err != nil {
				return err
			}
			return errors.New("设备正在切换服务，旧 Core 写入已拦截")
		}
	}
	ctx, cancel := context.WithDeadline(ctx, credential.ExpiresAt)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	return write(ctx)
}

func (s *CredentialStore) SaveCredential(cred *StoredCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dirPath, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return err
	}

	return secretstore.Write(s.credFile, data)
}

func (s *CredentialStore) LoadCandidate() (*StoredCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := secretstore.Read(filepath.Join(s.dirPath, "candidate-credential.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var credential StoredCredential
	if err := json.Unmarshal(data, &credential); err != nil {
		return nil, err
	}
	return &credential, nil
}

func (s *CredentialStore) SaveCandidate(credential *StoredCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dirPath, 0700); err != nil {
		return err
	}
	return secretstore.Write(filepath.Join(s.dirPath, "candidate-credential.json"), data)
}

func (s *CredentialStore) DeleteCandidate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := secretstore.Delete(filepath.Join(s.dirPath, "candidate-credential.json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *CredentialStore) DeleteCredential() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := secretstore.Delete(s.credFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *CredentialStore) LoadCursor() (*SessionCursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadCursorLocked()
}

func (s *CredentialStore) loadCursorLocked() (*SessionCursor, error) {

	data, err := os.ReadFile(s.sessFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cursor SessionCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, err
	}

	return &cursor, nil
}

func (s *CredentialStore) SaveCursor(cursor *SessionCursor) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dirPath, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cursor, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.sessFile + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}

	return os.Rename(tmp, s.sessFile)
}

func (s *CredentialStore) DeleteCursor() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.sessFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

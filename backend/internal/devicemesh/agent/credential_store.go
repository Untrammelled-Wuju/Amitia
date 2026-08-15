package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type StoredCredential struct {
	CloudBaseUrl   string                  `json:"cloudBaseUrl"`
	CredentialID   string                  `json:"credentialId"`
	Credential     string                  `json:"credential"`
	UserID         runtimeidentity.UserID  `json:"userId"`
	DeviceID       runtimeidentity.DeviceID `json:"deviceId"`
	RuntimeID      runtimeidentity.RuntimeID `json:"runtimeId"`
	ExpiresAt      time.Time               `json:"expiresAt"`
	Protocol       string                  `json:"protocol"`
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
	mu       sync.Mutex
	dirPath  string
	credFile string
	sessFile string
}

func NewCredentialStore(dataDir string) *CredentialStore {
	dir := filepath.Join(dataDir, "device-mesh")
	return &CredentialStore{
		dirPath:  dir,
		credFile: filepath.Join(dir, "credential.json"),
		sessFile: filepath.Join(dir, "session-state.json"),
	}
}

func (s *CredentialStore) LoadCredential() (*StoredCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.credFile)
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

	tmp := s.credFile + ".tmp"
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

	return os.Rename(tmp, s.credFile)
}

func (s *CredentialStore) DeleteCredential() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.credFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *CredentialStore) LoadCursor() (*SessionCursor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

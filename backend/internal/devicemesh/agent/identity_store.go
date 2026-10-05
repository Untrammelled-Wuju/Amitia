package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/secretstore"
)

type LocalIdentity struct {
	DeviceID  runtimeidentity.DeviceID  `json:"deviceId"`
	RuntimeID runtimeidentity.RuntimeID `json:"runtimeId"`
	CreatedAt time.Time                 `json:"createdAt"`
	PublicKey string                    `json:"publicKey,omitempty"`
}

var identityLocks sync.Map

type IdentityStore struct {
	mu       sync.Mutex
	filePath string
	cached   *LocalIdentity
	private  ed25519.PrivateKey
}

func NewIdentityStore(dataDir string) *IdentityStore {
	return &IdentityStore{
		filePath: filepath.Join(dataDir, "device-mesh", "identity.json"),
	}
}

func (s *IdentityStore) Load() (*LocalIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cached != nil {
		return s.cached, nil
	}
	lockValue, _ := identityLocks.LoadOrStore(filepath.Clean(s.filePath), &sync.Mutex{})
	pathLock := lockValue.(*sync.Mutex)
	pathLock.Lock()
	defer pathLock.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return s.createDefault()
		}
		return nil, err
	}

	var id LocalIdentity
	if err := json.Unmarshal(data, &id); err != nil {
		return nil, err
	}

	if id.DeviceID == "" || id.RuntimeID == "" {
		return s.createDefault()
	}
	if err := s.ensureKey(&id); err != nil {
		return nil, err
	}

	s.cached = &id
	return s.cached, nil
}

func (s *IdentityStore) createDefault() (*LocalIdentity, error) {
	id := LocalIdentity{
		DeviceID:  runtimeidentity.DeviceID("dev_" + uuid.New().String()),
		RuntimeID: runtimeidentity.RuntimeID("rt_" + uuid.New().String()),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.ensureKey(&id); err != nil {
		return nil, err
	}

	if err := s.save(&id); err != nil {
		return nil, err
	}

	s.cached = &id
	return s.cached, nil
}

func (s *IdentityStore) ensureKey(identity *LocalIdentity) error {
	path := filepath.Join(filepath.Dir(s.filePath), "identity-key")
	data, err := secretstore.Read(path)
	if errors.Is(err, os.ErrNotExist) {
		if identity.PublicKey != "" {
			return errors.New("设备私钥已丢失，请重新生成身份并配对")
		}
		_, private, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		if err := secretstore.Write(path, private); err != nil {
			return err
		}
		data = private
	} else if err != nil {
		return err
	}
	if len(data) != ed25519.PrivateKeySize {
		return errors.New("设备私钥无效")
	}
	private := ed25519.PrivateKey(data)
	public := base64.RawURLEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	if identity.PublicKey != "" && identity.PublicKey != public {
		return errors.New("设备身份和私钥不匹配，拒绝使用复制的身份")
	}
	if identity.PublicKey == "" {
		identity.PublicKey = public
		if err := s.save(identity); err != nil {
			return err
		}
	}
	s.private = private
	return nil
}

func (s *IdentityStore) Sign(data []byte) (string, error) {
	if _, err := s.Load(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(s.private, data)), nil
}

func (s *IdentityStore) save(id *LocalIdentity) error {
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return err
	}

	tmp := s.filePath + ".tmp"
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

	return os.Rename(tmp, s.filePath)
}

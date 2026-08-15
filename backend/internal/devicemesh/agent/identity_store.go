package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type LocalIdentity struct {
	DeviceID  runtimeidentity.DeviceID  `json:"deviceId"`
	RuntimeID runtimeidentity.RuntimeID `json:"runtimeId"`
	CreatedAt time.Time                 `json:"createdAt"`
}

type IdentityStore struct {
	mu       sync.Mutex
	filePath string
	cached   *LocalIdentity
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

	s.cached = &id
	return s.cached, nil
}

func (s *IdentityStore) createDefault() (*LocalIdentity, error) {
	id := LocalIdentity{
		DeviceID:  runtimeidentity.DeviceID("dev_" + uuid.New().String()),
		RuntimeID: runtimeidentity.RuntimeID("rt_" + uuid.New().String()),
		CreatedAt: time.Now().UTC(),
	}

	if err := s.save(&id); err != nil {
		return nil, err
	}

	s.cached = &id
	return s.cached, nil
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

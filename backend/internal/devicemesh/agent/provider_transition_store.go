package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/secretstore"
)

type PendingProviderTransition struct {
	PreviousCoreID       string    `json:"previousCoreId"`
	PreviousCredentialID string    `json:"previousCredentialId"`
	DeviceID             string    `json:"deviceId"`
	RuntimeID            string    `json:"runtimeId"`
	Successor            Successor `json:"successor"`
}

func (s *CredentialStore) LoadTransition() (*PendingProviderTransition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	encoded, err := secretstore.Read(filepath.Join(s.dirPath, "provider-transition.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pending PendingProviderTransition
	if len(encoded) > 64<<10 || json.Unmarshal(encoded, &pending) != nil || pending.PreviousCoreID == "" || pending.DeviceID == "" || pending.RuntimeID == "" || pending.Successor.Endpoint.CoreID == "" || pending.Successor.Endpoint.CoreID == pending.PreviousCoreID || pending.Successor.DeviceID != pending.DeviceID {
		return nil, errors.New("待完成的服务切换记录无效，请撤销配对后重新连接")
	}
	return &pending, nil
}

func (s *CredentialStore) SaveTransition(pending PendingProviderTransition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	encoded, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	if len(encoded) > 64<<10 {
		return errors.New("服务切换记录超过上限")
	}
	if err := os.MkdirAll(s.dirPath, 0700); err != nil {
		return err
	}
	return secretstore.Write(filepath.Join(s.dirPath, "provider-transition.json"), encoded)
}

func (s *CredentialStore) DeleteTransition() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(filepath.Join(s.dirPath, "provider-transition.json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/secretstore"
)

func (s *CredentialStore) unpairIntent(begin bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dirPath, "unpair-intent.json")
	if !begin {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(s.dirPath, 0700); err != nil {
		return err
	}
	return secretstore.Write(path, []byte(`{"resetLocalAuthority":true}`))
}

func (s *CredentialStore) pendingUnpair() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := secretstore.Read(filepath.Join(s.dirPath, "unpair-intent.json"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var intent struct {
		Reset bool `json:"resetLocalAuthority"`
	}
	if json.Unmarshal(raw, &intent) != nil || !intent.Reset {
		return false, errors.New("解绑状态损坏，禁止恢复本地服务")
	}
	return true, nil
}

func (h *LocalHandler) finishUnpair() error {
	h.pauseProvider()
	h.bindingVersion++
	h.pendingCore = ""
	if h.mesh != nil {
		h.mesh.Stop()
		h.mesh = nil
	}
	for _, remove := range []func() error{h.credStore.DeleteCredential, h.credStore.DeleteCandidate, h.credStore.DeleteTransition, h.credStore.DeleteCursor} {
		if err := remove(); err != nil {
			return err
		}
	}
	if h.credentialObserver != nil {
		if err := h.credentialObserver(nil); err != nil {
			return err
		}
	}
	if err := h.credStore.unpairIntent(false); err != nil {
		return err
	}
	h.resumeProvider()
	return nil
}

func (h *LocalHandler) RecoverUnpair() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	pending, err := h.credStore.pendingUnpair()
	if err != nil {
		h.pauseProvider()
		return err
	}
	if pending {
		return h.finishUnpair()
	}
	return nil
}

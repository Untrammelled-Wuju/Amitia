package startup

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type durableHostIdentityRecord struct {
	InstanceID string `json:"instance_id"`
	SessionID  string `json:"session_id"`
	CreatedAt  string `json:"created_at"`
}

type DurableHostIdentity struct {
	mu         sync.RWMutex
	instanceID string
	sessionID  string
	dataPath   string
}

func NewDurableHostIdentity(dataDir string) (*DurableHostIdentity, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data dir is required for durable host identity")
	}

	identity := &DurableHostIdentity{
		dataPath: filepath.Join(dataDir, "host_identity.json"),
	}

	if err := identity.load(); err != nil {
		return nil, err
	}

	if identity.instanceID == "" || identity.sessionID == "" {
		identity.generateNew()
		if err := identity.save(); err != nil {
			return nil, err
		}
	}

	return identity, nil
}

func (h *DurableHostIdentity) load() error {
	data, err := os.ReadFile(h.dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read host identity: %w", err)
	}

	var record durableHostIdentityRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return fmt.Errorf("unmarshal host identity: %w", err)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.instanceID = record.InstanceID
	h.sessionID = record.SessionID
	return nil
}

func (h *DurableHostIdentity) save() error {
	h.mu.RLock()
	record := durableHostIdentityRecord{
		InstanceID: h.instanceID,
		SessionID:  h.sessionID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	h.mu.RUnlock()

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal host identity: %w", err)
	}

	tmp := h.dataPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write host identity tmp: %w", err)
	}
	return os.Rename(tmp, h.dataPath)
}

func (h *DurableHostIdentity) generateNew() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.instanceID, _ = os.Hostname()
	if h.instanceID == "" {
		h.instanceID = randomID("host")
	}
	h.sessionID = randomID("session")
}

func (h *DurableHostIdentity) GetHostInstanceID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.instanceID
}

func (h *DurableHostIdentity) GetHostSessionID() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessionID
}

func randomID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}

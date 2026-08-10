package upgrade

import (
	"fmt"
	"sync"
)

type upgradeGateEntry struct {
	operationID UpgradeOperationID
	extensionID string
}

type UpgradeGate struct {
	mu      sync.RWMutex
	entries map[string]upgradeGateEntry
}

func NewUpgradeGate() *UpgradeGate {
	return &UpgradeGate{
		entries: make(map[string]upgradeGateEntry),
	}
}

func (g *UpgradeGate) Acquire(extensionID string, operationID UpgradeOperationID) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if existing, exists := g.entries[extensionID]; exists {
		return fmt.Errorf("extension %s already upgrading: operation=%s", extensionID, existing.operationID)
	}

	g.entries[extensionID] = upgradeGateEntry{
		operationID: operationID,
		extensionID: extensionID,
	}
	return nil
}

func (g *UpgradeGate) Release(extensionID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, extensionID)
}

func (g *UpgradeGate) IsUpgrading(extensionID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.entries[extensionID]
	return exists
}

func (g *UpgradeGate) GetOperationID(extensionID string) (UpgradeOperationID, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	entry, exists := g.entries[extensionID]
	if !exists {
		return "", false
	}
	return entry.operationID, true
}

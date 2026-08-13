package upgrade

import (
	"sync"
)

type upgradeGateEntry struct {
	operationID UpgradeOperationID
	extensionID string
}

type UpgradeGate struct {
	mu      sync.RWMutex
	entries map[UpgradeOperationID]upgradeGateEntry
}

func NewUpgradeGate() *UpgradeGate {
	return &UpgradeGate{
		entries: make(map[UpgradeOperationID]upgradeGateEntry),
	}
}

func (g *UpgradeGate) Acquire(extensionID string, operationID UpgradeOperationID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.entries[operationID] = upgradeGateEntry{
		operationID: operationID,
		extensionID: extensionID,
	}
	return nil
}

func (g *UpgradeGate) Release(extensionID string, operationID UpgradeOperationID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, operationID)
}

func (g *UpgradeGate) IsUpgrading(extensionID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, entry := range g.entries {
		if entry.extensionID == extensionID {
			return true
		}
	}
	return false
}

func (g *UpgradeGate) GetOperationID(extensionID string) (UpgradeOperationID, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, entry := range g.entries {
		if entry.extensionID == extensionID {
			return entry.operationID, true
		}
	}
	return "", false
}

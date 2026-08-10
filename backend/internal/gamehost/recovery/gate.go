package recovery

import (
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RecoveryGate struct {
	mu      sync.RWMutex
	entries map[domain.RuntimeInstanceID]recoveryGateEntry
}

type recoveryGateEntry struct {
	operationID RecoveryOperationID
}

func NewRecoveryGate() *RecoveryGate {
	return &RecoveryGate{
		entries: make(map[domain.RuntimeInstanceID]recoveryGateEntry),
	}
}

func (g *RecoveryGate) Acquire(runtimeID domain.RuntimeInstanceID, operationID RecoveryOperationID) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.entries[runtimeID]; exists {
		return NewRuntimeAlreadyRecoveringError(runtimeID)
	}
	g.entries[runtimeID] = recoveryGateEntry{operationID: operationID}
	return nil
}

func (g *RecoveryGate) Release(runtimeID domain.RuntimeInstanceID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, runtimeID)
}

func (g *RecoveryGate) IsRecovering(runtimeID domain.RuntimeInstanceID) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, exists := g.entries[runtimeID]
	return exists
}

func (g *RecoveryGate) GetOperationID(runtimeID domain.RuntimeInstanceID) (RecoveryOperationID, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	entry, exists := g.entries[runtimeID]
	if !exists {
		return "", false
	}
	return entry.operationID, true
}

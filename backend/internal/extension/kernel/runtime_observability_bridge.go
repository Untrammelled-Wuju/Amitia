package kernel

import (
	"context"
	"sync"

	"github.com/u-ai/backend/internal/runtimeobservability"
	"gorm.io/gorm"
)

type RuntimeObservabilityBridge struct {
	mu             sync.RWMutex
	db             *gorm.DB
	reconciliation runtimeobservability.ReconciliationSource
}

func NewRuntimeObservabilityBridge(db *gorm.DB) *RuntimeObservabilityBridge {
	return &RuntimeObservabilityBridge{db: db}
}

func (b *RuntimeObservabilityBridge) SetDB(db *gorm.DB) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.db = db
	b.mu.Unlock()
}

func (b *RuntimeObservabilityBridge) SetReconciliation(source runtimeobservability.ReconciliationSource) {
	if b == nil {
		return
	}
	b.mu.Lock()
	b.reconciliation = source
	b.mu.Unlock()
}

func (b *RuntimeObservabilityBridge) Snapshot(ctx context.Context) map[string]any {
	if b == nil {
		return runtimeobservability.Snapshot(ctx, nil, nil)
	}
	b.mu.RLock()
	db := b.db
	reconciliation := b.reconciliation
	b.mu.RUnlock()
	return runtimeobservability.Snapshot(ctx, db, reconciliation)
}

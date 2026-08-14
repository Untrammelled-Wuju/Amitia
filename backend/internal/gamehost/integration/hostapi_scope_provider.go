package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/gamehost/hostapi"
)

type HostAPIScopeProvider struct {
	manager scope.ScopeManager
	mu      sync.RWMutex
	cache   map[string]scope.ScopeSnapshot
}

func NewHostAPIScopeProvider(manager scope.ScopeManager) *HostAPIScopeProvider {
	return &HostAPIScopeProvider{
		manager: manager,
		cache:   make(map[string]scope.ScopeSnapshot),
	}
}

func (p *HostAPIScopeProvider) CurrentSnapshotID(
	ctx context.Context,
	extensionID string,
	moduleID string,
	generation int64,
) (snapshotID string, ok bool, err error) {
	if p.manager == nil {
		return "", false, fmt.Errorf("hostapi scope: manager not configured")
	}

	cacheKey := fmt.Sprintf("%s:%s:%d", extensionID, moduleID, generation)

	p.mu.RLock()
	cached, exists := p.cache[cacheKey]
	p.mu.RUnlock()

	if exists {
		if cached.ExpiresAt == nil || time.Now().Before(*cached.ExpiresAt) {
			return cached.SnapshotID, true, nil
		}
	}

	snap, err := p.manager.Snapshot(ctx, scope.ScopeResolveRequest{
		ExtensionID: extensionID,
		ModuleID:    moduleID,
		Generation:  generation,
	})
	if err != nil {
		return "", false, fmt.Errorf("hostapi scope: snapshot failed: %w", err)
	}

	p.mu.Lock()
	p.cache[cacheKey] = snap
	p.mu.Unlock()

	return snap.SnapshotID, true, nil
}

var _ hostapi.ScopeSnapshotIDProvider = (*HostAPIScopeProvider)(nil)

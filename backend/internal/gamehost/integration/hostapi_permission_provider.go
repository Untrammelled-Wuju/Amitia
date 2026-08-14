package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/hostapi"
)

type HostAPIPermissionProvider struct {
	mu        sync.RWMutex
	snapshots map[string]*permission.PermissionSnapshot
	ordered   []string
}

func NewHostAPIPermissionProvider() *HostAPIPermissionProvider {
	return &HostAPIPermissionProvider{
		snapshots: make(map[string]*permission.PermissionSnapshot),
	}
}

func (p *HostAPIPermissionProvider) TrackSnapshot(snap permission.PermissionSnapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshots[snap.SnapshotID] = &snap
	p.ordered = append(p.ordered, snap.SnapshotID)
}

func (p *HostAPIPermissionProvider) CurrentSnapshotID(
	ctx context.Context,
	extensionID string,
	moduleID string,
	generation int64,
) (snapshotID string, ok bool, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	now := time.Now().UTC()
	for i := len(p.ordered) - 1; i >= 0; i-- {
		snap, exists := p.snapshots[p.ordered[i]]
		if !exists {
			continue
		}
		if snap.ExtensionID != extensionID {
			continue
		}
		if moduleID != "" && snap.ModuleID != "" && snap.ModuleID != moduleID {
			continue
		}
		if generation > 0 && snap.Generation != 0 && snap.Generation != generation {
			continue
		}
		if snap.ExpiresAt != nil && now.After(*snap.ExpiresAt) {
			continue
		}
		if snap.RevokedAt != nil {
			continue
		}
		return snap.SnapshotID, true, nil
	}
	return "", false, nil
}

var _ hostapi.PermissionSnapshotIDProvider = (*HostAPIPermissionProvider)(nil)

type kernelPermissionBridge struct {
	broker permission.PermissionBroker
	store  permission.PermissionSnapshotStore
}

func NewKernelPermissionBridge(broker permission.PermissionBroker, store permission.PermissionSnapshotStore) *kernelPermissionBridge {
	return &kernelPermissionBridge{broker: broker, store: store}
}

func (b *kernelPermissionBridge) CreateSnapshot(ctx context.Context, req permission.PermissionSnapshotRequest) (permission.PermissionSnapshot, error) {
	snap := permission.NewPermissionSnapshot(req)
	if b.store != nil {
		if err := b.store.SaveSnapshot(ctx, snap); err != nil {
			return permission.PermissionSnapshot{}, fmt.Errorf("save permission snapshot: %w", err)
		}
	}
	return snap, nil
}

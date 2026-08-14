package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/gamehost/hostapi"
)

type HostAPIPermissionProvider struct {
	store permission.PermissionSnapshotStore
}

func NewHostAPIPermissionProvider(store permission.PermissionSnapshotStore) *HostAPIPermissionProvider {
	return &HostAPIPermissionProvider{store: store}
}

func (p *HostAPIPermissionProvider) CurrentSnapshotID(
	ctx context.Context,
	extensionID string,
	moduleID string,
	generation int64,
) (snapshotID string, ok bool, err error) {
	if p == nil || p.store == nil {
		return "", false, fmt.Errorf("hostapi permission: snapshot store not configured")
	}
	finder, ok := p.store.(permission.ActivePermissionSnapshotFinder)
	if !ok {
		return "", false, fmt.Errorf("hostapi permission: snapshot store does not support active snapshot lookup")
	}
	snap, ok, err := finder.FindActiveSnapshot(ctx, extensionID, moduleID, generation)
	if err != nil {
		return "", false, fmt.Errorf("find active snapshot: %w", err)
	}
	if !ok {
		return "", false, nil
	}
	return snap.SnapshotID, true, nil
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

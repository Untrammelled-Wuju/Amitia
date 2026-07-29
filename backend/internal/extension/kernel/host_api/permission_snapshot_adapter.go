package host_api

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
)

type PermissionSnapshotStoreAdapter struct {
	Reader PermissionSnapshotReader
}

func NewPermissionSnapshotStoreAdapter(reader PermissionSnapshotReader) *PermissionSnapshotStoreAdapter {
	return &PermissionSnapshotStoreAdapter{Reader: reader}
}

func (a *PermissionSnapshotStoreAdapter) Get(ctx context.Context, snapshotID string) (*permission.PermissionSnapshot, error) {
	if a == nil || a.Reader == nil {
		return nil, fmt.Errorf("host_api: permission snapshot reader not wired")
	}
	snap, err := a.Reader.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

type BrokerPermissionSnapshotChecker struct {
	SnapshotStore PermissionSnapshotStore
	Validator    *permission.PermissionIDValidator
	Now          func() time.Time
}

func NewBrokerPermissionSnapshotChecker(store PermissionSnapshotStore) *BrokerPermissionSnapshotChecker {
	return &BrokerPermissionSnapshotChecker{SnapshotStore: store, Now: time.Now}
}

func (c *BrokerPermissionSnapshotChecker) WithValidator(v *permission.PermissionIDValidator) *BrokerPermissionSnapshotChecker {
	c.Validator = v
	return c
}

func (c *BrokerPermissionSnapshotChecker) Check(ctx context.Context, identity runtime_supervisor.RuntimeIdentity, permissionSnapshotID string, reqs []PermissionRequirement) error {
	if c == nil || c.SnapshotStore == nil {
		return nil
	}

	if permissionSnapshotID == "" {
		return nil
	}

	snap, err := c.SnapshotStore.Get(ctx, permissionSnapshotID)
	if err != nil || snap == nil {
		return fmt.Errorf("%w: permission snapshot %s not found", ErrPermissionDenied, permissionSnapshotID)
	}

	now := c.Now()
	if snap.IsExpired(now) {
		return fmt.Errorf("%w: permission snapshot %s expired", ErrPermissionDenied, permissionSnapshotID)
	}

	if snap.IsRevoked() {
		return fmt.Errorf("%w: permission snapshot %s revoked", ErrPermissionDenied, permissionSnapshotID)
	}

	extID := string(identity.ExtensionID)
	modID := string(identity.ModuleID)
	if err := snap.VerifyIdentity(extID, modID, identity.Generation); err != nil {
		return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
	}

	if c.Validator != nil && snap.HasActionIDs(c.Validator) {
		return fmt.Errorf("%w: permission snapshot %s contains action IDs instead of permission IDs", ErrPermissionDenied, permissionSnapshotID)
	}

	if len(reqs) > 0 {
		if len(snap.GrantedPerms) == 0 {
			return fmt.Errorf("%w: permission snapshot %s has no granted permissions but %d required", ErrPermissionDenied, permissionSnapshotID, len(reqs))
		}
		permSet := make(map[string]bool, len(snap.GrantedPerms))
		for _, p := range snap.GrantedPerms {
			permSet[p] = true
		}
		for _, req := range reqs {
			if req.Name != "" && !permSet[req.Name] {
				return fmt.Errorf("%w: permission %s not in snapshot", ErrPermissionDenied, req.Name)
			}
		}
	}

	return nil
}

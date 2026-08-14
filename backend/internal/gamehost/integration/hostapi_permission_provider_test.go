package integration

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
)

func TestHostAPIPermissionProviderReadsKernelSnapshotStore(t *testing.T) {
	store := permission.NewMemoryPermissionSnapshotStore()
	snapshot := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{
		ExtensionID: "example.test/game",
		ModuleID:    "runtime",
		Generation:  3,
	})
	if err := store.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	provider := NewHostAPIPermissionProvider(store)
	snapshotID, ok, err := provider.CurrentSnapshotID(context.Background(), "example.test/game", "runtime", 3)
	if err != nil {
		t.Fatalf("resolve snapshot: %v", err)
	}
	if !ok || snapshotID != snapshot.SnapshotID {
		t.Fatalf("expected kernel snapshot %q, got %q", snapshot.SnapshotID, snapshotID)
	}
}

func TestHostAPIPermissionProviderRejectsMismatchedIdentity(t *testing.T) {
	store := permission.NewMemoryPermissionSnapshotStore()
	snapshot := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{
		ExtensionID: "example.test/game",
		ModuleID:    "runtime",
		Generation:  3,
	})
	if err := store.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}

	provider := NewHostAPIPermissionProvider(store)
	_, ok, err := provider.CurrentSnapshotID(context.Background(), "example.test/game", "runtime", 4)
	if err != nil {
		t.Fatalf("resolve snapshot: %v", err)
	}
	if ok {
		t.Fatal("expected no snapshot for a different generation")
	}
}

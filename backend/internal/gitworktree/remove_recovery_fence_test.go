package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveRejectsUnsafeOrphanCheckpointBeforeGitDeletion(t *testing.T) {
	manager, root, entry := checkpointFixture(t)
	ctx := context.Background()
	if _, err := manager.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1"); err != nil {
		t.Fatal(err)
	}
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(manager.bindingPath(key, entry.ID)); err != nil {
		t.Fatal(err)
	}
	checkpoint := manager.checkpointPath(key, entry.ID)
	if err := os.Remove(checkpoint); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, checkpoint); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := manager.Remove(ctx, root, entry.ID); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("unsafe checkpoint removal should fail before git deletion: %v", err)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Fatalf("git worktree deleted despite checkpoint invalidation failure: %v", err)
	}
	if contents, err := os.ReadFile(outside); err != nil || string(contents) != "keep" {
		t.Fatalf("external file unexpectedly modified: %q %v", contents, err)
	}
	if err := os.Remove(checkpoint); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Remove(ctx, root, entry.ID); err != nil {
		t.Fatalf("remove after stale checkpoint cleanup: %v", err)
	}
	if _, err := os.Stat(entry.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("managed worktree still present: %v", err)
	}
}

func TestRemoveDiscardsOrphanCheckpointBeforeSuccessfulDeletion(t *testing.T) {
	manager, root, entry := checkpointFixture(t)
	ctx := context.Background()
	if _, err := manager.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1"); err != nil {
		t.Fatal(err)
	}
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(manager.bindingPath(key, entry.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Remove(ctx, root, entry.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(manager.checkpointPath(key, entry.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale approval data remains after removal: %v", err)
	}
}

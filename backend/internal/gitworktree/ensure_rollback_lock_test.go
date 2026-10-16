package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnsureTaskRollbackDoesNotReacquireHeldMergeLock(t *testing.T) {
	manager, root := fixture(t)
	_, key, err := manager.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	poisoned := filepath.Join(manager.managedRoot(key), ".checkpoints")
	if err := os.MkdirAll(filepath.Dir(poisoned), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(poisoned, []byte("unexpected file"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, created, err := manager.EnsureTask(ctx, root, "", "HEAD", "rollback-job", "rollback-conversation")
	if err == nil || created {
		t.Fatalf("invalid checkpoint storage was accepted: %v %v", created, err)
	}
	if errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
		t.Fatalf("rollback deadlocked on already-held merge lock: %v", err)
	}
	if !strings.Contains(err.Error(), "worktree rollback") {
		t.Fatalf("failed rollback was not reported to caller: %v", err)
	}
	entries, err := manager.List(context.Background(), root)
	if err != nil || len(entries) != 2 || !entries[1].Managed || entries[1].TaskBinding != nil {
		t.Fatalf("failed bind must leave orphan worktree visible for recovery: %+v %v", entries, err)
	}
	if err := os.Remove(poisoned); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Remove(context.Background(), root, entries[1].ID); err != nil {
		t.Fatalf("orphan worktree could not be safely reclaimed: %v", err)
	}
}

func TestRemoveWithHeldMergeLockAvoidsReentrantAcquisition(t *testing.T) {
	manager, root := fixture(t)
	entry, err := manager.Create(context.Background(), root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := manager.lockRepositoryMerge(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	removed, err := manager.removeWithMergeLock(ctx, root, entry.ID)
	if err != nil || removed.ID != entry.ID || ctx.Err() != nil {
		t.Fatalf("held-lock removal failed: %+v %v", removed, err)
	}
	if _, err := os.Stat(entry.Path); !os.IsNotExist(err) {
		t.Fatalf("removed worktree still exists: %v", err)
	}
}

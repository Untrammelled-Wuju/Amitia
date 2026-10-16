package gitworktree

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestBindRejectsOrphanPendingMerge(t *testing.T) {
	manager, root, entry, _ := pendingMergeFixture(t)
	ctx := context.Background()
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(manager.bindingPath(key, entry.ID)); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(manager.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Bind(ctx, root, entry.ID, "other-task", "other-conversation"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("orphan pending audit was reassigned: %v", err)
	}
	binding, err := restarted.readBinding(key, entry.ID)
	if err != nil || binding != nil {
		t.Fatalf("unexpected replacement binding: %+v %v", binding, err)
	}
	if _, err := os.Stat(restarted.mergeAuditPath(key, entry.ID)); err != nil {
		t.Fatalf("recovery journal lost: %v", err)
	}
}

func TestBindRejectsCompletedMergeJournalReuse(t *testing.T) {
	manager, root, entry, plan := pendingMergeFixture(t)
	ctx := context.Background()
	gitTest(t, root, "merge", "--ff-only", plan.SourceHead)
	if audit, err := manager.ReadMergeAudit(ctx, root, entry.ID, "task-1", "conversation-1"); err != nil || audit.Result != "applied" {
		t.Fatalf("merge reconciliation: %+v %v", audit, err)
	}
	if err := manager.Release(ctx, root, entry.ID, "task-1", "conversation-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Bind(ctx, root, entry.ID, "other-task", "other-conversation"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("completed audit was silently reused: %v", err)
	}
}

func TestBindRejectsOrphanCheckpointAndCorruptJournal(t *testing.T) {
	manager, root, entry := checkpointFixture(t)
	ctx := context.Background()
	checkpoint, err := manager.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil || checkpoint == nil {
		t.Fatalf("prepare: %+v %v", checkpoint, err)
	}
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(manager.bindingPath(key, entry.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Bind(ctx, root, entry.ID, "other-task", "other-conversation"); !errors.Is(err, ErrCheckpointRequired) {
		t.Fatalf("orphan checkpoint was rebound: %v", err)
	}
	if err := os.WriteFile(manager.mergeAuditPath(key, entry.ID), []byte(`{"result":"pending"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Bind(ctx, root, entry.ID, "other-task", "other-conversation"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("corrupt audit was bypassed: %v", err)
	}
}

func TestBindWaitsForMergeLock(t *testing.T) {
	manager, root := fixture(t)
	ctx := context.Background()
	entry, err := manager.Create(ctx, root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := manager.lockRepositoryMerge(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	deadline, cancel := context.WithTimeout(ctx, 90*time.Millisecond)
	defer cancel()
	if _, err := manager.Bind(deadline, root, entry.ID, "task-lock", "conversation-lock"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("new binding bypassed merge lock: %v", err)
	}
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := manager.readBinding(key, entry.ID)
	if err != nil || saved != nil {
		t.Fatalf("binding modified before merge unlock: %+v %v", saved, err)
	}
}

func TestBindAllowsCleanReuseWithoutAudit(t *testing.T) {
	manager, root := fixture(t)
	ctx := context.Background()
	entry, err := manager.Create(ctx, root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Bind(ctx, root, entry.ID, "first-task", "first-conversation"); err != nil {
		t.Fatal(err)
	}
	if err := manager.Release(ctx, root, entry.ID, "first-task", "first-conversation"); err != nil {
		t.Fatal(err)
	}
	rebound, err := manager.Bind(ctx, root, entry.ID, "second-task", "second-conversation")
	if err != nil || rebound.TaskID != "second-task" {
		t.Fatalf("clean reuse failed: %+v %v", rebound, err)
	}
}

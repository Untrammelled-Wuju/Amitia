package gitworktree

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestPendingMergeProtectsCheckpointBindingAndRecovery(t *testing.T) {
	manager, root, entry, plan := pendingMergeFixture(t)
	ctx := context.Background()
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(manager.checkpointPath(key, entry.ID))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Release(ctx, root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("release discarded unresolved audit: %v", err)
	}
	if err := manager.InvalidateCheckpoint(ctx, root, entry.ID); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("invalidation discarded unresolved audit: %v", err)
	}
	if _, err := manager.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("preparation ignored unresolved audit: %v", err)
	}
	if _, err := manager.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", plan.Revision, "go"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("test rerun ignored unresolved audit: %v", err)
	}
	if _, err := manager.ApproveCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1", plan.Revision, "local-owner"); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("reapproval ignored unresolved audit: %v", err)
	}
	after, err := os.ReadFile(manager.checkpointPath(key, entry.ID))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("pending merge checkpoint changed")
	}
	binding, err := manager.readBinding(key, entry.ID)
	if err != nil || binding == nil || binding.TaskID != "task-1" {
		t.Fatalf("binding lost: %+v %v", binding, err)
	}
	gitTest(t, root, "merge", "--ff-only", plan.SourceHead)
	recovered, err := New(manager.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := recovered.ReadMergeAudit(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil || audit.Result != "applied" || audit.RecoveredAt == nil {
		t.Fatalf("pending audit not recovered after restart: %+v %v", audit, err)
	}
	if err := recovered.Release(ctx, root, entry.ID, "task-1", "conversation-1"); err != nil {
		t.Fatalf("resolved merge should allow binding cleanup: %v", err)
	}
}

func TestPendingAuditBlocksOrphanWorktreeRemoval(t *testing.T) {
	manager, root, entry, _ := pendingMergeFixture(t)
	_, key, err := manager.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(manager.bindingPath(key, entry.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Remove(context.Background(), root, entry.ID); !errors.Is(err, ErrMergePreviouslyAttempted) {
		t.Fatalf("orphan cleanup destroyed pending audit: %v", err)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Fatalf("pending source removed: %v", err)
	}
}

func TestReleaseRejectsCorruptedMergeAudit(t *testing.T) {
	manager, root, entry, _ := pendingMergeFixture(t)
	_, key, err := manager.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.mergeAuditPath(key, entry.ID), []byte(`{"version":1,"result":"pending"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := manager.Release(context.Background(), root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid journal bypassed release fence: %v", err)
	}
	binding, err := manager.readBinding(key, entry.ID)
	if err != nil || binding == nil {
		t.Fatalf("binding disappeared after corrupt journal: %+v %v", binding, err)
	}
}

func TestReleaseWaitsForInFlightMergeLock(t *testing.T) {
	manager, root, entry := checkpointFixture(t)
	ctx := context.Background()
	held, err := manager.lockRepositoryMerge(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer held()
	deadline, cancel := context.WithTimeout(ctx, 70*time.Millisecond)
	defer cancel()
	if err := manager.Release(deadline, root, entry.ID, "task-1", "conversation-1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("release bypassed concurrent merge: %v", err)
	}
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := manager.readBinding(key, entry.ID)
	if err != nil || binding == nil {
		t.Fatalf("binding changed while merge lock held: %+v %v", binding, err)
	}
}

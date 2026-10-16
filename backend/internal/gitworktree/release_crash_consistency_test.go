package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestReleasePreservesBindingIfCheckpointCannotBeInvalidated(t *testing.T) {
	manager, root := fixture(t)
	ctx := context.Background()
	entry, err := manager.Create(ctx, root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Bind(ctx, root, entry.ID, "task-release", "conv-release"); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.PrepareCheckpoint(ctx, root, entry.ID, "task-release", "conv-release"); err != nil {
		t.Fatal(err)
	}
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := manager.checkpointPath(key, entry.ID)
	if err = os.Remove(checkpoint); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filepath.Join(t.TempDir(), "untrusted"), checkpoint); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err = manager.Release(ctx, root, entry.ID, "task-release", "conv-release"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected checkpoint invalidation failure, got %v", err)
	}
	listed, err := manager.List(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	preserved := false
	for _, item := range listed {
		if item.ID == entry.ID && item.TaskBinding != nil && item.TaskBinding.TaskID == "task-release" {
			preserved = true
		}
	}
	if !preserved {
		t.Fatal("binding disappeared before checkpoint invalidation succeeded")
	}
	if err = os.Remove(checkpoint); err != nil {
		t.Fatal(err)
	}
	if err = manager.Release(ctx, root, entry.ID, "task-release", "conv-release"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(manager.bindingPath(key, entry.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("binding not removed: %v", err)
	}
}

func TestMetadataDirectorySyncAfterAtomicReplacement(t *testing.T) {
	manager, root := fixture(t)
	ctx := context.Background()
	entry, err := manager.Create(ctx, root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Bind(ctx, root, entry.ID, "task-durable", "conv-durable"); err != nil {
		t.Fatal(err)
	}
	checkpoint, err := manager.PrepareCheckpoint(ctx, root, entry.ID, "task-durable", "conv-durable")
	if err != nil {
		t.Fatal(err)
	}
	other, err := New(manager.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := other.ReadCheckpoint(ctx, root, entry.ID, "task-durable", "conv-durable")
	if err != nil || recovered == nil || recovered.Revision != checkpoint.Revision {
		t.Fatalf("checkpoint read after reinitialization failed: %+v %v", recovered, err)
	}
	_, key, err := other.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if err = other.Release(ctx, root, entry.ID, "task-durable", "conv-durable"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(other.checkpointPath(key, entry.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale checkpoint after release: %v", err)
	}
}

func TestMetadataRejectsWorldReadableBindingAndCheckpoint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX metadata permissions")
	}
	manager, root := fixture(t)
	ctx := context.Background()
	entry, err := manager.Create(ctx, root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Bind(ctx, root, entry.ID, "task-secret", "conv-secret"); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.PrepareCheckpoint(ctx, root, entry.ID, "task-secret", "conv-secret"); err != nil {
		t.Fatal(err)
	}
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	bindingPath := manager.bindingPath(key, entry.ID)
	checkpointPath := manager.checkpointPath(key, entry.ID)
	for _, path := range []string{bindingPath, checkpointPath} {
		if err = os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = manager.readBinding(key, entry.ID); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("unsafe binding file permitted: %v", err)
	}
	if _, err = manager.readCheckpoint(key, entry.ID); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("unsafe checkpoint file permitted: %v", err)
	}
	for _, path := range []string{bindingPath, checkpointPath} {
		if err = os.Chmod(path, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = manager.readBinding(key, entry.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.readCheckpoint(key, entry.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMergeAuditRejectsWorldReadableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX metadata permissions")
	}
	ctx := context.Background()
	manager, root, entry, _ := approvedMergeFixture(t)
	plan, err := manager.PreviewApprovedMerge(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := manager.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	audit := MergeAudit{Version: 1, Plan: *plan, Actor: plan.ApprovedBy, StartedAt: time.Now().UTC(), Result: "pending"}
	if err := manager.writeMergeAudit(key, entry.ID, audit); err != nil {
		t.Fatal(err)
	}
	path := manager.mergeAuditPath(key, entry.ID)
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.readMergeAudit(key, entry.ID); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("unsafe merge audit permitted: %v", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.readMergeAudit(key, entry.ID); err != nil {
		t.Fatal(err)
	}
}

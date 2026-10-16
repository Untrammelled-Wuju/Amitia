package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestTaskBindingLifecycle(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	tree, err := m.Create(ctx, root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	attached, err := m.Bind(ctx, root, tree.ID, "task-101", "conv-1")
	if err != nil || attached.TaskID != "task-101" {
		t.Fatalf("bind: %#v %v", attached, err)
	}
	attached2, err := m.Bind(ctx, root, tree.ID, "task-101", "conv-1")
	if err != nil || attached2.BoundAt != attached.BoundAt {
		t.Fatalf("idempotent bind: %#v %v", attached2, err)
	}
	if _, err := m.Bind(ctx, root, tree.ID, "task-102", "conv-1"); !errors.Is(err, ErrBound) {
		t.Fatalf("overwrite should fail: %v", err)
	}
	if _, err := m.Remove(ctx, root, tree.ID); !errors.Is(err, ErrBound) {
		t.Fatalf("active removal: %v", err)
	}
	another, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := another.List(ctx, tree.Path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range listed {
		if item.ID == tree.ID && item.TaskBinding != nil && item.TaskBinding.ConversationID == "conv-1" {
			found = true
		}
	}
	if !found {
		t.Fatal("binding did not survive manager recreation")
	}
	if err := another.Release(ctx, root, tree.ID, "task-101", "conv-wrong"); !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("mismatched release: %v", err)
	}
	if err := another.Release(ctx, root, tree.ID, "task-101", "conv-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := another.Remove(ctx, root, tree.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBindingRejectsUnmanagedAndInvalidIDs(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	if _, err := m.Bind(ctx, root, "not-an-id", "task1", "conv1"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatal(err)
	}
	if _, err := m.Bind(ctx, root, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "task1", "conv1"); !errors.Is(err, ErrNotManaged) {
		t.Fatal(err)
	}
	tree, err := m.Create(ctx, root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"../other", "  ", "x/y", "x\n"} {
		if _, err := m.Bind(ctx, root, tree.ID, bad, "conv1"); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("bad %q: %v", bad, err)
		}
	}
	if err := m.Release(ctx, root, tree.ID, "task1", "conv1"); !errors.Is(err, ErrBindingMismatch) {
		t.Fatal(err)
	}
}

func TestTaskBindingStorageSymlinkDenied(t *testing.T) {
	m, root := fixture(t)
	tree, err := m.Create(context.Background(), root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(m.bindingPath(key, tree.ID))
	if err := os.Symlink(t.TempDir(), dir); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := m.Bind(context.Background(), root, tree.ID, "task1", "conv1"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("symlink storage permitted: %v", err)
	}
}

func TestTaskBindingConcurrentClaim(t *testing.T) {
	m, root := fixture(t)
	tree, err := m.Create(context.Background(), root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.Bind(context.Background(), root, tree.ID, "task-parallel", "conv1")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("identical claims should be idempotent: %v", err)
		}
	}
}

func TestConcurrentCreateRespectsCap(t *testing.T) {
	m, root := fixture(t)
	var wg sync.WaitGroup
	results := make(chan error, MaxManagedWorktrees+8)
	for i := 0; i < MaxManagedWorktrees+8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := m.Create(context.Background(), root, "", "HEAD")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	if successes != MaxManagedWorktrees {
		t.Fatalf("expected exactly %d worktrees, got %d", MaxManagedWorktrees, successes)
	}
}

func TestCorruptBindingFailsClosed(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	tree, err := m.Create(ctx, root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Bind(ctx, root, tree.ID, "task1", "conv1"); err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := m.bindingPath(key, tree.ID)
	if err := os.WriteFile(path, []byte("invalid json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.List(ctx, root); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid binding should block listing: %v", err)
	}
	if _, err := m.Remove(ctx, root, tree.ID); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid binding should block removal: %v", err)
	}
	if _, err := os.Stat(tree.Path); err != nil {
		t.Fatalf("worktree should still exist: %v", err)
	}
}

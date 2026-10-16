package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEnsureTaskReusesExistingAfterRestart(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	first, created, err := m.EnsureTask(ctx, root, "", "HEAD", "task100", "conv100")
	if err != nil || !created || first.TaskBinding == nil {
		t.Fatalf("first: %+v %v %v", first, created, err)
	}
	other, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	second, created, err := other.EnsureTask(ctx, root, "", "HEAD", "task100", "conv100")
	if err != nil || created || first.ID != second.ID || first.Path != second.Path {
		t.Fatalf("reused: %+v %v %v", second, created, err)
	}
	entries, err := m.List(ctx, root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("duplicates: %d %v", len(entries), err)
	}
}

func TestEnsureTaskRejectsRebindingAndMismatchedBranch(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	_, _, err := m.EnsureTask(ctx, root, "topic/branch", "HEAD", "jobA", "convA")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ branch, task, conv string }{
		{"topic/other", "jobA", "convA"},
		{"", "jobA", "convB"},
	} {
		if _, _, err := m.EnsureTask(ctx, root, tc.branch, "HEAD", tc.task, tc.conv); !errors.Is(err, ErrBound) {
			t.Fatalf("%+v got %v", tc, err)
		}
	}
	entries, err := m.List(ctx, root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected worktrees: %d %v", len(entries), err)
	}
}

func TestEnsureTaskConcurrentIdempotency(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan Entry, 12)
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e, _, err := m.EnsureTask(ctx, root, "", "HEAD", "parallel-job", "conv100")
			results <- e
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for e := range results {
		if id != "" && e.ID != id {
			t.Fatalf("worktree duplication %s != %s", id, e.ID)
		}
		id = e.ID
	}
	entries, err := m.List(ctx, root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("unexpected worktrees: %d %v", len(entries), err)
	}
}

func TestEnsureTaskRetainsDirtyWorktree(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	e, created, err := m.EnsureTask(ctx, root, "", "HEAD", "job", "conversation")
	if err != nil || !created {
		t.Fatalf("allocate: %v", err)
	}
	artifact := filepath.Join(e.Path, "work-in-progress.txt")
	if err := os.WriteFile(artifact, []byte("preserve work"), 0600); err != nil {
		t.Fatal(err)
	}
	existing, created, err := m.EnsureTask(ctx, root, "", "HEAD", "job", "conversation")
	if err != nil || created || existing.ID != e.ID {
		t.Fatalf("expected recovery: %+v %v %v", existing, created, err)
	}
	if _, err := os.Stat(artifact); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Remove(ctx, root, e.ID); !errors.Is(err, ErrBound) {
		t.Fatalf("bound worktree removed: %v", err)
	}
}

func TestEnsureTaskRejectsMalformedIdentity(t *testing.T) {
	m, root := fixture(t)
	for _, task := range []string{"", "../task", "--", " bad", "bad/task"} {
		if _, _, err := m.EnsureTask(context.Background(), root, "", "HEAD", task, "conversation"); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("malformed %q: %v", task, err)
		}
	}
}

func TestEnsureTaskBaselineSurvivesCommits(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	initial := gitTest(t, root, "rev-parse", "HEAD")
	entry, _, err := m.EnsureTask(ctx, root, "", "HEAD", "task-baseline", "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if entry.TaskBinding.BaseHead != initial {
		t.Fatalf("expected %s baseline, got %+v", initial, entry.TaskBinding)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, "change.txt"), []byte("final result"), 0600); err != nil {
		t.Fatal(err)
	}
	gitTest(t, entry.Path, "add", "change.txt")
	gitTest(t, entry.Path, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-q", "-m", "agent result")
	restored, created, err := m.EnsureTask(ctx, root, "", "HEAD", "task-baseline", "conv-1")
	if err != nil || created {
		t.Fatalf("expected same task: %v %v", created, err)
	}
	if restored.TaskBinding.BaseHead != initial {
		t.Fatalf("baseline changed after commit: %+v", restored.TaskBinding)
	}
	output := gitTest(t, entry.Path, "diff", "--name-only", initial)
	if output != "change.txt" {
		t.Fatalf("committed changes disappeared from review base: %q", output)
	}
}

func TestInvalidBaselineFailsClosedAndLegacyBindingIsReadable(t *testing.T) {
	m, root := fixture(t)
	ctx := context.Background()
	entry, _, err := m.EnsureTask(ctx, root, "", "HEAD", "task-legacy", "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	_, key, err := m.repository(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	path := m.bindingPath(key, entry.ID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.Replace(string(data), `,"baseHead":"`+entry.TaskBinding.BaseHead+`"`, "", 1)
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	existing, err := m.List(ctx, root)
	if err != nil {
		t.Fatalf("old binding rejected: %v", err)
	}
	for _, item := range existing {
		if item.ID == entry.ID && item.TaskBinding.BaseHead != "" {
			t.Fatal("legacy baseline fabricated")
		}
	}
	invalid := strings.Replace(legacy, `"taskId":"task-legacy"`, `"baseHead":"../bad","taskId":"task-legacy"`, 1)
	if err := os.WriteFile(path, []byte(invalid), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.List(ctx, root); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("malformed baseline accepted: %v", err)
	}
}

func TestEnsureTaskWaitsForMergeLockBeforeAllocation(t *testing.T) {
	manager, root := fixture(t)
	ctx := context.Background()
	release, err := manager.lockRepositoryMerge(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	deadline, cancel := context.WithTimeout(ctx, 90*time.Millisecond)
	defer cancel()
	_, _, err = manager.EnsureTask(deadline, root, "", "HEAD", "task-lock", "conversation-lock")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("allocation bypassed repository merge: %v", err)
	}
	items, err := manager.List(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("merge-locked allocation created worktrees: %d", len(items))
	}
}

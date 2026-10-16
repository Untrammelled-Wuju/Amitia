package gitworktree

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnsureRollbackCompletesAfterCallerCancellation(t *testing.T) {
	for _, test := range []struct {
		name        string
		makeContext func() (context.Context, context.CancelFunc, error)
	}{
		{"cancelled", func() (context.Context, context.CancelFunc, error) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel, context.Canceled
		}},
		{"deadline", func() (context.Context, context.CancelFunc, error) {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			return ctx, cancel, context.DeadlineExceeded
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, repo := fixture(t)
			item, err := m.Create(context.Background(), repo, "", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel, expected := test.makeContext()
			defer cancel()
			unlock, err := m.lockRepositoryMerge(context.Background(), repo)
			if err != nil {
				t.Fatal(err)
			}
			returned := m.rollbackCreatedTask(ctx, repo, item.ID, expected)
			unlock()
			if !errors.Is(returned, expected) {
				t.Fatalf("original cancellation lost: %v", returned)
			}
			if _, err := os.Stat(item.Path); !os.IsNotExist(err) {
				t.Fatalf("cancellation left orphan worktree: %v", err)
			}
			entries, err := m.List(context.Background(), repo)
			if err != nil || len(entries) != 1 {
				t.Fatalf("repository was not restored: %+v %v", entries, err)
			}
		})
	}
}

func TestEnsureRollbackPreservesBoundWorktreeAfterCancellation(t *testing.T) {
	m, repo := fixture(t)
	item, _, err := m.EnsureTask(context.Background(), repo, "", "HEAD", "task-bound", "conv-bound")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	unlock, err := m.lockRepositoryMerge(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	returned := m.rollbackCreatedTask(ctx, repo, item.ID, context.Canceled)
	unlock()
	if !errors.Is(returned, context.Canceled) || !strings.Contains(returned.Error(), "worktree rollback") {
		t.Fatalf("rollback refused to report protected worktree: %v", returned)
	}
	if _, err := os.Stat(item.Path); err != nil {
		t.Fatalf("bound worktree was improperly removed: %v", err)
	}
}

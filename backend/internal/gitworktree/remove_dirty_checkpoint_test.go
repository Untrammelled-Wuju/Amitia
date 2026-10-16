package gitworktree

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveDirtyOrphanRetainsCheckpointUntilGitWillAcceptRemoval(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, path string)
		cleanup func(t *testing.T, path string)
	}{
		{
			name: "untracked",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(path, "pending-file.txt"), []byte("not reviewed"), 0600); err != nil {
					t.Fatal(err)
				}
			},
			cleanup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Remove(filepath.Join(path, "pending-file.txt")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "staged",
			prepare: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(path, "staged.txt"), []byte("not reviewed"), 0600); err != nil {
					t.Fatal(err)
				}
				gitTest(t, path, "add", "--", "staged.txt")
			},
			cleanup: func(t *testing.T, path string) {
				t.Helper()
				gitTest(t, path, "reset", "--", "staged.txt")
				if err := os.Remove(filepath.Join(path, "staged.txt")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
			checkpointPath := manager.checkpointPath(key, entry.ID)
			before, err := os.ReadFile(checkpointPath)
			if err != nil {
				t.Fatal(err)
			}
			test.prepare(t, entry.Path)
			if _, err := manager.Remove(ctx, root, entry.ID); !errors.Is(err, ErrDirty) {
				t.Fatalf("dirty removal should be rejected before evidence deletion: %v", err)
			}
			after, err := os.ReadFile(checkpointPath)
			if err != nil || !bytes.Equal(after, before) {
				t.Fatalf("checkpoint changed after rejected remove: %v", err)
			}
			if _, err := os.Stat(entry.Path); err != nil {
				t.Fatalf("worktree unexpectedly deleted: %v", err)
			}
			test.cleanup(t, entry.Path)
			if _, err := manager.Remove(ctx, root, entry.ID); err != nil {
				t.Fatalf("clean worktree deletion: %v", err)
			}
			if _, err := os.Lstat(checkpointPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("checkpoint not invalidated on successful deletion: %v", err)
			}
		})
	}
}

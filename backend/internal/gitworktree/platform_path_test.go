package gitworktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestManagedRepositoryPlatformPaths(t *testing.T) {
	m, root := fixture(t)
	canonical, key, err := m.repository(context.Background(), root)
	if err != nil {
		t.Fatal("repository", err)
	}
	directory := filepath.Join(m.managedRoot(key), ".bindings")
	if err = os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil || canonical == "" || filepath.Clean(directory) != filepath.Clean(resolved) {
		t.Fatalf("managed directory does not use canonical platform path: %q %q %v", directory, resolved, err)
	}
	unlock, err := m.lockRepositoryAllocation(context.Background(), root)
	if err != nil {
		t.Fatal("allocation", err)
	}
	unlock()
	unlock, err = m.lockRepositoryMerge(context.Background(), root)
	if err != nil {
		t.Fatal("merge", err)
	}
	unlock()
	if _, err = m.Create(context.Background(), root, "", "HEAD"); err != nil {
		t.Fatal("create", err)
	}
}

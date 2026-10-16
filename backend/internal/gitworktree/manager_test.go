package gitworktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitTest(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	command.Env = safeGitEnvironment()
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func fixture(t *testing.T) (*Manager, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "init", "-q")
	gitTest(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.org", "commit", "-q", "--allow-empty", "-m", "initial")
	manager, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return manager, root
}

func TestCreateListRemoveDetached(t *testing.T) {
	m, root := fixture(t)
	created, err := m.Create(context.Background(), root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !created.Managed || !created.Detached || !managedID.MatchString(created.ID) {
		t.Fatalf("unexpected created: %#v", created)
	}
	listed, err := m.List(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 {
		t.Fatalf("list expected primary+managed: %#v", listed)
	}
	found := false
	for _, item := range listed {
		if item.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("created worktree missing")
	}
	if _, err := m.Remove(context.Background(), root, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(created.Path); !os.IsNotExist(err) {
		t.Fatalf("target should be removed: %v", err)
	}
	if _, err := m.Remove(context.Background(), root, created.ID); !errors.Is(err, ErrNotManaged) {
		t.Fatalf("expected unmanaged: %v", err)
	}
}

func TestCreateBranchAndRefInjection(t *testing.T) {
	m, root := fixture(t)
	for _, ref := range []string{"--upload-pack=/bin/sh", "HEAD^{tree}", "HEAD..main", "../HEAD", "not-a-ref"} {
		if _, err := m.Create(context.Background(), root, "", ref); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("ref %q: %v", ref, err)
		}
	}
	for _, branch := range []string{"--option", "../evil", "foo..bar", "foo.lock", "bad@{x}"} {
		if _, err := m.Create(context.Background(), root, branch, "HEAD"); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("branch %q: %v", branch, err)
		}
	}
	entry, err := m.Create(context.Background(), root, "agent/feature", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if entry.Branch != "agent/feature" {
		t.Fatalf("created incorrect branch: %#v", entry)
	}
	if _, err := m.Remove(context.Background(), root, entry.ID); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveProtectsDirtyAndUnmanaged(t *testing.T) {
	m, root := fixture(t)
	managed, err := m.Create(context.Background(), root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(managed.Path, "unsaved.txt"), []byte("work"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Remove(context.Background(), root, managed.ID); !errors.Is(err, ErrDirty) {
		t.Fatalf("dirty removal should fail: %v", err)
	}
	if _, err := os.Stat(filepath.Join(managed.Path, "unsaved.txt")); err != nil {
		t.Fatal("dirty data was deleted", err)
	}
	if _, err := m.Remove(context.Background(), root, "../repo"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("injection not rejected: %v", err)
	}
	other, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := other.Remove(context.Background(), root, managed.ID); !errors.Is(err, ErrNotManaged) {
		t.Fatalf("unmanaged path should be denied: %v", err)
	}
}

func TestLinkedWorktreeSharesRepositoryGroup(t *testing.T) {
	m, root := fixture(t)
	item, err := m.Create(context.Background(), root, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	other, err := m.List(context.Background(), item.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 2 {
		t.Fatalf("linked root should see main and managed: %#v", other)
	}
	if _, err := m.Remove(context.Background(), item.Path, item.ID); err != nil {
		t.Fatal(err)
	}
}

func TestMaxWorktreesAndCancelledContext(t *testing.T) {
	m, root := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Create(ctx, root, "", "HEAD"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation: %v", err)
	}
	created := []Entry{}
	for i := 0; i < MaxManagedWorktrees; i++ {
		item, err := m.Create(context.Background(), root, "", "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		created = append(created, item)
	}
	if _, err := m.Create(context.Background(), root, "", "HEAD"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("limit not enforced: %v", err)
	}
}

func TestExternalGitFiltersAreRejected(t *testing.T) {
	manager, root := fixture(t)
	gitTest(t, root, "config", "--local", "filter.danger.process", "sh -c echo-should-not-run")
	if _, err := manager.Create(context.Background(), root, "", "HEAD"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("repository filter command was not rejected: %v", err)
	}
}

func TestManagedRootSymlinkIsRejected(t *testing.T) {
	manager, root := fixture(t)
	_, key, err := manager.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	parent := manager.managedRoot(key)
	if err := os.MkdirAll(filepath.Dir(parent), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), parent); err != nil {
		t.Skipf("symlinks not available: %v", err)
	}
	if _, err := manager.Create(context.Background(), root, "", "HEAD"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("managed root symlink was not rejected: %v", err)
	}
}

package gitworktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func createFailureShim(t *testing.T, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell failure injection requires Unix")
	}
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "fail-once")
	script := fmt.Sprintf(`#!/bin/sh
case " $* " in
  *" worktree add "*)
    %q "$@" || exit $?
    case %q in
      after-add) exit 53 ;;
      dirty-add)
        prev=
        for arg in "$@"; do target="$prev"; prev="$arg"; done
        printf 'not safe to remove' > "$target/orphan.txt"
        exit 53 ;;
      list-failure) touch %q; exit 0 ;;
      cancelled-add) touch %q; while :; do :; done ;;
    esac
    ;;
  *" worktree list "*)
    if [ %q = list-failure ] && [ -f %q ]; then rm %q; exit 54; fi ;;
esac
exec %q "$@"
`, git, mode, marker, marker, mode, marker, marker, git)
	path := filepath.Join(dir, "git-shim")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCreateFailureRecoversRegisteredWorktree(t *testing.T) {
	for _, mode := range []string{"after-add", "list-failure"} {
		t.Run(mode, func(t *testing.T) {
			m, repo := fixture(t)
			m.gitPath = createFailureShim(t, mode)
			_, err := m.Create(context.Background(), repo, "", "HEAD")
			if err == nil {
				t.Fatal("injected post-creation failure was not returned")
			}
			entries, listErr := m.List(context.Background(), repo)
			if listErr != nil || len(entries) != 1 {
				t.Fatalf("failure left a registered worktree: %+v %v", entries, listErr)
			}
			_, key, err := m.repository(context.Background(), repo)
			if err != nil {
				t.Fatal(err)
			}
			dirs, err := os.ReadDir(m.managedRoot(key))
			if err != nil {
				t.Fatal(err)
			}
			for _, dir := range dirs {
				if managedID.MatchString(dir.Name()) {
					t.Fatalf("failed creation left managed directory %s", dir.Name())
				}
			}
		})
	}
}

func TestCreateFailurePreservesDirtyWorktree(t *testing.T) {
	m, repo := fixture(t)
	m.gitPath = createFailureShim(t, "dirty-add")
	if _, err := m.Create(context.Background(), repo, "", "HEAD"); err == nil || !strings.Contains(err.Error(), "recovery removal failed") || !errors.Is(err, ErrDirty) {
		t.Fatalf("unsafe cleanup must be reported: %v", err)
	}
	entries, err := m.List(context.Background(), repo)
	if err != nil || len(entries) != 2 {
		t.Fatalf("dirty orphan must remain available for inspection: %+v %v", entries, err)
	}
	if _, err := os.Stat(filepath.Join(entries[1].Path, "orphan.txt")); err != nil {
		t.Fatalf("dirty data unexpectedly deleted: %v", err)
	}
}

func TestCancelledCreateRecoversAlreadyRegisteredWorktree(t *testing.T) {
	m, repo := fixture(t)
	shim := createFailureShim(t, "cancelled-add")
	m.gitPath = shim
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	complete := make(chan error, 1)
	go func() {
		_, err := m.Create(ctx, repo, "", "HEAD")
		complete <- err
	}()
	signal := filepath.Join(filepath.Dir(shim), "fail-once")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(signal); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("injected git add did not create worktree")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-complete:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation was not preserved: %v", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("cancelled git creation did not finish recovery")
	}
	entries, err := m.List(context.Background(), repo)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cancelled create leaked managed worktree: %+v %v", entries, err)
	}
}

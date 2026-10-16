package gitworktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMergeLockSerializesDistinctManagers(t *testing.T) {
	m, root, _, _ := approvedMergeFixture(t)
	other, err := New(m.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	release, err := m.lockRepositoryMerge(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Millisecond)
	defer cancel()
	_, err = other.lockRepositoryMerge(ctx, root)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second manager did not wait for active lock: %v", err)
	}
	release()
	release, err = other.lockRepositoryMerge(context.Background(), root)
	if err != nil {
		t.Fatalf("lock not released: %v", err)
	}
	release()
}

func TestMergeLockBlocksSymlinkReplacement(t *testing.T) {
	m, root, _, _ := approvedMergeFixture(t)
	_, key, err := m.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.managedRoot(key), ".merges", ".apply.lock")
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(filepath.Join(t.TempDir(), "escape"), path); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	_, err = m.lockRepositoryMerge(context.Background(), root)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("merge accepted untrusted lock file: %v", err)
	}
}

func TestMergeLockAcrossProcesses(t *testing.T) {
	if os.Getenv("WORKBENCH_MERGE_LOCK_HELPER") == "1" {
		root := os.Getenv("WORKBENCH_MERGE_ROOT")
		dataRoot := os.Getenv("WORKBENCH_MERGE_DATA")
		m, err := New(dataRoot)
		if err != nil {
			t.Fatal(err)
		}
		unlock, err := m.lockRepositoryMerge(context.Background(), root)
		if err != nil {
			t.Fatal(err)
		}
		defer unlock()
		if err = os.WriteFile(os.Getenv("WORKBENCH_MERGE_READY"), []byte("ready"), 0600); err != nil {
			t.Fatal(err)
		}
		time.Sleep(300 * time.Millisecond)
		return
	}
	m, root, _, _ := approvedMergeFixture(t)
	ready := filepath.Join(t.TempDir(), "ready")
	proc := exec.Command(os.Args[0], "-test.run=^TestMergeLockAcrossProcesses$")
	proc.Env = append(os.Environ(), "WORKBENCH_MERGE_LOCK_HELPER=1", "WORKBENCH_MERGE_ROOT="+root, "WORKBENCH_MERGE_DATA="+m.dataRoot, "WORKBENCH_MERGE_READY="+ready)
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proc.Process.Kill(); _ = proc.Wait() }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper failed to acquire merge lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	start := time.Now()
	unlock, err := m.lockRepositoryMerge(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	if time.Since(start) < 140*time.Millisecond {
		t.Fatalf("independent process lock was bypassed after %s", time.Since(start))
	}
}

func TestMergeLockCancellation(t *testing.T) {
	m, root, _, _ := approvedMergeFixture(t)
	unlock, err := m.lockRepositoryMerge(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = m.lockRepositoryMerge(ctx, root)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel ignored: %v", err)
	}
}

func TestMergeLockExpiryPrefersContextDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if err := mergeLockExpiry(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lost context deadline: %v", err)
	}
	if err := mergeLockExpiry(context.Background()); !errors.Is(err, ErrMergeLockUnavailable) {
		t.Fatalf("non-context lock timeout changed: %v", err)
	}
}

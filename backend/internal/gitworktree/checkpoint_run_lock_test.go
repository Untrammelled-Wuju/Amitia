package gitworktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCheckpointExecutionFenceRejectsConcurrentLifecycleOperations(t *testing.T) {
	manager, root, entry := checkpointFixture(t)
	ctx := context.Background()
	record, err := manager.PrepareCheckpoint(ctx, root, entry.ID, "task-1", "conversation-1")
	if err != nil {
		t.Fatal(err)
	}
	other, err := New(manager.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := manager.lockCheckpointExecution(ctx, root, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name string
		call func() error
	}{
		{"another test", func() error {
			_, e := other.RunCheckpointTest(ctx, root, entry.ID, "task-1", "conversation-1", record.Revision, "go")
			return e
		}},
		{"release binding", func() error { return other.Release(ctx, root, entry.ID, "task-1", "conversation-1") }},
		{"invalidate checkpoint", func() error { return other.InvalidateCheckpoint(ctx, root, entry.ID) }},
		{"remove worktree", func() error { _, e := other.Remove(ctx, root, entry.ID); return e }},
	}
	for _, test := range checks {
		t.Run(test.name, func(t *testing.T) {
			started := time.Now()
			if err := test.call(); !errors.Is(err, ErrCheckpointTestRunning) {
				t.Fatalf("active test fence bypassed: %v", err)
			}
			if time.Since(started) > 2*time.Second {
				t.Fatal("busy checkpoint lock took too long to reject")
			}
		})
	}
	unlock()
	if err := other.Release(ctx, root, entry.ID, "task-1", "conversation-1"); err != nil {
		t.Fatalf("release after verified test fence unlock: %v", err)
	}
	if _, err := other.Remove(ctx, root, entry.ID); err != nil {
		t.Fatalf("remove after verified test fence unlock: %v", err)
	}
}

func TestCheckpointExecutionFenceRejectsSymlinkFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup needs developer privilege")
	}
	manager, root, entry := checkpointFixture(t)
	_, key, err := manager.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Dir(manager.checkpointPath(key, entry.ID))
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(directory, entry.ID+".test.lock")
	if err := os.Symlink(filepath.Join(t.TempDir(), "redirected"), lock); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := manager.lockCheckpointExecution(context.Background(), root, entry.ID); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("symlink-based checkpoint lock redirect accepted: %v", err)
	}
}

func TestCheckpointFenceAcrossProcesses(t *testing.T) {
	if os.Getenv("WORKBENCH_TEST_FENCE_CHILD") == "1" {
		m, err := New(os.Getenv("WORKBENCH_TEST_FENCE_DATA"))
		if err != nil {
			t.Fatal(err)
		}
		err = m.Release(context.Background(), os.Getenv("WORKBENCH_TEST_FENCE_ROOT"), os.Getenv("WORKBENCH_TEST_FENCE_ID"), "task-1", "conversation-1")
		if !errors.Is(err, ErrCheckpointTestRunning) {
			t.Fatalf("separate process bypassed active test execution lock: %v", err)
		}
		return
	}
	manager, root, entry := checkpointFixture(t)
	unlock, err := manager.lockCheckpointExecution(context.Background(), root, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	command := exec.Command(os.Args[0], "-test.run=^TestCheckpointFenceAcrossProcesses$")
	command.Env = append(os.Environ(),
		"WORKBENCH_TEST_FENCE_CHILD=1", "WORKBENCH_TEST_FENCE_DATA="+manager.dataRoot,
		"WORKBENCH_TEST_FENCE_ROOT="+root, "WORKBENCH_TEST_FENCE_ID="+entry.ID)
	result, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("checkpoint lock did not survive process boundary: %s %v", string(result), err)
	}
}

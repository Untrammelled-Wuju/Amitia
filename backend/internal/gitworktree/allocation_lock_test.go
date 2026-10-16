package gitworktree

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAllocationHelperProcess(t *testing.T) {
	root, data := os.Getenv("UAI_TEST_ALLOC_ROOT"), os.Getenv("UAI_TEST_ALLOC_DATA")
	if root == "" || data == "" {
		return
	}
	manager, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	entry, _, err := manager.EnsureTask(context.Background(), root, "", "HEAD", "same-task", "same-conversation")
	if err != nil {
		t.Fatal(err)
	}
	println("ALLOCATION_ID=" + entry.ID)
}

func TestConcurrentAllocationAcrossProcessesSingleWorktree(t *testing.T) {
	manager, root := fixture(t)
	const workers = 5
	var wg sync.WaitGroup
	start := make(chan struct{})
	outputs := make([]string, workers)
	errorsByWorker := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestAllocationHelperProcess$", "-test.v")
			cmd.Env = append(os.Environ(), "UAI_TEST_ALLOC_ROOT="+root, "UAI_TEST_ALLOC_DATA="+manager.dataRoot)
			body, err := cmd.CombinedOutput()
			outputs[i], errorsByWorker[i] = string(body), err
		}(i)
	}
	close(start)
	wg.Wait()
	var id string
	for i := range outputs {
		if errorsByWorker[i] != nil {
			t.Fatalf("worker %d: %s: %v", i, outputs[i], errorsByWorker[i])
		}
		index := strings.Index(outputs[i], "ALLOCATION_ID=")
		if index < 0 {
			t.Fatalf("worker %d omitted allocation: %s", i, outputs[i])
		}
		candidate := strings.Fields(strings.TrimPrefix(outputs[i][index:], "ALLOCATION_ID="))[0]
		if !managedID.MatchString(candidate) {
			t.Fatalf("worker %d invalid ID %q", i, candidate)
		}
		if id == "" {
			id = candidate
		} else if id != candidate {
			t.Fatalf("duplicate worktree allocation: %s vs %s", id, candidate)
		}
	}
	entries, err := manager.List(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if entry.Managed && entry.TaskBinding != nil {
			count++
			if entry.ID != id {
				t.Fatalf("unexpected assigned worktree %s", entry.ID)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly one allocation, got %d", count)
	}
}

func TestAllocationLockRejectsSymlinkDirectory(t *testing.T) {
	manager, root := fixture(t)
	_, key, err := manager.repository(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	directory := manager.managedRoot(key) + "/.bindings"
	if err = os.MkdirAll(manager.managedRoot(key), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(t.TempDir(), directory); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err = manager.lockRepositoryAllocation(context.Background(), root)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("symlink allocation storage must fail closed: %v", err)
	}
}

package task_runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRound23HostRootRejectsSymlinkAndNonDirectory(t *testing.T) {
	root := t.TempDir()
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	actual := filepath.Join(root, "actual")
	if err := os.Mkdir(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(actual, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	svc.config.WorkspaceRoot = link
	if path, err := svc.createTaskWorkspace("safe"); err == nil {
		t.Fatalf("symlink root accepted: %s", path)
	}
	if _, err := os.Stat(filepath.Join(actual, "task-workspace-safe")); !os.IsNotExist(err) {
		t.Fatalf("wrote through symlink root: %v", err)
	}
	svc.config.WorkspaceRoot = filepath.Join(root, "file")
	if err := os.WriteFile(svc.config.WorkspaceRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if path, err := svc.createTaskWorkspace("safe"); err == nil {
		t.Fatalf("file root accepted: %s", path)
	}
}

func TestRound23HostCleanupDoesNotFollowReplacedRootOrWorkspace(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "work")
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	svc.config.WorkspaceRoot = root
	workspace, err := svc.createTaskWorkspace("first")
	if err != nil {
		t.Fatal(err)
	}

	renamed := filepath.Join(base, "original")
	if err := os.Rename(root, renamed); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outside, "task-workspace-first")
	if err := os.Mkdir(victim, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, root); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	svc.cleanupWorkspace("first", workspace)
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("cleanup followed replaced root: %v", err)
	}
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(renamed, root); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(workspace); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, workspace); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	svc.cleanupWorkspace("first", workspace)
	if _, err := os.Lstat(workspace); err != nil {
		t.Fatalf("cleanup removed an unexpected symlink: %v", err)
	}
}

func TestRound23PlacementSummaryShowsFullConnectionGeneration(t *testing.T) {
	for _, tc := range []struct {
		generation int64
		want       string
	}{{1, "gen=1"}, {10, "gen=10"}, {27, "gen=27"}, {1024, "gen=1024"}} {
		got := (TaskExecutionTarget{ConnectionGeneration: tc.generation}).Summary()
		if !strings.Contains(got, tc.want) {
			t.Fatalf("generation %d reported as %q", tc.generation, got)
		}
	}
}

func TestRound23CloudPlacementNeverFallsBackToUnrestrictedLocalHost(t *testing.T) {
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	local, err := svc.executorFor(TaskExecutionPlacementLocal)
	if err != nil || local == nil {
		t.Fatalf("existing local executor unavailable: %v", err)
	}
	cloud, err := svc.executorFor(TaskExecutionPlacementCloud)
	if _, unavailable := cloud.(UnavailableRemoteTaskExecutor); err == nil && !unavailable {
		t.Fatalf("unsupported cloud-to-cloud routed to local host: %T %v", cloud, err)
	}
}

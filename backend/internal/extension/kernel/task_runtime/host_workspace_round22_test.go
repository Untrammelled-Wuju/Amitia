package task_runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRound22TaskWorkspaceRejectsUnsafeIdentityAndExistingDirectory(t *testing.T) {
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	svc.config.WorkspaceRoot = t.TempDir()
	for _, id := range []string{"", "../outside", "../../tmp", "id/../other", "id\\other", "hello\nworld", "unicode-中文", strings.Repeat("x", 129)} {
		if path, err := svc.createTaskWorkspace(id); err == nil {
			t.Fatalf("invalid ID %q created workspace %q", id, path)
		}
	}

	for _, id := range []string{"tr-123", "source-8e5d5912-68ef-4bce-90af-a849d70486b7"} {
		path, err := svc.createTaskWorkspace(id)
		if err != nil {
			t.Fatal(err)
		}
		st, err := os.Lstat(path)
		if err != nil || !st.IsDir() || st.Mode().Perm() != 0o700 {
			t.Fatalf("task directory not private: %v %v", st, err)
		}
		svc.cleanupWorkspace(id, path)
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("workspace not removed: %v", err)
		}
	}
	target := filepath.Join(t.TempDir(), "other")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(svc.config.WorkspaceRoot, "task-workspace-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := svc.createTaskWorkspace("link"); err == nil {
		t.Fatal("existing workspace symlink was accepted")
	}
	if err := os.Mkdir(filepath.Join(svc.config.WorkspaceRoot, "task-workspace-open"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.createTaskWorkspace("open"); err == nil {
		t.Fatal("world-readable workspace was accepted")
	}
	svc.cleanupWorkspace("tr-123", target)
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("unsafe cleanup removed other directory: %v", err)
	}
}

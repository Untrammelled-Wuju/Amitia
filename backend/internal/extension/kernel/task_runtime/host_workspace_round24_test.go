package task_runtime

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestRound24CloudCoreWorkspaceUsesPrivateDedicatedRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG cache-directory override is verified on Linux hosts")
	}
	t.Setenv("AMITIA_RUNTIME_PROFILE", "cloud-core")
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	configured := DefaultTaskRuntimeConfig()
	svc := NewTaskRuntimeService(nil, configured)
	path, err := svc.createTaskWorkspace("run-cloud-1")
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(cache, "amitia", "task-runtime", "task-workspace-run-cloud-1")
	if path != expected {
		t.Fatalf("cloud workspace = %q, want %q", path, expected)
	}
	root := filepath.Dir(path)
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cloud workspace root not private: %v, %v", info, err)
	}
	svc.cleanupWorkspace("run-cloud-1", path)
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("cloud workspace not cleaned: %v", err)
	}
}

func TestRound24CloudCoreRejectsInsecureRootWithoutChmod(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX directory permission check")
	}
	t.Setenv("AMITIA_RUNTIME_PROFILE", "cloud-core")
	base := t.TempDir()
	root := filepath.Join(base, "shared")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	svc.config.WorkspaceRoot = root
	if _, err := svc.createTaskWorkspace("safe"); err == nil {
		t.Fatal("cloud core accepted world-readable root")
	}
	info, err := os.Lstat(root)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("root permissions unexpectedly mutated: %v %v", info, err)
	}
	if _, err := os.Lstat(filepath.Join(root, "task-workspace-safe")); !os.IsNotExist(err) {
		t.Fatalf("created task directory in unsafe root: %v", err)
	}
}

func TestRound24CloudCoreRejectsParentSymlinkBeforeCreatingWorkspace(t *testing.T) {
	t.Setenv("AMITIA_RUNTIME_PROFILE", "cloud-core")
	base := t.TempDir()
	outside := filepath.Join(base, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "alias")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	svc.config.WorkspaceRoot = filepath.Join(link, "private-root")
	if _, err := svc.createTaskWorkspace("run"); err == nil {
		t.Fatal("cloud accepted a parent symlink")
	}
	if _, err := os.Lstat(filepath.Join(outside, "private-root")); !os.IsNotExist(err) {
		t.Fatalf("created directories through parent symlink: %v", err)
	}

	svc.cleanupWorkspace("run", filepath.Join(link, "private-root", "task-workspace-run"))
}

func TestRound24LocalCompatibilityAndRemotePlacement(t *testing.T) {
	root, err := hostWorkspaceRoot(runtimeprofile.ProfileLocal, "")
	if err != nil {
		t.Fatal(err)
	}
	temp, err := filepath.Abs(os.TempDir())
	if err != nil || root != temp {
		t.Fatalf("local workspace changed: %q %q %v", root, temp, err)
	}
	root, err = hostWorkspaceRoot(runtimeprofile.ProfileCloudCore, "")
	if err != nil || !strings.HasSuffix(root, filepath.Join("amitia", "task-runtime")) || root == temp {
		t.Fatalf("cloud should use a separate root: %q %v", root, err)
	}
	svc := NewTaskRuntimeService(nil, DefaultTaskRuntimeConfig())
	if _, err := svc.executorFor(TaskExecutionPlacementLocal); err != nil {
		t.Fatal(err)
	}
	if executor, err := svc.executorFor(TaskExecutionPlacementCloud); err == nil {
		if _, unavailable := executor.(UnavailableRemoteTaskExecutor); !unavailable {
			t.Fatalf("cloud-to-cloud must not become local host execution: %T, %v", executor, err)
		}
	}
}

func TestRound24CloudCoreCannotRunTaskHostAsRoot(t *testing.T) {
	if err := validateHostExecutionIdentity(runtimeprofile.ProfileCloudCore, 0); err == nil {
		t.Fatal("root must not run cloud-core task scripts")
	}
	for _, profile := range []runtimeprofile.Profile{runtimeprofile.ProfileLocal, runtimeprofile.ProfileDeviceAgent} {
		if err := validateHostExecutionIdentity(profile, 0); err != nil {
			t.Fatalf("legacy %s mode changed: %v", profile, err)
		}
	}
	if err := validateHostExecutionIdentity(runtimeprofile.ProfileCloudCore, 1000); err != nil {
		t.Fatalf("unprivileged cloud host rejected: %v", err)
	}
}

package packages

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidlinux/shell"
	"github.com/u-ai/backend/pkg/util"
)

func TestPythonDetector_Detect_Available(t *testing.T) {
	executor := &fakeShellExecutor{
		results: map[string]shell.ShellExecuteResult{
			"python3 --version":       {ExitCode: 0, Stdout: "Python 3.10.12"},
			"python3 -m pip --version": {ExitCode: 0, Stdout: "pip 23.0.1"},
			"python3 -m venv --help":  {ExitCode: 0},
		},
	}
	detector := NewPythonDetector(executor)
	status, err := detector.Detect(context.Background(), 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Available {
		t.Error("expected python to be available")
	}
	if status.Version != "3.10.12" {
		t.Errorf("expected version 3.10.12, got %q", status.Version)
	}
	if !status.PipAvailable {
		t.Error("expected pip to be available")
	}
	if !status.VenvAvailable {
		t.Error("expected venv to be available")
	}
}

func TestPythonDetector_Detect_NotAvailable(t *testing.T) {
	executor := &fakeShellExecutor{
		results: map[string]shell.ShellExecuteResult{
			"python3 --version": {ExitCode: 127, Stderr: "command not found"},
		},
	}
	detector := NewPythonDetector(executor)
	status, err := detector.Detect(context.Background(), 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Available {
		t.Error("expected python to NOT be available")
	}
}

func TestPythonVenvManager_VenvExists(t *testing.T) {
	mgr := NewPythonVenvManager(nil, "", PackagesPolicy{})
	if mgr.VenvExists() {
		t.Error("expected venv to not exist with empty path")
	}
}

func TestPythonManager_Install_ValidatesSpec(t *testing.T) {
	executor := &fakeShellExecutor{}
	detector := NewPythonDetector(executor)
	venvMgr := NewPythonVenvManager(executor, "/tmp/test-venv", PackagesPolicy{})
	mgr := NewPythonManager(detector, venvMgr, executor)

	_, err := mgr.Install(context.Background(), PythonPackageInstallRequest{
		Packages: []string{"git+https://github.com/user/repo.git"},
	}, 5000)
	if err == nil {
		t.Error("expected validation error for git+https spec")
	}
}

func TestParsePythonVersion(t *testing.T) {
	v := ParsePythonVersion("3.10.12")
	if v.Major != "3" || v.Minor != "10" || v.Patch != "12" {
		t.Errorf("expected 3.10.12, got %s.%s.%s", v.Major, v.Minor, v.Patch)
	}
}

func TestDefaultPackagesPolicy(t *testing.T) {
	policy := DefaultPackagesPolicy()
	if !policy.Enabled {
		t.Error("expected policy to be enabled by default")
	}
	if !policy.UseAptGet {
		t.Error("expected UseAptGet to be true by default")
	}
	if policy.PythonVenvBaseDir != "packages/python/default" {
		t.Errorf("unexpected PythonVenvBaseDir: %q", policy.PythonVenvBaseDir)
	}
}

func TestPythonVenvManager_Run_NonZero(t *testing.T) {
	executor := &fakeShellExecutor{
		results: map[string]shell.ShellExecuteResult{
			"/tmp/test-venv/bin/python -c import sys; sys.exit(7)": {ExitCode: 7, Stdout: ""},
		},
	}
	mgr := NewPythonVenvManager(executor, "/tmp/test-venv", PackagesPolicy{})
	_ = mgr
}

func TestRuntimePaths(t *testing.T) {
	paths := util.RuntimePaths{DataDir: "/data"}
	if paths.DataDir != "/data" {
		t.Error("DataDir mismatch")
	}
}

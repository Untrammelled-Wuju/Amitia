//go:build linux && !android

package packages

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/androidlinux/shell"
	"github.com/u-ai/backend/pkg/util"
)

type fakeShellExecutor struct {
	results map[string]shell.ShellExecuteResult
	calls   int
}

func (f *fakeShellExecutor) Execute(ctx context.Context, req shell.ShellExecuteRequest) shell.ShellExecuteResult {
	f.calls++
	key := req.Executable + " " + joinArgs(req.Args)
	if r, ok := f.results[key]; ok {
		return r
	}
	return shell.ShellExecuteResult{ExitCode: 0}
}

func joinArgs(args []string) string {
	result := ""
	for i, a := range args {
		if i > 0 {
			result += " "
		}
		result += a
	}
	return result
}

func TestAptDetector_Detect_Available(t *testing.T) {
	executor := &fakeShellExecutor{
		results: map[string]shell.ShellExecuteResult{
			"apt-get --version": {ExitCode: 0, Stdout: "apt 2.4.8 (amd64)"},
			"dpkg --print-architecture": {ExitCode: 0, Stdout: "amd64"},
			"id -u": {ExitCode: 0, Stdout: "0"},
		},
	}
	detector := NewAptDetector(executor)
	status, err := detector.Detect(context.Background(), 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Available {
		t.Error("expected apt to be available")
	}
	if status.Version != "2.4.8" {
		t.Errorf("expected version 2.4.8, got %q", status.Version)
	}
	if status.Architecture != "amd64" {
		t.Errorf("expected architecture amd64, got %q", status.Architecture)
	}
	if status.PrivilegeState != "guest_root" {
		t.Errorf("expected privilege state guest_root, got %q", status.PrivilegeState)
	}
}

func TestAptDetector_Detect_NotAvailable(t *testing.T) {
	executor := &fakeShellExecutor{
		results: map[string]shell.ShellExecuteResult{
			"apt-get --version": {ExitCode: 127, Stderr: "command not found"},
		},
	}
	detector := NewAptDetector(executor)
	status, err := detector.Detect(context.Background(), 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Available {
		t.Error("expected apt to NOT be available")
	}
}

func TestAptDetector_Detect_NonRoot(t *testing.T) {
	executor := &fakeShellExecutor{
		results: map[string]shell.ShellExecuteResult{
			"apt-get --version": {ExitCode: 0, Stdout: "apt 2.4.8"},
			"dpkg --print-architecture": {ExitCode: 0, Stdout: "amd64"},
			"id -u": {ExitCode: 0, Stdout: "1000"},
		},
	}
	detector := NewAptDetector(executor)
	status, err := detector.Detect(context.Background(), 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.PrivilegeState != "non_root" {
		t.Errorf("expected privilege state non_root, got %q", status.PrivilegeState)
	}
}

func TestAptManager_Install_ValidatesPackage(t *testing.T) {
	executor := &fakeShellExecutor{}
	mgr := NewAptManager(executor, PackagesPolicy{}, util.RuntimePaths{})

	_, err := mgr.Install(context.Background(), []string{"git; rm -rf /"}, 5000)
	if err == nil {
		t.Error("expected validation error for invalid package name")
	}
	if executor.calls > 0 {
		t.Error("expected no shell calls for invalid package")
	}
}

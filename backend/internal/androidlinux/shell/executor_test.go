//go:build linux && !android

package shell

import (
	"context"
	"testing"
)

func TestShellExecutorImpl_ValidateRequest_DefaultMode(t *testing.T) {
	executor := &ShellExecutorImpl{}

	req := ShellExecuteRequest{
		Executable: "/bin/echo",
	}
	err := executor.validateRequest(&req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Mode != ShellModeArgv {
		t.Errorf("expected default mode to be argv, got %s", req.Mode)
	}
}

func TestShellExecutorImpl_ValidateRequest_ArgvRequiresExecutable(t *testing.T) {
	executor := &ShellExecutorImpl{}
	req := ShellExecuteRequest{
		Mode: ShellModeArgv,
	}
	err := executor.validateRequest(&req)
	if err == nil {
		t.Fatalf("expected error for missing executable in argv mode")
	}
}

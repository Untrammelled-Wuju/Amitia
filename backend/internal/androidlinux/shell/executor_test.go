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
		t.Error("expected error when executable is missing in argv mode")
	}
}

func TestShellExecutorImpl_ValidateRequest_ShellRequiresCommand(t *testing.T) {
	executor := &ShellExecutorImpl{}

	req := ShellExecuteRequest{
		Mode: ShellModeShell,
	}
	err := executor.validateRequest(&req)
	if err == nil {
		t.Error("expected error when command is missing in shell mode")
	}
}

func TestShellExecutorImpl_ValidateRequest_InvalidMode(t *testing.T) {
	executor := &ShellExecutorImpl{}

	req := ShellExecuteRequest{
		Mode: ShellMode("invalid"),
	}
	err := executor.validateRequest(&req)
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestShellExecutorImpl_BuildArgv_ArgvMode(t *testing.T) {
	executor := &ShellExecutorImpl{}

	req := ShellExecuteRequest{
		Mode:       ShellModeArgv,
		Executable: "/bin/echo",
		Args:       []string{"hello", "world"},
	}
	argv := executor.buildArgv(req)
	if len(argv) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(argv))
	}
	if argv[0] != "/bin/echo" {
		t.Errorf("expected /bin/echo, got %s", argv[0])
	}
}

func TestShellExecutorImpl_BuildArgv_ShellMode(t *testing.T) {
	executor := &ShellExecutorImpl{}

	req := ShellExecuteRequest{
		Mode:    ShellModeShell,
		Command: "echo hello",
	}
	argv := executor.buildArgv(req)
	if len(argv) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(argv))
	}
	if argv[0] != "/bin/sh" {
		t.Errorf("expected /bin/sh, got %s", argv[0])
	}
	if argv[2] != "echo hello" {
		t.Errorf("expected 'echo hello', got %s", argv[2])
	}
}

func TestShellExecutorImpl_CalculateTimeout(t *testing.T) {
	executor := &ShellExecutorImpl{
		policy: DefaultShellPolicy(),
	}

	tests := []struct {
		input    int64
		expected int64
	}{
		{0, 30000},
		{10000, 10000},
		{600000, 300000},
	}
	for _, tt := range tests {
		result := executor.calculateTimeout(tt.input)
		if result != tt.expected {
			t.Errorf("calculateTimeout(%d) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestShellExecutorImpl_Execute_Disabled(t *testing.T) {
	policy := DefaultShellPolicy()
	policy.Enabled = false
	executor := NewShellExecutor(policy, "/workspace", "/tmp")

	result := executor.Execute(context.Background(), ShellExecuteRequest{
		Mode:    ShellModeShell,
		Command: "echo test",
	})
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code when shell is disabled")
	}
}

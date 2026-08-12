package root

import "testing"

func TestPolicy_ValidateExecute_EmptyExecutable(t *testing.T) {
	p := DefaultPolicy()
	err := p.ValidateExecute(&ExecuteRequest{Executable: ""})
	if err == nil || err.Code != ROOT_INVALID_ARGUMENT {
		t.Fatalf("expected ROOT_INVALID_ARGUMENT, got %v", err)
	}
}

func TestPolicy_ValidateExecute_ShellExecutableNoMode(t *testing.T) {
	p := DefaultPolicy()
	err := p.ValidateExecute(&ExecuteRequest{Executable: "sh"})
	if err == nil || err.Code != ROOT_COMMAND_NOT_ALLOWED {
		t.Fatalf("expected ROOT_COMMAND_NOT_ALLOWED, got %v", err)
	}
}

func TestPolicy_ValidateExecute_ShellExecutableWithMode(t *testing.T) {
	p := DefaultPolicy()
	err := p.ValidateExecute(&ExecuteRequest{Executable: "bash", Mode: "shell"})
	if err != nil {
		t.Fatalf("expected no error for shell mode, got %v", err)
	}
}

func TestPolicy_ValidateExecute_TooManyArgs(t *testing.T) {
	p := DefaultPolicy()
	args := make([]string, MaxArgCount+1)
	err := p.ValidateExecute(&ExecuteRequest{Executable: "id", Args: args})
	if err == nil || err.Code != ROOT_INVALID_ARGUMENT {
		t.Fatalf("expected ROOT_INVALID_ARGUMENT, got %v", err)
	}
}

func TestPolicy_ValidateExecute_ArgTooLong(t *testing.T) {
	p := DefaultPolicy()
	longArg := make([]byte, MaxArgBytes+1)
	err := p.ValidateExecute(&ExecuteRequest{Executable: "echo", Args: []string{string(longArg)}})
	if err == nil || err.Code != ROOT_INVALID_ARGUMENT {
		t.Fatalf("expected ROOT_INVALID_ARGUMENT, got %v", err)
	}
}

func TestPolicy_ValidateExecute_StdinTooLarge(t *testing.T) {
	p := DefaultPolicy()
	largeStdin := make([]byte, MaxInputBytes+1)
	err := p.ValidateExecute(&ExecuteRequest{Executable: "cat", Stdin: string(largeStdin)})
	if err == nil || err.Code != ROOT_INPUT_TOO_LARGE {
		t.Fatalf("expected ROOT_INPUT_TOO_LARGE, got %v", err)
	}
}

func TestPolicy_ValidateExecute_TooManyEnvEntries(t *testing.T) {
	p := DefaultPolicy()
	env := make(map[string]string)
	for i := 0; i < MaxEnvironmentEntries+1; i++ {
		env[string(rune('a'+i%26))+string(rune('0'+i/26))] = "value"
	}
	err := p.ValidateExecute(&ExecuteRequest{Executable: "env", Env: env})
	if err == nil || err.Code != ROOT_INVALID_ARGUMENT {
		t.Fatalf("expected ROOT_INVALID_ARGUMENT, got %v", err)
	}
}

func TestPolicy_ValidateExecute_InvalidMode(t *testing.T) {
	p := DefaultPolicy()
	err := p.ValidateExecute(&ExecuteRequest{Executable: "id", Mode: "invalid"})
	if err == nil || err.Code != ROOT_INVALID_ARGUMENT {
		t.Fatalf("expected ROOT_INVALID_ARGUMENT, got %v", err)
	}
}

func TestPolicy_ValidateExecute_ValidStructured(t *testing.T) {
	p := DefaultPolicy()
	err := p.ValidateExecute(&ExecuteRequest{
		Executable: "id",
		Args:       []string{},
		Mode:       "structured",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestPolicy_ValidateTimeout_Default(t *testing.T) {
	p := DefaultPolicy()
	timeout := p.ValidateTimeout(0)
	if timeout != DefaultTimeoutMS {
		t.Fatalf("expected default timeout %d, got %d", DefaultTimeoutMS, timeout)
	}
}

func TestPolicy_ValidateTimeout_BelowMin(t *testing.T) {
	p := DefaultPolicy()
	timeout := p.ValidateTimeout(50)
	if timeout != MinTimeoutMS {
		t.Fatalf("expected min timeout %d, got %d", MinTimeoutMS, timeout)
	}
}

func TestPolicy_ValidateTimeout_AboveMax(t *testing.T) {
	p := DefaultPolicy()
	timeout := p.ValidateTimeout(120000)
	if timeout != HardTimeoutMS {
		t.Fatalf("expected hard timeout %d, got %d", HardTimeoutMS, timeout)
	}
}

func TestPolicy_ValidateTimeout_Valid(t *testing.T) {
	p := DefaultPolicy()
	timeout := p.ValidateTimeout(5000)
	if timeout != 5000 {
		t.Fatalf("expected 5000, got %d", timeout)
	}
}

func TestValidateWorkDir_Empty(t *testing.T) {
	err := ValidateWorkDir("")
	if err != nil {
		t.Fatalf("expected no error for empty workDir, got %v", err)
	}
}

func TestValidateWorkDir_Absolute(t *testing.T) {
	err := ValidateWorkDir("/data/local/tmp")
	if err != nil {
		t.Fatalf("expected no error for absolute path, got %v", err)
	}
}

func TestValidateWorkDir_Relative(t *testing.T) {
	err := ValidateWorkDir("relative/path")
	if err == nil {
		t.Fatal("expected error for relative path, got nil")
	}
}

func TestIsShellExecutable(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"sh", "sh", true},
		{"bash", "bash", true},
		{"zsh", "zsh", true},
		{"cmd", "cmd", true},
		{"powershell", "powershell", true},
		{"SH", "SH", true},
		{"ls", "ls", false},
		{"id", "id", false},
		{"getprop", "getprop", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isShellExecutable(tt.input)
			if result != tt.expected {
				t.Fatalf("isShellExecutable(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

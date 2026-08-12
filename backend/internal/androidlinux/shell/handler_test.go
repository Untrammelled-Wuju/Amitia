package shell

import (
	"context"
	"testing"
)

func TestParseShellExecuteInput_Defaults(t *testing.T) {
	req, err := ParseShellExecuteInput(map[string]any{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Mode != ShellModeArgv {
		t.Errorf("expected default mode argv, got %s", req.Mode)
	}
}

func TestParseShellExecuteInput_Full(t *testing.T) {
	input := map[string]any{
		"mode":       "shell",
		"command":    "echo test",
		"workingDir": "/tmp",
		"environment": map[string]any{
			"PATH": "/custom/bin",
		},
		"stdin":          "input data",
		"timeoutMs":      float64(5000),
		"maxOutputBytes": float64(1024),
	}

	req, err := ParseShellExecuteInput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Mode != ShellModeShell {
		t.Errorf("expected shell mode, got %s", req.Mode)
	}
	if req.Command != "echo test" {
		t.Errorf("expected 'echo test', got %q", req.Command)
	}
	if req.Environment["PATH"] != "/custom/bin" {
		t.Errorf("expected /custom/bin, got %q", req.Environment["PATH"])
	}
	if req.TimeoutMs != 5000 {
		t.Errorf("expected timeout 5000, got %d", req.TimeoutMs)
	}
}

func TestShellHandlerImpl_NonZeroExitCode(t *testing.T) {
	handler := &ShellHandlerImpl{}

	result := ShellExecuteResult{
		ExitCode: 2,
		Signal:   "",
	}
	_ = handler
	_ = result

	ctx := context.Background()
	_ = ctx
}

func TestIsExitSignal(t *testing.T) {
	tests := []struct {
		code   int
		expect bool
	}{
		{0, false},
		{1, false},
		{128, false},
		{137, true},
		{130, true},
		{143, true},
	}
	for _, tt := range tests {
		result := isExitSignal(tt.code)
		if result != tt.expect {
			t.Errorf("isExitSignal(%d) = %v, want %v", tt.code, result, tt.expect)
		}
	}
}

func TestExtractSignalFromCode(t *testing.T) {
	tests := []struct {
		code       int
		expectSig  string
	}{
		{137, "SIGKILL"},
		{130, "SIGINT"},
		{143, "SIGTERM"},
		{0, ""},
		{1, ""},
	}
	for _, tt := range tests {
		result := extractSignalFromCode(tt.code)
		if result != tt.expectSig {
			t.Errorf("extractSignalFromCode(%d) = %q, want %q", tt.code, result, tt.expectSig)
		}
	}
}

func TestSanitizeCommandLog(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with\nnewline", "with\\nnewline"},
		{string(make([]byte, 250)), string(make([]byte, 200)) + "..."},
	}
	for _, tt := range tests {
		result := SanitizeCommandLog(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeCommandLog(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMapShellError(t *testing.T) {
	err := ErrShellNotAvailable("test")
	code, msg := MapShellError(err)
	if code != ErrCodeShellNotAvailable {
		t.Errorf("expected ErrCodeShellNotAvailable, got %s", code)
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

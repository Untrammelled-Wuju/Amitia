//go:build linux && !android

package shell

import (
	"testing"
)

func TestEnvironmentBuilder_Build_Defaults(t *testing.T) {
	policy := DefaultShellPolicy()
	builder := NewEnvironmentBuilder(policy)

	env, envSlice, err := builder.Build(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["PATH"] == "" {
		t.Error("expected PATH to be set")
	}
	if env["HOME"] != "/root" {
		t.Errorf("expected HOME=/root, got %s", env["HOME"])
	}
	if env["TMPDIR"] != "/tmp" {
		t.Errorf("expected TMPDIR=/tmp, got %s", env["TMPDIR"])
	}

	if len(envSlice) < 4 {
		t.Errorf("expected at least 4 env vars in slice, got %d", len(envSlice))
	}
}

func TestEnvironmentBuilder_Build_AllowedKeys(t *testing.T) {
	policy := DefaultShellPolicy()
	builder := NewEnvironmentBuilder(policy)

	env, _, err := builder.Build(map[string]string{
		"PATH":   "/custom/bin:/usr/bin",
		"HOME":   "/home/user",
		"LC_ALL": "en_US.UTF-8",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env["PATH"] != "/custom/bin:/usr/bin" {
		t.Errorf("expected custom PATH, got %s", env["PATH"])
	}
	if env["HOME"] != "/home/user" {
		t.Errorf("expected custom HOME, got %s", env["HOME"])
	}
	if env["LC_ALL"] != "en_US.UTF-8" {
		t.Errorf("expected LC_ALL=en_US.UTF-8, got %s", env["LC_ALL"])
	}
}

func TestEnvironmentBuilder_Build_DeniedKey(t *testing.T) {
	policy := DefaultShellPolicy()
	builder := NewEnvironmentBuilder(policy)

	_, _, err := builder.Build(map[string]string{
		"SECRET_KEY": "should-be-denied",
	})
	if err == nil {
		t.Error("expected error for denied environment key")
	}

	shellErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected Error type, got %T", err)
	}
	if shellErr.Code() != ErrCodeEnvironmentDenied {
		t.Errorf("expected ErrCodeEnvironmentDenied, got %s", shellErr.Code())
	}
}

func TestEnvironmentBuilder_Build_TooManyEntries(t *testing.T) {
	policy := DefaultShellPolicy()
	policy.MaxEnvironmentEntries = 2
	builder := NewEnvironmentBuilder(policy)

	_, _, err := builder.Build(map[string]string{
		"PATH": "/bin",
		"HOME": "/root",
		"TERM": "xterm",
	})
	if err == nil {
		t.Error("expected error for too many environment entries")
	}
}

func TestEnvironmentBuilder_Build_InvalidKeyFormat(t *testing.T) {
	policy := DefaultShellPolicy()
	builder := NewEnvironmentBuilder(policy)

	_, _, err := builder.Build(map[string]string{
		"123-invalid": "value",
	})
	if err == nil {
		t.Error("expected error for invalid environment key format")
	}
}

func TestSanitizeEnvValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"with\nnewline", "withnewline"},
		{"with\rcarriage", "withcarriage"},
		{"with\x00null", "withnull"},
	}
	for _, tt := range tests {
		result := SanitizeEnvValue(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizeEnvValue(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

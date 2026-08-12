package adb

import (
	"testing"
)

func TestCommandPolicy_Validate(t *testing.T) {
	policy := NewCommandPolicy()

	tests := []struct {
		name       string
		executable string
		args       []string
		shouldErr  bool
	}{
		{"getprop no args", "getprop", nil, false},
		{"getprop with valid arg", "getprop", []string{"ro.build.version.sdk"}, false},
		{"getprop too many args", "getprop", []string{"arg1", "arg2"}, true},
		{"getprop invalid arg", "getprop", []string{"invalid name!"}, true},
		{"id no args", "id", nil, false},
		{"id with args", "id", []string{"-u"}, true},
		{"uname -a", "uname", []string{"-a"}, false},
		{"uname -z", "uname", []string{"-z"}, true},
		{"unknown command", "ls", nil, true},
		{"shell not allowed", "sh", nil, true},
		{"dash not allowed", "dash", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := policy.Validate(tt.executable, tt.args)
			if (err != nil) != tt.shouldErr {
				t.Errorf("expected shouldErr=%v, got err=%v", tt.shouldErr, err)
			}
		})
	}
}

func TestCommandPolicy_IsAllowed(t *testing.T) {
	policy := NewCommandPolicy()
	allowed := []string{"getprop", "id", "uname"}
	for _, cmd := range allowed {
		if !policy.IsAllowed(cmd) {
			t.Errorf("expected %s to be allowed", cmd)
		}
	}
	notAllowed := []string{"sh", "ls", "cat", "rm", "chmod", "su", "toybox"}
	for _, cmd := range notAllowed {
		if policy.IsAllowed(cmd) {
			t.Errorf("expected %s to not be allowed", cmd)
		}
	}
}

func TestIsValidPropertyName(t *testing.T) {
	tests := map[string]bool{
		"ro.build.version.sdk": true,
		"ro.product.model":     true,
		"":                     false,
		"invalid name":         false,
		"invalid!":             false,
	}
	for name, expected := range tests {
		actual := isValidPropertyName(name)
		if actual != expected {
			t.Errorf("isValidPropertyName(%q) = %v, expected %v", name, actual, expected)
		}
	}
}

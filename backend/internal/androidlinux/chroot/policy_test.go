//go:build linux && !android

package chroot

import "testing"

func TestDefaultPolicyValues(t *testing.T) {
	p := DefaultPolicy()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Enabled", p.Enabled, true},
		{"MaxFSBytes", p.MaxFSBytes, int64(10 * 1024 * 1024 * 1024)},
		{"MaxEnvironments", p.MaxEnvironments, 4},
		{"RequireBinSH", p.RequireBinSH, true},
		{"AllowProotExec", p.AllowProotExec, true},
		{"MaxOutputBytes", p.MaxOutputBytes, int64(100 * 1024 * 1024)},
		{"DefaultTimeoutSec", p.DefaultTimeoutSec, 30},
		{"MaxTimeoutSec", p.MaxTimeoutSec, 600},
		{"MaxStdinBytes", p.MaxStdinBytes, int64(10 * 1024 * 1024)},
	}

	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
		}
	}
}

func TestDefaultExecBackend(t *testing.T) {
	p := DefaultPolicy()
	if p.DefaultExecBackend != "proot" {
		t.Errorf("DefaultExecBackend = %s, want proot", p.DefaultExecBackend)
	}
}

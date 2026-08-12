//go:build linux && !android

package ssh

import "testing"

func TestDefaultPolicyValues(t *testing.T) {
	p := DefaultPolicy()

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Enabled", p.Enabled, true},
		{"MaxSessionIdleSecond", p.MaxSessionIdleSecond, 120},
		{"MaxSessions", p.MaxSessions, 10},
		{"MaxOutputBytes", p.MaxOutputBytes, int64(2 * 1024 * 1024)},
		{"DefaultTimeoutSecond", p.DefaultTimeoutSecond, 30},
		{"MaxTimeoutSecond", p.MaxTimeoutSecond, 600},
		{"MaxStdinBytes", p.MaxStdinBytes, int64(1 * 1024 * 1024)},
		{"EnableAgentAuth", p.EnableAgentAuth, false},
		{"MaxSessionCount", p.MaxSessionCount, 10},
	}

	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("%s = %v, want %v", tt.name, tt.got, tt.expected)
		}
	}
}

func TestDefaultHostKeyPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.DefaultHostKeyPolicy != HostKeyPolicyReject {
		t.Errorf("DefaultHostKeyPolicy = %v, want %v", p.DefaultHostKeyPolicy, HostKeyPolicyReject)
	}
}

func TestDeniedPortList(t *testing.T) {
	p := DefaultPolicy()
	if len(p.DeniedPortList) == 0 {
		t.Error("DeniedPortList should not be empty")
	}
}

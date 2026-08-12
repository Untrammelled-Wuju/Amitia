//go:build linux && !android

package ssh

import (
	"testing"
)

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	if !p.Enabled {
		t.Error("Enabled = false, want true")
	}
	if p.DefaultTimeoutSecond != 30 {
		t.Errorf("DefaultTimeoutSecond = %d, want 30", p.DefaultTimeoutSecond)
	}
	if p.MaxTimeoutSecond != 600 {
		t.Errorf("MaxTimeoutSecond = %d, want 600", p.MaxTimeoutSecond)
	}
	if p.MaxSessions != 10 {
		t.Errorf("MaxSessions = %d, want 10", p.MaxSessions)
	}
	if p.DefaultHostKeyPolicy != HostKeyPolicyReject {
		t.Errorf("DefaultHostKeyPolicy = %v, want reject", p.DefaultHostKeyPolicy)
	}
}

func TestPolicyIsPortAllowed(t *testing.T) {
	p := DefaultPolicy()

	if !p.IsPortAllowed(22, "public") {
		t.Error("IsPortAllowed(22) = false, want true")
	}
	if !p.IsPortAllowed(2222, "public") {
		t.Error("IsPortAllowed(2222) = false, want true")
	}
	if p.IsPortAllowed(0, "public") {
		t.Error("IsPortAllowed(0) = true, want false")
	}
}

func TestIsDeniedTarget(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"8.8.8.8", false},
		{"example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			result := isDeniedTarget(tt.host)
			if result != tt.expected {
				t.Errorf("isDeniedTarget(%q) = %v, want %v", tt.host, result, tt.expected)
			}
		})
	}
}

func TestComputeFingerprint(t *testing.T) {
	data := []byte("test-public-key-data")
	fp := ComputeFingerprint(data)

	if fp == "" {
		t.Error("ComputeFingerprint returned empty string")
	}
	if len(fp) < 8 {
		t.Errorf("ComputeFingerprint returned too short: %s", fp)
	}
}

func TestHostKeyStoreMemory(t *testing.T) {
	store, err := NewHostKeyStore("", false)
	if err != nil {
		t.Fatalf("NewHostKeyStore() error = %v", err)
	}

	host := KnownHost{
		Host:        "example.com",
		Port:        22,
		Algorithm:   "ssh-ed25519",
		Fingerprint: "SHA256:abcdef1234567890",
	}

	if err := store.Put(host); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, found := store.Get("example.com", 22)
	if !found {
		t.Fatal("Get() not found after Put()")
	}
	if got.Fingerprint != host.Fingerprint {
		t.Errorf("Get() fingerprint = %v, want %v", got.Fingerprint, host.Fingerprint)
	}

	if store.Count() != 1 {
		t.Errorf("Count() = %d, want 1", store.Count())
	}

	if err := store.Delete("example.com", 22); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, found = store.Get("example.com", 22)
	if found {
		t.Error("Get() found after Delete()")
	}
}

func TestClassifyHost(t *testing.T) {
	tests := []struct {
		host     string
		expected string
	}{
		{"127.0.0.1", "loopback"},
		{"::1", "loopback"},
		{"192.168.1.1", "private"},
		{"10.0.0.1", "private"},
		{"8.8.8.8", "public"},
		{"localhost", "loopback"},
		{"example.com", "public"},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			result := classifyHost(tt.host)
			if result != tt.expected {
				t.Errorf("classifyHost(%q) = %v, want %v", tt.host, result, tt.expected)
			}
		})
	}
}

func TestServiceValidation(t *testing.T) {
	policy := DefaultPolicy()
	store, _ := NewHostKeyStore("", false)
	svc := NewService(policy, store)

	t.Run("exec without host", func(t *testing.T) {
		_, err := svc.Exec(nil, SSHExecRequest{
			Command: "ls",
		})
		if err == nil {
			t.Error("Exec() expected error without host")
		}
	})

	t.Run("exec without command", func(t *testing.T) {
		_, err := svc.Exec(nil, SSHExecRequest{
			Host: "example.com",
		})
		if err == nil {
			t.Error("Exec() expected error without command")
		}
	})

	t.Run("exec to denied host", func(t *testing.T) {
		_, err := svc.Exec(nil, SSHExecRequest{
			Host:    "localhost",
			Command: "ls",
		})
		if err == nil {
			t.Error("Exec() expected error for denied host")
		}
	})

	t.Run("exec without auth", func(t *testing.T) {
		_, err := svc.Exec(nil, SSHExecRequest{
			Host:    "8.8.8.8",
			Command: "ls",
		})
		if err == nil {
			t.Error("Exec() expected error without auth")
		}
	})
}

func TestHostKeyCallbackReject(t *testing.T) {
	policy := DefaultPolicy()
	store, _ := NewHostKeyStore("", false)
	svc := NewService(policy, store)

	callback := svc.hostKeyCallback(HostKeyPolicyReject, "", "unknown.com", 22)

	err := callback("unknown.com", nil, nil)
	if err == nil {
		t.Error("hostKeyCallback expected error for unknown host")
	}
}

func TestHostKeyCallbackAcceptNew(t *testing.T) {
	policy := DefaultPolicy()
	store, _ := NewHostKeyStore("", false)
	svc := NewService(policy, store)

	callback := svc.hostKeyCallback(HostKeyPolicyAcceptNew, "", "newhost.com", 22)

	err := callback("newhost.com", nil, nil)
	if err != nil {
		t.Errorf("hostKeyCallback accept_new should not error: %v", err)
	}
}

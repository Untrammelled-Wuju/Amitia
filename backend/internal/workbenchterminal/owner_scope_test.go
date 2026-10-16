package workbenchterminal

import (
	"errors"
	"testing"
)

func TestTerminalOwnerKeySeparatesAmbiguousIdentityComponents(t *testing.T) {
	cases := [][5]string{
		{"space", "device", "runtime", "session:part", "conversation"},
		{"space", "device", "runtime", "session", "part:conversation"},
		{"space", "device", "runtime:session", "part", "conversation"},
		{"space", "device:runtime", "session", "part", "conversation"},
		{"space:device", "runtime", "session", "part", "conversation"},
		{"space", "device", "runtime", "", "session:part:conversation"},
	}
	seen := make(map[string]bool)
	for _, fields := range cases {
		key := terminalOwnerKey(fields[0], fields[1], fields[2], fields[3], fields[4])
		if seen[key] {
			t.Fatalf("two identities produced the same owner: %v", fields)
		}
		seen[key] = true
		if key != terminalOwnerKey(fields[0], fields[1], fields[2], fields[3], fields[4]) {
			t.Fatalf("owner key is not stable: %v", fields)
		}
	}
}

func TestTerminalOwnerKeyIsolatesEachDeviceAndConversation(t *testing.T) {
	m, _, id := sessionFixture(t)
	original := terminalOwnerKey("space", "device-A", "runtime-1", "session", "conversation")
	m.mu.Lock()
	m.sessions[id].owner = original
	m.mu.Unlock()
	foreign := []string{
		terminalOwnerKey("another-space", "device-A", "runtime-1", "session", "conversation"),
		terminalOwnerKey("space", "device-B", "runtime-1", "session", "conversation"),
		terminalOwnerKey("space", "device-A", "runtime-2", "session", "conversation"),
		terminalOwnerKey("space", "device-A", "runtime-1", "another-session", "conversation"),
		terminalOwnerKey("space", "device-A", "runtime-1", "session", "another-conversation"),
	}
	for _, other := range foreign {
		if _, err := m.Read(other, id, 0); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign read granted: %v", err)
		}
		if err := m.Input(other, id, []byte("x")); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign input granted: %v", err)
		}
		if err := m.CloseSession(other, id); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign close granted: %v", err)
		}
		if got := m.List(other); len(got) != 0 {
			t.Fatalf("foreign list returned %d sessions", len(got))
		}
	}
	if _, err := m.Read(original, id, 0); err != nil {
		t.Fatalf("owner cannot read its own session: %v", err)
	}
}

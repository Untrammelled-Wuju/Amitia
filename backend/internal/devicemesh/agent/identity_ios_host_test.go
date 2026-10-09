package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIOSIdentityCannotFallBackToCopiedGuestIdentity(t *testing.T) {
	dir := t.TempDir()
	store := NewIdentityStore(dir)
	identity, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "device-mesh", "identity.json")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AMITIA_IOS_HOST_BRIDGE_REQUIRED", "true")
	for _, key := range []string{"READ_FD", "WRITE_FD", "GENERATION"} {
		t.Setenv("AMITIA_IOS_HOST_BRIDGE_"+key, "")
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("cached copied identity was accepted")
	}
	if _, err := NewIdentityStore(dir).Load(); err == nil {
		t.Fatal("copied identity was accepted")
	}
	if _, err := store.Sign([]byte("proof")); err == nil {
		t.Fatal("guest private key signed without host authority")
	}
	after, err := os.ReadFile(file)
	if err != nil || string(before) != string(after) || identity.DeviceID == "" {
		t.Fatal("guest identity altered while host absent")
	}
}

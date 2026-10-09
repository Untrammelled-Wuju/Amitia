package secret

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIOSVaultRefusesGuestMasterKeyWithoutHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	keyPath := filepath.Join(dir, "vault.key")
	if _, err := NewEncryptedFileStore(path, keyPath); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AMITIA_RUNTIME_MODE", "ios-ish")
	for _, key := range []string{"READ_FD", "WRITE_FD", "GENERATION"} {
		t.Setenv("AMITIA_IOS_HOST_BRIDGE_"+key, "")
	}
	if _, err := NewEncryptedFileStore(path, keyPath); err == nil {
		t.Fatal("guest master key reused without host")
	}
	after, err := os.ReadFile(keyPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("master key changed without host authority")
	}
}

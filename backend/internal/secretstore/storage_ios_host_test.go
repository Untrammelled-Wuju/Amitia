package secretstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIOSSecretsCannotReadOrOverwriteGuestCopiesWithoutHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "copied-credential")
	if err := Write(path, []byte("original-proof")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AMITIA_RUNTIME_MODE", "ios-ish")
	t.Setenv("AMITIA_DATA_DIR", dir)
	for _, key := range []string{"READ_FD", "WRITE_FD", "GENERATION"} {
		t.Setenv("AMITIA_IOS_HOST_BRIDGE_"+key, "")
	}
	if _, err := Read(path); err == nil {
		t.Fatal("copied credential was read without host")
	}
	if err := Write(path, []byte("replacement")); err == nil {
		t.Fatal("credential was written without host")
	}
	if err := Delete(path); err == nil {
		t.Fatal("credential was deleted without host acknowledgement")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("guest secret changed without host")
	}
}

func TestIOSSecretKeyRejectsOutsideDataDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AMITIA_DATA_DIR", filepath.Join(dir, "data"))
	if _, err := hostKey(filepath.Join(dir, "outside")); err == nil {
		t.Fatal("outside path accepted")
	}
	key, err := hostKey(filepath.Join(dir, "data", "device-mesh", "credential.json"))
	if err != nil || len(key) != 64 {
		t.Fatalf("invalid stable host key: %v", err)
	}
	keyAgain, err := hostKey(filepath.Join(dir, "data", "device-mesh", ".", "credential.json"))
	if err != nil || keyAgain != key {
		t.Fatal("non-canonical key")
	}
}

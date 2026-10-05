package secretstore

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestProtectedSecretRoundTripAndTamperRejection(t *testing.T) {
	t.Setenv("AMITIA_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)))
	path := filepath.Join(t.TempDir(), "credential.json")
	secret := []byte(`{"credential":"private-value"}`)
	if err := Write(path, secret); err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(path)
	if err != nil || bytes.Contains(encoded, []byte("private-value")) || !bytes.HasPrefix(encoded, envelope) {
		t.Fatal("secret written without protection")
	}
	decoded, err := Read(path)
	if err != nil || !bytes.Equal(secret, decoded) {
		t.Fatal("secret round trip failed")
	}
	encoded[len(encoded)-1] ^= 1
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("tampered secret accepted")
	}
}

func TestLegacySecretUpgradesWithoutChangingCredential(t *testing.T) {
	t.Setenv("AMITIA_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{4}, 32)))
	path := filepath.Join(t.TempDir(), "credential.json")
	secret := []byte(`{"credential":"legacy"}`)
	if err := os.WriteFile(path, secret, 0600); err != nil {
		t.Fatal(err)
	}
	decoded, err := Read(path)
	if err != nil || !bytes.Equal(secret, decoded) {
		t.Fatal("legacy credential changed")
	}
	encoded, err := os.ReadFile(path)
	if err != nil || !bytes.HasPrefix(encoded, envelope) {
		t.Fatal("legacy plaintext was retained")
	}
}

func TestCopiedPortableSecretRequiresOriginalKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	t.Setenv("AMITIA_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{5}, 32)))
	path := filepath.Join(t.TempDir(), "key")
	if err := Write(path, []byte("device-private-key")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AMITIA_SECRET_KEY", base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{6}, 32)))
	if _, err := Read(path); err == nil {
		t.Fatal("copied identity decrypted with another device key")
	}
}

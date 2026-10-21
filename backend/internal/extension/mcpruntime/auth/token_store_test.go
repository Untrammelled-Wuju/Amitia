package auth

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptedFileStoreRoundTripAndNoPlaintext(t *testing.T) {
	directory := t.TempDir()
	store, err := NewEncryptedFileStore(filepath.Join(directory, "secrets.json"), filepath.Join(directory, "secrets.key"))
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("access-token-value")
	reference, err := store.Put(context.Background(), "server-1", secret)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reference, "mcp-secret://server-1/") {
		t.Fatalf("unexpected reference: %s", reference)
	}
	stored, err := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, secret) {
		t.Fatal("plaintext secret was persisted")
	}
	resolved, err := store.Get(context.Background(), reference)
	if err != nil || !bytes.Equal(resolved, secret) {
		t.Fatalf("secret roundtrip failed: %v", err)
	}
	if err := store.Delete(context.Background(), reference); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(context.Background(), reference); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("expected missing secret, got %v", err)
	}
}

func TestEncryptedFileStoreRejectsWrongKey(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "secrets.json")
	first, err := NewEncryptedFileStore(path, filepath.Join(directory, "first.key"))
	if err != nil {
		t.Fatal(err)
	}
	reference, err := first.Put(context.Background(), "server", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewEncryptedFileStore(path, filepath.Join(directory, "second.key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.Get(context.Background(), reference); err == nil {
		t.Fatal("wrong key decrypted secret")
	}
}

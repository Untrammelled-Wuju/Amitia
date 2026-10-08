package secret

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPreparedSecretReferenceIsReservedWithoutWritingAndCannotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.json")
	store, err := NewEncryptedFileStore(path, filepath.Join(dir, "vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	broker, err := NewBroker(BrokerConfig{Store: store})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := broker.ReserveReference("search/serper")
	if err != nil || !ref.Valid() {
		t.Fatal("reservation failed", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("reservation wrote a secret")
	}
	if err := broker.StoreReference(t.Context(), ref, []byte("prepared-fixture-value")); err != nil {
		t.Fatal(err)
	}
	if err := broker.StoreReference(t.Context(), ref, []byte("replacement")); !errors.Is(err, ErrSecretRefInvalid) {
		t.Fatal("existing reference could be overwritten", err)
	}
	value, err := store.Get(t.Context(), ref.String())
	if err != nil || string(value) != "prepared-fixture-value" {
		t.Fatal("previous secret was not preserved")
	}
	raw, err := os.ReadFile(path)
	if err != nil || bytes.Contains(raw, []byte("prepared-fixture-value")) {
		t.Fatal("plaintext secret persisted")
	}
	reserved, err := broker.ReserveReference("search/serper")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := broker.StoreReference(ctx, reserved, []byte("canceled")); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled stage was accepted", err)
	}
	if _, err := store.Get(t.Context(), reserved.String()); !errors.Is(err, ErrSecretNotFound) {
		t.Fatal("canceled stage left data", err)
	}
}

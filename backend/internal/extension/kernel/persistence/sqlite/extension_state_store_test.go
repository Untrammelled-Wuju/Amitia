package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestExtensionStateStoreEnforcesByteQuotaAcrossModules(t *testing.T) {
	db := openWorkflowTestDB(t)
	store := NewExtensionStateStoreWithQuota(db, 8, 0)
	ctx := context.Background()

	if _, err := store.Set(ctx, "com.example/quota", "one", "key", json.RawMessage(`123456`)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Set(ctx, "com.example/quota", "two", "key", json.RawMessage(`123`)); !errors.Is(err, ErrExtensionStateQuotaExceeded) {
		t.Fatalf("expected byte quota failure, got %v", err)
	}
	if _, err := store.Set(ctx, "com.example/quota", "one", "key", json.RawMessage(`1234567`)); err != nil {
		t.Fatalf("replacement within quota failed: %v", err)
	}
	keys, total, err := store.Usage(ctx, "com.example/quota")
	if err != nil {
		t.Fatal(err)
	}
	if keys != 1 || total != 7 {
		t.Fatalf("unexpected usage: keys=%d bytes=%d", keys, total)
	}
}

func TestExtensionStateStoreEnforcesKeyQuotaAndCASReplacement(t *testing.T) {
	db := openWorkflowTestDB(t)
	store := NewExtensionStateStoreWithQuota(db, 0, 1)
	ctx := context.Background()

	if _, err := store.Set(ctx, "com.example/quota", "module", "one", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}
	if swapped, _, err := store.CAS(ctx, "com.example/quota", "module", "one", 0, json.RawMessage(`2`)); err != nil || !swapped {
		t.Fatalf("replacement CAS failed: swapped=%v err=%v", swapped, err)
	}
	if _, _, err := store.CAS(ctx, "com.example/quota", "module", "two", 0, json.RawMessage(`3`)); !errors.Is(err, ErrExtensionStateQuotaExceeded) {
		t.Fatalf("expected key quota failure, got %v", err)
	}
}

func TestExtensionStateStoreDeleteByExtension(t *testing.T) {
	db := openWorkflowTestDB(t)
	store := NewExtensionStateStore(db)
	ctx := context.Background()

	if _, err := store.Set(ctx, "com.example/one", "module", "key", json.RawMessage(`1`)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Set(ctx, "com.example/two", "module", "key", json.RawMessage(`2`)); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteByExtension(ctx, "com.example/one"); err != nil {
		t.Fatal(err)
	}
	if _, _, found, err := store.Get(ctx, "com.example/one", "module", "key"); err != nil || found {
		t.Fatalf("deleted extension state remains: found=%v err=%v", found, err)
	}
	if value, _, found, err := store.Get(ctx, "com.example/two", "module", "key"); err != nil || !found || string(value) != "2" {
		t.Fatalf("unrelated extension state changed: value=%s found=%v err=%v", value, found, err)
	}
}

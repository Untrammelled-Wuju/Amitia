package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func owner() ResourceOwner {
	return ResourceOwner{ExtensionID: "com.example/test", ModuleID: "main"}
}

func TestStorageGetMissing(t *testing.T) {
	b := NewDefaultBroker()
	_, err := b.Get(context.Background(), GetRequest{
		Owner:     owner(),
		Scope:     ScopeModule,
		Namespace: "default",
		Key:       "missing",
	})
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestStorageCAS(t *testing.T) {
	b := NewDefaultBroker()
	body, _ := json.Marshal("hello")
	v, err := b.CompareAndSwap(context.Background(), CASRequest{
		Owner:     owner(),
		Scope:     ScopeModule,
		Namespace: "default",
		Key:       "k1",
		Set:       body,
	})
	if err != nil {
		t.Fatalf("CAS: %v", err)
	}
	if v.Version != 1 {
		t.Errorf("expected v1, got %d", v.Version)
	}
	body2, _ := json.Marshal("world")
	v2, err := b.CompareAndSwap(context.Background(), CASRequest{
		Owner:     owner(),
		Scope:     ScopeModule,
		Namespace: "default",
		Key:       "k1",
		Compare:   &ValuePredicate{Version: 1},
		Set:       body2,
	})
	if err != nil {
		t.Fatalf("CAS2: %v", err)
	}
	if v2.Version != 2 {
		t.Errorf("expected v2, got %d", v2.Version)
	}
	_, err = b.CompareAndSwap(context.Background(), CASRequest{
		Owner:     owner(),
		Scope:     ScopeModule,
		Namespace: "default",
		Key:       "k1",
		Compare:   &ValuePredicate{Version: 1},
		Set:       body2,
	})
	if !errors.Is(err, ErrCASConflict) {
		t.Errorf("expected conflict, got %v", err)
	}
}

func TestStorageTTL(t *testing.T) {
	b := NewDefaultBroker()
	body, _ := json.Marshal("ephemeral")
	_, err := b.CompareAndSwap(context.Background(), CASRequest{
		Owner:     owner(),
		Scope:     ScopeModule,
		Namespace: "default",
		Key:       "k2",
		Set:       body,
		TTL:       50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	_, err = b.Get(context.Background(), GetRequest{
		Owner:     owner(),
		Scope:     ScopeModule,
		Namespace: "default",
		Key:       "k2",
	})
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected TTL expiry, got %v", err)
	}
}

func TestStorageListAndDelete(t *testing.T) {
	b := NewDefaultBroker()
	for _, k := range []string{"a", "b", "c"} {
		body, _ := json.Marshal(k)
		_, _ = b.CompareAndSwap(context.Background(), CASRequest{
			Owner: owner(), Scope: ScopeModule, Namespace: "default", Key: k, Set: body,
		})
	}
	page, err := b.List(context.Background(), ListRequest{
		Owner: owner(), Scope: ScopeModule, Namespace: "default", PageSize: 2,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(page.Items))
	}
	if page.NextCursor == "" {
		t.Errorf("expected next cursor")
	}
	if err := b.Delete(context.Background(), DeleteRequest{
		Owner: owner(), Scope: ScopeModule, Namespace: "default", Key: "a",
	}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err = b.Get(context.Background(), GetRequest{
		Owner: owner(), Scope: ScopeModule, Namespace: "default", Key: "a",
	})
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestStorageTransaction(t *testing.T) {
	b := NewDefaultBroker()
	body1, _ := json.Marshal("v1")
	body2, _ := json.Marshal("v2")
	result := b.Transaction(context.Background(), TxRequest{
		Ops: []TxOp{
			{Kind: TxOpSet, Owner: owner(), Scope: ScopeModule, Namespace: "default", Key: "tx1", Value: body1},
			{Kind: TxOpSet, Owner: owner(), Scope: ScopeModule, Namespace: "default", Key: "tx2", Value: body2},
		},
	})
	if !result.Applied {
		t.Fatalf("expected applied: %s", result.Error)
	}
	if len(result.Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(result.Values))
	}
}

func TestStorageQuota(t *testing.T) {
	b := NewDefaultBroker()
	b.SetQuota(owner(), 100, 2)
	body, _ := json.Marshal("hello")
	_, _ = b.CompareAndSwap(context.Background(), CASRequest{
		Owner: owner(), Scope: ScopeModule, Namespace: "default", Key: "q1", Set: body,
	})
	_, _ = b.CompareAndSwap(context.Background(), CASRequest{
		Owner: owner(), Scope: ScopeModule, Namespace: "default", Key: "q2", Set: body,
	})
	_, err := b.CompareAndSwap(context.Background(), CASRequest{
		Owner: owner(), Scope: ScopeModule, Namespace: "default", Key: "q3", Set: body,
	})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Errorf("expected quota exceeded, got %v", err)
	}
}

func TestSecretCreateRead(t *testing.T) {
	b := NewDefaultSecretBroker([]byte("test-key"))
	ref, err := b.Create(context.Background(), SecretCreateRequest{
		Owner:     owner(),
		Name:      "api-key",
		Plaintext: []byte("super-secret-value"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ref.RefID == "" {
		t.Errorf("expected ref id")
	}
	val, err := b.Read(context.Background(), SecretReadRequest{
		Owner: owner(),
		Name:  "api-key",
	})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(val.Plaintext) != "super-secret-value" {
		t.Errorf("expected plaintext, got %s", val.Plaintext)
	}
}

func TestSecretRotate(t *testing.T) {
	b := NewDefaultSecretBroker([]byte("test-key"))
	_, _ = b.Create(context.Background(), SecretCreateRequest{
		Owner:     owner(),
		Name:      "rotate-test",
		Plaintext: []byte("v1"),
	})
	ref, err := b.Rotate(context.Background(), SecretRotateRequest{
		Owner:     owner(),
		Name:      "rotate-test",
		Plaintext: []byte("v2"),
	})
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if ref.Version != 2 {
		t.Errorf("expected v2, got %d", ref.Version)
	}
	val, _ := b.Read(context.Background(), SecretReadRequest{Owner: owner(), Name: "rotate-test"})
	if string(val.Plaintext) != "v2" {
		t.Errorf("expected v2, got %s", val.Plaintext)
	}
	old, _ := b.Read(context.Background(), SecretReadRequest{Owner: owner(), Name: "rotate-test", Version: 1})
	if string(old.Plaintext) != "v1" {
		t.Errorf("expected v1 historical, got %s", old.Plaintext)
	}
}

func TestSecretRevoke(t *testing.T) {
	b := NewDefaultSecretBroker([]byte("test-key"))
	_, _ = b.Create(context.Background(), SecretCreateRequest{
		Owner:     owner(),
		Name:      "revoke-test",
		Plaintext: []byte("secret"),
	})
	if err := b.Revoke(context.Background(), owner(), "revoke-test"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	_, err := b.Read(context.Background(), SecretReadRequest{Owner: owner(), Name: "revoke-test"})
	if !errors.Is(err, ErrSecretRevoked) {
		t.Errorf("expected revoked, got %v", err)
	}
}

func TestSecretShare(t *testing.T) {
	b := NewDefaultSecretBroker([]byte("test-key"))
	_, _ = b.Create(context.Background(), SecretCreateRequest{
		Owner:     owner(),
		Name:      "shared",
		Plaintext: []byte("shared-value"),
		Shared:    true,
	})
	target := ResourceOwner{ExtensionID: "com.example/other", ModuleID: "main"}
	_, err := b.Share(context.Background(), SecretShareRequest{
		Owner:       owner(),
		Name:        "shared",
		TargetOwner: target,
		ExpiresIn:   1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Share: %v", err)
	}
	val, err := b.Read(context.Background(), SecretReadRequest{Owner: target, Name: "shared"})
	if err != nil {
		t.Fatalf("Read shared: %v", err)
	}
	if string(val.Plaintext) != "shared-value" {
		t.Errorf("expected shared-value, got %s", val.Plaintext)
	}
}

func TestSecretAccessDenied(t *testing.T) {
	b := NewDefaultSecretBroker([]byte("test-key"))
	_, _ = b.Create(context.Background(), SecretCreateRequest{
		Owner:     owner(),
		Name:      "private",
		Plaintext: []byte("mine"),
	})
	stranger := ResourceOwner{ExtensionID: "com.example/stranger", ModuleID: "main"}
	_, err := b.Read(context.Background(), SecretReadRequest{Owner: stranger, Name: "private"})
	if !errors.Is(err, ErrSecretAccessDenied) {
		t.Errorf("expected access denied, got %v", err)
	}
}

func TestSecretDelete(t *testing.T) {
	b := NewDefaultSecretBroker([]byte("test-key"))
	_, _ = b.Create(context.Background(), SecretCreateRequest{
		Owner:     owner(),
		Name:      "del",
		Plaintext: []byte("will-be-deleted"),
	})
	if err := b.Delete(context.Background(), owner(), "del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	_, err := b.Read(context.Background(), SecretReadRequest{Owner: owner(), Name: "del"})
	if !errors.Is(err, ErrSecretNotFound) {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestSecretNameConflict(t *testing.T) {
	b := NewDefaultSecretBroker([]byte("test-key"))
	_, _ = b.Create(context.Background(), SecretCreateRequest{
		Owner:     owner(),
		Name:      "dup",
		Plaintext: []byte("first"),
	})
	_, err := b.Create(context.Background(), SecretCreateRequest{
		Owner:     owner(),
		Name:      "dup",
		Plaintext: []byte("second"),
	})
	if !errors.Is(err, ErrSecretNameConflict) {
		t.Errorf("expected conflict, got %v", err)
	}
}

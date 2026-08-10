package secret

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"
)

type mockStore struct {
	secrets map[string][]byte
}

func (m *mockStore) Put(ctx context.Context, namespace string, value []byte) (string, error) {
	ref := "secret://" + namespace + "/store-id"
	m.secrets[ref] = value
	return ref, nil
}

func (m *mockStore) Get(ctx context.Context, ref string) ([]byte, error) {
	v, ok := m.secrets[ref]
	if !ok {
		return nil, ErrSecretNotFound
	}
	out := make([]byte, len(v))
	copy(out, v)
	return out, nil
}

func (m *mockStore) Delete(ctx context.Context, ref string) error {
	delete(m.secrets, ref)
	return nil
}

func TestBrokerIssue(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-1")
	store.secrets[string(ref)] = []byte("SUPER_SECRET_123456")

	broker, err := NewBroker(BrokerConfig{Store: store})
	if err != nil {
		t.Fatalf("new broker: %v", err)
	}

	lease, err := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test.purpose",
		InvocationID:      "inv-1",
		RuntimeInstanceID: "inst-1",
		ExtensionID:       "ext-1",
		ModuleID:          "mod-1",
		Generation:        1,
		MaxUses:           3,
		TTL:               5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if lease.ID == "" {
		t.Fatal("expected non-empty lease ID")
	}
	if string(lease.Ref) != "secret://test/key-1" {
		t.Fatalf("unexpected ref: %s", lease.Ref)
	}
	if lease.MaxUses != 3 {
		t.Fatalf("expected MaxUses=3, got %d", lease.MaxUses)
	}
}

func TestBrokerIssue_StoreUnavailable(t *testing.T) {
	_, err := NewBroker(BrokerConfig{Store: nil})
	if err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestBrokerIssue_InvalidRef(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	broker, _ := NewBroker(BrokerConfig{Store: store})

	_, err := broker.Issue(context.Background(), LeaseRequest{
		Ref:               "",
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
	})
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
}

func TestBrokerIssue_FailClosedWhenS(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/nonexistent")
	broker, _ := NewBroker(BrokerConfig{Store: store})

	_, err := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
	})
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestBrokerConsume_Success(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-2")
	store.secrets[string(ref)] = []byte("SECRET_VALUE_123")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		InvocationID:      "inv-1",
		RuntimeInstanceID: "inst-1",
		ExtensionID:       "ext-1",
		ModuleID:          "mod-1",
		Generation:        1,
		MaxUses:           2,
	})

	val, err := broker.Consume(context.Background(), lease.ID, LeaseUseContext{
		InvocationID:      "inv-1",
		RuntimeInstanceID: "inst-1",
		ExtensionID:       "ext-1",
		ModuleID:          "mod-1",
		Generation:        1,
	})
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if !bytes.Equal(val, []byte("SECRET_VALUE_123")) {
		t.Fatalf("unexpected value: %s", val)
	}
}

func TestBrokerConsume_InvocationMismatch(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-3")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		InvocationID:      "inv-A",
		RuntimeInstanceID: "inst-1",
		ExtensionID:       "ext-1",
	})

	_, err := broker.Consume(context.Background(), lease.ID, LeaseUseContext{
		InvocationID: "inv-B",
	})
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestBrokerConsume_GenerationMismatch(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-4")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
		ExtensionID:       "ext-1",
		Generation:        5,
	})

	_, err := broker.Consume(context.Background(), lease.ID, LeaseUseContext{
		ExtensionID: "ext-1",
		Generation:  6,
	})
	if err == nil {
		t.Fatal("expected generation mismatch error")
	}
}

func TestBrokerConsume_Revoked(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-5")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
	})

	if err := broker.RevokeLease(lease.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	_, err := broker.Consume(context.Background(), lease.ID, LeaseUseContext{})
	if err != ErrSecretLeaseRevoked {
		t.Fatalf("expected ErrSecretLeaseRevoked, got %v", err)
	}
}

func TestBrokerConsume_Expired(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-6")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	broker.now = func() time.Time { return time.Now().Add(-1 * time.Hour) }
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
		TTL:               1 * time.Millisecond,
	})
	broker.now = time.Now

	_, err := broker.Consume(context.Background(), lease.ID, LeaseUseContext{})
	if err != ErrSecretLeaseExpired {
		t.Fatalf("expected ErrSecretLeaseExpired, got %v", err)
	}
}

func TestBrokerConsume_Exhausted(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-7")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
		MaxUses:           1,
	})

	_, _ = broker.Consume(context.Background(), lease.ID, LeaseUseContext{})

	_, err := broker.Consume(context.Background(), lease.ID, LeaseUseContext{})
	if err != ErrSecretLeaseExhausted && err != ErrSecretLeaseRevoked {
		t.Fatalf("expected exhausted or revoked error, got %v", err)
	}
}

func TestBrokerConsume_MaxUsesConcurrent(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-concurrent")
	store.secrets[string(ref)] = []byte("B24-SUPER-SECRET-9482")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
		MaxUses:           1,
	})

	var wg sync.WaitGroup
	successCount := int32(0)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := broker.Consume(context.Background(), lease.ID, LeaseUseContext{})
			if err == nil {
				successCount++
			}
		}()
	}
	wg.Wait()
	if successCount != 1 {
		t.Fatalf("expected exactly 1 success, got %d", successCount)
	}
}

func TestBrokerRevoke(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-8")
	store.secrets[string(ref)] = []byte("secret-value")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
	})

	if err := broker.RevokeLease(lease.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	got, ok := broker.GetLease(lease.ID)
	if !ok {
		t.Fatal("expected lease descriptor to persist after revoke")
	}
	if !got.Revoked {
		t.Fatal("expected lease to be marked revoked")
	}
}

func TestBrokerRevoke_Idempotent(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-9")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
	})

	_ = broker.RevokeLease(lease.ID)
	if err := broker.RevokeLease(lease.ID); err != nil {
		t.Fatalf("second revoke should be nil: %v", err)
	}
}

func TestBrokerGet_NoValue(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-10")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
	})

	got, ok := broker.GetLease(lease.ID)
	if !ok {
		t.Fatal("expected to find lease")
	}
	if got.ID != lease.ID {
		t.Fatalf("expected %s, got %s", lease.ID, got.ID)
	}
}

func TestBrokerRedact(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-redact")
	store.secrets[string(ref)] = []byte("B24-SUPER-SECRET-9482")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	_, err := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
	})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	redacted := broker.redactor.Redact("log contains B24-SUPER-SECRET-9482 in text")
	if redacted == "log contains B24-SUPER-SECRET-9482 in text" {
		t.Fatalf("expected redaction, got: %s", redacted)
	}
}

func TestLeaseCanUse(t *testing.T) {
	now := time.Now()
	l := Lease{
		IssuedAt:  now,
		ExpiresAt: now.Add(5 * time.Minute),
		MaxUses:   2,
	}
	if !l.CanUse() {
		t.Fatal("expected CanUse=true")
	}

	l.Revoked = true
	if l.CanUse() {
		t.Fatal("expected CanUse=false when revoked")
	}

	l.Revoked = false
	l.ExpiresAt = now.Add(-1 * time.Minute)
	if l.CanUse() {
		t.Fatal("expected CanUse=false when expired")
	}

	l.ExpiresAt = now.Add(5 * time.Minute)
	l.UsedCount = 2
	if l.CanUse() {
		t.Fatal("expected CanUse=false when exhausted")
	}
}

func TestWithSecret(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-with")
	store.secrets[string(ref)] = []byte("secret-for-fn")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	lease, _ := broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
		MaxUses:           1,
	})

	var captured []byte
	err := broker.WithSecret(context.Background(), lease.ID, LeaseUseContext{}, func(v []byte) error {
		captured = make([]byte, len(v))
		copy(captured, v)
		return nil
	})
	if err != nil {
		t.Fatalf("with secret: %v", err)
	}
	if !bytes.Equal(captured, []byte("secret-for-fn")) {
		t.Fatalf("unexpected captured: %s", captured)
	}
}

func TestRevokeByInvocation(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-batch")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	for i := 0; i < 3; i++ {
		_, _ = broker.Issue(context.Background(), LeaseRequest{
			Ref:               ref,
			Purpose:           "test",
			InvocationID:      "inv-target",
			RuntimeInstanceID: "inst-1",
		})
	}
	_, _ = broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		InvocationID:      "inv-other",
		RuntimeInstanceID: "inst-2",
	})

	count := broker.RevokeByInvocation("inv-target")
	if count != 3 {
		t.Fatalf("expected 3 revoked, got %d", count)
	}
}

func TestRevokeByRuntimeInstance(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-ri")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	for i := 0; i < 2; i++ {
		_, _ = broker.Issue(context.Background(), LeaseRequest{
			Ref:               ref,
			Purpose:           "test",
			RuntimeInstanceID: "inst-target",
		})
	}
	_, _ = broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-other",
	})

	count := broker.RevokeByRuntimeInstance("inst-target")
	if count != 2 {
		t.Fatalf("expected 2 revoked, got %d", count)
	}
}

func TestRevokeByExtensionGeneration(t *testing.T) {
	store := &mockStore{secrets: map[string][]byte{}}
	ref := SecretRef("secret://test/key-gen")
	store.secrets[string(ref)] = []byte("secret")

	broker, _ := NewBroker(BrokerConfig{Store: store})
	for i := 0; i < 3; i++ {
		_, _ = broker.Issue(context.Background(), LeaseRequest{
			Ref:               ref,
			Purpose:           "test",
			RuntimeInstanceID: "inst-1",
			ExtensionID:       "ext-a",
			Generation:        5,
		})
	}
	_, _ = broker.Issue(context.Background(), LeaseRequest{
		Ref:               ref,
		Purpose:           "test",
		RuntimeInstanceID: "inst-1",
		ExtensionID:       "ext-a",
		Generation:        6,
	})

	count := broker.RevokeByExtensionGeneration("ext-a", 5)
	if count != 3 {
		t.Fatalf("expected 3 revoked, got %d", count)
	}
}

func TestRedactorShortValue(t *testing.T) {
	r := NewRedactor()
	r.Add([]byte("ab"))
	if r.Redact("text with ab in it") != "text with ab in it" {
		t.Fatal("short values should not be redacted")
	}
}

func TestSensitiveEnvKey(t *testing.T) {
	sensitive := []string{"MY_TOKEN", "my_password", "API_KEY", "GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY"}
	for _, k := range sensitive {
		if !IsSensitiveEnvKey(k) {
			t.Fatalf("expected %s to be sensitive", k)
		}
	}
	normal := []string{"AMITIA_INSTANCE_ID", "HOME", "PATH"}
	for _, k := range normal {
		if IsSensitiveEnvKey(k) {
			t.Fatalf("expected %s to not be sensitive", k)
		}
	}
}

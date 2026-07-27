package trusted_service

import (
	"testing"
	"time"
)

type mockSecretProvider struct {
	secrets map[string]string
}

func (m *mockSecretProvider) GetSecret(name string) (string, error) {
	if v, ok := m.secrets[name]; ok {
		return v, nil
	}
	return "", ErrSecretLeaseNotFound
}

func TestSecretLease_Issue(t *testing.T) {
	provider := &mockSecretProvider{secrets: map[string]string{"api_key": "secret123"}}
	m := NewSecretLeaseManager(provider)

	lease, err := m.Issue(SecretLeaseRequest{
		SecretName:      "api_key",
		Purpose:         "test",
		RuntimeInstance: "inst-1",
		ExtensionID:     "ext-1",
		ModuleID:        "mod-1",
		TTL:             5 * time.Minute,
		MaxUses:         3,
	})
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	if lease.LeaseID == "" {
		t.Fatal("expected non-empty lease ID")
	}
	if lease.Value != "secret123" {
		t.Fatalf("expected value secret123, got %s", lease.Value)
	}
	if lease.MaxUses != 3 {
		t.Fatalf("expected MaxUses=3, got %d", lease.MaxUses)
	}
	if !lease.CanUse() {
		t.Fatal("expected CanUse=true for fresh lease")
	}
}

func TestSecretLease_Consume(t *testing.T) {
	provider := &mockSecretProvider{secrets: map[string]string{"token": "abc"}}
	m := NewSecretLeaseManager(provider)

	lease, _ := m.Issue(SecretLeaseRequest{
		SecretName:      "token",
		Purpose:         "test",
		RuntimeInstance: "inst-1",
		TTL:             5 * time.Minute,
		MaxUses:         2,
	})

	val, err := m.Consume(lease.LeaseID)
	if err != nil {
		t.Fatalf("consume failed: %v", err)
	}
	if val != "abc" {
		t.Fatalf("expected value abc, got %s", val)
	}

	val, err = m.Consume(lease.LeaseID)
	if err != nil {
		t.Fatalf("second consume failed: %v", err)
	}

	_, err = m.Consume(lease.LeaseID)
	if err == nil {
		t.Fatal("expected error on third consume (max uses exceeded)")
	}
}

func TestSecretLease_Revoke(t *testing.T) {
	provider := &mockSecretProvider{secrets: map[string]string{"key": "val"}}
	m := NewSecretLeaseManager(provider)

	lease, _ := m.Issue(SecretLeaseRequest{
		SecretName:      "key",
		Purpose:         "test",
		RuntimeInstance: "inst-1",
		TTL:             5 * time.Minute,
	})

	err := m.Revoke(lease.LeaseID, "security incident")
	if err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	if !lease.IsRevoked() {
		t.Fatal("expected revoked=true")
	}

	_, err = m.Consume(lease.LeaseID)
	if err == nil {
		t.Fatal("expected error consuming revoked lease")
	}
}

func TestSecretLease_RevokeByInstance(t *testing.T) {
	provider := &mockSecretProvider{secrets: map[string]string{"key": "val"}}
	m := NewSecretLeaseManager(provider)

	_, _ = m.Issue(SecretLeaseRequest{
		SecretName:      "key",
		Purpose:         "test1",
		RuntimeInstance: "inst-1",
		TTL:             5 * time.Minute,
	})
	_, _ = m.Issue(SecretLeaseRequest{
		SecretName:      "key",
		Purpose:         "test2",
		RuntimeInstance: "inst-1",
		TTL:             5 * time.Minute,
	})
	_, _ = m.Issue(SecretLeaseRequest{
		SecretName:      "key",
		Purpose:         "test3",
		RuntimeInstance: "inst-2",
		TTL:             5 * time.Minute,
	})

	count := m.RevokeByInstance("inst-1", "shutdown")
	if count != 2 {
		t.Fatalf("expected 2 revoked, got %d", count)
	}

	leases := m.ListByInstance("inst-1")
	for _, l := range leases {
		if !l.IsRevoked() {
			t.Fatal("expected all inst-1 leases revoked")
		}
	}
}

func TestSecretLease_Expiration(t *testing.T) {
	provider := &mockSecretProvider{secrets: map[string]string{"key": "val"}}
	m := NewSecretLeaseManager(provider)

	lease, _ := m.Issue(SecretLeaseRequest{
		SecretName:      "key",
		Purpose:         "test",
		RuntimeInstance: "inst-1",
		TTL:             1 * time.Millisecond,
	})

	time.Sleep(10 * time.Millisecond)

	if !lease.IsExpired() {
		t.Fatal("expected expired")
	}
	if lease.CanUse() {
		t.Fatal("expected CanUse=false for expired lease")
	}
}

func TestSecretLease_Cleanup(t *testing.T) {
	provider := &mockSecretProvider{secrets: map[string]string{"key": "val"}}
	m := NewSecretLeaseManager(provider)

	_, _ = m.Issue(SecretLeaseRequest{
		SecretName:      "key",
		Purpose:         "test1",
		RuntimeInstance: "inst-1",
		TTL:             1 * time.Millisecond,
	})
	lease2, _ := m.Issue(SecretLeaseRequest{
		SecretName:      "key",
		Purpose:         "test2",
		RuntimeInstance: "inst-1",
		TTL:             5 * time.Minute,
	})
	_ = m.Revoke(lease2.LeaseID, "test")

	time.Sleep(10 * time.Millisecond)

	removed := m.Cleanup()
	if removed != 2 {
		t.Fatalf("expected 2 removed, got %d", removed)
	}
}

func TestSecretLease_AuditTrail(t *testing.T) {
	provider := &mockSecretProvider{secrets: map[string]string{"key": "val"}}
	m := NewSecretLeaseManager(provider)

	lease, _ := m.Issue(SecretLeaseRequest{
		SecretName:      "key",
		Purpose:         "test",
		RuntimeInstance: "inst-1",
		TTL:             5 * time.Minute,
	})
	_, _ = m.Consume(lease.LeaseID)
	_ = m.Revoke(lease.LeaseID, "done")

	entries := m.AuditEntries()
	if len(entries) < 3 {
		t.Fatalf("expected at least 3 audit entries, got %d", len(entries))
	}

	actions := make(map[string]bool)
	for _, e := range entries {
		actions[e.Action] = true
	}
	if !actions["issue"] {
		t.Fatal("expected 'issue' action in audit")
	}
	if !actions["consume"] {
		t.Fatal("expected 'consume' action in audit")
	}
	if !actions["revoke"] {
		t.Fatal("expected 'revoke' action in audit")
	}
}

func TestSecretLease_IssueValidation(t *testing.T) {
	m := NewSecretLeaseManager(nil)

	_, err := m.Issue(SecretLeaseRequest{
		Purpose:         "test",
		RuntimeInstance: "inst-1",
	})
	if err == nil {
		t.Fatal("expected error for missing secret name")
	}

	_, err = m.Issue(SecretLeaseRequest{
		SecretName: "key",
		RuntimeInstance: "inst-1",
	})
	if err == nil {
		t.Fatal("expected error for missing purpose")
	}

	_, err = m.Issue(SecretLeaseRequest{
		SecretName: "key",
		Purpose:    "test",
	})
	if err == nil {
		t.Fatal("expected error for missing runtime instance")
	}
}

package search

import (
	"context"
	"errors"
	"testing"
)

type fakeLeaseIssuer struct {
	leaseIDs    []string
	consumeIDs  []string
	credential  string
	issueErr    error
	consumeErr   error
	refusalLease string
}

func (f *fakeLeaseIssuer) Issue(ctx context.Context, ref string, purpose string) (string, error) {
	if f.issueErr != nil {
		return "", f.issueErr
	}
	id := "lease-" + ref
	f.leaseIDs = append(f.leaseIDs, id)
	return id, nil
}

func (f *fakeLeaseIssuer) Consume(ctx context.Context, leaseID string) (string, error) {
	f.consumeIDs = append(f.consumeIDs, leaseID)
	if f.consumeErr != nil {
		return "", f.consumeErr
	}
	return f.credential, nil
}

func TestSecretBridge_Resolve_Success(t *testing.T) {
	issuer := &fakeLeaseIssuer{credential: "api-key-xyz"}
	bridge := NewSecretBridge(issuer)
	cred, release, err := bridge.Resolve(context.Background(), "brave", "inv-1", "secret/brave-api-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cred != "api-key-xyz" {
		t.Fatalf("wrong credential: %q", cred)
	}
	if len(issuer.leaseIDs) != 1 || len(issuer.consumeIDs) != 1 {
		t.Fatalf("issue/consume not called: %d %d", len(issuer.leaseIDs), len(issuer.consumeIDs))
	}
	if release == nil {
		t.Fatal("release func should not be nil")
	}
}

func TestSecretBridge_Resolve_NoRef(t *testing.T) {
	issuer := &fakeLeaseIssuer{credential: "x"}
	bridge := NewSecretBridge(issuer)
	cred, release, err := bridge.Resolve(context.Background(), "brave", "inv-1", "")
	if err != nil {
		t.Fatalf("empty ref should not error: %v", err)
	}
	if cred != "" {
		t.Fatal("expected empty credential when no ref")
	}
	if release == nil {
		t.Fatal("release func expected")
	}
	if len(issuer.leaseIDs) != 0 {
		t.Fatal("issue should NOT be called when no ref")
	}
}

func TestSecretBridge_Resolve_IssueError(t *testing.T) {
	issuer := &fakeLeaseIssuer{issueErr: errors.New("no broker")}
	bridge := NewSecretBridge(issuer)
	_, _, err := bridge.Resolve(context.Background(), "brave", "inv-1", "secret/x")
	if err == nil {
		t.Fatal("should propagate issue error")
	}
}

func TestSecretBridge_Resolve_ConsumeError(t *testing.T) {
	issuer := &fakeLeaseIssuer{consumeErr: errors.New("leaked")}
	bridge := NewSecretBridge(issuer)
	_, _, err := bridge.Resolve(context.Background(), "brave", "inv-1", "secret/x")
	if err == nil {
		t.Fatal("should propagate consume error")
	}
}

func TestSecretBridge_NilIssuer(t *testing.T) {
	if NewSecretBridge(nil) != nil {
		t.Fatal("nil issuer should produce nil bridge")
	}
}

func TestSecretBridge_NilReceiverNoPanic(t *testing.T) {
	var bridge *SecretBridge
	cred, release, err := bridge.Resolve(context.Background(), "b", "i", "secret/x")
	if err != nil {
		t.Fatalf("nil bridge should not error: %v", err)
	}
	if cred != "" || release == nil {
		t.Fatal("expected safe defaults")
	}
}

func TestSecretBridge_PurposePassed(t *testing.T) {
	var capturedPurpose string
	issuer := &purposeCapturingIssuer{purpose: &capturedPurpose}
	_ = SearchPurpose
	bridge := NewSecretBridge(issuer)
	_, _, _ = bridge.Resolve(context.Background(), "b", "i", "secret/p")
	if capturedPurpose != SearchPurpose {
		t.Fatalf("wrong purpose: %q", capturedPurpose)
	}
}

type purposeCapturingIssuer struct {
	purpose *string
}

func (p *purposeCapturingIssuer) Issue(ctx context.Context, ref string, purpose string) (string, error) {
	*p.purpose = purpose
	return "lease-id", nil
}

func (p *purposeCapturingIssuer) Consume(ctx context.Context, leaseID string) (string, error) {
	return "cred", nil
}

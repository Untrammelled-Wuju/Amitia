package kernel

import (
	"testing"
	"time"
)

func TestR45TemporalBindingAcceptsValidWindow(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(-time.Minute).Unix()
	expiresAt := now.Add(5 * time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, "valid-nonce-123", now); err != nil {
		t.Fatalf("valid temporal binding should be accepted: %v", err)
	}
}

func TestR45TemporalBindingRejectsMissingIssuedAt(t *testing.T) {
	now := time.Now().UTC()
	expiresAt := now.Add(5 * time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(0, expiresAt, "nonce-1", now); err == nil {
		t.Fatal("missing issuedAt should be rejected")
	}
	if err := validatePackageConfirmationTemporalBinding(-1, expiresAt, "nonce-1", now); err == nil {
		t.Fatal("negative issuedAt should be rejected")
	}
}

func TestR45TemporalBindingRejectsMissingExpiresAt(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(-time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, 0, "nonce-2", now); err == nil {
		t.Fatal("missing expiresAt should be rejected")
	}
	if err := validatePackageConfirmationTemporalBinding(issuedAt, -1, "nonce-2", now); err == nil {
		t.Fatal("negative expiresAt should be rejected")
	}
}

func TestR45TemporalBindingRejectsFutureIssuedAt(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(time.Minute).Unix()
	expiresAt := now.Add(5 * time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, "nonce-3", now); err == nil {
		t.Fatal("future issuedAt beyond clock skew should be rejected")
	}
}

func TestR45TemporalBindingAllowsConfiguredClockSkew(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(20 * time.Second).Unix()
	expiresAt := now.Add(5 * time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, "nonce-4", now); err != nil {
		t.Fatalf("issuedAt within clock skew should be accepted: %v", err)
	}
}

func TestR45TemporalBindingRejectsExpiresEqualIssued(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(-time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, issuedAt, "nonce-5", now); err == nil {
		t.Fatal("expiresAt equal to issuedAt should be rejected")
	}
}

func TestR45TemporalBindingRejectsExpiresBeforeIssued(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(5 * time.Minute).Unix()
	expiresAt := now.Add(time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, "nonce-6", now); err == nil {
		t.Fatal("expiresAt before issuedAt should be rejected")
	}
}

func TestR45TemporalBindingRejectsExpiredClaims(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(-10 * time.Minute).Unix()
	expiresAt := now.Add(-time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, "nonce-7", now); err == nil {
		t.Fatal("expired claims should be rejected")
	}
}

func TestR45TemporalBindingRejectsExcessiveLifetime(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(-time.Minute).Unix()
	expiresAt := issuedAt + int64((packageConfirmationLifetime+packageConfirmationClockSkew+time.Second).Seconds())
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, "nonce-8", now); err == nil {
		t.Fatal("lifetime exceeding policy should be rejected")
	}
}

func TestR45TemporalBindingRejectsEmptyNonce(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(-time.Minute).Unix()
	expiresAt := now.Add(5 * time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, "", now); err == nil {
		t.Fatal("empty nonce should be rejected")
	}
}

func TestR45TemporalBindingRejectsWhitespaceNonce(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(-time.Minute).Unix()
	expiresAt := now.Add(5 * time.Minute).Unix()
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, "   ", now); err == nil {
		t.Fatal("whitespace-only nonce should be rejected")
	}
}

func TestR45TemporalBindingRejectsOversizedNonce(t *testing.T) {
	now := time.Now().UTC()
	issuedAt := now.Add(-time.Minute).Unix()
	expiresAt := now.Add(5 * time.Minute).Unix()
	bigNonce := make([]byte, packageConfirmationMaxNonceLength+1)
	for i := range bigNonce {
		bigNonce[i] = 'a'
	}
	if err := validatePackageConfirmationTemporalBinding(issuedAt, expiresAt, string(bigNonce), now); err == nil {
		t.Fatal("oversized nonce should be rejected")
	}
}

package kernel

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/sqlite"
	persistencesqlite "github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

func newR45FinalGateTestRuntime(t *testing.T) (*Runtime, *PackageRepository) {
	t.Helper()
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "r45-finalgate.db")) + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := persistencesqlite.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	repo := NewPackageRepository(db)
	r := &Runtime{container: &Container{PackageRepository: repo}}
	return r, repo
}

func r45ValidClaimsTemplate() PackageConfirmationClaims {
	now := time.Now().UTC()
	return PackageConfirmationClaims{
		IssuedAt:                  now.Add(-time.Minute).Unix(),
		ExpiresAt:                 now.Add(5 * time.Minute).Unix(),
		Nonce:                     "final-gate-nonce",
		OperationType:             "install",
		ExtensionID:               "ext-finalgate",
		UserID:                    "user-finalgate",
		SecurityPolicyHash:        computeSecurityPolicyHash(),
		RequiredConfirmationsHash: "sha256:req-conf-hash",
		DependenciesHash:          "sha256:dep-hash",
		ConfirmedItems:            []string{"confirm.scripts"},
		Confirmations:             map[string]bool{"confirm.scripts": true},
		PreviewHash:               "sha256:preview",
	}
}

func TestR45FinalGateRejectsFutureIssuedAt(t *testing.T) {
	r, _ := newR45FinalGateTestRuntime(t)
	claims := r45ValidClaimsTemplate()
	claims.IssuedAt = time.Now().UTC().Add(time.Minute).Unix()
	op := PackageOperationRecord{OperationID: "op-fg-1", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("future issuedAt should be rejected")
	}
}

func TestR45FinalGateRejectsExpiresEqualIssued(t *testing.T) {
	r, _ := newR45FinalGateTestRuntime(t)
	claims := r45ValidClaimsTemplate()
	claims.ExpiresAt = claims.IssuedAt
	op := PackageOperationRecord{OperationID: "op-fg-2", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("expiresAt == issuedAt should be rejected")
	}
}

func TestR45FinalGateRejectsExpiresBeforeIssued(t *testing.T) {
	r, _ := newR45FinalGateTestRuntime(t)
	claims := r45ValidClaimsTemplate()
	claims.ExpiresAt = claims.IssuedAt - int64(time.Hour.Seconds())
	op := PackageOperationRecord{OperationID: "op-fg-3", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("expiresAt before issuedAt should be rejected")
	}
}

func TestR45FinalGateRejectsCurrentExpiredClaims(t *testing.T) {
	r, _ := newR45FinalGateTestRuntime(t)
	claims := r45ValidClaimsTemplate()
	claims.IssuedAt = time.Now().UTC().Add(-2 * time.Hour).Unix()
	claims.ExpiresAt = time.Now().UTC().Add(-time.Hour).Unix()
	op := PackageOperationRecord{OperationID: "op-fg-4", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("expired claims should be rejected at final gate")
	}
}

func TestR45FinalGateRejectsCurrentSecurityPolicyDrift(t *testing.T) {
	r, _ := newR45FinalGateTestRuntime(t)
	claims := r45ValidClaimsTemplate()
	claims.SecurityPolicyHash = "stale-policy-hash-revised-2028XYZ"
	op := PackageOperationRecord{OperationID: "op-fg-5", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("stale security policy hash should be rejected")
	}
}

func TestR45FinalGateRejectsMissingNonceBinding(t *testing.T) {
	r, _ := newR45FinalGateTestRuntime(t)
	claims := r45ValidClaimsTemplate()
	claims.Nonce = "nonexistent-nonce-finalgate"
	op := PackageOperationRecord{OperationID: "op-fg-6", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("missing nonce binding should be rejected at final gate")
	}
}

func TestR45FinalGateRejectsEmptyRequiredConfirmationsHash(t *testing.T) {
	r, _ := newR45FinalGateTestRuntime(t)
	claims := r45ValidClaimsTemplate()
	claims.RequiredConfirmationsHash = ""
	op := PackageOperationRecord{OperationID: "op-fg-7", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("empty requiredConfirmationsHash should be rejected")
	}
}

func TestR45FinalGateRejectsEmptyDependenciesHash(t *testing.T) {
	r, _ := newR45FinalGateTestRuntime(t)
	claims := r45ValidClaimsTemplate()
	claims.DependenciesHash = ""
	op := PackageOperationRecord{OperationID: "op-fg-8", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("empty dependenciesHash should be rejected")
	}
}

func TestR45RuntimeNilContainerRejectsBinding(t *testing.T) {
	r := &Runtime{}
	claims := r45ValidClaimsTemplate()
	op := PackageOperationRecord{OperationID: "op-fg-9", OperationType: "install", ExtensionID: "ext-finalgate", UserID: "user-finalgate"}
	if err := r.verifyPackageClaimsBinding(context.Background(), op, claims); err == nil {
		t.Fatal("nil container should be rejected at final gate")
	}
}

package kernel

import (
	"testing"
	"time"
)

func TestPackageConfirmationTokenRejectsTamperAndExpiry(t *testing.T) {
	claims := packageConfirmationClaims{SessionID: "preview-1", ArtifactID: "artifact-1", ArchiveHash: "sha256:a", ManifestHash: "sha256:m", ContentTreeHash: "sha256:t", UserID: "user-1", ScopeType: "global", PolicyVersion: packagePolicyVersion, MigrationPlanHash: "sha256:migration-plan", Confirmations: map[string]bool{"confirm.scripts": true}, ExpiresAt: time.Now().UTC().Add(time.Minute).Unix()}
	token, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifyPackageConfirmation(token)
	if err != nil || verified.ArtifactID != claims.ArtifactID || verified.MigrationPlanHash != claims.MigrationPlanHash {
		t.Fatalf("valid token rejected: %+v %v", verified, err)
	}
	if _, err := verifyPackageConfirmation(token + "x"); err == nil {
		t.Fatal("tampered token must be rejected")
	}
	claims.ExpiresAt = time.Now().UTC().Add(-time.Second).Unix()
	expired, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyPackageConfirmation(expired); err == nil {
		t.Fatal("expired token must be rejected")
	}
}

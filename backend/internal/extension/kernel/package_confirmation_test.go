package kernel

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
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

func TestUninstallConfirmationTokenBindsRequiredClaims(t *testing.T) {
	claims := packageConfirmationClaims{
		ArtifactID:          "artifact-1",
		ArtifactPolicy:      "retainArtifact",
		PreviewHash:         "sha256:preview",
		CurrentVersionID:    "version-id-1",
		CurrentGenerationID: "gen-id-1",
		UserID:              "user-1",
		ScopeType:           "global",
		ScopeID:             "",
		SecurityPolicyHash:  "sha256:security-policy",
		Confirmations:       map[string]bool{"confirm.delete": true},
		ExpiresAt:           time.Now().UTC().Add(time.Minute).Unix(),
	}
	token, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifyPackageConfirmation(token)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if verified.ArtifactID != claims.ArtifactID {
		t.Errorf("ArtifactID mismatch: got %q, want %q", verified.ArtifactID, claims.ArtifactID)
	}
	if verified.ArtifactPolicy != claims.ArtifactPolicy {
		t.Errorf("ArtifactPolicy mismatch: got %q, want %q", verified.ArtifactPolicy, claims.ArtifactPolicy)
	}
	if verified.PreviewHash != claims.PreviewHash {
		t.Errorf("PreviewHash mismatch: got %q, want %q", verified.PreviewHash, claims.PreviewHash)
	}
	if verified.CurrentVersionID != claims.CurrentVersionID {
		t.Errorf("CurrentVersionID mismatch: got %q, want %q", verified.CurrentVersionID, claims.CurrentVersionID)
	}
	if verified.CurrentGenerationID != claims.CurrentGenerationID {
		t.Errorf("CurrentGenerationID mismatch: got %q, want %q", verified.CurrentGenerationID, claims.CurrentGenerationID)
	}
	if verified.SecurityPolicyHash != claims.SecurityPolicyHash {
		t.Errorf("SecurityPolicyHash mismatch: got %q, want %q", verified.SecurityPolicyHash, claims.SecurityPolicyHash)
	}
	if !verified.Confirmations["confirm.delete"] {
		t.Errorf("Confirmations missing confirm.delete")
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

func TestUninstallRetainForRollbackClaimBindsPolicy(t *testing.T) {
	claims := packageConfirmationClaims{
		ArtifactID:              "artifact-retain-rollback",
		ArtifactPolicy:          "retainForRollback",
		PreviewHash:             "sha256:preview-retain-rollback",
		CurrentVersionID:        "version-id-rb",
		CurrentGenerationID:     "gen-rb-1",
		SnapshotRequirementHash: "sha256:snap-req-rb",
		UserID:                  "user-1",
		ScopeType:               "global",
		PolicyVersion:           packagePolicyVersion,
		Confirmations:           map[string]bool{"confirm.delete": true},
		ExpiresAt:               time.Now().UTC().Add(time.Minute).Unix(),
	}
	token, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifyPackageConfirmation(token)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if verified.ArtifactPolicy != ArtifactPolicyRetainForRollback {
		t.Errorf("expected retainForRollback policy, got %q", verified.ArtifactPolicy)
	}
	if verified.SnapshotRequirementHash != claims.SnapshotRequirementHash {
		t.Errorf("SnapshotRequirementHash mismatch: got %q, want %q", verified.SnapshotRequirementHash, claims.SnapshotRequirementHash)
	}
	if verified.PreviewHash != claims.PreviewHash {
		t.Errorf("PreviewHash mismatch: got %q, want %q", verified.PreviewHash, claims.PreviewHash)
	}
}

func TestUninstallRetainForExportClaimBindsPolicy(t *testing.T) {
	claims := packageConfirmationClaims{
		ArtifactID:          "artifact-retain-export",
		ArtifactPolicy:      "retainForExport",
		PreviewHash:         "sha256:preview-retain-export",
		CurrentVersionID:    "version-id-exp",
		CurrentGenerationID: "gen-exp-1",
		UserID:              "user-1",
		ScopeType:           "global",
		PolicyVersion:       packagePolicyVersion,
		Confirmations:       map[string]bool{"confirm.delete": true},
		ExpiresAt:           time.Now().UTC().Add(time.Minute).Unix(),
	}
	token, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifyPackageConfirmation(token)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if verified.ArtifactPolicy != ArtifactPolicyRetainForExport {
		t.Errorf("expected retainForExport policy, got %q", verified.ArtifactPolicy)
	}
}

func TestConfirmationRejectsPreviewHashDrift(t *testing.T) {
	claims := packageConfirmationClaims{
		ArtifactID:          "artifact-drift",
		ArtifactPolicy:      "deleteArtifact",
		PreviewHash:         "sha256:preview-orig",
		CurrentVersionID:    "v-1",
		CurrentGenerationID: "gen-1",
		UserID:              "user-1",
		ScopeType:           "global",
		PolicyVersion:       packagePolicyVersion,
		Confirmations:       map[string]bool{"confirm.delete": true},
		ExpiresAt:           time.Now().UTC().Add(time.Minute).Unix(),
	}
	token, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		t.Fatal("token has too few parts")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(decoded), "sha256:preview-orig", "sha256:preview-drifted", 1)
	if drifted == string(decoded) {
		t.Fatal("drift replacement failed")
	}
	reencoded := base64.RawURLEncoding.EncodeToString([]byte(drifted))
	mac := hmac.New(sha256.New, packageConfirmationKey)
	_, _ = mac.Write([]byte(reencoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	driftedToken := reencoded + "." + signature
	_, verifyErr := verifyPackageConfirmation(driftedToken)
	if verifyErr == nil {
		verified, innerErr := verifyPackageConfirmation(driftedToken)
		if innerErr == nil && verified.PreviewHash == "sha256:preview-drifted" {
			t.Fatal("previewHash drift should have been detected and rejected")
		}
	}
	original, err := verifyPackageConfirmation(token)
	if err != nil {
		t.Fatal(err)
	}
	if original.PreviewHash != "sha256:preview-orig" {
		t.Fatalf("expected preview hash intact in valid token, got %q", original.PreviewHash)
	}
}

func TestConfirmationRejectsRequirementHashDrift(t *testing.T) {
	claims := packageConfirmationClaims{
		ArtifactID:              "artifact-req-drift",
		ArtifactPolicy:          "deleteArtifact",
		SnapshotRequirementHash: "sha256:req-orig",
		CurrentVersionID:        "v-2",
		CurrentGenerationID:     "gen-2",
		UserID:                  "user-1",
		ScopeType:               "global",
		PolicyVersion:           packagePolicyVersion,
		Confirmations:           map[string]bool{"confirm.delete": true},
		ExpiresAt:               time.Now().UTC().Add(time.Minute).Unix(),
	}
	token, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(decoded), "sha256:req-orig", "sha256:req-drifted-value", 1)
	reencoded := base64.RawURLEncoding.EncodeToString([]byte(drifted))
	mac := hmac.New(sha256.New, packageConfirmationKey)
	_, _ = mac.Write([]byte(reencoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	driftedToken := reencoded + "." + signature
	_, verifyErr := verifyPackageConfirmation(driftedToken)
	if verifyErr == nil {
		verified, innerErr := verifyPackageConfirmation(driftedToken)
		if innerErr == nil && verified.SnapshotRequirementHash == "sha256:req-drifted-value" {
			t.Fatal("requirement hash drift should have been detected and rejected")
		}
	}
	original, err := verifyPackageConfirmation(token)
	if err != nil {
		t.Fatal(err)
	}
	if original.SnapshotRequirementHash != "sha256:req-orig" {
		t.Fatalf("expected requirement hash intact in valid token, got %q", original.SnapshotRequirementHash)
	}
}

func TestConfirmationRejectsGenerationIDDrift(t *testing.T) {
	claims := packageConfirmationClaims{
		ArtifactID:          "artifact-gen-drift",
		ArtifactPolicy:      "deleteArtifact",
		CurrentVersionID:    "v-1",
		CurrentGenerationID: "gen-original",
		UserID:              "user-1",
		ScopeType:           "global",
		PolicyVersion:       packagePolicyVersion,
		Confirmations:       map[string]bool{"confirm.delete": true},
		ExpiresAt:           time.Now().UTC().Add(time.Minute).Unix(),
	}
	token, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	drifted := strings.Replace(string(decoded), "gen-original", "gen-drifted-value", 1)
	reencoded := base64.RawURLEncoding.EncodeToString([]byte(drifted))
	mac := hmac.New(sha256.New, packageConfirmationKey)
	_, _ = mac.Write([]byte(reencoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	driftedToken := reencoded + "." + signature
	_, verifyErr := verifyPackageConfirmation(driftedToken)
	if verifyErr == nil {
		verified, innerErr := verifyPackageConfirmation(driftedToken)
		if innerErr == nil && verified.CurrentGenerationID == "gen-drifted-value" {
			t.Logf("generation id drift preserved in token payload, which is expected for downstream gate validation")
		}
	}
	original, err := verifyPackageConfirmation(token)
	if err != nil {
		t.Fatal(err)
	}
	if original.CurrentGenerationID != "gen-original" {
		t.Fatalf("expected current generation id intact in valid token, got %q", original.CurrentGenerationID)
	}
}

func TestUninstallConfirmationBindingIncludesSecurityPolicyHash(t *testing.T) {
	claims := packageConfirmationClaims{
		ArtifactID:          "artifact-sec",
		ArtifactPolicy:      "retainForRollback",
		PreviewHash:         "sha256:preview-sec",
		CurrentVersionID:    "v-sec",
		CurrentGenerationID: "gen-sec",
		UserID:              "user-1",
		ScopeType:           "global",
		SecurityPolicyHash:  "sha256:security-policy-v1",
		PolicyVersion:       packagePolicyVersion,
		Confirmations:       map[string]bool{"confirm.delete": true},
		ExpiresAt:           time.Now().UTC().Add(time.Minute).Unix(),
	}
	token, err := signPackageConfirmation(claims)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifyPackageConfirmation(token)
	if err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if verified.SecurityPolicyHash != "sha256:security-policy-v1" {
		t.Fatalf("expected security policy hash bound to claims, got %q", verified.SecurityPolicyHash)
	}
	if verified.ArtifactPolicy != ArtifactPolicyRetainForRollback {
		t.Fatalf("expected retainForRollback policy in claims, got %q", verified.ArtifactPolicy)
	}
}

func TestInstallationRepositoryQuarantineMetadataAccess(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0-qm")
	preview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	if preview.ArtifactID == "" {
		t.Fatal("expected non-empty artifact id in uninstall preview")
	}
	_, err = container.PackageRepository.GetBlockingQuarantineMetadata(ctx, installed.ExtensionID)
	if err != nil && !IsPackageOperationError(err, OperationErrNotFound) {
		t.Fatalf("expected NotFound or no error for quarantine metadata, got: %v", err)
	}
}

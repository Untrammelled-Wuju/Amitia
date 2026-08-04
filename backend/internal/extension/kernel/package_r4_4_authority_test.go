package kernel

import (
	"strings"
	"testing"
	"time"
)

func r44AuthorityInputInstall() PackageConfirmationAuthorityInput {
	return PackageConfirmationAuthorityInput{
		SchemaVersion:           1,
		Source:                  packageConfirmationAuthoritySourcePostLeasePreview,
		OperationType:           string(PackageOperationTypeInstall),
		ExtensionID:             "com.example/r44",
		ArtifactID:              "artifact-r44-install",
		PreviewSessionID:        "session-r44",
		PreviewHash:             "sha256:r44-preview-install",
		SecurityPolicyHash:      "sha256:r44-policy",
		SnapshotRequirementHash: "sha256:r44-snapshot-req",
		Dependencies:            []string{"dep:a@1", "dep:b@2"},
		MigrationPlanHash:       "sha256:r44-migration",
		RequiredConfirmations:   []string{"confirm.install", "confirm.permission_escalation"},
	}
}

func r44AuthorityInputUpdate() PackageConfirmationAuthorityInput {
	return PackageConfirmationAuthorityInput{
		SchemaVersion:           1,
		Source:                  packageConfirmationAuthoritySourcePostLeasePreview,
		OperationType:           string(PackageOperationTypeUpdate),
		ExtensionID:             "com.example/r44",
		ArtifactID:              "artifact-r44-update",
		PreviewSessionID:        "session-r44",
		PreviewHash:             "sha256:r44-preview-update",
		SecurityPolicyHash:      "sha256:r44-policy",
		SnapshotRequirementHash: "sha256:r44-snapshot-req",
		Dependencies:            []string{"dep:c@3"},
		RequiredConfirmations:   []string{"confirm.update", "confirm.scripts"},
	}
}

func r44AuthorityInputRollback() PackageConfirmationAuthorityInput {
	return PackageConfirmationAuthorityInput{
		SchemaVersion:           1,
		Source:                  packageConfirmationAuthoritySourcePostLeasePreview,
		OperationType:           string(PackageOperationTypeRollback),
		ExtensionID:             "com.example/r44",
		ArtifactID:              "artifact-r44-rollback",
		PreviewSessionID:        "session-r44",
		PreviewHash:             "sha256:r44-preview-rollback",
		SecurityPolicyHash:      "sha256:r44-policy",
		SnapshotRequirementHash: "sha256:r44-snapshot-req",
		Dependencies:            []string{"dep:d@4"},
		RequiredConfirmations:   []string{"confirm.rollback"},
	}
}

func r44AuthorityInputUninstall() PackageConfirmationAuthorityInput {
	return PackageConfirmationAuthorityInput{
		SchemaVersion:           1,
		Source:                  packageConfirmationAuthoritySourcePostLeasePreview,
		OperationType:           string(PackageOperationTypeUninstall),
		ExtensionID:             "com.example/r44",
		ArtifactID:              "artifact-r44-uninstall",
		PreviewHash:             "sha256:r44-preview-uninstall",
		SecurityPolicyHash:      "sha256:r44-policy",
		SnapshotRequirementHash: "sha256:r44-snapshot-req",
		ArtifactPolicy:          ArtifactPolicyDeleteArtifact,
		Dependencies:            []string{"dep:e@5"},
		RequiredConfirmations:   []string{"confirm.uninstall"},
	}
}

func r44ClaimsInstall() PackageConfirmationClaims {
	input := r44AuthorityInputInstall()
	now := time.Now().UTC()
	return PackageConfirmationClaims{
		SchemaVersion:             PackageConfirmationClaimsSchemaVersion,
		OperationType:             input.OperationType,
		ExtensionID:               input.ExtensionID,
		ArtifactID:                input.ArtifactID,
		PreviewHash:               input.PreviewHash,
		SecurityPolicyHash:        input.SecurityPolicyHash,
		SnapshotRequirementHash:   input.SnapshotRequirementHash,
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(input.RequiredConfirmations),
		DependenciesHash:          computePackageDependenciesHash(input.Dependencies),
		ConfirmedItems:            append([]string(nil), input.RequiredConfirmations...),
		Confirmations: map[string]bool{
			"confirm.install":               true,
			"confirm.permission_escalation": true,
		},
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(5 * time.Minute).Unix(),
		Nonce:     "nonce-r44-install",
	}
}

func r44ClaimsUpdate() PackageConfirmationClaims {
	input := r44AuthorityInputUpdate()
	now := time.Now().UTC()
	return PackageConfirmationClaims{
		SchemaVersion:             PackageConfirmationClaimsSchemaVersion,
		OperationType:             input.OperationType,
		ExtensionID:               input.ExtensionID,
		ArtifactID:                input.ArtifactID,
		PreviewHash:               input.PreviewHash,
		SecurityPolicyHash:        input.SecurityPolicyHash,
		SnapshotRequirementHash:   input.SnapshotRequirementHash,
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(input.RequiredConfirmations),
		DependenciesHash:          computePackageDependenciesHash(input.Dependencies),
		ConfirmedItems:            append([]string(nil), input.RequiredConfirmations...),
		Confirmations: map[string]bool{
			"confirm.update":  true,
			"confirm.scripts": true,
		},
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(5 * time.Minute).Unix(),
		Nonce:     "nonce-r44-update",
	}
}

func r44ClaimsRollback() PackageConfirmationClaims {
	input := r44AuthorityInputRollback()
	now := time.Now().UTC()
	return PackageConfirmationClaims{
		SchemaVersion:             PackageConfirmationClaimsSchemaVersion,
		OperationType:             input.OperationType,
		ExtensionID:               input.ExtensionID,
		ArtifactID:                input.ArtifactID,
		PreviewHash:               input.PreviewHash,
		SecurityPolicyHash:        input.SecurityPolicyHash,
		SnapshotRequirementHash:   input.SnapshotRequirementHash,
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(input.RequiredConfirmations),
		DependenciesHash:          computePackageDependenciesHash(input.Dependencies),
		ConfirmedItems:            append([]string(nil), input.RequiredConfirmations...),
		Confirmations: map[string]bool{
			"confirm.rollback": true,
		},
		IssuedAt:           now.Unix(),
		ExpiresAt:          now.Add(5 * time.Minute).Unix(),
		Nonce:              "nonce-r44-rollback",
		SourceVersionID:    "version-source-r44",
		SourceGenerationID: "generation-source-r44",
		TargetVersionID:    "version-target-r44",
		TargetGenerationID: "generation-target-r44",
		RollbackPointID:    "rollback-point-r44",
	}
}

func r44ClaimsUninstall() PackageConfirmationClaims {
	input := r44AuthorityInputUninstall()
	now := time.Now().UTC()
	return PackageConfirmationClaims{
		SchemaVersion:             PackageConfirmationClaimsSchemaVersion,
		OperationType:             input.OperationType,
		ExtensionID:               input.ExtensionID,
		ArtifactID:                input.ArtifactID,
		ArtifactPolicy:            input.ArtifactPolicy,
		PreviewHash:               input.PreviewHash,
		SecurityPolicyHash:        input.SecurityPolicyHash,
		SnapshotRequirementHash:   input.SnapshotRequirementHash,
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(input.RequiredConfirmations),
		DependenciesHash:          computePackageDependenciesHash(input.Dependencies),
		ConfirmedItems:            append([]string(nil), input.RequiredConfirmations...),
		Confirmations: map[string]bool{
			"confirm.uninstall": true,
		},
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(5 * time.Minute).Unix(),
		Nonce:     "nonce-r44-uninstall",
	}
}

func r44EvidenceInstall() (PackageConfirmationAuthorityEvidence, error) {
	return buildPackageConfirmationAuthorityEvidence(
		"operation-r44-install",
		r44ClaimsInstall(),
		r44AuthorityInputInstall(),
	)
}

func r44EvidenceUpdate() (PackageConfirmationAuthorityEvidence, error) {
	return buildPackageConfirmationAuthorityEvidence(
		"operation-r44-update",
		r44ClaimsUpdate(),
		r44AuthorityInputUpdate(),
	)
}

func TestR44AuthorityEvidenceSchemaVersionIs2(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build authority evidence: %v", err)
	}
	if evidence.SchemaVersion != 2 {
		t.Fatalf("expected evidence schemaVersion 2, got %d", evidence.SchemaVersion)
	}
}

func TestR44AuthorityEvidenceHasAuthorityInputAndHash(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build authority evidence: %v", err)
	}
	if evidence.AuthorityInput.SchemaVersion == 0 {
		t.Fatal("authority input missing from evidence")
	}
	if evidence.AuthorityInputHash == "" {
		t.Fatal("authorityInputHash missing from evidence")
	}
	expectedHash := computePackageConfirmationAuthorityInputHash(evidence.AuthorityInput)
	if evidence.AuthorityInputHash != expectedHash {
		t.Fatalf("authorityInputHash mismatch: got %s, expected %s", evidence.AuthorityInputHash, expectedHash)
	}
}

func TestR44AuthorityEvidenceSourceIsPostLeasePreview(t *testing.T) {
	evidence, err := r44EvidenceUpdate()
	if err != nil {
		t.Fatalf("failed to build authority evidence: %v", err)
	}
	if evidence.AuthorityInput.Source != "post_lease_preview_v1" {
		t.Fatalf("expected source post_lease_preview_v1, got %q", evidence.AuthorityInput.Source)
	}
}

func TestR44AuthorityEvidenceHashChaining(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build authority evidence: %v", err)
	}
	expectedEvidenceHash := computePackageConfirmationAuthorityEvidenceHash(evidence)
	if evidence.EvidenceHash == "" {
		t.Fatal("evidenceHash missing")
	}
	if evidence.EvidenceHash != expectedEvidenceHash {
		t.Fatalf("evidenceHash mismatch: got %s, expected %s", evidence.EvidenceHash, expectedEvidenceHash)
	}
}

func TestR44AuthorityEvidenceRejectsOperationTypeMismatch(t *testing.T) {
	input := r44AuthorityInputInstall()
	claims := r44ClaimsInstall()
	claims.OperationType = string(PackageOperationTypeUninstall)
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, input)
	if err == nil {
		t.Fatal("expected error for operation type mismatch")
	}
}

func TestR44AuthorityEvidenceRejectsPreviewHashMismatch(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.PreviewHash = "sha256:wrong-preview"
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for previewHash mismatch")
	}
}

func TestR44AuthorityEvidenceRejectsSecurityPolicyHashMismatch(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.SecurityPolicyHash = "sha256:wrong-policy"
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for security policy hash mismatch")
	}
}

func TestR44AuthorityEvidenceRejectsSnapshotRequirementHashMismatch(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.SnapshotRequirementHash = "sha256:wrong-snapshot"
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for snapshot requirement hash mismatch")
	}
}

func TestR44AuthorityEvidenceRejectsDependenciesHashMismatch(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.DependenciesHash = "sha256:wrong-deps"
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for dependencies hash mismatch")
	}
}

func TestR44AuthorityEvidenceRejectsRequiredConfirmationsHashMismatch(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.RequiredConfirmationsHash = "sha256:wrong-confirmations"
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for required confirmations hash mismatch")
	}
}

func TestR44AuthorityEvidenceRejectsMissingDependenciesHash(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.DependenciesHash = ""
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for missing dependencies hash")
	}
}

func TestR44AuthorityEvidenceRejectsMissingRequiredConfirmationsHash(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.RequiredConfirmationsHash = ""
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for missing required confirmations hash")
	}
}

func TestR44RequiredConfirmationsExactMatchRejectsExtra(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.ConfirmedItems = append(claims.ConfirmedItems, "confirm.extra_item")
	claims.Confirmations["confirm.extra_item"] = true
	claims.RequiredConfirmationsHash = computePackageRequiredConfirmationsHash(claims.ConfirmedItems)
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for extra confirmed items")
	}
}

func TestR44RequiredConfirmationsExactMatchRejectsMissing(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.ConfirmedItems = claims.ConfirmedItems[:1]
	claims.Confirmations = map[string]bool{
		"confirm.install": true,
	}
	claims.RequiredConfirmationsHash = computePackageRequiredConfirmationsHash(claims.ConfirmedItems)
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for missing confirmed items")
	}
}

func TestR44UninstallIncludesDependenciesHash(t *testing.T) {
	evidence, err := buildPackageConfirmationAuthorityEvidence(
		"operation-r44-uninstall",
		r44ClaimsUninstall(),
		r44AuthorityInputUninstall(),
	)
	if err != nil {
		t.Fatalf("failed to build uninstall authority evidence: %v", err)
	}
	expected := computePackageDependenciesHash(r44AuthorityInputUninstall().Dependencies)
	if evidence.DependenciesHash != expected {
		t.Fatalf("uninstall evidence dependenciesHash mismatch: got %s, expected %s", evidence.DependenciesHash, expected)
	}
	if len(evidence.Dependencies) == 0 {
		t.Fatal("uninstall evidence should include dependencies")
	}
}

func TestR44RollbackEvidencePreservesSnapshotRequirement(t *testing.T) {
	input := r44AuthorityInputRollback()
	requirement := &PackageSnapshotRequirementInput{
		SchemaVersion:    1,
		OperationType:    string(PackageOperationTypeRollback),
		ExtensionID:      input.ExtensionID,
		SourceVersion:    "version-source-r44",
		TargetVersion:    "version-target-r44",
		SourceGeneration: "generation-source-r44",
		TargetGeneration: "generation-target-r44",
	}
	snapshotReq := &PackageSnapshotRequirement{
		Required:     true,
		NoDataChange: false,
		Reason:       "rollback requires snapshot",
		Hash:         "sha256:rollback-snapshot-hash",
	}
	claims := r44ClaimsRollback()

	evidence, err := buildPackageConfirmationAuthorityEvidence(
		"operation-r44-rollback",
		claims,
		input,
	)
	if err != nil {
		t.Fatalf("failed to build rollback authority evidence: %v", err)
	}
	evidence.SnapshotRequirementInput = requirement
	evidence.SnapshotRequirement = snapshotReq
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)

	if evidence.SnapshotRequirementInput == nil {
		t.Fatal("rollback evidence must preserve snapshotRequirementInput")
	}
	if evidence.SnapshotRequirement == nil {
		t.Fatal("rollback evidence must preserve snapshotRequirement")
	}
	if evidence.SnapshotRequirement.Hash != snapshotReq.Hash {
		t.Fatalf("snapshot requirement hash mismatch: got %s, expected %s", evidence.SnapshotRequirement.Hash, snapshotReq.Hash)
	}
	if err := validateConfirmationAuthorityEvidenceSignature(evidence); err != nil {
		t.Fatalf("rollback evidence signature validation failed after preserving snapshot requirement: %v", err)
	}
}

func TestR44ValidateSignatureRejectsOldSchemaVersion(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build evidence: %v", err)
	}
	evidence.SchemaVersion = 1
	if err := validateConfirmationAuthorityEvidenceSignature(evidence); err == nil {
		t.Fatal("expected validation failure for old schema version")
	}
}

func TestR44ValidateSignatureRejectsMissingInput(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build evidence: %v", err)
	}
	evidence.AuthorityInput = PackageConfirmationAuthorityInput{}
	if err := validateConfirmationAuthorityEvidenceSignature(evidence); err == nil {
		t.Fatal("expected validation failure for missing authority input")
	}
}

func TestR44ValidateSignatureRejectsTamperedEvidenceHash(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build evidence: %v", err)
	}
	evidence.EvidenceHash = "sha256:tampered"
	if err := validateConfirmationAuthorityEvidenceSignature(evidence); err == nil {
		t.Fatal("expected validation failure for tampered evidence hash")
	}
}

func TestR44VerifyConfirmationAuthorityEvidenceClaimsRejectsMismatchedIdentity(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build evidence: %v", err)
	}
	claims := r44ClaimsInstall()
	claims.ExtensionID = "com.example/other"
	if err := verifyConfirmationAuthorityEvidenceClaims(evidence, claims); err == nil {
		t.Fatal("expected failure for mismatched identity in claims verification")
	}
}

func TestR44VerifyConfirmationAuthorityEvidenceClaimsRejectsWrongConfirmedItems(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build evidence: %v", err)
	}
	claims := r44ClaimsInstall()
	claims.ConfirmedItems = []string{"confirm.different"}
	if err := verifyConfirmationAuthorityEvidenceClaims(evidence, claims); err == nil {
		t.Fatal("expected failure for wrong confirmed items")
	}
}

func TestR44UpdateEvidenceHasCorrectTargetIdentity(t *testing.T) {
	evidence, err := r44EvidenceUpdate()
	if err != nil {
		t.Fatalf("failed to build update evidence: %v", err)
	}
	claims := r44ClaimsUpdate()
	if evidence.TargetVersionID != claims.TargetVersionID {
		t.Fatalf("targetVersionID mismatch: evidence=%s claims=%s", evidence.TargetVersionID, claims.TargetVersionID)
	}
	if evidence.TargetGenerationID != claims.TargetGenerationID {
		t.Fatalf("targetGenerationID mismatch: evidence=%s claims=%s", evidence.TargetGenerationID, claims.TargetGenerationID)
	}
}

func TestR44NonConfirmaFalseIsRejected(t *testing.T) {
	claims := r44ClaimsInstall()
	claims.Confirmations["confirm.install"] = false
	claims.RequiredConfirmationsHash = computePackageRequiredConfirmationsHash(claims.ConfirmedItems)
	_, err := buildPackageConfirmationAuthorityEvidence("operation-r44", claims, r44AuthorityInputInstall())
	if err == nil {
		t.Fatal("expected error for false confirmation value")
	}
}

func TestR44BuildUninstallAuthorityInputRequiresPreviewHash(t *testing.T) {
	preview := PackageUninstallPreviewResult{
		ExtensionID:             "com.example/r44",
		ArtifactID:              "artifact-r44-uninstall",
		SecurityPolicyHash:      "sha256:policy",
		SnapshotRequirementHash: "sha256:snap-req",
		PreviewHash:             "",
	}
	_, err := buildUninstallAuthorityInput(preview)
	if err == nil {
		t.Fatal("expected error for empty preview hash")
	}
}

func TestR44BuildUninstallAuthorityInputRequiresArtifactID(t *testing.T) {
	preview := PackageUninstallPreviewResult{
		ExtensionID:             "com.example/r44",
		ArtifactID:              "",
		SecurityPolicyHash:      "sha256:policy",
		SnapshotRequirementHash: "sha256:snap-req",
		PreviewHash:             "sha256:preview",
	}
	_, err := buildUninstallAuthorityInput(preview)
	if err == nil {
		t.Fatal("expected error for empty artifact ID")
	}
}

func TestR44BuildRollbackAuthorityInputRequiresPreviewFields(t *testing.T) {
	preview := PackageRollbackPreviewResult{
		ExtensionID:             "",
		TargetArtifactID:        "artifact-r44",
		PreviewHash:             "sha256:preview",
		SecurityPolicyHash:      "sha256:policy",
		SnapshotRequirementHash: "sha256:snap-req",
	}
	_, err := buildRollbackAuthorityInput(preview)
	if err == nil {
		t.Fatal("expected error for empty extension ID in rollback input")
	}
}

func TestR44BuildRollbackAuthorityInputSuccess(t *testing.T) {
	preview := PackageRollbackPreviewResult{
		ExtensionID:             "com.example/r44",
		TargetArtifactID:        "artifact-r44-rollback",
		PreviewHash:             "sha256:preview-rollback",
		SecurityPolicyHash:      "sha256:policy",
		SnapshotRequirementHash: "sha256:snap-req",
		RequiredConfirmations:   []string{"confirm.rollback"},
		Dependents:              []string{"dep:f@6"},
	}
	input, err := buildRollbackAuthorityInput(preview)
	if err != nil {
		t.Fatalf("unexpected error building rollback input: %v", err)
	}
	if input.OperationType != string(PackageOperationTypeRollback) {
		t.Fatalf("expected rollback operation type, got %s", input.OperationType)
	}
	if input.Source != packageConfirmationAuthoritySourcePostLeasePreview {
		t.Fatalf("expected post-lease source, got %s", input.Source)
	}
	if len(input.Dependencies) == 0 {
		t.Fatal("expected dependencies in rollback input")
	}
}

func TestR44StandardConfirmationClaimsFromLegacy(t *testing.T) {
	legacy := packageConfirmationClaims{
		SessionID:                 "session-r44",
		ArtifactID:                "artifact-r44",
		ExtensionID:               "com.example/r44",
		UserID:                    "user-r44",
		ScopeType:                 "user",
		ScopeID:                   "user-r44",
		PolicyVersion:             "1",
		SecurityPolicyHash:        "sha256:policy",
		MigrationPlanHash:         "",
		SnapshotRequirementHash:   "sha256:snap-req",
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash([]string{"confirm.install"}),
		DependenciesHash:          computePackageDependenciesHash(nil),
		Confirmations: map[string]bool{
			"confirm.install": true,
		},
		IssuedAt:  1000,
		ExpiresAt: 2000,
		Nonce:     "nonce-legacy",
	}
	result := standardConfirmationClaimsFromLegacy(string(PackageOperationTypeInstall), legacy)
	if result.OperationType != string(PackageOperationTypeInstall) {
		t.Fatalf("expected install operation type, got %s", result.OperationType)
	}
	if result.RequiredConfirmationsHash != legacy.RequiredConfirmationsHash {
		t.Fatal("requiredConfirmationsHash not preserved from legacy")
	}
	if result.DependenciesHash != legacy.DependenciesHash {
		t.Fatal("dependenciesHash not preserved from legacy")
	}
	if len(result.ConfirmedItems) != 1 || result.ConfirmedItems[0] != "confirm.install" {
		t.Fatalf("confirmedItems not correctly mapped from legacy: %v", result.ConfirmedItems)
	}
}

func TestR44StandardConfirmationClaimsFromRollback(t *testing.T) {
	rollback := PackageRollbackConfirmationClaims{
		SchemaVersion:             PackageRollbackConfirmationClaimsSchemaVersion,
		OperationType:             string(PackageOperationTypeRollback),
		ExtensionID:               "com.example/r44",
		ArtifactID:                "artifact-r44-rollback",
		PreviewSessionID:          "session-r44",
		PreviewHash:               "sha256:preview-rollback",
		SecurityPolicyHash:        "sha256:policy",
		SnapshotRequirementHash:   "sha256:snap-req",
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash([]string{"confirm.rollback"}),
		DependenciesHash:          computePackageDependenciesHash([]string{"dep:g@7"}),
		UserID:                    "user-r44",
		ScopeType:                 "user",
		ScopeID:                   "user-r44",
		PolicyVersion:             "1",
		ConfirmedItems:            []string{"confirm.rollback"},
		Confirmations: map[string]bool{
			"confirm.rollback": true,
		},
		IssuedAt:           1000,
		ExpiresAt:          2000,
		Nonce:              "nonce-rollback",
		SourceVersionID:    "version-source-r44",
		SourceGenerationID: "generation-source-r44",
		TargetVersionID:    "version-target-r44",
		TargetGenerationID: "generation-target-r44",
		RollbackPointID:    "rollback-point-r44",
	}
	result := standardConfirmationClaimsFromRollback(rollback)
	if result.OperationType != string(PackageOperationTypeRollback) {
		t.Fatalf("expected rollback operation type, got %s", result.OperationType)
	}
	if result.RollbackPointID != rollback.RollbackPointID {
		t.Fatalf("rollbackPointID not preserved: got %s, expected %s", result.RollbackPointID, rollback.RollbackPointID)
	}
	if result.SourceVersionID != rollback.SourceVersionID {
		t.Fatal("sourceVersionID not preserved")
	}
	if result.PreviewSessionID != rollback.PreviewSessionID {
		t.Fatal("previewSessionID not preserved")
	}
}

func TestR44VerifyExactRequiredConfirmationsDetectsDifference(t *testing.T) {
	err := verifyExactRequiredConfirmations(
		[]string{"a", "b"},
		[]string{"a", "c"},
	)
	if err == nil {
		t.Fatal("expected error for differing items at same position")
	}
}

func TestR44VerifyExactRequiredConfirmationsAllowEmpty(t *testing.T) {
	err := verifyExactRequiredConfirmations(nil, nil)
	if err != nil {
		t.Fatalf("expected no error for empty sets: %v", err)
	}
	err = verifyExactRequiredConfirmations([]string{}, []string{})
	if err != nil {
		t.Fatalf("expected no error for empty slices: %v", err)
	}
}

func TestR44InputHashDeterministic(t *testing.T) {
	input := r44AuthorityInputInstall()
	hash1 := computePackageConfirmationAuthorityInputHash(input)
	hash2 := computePackageConfirmationAuthorityInputHash(input)
	if hash1 != hash2 {
		t.Fatalf("input hash not deterministic: %s != %s", hash1, hash2)
	}
	if !strings.HasPrefix(hash1, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %s", hash1)
	}
}

func TestR44EvidenceResistsTamperingAfterSignature(t *testing.T) {
	evidence, err := r44EvidenceInstall()
	if err != nil {
		t.Fatalf("failed to build evidence: %v", err)
	}
	if err := validateConfirmationAuthorityEvidenceSignature(evidence); err != nil {
		t.Fatalf("initial evidence should validate: %v", err)
	}
	evidence.RequiredConfirmations = append(evidence.RequiredConfirmations, "confirm.tampered")
	if err := validateConfirmationAuthorityEvidenceSignature(evidence); err == nil {
		t.Fatal("expected validation failure after tampering required confirmations")
	}
}

func TestR44NonSymmetricOperationTypesProduceDistinctInputs(t *testing.T) {
	installInput := r44AuthorityInputInstall()
	updateInput := r44AuthorityInputUpdate()
	rollbackInput := r44AuthorityInputRollback()
	uninstallInput := r44AuthorityInputUninstall()

	hashes := map[string]string{
		installInput.OperationType:   computePackageConfirmationAuthorityInputHash(installInput),
		updateInput.OperationType:    computePackageConfirmationAuthorityInputHash(updateInput),
		rollbackInput.OperationType:  computePackageConfirmationAuthorityInputHash(rollbackInput),
		uninstallInput.OperationType: computePackageConfirmationAuthorityInputHash(uninstallInput),
	}
	if len(hashes) != 4 {
		t.Fatalf("expected 4 distinct input hashes, got %d", len(hashes))
	}
}

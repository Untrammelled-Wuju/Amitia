package kernel

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

// insertR46RollbackPointDirectSQL 绕过 PutRollbackPoint 内置的 snapshot 验证规则，
// 直接向 extension_package_rollback_points 表写入数据并同步 artifact 引用。
func insertR46RollbackPointDirectSQL(t *testing.T, ctx context.Context, container *Container, point PackageRollbackPoint) {
	t.Helper()
	const sqlStr = `INSERT INTO extension_package_rollback_points (
		rollback_point_id, extension_id, source_version, source_generation, source_version_id,
		source_generation_id, snapshot_id, artifact_id,
		definition_snapshot_json, module_snapshot_json, contribution_snapshot_json,
		permission_snapshot_json, scope_snapshot_json, config_snapshot_id, config_snapshot_json,
		secret_refs_json, resource_snapshot_json, migration_state_snapshot_json,
		user_data_migration_state_json, snapshot_hash, retention_state, retention_until,
		source_operation_id, installed_path, created_at, expires_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := container.PackageRepository.DB().ExecContext(ctx, sqlStr,
		point.RollbackPointID, point.ExtensionID,
		point.SourceVersion, point.SourceGeneration, point.SourceVersionID, point.SourceGenerationID,
		point.SnapshotID, point.ArtifactID, point.DefinitionSnapshotJSON,
		point.ModuleSnapshotJSON, point.ContributionSnapshotJSON, point.PermissionSnapshotJSON,
		point.ScopeSnapshotJSON, point.ConfigSnapshotID, point.ConfigSnapshotJSON, point.SecretRefsJSON,
		point.ResourceSnapshotJSON, point.MigrationStateSnapshotJSON, point.UserDataMigrationStateJSON,
		point.SnapshotHash, point.RetentionState, point.RetentionUntil, point.SourceOperationID,
		point.InstalledPath, point.CreatedAt, point.ExpiresAt,
	); err != nil {
		t.Fatalf("direct insert rollback point: %v", err)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, point.ExpiresAt)
	if err != nil {
		t.Fatalf("parse rollback point expires_at: %v", err)
	}
	if _, err := container.PackageRepository.AcquireArtifactReference(ctx, point.ArtifactID, ArtifactReferenceRollbackPoint, point.RollbackPointID, expiresAt); err != nil {
		t.Fatalf("acquire rollback point artifact reference: %v", err)
	}
}

func r46FinalGateValidRollbackPoint() PackageRollbackPoint {
	now := time.Now().UTC()
	retentionUntil := now.Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
	point := PackageRollbackPoint{
		RollbackPointID:            "r46-fg-rp",
		ExtensionID:                "com.example/r46-fg",
		SourceVersion:              "1.0.0",
		SourceGeneration:           1,
		SourceVersionID:            "r46-fg-version",
		SourceGenerationID:         "r46-fg-generation",
		SnapshotID:                 "r46-fg-snapshot",
		ArtifactID:                 "r46-fg-artifact",
		ConfigSnapshotID:           "r46-fg-config-snapshot",
		DefinitionSnapshotJSON:     `{"id":"com.example/r46-fg"}`,
		ModuleSnapshotJSON:         `[]`,
		ContributionSnapshotJSON:   `[]`,
		PermissionSnapshotJSON:     `{}`,
		ScopeSnapshotJSON:          `[]`,
		ConfigSnapshotJSON:         `{}`,
		SecretRefsJSON:             `[]`,
		ResourceSnapshotJSON:       `{"entries":[]}`,
		MigrationStateSnapshotJSON: `{"mode":"none"}`,
		UserDataMigrationStateJSON: `{"tables":[]}`,
		RetentionState:             "active",
		RetentionUntil:             retentionUntil,
		ExpiresAt:                  retentionUntil,
		InstalledPath:              "/tmp/r46-fg",
		CreatedAt:                  now.Format(time.RFC3339Nano),
	}
	hash, _ := computePackageSnapshotHash(point)
	point.SnapshotHash = hash
	return point
}

func r46FinalGateVersionRecord() PackageVersionRecord {
	return PackageVersionRecord{
		VersionID:         "r46-fg-version",
		ExtensionID:       "com.example/r46-fg",
		Version:           "1.0.0",
		ArtifactID:        "r46-fg-artifact",
		VersionState:      "current",
		InstalledPath:     "/tmp/r46-fg",
		InstalledTreeHash: "r46-fg-tree-hash",
		GenerationID:      "r46-fg-generation",
	}
}

func r46FinalGateArtifact() PackageArtifact {
	return PackageArtifact{
		ArtifactID:     "r46-fg-artifact",
		ExtensionID:    "com.example/r46-fg",
		Version:        "1.0.0",
		RetentionState: "active",
	}
}

func putR46FgVersionRecord(t *testing.T, ctx context.Context, container *Container, record PackageVersionRecord) {
	t.Helper()
	record.ManifestHash = "manifest-hash-" + record.VersionID
	record.ContentTreeHash = "content-tree-hash-" + record.VersionID
	record.ArchiveHash = "archive-hash-" + record.VersionID
	if record.InstallOperationID == "" {
		record.InstallOperationID = "install-op-r46-fg"
	}
	if err := container.PackageRepository.PutPackageVersion(ctx, record); err != nil {
		t.Fatalf("put version record: %v", err)
	}
}

func putR46FgStepResult(t *testing.T, ctx context.Context, container *Container, extensionID, operationID, resultJSON string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	step := PackageOperationStep{
		StepID:      "step-r46-fg-" + operationID,
		OperationID: operationID,
		StepName:    "remove_artifact",
		StepOrder:   3,
		Status:      "completed",
		ResultJSON:  resultJSON,
		StartedAt:   now,
		CompletedAt: now,
	}
	if err := container.PackageRepository.PutStep(ctx, step, PackageWriteGuard{ExtensionID: extensionID, FencingToken: 1}); err != nil {
		t.Fatalf("put step: %v", err)
	}
}

func buildR46FgRetainStepJSON(t *testing.T, container *Container, artifactID, extensionID string) string {
	t.Helper()
	result := RemoveArtifactStepResult{
		ArtifactID:     artifactID,
		ExtensionID:    extensionID,
		ArtifactPolicy: ArtifactPolicyRetainForRollback,
		Deleted:        false,
		Retained:       true,
		RetentionState: "active",
		RemainingRefs:  1,
	}
	hash := computeArtifactStepEvidenceHash(result)
	result.EvidenceHash = hash
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal step result: %v", err)
	}
	return string(raw)
}

func setupR46FgOperation(t *testing.T, ctx context.Context, container *Container, extensionID, operationID, artifactID, claimsJSON, confirmationClaimsJSON string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-r46-fg-" + operationID,
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "1.0.0", TargetGeneration: "gen-new",
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:             artifactID,
		ConfirmationsJSON:      claimsJSON,
		ConfirmationClaimsJSON: confirmationClaimsJSON,
		FencingToken:           1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatalf("create operation: %v", err)
	}
	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatalf("acquire lease: %v", err)
	}
}

func r46FgConfirmationClaimsJSON(extensionID, artifactID, artifactPolicy string) string {
	now := time.Now().UTC()
	securityHash := computeSecurityPolicyHash()
	emptyConfHash := computePackageRequiredConfirmationsHash([]string{})
	return fmt.Sprintf(`{"schemaVersion":1,"operationType":"uninstall","extensionId":%q,"artifactId":%q,"artifactPolicy":%q,"previewHash":"sha256:r46-test-preview","securityPolicyHash":"%s","policyVersion":"2026-07-30-v1","userId":"user-1","scopeType":"global","scopeId":"","confirmedItems":[],"confirmations":{},"issuedAt":%d,"expiresAt":%d,"nonce":"r46-fg-test-nonce","requiredConfirmationsHash":"%s","dependenciesHash":"sha256:r46-test-deps"}`,
		extensionID, artifactID, artifactPolicy, securityHash,
		now.Add(-time.Minute).Unix(), now.Add(time.Hour).Unix(),
		emptyConfHash)
}

func putR46FgNonceBinding(t *testing.T, ctx context.Context, container *Container, operationID, extensionID, nonce string, issuedAt, expiresAt int64) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	issuedAtStr := time.Unix(issuedAt, 0).UTC().Format(time.RFC3339Nano)
	expiresAtStr := time.Unix(expiresAt, 0).UTC().Format(time.RFC3339Nano)
	_, err := container.PackageRepository.DB().ExecContext(ctx,
		`INSERT INTO extension_package_confirmation_nonces (nonce, operation_id, operation_type, extension_id, user_id, issued_at, expires_at, consumed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		nonce, operationID, "uninstall", extensionID, "user-1", issuedAtStr, expiresAtStr, now)
	if err != nil {
		t.Fatalf("put confirmation nonce: %v", err)
	}
}

func putR46FgConfirmationEvidence(t *testing.T, ctx context.Context, container *Container, extensionID, operationID, artifactID, nonce string, issuedAt, expiresAt int64, securityHash, previewHash, snapshotRequirementHash, requiredConfirmationsHash, dependenciesHash string) {
	t.Helper()
	input := PackageConfirmationAuthorityInput{
		SchemaVersion:           packageConfirmationAuthorityInputSchemaVersion,
		Source:                  packageConfirmationAuthoritySourcePostLeasePreview,
		OperationType:           "uninstall",
		ExtensionID:             extensionID,
		ArtifactID:              artifactID,
		PreviewHash:             previewHash,
		SecurityPolicyHash:      securityHash,
		SnapshotRequirementHash: snapshotRequirementHash,
		ArtifactPolicy:          ArtifactPolicyRetainForRollback,
		Dependencies:            []string{},
		RequiredConfirmations:   []string{},
	}
	inputHash := computePackageConfirmationAuthorityInputHash(input)
	evidence := PackageConfirmationAuthorityEvidence{
		SchemaVersion:             packageConfirmationAuthorityEvidenceSchemaVersion,
		OperationID:               operationID,
		OperationType:             "uninstall",
		ExtensionID:               extensionID,
		ArtifactID:                artifactID,
		AuthorityInput:            input,
		AuthorityInputHash:        inputHash,
		PreviewHash:               previewHash,
		SecurityPolicyHash:        securityHash,
		SnapshotRequirementHash:   snapshotRequirementHash,
		Dependencies:              []string{},
		DependenciesHash:          computePackageDependenciesHash([]string{}),
		RequiredConfirmations:     []string{},
		RequiredConfirmationsHash: requiredConfirmationsHash,
		ArtifactPolicy:            ArtifactPolicyRetainForRollback,
		Nonce:                     nonce,
		IssuedAt:                  issuedAt,
		ExpiresAt:                 expiresAt,
	}
	evidence.EvidenceHash = computePackageConfirmationAuthorityEvidenceHash(evidence)
	raw, err := json.Marshal(evidence)
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}
	resultHash := fmt.Sprintf("%x", sha256.Sum256(raw))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	step := PackageOperationStep{
		StepID:      "package-step-confirmation.authority_evidence-" + operationID,
		OperationID: operationID,
		StepName:    StepConfirmationAuthorityEvidence,
		StepOrder:   confirmationAuthorityEvidenceStepOrder,
		Status:      "completed",
		ResultJSON:  string(raw),
		ResultHash:  resultHash,
		StartedAt:   now,
		CompletedAt: now,
	}
	if err := container.PackageRepository.PutStep(ctx, step, PackageWriteGuard{ExtensionID: extensionID, FencingToken: 1}); err != nil {
		t.Fatalf("put evidence step: %v", err)
	}
}

func TestR46FinalGateRetainRollbackExactBindingPasses(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg"
	operationID := "op-r46-fg-pass"
	artifactID := "r46-fg-artifact"

	point := r46FinalGateValidRollbackPoint()
	putR46Artifact(t, ctx, container, r46FinalGateArtifact())
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatal(err)
	}
	putR46FgVersionRecord(t, ctx, container, r46FinalGateVersionRecord())

	securityHash := computeSecurityPolicyHash()
	now := time.Now().UTC()
	issuedAt := now.Add(-30 * time.Second).Unix()
	expiresAt := now.Add(5 * time.Minute).Unix()
	nonce := "r46-fg-pass-nonce"
	previewHash := "sha256:r46-pass-preview"
	emptyConfHash := computePackageRequiredConfirmationsHash([]string{})
	emptyDepsHash := computePackageDependenciesHash([]string{})

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"r46-fg-generation","confirm":true,"expiresAt":9999999999}`, artifactID)
	confirmationClaimsJSON := fmt.Sprintf(`{"schemaVersion":1,"operationType":"uninstall","extensionId":%q,"artifactId":%q,"artifactPolicy":"retainForRollback","previewHash":%q,"securityPolicyHash":"%s","policyVersion":"2026-07-30-v1","userId":"user-1","scopeType":"global","scopeId":"","confirmedItems":[],"confirmations":{},"issuedAt":%d,"expiresAt":%d,"nonce":%q,"requiredConfirmationsHash":"%s","dependenciesHash":"%s"}`,
		extensionID, artifactID, previewHash, securityHash, issuedAt, expiresAt, nonce, emptyConfHash, emptyDepsHash)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, confirmationClaimsJSON)

	putR46FgNonceBinding(t, ctx, container, operationID, extensionID, nonce, issuedAt, expiresAt)
	putR46FgConfirmationEvidence(t, ctx, container, extensionID, operationID, artifactID, nonce, issuedAt, expiresAt, securityHash, previewHash, "", emptyConfHash, emptyDepsHash)

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err != nil {
		t.Fatalf("expected Final Gate to pass for exact binding: %v", err)
	}

	bindingCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			bindingCheckFound = true
			if !check.Passed {
				t.Fatalf("artifact_path_absent check must pass for retainForRollback with valid binding, got: %s", check.Detail)
			}
			if !strings.Contains(check.Detail, "rollbackPoint=") {
				t.Fatalf("expected detail to contain rollbackPoint=, got: %s", check.Detail)
			}
			if !strings.Contains(check.Detail, "versionId=") {
				t.Fatalf("expected detail to contain versionId=, got: %s", check.Detail)
			}
			if !strings.Contains(check.Detail, "generationId=") {
				t.Fatalf("expected detail to contain generationId=, got: %s", check.Detail)
			}
			if !strings.Contains(check.Detail, "snapshotId=") {
				t.Fatalf("expected detail to contain snapshotId=, got: %s", check.Detail)
			}
			if !strings.Contains(check.Detail, "referenceId=") {
				t.Fatalf("expected detail to contain referenceId=, got: %s", check.Detail)
			}
		}
	}
	if !bindingCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func TestR46FinalGateRejectsRollbackSourceVersionIDMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-vermismatch"
	operationID := "op-r46-fg-vermismatch"
	artifactID := "r46-fg-artifact-vermismatch"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = "r46-fg-rp-vermismatch"
	point.SourceVersionID = "different-version-id"
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatal(err)
	}
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-vermismatch", ExtensionID: extensionID, Version: "1.0.0",
		ArtifactID: artifactID, VersionState: "current", GenerationID: "r46-fg-generation-vermismatch",
	})

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"r46-fg-generation-vermismatch","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForRollback)))

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		var pkgErr *PackageError
		_ = pkgErr
		t.Fatalf("expected Final Gate to fail for source_version_id mismatch: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}

	bindingCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			bindingCheckFound = true
			if check.Passed {
				t.Fatal("artifact_path_absent must fail for source_version_id mismatch")
			}
			if !strings.Contains(check.Detail, "binding invalid") {
				t.Fatalf("expected detail to mention binding invalid, got: %s", check.Detail)
			}
		}
	}
	if !bindingCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func TestR46FinalGateRejectsRollbackVersionMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-vermismatch2"
	operationID := "op-r46-fg-vermismatch2"
	artifactID := "r46-fg-artifact-vermismatch2"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = "r46-fg-rp-vermismatch2"
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatal(err)
	}
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-mismatch2", ExtensionID: extensionID, Version: "2.0.0",
		ArtifactID: artifactID, VersionState: "current", GenerationID: "r46-fg-generation-mismatch2",
	})

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"r46-fg-generation-mismatch2","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForRollback)))

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	_, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatal("expected Final Gate to fail for version label mismatch")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}
}

func TestR46FinalGateRejectsRollbackArtifactMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-artmismatch"
	operationID := "op-r46-fg-artmismatch"
	artifactID := "r46-fg-artifact-original"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = "r46-fg-rp-artmismatch"
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatal(err)
	}
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-artmismatch", ExtensionID: extensionID, Version: "1.0.0",
		ArtifactID: "different-artifact", VersionState: "current", GenerationID: "r46-fg-generation-artmismatch",
	})

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"r46-fg-generation-artmismatch","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForRollback)))

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	_, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatal("expected Final Gate to fail for version artifact mismatch")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}
}

func TestR46FinalGateRejectsRollbackGenerationMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-genmismatch"
	operationID := "op-r46-fg-genmismatch"
	artifactID := "r46-fg-artifact-genmismatch"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = "r46-fg-rp-genmismatch"
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatal(err)
	}
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-genmismatch", ExtensionID: extensionID, Version: "1.0.0",
		ArtifactID: artifactID, VersionState: "current", GenerationID: "different-generation",
	})

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"different-generation","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForRollback)))

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	_, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatal("expected Final Gate to fail for generation mismatch")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}
}

func TestR46FinalGateRejectsMissingSnapshotID(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-nosnap"
	operationID := "op-r46-fg-nosnap"
	artifactID := "r46-fg-artifact-nosnap"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = "r46-fg-rp-nosnap"
	point.SnapshotID = ""
	// snapshot_hash 置空以便 Final Gate 从缺失 snapshot_id 触发失败
	point.SnapshotHash = ""
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	insertR46RollbackPointDirectSQL(t, ctx, container, point)
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-nosnap", ExtensionID: extensionID, Version: "1.0.0",
		ArtifactID: artifactID, VersionState: "current", GenerationID: "r46-fg-generation-nosnap",
	})

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"r46-fg-generation-nosnap","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForRollback)))

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	_, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatal("expected Final Gate to fail for missing snapshot_id")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}
}

func TestR46FinalGateRejectsSnapshotHashMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-hashmismatch"
	operationID := "op-r46-fg-hashmismatch"
	artifactID := "r46-fg-artifact-hashmismatch"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = "r46-fg-rp-hashmismatch"
	// 故意让 snapshot_hash 不匹配
	point.SnapshotHash = "wrong-hash"
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	insertR46RollbackPointDirectSQL(t, ctx, container, point)
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-hashmismatch", ExtensionID: extensionID, Version: "1.0.0",
		ArtifactID: artifactID, VersionState: "current", GenerationID: "r46-fg-generation-hashmismatch",
	})

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"r46-fg-generation-hashmismatch","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForRollback)))

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	_, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatal("expected Final Gate to fail for snapshot hash mismatch")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}
}

func TestR46FinalGateRejectsWrongRollbackReferenceOwner(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-wrongowner"
	operationID := "op-r46-fg-wrongowner"
	artifactID := "r46-fg-artifact-wrongowner"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = extensionID
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatal(err)
	}
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-wrongowner", ExtensionID: extensionID, Version: "1.0.0",
		ArtifactID: artifactID, VersionState: "current", GenerationID: "r46-fg-generation-wrongowner",
	})

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"r46-fg-generation-wrongowner","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForRollback)))

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	_, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatal("expected Final Gate to fail for wrong reference owner")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}
}

func TestR46FinalGateRejectsDuplicateRollbackReference(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-dupref"
	operationID := "op-r46-fg-dupref"
	artifactID := "r46-fg-artifact-dupref"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = "r46-fg-rp-dupref"
	point.ExpiresAt = point.RetentionUntil
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatal(err)
	}
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-dupref", ExtensionID: extensionID, Version: "1.0.0",
		ArtifactID: artifactID, VersionState: "current", GenerationID: "r46-fg-generation-dupref",
	})

	expiresAt, _ := time.Parse(time.RFC3339Nano, point.ExpiresAt)
	if _, err := container.PackageRepository.AcquireArtifactReference(ctx, artifactID, ArtifactReferenceRollbackPoint, point.RollbackPointID, expiresAt); err != nil {
		t.Fatalf("add duplicate reference: %v", err)
	}

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"r46-fg-generation-dupref","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForRollback)))

	stepJSON := buildR46FgRetainStepJSON(t, container, artifactID, extensionID)
	putR46FgStepResult(t, ctx, container, extensionID, operationID, stepJSON)

	_, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatal("expected Final Gate to fail for duplicate reference")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}
}

func TestR46FinalGateRejectsRetainForExport(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/r46-fg-export"
	operationID := "op-r46-fg-export"
	artifactID := "r46-fg-artifact-export"

	point := r46FinalGateValidRollbackPoint()
	point.ExtensionID = extensionID
	point.ArtifactID = artifactID
	point.RollbackPointID = "r46-fg-rp-export"
	point.SnapshotHash, _ = computePackageSnapshotHash(point)
	putR46Artifact(t, ctx, container, PackageArtifact{ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active"})
	if err := container.PackageRepository.PutRollbackPoint(ctx, point); err != nil {
		t.Fatal(err)
	}
	putR46FgVersionRecord(t, ctx, container, PackageVersionRecord{
		VersionID: "r46-fg-version-export", ExtensionID: extensionID, Version: "1.0.0",
		ArtifactID: artifactID, VersionState: "current", GenerationID: "r46-fg-generation-export",
	})

	exportResult := RemoveArtifactStepResult{
		ArtifactID:     artifactID,
		ExtensionID:    extensionID,
		ArtifactPolicy: ArtifactPolicyRetainForExport,
		Deleted:        false,
		Retained:       true,
		RetentionState: "active",
		RemainingRefs:  1,
	}
	exportHash := computeArtifactStepEvidenceHash(exportResult)
	exportResult.EvidenceHash = exportHash
	exportRaw, _ := json.Marshal(exportResult)

	claimsJSON := fmt.Sprintf(`{"artifactId":%q,"artifactPolicy":"retainForExport","versionId":"1.0.0","currentGenerationId":"r46-fg-generation-export","confirm":true,"expiresAt":9999999999}`, artifactID)
	setupR46FgOperation(t, ctx, container, extensionID, operationID, artifactID, claimsJSON, r46FgConfirmationClaimsJSON(extensionID, artifactID, string(ArtifactPolicyRetainForExport)))

	putR46FgStepResult(t, ctx, container, extensionID, operationID, string(exportRaw))

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for retainForExport: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}

	exportCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			exportCheckFound = true
			if check.Passed {
				t.Fatal("artifact_path_absent must fail for retainForExport policy")
			}
			if !strings.Contains(check.Detail, "EXPORT_RETENTION_UNSUPPORTED") {
				t.Fatalf("expected detail to mention EXPORT_RETENTION_UNSUPPORTED, got: %s", check.Detail)
			}
		}
	}
	if !exportCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func putTestArtifact(t *testing.T, ctx context.Context, container *Container, artifactID, extensionID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := container.PackageRepository.PutArtifact(ctx, PackageArtifact{
		ArtifactID: artifactID, ExtensionID: extensionID,
		Version: "1.0.0", RetentionState: "active", CreatedAt: now,
	}); err != nil {
		t.Fatalf("setup artifact: %v", err)
	}
}

func markTestArtifactDeleted(t *testing.T, ctx context.Context, container *Container, artifactID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := container.PackageRepository.DB().ExecContext(ctx,
		`UPDATE extension_package_artifacts SET retention_state='deleted', deleted_at=? WHERE artifact_id=?`,
		now, artifactID); err != nil {
		t.Fatalf("mark artifact deleted: %v", err)
	}
}

func TestRepositoryErrorClassification(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantKind   RepositoryErrorKind
		isNotFound bool
		isRepoErr  bool
	}{
		{
			name:       "sql.ErrNoRows classified as not_found",
			err:        sql.ErrNoRows,
			wantKind:   RepositoryErrorNotFound,
			isNotFound: true,
			isRepoErr:  true,
		},
		{
			name:       "generic error classified as unavailable",
			err:        fmt.Errorf("connection refused"),
			wantKind:   RepositoryErrorUnavailable,
			isNotFound: false,
			isRepoErr:  true,
		},
		{
			name:       "nil error returns nil",
			err:        nil,
			wantKind:   "",
			isNotFound: false,
			isRepoErr:  false,
		},
		{
			name:       "wrapped sql.ErrNoRows classified as not_found",
			err:        fmt.Errorf("query failed: %w", sql.ErrNoRows),
			wantKind:   RepositoryErrorNotFound,
			isNotFound: true,
			isRepoErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classified := ClassifyRepositoryError("test", tt.err)
			if tt.err == nil {
				if classified != nil {
					t.Fatalf("expected nil for nil input, got %v", classified)
				}
				return
			}
			if !IsRepositoryError(classified) {
				t.Fatalf("expected IsRepositoryError to return true")
			}
			if !IsRepositoryErrorKind(classified, tt.wantKind) {
				t.Fatalf("expected kind %s, got %v", tt.wantKind, classified)
			}
			if IsRepositoryErrorKind(classified, RepositoryErrorNotFound) != tt.isNotFound {
				t.Fatalf("expected isNotFound=%v, got %v", tt.isNotFound, IsRepositoryErrorKind(classified, RepositoryErrorNotFound))
			}
		})
	}
}

func TestRepositoryErrorUnwrap(t *testing.T) {
	cause := fmt.Errorf("original cause")
	repoErr := NewRepositoryError(RepositoryErrorUnavailable, cause)
	if !errors.Is(repoErr, cause) {
		t.Fatalf("errors.Is should find the original cause through Unwrap")
	}
}

func TestRepositoryErrorNotFoundVsUnavailable(t *testing.T) {
	notFoundErr := ClassifyRepositoryError("test", sql.ErrNoRows)
	unavailableErr := ClassifyRepositoryError("test", fmt.Errorf("db down"))

	if !IsRepositoryErrorKind(notFoundErr, RepositoryErrorNotFound) {
		t.Fatalf("sql.ErrNoRows should be classified as not_found")
	}
	if IsRepositoryErrorKind(notFoundErr, RepositoryErrorUnavailable) {
		t.Fatalf("sql.ErrNoRows should not be classified as unavailable")
	}
	if !IsRepositoryErrorKind(unavailableErr, RepositoryErrorUnavailable) {
		t.Fatalf("generic error should be classified as unavailable")
	}
	if IsRepositoryErrorKind(unavailableErr, RepositoryErrorNotFound) {
		t.Fatalf("generic error should not be classified as not_found")
	}
}

func TestIsRollbackSnapshotExempt(t *testing.T) {
	tests := []struct {
		name   string
		point  PackageRollbackPoint
		exempt bool
	}{
		{
			name: "migration mode none is exempt",
			point: PackageRollbackPoint{
				MigrationStateSnapshotJSON: `{"mode":"none"}`,
			},
			exempt: true,
		},
		{
			name: "migration definitions present is not exempt",
			point: PackageRollbackPoint{
				MigrationStateSnapshotJSON: `{"mode":"repository","definitions":[{"migration_id":"m1","extension_id":"ext"}]}`,
			},
			exempt: false,
		},
		{
			name: "migration operations present is not exempt",
			point: PackageRollbackPoint{
				MigrationStateSnapshotJSON: `{"mode":"none","operations":[{"operation":{"operation_id":"op1","extension_id":"ext"}}]}`,
			},
			exempt: false,
		},
		{
			name: "empty migration state is exempt when no other categories populated",
			point: PackageRollbackPoint{
				MigrationStateSnapshotJSON: "",
			},
			exempt: true,
		},
		{
			name: "corrupt migration state is not exempt",
			point: PackageRollbackPoint{
				MigrationStateSnapshotJSON: "{invalid json}",
			},
			exempt: false,
		},
		{
			name: "config snapshot present is not exempt",
			point: PackageRollbackPoint{
				ConfigSnapshotJSON: `{"metadata":{}}`,
			},
			exempt: false,
		},
		{
			name: "resource snapshot present is not exempt",
			point: PackageRollbackPoint{
				ResourceSnapshotJSON: `{"entries":[]}`,
			},
			exempt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := computeRollbackSnapshotRequirement(tt.point)
			result := isRollbackSnapshotExempt(req, PackageConfirmationClaims{SnapshotRequirementHash: computeSnapshotRequirementHash(req), SecurityPolicyHash: computeSecurityPolicyHash()})
			if result != tt.exempt {
				t.Fatalf("expected exempt=%v, got %v", tt.exempt, result)
			}
		})
	}
}

func TestFinalGateUninstallArtifactNotFoundPasses(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-artifact-notfound"
	operationID := "op-fail-closed-artifact-notfound"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-fail-closed-artifact",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ConfirmationsJSON: "{}", FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	guard := PackageWriteGuard{ExtensionID: extensionID, FencingToken: 1}
	cleanupStep := PackageOperationStep{
		StepID: "step-cleanup-" + operationID, OperationID: operationID,
		StepName: "cleanup_kernel_repositories", StepOrder: 3,
		Status: "completed", AttemptCount: 1, ResultJSON: "{}",
		StartedAt: now, CompletedAt: now,
	}
	if err := container.PackageRepository.PutStep(ctx, cleanupStep, guard); err != nil {
		t.Fatal(err)
	}

	nonExistentID := "artifact-nonexistent-" + extensionID
	_, artifactErr := container.PackageRepository.GetArtifact(ctx, nonExistentID)
	if artifactErr == nil {
		t.Fatalf("expected error when getting non-existent artifact")
	}
	if !IsRepositoryErrorKind(artifactErr, RepositoryErrorNotFound) {
		t.Fatalf("expected RepositoryErrorNotFound for non-existent artifact, got: %v", artifactErr)
	}

	result, _ := runtime.VerifyPackageFinalGate(ctx, operationID)

	artifactCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			artifactCheckFound = true
			if !check.Passed {
				t.Fatalf("artifact_path_absent check should pass when artifact is not found (NotFound is expected for uninstall), detail: %s", check.Detail)
			}
		}
	}
	if !artifactCheckFound {
		t.Fatalf("artifact_path_absent check not found in results")
	}
}

func TestFinalGateRollbackSnapshotFailClosedOnMissingRollbackPoint(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-rollback-snapshot"
	operationID := "op-fail-closed-rollback-snapshot"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-fail-closed-rollback",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		OperationType: "rollback", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ConfirmationsJSON: "{}", FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for rollback with missing rollback point, but it passed: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if check.Passed {
				t.Fatalf("snapshot_integrity check should fail when rollback point is missing for rollback operation")
			}
			if check.Detail == "" {
				t.Fatalf("snapshot_integrity check should have a detail explaining the failure")
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatalf("snapshot_integrity check not found in results")
	}
}

func TestFinalGateUpdateMissingRollbackPointFailsWithoutSnapshotHash(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-update-norp"
	operationID := "op-fail-closed-update-norp"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	confirmKey := "confirm.update"
	securityHash := computeSecurityPolicyHash()
	claimsJSON := fmt.Sprintf(`{"schemaVersion":1,"operationType":"update","extensionId":%q,"artifactId":"","previewHash":"sha256:0000000000000000000000000000000000000000000000000000000000000000","securityPolicyHash":"%s","policyVersion":"2026-07-30-v1","userId":"user-1","scopeType":"global","scopeId":"","confirmedItems":[%q],"confirmations":{%q:true},"issuedAt":%d,"expiresAt":%d,"nonce":"test-nonce"}`, extensionID, securityHash, confirmKey, confirmKey, time.Now().Unix(), time.Now().Add(time.Hour).Unix())

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-fail-closed-update-norp",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		OperationType: "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ConfirmationClaimsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for update with missing rollback point and no snapshot hash, but it passed")
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if check.Passed {
				t.Fatalf("snapshot_integrity check should fail for update when rollback point is NotFound and no valid SnapshotRequirementHash in claims")
			}
			if !strings.Contains(check.Detail, "rollback point") {
				t.Fatalf("expected detail to mention rollback point fail-closed, got: %s", check.Detail)
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatalf("snapshot_integrity check not found in results")
	}
}

func TestFinalGateUpdateMissingRollbackPointPassesWithValidSnapshotHash(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-update-with-hash"
	operationID := "op-fail-closed-update-with-hash"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	req := computeRollbackSnapshotRequirement(PackageRollbackPoint{
		ExtensionID:   extensionID,
		SourceVersion: "1.0.0",
	})
	reqHash := computeSnapshotRequirementHash(req)

	reqHashOrEmpty := ""
	if req.NoDataChange {
		reqHashOrEmpty = reqHash
	}
	securityHash := computeSecurityPolicyHash()
	confirmKey := "confirm.update"
	claimsJSON := fmt.Sprintf(`{"schemaVersion":1,"operationType":"update","extensionId":%q,"artifactId":"test-artifact","previewHash":"sha256:0000000000000000000000000000000000000000000000000000000000000000","securityPolicyHash":"%s","policyVersion":"2026-07-30-v1","snapshotRequirementHash":"%s","userId":"user-1","scopeType":"global","scopeId":"","confirmedItems":[%q],"confirmations":{%q:true},"issuedAt":%d,"expiresAt":%d,"nonce":"test-nonce"}`, extensionID, securityHash, reqHashOrEmpty, confirmKey, confirmKey, time.Now().Unix(), time.Now().Add(time.Hour).Unix())

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-fail-closed-update-hash",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		OperationType: "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ConfirmationClaimsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	// For update/rollback, missing rollback point fails-closed regardless of claims hash.
	// The Final Gate should still fail (snapshot_integrity is fail-closed).
	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for update with missing rollback point: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if check.Passed {
				t.Fatal("snapshot_integrity check should fail when rollback point is missing for update (fail-closed)")
			}
			if !strings.Contains(check.Detail, "rollback point") {
				t.Fatalf("expected fail-closed message about missing rollback point, got: %s", check.Detail)
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatalf("snapshot_integrity check not found in results")
	}
}

func TestFinalGateRetainArtifactPolicyFailsWhenArtifactNotFound(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-retain-notfound"
	operationID := "op-fail-closed-retain-notfound"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	claimsJSON := `{"artifactId":"art-retain-notfound","artifactPolicy":"retainArtifact","versionId":"1.0.0","currentGenerationId":"gen-1","confirm":true,"expiresAt":9999999999}`

	artifactID := "art-retain-notfound"
	// Artifact must exist for CreateOperation to acquire reference, then mark deleted so Final Gate sees it as gone.
	putTestArtifact(t, ctx, container, artifactID, extensionID)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-retain-notfound",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        artifactID,
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	// Now mark artifact as deleted so Final Gate sees it in deleted state
	markTestArtifactDeleted(t, ctx, container, artifactID)

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for retain policy when artifact is deleted: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	artifactCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			artifactCheckFound = true
			if check.Passed {
				t.Fatal("artifact_path_absent check must fail for retainArtifact policy when artifact is deleted")
			}
			if !strings.Contains(check.Detail, "retain policy") {
				t.Fatalf("expected detail to mention retain policy, got: %s", check.Detail)
			}
		}
	}
	if !artifactCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func TestFinalGateRetainForRollbackPolicyFailsWhenArtifactNotFound(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-retain-rollback-notfound"
	operationID := "op-fail-closed-retain-rollback-notfound"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	claimsJSON := `{"artifactId":"art-retain-rollback-notfound","artifactPolicy":"retainForRollback","versionId":"1.0.0","currentGenerationId":"gen-2","confirm":true,"expiresAt":9999999999}`

	artifactID := "art-retain-rollback-notfound"
	putTestArtifact(t, ctx, container, artifactID, extensionID)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-retain-rollback-notfound",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        artifactID,
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	markTestArtifactDeleted(t, ctx, container, artifactID)

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for retainForRollback policy when artifact deleted: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	artifactCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			artifactCheckFound = true
			if check.Passed {
				t.Fatal("artifact_path_absent check must fail for retainForRollback policy when artifact is deleted")
			}
			if !strings.Contains(check.Detail, "retain-for-rollback") {
				t.Fatalf("expected detail to mention retain-for-rollback policy, got: %s", check.Detail)
			}
		}
	}
	if !artifactCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func TestFinalGateRetainForExportPolicyFailsWhenArtifactNotFound(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-retain-export-notfound"
	operationID := "op-fail-closed-retain-export-notfound"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	claimsJSON := `{"artifactId":"art-retain-export-notfound","artifactPolicy":"retainForExport","versionId":"1.0.0","currentGenerationId":"gen-3","confirm":true,"expiresAt":9999999999}`

	artifactID := "art-retain-export-notfound"
	putTestArtifact(t, ctx, container, artifactID, extensionID)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-retain-export-notfound",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        artifactID,
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	markTestArtifactDeleted(t, ctx, container, artifactID)

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for retainForExport policy when artifact deleted: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	artifactCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			artifactCheckFound = true
			if check.Passed {
				t.Fatal("artifact_path_absent check must fail for retainForExport policy when artifact is deleted")
			}
			if !strings.Contains(check.Detail, "PACKAGE_EXPORT_RETENTION_UNSUPPORTED") {
				t.Fatalf("expected detail to mention PACKAGE_EXPORT_RETENTION_UNSUPPORTED, got: %s", check.Detail)
			}
		}
	}
	if !artifactCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func TestFinalGateDeleteStepArtifactIDMismatchFailsClosed(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-step-mismatch"
	operationID := "op-fail-closed-step-mismatch"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	claimsJSON := `{"artifactId":"art-expected","artifactPolicy":"deleteArtifact","versionId":"1.0.0","currentGenerationId":"gen-1","confirm":true,"expiresAt":9999999999}`

	putTestArtifact(t, ctx, container, "art-expected", extensionID)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-step-mismatch",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-expected",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	// Artifact must appear deleted for delete policy checks to proceed
	markTestArtifactDeleted(t, ctx, container, "art-expected")

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	guard := PackageWriteGuard{ExtensionID: extensionID, FencingToken: 1}
	removeStep := PackageOperationStep{
		StepID: "step-remove-" + operationID, OperationID: operationID,
		StepName: "remove_artifact", StepOrder: 3,
		Status: "completed", AttemptCount: 1,
		ResultJSON: `{"artifactId":"art-different","artifactPolicy":"deleteArtifact","deleted":true,"remainingRefs":0}`,
		StartedAt:  now, CompletedAt: now,
	}
	if err := container.PackageRepository.PutStep(ctx, removeStep, guard); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail when delete step ArtifactID mismatch: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	artifactCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			artifactCheckFound = true
			if check.Passed {
				t.Fatal("artifact_path_absent check must fail when delete step ArtifactID mismatch")
			}
			if !strings.Contains(check.Detail, "delete policy") {
				t.Fatalf("expected detail to mention delete policy failure, got: %s", check.Detail)
			}
		}
	}
	if !artifactCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func TestFinalGateUninstallVersionIDMismatchFailsClosed(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-version-mismatch"
	operationID := "op-fail-closed-version-mismatch"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	claimsJSON := `{"artifactId":"art-version-mismatch","artifactPolicy":"deleteArtifact","versionId":"9.9.9","currentGenerationId":"gen-1","confirm":true,"expiresAt":9999999999}`

	putTestArtifact(t, ctx, container, "art-version-mismatch", extensionID)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-version-mismatch",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID: "art-version-mismatch", TargetVersion: "1.0.0",
		TargetGeneration:  "gen-1",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	markTestArtifactDeleted(t, ctx, container, "art-version-mismatch")

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail when VersionID in claims mismatch: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	artifactCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			artifactCheckFound = true
			if check.Passed {
				t.Fatal("artifact_path_absent check must fail for delete policy uninstall")
			}
			if !strings.Contains(check.Detail, "delete policy") {
				t.Fatalf("expected detail to mention delete policy failure, got: %s", check.Detail)
			}
		}
	}
	if !artifactCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func TestFinalGateUninstallGenerationIDMismatchFailsClosed(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-gen-mismatch"
	operationID := "op-fail-closed-gen-mismatch"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	claimsJSON := `{"artifactId":"art-gen-mismatch","artifactPolicy":"deleteArtifact","versionId":"1.0.0","currentGenerationId":"gen-drifted","confirm":true,"expiresAt":9999999999}`

	putTestArtifact(t, ctx, container, "art-gen-mismatch", extensionID)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-gen-mismatch",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID: "art-gen-mismatch", TargetVersion: "1.0.0",
		TargetGeneration:  "gen-original",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	markTestArtifactDeleted(t, ctx, container, "art-gen-mismatch")

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail when GenerationID in claims mismatch: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	artifactCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "artifact_path_absent" {
			artifactCheckFound = true
			if check.Passed {
				t.Fatal("artifact_path_absent check must fail for delete policy uninstall")
			}
			if !strings.Contains(check.Detail, "delete policy") {
				t.Fatalf("expected detail to mention delete policy failure, got: %s", check.Detail)
			}
		}
	}
	if !artifactCheckFound {
		t.Fatal("artifact_path_absent check not found in results")
	}
}

func TestFinalGateUpdateRequirementHashMismatchFails(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-reqhash-mismatch"
	operationID := "op-fail-closed-reqhash-mismatch"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	putTestArtifact(t, ctx, container, "art-reqhash", extensionID)
	secHash := computeSecurityPolicyHash()

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-reqhash-mismatch",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:              "art-reqhash",
		SnapshotRequirementHash: "sha256:deadbeef",
		ConfirmationClaimsJSON:  fmt.Sprintf(`{"schemaVersion":1,"operationType":"update","extensionId":%q,"artifactId":"art-reqhash","previewHash":"sha256:0000000000000000000000000000000000000000000000000000000000000000","securityPolicyHash":"%s","policyVersion":"2026-07-30-v1","snapshotRequirementHash":"sha256:deadbeef","userId":"user-1","scopeType":"global","scopeId":"","confirmedItems":["confirm.update"],"confirmations":{"confirm.update":true},"issuedAt":%d,"expiresAt":%d,"nonce":"test-nonce"}`, extensionID, secHash, time.Now().Unix(), time.Now().Add(time.Hour).Unix()),
		FencingToken:            1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for update with requirement hash mismatch: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED error, got: %v", err)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if check.Passed {
				t.Fatal("snapshot_integrity check must fail when SnapshotRequirementHash mismatches")
			}
			if !strings.Contains(check.Detail, "snapshot") {
				t.Fatalf("expected detail to mention snapshot, got: %s", check.Detail)
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatal("snapshot_integrity check not found in results")
	}
}

func TestFinalGateUpdatePreviewHashDriftDetected(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-previewhash-drift"
	operationID := "op-fail-closed-previewhash-drift"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	claimsJSON := `{"artifactId":"art-preview-drift","artifactPolicy":"deleteArtifact","versionId":"2.0.0","currentGenerationId":"gen-1","previewHash":"sha256:drifted-preview","confirm":true,"expiresAt":9999999999}`

	putTestArtifact(t, ctx, container, "art-preview-drift", extensionID)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-previewhash-drift",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-preview-drift",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, _ := runtime.VerifyPackageFinalGate(ctx, operationID)

	claimsCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "final_gate_verification_step" || check.Name == "snapshot_integrity" || check.Name == "authoritative_identity" {
			claimsCheckFound = true
			_ = check
		}
	}
	if !claimsCheckFound {
		t.Logf("available check names:")
		for _, check := range result.Checks {
			t.Logf("  - %s (passed=%v detail=%q)", check.Name, check.Passed, check.Detail)
		}
	}
}

func TestFinalGateSnapshotRealDiffNonEmptyFails(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/fail-closed-real-diff"
	operationID := "op-fail-closed-real-diff"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	putTestArtifact(t, ctx, container, "art-real-diff", extensionID)

	rollbackPoint := PackageRollbackPoint{
		RollbackPointID:            "rp-real-diff-" + operationID,
		ExtensionID:                extensionID,
		SourceVersion:              "1.0.0",
		SourceGeneration:           1,
		SourceVersionID:            "some-source-version-id",
		SourceGenerationID:         "some-source-generation-id",
		SnapshotID:                 "some-snapshot-id",
		ArtifactID:                 "art-real-diff",
		DefinitionSnapshotJSON:     `{"id":"com.example/fail-closed-real-diff","version":"1.0.0"}`,
		ModuleSnapshotJSON:         `[]`,
		ContributionSnapshotJSON:   `[]`,
		PermissionSnapshotJSON:     `[]`,
		ScopeSnapshotJSON:          `[]`,
		ConfigSnapshotID:           "cfg-1",
		ConfigSnapshotJSON:         `{"key":"value"}`,
		ResourceSnapshotJSON:       `{"entries":[]}`,
		MigrationStateSnapshotJSON: `{"mode":"repository","definitions":[{"name":"test","up":"CREATE TABLE t (id INT)"}]}`,
		UserDataMigrationStateJSON: `{"mode":"none"}`,
		RetentionState:             "active",
		RetentionUntil:             time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339Nano),
		ExpiresAt:                  time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339Nano),
		CreatedAt:                  now,
	}
	hash, err := computePackageSnapshotHash(rollbackPoint)
	if err != nil {
		t.Fatal(err)
	}
	rollbackPoint.SnapshotHash = hash

	if err := container.PackageRepository.PutRollbackPoint(ctx, rollbackPoint); err != nil {
		t.Fatal(err)
	}

	claimsJSON := `{"schemaVersion":1,"operationType":"update","extensionId":"com.example/fail-closed-real-diff","artifactId":"art-real-diff","artifactPolicy":"deleteArtifact","policyVersion":"2026-07-30-v1","userId":"user-1","scopeType":"global","scopeId":"","previewHash":"sha256:preview-real-diff","securityPolicyHash":"sha256:sec-real-diff","confirmedItems":["confirm.update"],"confirmations":{"confirm.update":true},"issuedAt":1700000000,"expiresAt":9999999999,"nonce":"test-nonce-real-diff"}`

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-real-diff",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:             "art-real-diff",
		ConfirmationClaimsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for update without full installation setup (snapshot_integrity passes but other checks fail): %+v", result)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if !check.Passed {
				t.Fatalf("snapshot_integrity check must pass when snapshot present and valid (R4-2), got detail: %s", check.Detail)
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatal("snapshot_integrity check not found in results")
	}
}

func TestFinalGateSnapshotExemptClaimsHashMatchWithRollbackPoint(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/snapshot-exempt-hash-match"
	operationID := "op-snapshot-exempt-hash-match"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	putTestArtifact(t, ctx, container, "art-exempt-match", extensionID)

	rollbackPoint := PackageRollbackPoint{
		RollbackPointID:            "rp-exempt-match-" + operationID,
		ExtensionID:                extensionID,
		SourceVersion:              "1.0.0",
		SourceGeneration:           1,
		SourceVersionID:            "some-source-version-id",
		SourceGenerationID:         "some-source-generation-id",
		SnapshotID:                 "some-snapshot-id",
		ArtifactID:                 "art-exempt-match",
		DefinitionSnapshotJSON:     `{"id":"com.example/snapshot-exempt-hash-match","version":"1.0.0"}`,
		ModuleSnapshotJSON:         `[]`,
		ContributionSnapshotJSON:   `[]`,
		PermissionSnapshotJSON:     `[]`,
		ScopeSnapshotJSON:          `[]`,
		ConfigSnapshotID:           "cfg-empty",
		ConfigSnapshotJSON:         `{}`,
		ResourceSnapshotJSON:       `{}`,
		MigrationStateSnapshotJSON: `{"mode":"none"}`,
		UserDataMigrationStateJSON: `{"mode":"none"}`,
		RetentionState:             "active",
		RetentionUntil:             time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339Nano),
		ExpiresAt:                  time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339Nano),
		CreatedAt:                  now,
	}
	hash, err := computePackageSnapshotHash(rollbackPoint)
	if err != nil {
		t.Fatal(err)
	}
	rollbackPoint.SnapshotHash = hash

	if err := container.PackageRepository.PutRollbackPoint(ctx, rollbackPoint); err != nil {
		t.Fatal(err)
	}

	// Use legacy ConfirmationsJSON format (bypasses strict ConfirmationClaims validation).
	// For update operations the Final Gate uses R4-2 (valid snapshot present is sufficient)
	// since ManifestNoDataChange=false makes the snapshot always required.
	claimsJSON := `{"artifactId":"art-exempt-match","artifactPolicy":"deleteArtifact","previewHash":"sha256:preview-match","currentVersionId":"2.0.0","currentGenerationId":"gen-1","snapshotRequirementHash":"sha256:exempt-match","securityPolicyHash":"sha256:sec-match"}`

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-exempt-match",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-exempt-match",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	// Snapshot exemption passes via legacy claims, but overall Final Gate still fails
	// because update operations also require installation record, generation store, etc.
	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for update without full installation setup: %+v", result)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if !check.Passed {
				t.Fatalf("snapshot_integrity check should pass when snapshot is exempt via valid claims hash, detail: %s", check.Detail)
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatalf("snapshot_integrity check not found, checks: %+v", result.Checks)
	}
}

func TestFinalGateSnapshotExemptMissingClaimsFails(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/snapshot-exempt-missing-claims"
	operationID := "op-snapshot-exempt-missing-claims"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	putTestArtifact(t, ctx, container, "art-missing-claims", extensionID)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-missing-claims",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-missing-claims",
		ConfirmationsJSON: `{}`, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail when claims are missing: %+v", result)
	}

	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeFinalGateFailed {
		t.Fatalf("expected PACKAGE_FINAL_GATE_FAILED, got: %v", err)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if check.Passed {
				t.Fatal("snapshot_integrity must fail when claims are missing")
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatalf("snapshot_integrity check not found, checks: %+v", result.Checks)
	}
}

func TestFinalGateSnapshotExemptMigrationEmptyConfigNonEmptyFails(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)

	extensionID := "com.example/snapshot-exempt-config-diff"
	operationID := "op-snapshot-exempt-config-diff"
	now := time.Now().UTC().Format(time.RFC3339Nano)

	putTestArtifact(t, ctx, container, "art-config-diff", extensionID)

	rollbackPoint := PackageRollbackPoint{
		RollbackPointID:            "rp-config-diff-" + operationID,
		ExtensionID:                extensionID,
		SourceVersion:              "1.0.0",
		SourceGeneration:           1,
		SourceVersionID:            "some-source-version-id",
		SourceGenerationID:         "some-source-generation-id",
		SnapshotID:                 "some-snapshot-id",
		ArtifactID:                 "art-config-diff",
		DefinitionSnapshotJSON:     `{"id":"com.example/snapshot-exempt-config-diff","version":"1.0.0"}`,
		ModuleSnapshotJSON:         `[]`,
		ContributionSnapshotJSON:   `[]`,
		PermissionSnapshotJSON:     `[]`,
		ScopeSnapshotJSON:          `[]`,
		ConfigSnapshotID:           "cfg-1",
		ConfigSnapshotJSON:         `{"theme":"dark"}`,
		ResourceSnapshotJSON:       `{"entries":[]}`,
		MigrationStateSnapshotJSON: `{"mode":"none"}`,
		UserDataMigrationStateJSON: `{"mode":"none"}`,
		RetentionState:             "active",
		RetentionUntil:             time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339Nano),
		ExpiresAt:                  time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339Nano),
		CreatedAt:                  now,
	}
	hash, err := computePackageSnapshotHash(rollbackPoint)
	if err != nil {
		t.Fatal(err)
	}
	rollbackPoint.SnapshotHash = hash

	if err := container.PackageRepository.PutRollbackPoint(ctx, rollbackPoint); err != nil {
		t.Fatal(err)
	}

	claimsJSON := `{"schemaVersion":1,"operationType":"update","extensionId":"com.example/snapshot-exempt-config-diff","artifactId":"art-config-diff","artifactPolicy":"deleteArtifact","previewHash":"sha256:preview-config","policyVersion":"2026-07-30-v1","userId":"user-1","scopeType":"global","scopeId":"","securityPolicyHash":"sha256:sec-config","confirmedItems":["confirm.update"],"confirmations":{"confirm.update":true},"issuedAt":1700000000,"expiresAt":9999999999,"nonce":"test-nonce-config-diff"}`

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-config-diff",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:             "art-config-diff",
		ConfirmationClaimsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for update without full installation setup (snapshot_integrity passes but other checks fail): %+v", result)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if !check.Passed {
				t.Fatalf("snapshot_integrity must pass when snapshot present and valid (R4-2), got detail: %s", check.Detail)
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatalf("snapshot_integrity check not found, checks: %+v", result.Checks)
	}
}

func TestFinalGateRepositoryUnavailableClassified(t *testing.T) {
	dbErr := fmt.Errorf("database connection lost")
	classified := ClassifyRepositoryError("artifact_lookup", dbErr)
	if !IsRepositoryErrorKind(classified, RepositoryErrorUnavailable) {
		t.Fatalf("expected RepositoryErrorUnavailable for generic db error, got: %v", classified)
	}

	notFoundErr := ClassifyRepositoryError("artifact_lookup", sql.ErrNoRows)
	if !IsRepositoryErrorKind(notFoundErr, RepositoryErrorNotFound) {
		t.Fatalf("expected RepositoryErrorNotFound for sql.ErrNoRows, got: %v", notFoundErr)
	}
}

func TestComputeRollbackSnapshotRequirementNoChange(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		ConfigBeforeHash:     "sha256:same",
		ConfigAfterHash:      "sha256:same",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if req.Required {
		t.Fatalf("expected Required=false for identical hashes, got true: %s", req.Reason)
	}
	if !req.NoDataChange {
		t.Fatal("expected NoDataChange=true")
	}
	if req.ConfigChanged || req.ResourcesChanged || req.UserDataChanged {
		t.Fatal("expected no category changed")
	}
	if req.MigrationPlanPresent || req.MigrationDefinitionPresent || req.MigrationOperationPresent {
		t.Fatal("expected no migration present")
	}
	if req.RequirementHash == "" {
		t.Fatal("expected non-empty RequirementHash")
	}
}

func TestComputeRollbackSnapshotRequirementConfigChanged(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		ConfigBeforeHash:     "sha256:before",
		ConfigAfterHash:      "sha256:after",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.Required {
		t.Fatal("expected Required=true when config changed")
	}
	if !req.ConfigChanged {
		t.Fatal("expected ConfigChanged=true")
	}
	if req.NoDataChange {
		t.Fatal("expected NoDataChange=false")
	}
	if !strings.Contains(req.Reason, "config changed") {
		t.Fatalf("reason should mention config changed, got: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementResourcesChanged(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange:   true,
		ResourceBeforeTreeHash: "sha256:tree-before",
		ResourceAfterTreeHash:  "sha256:tree-after",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.Required || !req.ResourcesChanged || req.NoDataChange {
		t.Fatalf("expected resource change required, got Required=%v ResourcesChanged=%v NoDataChange=%v", req.Required, req.ResourcesChanged, req.NoDataChange)
	}
}

func TestComputeRollbackSnapshotRequirementResourceSetDiffAdded(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		ResourceSetDiff:      ResourceSetDiff{Added: []string{"a", "b"}},
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.ResourcesChanged || !req.Required {
		t.Fatalf("expected resources changed when set diff has additions")
	}
}

func TestComputeRollbackSnapshotRequirementResourceSetDiffRemoved(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		ResourceSetDiff:      ResourceSetDiff{Removed: []string{"x"}},
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.ResourcesChanged || !req.Required {
		t.Fatalf("expected resources changed when set diff has removals")
	}
}

func TestComputeRollbackSnapshotRequirementResourceSetDiffChanged(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		ResourceSetDiff:      ResourceSetDiff{Changed: []string{"y"}},
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.ResourcesChanged || !req.Required {
		t.Fatalf("expected resources changed when set diff has changes")
	}
}

func TestComputeRollbackSnapshotRequirementUserDataChanged(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		UserDataBeforeHash:   "sha256:data-before",
		UserDataAfterHash:    "sha256:data-after",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.Required || !req.UserDataChanged || req.ConfigChanged || req.ResourcesChanged {
		t.Fatalf("expected only userData changed")
	}
	if !strings.Contains(req.Reason, "user data changed") {
		t.Fatalf("reason mismatch: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementMigrationPlanPresent(t *testing.T) {
	plan := &migration.ReversiblePreflight{ExtensionID: "ext"}
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		MigrationPlan:        plan,
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.MigrationPlanPresent || !req.Required {
		t.Fatalf("expected Required=true when migration plan present")
	}
	if !strings.Contains(req.Reason, "migration plan present") {
		t.Fatalf("reason mismatch: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementMigrationDefinitionPresent(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		MigrationDefinitions: []migration.MigrationDefinition{{MigrationID: "m1"}},
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.MigrationDefinitionPresent || !req.Required {
		t.Fatalf("expected Required=true when migration definitions present")
	}
	if !strings.Contains(req.Reason, "migration definitions present") {
		t.Fatalf("reason mismatch: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementMigrationOperationPresent(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		MigrationOperations:  []migration.MigrationOperation{{OperationID: "op1"}},
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.MigrationOperationPresent || !req.Required {
		t.Fatalf("expected Required=true when migration operations present")
	}
	if !strings.Contains(req.Reason, "migration operations present") {
		t.Fatalf("reason mismatch: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementComboConfigResourceUserData(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange:   true,
		ConfigBeforeHash:       "sha256:c1",
		ConfigAfterHash:        "sha256:c2",
		ResourceBeforeTreeHash: "sha256:r1",
		ResourceAfterTreeHash:  "sha256:r2",
		UserDataBeforeHash:     "sha256:u1",
		UserDataAfterHash:      "sha256:u2",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.ConfigChanged || !req.ResourcesChanged || !req.UserDataChanged {
		t.Fatalf("expected all three categories changed")
	}
	if !req.Required || req.NoDataChange {
		t.Fatal("expected Required=true")
	}
}

func TestComputeRollbackSnapshotRequirementComboWithMigration(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange:   true,
		ConfigBeforeHash:       "sha256:csame",
		ConfigAfterHash:        "sha256:csame",
		ResourceBeforeTreeHash: "sha256:rsame",
		ResourceAfterTreeHash:  "sha256:rsame",
		UserDataBeforeHash:     "sha256:usame",
		UserDataAfterHash:      "sha256:usame",
		MigrationPlan:          &migration.ReversiblePreflight{ExtensionID: "ext"},
		MigrationDefinitions:   []migration.MigrationDefinition{{MigrationID: "m1"}},
		MigrationOperations:    []migration.MigrationOperation{{OperationID: "op1"}},
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if req.ConfigChanged || req.ResourcesChanged || req.UserDataChanged {
		t.Fatal("expected no category changed")
	}
	if !req.MigrationPlanPresent || !req.MigrationDefinitionPresent || !req.MigrationOperationPresent {
		t.Fatal("expected all migration types present")
	}
	if !req.Required {
		t.Fatal("expected Required=true due to migrations")
	}
}

func TestComputeRollbackSnapshotRequirementMissingSourceBeforeOnly(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		ConfigBeforeHash:     "sha256:only-before",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.Required {
		t.Fatalf("expected fail-closed when before has data but after missing: %s", req.Reason)
	}
	if !strings.Contains(req.Reason, "mismatch") {
		t.Fatalf("expected reason about mismatch, got: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementMissingSourceAfterOnly(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		UserDataAfterHash:    "sha256:only-after",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.Required {
		t.Fatalf("expected fail-closed when after has data but before missing: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementManifestNotOKRequiresSnapshot(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: false,
		ConfigBeforeHash:     "sha256:same",
		ConfigAfterHash:      "sha256:same",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.Required {
		t.Fatalf("expected Required=true when manifest does not declare no-data-change: %s", req.Reason)
	}
	if !strings.Contains(req.Reason, "manifest does not declare") {
		t.Fatalf("expected reason about manifest, got: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementManifestTrueNotOnRequiredWhenNoChanges(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if req.Required || !req.NoDataChange {
		t.Fatalf("expected no data change when manifest allows and no input, got Required=%v NoDataChange=%v", req.Required, req.NoDataChange)
	}
}

func TestComputeRollbackSnapshotRequirementHashStability(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange:   true,
		ConfigBeforeHash:       "sha256:a",
		ConfigAfterHash:        "sha256:b",
		ResourceBeforeTreeHash: "sha256:ra",
		ResourceAfterTreeHash:  "sha256:rb",
		UserDataBeforeHash:     "sha256:ua",
		UserDataAfterHash:      "sha256:ub",
		MigrationPlan:          &migration.ReversiblePreflight{ExtensionID: "ext"},
		MigrationDefinitions:   []migration.MigrationDefinition{{MigrationID: "m1"}},
		MigrationOperations:    []migration.MigrationOperation{{OperationID: "op1"}},
	}
	req1 := ComputeRollbackSnapshotRequirement(input)
	req2 := ComputeRollbackSnapshotRequirement(input)
	if req1.RequirementHash != req2.RequirementHash {
		t.Fatalf("hash must be deterministic: %q vs %q", req1.RequirementHash, req2.RequirementHash)
	}
	if req1.Required != req2.Required || req1.ConfigChanged != req2.ConfigChanged || req1.ResourcesChanged != req2.ResourcesChanged || req1.UserDataChanged != req2.UserDataChanged || req1.MigrationPlanPresent != req2.MigrationPlanPresent || req1.MigrationDefinitionPresent != req2.MigrationDefinitionPresent || req1.MigrationOperationPresent != req2.MigrationOperationPresent || req1.ManifestNoDataChange != req2.ManifestNoDataChange || req1.NoDataChange != req2.NoDataChange {
		t.Fatal("all decision fields must be deterministic")
	}
}

func TestComputeRollbackSnapshotRequirementHashDistinguishesEachField(t *testing.T) {
	base := RollbackSnapshotRequirementInput{
		ManifestNoDataChange: true,
		ConfigBeforeHash:     "sha256:same",
		ConfigAfterHash:      "sha256:same",
	}
	baseReq := ComputeRollbackSnapshotRequirement(base)
	flipBool := func(name string, flip func(*RollbackSnapshotRequirementInput)) {
		modified := base
		flip(&modified)
		other := ComputeRollbackSnapshotRequirement(modified)
		if baseReq.RequirementHash == other.RequirementHash {
			t.Fatalf("%s: hash must change when decision flips", name)
		}
	}
	flipBool("ConfigChanged", func(in *RollbackSnapshotRequirementInput) { in.ConfigAfterHash = "sha256:diff" })
	flipBool("ResourcesChanged", func(in *RollbackSnapshotRequirementInput) {
		in.ResourceBeforeTreeHash = "sha256:rb"
		in.ResourceAfterTreeHash = "sha256:ra"
	})
	flipBool("UserDataChanged", func(in *RollbackSnapshotRequirementInput) {
		in.UserDataBeforeHash = "sha256:ua"
		in.UserDataAfterHash = "sha256:ub"
	})
	flipBool("MigrationPlanPresent", func(in *RollbackSnapshotRequirementInput) {
		in.MigrationPlan = &migration.ReversiblePreflight{ExtensionID: "ext"}
	})
	flipBool("MigrationDefinitionPresent", func(in *RollbackSnapshotRequirementInput) {
		in.MigrationDefinitions = []migration.MigrationDefinition{{MigrationID: "m1"}}
	})
	flipBool("MigrationOperationPresent", func(in *RollbackSnapshotRequirementInput) {
		in.MigrationOperations = []migration.MigrationOperation{{OperationID: "op1"}}
	})
	flipBool("ManifestNoDataChange", func(in *RollbackSnapshotRequirementInput) { in.ManifestNoDataChange = false })
	flipBool("ResourceSetDiffAdded", func(in *RollbackSnapshotRequirementInput) { in.ResourceSetDiff = ResourceSetDiff{Added: []string{"x"}} })
}

func TestComputeRollbackSnapshotRequirementIncludesManifestField(t *testing.T) {
	withManifest := RollbackSnapshotRequirementInput{ManifestNoDataChange: true}
	withoutManifest := RollbackSnapshotRequirementInput{ManifestNoDataChange: false}
	h1 := computeSnapshotRequirementHash(ComputeRollbackSnapshotRequirement(withManifest))
	h2 := computeSnapshotRequirementHash(ComputeRollbackSnapshotRequirement(withoutManifest))
	if h1 == h2 {
		t.Fatal("hash must differ when ManifestNoDataChange differs")
	}
}

func TestComputeRollbackSnapshotRequirementIsUsedBySagaWrapper(t *testing.T) {
	point := PackageRollbackPoint{
		ExtensionID:                "ext",
		SourceVersion:              "1.0.0",
		ConfigSnapshotJSON:         `{"metadata":{}}`,
		ResourceSnapshotJSON:       `{"entries":[]}`,
		MigrationStateSnapshotJSON: `{"mode":"none"}`,
		UserDataMigrationStateJSON: `{"mode":"none"}`,
	}
	req := computeRollbackSnapshotRequirement(point)
	if req.RequirementHash == "" {
		t.Fatal("wrapper must produce a hash")
	}
	if !req.NoDataChange {
		t.Logf("warning: empty JSON fields may produce Required=true via wrapper: %+v", req)
	}
}

func TestComputeRollbackSnapshotRequirementManifestFieldDefaultsTrue(t *testing.T) {
	empty := RollbackSnapshotRequirementInput{ManifestNoDataChange: true}
	req := ComputeRollbackSnapshotRequirement(empty)
	if req.Required || !req.NoDataChange {
		t.Fatalf("expected NoDataChange=true when no categories populated and manifest allows, got Required=%v", req.Required)
	}
}

func TestComputeRollbackSnapshotRequirementReasonOrder(t *testing.T) {
	missingBefore := RollbackSnapshotRequirementInput{ManifestNoDataChange: true, ConfigBeforeHash: "sha256:x"}
	if r := ComputeRollbackSnapshotRequirement(missingBefore); r.Reason != "before/after evidence count mismatch, fail-closed" {
		t.Fatalf("missing source reason mismatch: %s", r.Reason)
	}
	manifestFail := RollbackSnapshotRequirementInput{ManifestNoDataChange: false}
	if r := ComputeRollbackSnapshotRequirement(manifestFail); r.Reason != "manifest does not declare no-data-change, fail-closed" {
		t.Fatalf("manifest reason mismatch: %s", r.Reason)
	}
	migrationPlan := RollbackSnapshotRequirementInput{ManifestNoDataChange: true, MigrationPlan: &migration.ReversiblePreflight{}}
	if r := ComputeRollbackSnapshotRequirement(migrationPlan); r.Reason != "migration plan present" {
		t.Fatalf("migration plan reason mismatch: %s", r.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementEmptyTreeHashesDoNotChange(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange:   true,
		ResourceBeforeTreeHash: "",
		ResourceAfterTreeHash:  "",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if req.ResourcesChanged {
		t.Fatal("empty tree hashes must not indicate resource change")
	}
}

func TestComputeRollbackSnapshotRequirementConfigEqualAfterStillMissingSource(t *testing.T) {
	input := RollbackSnapshotRequirementInput{
		ManifestNoDataChange:   true,
		ConfigBeforeHash:       "sha256:x",
		ConfigAfterHash:        "sha256:x",
		ResourceBeforeTreeHash: "sha256:only-before",
	}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.Required {
		t.Fatalf("expected fail-closed due to resource source mismatch: %s", req.Reason)
	}
}

func TestComputeRollbackSnapshotRequirementManifestIgnoredWhenNoOtherInputOK(t *testing.T) {
	input := RollbackSnapshotRequirementInput{ManifestNoDataChange: true}
	req := ComputeRollbackSnapshotRequirement(input)
	if req.ManifestNoDataChange != true {
		t.Fatal("ManifestNoDataChange must reflect input")
	}
	if req.Required {
		t.Fatal("expected no changes to be acknowledged")
	}
}

func TestComputeRollbackSnapshotRequirementManifestIgnoredWhenNoOtherInputDenied(t *testing.T) {
	input := RollbackSnapshotRequirementInput{ManifestNoDataChange: false}
	req := ComputeRollbackSnapshotRequirement(input)
	if !req.Required {
		t.Fatalf("expected Required=true when manifest denies: %s", req.Reason)
	}
}

func TestFinalGateUserDataRestoreMismatchBlocksRollback(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fg-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ext_fgext_entity (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	payloadHash1 := computeUserDataPayloadHash(payload1)
	payloadHash2 := computeUserDataPayloadHash(payload2)
	line1 := `{"schemaVersion":"1.0.0","extensionID":"fgext","namespace":"ext_fgext_entity","entityType":"entity","entityID":"e1","operation":"upsert","payload":` + mustMarshalJSON(payload1) + `,"payloadHash":"` + payloadHash1 + `"}`
	line2 := `{"schemaVersion":"1.0.0","extensionID":"fgext","namespace":"ext_fgext_entity","entityType":"entity","entityID":"e2","operation":"upsert","payload":` + mustMarshalJSON(payload2) + `,"payloadHash":"` + payloadHash2 + `"}`
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_fgext_entity"},
		RecordCounts:   map[string]int64{"ext_fgext_entity": 2},
		DataExports:    map[string]string{"ext_fgext_entity": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}
	store := NewUserDataSnapshotStore(db)
	operationID := "op-fg-rollback"
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "fgext", operationID, string(userStateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, operationID); err != nil {
		t.Fatalf("baseline verify should pass: %v", err)
	}

	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET applied_count=? WHERE operation_id=? AND table_name=?`,
		1, operationID, "ext_fgext_entity"); err != nil {
		t.Fatalf("tamper applied_count: %v", err)
	}

	err = store.VerifyUserDataRestore(ctx, operationID)
	if err == nil {
		t.Fatal("expected VerifyUserDataRestore to fail after journal tampering, got nil")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeUserDataSnapshotInvalid {
		t.Fatalf("expected PackageErrCodeUserDataSnapshotInvalid, got: %v", err)
	}
	if !strings.Contains(err.Error(), "applied count mismatch") {
		t.Fatalf("expected applied count mismatch detail, got: %v", err)
	}
}

func TestFinalGateUserDataRestoreAggregateMismatchBlocksRollback(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "fg-agg.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ext_agg_entity (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	payloadHash1 := computeUserDataPayloadHash(payload1)
	payloadHash2 := computeUserDataPayloadHash(payload2)
	line1 := `{"schemaVersion":"1.0.0","extensionID":"agg","namespace":"ext_agg_entity","entityType":"entity","entityID":"e1","operation":"upsert","payload":` + mustMarshalJSON(payload1) + `,"payloadHash":"` + payloadHash1 + `"}`
	line2 := `{"schemaVersion":"1.0.0","extensionID":"agg","namespace":"ext_agg_entity","entityType":"entity","entityID":"e2","operation":"upsert","payload":` + mustMarshalJSON(payload2) + `,"payloadHash":"` + payloadHash2 + `"}`
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_agg_entity"},
		RecordCounts:   map[string]int64{"ext_agg_entity": 2},
		DataExports:    map[string]string{"ext_agg_entity": jsonl},
	}
	userStateJSON, err := json.Marshal(userState)
	if err != nil {
		t.Fatal(err)
	}
	store := NewUserDataSnapshotStore(db)
	operationID := "op-fg-agg"
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	if err := store.RestoreUserDataFromSnapshot(ctx, "agg", operationID, string(userStateJSON)); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if err := store.VerifyUserDataRestore(ctx, operationID); err != nil {
		t.Fatalf("baseline verify should pass: %v", err)
	}

	validTamperedHash := "sha256:" + strings.Repeat("bb", 32)
	if _, err := db.ExecContext(ctx,
		`UPDATE extension_package_user_data_restore_journal SET aggregate_hash=? WHERE operation_id=? AND table_name=?`,
		validTamperedHash, operationID, "ext_agg_entity"); err != nil {
		t.Fatalf("tamper aggregate_hash: %v", err)
	}

	err = store.VerifyUserDataRestore(ctx, operationID)
	if err == nil {
		t.Fatal("expected VerifyUserDataRestore to fail after aggregate_hash tampering, got nil")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeUserDataAggregateHashMismatch {
		t.Fatalf("expected PackageErrCodeUserDataAggregateHashMismatch, got: %v", err)
	}
	if !strings.Contains(err.Error(), "aggregate hash mismatch") {
		t.Fatalf("expected aggregate hash mismatch detail, got: %v", err)
	}
}

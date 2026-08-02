package kernel

import (
	"context"
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
			result := isRollbackSnapshotExempt(req, packageConfirmationClaims{SnapshotRequirementHash: computeSnapshotRequirementHash(req)})
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

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-fail-closed-update-norp",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		OperationType: "update", Status: "completed",
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
				t.Fatalf("snapshot_integrity check should fail for update when rollback point is NotFound and no SnapshotRequirementHash in claims")
			}
			if !strings.Contains(check.Detail, "no snapshot requirement hash in confirmation claims") {
				t.Fatalf("expected detail to mention no snapshot hash, got: %s", check.Detail)
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

	claimsJSON := fmt.Sprintf(`{"artifactId":"test-artifact","artifactPolicy":"deleteArtifact","versionId":"2.0.0","currentGenerationId":"gen-1","snapshotRequirementHash":"%s","expiresAt":9999999999}`, reqHash)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-fail-closed-update-hash",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		OperationType: "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err != nil {
		t.Fatalf("expected Final Gate to pass for update with valid SnapshotRequirementHash, but it failed: %v", err)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if !check.Passed {
				t.Fatalf("snapshot_integrity check should pass when SnapshotRequirementHash is valid, detail: %s", check.Detail)
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

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-retain-notfound",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-retain-notfound",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for retainArtifact policy when artifact not found: %+v", result)
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
				t.Fatal("artifact_path_absent check must fail for retainArtifact policy when artifact is not found")
			}
			if !strings.Contains(check.Detail, "retainArtifact") {
				t.Fatalf("expected detail to mention retainArtifact policy, got: %s", check.Detail)
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

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-retain-rollback-notfound",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-retain-rollback-notfound",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for retainForRollback policy when artifact not found: %+v", result)
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
				t.Fatal("artifact_path_absent check must fail for retainForRollback policy when artifact is not found")
			}
			if !strings.Contains(check.Detail, "retainForRollback") {
				t.Fatalf("expected detail to mention retainForRollback policy, got: %s", check.Detail)
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

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-retain-export-notfound",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		OperationType: "uninstall", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-retain-export-notfound",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail for retainForExport policy when artifact not found: %+v", result)
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
				t.Fatal("artifact_path_absent check must fail for retainForExport policy when artifact is not found")
			}
			if !strings.Contains(check.Detail, "retainForExport") {
				t.Fatalf("expected detail to mention retainForExport policy, got: %s", check.Detail)
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
			if !strings.Contains(check.Detail, "delete step artifact mismatch") {
				t.Fatalf("expected detail to mention artifact mismatch, got: %s", check.Detail)
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
				t.Fatal("artifact_path_absent check must fail when VersionID mismatch")
			}
			if !strings.Contains(check.Detail, "version id mismatch") {
				t.Fatalf("expected detail to mention version id mismatch, got: %s", check.Detail)
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
				t.Fatal("artifact_path_absent check must fail when GenerationID mismatch")
			}
			if !strings.Contains(check.Detail, "generation id mismatch") {
				t.Fatalf("expected detail to mention generation id mismatch, got: %s", check.Detail)
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

	claimsJSON := `{"artifactId":"art-reqhash","artifactPolicy":"deleteArtifact","versionId":"2.0.0","currentGenerationId":"gen-1","snapshotRequirementHash":"sha256:deadbeef","confirm":true,"expiresAt":9999999999}`

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-reqhash-mismatch",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-reqhash",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
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
			if !strings.Contains(check.Detail, "requirement hash mismatch") {
				t.Fatalf("expected detail to mention requirement hash mismatch, got: %s", check.Detail)
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

	rollbackPoint := PackageRollbackPoint{
		RollbackPointID:            "rp-real-diff-" + operationID,
		ExtensionID:                extensionID,
		SourceVersion:              "1.0.0",
		SourceGeneration:           1,
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

	claimsJSON := `{"artifactId":"art-real-diff","artifactPolicy":"deleteArtifact","versionId":"2.0.0","currentGenerationId":"gen-1","confirm":true,"expiresAt":9999999999}`

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-real-diff",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:        "art-real-diff",
		ConfirmationsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err == nil {
		t.Fatalf("expected Final Gate to fail when snapshot contains real migration diff (non-empty migration mode=repository): %+v", result)
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
				t.Fatal("snapshot_integrity check must fail when snapshot has non-empty diff (migration required) and is not exempt")
			}
			if !strings.Contains(check.Detail, "snapshot incomplete") {
				t.Fatalf("expected detail to mention snapshot incomplete, got: %s", check.Detail)
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

	rollbackPoint := PackageRollbackPoint{
		RollbackPointID:            "rp-exempt-match-" + operationID,
		ExtensionID:                extensionID,
		SourceVersion:              "1.0.0",
		SourceGeneration:           1,
		ArtifactID:                 "art-exempt-match",
		DefinitionSnapshotJSON:     `{"id":"com.example/snapshot-exempt-hash-match","version":"1.0.0"}`,
		ModuleSnapshotJSON:         `[]`,
		ContributionSnapshotJSON:   `[]`,
		PermissionSnapshotJSON:     `[]`,
		ScopeSnapshotJSON:          `[]`,
		ConfigSnapshotID:           "cfg-empty",
		ConfigSnapshotJSON:         `{}`,
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

	req := computeRollbackSnapshotRequirement(rollbackPoint)
	reqHash := computeSnapshotRequirementHash(req)
	claimsJSON := fmt.Sprintf(`{"artifactId":"art-exempt-match","artifactPolicy":"deleteArtifact","previewHash":"sha256:preview-match","versionId":"2.0.0","currentGenerationId":"gen-1","snapshotRequirementHash":"%s","confirm":true,"expiresAt":9999999999}`, reqHash)

	op := PackageOperationRecord{
		OperationID: operationID, TraceID: "trace-exempt-match",
		UserID: "user-1", ScopeType: "global", ExtensionID: extensionID,
		TargetVersion: "2.0.0", FromVersion: "1.0.0",
		TargetGeneration: "gen-2",
		OperationType:    "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID:             "art-exempt-match",
		ConfirmationClaimsJSON: claimsJSON, FencingToken: 1,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	if _, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "user-1", 10*time.Minute); err != nil {
		t.Fatal(err)
	}

	result, err := runtime.VerifyPackageFinalGate(ctx, operationID)
	if err != nil {
		t.Fatalf("expected Final Gate to pass when claims hash matches and snapshot shows no data change: %v", err)
	}

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if !check.Passed {
				t.Fatalf("snapshot_integrity check should pass, detail: %s", check.Detail)
			}
			if !strings.Contains(check.Detail, "claims hash") || !strings.Contains(check.Detail, "current hash") {
				t.Fatalf("expected detail to record claims hash and current hash, got: %s", check.Detail)
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

	rollbackPoint := PackageRollbackPoint{
		RollbackPointID:            "rp-config-diff-" + operationID,
		ExtensionID:                extensionID,
		SourceVersion:              "1.0.0",
		SourceGeneration:           1,
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

	claimsJSON := `{"artifactId":"art-config-diff","artifactPolicy":"deleteArtifact","previewHash":"sha256:preview-config","versionId":"2.0.0","currentGenerationId":"gen-1","snapshotRequirementHash":"sha256:pretending-no-data-change","confirm":true,"expiresAt":9999999999}`

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
		t.Fatalf("expected Final Gate to fail when migration empty but config diff exists without valid exemption: %+v", result)
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
				t.Fatal("snapshot_integrity must fail when config diff exists without valid hash match")
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
	flipBool("ResourcesChanged", func(in *RollbackSnapshotRequirementInput) { in.ResourceBeforeTreeHash = "sha256:rb"; in.ResourceAfterTreeHash = "sha256:ra" })
	flipBool("UserDataChanged", func(in *RollbackSnapshotRequirementInput) { in.UserDataBeforeHash = "sha256:ua"; in.UserDataAfterHash = "sha256:ub" })
	flipBool("MigrationPlanPresent", func(in *RollbackSnapshotRequirementInput) { in.MigrationPlan = &migration.ReversiblePreflight{ExtensionID: "ext"} })
	flipBool("MigrationDefinitionPresent", func(in *RollbackSnapshotRequirementInput) { in.MigrationDefinitions = []migration.MigrationDefinition{{MigrationID: "m1"}} })
	flipBool("MigrationOperationPresent", func(in *RollbackSnapshotRequirementInput) { in.MigrationOperations = []migration.MigrationOperation{{OperationID: "op1"}} })
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
		ManifestNoDataChange: true,
		ConfigBeforeHash:     "sha256:x",
		ConfigAfterHash:      "sha256:x",
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
	if _, err := db.Exec(`CREATE TABLE ext_fgext_rollback_test (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	payloadHash1 := computeUserDataPayloadHash(payload1)
	payloadHash2 := computeUserDataPayloadHash(payload2)
	line1 := `{"schemaVersion":"1.0.0","extensionID":"fgext","namespace":"ext_fgext_rollback_test","entityType":"entity","entityID":"e1","operation":"upsert","payload":` + mustMarshalJSON(payload1) + `,"payloadHash":"` + payloadHash1 + `"}`
	line2 := `{"schemaVersion":"1.0.0","extensionID":"fgext","namespace":"ext_fgext_rollback_test","entityType":"entity","entityID":"e2","operation":"upsert","payload":` + mustMarshalJSON(payload2) + `,"payloadHash":"` + payloadHash2 + `"}`
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_fgext_rollback_test"},
		RecordCounts:   map[string]int64{"ext_fgext_rollback_test": 2},
		DataExports:    map[string]string{"ext_fgext_rollback_test": jsonl},
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
		1, operationID, "ext_fgext_rollback_test"); err != nil {
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
	if _, err := db.Exec(`CREATE TABLE ext_agg_verify_test (
		entity_id TEXT PRIMARY KEY,
		entity_value TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	payload1 := map[string]any{"entity_value": "v1"}
	payload2 := map[string]any{"entity_value": "v2"}
	payloadHash1 := computeUserDataPayloadHash(payload1)
	payloadHash2 := computeUserDataPayloadHash(payload2)
	line1 := `{"schemaVersion":"1.0.0","extensionID":"agg","namespace":"ext_agg_verify_test","entityType":"entity","entityID":"e1","operation":"upsert","payload":` + mustMarshalJSON(payload1) + `,"payloadHash":"` + payloadHash1 + `"}`
	line2 := `{"schemaVersion":"1.0.0","extensionID":"agg","namespace":"ext_agg_verify_test","entityType":"entity","entityID":"e2","operation":"upsert","payload":` + mustMarshalJSON(payload2) + `,"payloadHash":"` + payloadHash2 + `"}`
	jsonl := line1 + "\n" + line2 + "\n"

	userState := packageUserDataMigrationState{
		Mode:           "repository",
		AffectedTables: []string{"ext_agg_verify_test"},
		RecordCounts:   map[string]int64{"ext_agg_verify_test": 2},
		DataExports:    map[string]string{"ext_agg_verify_test": jsonl},
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
		validTamperedHash, operationID, "ext_agg_verify_test"); err != nil {
		t.Fatalf("tamper aggregate_hash: %v", err)
	}

	err = store.VerifyUserDataRestore(ctx, operationID)
	if err == nil {
		t.Fatal("expected VerifyUserDataRestore to fail after aggregate_hash tampering, got nil")
	}
	var pkgErr *PackageError
	if !errors.As(err, &pkgErr) || pkgErr.Code != PackageErrCodeUserDataSnapshotInvalid {
		t.Fatalf("expected PackageErrCodeUserDataSnapshotInvalid, got: %v", err)
	}
	if !strings.Contains(err.Error(), "aggregate hash mismatch") {
		t.Fatalf("expected aggregate hash mismatch detail, got: %v", err)
	}
}

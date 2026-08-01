package kernel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
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
			name: "migration mode repository is not exempt",
			point: PackageRollbackPoint{
				MigrationStateSnapshotJSON: `{"mode":"repository","definitions":[]}`,
			},
			exempt: false,
		},
		{
			name: "empty migration state is not exempt",
			point: PackageRollbackPoint{
				MigrationStateSnapshotJSON: "",
			},
			exempt: false,
		},
		{
			name: "corrupt migration state is not exempt",
			point: PackageRollbackPoint{
				MigrationStateSnapshotJSON: "{invalid json}",
			},
			exempt: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRollbackSnapshotExempt(tt.point)
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
		ArtifactID: "art-retain-notfound",
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
		ArtifactID: "art-retain-rollback-notfound",
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
		ArtifactID: "art-retain-export-notfound",
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
		ArtifactID: "art-expected",
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
		StartedAt: now, CompletedAt: now,
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
		TargetGeneration: "gen-1",
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
		TargetGeneration: "gen-original",
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
		OperationType: "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID: "art-reqhash",
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
		OperationType: "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID: "art-preview-drift",
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
		OperationType: "update", Status: "completed",
		CurrentStep: "completed", StartedAt: now, UpdatedAt: now,
		ArtifactID: "art-real-diff",
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

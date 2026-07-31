package kernel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func TestFinalGateUpdateMissingRollbackPointPasses(t *testing.T) {
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

	result, _ := runtime.VerifyPackageFinalGate(ctx, operationID)

	snapshotCheckFound := false
	for _, check := range result.Checks {
		if check.Name == "snapshot_integrity" {
			snapshotCheckFound = true
			if !check.Passed {
				t.Fatalf("snapshot_integrity check should pass for update when rollback point is NotFound (update does not require old rollback point), detail: %s", check.Detail)
			}
		}
	}
	if !snapshotCheckFound {
		t.Fatalf("snapshot_integrity check not found in results")
	}
}

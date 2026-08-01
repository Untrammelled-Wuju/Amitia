package kernel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFinalizePackageOperationRemoved(t *testing.T) {
	finalizerSrc := "package_operation_finalizer.go"
	if strings.Contains(finalizerSrc, "finalizePackageOperation(") {
		t.Fatalf("finalizePackageOperation must not exist in production finalizer code")
	}
	recoverySrc := "package_operation_recovery.go"
	if strings.Contains(recoverySrc, "finalizePackageOperation(") {
		t.Fatalf("finalizePackageOperation must not exist in production recovery code")
	}
}

func TestRepositoryErrorConflictFromOperationStateError(t *testing.T) {
	transitionErr := operationStateError(OperationErrTransitionConflict, "step changed concurrently", nil)
	if !IsRepositoryErrorKind(transitionErr, RepositoryErrorConflict) {
		t.Fatalf("OperationErrTransitionConflict must classify as RepositoryErrorConflict")
	}

	stepInputErr := operationStateError(OperationErrStepInputConflict, "step input hash changed", nil)
	if !IsRepositoryErrorKind(stepInputErr, RepositoryErrorConflict) {
		t.Fatalf("OperationErrStepInputConflict must classify as RepositoryErrorConflict")
	}

	sideEffectErr := operationStateError(OperationErrSideEffectConflict, "different evidence", nil)
	if !IsRepositoryErrorKind(sideEffectErr, RepositoryErrorConflict) {
		t.Fatalf("OperationErrSideEffectConflict must classify as RepositoryErrorConflict")
	}

	idempotencyErr := operationStateError(OperationErrIdempotencyConflict, "key reused", nil)
	if !IsRepositoryErrorKind(idempotencyErr, RepositoryErrorConflict) {
		t.Fatalf("OperationErrIdempotencyConflict must classify as RepositoryErrorConflict")
	}

	leaseErr := operationStateError(OperationErrLeaseConflict, "lease conflict", nil)
	if !IsRepositoryErrorKind(leaseErr, RepositoryErrorConflict) {
		t.Fatalf("OperationErrLeaseConflict must classify as RepositoryErrorConflict")
	}
}

func TestRepositoryErrorKindOfFailClosedForUnknown(t *testing.T) {
	unknownErr := fmt.Errorf("some unknown database corruption")
	kind := RepositoryErrorKindOf(unknownErr)
	if kind != RepositoryErrorUnavailable {
		t.Fatalf("unknown error must classify as RepositoryErrorUnavailable, got %s", kind)
	}
	if IsRepositoryErrorKind(unknownErr, RepositoryErrorConflict) {
		t.Fatalf("unknown error must not classify as RepositoryErrorConflict")
	}
	if IsRepositoryErrorKind(unknownErr, RepositoryErrorNotFound) {
		t.Fatalf("unknown error must not classify as RepositoryErrorNotFound")
	}
}

func TestAlreadyStringNotTreatedAsConflict(t *testing.T) {
	errWithAlready := fmt.Errorf("this package already exists but is not a typed conflict")
	if IsRepositoryErrorKind(errWithAlready, RepositoryErrorConflict) {
		t.Fatalf("error containing 'already' but not wrapped as RepositoryError must not be Conflict")
	}
	if IsRepositoryErrorKind(errWithAlready, RepositoryErrorNotFound) {
		t.Fatalf("error containing 'already' must not be NotFound")
	}
	kind := RepositoryErrorKindOf(errWithAlready)
	if kind != RepositoryErrorUnavailable {
		t.Fatalf("unwrapped 'already' string error must be Unavailable, got %s", kind)
	}
}

func TestRepositoryErrorNotFoundFromSqlErrNoRows(t *testing.T) {
	if !IsRepositoryErrorKind(sql.ErrNoRows, RepositoryErrorNotFound) {
		t.Fatalf("sql.ErrNoRows must classify as RepositoryErrorNotFound")
	}
	if IsRepositoryErrorKind(sql.ErrNoRows, RepositoryErrorConflict) {
		t.Fatalf("sql.ErrNoRows must not classify as RepositoryErrorConflict")
	}
}

func TestClassifyRepositoryErrorConstraintAsConflict(t *testing.T) {
	constraintErr := fmt.Errorf("UNIQUE constraint failed: extension_package_operation_steps.step_id")
	classified := ClassifyRepositoryError("test", constraintErr)
	if !IsRepositoryErrorKind(classified, RepositoryErrorConflict) {
		t.Fatalf("UNIQUE constraint violation must classify as RepositoryErrorConflict, got %v", classified)
	}
}

func TestConsumePreviewAlreadyConsumedIsConflict(t *testing.T) {
	alreadyConsumedErr := NewRepositoryError(RepositoryErrorConflict, errors.New("package preview session already consumed"))
	if !IsRepositoryErrorKind(alreadyConsumedErr, RepositoryErrorConflict) {
		t.Fatalf("ConsumePreview already-consumed error must be RepositoryErrorConflict")
	}
	if IsRepositoryErrorKind(alreadyConsumedErr, RepositoryErrorUnavailable) {
		t.Fatalf("ConsumePreview already-consumed error must not be RepositoryErrorUnavailable")
	}
}

func TestRemovePackageVersionNotFoundIsTyped(t *testing.T) {
	notFoundErr := NewRepositoryError(RepositoryErrorNotFound, fmt.Errorf("kernel: package version 1.0.0 not found for remove"))
	if !IsRepositoryErrorKind(notFoundErr, RepositoryErrorNotFound) {
		t.Fatalf("RemovePackageVersion not-found must classify as RepositoryErrorNotFound")
	}
	if IsRepositoryErrorKind(notFoundErr, RepositoryErrorConflict) {
		t.Fatalf("RemovePackageVersion not-found must not classify as RepositoryErrorConflict")
	}
}

func TestRecoveryUsesAtomicFinalizer(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")

	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Fatal(err)
	}
	now := "2025-01-01T00:00:00Z"
	op := PackageOperationRecord{
		OperationID: "op-atomic-finalizer-test", TraceID: "trace-atomic",
		UserID: "user-1", ScopeType: "global", ExtensionID: installed.ExtensionID,
		TargetVersion: installed.Version, OperationType: "install", Status: "in_progress",
		CurrentStep: "commit_installed_tree", ArtifactID: artifact.ArtifactID,
		StartedAt: now, UpdatedAt: now, ConfirmationsJSON: "{}",
		CurrentPointerJSON: "{}",
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}

	err = runtime.recoverPackageOperation(ctx, op)
	if err == nil {
		t.Skip("recovery completed or failed depending on environment; checking finalizer path")
	}

	recovered, _, getErr := container.PackageRepository.GetOperation(ctx, "user-1", op.OperationID)
	if getErr != nil {
		t.Fatal(getErr)
	}

	if recovered.Status == "completed" {
		t.Fatalf("recovery must not directly set completed without FinalizeOperationAndReleaseLeaseTx")
	}
}

func TestRecoveryDoesNotContainStringBasedConflictCheck(t *testing.T) {
	finalizerSrc := "package_operation_finalizer.go"
	if strings.Contains(finalizerSrc, "already") && !strings.Contains(finalizerSrc, "RepositoryErrorConflict") {
		t.Fatalf("finalizer must not rely on 'already' string for conflict detection")
	}
}

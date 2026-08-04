package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func newFinalizationOperation(t *testing.T, runtime *Runtime, container *Container, operationID, extensionID, operationType string) (PackageOperationRecord, PackageWriteGuard) {
	t.Helper()
	ctx := context.Background()
	op := operationFixture(operationID, "user-1", "finalize-"+operationID, "sha256:"+operationID, extensionID)
	op.OperationType = operationType
	op.TargetVersion = ""
	op.FromVersion = ""
	confirmKey := "confirm." + operationType
	secHash := computeSecurityPolicyHash()
	previewHash := "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	nonce := "test-nonce-" + operationID
	claims := fmt.Sprintf(`{"schemaVersion":1,"operationType":%q,"extensionId":%q,"artifactId":%q,"previewHash":%q,"securityPolicyHash":%q,"policyVersion":%q,"userId":%q,"scopeType":%q,"scopeId":%q,"confirmedItems":[%q],"confirmations":{%q:true},"issuedAt":%d,"expiresAt":%d,"nonce":%q}`,
		operationType, extensionID, op.ArtifactID, previewHash, secHash, "2026-07-30-v1",
		op.UserID, op.ScopeType, op.ScopeID, confirmKey, confirmKey,
		time.Now().Unix(), time.Now().Add(time.Hour).Unix(), nonce)
	op.ConfirmationClaimsJSON = claims
	if _, _, err := container.PackageRepository.CreateOrGetOperationWithConfirmationNonce(ctx, op, PackageConfirmationNonceBinding{Nonce: nonce}); err != nil {
		t.Fatal(err)
	}
	if err := container.PackageRepository.TransitionOperation(ctx, op.OperationID, []PackageOperationStatus{PackageOperationPending}, PackageOperationInProgress, PackageOperationTransition{CurrentStep: "prepared"}, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	lease, err := container.PackageRepository.AcquireExtensionLease(ctx, extensionID, operationID, "worker-1", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return op, PackageWriteGuard{ExtensionID: extensionID, FencingToken: lease.FencingToken}
}

func addFinalizationUninstallStep(t *testing.T, container *Container, op PackageOperationRecord, guard PackageWriteGuard) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	step := PackageOperationStep{
		StepID:       "step-" + op.OperationID,
		OperationID:  op.OperationID,
		StepName:     "cleanup_kernel_repositories",
		StepOrder:    3,
		Status:       "completed",
		AttemptCount: 1,
		ResultJSON:   "{}",
		StartedAt:    now,
		CompletedAt:  now,
	}
	if err := container.PackageRepository.PutStep(context.Background(), step, guard); err != nil {
		t.Fatal(err)
	}
}

func finalizationOperationRecord(t *testing.T, container *Container, operationID string) PackageOperationRecord {
	t.Helper()
	op, _, err := container.PackageRepository.GetOperation(context.Background(), "user-1", operationID)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func requireFinalizationError(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected finalization error")
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected error to contain %q, got %v", fragment, err)
		}
	}
}

func requireFinalizationPersistenceFailure(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected finalization error")
	}
	for _, fragment := range fragments {
		if !errorChainContains(err, fragment) {
			t.Fatalf("expected error chain to contain %q, got %v", fragment, err)
		}
	}
	var stateErr *PackageOperationStateError
	if !errors.As(err, &stateErr) || stateErr.Code != OperationErrStorageFailure {
		t.Fatalf("expected operation storage failure in combined error, got %v", err)
	}
}

func errorChainContains(err error, fragment string) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), fragment) {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range joined.Unwrap() {
			if errorChainContains(child, fragment) {
				return true
			}
		}
		return false
	}
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		return errorChainContains(unwrapped, fragment)
	}
	return false
}

func failRequiresRecoveryPersistence(t *testing.T, container *Container) {
	t.Helper()
	_, err := container.PackageRepository.DB().Exec(`CREATE TRIGGER finalize_requires_recovery_fail BEFORE UPDATE OF status ON extension_package_operations WHEN NEW.status='requires_recovery' BEGIN SELECT RAISE(FAIL, 'forced requires_recovery persistence failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
}

func failFinalizationCompletion(t *testing.T, container *Container) {
	t.Helper()
	_, err := container.PackageRepository.DB().Exec(`CREATE TRIGGER finalize_completion_fail BEFORE UPDATE OF status ON extension_package_operations WHEN NEW.status='completed' BEGIN SELECT RAISE(FAIL, 'forced finalization completion failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
}

func failReleasePendingPersistence(t *testing.T, container *Container) {
	t.Helper()
	_, err := container.PackageRepository.DB().Exec(`CREATE TRIGGER finalize_release_pending_fail BEFORE UPDATE OF status ON extension_package_operations WHEN NEW.status='release_pending' BEGIN SELECT RAISE(FAIL, 'forced release_pending persistence failure'); END;`)
	if err != nil {
		t.Fatal(err)
	}
}

func takeOverLeaseAfterFinalGate(t *testing.T, container *Container) {
	t.Helper()
	_, err := container.PackageRepository.DB().Exec(`CREATE TRIGGER finalize_lease_takeover AFTER UPDATE OF status ON extension_package_operation_steps WHEN NEW.step_name='final_gate_verification' AND NEW.status='completed' BEGIN UPDATE extension_package_operation_leases SET fencing_token=fencing_token+1 WHERE extension_id=(SELECT extension_id FROM extension_package_operations WHERE operation_id=NEW.operation_id); END;`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFinalizePackageOperationLeaseAssertFailurePersistsRequiresRecovery(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	op, guard := newFinalizationOperation(t, runtime, container, "op-finalize-lease-assert", "com.example/finalize-lease-assert", "install")
	leaseGuard := &PackageLeaseGuard{lastErr: errors.New("simulated lease lost")}

	err := runtime.FinalizePackageOperation(ctx, op.OperationID, op.ExtensionID, leaseGuard, guard)
	requireFinalizationError(t, err, "simulated lease lost")

	record := finalizationOperationRecord(t, container, op.OperationID)
	if record.Status != string(PackageOperationRequiresRecovery) || record.CurrentStep != "finalize_assert_lease" || record.ErrorCode != PackageErrCodeLeaseLost {
		t.Fatalf("unexpected operation state: %+v", record)
	}
}

func TestFinalizePackageOperationLeaseAssertPersistenceFailureReturnsCombinedError(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	op, guard := newFinalizationOperation(t, runtime, container, "op-finalize-lease-assert-fail", "com.example/finalize-lease-assert-fail", "install")
	leaseGuard := &PackageLeaseGuard{lastErr: errors.New("simulated lease lost")}
	failRequiresRecoveryPersistence(t, container)

	err := runtime.FinalizePackageOperation(ctx, op.OperationID, op.ExtensionID, leaseGuard, guard)
	requireFinalizationPersistenceFailure(t, err, "simulated lease lost", "forced requires_recovery persistence failure")

	record := finalizationOperationRecord(t, container, op.OperationID)
	if record.Status != string(PackageOperationInProgress) {
		t.Fatalf("unexpected operation state: %+v", record)
	}
}

func TestFinalizePackageOperationFinalGateFailurePersistsRequiresRecovery(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	op, guard := newFinalizationOperation(t, runtime, container, "op-finalize-final-gate", "com.example/finalize-final-gate", "install")

	err := runtime.FinalizePackageOperation(ctx, op.OperationID, op.ExtensionID, nil, guard)
	requireFinalizationError(t, err, PackageErrCodeFinalGateFailed)

	record := finalizationOperationRecord(t, container, op.OperationID)
	if record.Status != string(PackageOperationRequiresRecovery) || record.CurrentStep != "final_gate" || record.ErrorCode != PackageErrCodeFinalGateFailed {
		t.Fatalf("unexpected operation state: %+v", record)
	}
}

func TestFinalizePackageOperationFinalGatePersistenceFailureReturnsCombinedError(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	op, guard := newFinalizationOperation(t, runtime, container, "op-finalize-final-gate-fail", "com.example/finalize-final-gate-fail", "install")
	failRequiresRecoveryPersistence(t, container)

	err := runtime.FinalizePackageOperation(ctx, op.OperationID, op.ExtensionID, nil, guard)
	requireFinalizationPersistenceFailure(t, err, PackageErrCodeFinalGateFailed, "forced requires_recovery persistence failure")

	record := finalizationOperationRecord(t, container, op.OperationID)
	if record.Status != string(PackageOperationFinalizing) {
		t.Fatalf("unexpected operation state: %+v", record)
	}
}

func TestFinalizePackageOperationLeaseConflictPersistsRequiresRecovery(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	op, guard := newFinalizationOperation(t, runtime, container, "op-finalize-lease-conflict", "com.example/finalize-lease-conflict", "uninstall")
	addFinalizationUninstallStep(t, container, op, guard)
	takeOverLeaseAfterFinalGate(t, container)

	err := runtime.FinalizePackageOperation(ctx, op.OperationID, op.ExtensionID, nil, guard)
	requireFinalizationError(t, err, PackageErrCodeLeaseFenced)

	record := finalizationOperationRecord(t, container, op.OperationID)
	if record.Status != string(PackageOperationRequiresRecovery) || record.CurrentStep != "finalize_lease_release" || record.ErrorCode != PackageErrCodeLeaseFenced {
		t.Fatalf("unexpected operation state: %+v", record)
	}
}

func TestFinalizePackageOperationReleasePendingWhenLeaseReleaseFails(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	op, guard := newFinalizationOperation(t, runtime, container, "op-finalize-release-pending", "com.example/finalize-release-pending", "uninstall")
	addFinalizationUninstallStep(t, container, op, guard)
	failFinalizationCompletion(t, container)

	err := runtime.FinalizePackageOperation(ctx, op.OperationID, op.ExtensionID, nil, guard)
	requireFinalizationError(t, err, "PACKAGE_LEASE_RELEASE_FAILED")

	record := finalizationOperationRecord(t, container, op.OperationID)
	if record.Status != string(PackageOperationReleasePending) || record.CurrentStep != "finalize_lease_release" || record.ErrorCode != "PACKAGE_LEASE_RELEASE_FAILED" {
		t.Fatalf("unexpected operation state: %+v", record)
	}
}

func TestFinalizePackageOperationReleasePendingPersistenceFailureReturnsCombinedError(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	op, guard := newFinalizationOperation(t, runtime, container, "op-finalize-release-pending-fail", "com.example/finalize-release-pending-fail", "uninstall")
	addFinalizationUninstallStep(t, container, op, guard)
	failFinalizationCompletion(t, container)
	failReleasePendingPersistence(t, container)

	err := runtime.FinalizePackageOperation(ctx, op.OperationID, op.ExtensionID, nil, guard)
	requireFinalizationPersistenceFailure(t, err, "PACKAGE_LEASE_RELEASE_FAILED", "forced release_pending persistence failure")

	record := finalizationOperationRecord(t, container, op.OperationID)
	if record.Status != string(PackageOperationFinalizing) {
		t.Fatalf("unexpected operation state: %+v", record)
	}
}

package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func newFinalizationOperation(
	t *testing.T,
	runtime *Runtime,
	container *Container,
	operationID string,
	extensionID string,
	operationType string,
) (
	PackageOperationRecord,
	PackageWriteGuard,
) {
	t.Helper()

	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)

	operation := operationFixture(
		operationID,
		"user-1",
		"finalize-"+operationID,
		"sha256:"+operationID,
		extensionID,
	)

	operation.OperationType = operationType

	operation.TargetVersion = ""
	operation.TargetGeneration = ""
	operation.FromVersion = ""
	operation.ArtifactID = "artifact-" + operationID
	operation.PreviewSessionID = ""
	operation.SnapshotRequirementHash = "sha256:snapshot-requirement-" + operationID

	requiredConfirmations := []string{}
	dependencies := []string{}

	artifactPolicy := ArtifactPolicy("")

	if operationType == string(PackageOperationTypeUninstall) {
		artifactPolicy = ArtifactPolicyRetainArtifact
	}

	claims := PackageConfirmationClaims{
		SchemaVersion:           PackageConfirmationClaimsSchemaVersion,
		OperationType:           operation.OperationType,
		ExtensionID:             operation.ExtensionID,
		ArtifactID:              operation.ArtifactID,
		ArtifactPolicy:          artifactPolicy,
		PreviewSessionID:        operation.PreviewSessionID,
		PreviewHash:             "sha256:preview-" + operationID,
		SecurityPolicyHash:      computeSecurityPolicyHash(),
		SnapshotRequirementHash: operation.SnapshotRequirementHash,
		RequiredConfirmationsHash: computePackageRequiredConfirmationsHash(
			requiredConfirmations,
		),
		DependenciesHash: computePackageDependenciesHash(
			dependencies,
		),
		PolicyVersion:  packagePolicyVersion,
		UserID:         operation.UserID,
		ScopeType:      operation.ScopeType,
		ScopeID:        operation.ScopeID,
		ConfirmedItems: requiredConfirmations,
		Confirmations:  map[string]bool{},
		IssuedAt:       now.Unix(),
		ExpiresAt:      now.Add(5 * time.Minute).Unix(),
		Nonce:          "test-nonce-" + operationID,
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal finalization claims: %v", err)
	}

	operation.ConfirmationClaimsJSON = string(claimsJSON)
	operation.ConfirmationsJSON = "{}"

	if err := container.PackageRepository.PutArtifact(
		ctx,
		PackageArtifact{
			ArtifactID:     operation.ArtifactID,
			ExtensionID:    operation.ExtensionID,
			Version:        "1.0.0",
			RetentionState: "active",
			CreatedAt:      now.Format(time.RFC3339Nano),
		},
	); err != nil {
		t.Fatalf("put finalization artifact: %v", err)
	}

	binding := validTestConfirmationNonceBinding(
		operation,
		claims.Nonce,
		now,
	)

	existing, created, err := container.PackageRepository.CreateOrGetOperationWithConfirmationNonce(
		ctx,
		operation,
		binding,
	)
	if err != nil {
		t.Fatalf("create finalization operation with nonce: %v", err)
	}

	if !created {
		t.Fatal("finalization fixture unexpectedly reused operation")
	}

	if existing.OperationID != operation.OperationID {
		t.Fatalf(
			"unexpected authoritative operation: got=%s want=%s",
			existing.OperationID,
			operation.OperationID,
		)
	}

	if err := container.PackageRepository.TransitionOperation(
		ctx,
		operation.OperationID,
		[]PackageOperationStatus{PackageOperationPending},
		PackageOperationInProgress,
		PackageOperationTransition{
			CurrentStep: "prepared",
		},
		PackageWriteGuard{},
	); err != nil {
		t.Fatalf("transition finalization operation: %v", err)
	}

	lease, err := container.PackageRepository.AcquireExtensionLease(
		ctx,
		extensionID,
		operationID,
		"worker-1",
		10*time.Minute,
	)
	if err != nil {
		t.Fatalf("acquire finalization lease: %v", err)
	}

	guard := PackageWriteGuard{
		ExtensionID:  extensionID,
		FencingToken: lease.FencingToken,
	}

	authorityInput := PackageConfirmationAuthorityInput{
		SchemaVersion:           packageConfirmationAuthorityInputSchemaVersion,
		Source:                  packageConfirmationAuthoritySourcePostLeasePreview,
		OperationType:           operation.OperationType,
		ExtensionID:             operation.ExtensionID,
		ArtifactID:              operation.ArtifactID,
		PreviewSessionID:        operation.PreviewSessionID,
		PreviewHash:             claims.PreviewHash,
		SecurityPolicyHash:      claims.SecurityPolicyHash,
		SnapshotRequirementHash: claims.SnapshotRequirementHash,
		ArtifactPolicy:          claims.ArtifactPolicy,
		Dependencies:            dependencies,
		RequiredConfirmations:   requiredConfirmations,
		CapturedAt:              now.Format(time.RFC3339Nano),
	}

	evidence, err := buildPackageConfirmationAuthorityEvidence(
		operation.OperationID,
		claims,
		authorityInput,
	)
	if err != nil {
		t.Fatalf("build finalization authority evidence: %v", err)
	}

	if err := runtime.persistPackageConfirmationAuthorityEvidence(
		ctx,
		evidence,
		guard,
	); err != nil {
		t.Fatalf("persist finalization authority evidence: %v", err)
	}

	authoritative, _, err := container.PackageRepository.GetOperation(
		ctx,
		operation.UserID,
		operation.OperationID,
	)
	if err != nil {
		t.Fatalf("reload authoritative finalization operation: %v", err)
	}

	if authoritative.FencingToken != lease.FencingToken {
		t.Fatalf(
			"operation fencing token mismatch: operation=%d lease=%d",
			authoritative.FencingToken,
			lease.FencingToken,
		)
	}

	return authoritative, guard
}

func addFinalizationUninstallStep(
	t *testing.T,
	container *Container,
	operation PackageOperationRecord,
	guard PackageWriteGuard,
) {
	t.Helper()

	ctx := context.Background()

	now := time.Now().UTC().Format(time.RFC3339Nano)

	cleanupResultJSON := "{}"
	cleanupResultHash := sha256.Sum256([]byte(cleanupResultJSON))

	cleanupStep := PackageOperationStep{
		StepID:       "step-cleanup-" + operation.OperationID,
		OperationID:  operation.OperationID,
		StepName:     "cleanup_kernel_repositories",
		StepOrder:    3,
		Status:       StatusCompleted,
		AttemptCount: 1,
		ResultJSON:   cleanupResultJSON,
		ResultHash:   fmt.Sprintf("%x", cleanupResultHash),
		StartedAt:    now,
		CompletedAt:  now,
	}

	if err := container.PackageRepository.PutStep(ctx, cleanupStep, guard); err != nil {
		t.Fatalf("put cleanup step: %v", err)
	}

	removeResult := RemoveArtifactStepResult{
		ArtifactID:     operation.ArtifactID,
		ExtensionID:    operation.ExtensionID,
		ArtifactPolicy: ArtifactPolicyRetainArtifact,
		Deleted:        false,
		Retained:       true,
		RetentionState: "active",
		RemainingRefs:  0,
	}

	removeResult.EvidenceHash = computeArtifactStepEvidenceHash(removeResult)

	removeResultJSON, err := json.Marshal(removeResult)
	if err != nil {
		t.Fatalf("marshal remove artifact step: %v", err)
	}

	removeResultHash := sha256.Sum256(removeResultJSON)

	removeStep := PackageOperationStep{
		StepID:       "step-remove-artifact-" + operation.OperationID,
		OperationID:  operation.OperationID,
		StepName:     StepRemoveArtifact,
		StepOrder:    31,
		Status:       StatusCompleted,
		AttemptCount: 1,
		ResultJSON:   string(removeResultJSON),
		ResultHash:   fmt.Sprintf("%x", removeResultHash),
		StartedAt:    now,
		CompletedAt:  now,
	}

	if err := container.PackageRepository.PutStep(ctx, removeStep, guard); err != nil {
		t.Fatalf("put remove artifact step: %v", err)
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

func TestFinalizationUninstallFixturePassesFinalGate(t *testing.T) {
	ctx := context.Background()

	runtime, container := newPackagePipelineRuntime(t)

	operation, guard := newFinalizationOperation(
		t,
		runtime,
		container,
		"op-finalize-fixture-validation",
		"com.example/finalize-fixture-validation",
		string(PackageOperationTypeUninstall),
	)

	addFinalizationUninstallStep(t, container, operation, guard)

	result, err := runtime.VerifyPackageFinalGate(ctx, operation.OperationID)
	if err != nil {
		t.Fatalf("valid uninstall finalization fixture rejected: %v; checks=%+v", err, result.Checks)
	}

	if !result.Passed {
		t.Fatalf("valid uninstall finalization fixture did not pass: %+v", result.Checks)
	}

	requiredChecks := map[string]bool{
		"claims_verified":          false,
		"artifact_path_absent":     false,
		"valid_lease":              false,
		"step_result_integrity":    false,
		"operation_steps_complete": false,
	}

	for _, check := range result.Checks {
		if check.Passed {
			if _, exists := requiredChecks[check.Name]; exists {
				requiredChecks[check.Name] = true
			}
		}
	}

	for name, passed := range requiredChecks {
		if !passed {
			t.Fatalf("required Final Gate check %s did not pass: %+v", name, result.Checks)
		}
	}
}

func TestFinalizationFixturePersistsCompleteNonceBinding(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)

	operation, _ := newFinalizationOperation(
		t,
		runtime,
		container,
		"op-finalize-nonce-binding",
		"com.example/finalize-nonce-binding",
		string(PackageOperationTypeInstall),
	)

	claims, err := parseOperationConfirmationClaims(operation)
	if err != nil {
		t.Fatalf("parse finalization claims: %v", err)
	}

	var record PackageConfirmationNonceRecord

	err = container.PackageRepository.DB().QueryRow(
		`SELECT
			nonce,
			operation_id,
			operation_type,
			extension_id,
			user_id,
			issued_at,
			expires_at,
			consumed_at
		 FROM extension_package_confirmation_nonces
		 WHERE operation_id=?`,
		operation.OperationID,
	).Scan(
		&record.Nonce,
		&record.OperationID,
		&record.OperationType,
		&record.ExtensionID,
		&record.UserID,
		&record.IssuedAt,
		&record.ExpiresAt,
		&record.ConsumedAt,
	)

	if err != nil {
		t.Fatalf("query finalization nonce binding: %v", err)
	}

	if record.Nonce != claims.Nonce ||
		record.OperationID != operation.OperationID ||
		record.OperationType != operation.OperationType ||
		record.ExtensionID != operation.ExtensionID ||
		record.UserID != operation.UserID {
		t.Fatalf("finalization nonce identity mismatch: %+v", record)
	}

	if record.IssuedAt != confirmationTimestamp(claims.IssuedAt) ||
		record.ExpiresAt != confirmationTimestamp(claims.ExpiresAt) {
		t.Fatalf("finalization nonce temporal mismatch: %+v", record)
	}

	if record.ConsumedAt == "" {
		t.Fatal("finalization nonce consumedAt missing")
	}
}

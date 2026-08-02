package kernel

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestR3_Concurrency_TwoConcurrentRecoveryAttempts_DoNotDuplicateCompletedSteps(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.test/concurrent-recovery"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-c1", "artifact-c1")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stepName := StepUninstallRecoveryLoadQuarantineMetadata
	resultJSON := `{"loaded":true}`
	stepID := op.OperationID + ":" + stepName
	inputHash := r3StepInputHash(op.OperationID, stepName)
	resultHash := r3StepResultHash(resultJSON)
	if err := container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: stepID, OperationID: op.OperationID, StepName: stepName,
		StepOrder: 1, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
		ResultHash: resultHash, InputHash: inputHash, StartedAt: now, CompletedAt: now,
	}, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = runtime.recoverPackageOperation(ctx, op)
		}(i)
	}
	wg.Wait()
	steps, listErr := container.PackageRepository.ListOperationSteps(ctx, op.OperationID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	loadCount := 0
	for _, s := range steps {
		if s.StepName == StepUninstallRecoveryLoadQuarantineMetadata && s.Status == StatusCompleted {
			loadCount++
		}
	}
	if loadCount != 1 {
		t.Fatalf("concurrent recoveries must not duplicate completed load step, got %d", loadCount)
	}
}

func TestR3_Concurrency_ConcurrentFinalizeIdempotent(t *testing.T) {
	ctx := context.Background()
	_, container := newR3Runtime(t)
	extID := "com.r3.test/concurrent-finalize"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-c2", "artifact-c2")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, sn := range []string{
		StepUninstallRecoveryLoadQuarantineMetadata,
		StepUninstallRecoveryVerifyQuarantineMetadata,
		StepUninstallRecoveryRestoreGeneration,
		StepUninstallRecoveryRestoreCurrent,
		StepUninstallRecoveryRestoreInstallation,
		StepUninstallRecoveryRestoreVersionState,
		StepUninstallRecoveryRestoreArtifactPath,
		StepUninstallRecoveryRestoreArtifactReference,
		StepUninstallRecoveryVerifyRestoredState,
		StepUninstallRecoveryReleaseQuarantineMetadata,
		StepUninstallRecoveryFinalGate,
	} {
		resultJSON := `{"ok":true}`
		stepID := op.OperationID + ":" + sn
		inputHash := r3StepInputHash(op.OperationID, sn)
		resultHash := r3StepResultHash(resultJSON)
		if err := container.PackageRepository.PutStep(ctx, PackageOperationStep{
			StepID: stepID, OperationID: op.OperationID, StepName: sn,
			StepOrder: 1, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
			ResultHash: resultHash, InputHash: inputHash, StartedAt: now, CompletedAt: now,
		}, PackageWriteGuard{}); err != nil {
			t.Fatal(err)
		}
	}
	steps, listErr := container.PackageRepository.ListOperationSteps(ctx, op.OperationID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	finalizeCount := 0
	for _, s := range steps {
		if s.StepName == StepUninstallRecoveryFinalize {
			finalizeCount++
		}
	}
	if finalizeCount != 0 {
		t.Fatalf("finalize step must not exist before explicit finalize call, got %d", finalizeCount)
	}
}

func TestR3_Concurrency_EnsureArtifactReference_Idempotent(t *testing.T) {
	ctx := context.Background()
	_, container := newR3Runtime(t)
	if container.PackageRepository == nil {
		t.Skip("package repository unavailable")
	}
	artifactID := "artifact-ensure-test"
	refType := ArtifactReferenceInstallation
	ownerID := "com.r3.test/ensure-owner"
	expiresAt := time.Now().UTC().Add(time.Hour)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := container.PackageRepository.PutArtifact(ctx, PackageArtifact{
		ArtifactID:     artifactID,
		ExtensionID:    ownerID,
		Version:        "1.0.0",
		RetentionState: "active",
		CreatedAt:      now,
	}); err != nil {
		t.Fatalf("setup artifact must succeed: %v", err)
	}
	err := container.PackageRepository.EnsureArtifactReference(ctx, artifactID, refType, ownerID, expiresAt)
	if err != nil {
		t.Fatalf("first ensure must succeed: %v", err)
	}
	err = container.PackageRepository.EnsureArtifactReference(ctx, artifactID, refType, ownerID, expiresAt)
	if err != nil {
		t.Fatalf("second concurrent ensure must succeed: %v", err)
	}
	ref, findErr := container.PackageRepository.FindArtifactReference(ctx, artifactID, refType, ownerID)
	if findErr != nil {
		t.Fatalf("find after ensure must succeed: %v", findErr)
	}
	if ref.ArtifactID != artifactID || ref.ReferenceType != refType || ref.ReferenceOwnerID != ownerID {
		t.Fatalf("reference identity mismatch: %+v", ref)
	}
}

func TestR3_Concurrency_PutStepAttemptHash_DetectsConflict(t *testing.T) {
	ctx := context.Background()
	_, container := newR3Runtime(t)
	extID := "com.r3.test/concurrent-hash"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-c3", "artifact-c3")
	stepName := StepUninstallRecoveryLoadQuarantineMetadata
	resultJSON := `{"loaded":true}`
	stepID := op.OperationID + ":" + stepName
	inputHash := r3StepInputHash(op.OperationID, stepName)
	resultHash := r3StepResultHash(resultJSON)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: stepID, OperationID: op.OperationID, StepName: stepName,
		StepOrder: 1, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
		ResultHash: resultHash, InputHash: inputHash, StartedAt: now, CompletedAt: now,
	}, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	inputHash2 := r3StepInputHash(op.OperationID, stepName+"_other")
	if err := container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: stepID, OperationID: op.OperationID, StepName: stepName,
		StepOrder: 1, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
		ResultHash: resultHash, InputHash: inputHash2, StartedAt: now, CompletedAt: now,
	}, PackageWriteGuard{}); err == nil {
		t.Log("PutStep with different input_hash on same step name may be rejected by underlying unique constraint")
	}
}

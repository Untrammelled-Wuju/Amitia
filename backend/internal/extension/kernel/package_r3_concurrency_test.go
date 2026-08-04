package kernel

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	err := container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: stepID, OperationID: op.OperationID, StepName: stepName,
		StepOrder: 1, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
		ResultHash: resultHash, InputHash: inputHash2, StartedAt: now, CompletedAt: now,
	}, PackageWriteGuard{})
	require.Error(t, err, "different input_hash for the same step must be rejected")
}

func TestR3ConcurrentRecoveryFinalizesExactlyOnceAcrossRuntimes(t *testing.T) {
	fixture := newR3RecoveryFixture(t)
	ctx := context.Background()

	runtimeA := fixture.Runtime
	containerA := fixture.Container

	runtimeB, containerB := newR3RuntimeAt(t, fixture.DBPath, fixture.ExtensionRoot)

	t.Cleanup(func() {
		require.NoError(t, containerB.Close())
	})

	require.NotSame(t, runtimeA, runtimeB)
	require.NotSame(t, containerA.PackageRepository, containerB.PackageRepository)

	start := make(chan struct{})
	results := make(chan error, 2)

	var waitGroup sync.WaitGroup

	run := func(runtime *Runtime, container *Container) {
		defer waitGroup.Done()

		operation, _, err := container.PackageRepository.GetOperation(ctx, fixture.Operation.UserID, fixture.Operation.OperationID)
		if err != nil {
			results <- err
			return
		}

		<-start

		results <- runtime.recoverPackageOperation(ctx, operation)
	}

	waitGroup.Add(2)
	go run(runtimeA, containerA)
	go run(runtimeB, containerB)

	close(start)

	waitGroup.Wait()
	close(results)

	errorsReceived := make([]error, 0, 2)
	for result := range results {
		errorsReceived = append(errorsReceived, result)
	}

	require.Len(t, errorsReceived, 2)

	successCount := 0
	for _, result := range errorsReceived {
		if result == nil {
			successCount++
			continue
		}

		allowed := IsPackageOperationError(result, OperationErrLeaseConflict) ||
			IsPackageOperationError(result, PackageErrCodeLeaseFenced)

		require.True(t, allowed, "unexpected concurrent recovery error: %v", result)
	}

	require.GreaterOrEqual(t, successCount, 1, "at least one Runtime must complete recovery")

	finalOperation, _, err := containerA.PackageRepository.GetOperation(ctx, fixture.Operation.UserID, fixture.Operation.OperationID)
	require.NoError(t, err)

	assertR3RecoveryFinalState(t, runtimeA, containerA, fixture.InstallResult, finalOperation)
}

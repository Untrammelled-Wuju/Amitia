package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newR3Runtime(t *testing.T) (*Runtime, *Container) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	container, err := NewContainerBuilder().
		WithDBPath(filepath.Join(root, "kernel.db")).
		WithExtensionRoot(filepath.Join(root, "extensions")).
		Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	runtime, err := NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)
	return runtime, container
}

func makeUninstallRecoveryOperation(t *testing.T, ctx context.Context, container *Container, extensionID, generationID, artifactID string) PackageOperationRecord {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	opID := fmt.Sprintf("op-uninstall-recovery-%s-%d", extensionID, time.Now().UnixNano())
	if err := container.PackageRepository.PutArtifact(ctx, PackageArtifact{
		ArtifactID:      artifactID,
		ExtensionID:     extensionID,
		Version:         "1.0.0",
		ArchiveHash:     "sha256:" + fmt.Sprintf("%x", sha256.Sum256([]byte(artifactID))),
		ManifestHash:    "sha256:manifest",
		ContentTreeHash: "sha256:content",
		RetentionState:  "active",
		CreatedAt:       now,
	}); err != nil {
		t.Fatal(err)
	}
	op := PackageOperationRecord{
		OperationID:       opID,
		TraceID:           "trace-" + opID,
		UserID:            "user-1",
		ScopeType:         "global",
		ExtensionID:       extensionID,
		OperationType:     "uninstall",
		Status:            "in_progress",
		CurrentStep:       "quarantine_metadata_restoring",
		ArtifactID:        artifactID,
		ConfirmationsJSON: `{"artifactPolicy":"retain"}`,
		StartedAt:         now,
		UpdatedAt:         now,
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	return op
}

func r3StepInputHash(operationID, stepName string) string {
	sum := sha256.Sum256([]byte(operationID + ":" + stepName))
	return hex.EncodeToString(sum[:])
}

func r3StepResultHash(resultJSON string) string {
	sum := sha256.Sum256([]byte(resultJSON))
	return hex.EncodeToString(sum[:])
}

func TestR3_CrashRecovery_LoadQuarantineMetadata_CompletesIdempotently(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.test/crash-load-metadata"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-1", "artifact-1")
	resultJSON := `{"loaded":true}`
	stepID := op.OperationID + ":" + StepUninstallRecoveryLoadQuarantineMetadata
	inputHash := r3StepInputHash(op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata)
	resultHash := r3StepResultHash(resultJSON)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: stepID, OperationID: op.OperationID, StepName: StepUninstallRecoveryLoadQuarantineMetadata,
		StepOrder: 1, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
		ResultHash: resultHash, InputHash: inputHash, StartedAt: now, CompletedAt: now,
	}, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	_ = runtime.recoverPackageOperation(ctx, op)
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
		t.Fatalf("load_quarantine_metadata must be completed exactly once, got %d", loadCount)
	}
}

func TestR3_CrashRecovery_VerifyQuarantineMetadata_ResumesAfterLoad(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.test/crash-verify-metadata"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-2", "artifact-2")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stepName := StepUninstallRecoveryLoadQuarantineMetadata
	resultJSON := `{"ok":true}`
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
	_ = runtime.recoverPackageOperation(ctx, op)
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
		t.Fatalf("load step must not be duplicated, got %d", loadCount)
	}
}

func TestR3_CrashRecovery_RestoreArtifactReference_PriorStepsNotDuplicated(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.test/crash-artifact-ref"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-3", "artifact-3")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	priorSteps := []string{
		StepUninstallRecoveryLoadQuarantineMetadata,
		StepUninstallRecoveryVerifyQuarantineMetadata,
		StepUninstallRecoveryRestoreGeneration,
		StepUninstallRecoveryRestoreCurrent,
		StepUninstallRecoveryRestoreInstallation,
		StepUninstallRecoveryRestoreVersionState,
		StepUninstallRecoveryRestoreArtifactPath,
	}
	for _, sn := range priorSteps {
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
	_ = runtime.recoverPackageOperation(ctx, op)
	steps, listErr := container.PackageRepository.ListOperationSteps(ctx, op.OperationID)
	if listErr != nil {
		t.Fatal(listErr)
	}
	for _, prior := range []string{
		StepUninstallRecoveryLoadQuarantineMetadata,
		StepUninstallRecoveryVerifyQuarantineMetadata,
	} {
		count := 0
		for _, s := range steps {
			if s.StepName == prior && s.Status == StatusCompleted {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("step %s must be completed exactly once, got %d", prior, count)
		}
	}
}

func TestR3_CrashRecovery_Finalize_RequiresFinalGateCompletion(t *testing.T) {
	ctx := context.Background()
	_, container := newR3Runtime(t)
	extID := "com.r3.test/crash-finalize"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-4", "artifact-4")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	gateStep := op.OperationID + ":" + StepUninstallRecoveryFinalGate
	resultJSON := `{"passed":true}`
	inputHash := r3StepInputHash(op.OperationID, StepUninstallRecoveryFinalGate)
	resultHash := r3StepResultHash(resultJSON)
	if err := container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: gateStep, OperationID: op.OperationID, StepName: StepUninstallRecoveryFinalGate,
		StepOrder: 10, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
		ResultHash: resultHash, InputHash: inputHash, StartedAt: now, CompletedAt: now,
	}, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	steps, err := container.PackageRepository.ListOperationSteps(ctx, op.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	hasFinalize := false
	for _, s := range steps {
		if s.StepName == StepUninstallRecoveryFinalize {
			hasFinalize = true
		}
	}
	if hasFinalize {
		t.Fatal("finalize step must not exist before final gate evidence is processed")
	}
}

func TestR3_CrashRecovery_StepResultHash_PreservedAcrossCrashes(t *testing.T) {
	ctx := context.Background()
	_, container := newR3Runtime(t)
	extID := "com.r3.test/crash-hash"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-5", "artifact-5")
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stepName := StepUninstallRecoveryLoadQuarantineMetadata
	stepID := op.OperationID + ":" + stepName
	resultJSON := fmt.Sprintf(`{"loaded":true,"operationId":%q}`, op.OperationID)
	resultHash := r3StepResultHash(resultJSON)
	inputHash := r3StepInputHash(op.OperationID, stepName)
	if err := container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: stepID, OperationID: op.OperationID, StepName: stepName,
		StepOrder: 1, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
		ResultHash: resultHash, InputHash: inputHash, StartedAt: now, CompletedAt: now,
	}, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	loaded, err := container.PackageRepository.getOperationStep(ctx, op.OperationID, stepName)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ResultHash != resultHash {
		t.Fatalf("result_hash must be preserved across crashes, got %q want %q", loaded.ResultHash, resultHash)
	}
}

func TestR3_CrashRecovery_VerifiedPathBinding_MatchesStore(t *testing.T) {
	_, container := newR3Runtime(t)
	extID := "com.r3.test/path-binding"
	operationID := fmt.Sprintf("op-path-binding-%d", time.Now().UnixNano())
	generationID := "gen-binding-1"
	paths, err := container.PackageGenerationStore.ExpectedUninstallRecoveryPaths(extID, generationID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if paths.OriginalCurrentPath == "" {
		t.Fatal("OriginalCurrentPath must not be empty")
	}
	if paths.OriginalGenerationPath == "" {
		t.Fatal("OriginalGenerationPath must not be empty")
	}
	if paths.CurrentQuarantinePath == "" {
		t.Fatal("CurrentQuarantinePath must not be empty")
	}
	if paths.GenerationQuarantinePath == "" {
		t.Fatal("GenerationQuarantinePath must not be empty")
	}
	if !filepath.IsAbs(paths.OriginalCurrentPath) {
		t.Fatalf("OriginalCurrentPath must be absolute, got %s", paths.OriginalCurrentPath)
	}
	if !filepath.IsAbs(paths.CurrentQuarantinePath) {
		t.Fatalf("CurrentQuarantinePath must be absolute, got %s", paths.CurrentQuarantinePath)
	}
}

func TestR3_CrashRecovery_QuarantinePathFormat_IncludesSegments(t *testing.T) {
	_, container := newR3Runtime(t)
	extID := "com.r3.test/path-segments"
	operationID := "op-seg-001"
	generationID := "gen-seg-001"
	paths, err := container.PackageGenerationStore.ExpectedUninstallRecoveryPaths(extID, generationID, operationID)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(paths.CurrentQuarantinePath) {
		t.Fatalf("current quarantine must be absolute: %s", paths.CurrentQuarantinePath)
	}
	if !filepath.IsAbs(paths.GenerationQuarantinePath) {
		t.Fatalf("generation quarantine must be absolute: %s", paths.GenerationQuarantinePath)
	}
}

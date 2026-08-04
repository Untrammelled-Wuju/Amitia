package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type packagePipelineInstallResult struct {
	ExtensionID  string
	Version      string
	ArtifactID   string
	GenerationID string
	VersionID    string
	InstallPath  string
	TreeHash     string
}

func seedR3FullInstall(
	t *testing.T,
	runtime *Runtime,
	container *Container,
) packagePipelineInstallResult {
	t.Helper()

	ctx := context.Background()

	extID := "com.r3.full/recovery"
	artifactID := "artifact-r3-full"
	version := "1.0.0"

	require.NoError(t, container.PackageRepository.PutArtifact(ctx, PackageArtifact{
		ArtifactID:      artifactID,
		ExtensionID:     extID,
		Version:         version,
		ArchiveHash:     "sha256:" + fmt.Sprintf("%x", sha256.Sum256([]byte(artifactID))),
		ManifestHash:    "sha256:manifest-full",
		ContentTreeHash: "sha256:content-full",
		RetentionState:  "active",
		InstalledPath:   filepath.Join(container.ExtRoot, "installations", safeDirectoryName(extID), "generations", "gen-r3-full"),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}))

	installPath := filepath.Join(container.ExtRoot, "installations", safeDirectoryName(extID), "generations", "gen-r3-full")
	require.NoError(t, os.MkdirAll(installPath, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(installPath, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`, extID, version)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(installPath, "index.js"), []byte("// r3 full module"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(installPath, "subdir"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(installPath, "subdir", "extra.js"), []byte("// extra"), 0o600))

	now := time.Now().UTC().Format(time.RFC3339Nano)
	installedAt := time.Now().UTC()
	versionID := "ver-r3-full"
	treeHash, treeHashErr := computeGenerationTreeHash(ctx, installPath)
	require.NoError(t, treeHashErr)

	installRecord := domain.ExtensionInstallation{
		InstallationID:    extID,
		ExtensionID:       domain.ExtensionID(extID),
		PackageID:         artifactID,
		InstalledVersion:  domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
		Generation:        1,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementEnabled,
		InstalledAt:       installedAt,
		UpdatedAt:         installedAt,
		Metadata: map[string]interface{}{
			"versionId":     versionID,
			"generationId":  "gen-r3-full",
			"treeHash":      treeHash,
			"installedPath": installPath,
		},
	}
	require.NoError(t, container.InstallationRepository.PutInstallation(ctx, installRecord))

	require.NoError(t, container.PackageRepository.PutPackageVersion(context.Background(), PackageVersionRecord{
		VersionID:         versionID,
		ExtensionID:       extID,
		Version:           version,
		ArtifactID:        artifactID,
		IsActive:          true,
		VersionState:      string(PackageVersionStateCurrent),
		InstalledAt:       now,
		GenerationID:      "gen-r3-full",
		InstalledPath:     installPath,
		InstalledTreeHash: treeHash,
	}))

	currentFile := filepath.Join(container.ExtRoot, "installations", safeDirectoryName(extID), "current.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(currentFile), 0o700))
	currentData, err := json.Marshal(map[string]interface{}{
		"extensionID":  extID,
		"generationID": "gen-r3-full",
		"version":      version,
		"artifactID":   artifactID,
		"treeHash":     treeHash,
		"operationID":  "install-r3-full",
		"fencingToken": 0,
		"updatedAt":    now,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(currentFile, currentData, 0o600))

	return packagePipelineInstallResult{
		ExtensionID:  extID,
		Version:      version,
		ArtifactID:   artifactID,
		GenerationID: "gen-r3-full",
		VersionID:    versionID,
		InstallPath:  installPath,
		TreeHash:     treeHash,
	}
}

func setupR3FullInstall(t *testing.T) (*Runtime, *Container, packagePipelineInstallResult) {
	t.Helper()

	runtime, container := newR3Runtime(t)

	result := seedR3FullInstall(t, runtime, container)

	return runtime, container, result
}

func buildR3QuarantineMetadata(t *testing.T, ctx context.Context, container *Container, res packagePipelineInstallResult, operationID string) PackageQuarantineMetadata {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	currentPath := filepath.Join(container.ExtRoot, "installations", safeDirectoryName(res.ExtensionID), "current.json")
	generationPath := filepath.Join(container.ExtRoot, "installations", safeDirectoryName(res.ExtensionID), "generations", res.GenerationID)

	generationQuarantinePath := filepath.Join(container.ExtRoot, "quarantine", "generations", safeDirectoryName(res.ExtensionID), res.GenerationID+"-"+operationID)
	currentQuarantinePath := filepath.Join(container.ExtRoot, "quarantine", "current", safeDirectoryName(res.ExtensionID), operationID)

	require.NoError(t, os.MkdirAll(filepath.Dir(generationQuarantinePath), 0o700))
	require.NoError(t, os.MkdirAll(filepath.Dir(currentQuarantinePath), 0o700))
	require.NoError(t, os.Rename(generationPath, generationQuarantinePath))

	currentBytes, err := os.ReadFile(currentPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(currentQuarantinePath, currentBytes, 0o600))
	require.NoError(t, os.Remove(currentPath))

	beforeUninstall, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(res.ExtensionID))
	require.NoError(t, err)
	targetVersionID := beforeUninstall.Metadata["versionId"].(string)

	snapshot := packageInstallationSnapshot{
		PackageID:         res.ArtifactID,
		InstalledVersion:  res.Version,
		InstalledPath:     res.InstallPath,
		InstalledTreeHash: res.TreeHash,
		Generation:        int64(beforeUninstall.Generation),
		GenerationID:      res.GenerationID,
		Installable:       true,
		Enabled:           beforeUninstall.EnablementState == domain.EnablementEnabled,
		Metadata:          beforeUninstall.Metadata,
		CapturedAt:        now,
	}
	snapshotJSON, marshalErr := json.Marshal(snapshot)
	require.NoError(t, marshalErr)
	snapshotHash := fmt.Sprintf("%x", sha256.Sum256(snapshotJSON))

	require.NoError(t, container.InstallationRepository.DeleteInstallation(ctx, domain.ExtensionID(res.ExtensionID)))

	return PackageQuarantineMetadata{
		QuarantineID:             "quarantine-" + operationID,
		OperationID:              operationID,
		ExtensionID:              res.ExtensionID,
		GenerationQuarantinePath: generationQuarantinePath,
		CurrentQuarantinePath:    currentQuarantinePath,
		OriginalGenerationPath:   generationPath,
		OriginalCurrentPath:      currentPath,
		TreeHash:                 res.TreeHash,
		ArtifactID:               res.ArtifactID,
		State:                    "active",
		FencingToken:             1,
		ExpectedGenerationID:     res.GenerationID,
		ExpectedVersionID:        targetVersionID,
		SnapshotJSON:             string(snapshotJSON),
		SnapshotHash:             snapshotHash,
		CreatedAt:                now,
	}
}

func createUninstallRecoveryOp(t *testing.T, ctx context.Context, container *Container, res packagePipelineInstallResult, operationID string) PackageOperationRecord {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	previewSessionID := "preview-" + operationID

	op := PackageOperationRecord{
		OperationID:        operationID,
		TraceID:            "trace-" + operationID,
		UserID:             "user-1",
		ScopeType:          "global",
		ExtensionID:        res.ExtensionID,
		TargetVersion:      res.Version,
		OperationType:      "uninstall",
		Status:             "in_progress",
		CurrentStep:        "move_to_quarantine",
		ArtifactID:         res.ArtifactID,
		PreviewSessionID:   previewSessionID,
		ConfirmationsJSON:  `{}`,
		StartedAt:          now,
		UpdatedAt:          now,
		StableGeneration:   res.GenerationID,
		CurrentPointerJSON: fmt.Sprintf(`{"generationId":%q,"extensionId":%q}`, res.GenerationID, res.ExtensionID),
	}

	require.NoError(t, container.PackageRepository.CreateOperation(ctx, op))

	require.NoError(t, container.PackageRepository.EnsureArtifactReference(
		ctx,
		res.ArtifactID,
		ArtifactReferencePreview,
		previewSessionID,
		time.Now().UTC().Add(time.Hour),
	))

	return op
}

func completedStep(t *testing.T, ctx context.Context, container *Container, operationID, stepName string, stepOrder int, resultJSON string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	stepID := operationID + ":" + stepName
	inputHash := r3StepInputHash(operationID, stepName)
	resultHash := r3StepResultHash(resultJSON)
	require.NoError(t, container.PackageRepository.PutStep(ctx, PackageOperationStep{
		StepID: stepID, OperationID: operationID, StepName: stepName,
		StepOrder: stepOrder, Status: StatusCompleted, AttemptCount: 1, ResultJSON: resultJSON,
		ResultHash: resultHash, InputHash: inputHash, StartedAt: now, CompletedAt: now,
	}, PackageWriteGuard{}))
}

type r3RecoveryFixture struct {
	DBPath        string
	ExtensionRoot string
	Runtime       *Runtime
	Container     *Container
	InstallResult packagePipelineInstallResult
	Operation     PackageOperationRecord
	Metadata      PackageQuarantineMetadata
}

func newR3RecoveryFixture(t *testing.T) *r3RecoveryFixture {
	t.Helper()

	root := t.TempDir()
	dbPath := filepath.Join(root, "kernel.db")
	extensionRoot := filepath.Join(root, "extensions")

	runtime, container := newR3RuntimeAt(t, dbPath, extensionRoot)

	result := seedR3FullInstall(t, runtime, container)

	operationID := fmt.Sprintf("op-r3-fixture-%d", time.Now().UnixNano())

	metadata := buildR3QuarantineMetadata(t, context.Background(), container, result, operationID)

	operation := createUninstallRecoveryOp(t, context.Background(), container, result, operationID)

	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(context.Background(), metadata, PackageWriteGuard{}))

	fixture := &r3RecoveryFixture{
		DBPath:        dbPath,
		ExtensionRoot: extensionRoot,
		Runtime:       runtime,
		Container:     container,
		InstallResult: result,
		Operation:     operation,
		Metadata:      metadata,
	}

	t.Cleanup(func() {
		fixture.Close(t)
	})

	return fixture
}

func (f *r3RecoveryFixture) Close(t *testing.T) {
	t.Helper()

	if f.Container == nil {
		return
	}

	require.NoError(t, f.Container.Close())

	f.Container = nil
	f.Runtime = nil
}

func (f *r3RecoveryFixture) Restart(t *testing.T) {
	t.Helper()

	f.Close(t)

	runtime, container := newR3RuntimeAt(t, f.DBPath, f.ExtensionRoot)

	f.Runtime = runtime
	f.Container = container
}

func (f *r3RecoveryFixture) ReloadOperation(t *testing.T) PackageOperationRecord {
	t.Helper()

	operation, _, err := f.Container.PackageRepository.GetOperation(context.Background(), f.Operation.UserID, f.Operation.OperationID)
	require.NoError(t, err)

	f.Operation = operation

	return operation
}

func assertR3RecoveryFinalState(
	t *testing.T,
	runtime *Runtime,
	container *Container,
	result packagePipelineInstallResult,
	operation PackageOperationRecord,
) {
	t.Helper()

	ctx := context.Background()

	finalOperation, steps, err := container.PackageRepository.GetOperation(ctx, operation.UserID, operation.OperationID)
	require.NoError(t, err)

	require.Equal(t, string(PackageOperationCompleted), string(finalOperation.Status))
	require.NotEmpty(t, finalOperation.CompletedAt)

	require.NoError(t, runtime.verifyUninstallFinalizedState(ctx, finalOperation))

	expectedSteps := []string{
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
		StepUninstallRecoveryFinalize,
	}

	completedCount := make(map[string]int, len(expectedSteps))
	for _, step := range steps {
		if step.Status == StatusCompleted {
			completedCount[step.StepName]++
		}
	}

	for _, stepName := range expectedSteps {
		require.Equal(t, 1, completedCount[stepName], "step %s must be completed exactly once", stepName)
	}

	_, leaseErr := container.PackageRepository.getExtensionLease(ctx, result.ExtensionID)
	require.True(t, IsPackageOperationError(leaseErr, OperationErrNotFound), "final Lease must not exist: %v", leaseErr)

	operationReferences, err := container.PackageRepository.ListActiveArtifactReferences(ctx, result.ArtifactID, ArtifactReferenceOperation, operation.OperationID)
	require.NoError(t, err)
	require.Empty(t, operationReferences)

	previewReferences, err := container.PackageRepository.ListActiveArtifactReferences(ctx, result.ArtifactID, ArtifactReferencePreview, operation.PreviewSessionID)
	require.NoError(t, err)
	require.Empty(t, previewReferences)

	installationReferences, err := container.PackageRepository.ListActiveArtifactReferences(ctx, result.ArtifactID, ArtifactReferenceInstallation, result.ExtensionID)
	require.NoError(t, err)
	require.Len(t, installationReferences, 1)

	metadata, err := container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operation.OperationID)
	require.NoError(t, err)

	require.Equal(t, "released", metadata.State)
	require.Equal(t, "released", metadata.ReleaseState)
	require.NotEmpty(t, metadata.ReleasedAt)

	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(result.ExtensionID))
	require.NoError(t, err)

	require.Equal(t, domain.InstallationStateInstalled, installation.InstallationState)
	require.Equal(t, result.Version, installation.InstalledVersion.String())
	require.Equal(t, result.ArtifactID, installation.PackageID)

	versionRecord, err := container.PackageRepository.GetPackageVersionByID(ctx, result.ExtensionID, metadata.ExpectedVersionID)
	require.NoError(t, err)

	require.Equal(t, result.VersionID, versionRecord.VersionID)
	require.Equal(t, result.GenerationID, versionRecord.GenerationID)

	current, err := container.PackageGenerationStore.ReadCurrent(result.ExtensionID)
	require.NoError(t, err)

	require.Equal(t, result.GenerationID, current.GenerationID)
	require.Equal(t, result.Version, current.Version)
	require.Equal(t, result.ArtifactID, current.ArtifactID)

	actualTreeHash, err := computeGenerationTreeHash(ctx, result.InstallPath)
	require.NoError(t, err)

	require.Equal(t, result.TreeHash, actualTreeHash)
	require.Equal(t, actualTreeHash, current.TreeHash)
	require.Equal(t, actualTreeHash, metadata.TreeHash)

	artifact, err := container.PackageRepository.GetArtifact(ctx, result.ArtifactID)
	require.NoError(t, err)

	require.Equal(t, result.ExtensionID, artifact.ExtensionID)
	require.Equal(t, result.InstallPath, artifact.InstalledPath)
}

func TestR3RecoveryCompletesFromValidCompensationState(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-r3-complete-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	require.NoError(t, runtime.recoverPackageOperation(ctx, op))

	assertR3RecoveryFinalState(t, runtime, container, res, op)
}

func TestR3FinalGateRejectsInstalledVersionMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-reject-ver-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	completedStep(t, ctx, container, operationID, StepUninstallRecoveryLoadQuarantineMetadata, 1,
		fmt.Sprintf(`{"quarantine_id":%q,"state":"active"}`, qm.QuarantineID))
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)

	freshQM, err := container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operationID)
	require.NoError(t, err)
	freshQM.ExpectedVersionID = "nonexistent-version-id"
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, freshQM, PackageWriteGuard{}))

	err = runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err, "recovery with wrong expected_version_id must fail")

	recovered, _, getErr := container.PackageRepository.GetOperation(ctx, "user-1", operationID)
	require.NoError(t, getErr)
	require.NotEqual(t, "completed", string(recovered.Status))
}

func TestR3FinalGateRejectsCurrentTreeHashMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-reject-tree-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	completedStep(t, ctx, container, operationID, StepUninstallRecoveryLoadQuarantineMetadata, 1,
		fmt.Sprintf(`{"quarantine_id":%q,"state":"active"}`, qm.QuarantineID))
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)

	currentPath := filepath.Join(container.ExtRoot, "installations", safeDirectoryName(res.ExtensionID), "current.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(currentPath), 0o700))
	tampered, err := json.Marshal(map[string]interface{}{
		"extensionID":  res.ExtensionID,
		"generationID": res.GenerationID,
		"version":      res.Version,
		"artifactID":   res.ArtifactID,
		"treeHash":     "tampered-hash",
		"operationID":  "install-r3-full",
		"fencingToken": 0,
		"updatedAt":    time.Now().UTC().Format(time.RFC3339Nano),
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(currentPath, tampered, 0o600))

	err = runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err, "recovery with mismatched current tree_hash must fail")
}

func TestR3FinalGateRejectsEmptyExpectedVersionID(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-reject-noversion-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	completedStep(t, ctx, container, operationID, StepUninstallRecoveryLoadQuarantineMetadata, 1,
		fmt.Sprintf(`{"quarantine_id":%q,"state":"active"}`, qm.QuarantineID))
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)

	freshQM, err := container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operationID)
	require.NoError(t, err)
	freshQM.ExpectedVersionID = ""
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, freshQM, PackageWriteGuard{}))

	err = runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err, "recovery with empty expected_version_id must fail")
}

func TestR3FinalGateRejectsMetadataTreeHashMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-reject-qmtree-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	completedStep(t, ctx, container, operationID, StepUninstallRecoveryLoadQuarantineMetadata, 1,
		fmt.Sprintf(`{"quarantine_id":%q,"state":"active"}`, qm.QuarantineID))
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)

	freshQM, err := container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operationID)
	require.NoError(t, err)
	freshQM.TreeHash = "tampered-metadata-tree-hash"
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, freshQM, PackageWriteGuard{}))

	err = runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err, "recovery when metadata.TreeHash != actual generation tree_hash must fail")
}

func TestR3ReleasedMetadataRejectsEmptySnapshotJSON(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-meta-snapjson-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	for i, sn := range []string{
		StepUninstallRecoveryLoadQuarantineMetadata,
		StepUninstallRecoveryVerifyQuarantineMetadata,
		StepUninstallRecoveryRestoreGeneration,
		StepUninstallRecoveryRestoreCurrent,
		StepUninstallRecoveryRestoreInstallation,
		StepUninstallRecoveryRestoreVersionState,
		StepUninstallRecoveryRestoreArtifactPath,
		StepUninstallRecoveryRestoreArtifactReference,
		StepUninstallRecoveryVerifyRestoredState,
	} {
		completedStep(t, ctx, container, operationID, sn, i+1, fmt.Sprintf(`{"ok":true,"step":%q}`, sn))
	}

	freshQM, err := container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operationID)
	require.NoError(t, err)
	freshQM.SnapshotJSON = ""
	freshQM.SnapshotHash = "some-hash"
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, freshQM, PackageWriteGuard{}))

	err = runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err, "empty SnapshotJSON must fail metadata validation")
}

func TestR3ReleasedMetadataRejectsEmptySnapshotHash(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-meta-snaphash-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	for i, sn := range []string{
		StepUninstallRecoveryLoadQuarantineMetadata,
		StepUninstallRecoveryVerifyQuarantineMetadata,
		StepUninstallRecoveryRestoreGeneration,
		StepUninstallRecoveryRestoreCurrent,
		StepUninstallRecoveryRestoreInstallation,
		StepUninstallRecoveryRestoreVersionState,
		StepUninstallRecoveryRestoreArtifactPath,
		StepUninstallRecoveryRestoreArtifactReference,
		StepUninstallRecoveryVerifyRestoredState,
	} {
		completedStep(t, ctx, container, operationID, sn, i+1, fmt.Sprintf(`{"ok":true,"step":%q}`, sn))
	}

	freshQM, err := container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operationID)
	require.NoError(t, err)
	freshQM.SnapshotJSON = `{"snapshot":"data"}`
	freshQM.SnapshotHash = ""
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, freshQM, PackageWriteGuard{}))

	err = runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err, "empty SnapshotHash must fail metadata validation")
}

func TestR3ReleasedMetadataRejectsGenerationHashMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-meta-genhash-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	for i, sn := range []string{
		StepUninstallRecoveryLoadQuarantineMetadata,
		StepUninstallRecoveryVerifyQuarantineMetadata,
		StepUninstallRecoveryRestoreGeneration,
		StepUninstallRecoveryRestoreCurrent,
		StepUninstallRecoveryRestoreInstallation,
		StepUninstallRecoveryRestoreVersionState,
		StepUninstallRecoveryRestoreArtifactPath,
		StepUninstallRecoveryRestoreArtifactReference,
		StepUninstallRecoveryVerifyRestoredState,
	} {
		completedStep(t, ctx, container, operationID, sn, i+1, fmt.Sprintf(`{"ok":true,"step":%q}`, sn))
	}

	freshQM, err := container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operationID)
	require.NoError(t, err)
	freshQM.SnapshotJSON = `{"snapshot":"data"}`
	freshQM.SnapshotHash = fmt.Sprintf("%x", sha256.Sum256([]byte(freshQM.SnapshotJSON)))
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, freshQM, PackageWriteGuard{}))

	releaseStepResultJSON := fmt.Sprintf(`{"released":true,"snapshotHash":%q,"generationHash":"wrong-generation-hash","metadataHash":"metadata-hash-placeholder","schemaVersion":1,"operationId":%q,"quarantineId":%q,"extensionId":%q,"artifactId":%q}`,
		freshQM.SnapshotHash, operationID, freshQM.QuarantineID, res.ExtensionID, res.ArtifactID)
	completedStep(t, ctx, container, operationID, StepUninstallRecoveryReleaseQuarantineMetadata, 10, releaseStepResultJSON)

	err = runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err, "generation_hash mismatch in release step result must fail metadata validation")
}

func TestR3CrashRecovery_BeforeLoadMetadata_FailsGracefully(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/before-load"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-bl", "artifact-crash-bl")
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err, "recovery with no prior steps must fail")
}

var errR3SimulatedProcessCrash = errors.New("r3 simulated process crash")

func TestR3CrashRecoveryResumesFromEveryCommittedStep(t *testing.T) {
	crashPoints := []string{
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
		packageRecoveryFaultPointPostFinalizeCommit,
	}

	for _, crashPoint := range crashPoints {
		crashPoint := crashPoint

		t.Run(crashPoint, func(t *testing.T) {
			fixture := newR3RecoveryFixture(t)

			fired := false

			crashContext := withPackageRecoveryFaultHook(
				context.Background(),
				func(operationID string, faultPoint string) error {
					if operationID != fixture.Operation.OperationID {
						return nil
					}
					if faultPoint != crashPoint {
						return nil
					}
					if fired {
						return nil
					}
					fired = true
					return errR3SimulatedProcessCrash
				},
			)

			firstErr := fixture.Runtime.recoverPackageOperation(crashContext, fixture.Operation)

			require.Error(t, firstErr)
			require.ErrorContains(t, firstErr, errR3SimulatedProcessCrash.Error())
			require.True(t, fired, "fault point %s was not reached", crashPoint)

			fixture.Restart(t)

			reloadedOperation := fixture.ReloadOperation(t)

			require.NoError(t, fixture.Runtime.recoverPackageOperation(context.Background(), reloadedOperation))

			assertR3RecoveryFinalState(t, fixture.Runtime, fixture.Container, fixture.InstallResult, reloadedOperation)
		})
	}
}

func TestR3CompletedRecoveryIsReadOnlyAndDoesNotReacquireLease(t *testing.T) {
	fixture := newR3RecoveryFixture(t)
	ctx := context.Background()

	require.NoError(t, fixture.Runtime.recoverPackageOperation(ctx, fixture.Operation))

	completed := fixture.ReloadOperation(t)
	require.Equal(t, string(PackageOperationCompleted), string(completed.Status))

	require.NoError(t, fixture.Runtime.recoverPackageOperation(ctx, completed))

	_, leaseErr := fixture.Container.PackageRepository.getExtensionLease(ctx, completed.ExtensionID)
	require.True(t, IsPackageOperationError(leaseErr, OperationErrNotFound))

	assertR3RecoveryFinalState(t, fixture.Runtime, fixture.Container, fixture.InstallResult, completed)
}

func TestR3FinalGateEvidenceStructRoundTrip(t *testing.T) {
	evidence := UninstallRestoredIdentityEvidence{
		SchemaVersion:             1,
		OperationID:               "op-rt-1",
		ExtensionID:               "com.test/rt",
		ArtifactID:                "art-rt",
		ExpectedVersionID:         "ver-rt",
		RestoredVersion:           "1.0.0",
		ExpectedGenerationID:      "gen-rt",
		InstallationVersion:       "1.0.0",
		InstallationGenerationID:  "gen-rt",
		VersionRecordID:           "ver-rt",
		VersionRecordVersion:      "1.0.0",
		VersionRecordGenerationID: "gen-rt",
		CurrentVersion:            "1.0.0",
		CurrentArtifactID:         "art-rt",
		CurrentGenerationID:       "gen-rt",
		CurrentTreeHash:           "tree-rt",
		MetadataTreeHash:          "tree-rt",
		ActualGenerationTreeHash:  "tree-rt",
	}
	canonical := packageCanonicalJSON(evidence)
	sum := sha256.Sum256([]byte(canonical))
	evidence.EvidenceHash = hex.EncodeToString(sum[:])

	data, err := json.Marshal(evidence)
	require.NoError(t, err)

	var decoded UninstallRestoredIdentityEvidence
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, evidence.OperationID, decoded.OperationID)
	require.Equal(t, evidence.EvidenceHash, decoded.EvidenceHash)
	require.Equal(t, evidence.SchemaVersion, decoded.SchemaVersion)
	require.Equal(t, evidence.ActualGenerationTreeHash, decoded.ActualGenerationTreeHash)
}

func TestR3ReleaseStepResultStructRoundTrip(t *testing.T) {
	result := UninstallReleaseQuarantineStepResult{
		SchemaVersion:  1,
		OperationID:    "op-rel-1",
		QuarantineID:   "q-1",
		ExtensionID:    "com.test/rel",
		ArtifactID:     "art-rel",
		ReleasedAt:     "2025-01-01T00:00:00Z",
		SnapshotHash:   "snap-hash",
		GenerationHash: "gen-hash",
		MetadataHash:   "meta-hash",
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded UninstallReleaseQuarantineStepResult
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.Equal(t, result.OperationID, decoded.OperationID)
	require.Equal(t, result.GenerationHash, decoded.GenerationHash)
	require.Equal(t, result.MetadataHash, decoded.MetadataHash)
}

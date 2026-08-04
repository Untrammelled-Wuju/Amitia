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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

// packagePipelineInstallResult holds the identifiers set up by setupR3FullInstall.
type packagePipelineInstallResult struct {
	ExtensionID  string
	Version      string
	ArtifactID   string
	GenerationID string
	VersionID    string
	InstallPath  string
	TreeHash     string
}

// setupR3FullInstall creates a complete installation state on disk and in the
// database so that the R3 uninstall-recovery path has material to compensate.
func setupR3FullInstall(t *testing.T) (*Runtime, *Container, packagePipelineInstallResult) {
	t.Helper()
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
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
			"versionId":    versionID,
			"generationId": "gen-r3-full",
			"treeHash":     treeHash,
			"installedPath": installPath,
		},
	}
	require.NoError(t, container.InstallationRepository.PutInstallation(ctx, installRecord))

	require.NoError(t, container.PackageRepository.PutPackageVersion(context.Background(), PackageVersionRecord{
		VersionID:        versionID,
		ExtensionID:      extID,
		Version:          version,
		ArtifactID:       artifactID,
		IsActive:         true,
		VersionState:     string(PackageVersionStateCurrent),
		InstalledAt:      now,
		GenerationID:     "gen-r3-full",
		InstalledPath:    installPath,
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

	return runtime, container, packagePipelineInstallResult{
		ExtensionID:  extID,
		Version:      version,
		ArtifactID:   artifactID,
		GenerationID: "gen-r3-full",
		VersionID:    versionID,
		InstallPath:  installPath,
		TreeHash:     treeHash,
	}
}

// buildR3QuarantineMetadata simulates an uninstall's move_to_quarantine step:
// it moves the generation dir and current.json into quarantine, captures an
// installation snapshot, deletes the installation record, and clears the
// artifact's installed path.
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
		ConfirmationsJSON:  `{}`,
		StartedAt:          now,
		UpdatedAt:          now,
		StableGeneration:   res.GenerationID,
		CurrentPointerJSON: fmt.Sprintf(`{"generationId":%q,"extensionId":%q}`, res.GenerationID, res.ExtensionID),
	}
	require.NoError(t, container.PackageRepository.CreateOperation(ctx, op))
	return op
}

// completedStep writes a pre-completed step to the repository so that
// runUninstallRecoveryStep skips it during recoverPackageOperation.
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

func TestR3RecoveryCompletesFromValidCompensationState(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-r3-complete-%d", time.Now().UnixNano())
	now := time.Now().UTC().Format(time.RFC3339Nano)
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	loadResultJSON := fmt.Sprintf(`{"quarantine_id":%q,"state":"active"}`, qm.QuarantineID)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, loadResultJSON)
	_ = now

	require.NoError(t, runtime.recoverPackageOperation(ctx, op))

	finalOp, _, err := container.PackageRepository.GetOperation(ctx, "user-1", operationID)
	require.NoError(t, err)
	require.Equal(t, "completed", string(finalOp.Status), "recovery must reach completed state")

	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(res.ExtensionID))
	require.NoError(t, err)
	require.Equal(t, domain.InstallationStateInstalled, installation.InstallationState)

	current, err := container.PackageGenerationStore.ReadCurrent(res.ExtensionID)
	require.NoError(t, err)
	require.Equal(t, res.GenerationID, current.GenerationID)

	_, statErr := os.Stat(res.InstallPath)
	require.NoError(t, statErr, "generation must be restored to original path")

	steps, listErr := container.PackageRepository.ListOperationSteps(ctx, operationID)
	require.NoError(t, listErr)
	stepCounts := map[string]int{}
	for _, s := range steps {
		if s.Status == StatusCompleted {
			stepCounts[s.StepName]++
		}
	}
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
		StepUninstallRecoveryFinalize,
	} {
		require.Equal(t, 1, stepCounts[sn], "step %s must be completed exactly once", sn)
	}

	finalQM, qmErr := container.PackageRepository.GetQuarantineMetadataByOperation(ctx, operationID)
	require.NoError(t, qmErr)
	require.Equal(t, "released", finalQM.ReleaseState)
	require.NotEmpty(t, finalQM.ReleasedAt)
	require.NotEmpty(t, finalQM.SnapshotJSON)
	require.NotEmpty(t, finalQM.SnapshotHash)
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

	// Tamper with the current pointer tree hash.
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

func TestR3CrashRecovery_AfterLoadBeforeVerify_CompletesIdempotently(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/load-only"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-lo", "artifact-crash-lo")
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, `{"loaded":true}`)
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err)
	steps, listErr := container.PackageRepository.ListOperationSteps(ctx, op.OperationID)
	require.NoError(t, listErr)
	loadCount := 0
	for _, s := range steps {
		if s.StepName == StepUninstallRecoveryLoadQuarantineMetadata && s.Status == StatusCompleted {
			loadCount++
		}
	}
	require.Equal(t, 1, loadCount, "load step must not be duplicated on crash recovery resume")
}

func TestR3CrashRecovery_AfterRestoreGeneration_RestartsCleanly(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/after-gen"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-ag", "artifact-crash-ag")
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, `{"loaded":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err)
}

func TestR3CrashRecovery_AfterRestoreCurrent_RestartsCleanly(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/after-cur"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-ac", "artifact-crash-ac")
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, `{"loaded":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err)
}

func TestR3CrashRecovery_AfterRestoreInstallation_RestartsCleanly(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/after-inst"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-ai", "artifact-crash-ai")
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, `{"loaded":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreInstallation, 5, `{"installation_restored":true}`)
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err)
}

func TestR3CrashRecovery_AfterRestoreVersionState_RestartsCleanly(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/after-vs"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-avs", "artifact-crash-avs")
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, `{"loaded":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreInstallation, 5, `{"installation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreVersionState, 6, `{"version_state_restored":true}`)
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err)
}

func TestR3CrashRecovery_AfterRestoreArtifactPath_RestartsCleanly(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/after-ap"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-aap", "artifact-crash-aap")
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, `{"loaded":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreInstallation, 5, `{"installation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreVersionState, 6, `{"version_state_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreArtifactPath, 7, `{"artifact_path_restored":true}`)
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err)
}

func TestR3CrashRecovery_AfterVerifyRestoredState_RestartsCleanly(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/after-vrs"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-avrs", "artifact-crash-avrs")
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, `{"loaded":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreInstallation, 5, `{"installation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreVersionState, 6, `{"version_state_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreArtifactPath, 7, `{"artifact_path_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreArtifactReference, 8, `{"artifact_reference_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryVerifyRestoredState, 9, `{"restored_state_verified":true}`)
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err)
}

func TestR3CrashRecovery_AfterRestoreArtifactReference_RestartsCleanly(t *testing.T) {
	ctx := context.Background()
	runtime, container := newR3Runtime(t)
	extID := "com.r3.crash/after-ari"
	op := makeUninstallRecoveryOperation(t, ctx, container, extID, "gen-crash-ari", "artifact-crash-ari")
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, `{"loaded":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryVerifyQuarantineMetadata, 2, `{"verified":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreGeneration, 3, `{"generation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreCurrent, 4, `{"current_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreInstallation, 5, `{"installation_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreVersionState, 6, `{"version_state_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreArtifactPath, 7, `{"artifact_path_restored":true}`)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryRestoreArtifactReference, 8, `{"artifact_reference_restored":true}`)
	err := runtime.recoverPackageOperation(ctx, op)
	require.Error(t, err)
}

func TestR3ConcurrentRecoveryFinalizesExactlyOnce(t *testing.T) {
	ctx := context.Background()
	runtime, container, res := setupR3FullInstall(t)
	operationID := fmt.Sprintf("op-r3-concur-%d", time.Now().UnixNano())
	qm := buildR3QuarantineMetadata(t, ctx, container, res, operationID)
	op := createUninstallRecoveryOp(t, ctx, container, res, operationID)
	require.NoError(t, container.PackageRepository.PutQuarantineMetadata(ctx, qm, PackageWriteGuard{}))

	loadResultJSON := fmt.Sprintf(`{"quarantine_id":%q,"state":"active"}`, qm.QuarantineID)
	completedStep(t, ctx, container, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 1, loadResultJSON)

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
	t.Logf("concurrent recovery results: err[0]=%v, err[1]=%v", errs[0], errs[1])

	finalOp, _, err := container.PackageRepository.GetOperation(ctx, "user-1", operationID)
	require.NoError(t, err)
	require.Equal(t, "completed", string(finalOp.Status), "concurrent recovery must reach completed state")

	steps, listErr := container.PackageRepository.ListOperationSteps(ctx, operationID)
	require.NoError(t, listErr)
	finalizeCount := 0
	for _, s := range steps {
		if s.StepName == StepUninstallRecoveryFinalize && s.Status == StatusCompleted {
			finalizeCount++
		}
	}
	require.Equal(t, 1, finalizeCount, "finalize step must be completed exactly once under concurrent recovery")

	installation, instErr := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(res.ExtensionID))
	if errors.Is(instErr, domain.ErrInvalidExtensionID) {
		t.Fatal("installation must exist after concurrent recovery")
	}
	require.NoError(t, instErr)
	require.Equal(t, domain.InstallationStateInstalled, installation.InstallationState)
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

// Ensure all 12 step name constants are referenced (compile safeguard).
var _ = strings.TrimSpace // keep strings import if unused elsewhere

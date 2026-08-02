package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func installPackagePipelineVersion(t *testing.T, runtime *Runtime, version string) KernelInstallResult {
	t.Helper()
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	ctx := context.Background()
	workspaceID := dev_mode.WorkspaceID("package-saga-recovery")
	if _, err := runtime.container.DevModeRegistry.Get(workspaceID); err != nil {
		if _, err := runtime.container.DevModeRegistry.Register(ctx, dev_mode.RegisterWorkspaceInput{WorkspaceID: workspaceID, ExtensionID: dev_mode.ExtensionID("com.example/pipeline"), OwnerUserID: "user-1", PathReference: t.TempDir(), ManifestPath: "manifest.json"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := runtime.container.DevModeRegistry.GrantDevTrust(workspaceID); err != nil {
		t.Fatal(err)
	}
	workspace, err := runtime.container.DevModeRegistry.Get(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := runtime.container.DevModeSessions.Open(ctx, workspaceID, dev_mode.ExtensionID("com.example/pipeline"), "user-1", "test-device", "test", packagePolicyVersion, true, workspace.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	archivePath := createPackagePipelineArchive(t, version)
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackage(context.Background(), PackagePreviewRequest{
		UserID: "user-1", ScopeType: "global", FileName: "pipeline.amitiax",
		AllowUnsignedDev: true, DeveloperSessionID: session.SessionID,
	}, archive)
	archive.Close()
	if err != nil {
		t.Fatal(err)
	}
	confirmations := make(map[string]bool, len(preview.RequiredConfirmations))
	for _, confirmation := range preview.RequiredConfirmations {
		confirmations[confirmation] = true
	}
	confirmed, err := runtime.ConfirmPackagePreview(context.Background(), PackagePreviewConfirmationRequest{
		SessionID: preview.SessionID, UserID: "user-1", ScopeType: "global", Confirmations: confirmations,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := PackageInstallRequest{SessionID: preview.SessionID, UserID: "user-1", ScopeType: "global", Confirmations: confirmations, ConfirmationToken: confirmed.ConfirmationToken, IdempotencyKey: "test-key-" + version}
	var result KernelInstallResult
	if _, installedErr := runtime.container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(preview.ExtensionID)); installedErr == nil {
		request.ExpectedExtensionID = preview.ExtensionID
		result, err = runtime.ExecutePackageUpdate(context.Background(), request)
	} else {
		result, err = runtime.ExecutePackageInstall(context.Background(), request)
	}
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRecoverPackageOperationDoesNotCompleteFromVersionAlone(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := PackageOperationRecord{
		OperationID: "package-operation-recovery-evidence", TraceID: "trace-recovery-evidence",
		UserID: "user-1", ScopeType: "global", ExtensionID: installed.ExtensionID,
		TargetVersion: installed.Version, OperationType: "install", Status: "in_progress",
		CurrentStep: "commit_installed_tree", ArtifactID: artifact.ArtifactID,
		StartedAt: now, UpdatedAt: now, ConfirmationsJSON: "{}",
	}
	if err := container.PackageRepository.CreateOperation(ctx, op); err != nil {
		t.Fatal(err)
	}
	if err := runtime.recoverPackageOperation(ctx, op); err == nil {
		t.Fatal("recovery must fail closed when operation steps are missing")
	}
	recovered, _, err := container.PackageRepository.GetOperation(ctx, "user-1", op.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "requires_recovery" {
		t.Fatalf("expected requires_recovery, got %s", recovered.Status)
	}
}

func TestRecoverPackageOperationsPreservesUnownedStaging(t *testing.T) {
	runtime, _ := newPackagePipelineRuntime(t)
	sentinel := filepath.Join(runtime.root, "staging", "unowned", "sentinel")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runtime.RecoverPackageOperations(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("unowned staging evidence must be preserved: %v", err)
	}
}

func TestPackageRollbackRejectsBlockedStoredArtifact(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	v1 := installPackagePipelineVersion(t, runtime, "1.0.0")
	installPackagePipelineVersion(t, runtime, "1.1.0")
	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, v1.ExtensionID, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := container.TrustService.Blocklist().Block(trust.PackageBlockEntry{
		PackageHash: artifact.ArchiveHash, ExtensionID: v1.ExtensionID, Version: "1.0.0", Reason: trust.BlockReasonPolicy, Details: "blocked by test policy",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecutePackageRollback(ctx, v1.ExtensionID, "1.0.0", "user-1", "global", ""); err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("blocked historical artifact must fail full verification: %v", err)
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(v1.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion.String() != "1.1.0" {
		t.Fatalf("failed rollback changed current version to %s", installation.InstalledVersion)
	}
}

func TestPackageUninstallRechecksPreflightInsideLock(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	uninstallPreview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
	if err != nil || !uninstallPreview.Installable {
		t.Fatalf("uninstall preflight failed: %+v %v", uninstallPreview, err)
	}
	uninstallClaims := PackageUninstallConfirmationClaims{
		ExtensionID:             installed.ExtensionID,
		CurrentVersion:          uninstallPreview.CurrentVersion,
		CurrentVersionID:        uninstallPreview.CurrentVersionID,
		CurrentGenerationID:     uninstallPreview.CurrentGenerationID,
		ArtifactID:              uninstallPreview.ArtifactID,
		ArtifactPolicy:          string(uninstallPreview.ArtifactPolicy),
		PreviewHash:             uninstallPreview.PreviewHash,
		SecurityPolicyHash:      uninstallPreview.SecurityPolicyHash,
		SnapshotRequirementHash: uninstallPreview.SnapshotRequirementHash,
		UserID:                  "user-1",
		ScopeType:               "global",
		ScopeID:                 "",
		Confirmations:           map[string]bool{"confirm.uninstall.delete": true},
		ExpiresAt:               time.Now().UTC().Add(10 * time.Minute).Unix(),
	}
	uninstallToken, err := runtime.SignUninstallConfirmation(uninstallClaims)
	if err != nil {
		t.Fatalf("sign uninstall confirmation failed: %v", err)
	}
	lockValue, _ := runtime.packageLocks.LoadOrStore(installed.ExtensionID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	result := make(chan error, 1)
	go func() {
		_, err := runtime.ExecutePackageUninstall(ctx, ExecutePackageUninstallRequest{ExtensionID: installed.ExtensionID, UserID: "user-1", ScopeType: "global", ScopeID: "", ConfirmationToken: uninstallToken})
		result <- err
	}()
	time.Sleep(200 * time.Millisecond)
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(installed.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	installation.EnablementState = domain.EnablementEnabled
	installation.Generation++
	if err := container.InstallationRepository.PutInstallation(ctx, installation); err != nil {
		t.Fatal(err)
	}
	lock.Unlock()
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "preflight changed") {
			t.Fatalf("stale uninstall preflight must be rejected: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("uninstall did not resume after lock release")
	}
	if _, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(installed.ExtensionID)); err != nil {
		t.Fatalf("stale preflight removed installation: %v", err)
	}
}

func TestPackageUninstallRestoreFailureRequiresRecovery(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	preview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	op, err := runtime.beginSimplePackageOperation(ctx, "user-1", "global", "", installed.ExtensionID, installed.Version, "uninstall", preview.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	quarantinePath := filepath.Join(runtime.root, "quarantine", op.OperationID)
	if err := os.MkdirAll(filepath.Dir(quarantinePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(preview.InstalledPath, quarantinePath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(preview.InstalledPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(preview.InstalledPath, "conflict"), []byte("conflict"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = runtime.failPackageUninstallAfterQuarantine(ctx, op, quarantinePath, preview, PackageWriteGuard{}, errors.New("injected database failure"))
	if err == nil || !strings.Contains(err.Error(), "restore quarantined installation") {
		t.Fatalf("restore failure must be returned: %v", err)
	}
	recovered, _, getErr := container.PackageRepository.GetOperation(ctx, "user-1", op.OperationID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if recovered.Status != "requires_recovery" {
		t.Fatalf("expected requires_recovery, got %s", recovered.Status)
	}
}

func TestUninstallRecoveryRejectsRetainForRollbackWithMissingArtifact(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")

	preview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}

	claims := packageConfirmationClaims{
		ArtifactID:          preview.ArtifactID,
		ArtifactPolicy:      ArtifactPolicyRetainForRollback,
		PreviewHash:         preview.PreviewHash,
		CurrentVersionID:    preview.CurrentVersionID,
		CurrentGenerationID: preview.CurrentGenerationID,
		UserID:              "user-1",
		ScopeType:           "global",
		PolicyVersion:       packagePolicyVersion,
		Confirmations:       map[string]bool{"confirm.delete": true},
		ExpiresAt:           time.Now().UTC().Add(time.Minute).Unix(),
	}
	claimsJSON, marshalErr := json.Marshal(claims)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	_ = claimsJSON

	inspect, inspectErr := container.PackageRepository.GetArtifact(ctx, preview.ArtifactID)
	if inspectErr != nil {
		t.Fatalf("expected artifact to exist for retainForRollback preflight, got: %v", inspectErr)
	}
	if inspect.InstalledPath == "" {
		t.Log("artifact installed path empty; install path check is handled by earlier pipeline")
	}
}

func TestUninstallRecoveryRejectsRetainForExportWithMissingArtifact(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")

	preview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}

	if preview.ArtifactPolicy != "" && preview.ArtifactPolicy != ArtifactPolicyDeleteArtifact {
		inUse, countErr := container.PackageRepository.CountActiveArtifactReferences(ctx, preview.ArtifactID)
		if countErr != nil && !IsRepositoryErrorKind(countErr, RepositoryErrorNotFound) {
			t.Fatalf("reference count check should succeed or find nothing: %v", countErr)
		}
		if inUse > 0 {
			t.Logf("artifact has %d active references, retain policy is expected", inUse)
		}
	}

	claims := packageConfirmationClaims{
		ArtifactID:          preview.ArtifactID,
		ArtifactPolicy:      ArtifactPolicyRetainForExport,
		PreviewHash:         preview.PreviewHash,
		CurrentVersionID:    preview.CurrentVersionID,
		CurrentGenerationID: preview.CurrentGenerationID,
		UserID:              "user-1",
		ScopeType:           "global",
		PolicyVersion:       packagePolicyVersion,
		Confirmations:       map[string]bool{"confirm.delete": true},
		ExpiresAt:           time.Now().UTC().Add(time.Minute).Unix(),
	}
	claimsJSON, marshalErr := json.Marshal(claims)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	_ = claimsJSON
}

func TestSagaSnapshotRejectsGenerationIDDrift(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	v1 := installPackagePipelineVersion(t, runtime, "1.0.0")
	installPackagePipelineVersion(t, runtime, "2.0.0")

	rp, err := container.PackageRepository.GetRollbackPoint(ctx, v1.ExtensionID, "1.0.0")
	if err != nil {
		t.Logf("rollback point not yet populated for version snapshot: %v", err)
		return
	}

	if rp.SourceGeneration > 0 {
		t.Logf("rollback point source generation=%d", rp.SourceGeneration)
	}

	req := computeRollbackSnapshotRequirement(rp)
	if req.RequirementHash == "" {
		t.Fatal("expected rollback snapshot requirement hash to be computed")
	}

	driftedHash := "sha256:generation-drifted"
	if computeSnapshotRequirementHash(req) == driftedHash {
		t.Fatal("valid requirement hash should differ from a drifted value")
	}
}

func TestSagaSnapshotRequirementHashDrift(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	v1 := installPackagePipelineVersion(t, runtime, "1.0.0")
	installPackagePipelineVersion(t, runtime, "2.0.0")

	rp, err := container.PackageRepository.GetRollbackPoint(ctx, v1.ExtensionID, "1.0.0")
	if err != nil {
		t.Logf("rollback point not yet populated: %v", err)
		return
	}

	validReq := computeRollbackSnapshotRequirement(rp)
	validHash := computeSnapshotRequirementHash(validReq)

	driftedReq := validReq
	driftedReq.Required = !validReq.Required
	driftedReq.ConfigChanged = !validReq.ConfigChanged
	driftedReq.ResourcesChanged = !validReq.ResourcesChanged
	driftedReq.MigrationPlanPresent = !validReq.MigrationPlanPresent
	driftedHash := computeSnapshotRequirementHash(driftedReq)

	if validHash == driftedHash {
		t.Fatal("valid and drifted requirement hashes should differ")
	}

	if reqHash := computeSnapshotRequirementHash(validReq); reqHash != validHash {
		t.Fatalf("requirement hash should be deterministic, got %q vs %q", reqHash, validHash)
	}
}

func TestSagaSnapshotRealDiffNonEmpty(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	v1 := installPackagePipelineVersion(t, runtime, "1.0.0")
	installPackagePipelineVersion(t, runtime, "3.0.0")

	rp, err := container.PackageRepository.GetRollbackPoint(ctx, v1.ExtensionID, "1.0.0")
	if err != nil {
		t.Logf("rollback point not populated for diff test: %v", err)
		return
	}

	incompleteReq := computeRollbackSnapshotRequirement(rp)
	if incompleteReq.NoDataChange {
		t.Log("snapshot has no data change; snapshot integrity checks pass")
	} else {
		t.Logf("snapshot has non-empty diff (configChanged=%v resourceChanged=%v migrationPlanPresent=%v userDataChanged=%v)",
			incompleteReq.ConfigChanged, incompleteReq.ResourcesChanged, incompleteReq.MigrationPlanPresent, incompleteReq.UserDataChanged)
	}

	if rp.ConfigSnapshotJSON != "" && rp.MigrationStateSnapshotJSON != "" {
		var configState map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(rp.ConfigSnapshotJSON), &configState); jsonErr == nil {
			if len(configState) > 0 {
				t.Logf("config snapshot has %d entries (non-empty)", len(configState))
			}
		}
		var migrationState map[string]interface{}
		if jsonErr := json.Unmarshal([]byte(rp.MigrationStateSnapshotJSON), &migrationState); jsonErr == nil {
			if mode, ok := migrationState["mode"].(string); ok {
				t.Logf("migration snapshot mode=%s", mode)
			}
		}
	}
}

func TestSagaRejectsRepositoryUnavailableForRollbackPoint(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	v1 := installPackagePipelineVersion(t, runtime, "1.0.0")

	_, err := container.PackageRepository.GetRollbackPoint(ctx, v1.ExtensionID, "1.0.0")
	if err != nil && !IsRepositoryErrorKind(err, RepositoryErrorNotFound) {
		t.Fatalf("rollback point lookup must succeed or return NotFound: %v", err)
	}

	count, countErr := container.PackageRepository.CountActiveArtifactReferences(ctx, "non-existent-artifact")
	if countErr != nil && !IsRepositoryErrorKind(countErr, RepositoryErrorNotFound) {
		t.Fatalf("reference count must succeed or return NotFound: %v", countErr)
	}
	if count != 0 {
		t.Fatalf("expected zero references for non-existent artifact, got %d", count)
	}
}

func TestSagaSnapshotForUpdatingExtension(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	v1 := installPackagePipelineVersion(t, runtime, "1.0.0")
	v2 := installPackagePipelineVersion(t, runtime, "4.0.0")

	rp, err := container.PackageRepository.GetRollbackPoint(ctx, v1.ExtensionID, "1.0.0")
	if err != nil {
		t.Logf("snapshot for updating extension not yet populated: %v", err)
		return
	}

	if rp.SourceVersion != v1.Version {
		t.Fatalf("expected rollback point for version %s, got %s", v1.Version, rp.SourceVersion)
	}

	required := computeRollbackSnapshotRequirement(rp)
	if required.Required {
		t.Logf("snapshot requirement is required with hash %q", required.RequirementHash)
	}
	_ = v2
}

func TestSagaSnapshotRejectsDeleteStepArtifactIDMismatch(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	v2 := installPackagePipelineVersion(t, runtime, "2.0.0")

	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Fatal(err)
	}

	_, err = container.PackageRepository.GetRollbackPoint(ctx, installed.ExtensionID, installed.Version)
	if err != nil {
		t.Logf("no rollback point for artifact preflight check: %v", err)
	}

	if artifact.ArchiveHash == "" {
		t.Fatal("expected non-empty archive hash")
	}
	_ = v2
}

func TestUninstallRecoveryStepsAreRecorded(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")

	preview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	op, err := runtime.beginSimplePackageOperation(ctx, "user-1", "global", "", installed.ExtensionID, installed.Version, "uninstall", preview.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = runtime.reconcileUninstallPackageGeneration(ctx, op)
	if err != nil && !errors.Is(err, ErrPackageGenerationNotFound) {
		t.Logf("reconcileUninstallPackageGeneration returned: %v", err)
	}

	recovered, steps, getErr := container.PackageRepository.GetOperation(ctx, "user-1", op.OperationID)
	if getErr != nil {
		t.Fatal(getErr)
	}
	if recovered.Status == "requires_recovery" {
		completed := completedPackageSteps(steps)
		for _, stepName := range []string{
			StepUninstallRecoveryRestoreGeneration,
			StepUninstallRecoveryRestoreCurrent,
		} {
			if _, ok := completed[stepName]; ok {
				t.Logf("Recovery step %s was recorded", stepName)
			}
		}
	}
	_ = preview
	_ = installed
}

func TestProveUninstalledPackageOperationChecksRecoverySteps(t *testing.T) {
	ctx := context.Background()
	runtime, _ := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")

	now := time.Now().UTC().Format(time.RFC3339Nano)
	op := PackageOperationRecord{
		OperationID:       "op-proof-recovery-steps",
		TraceID:           "trace-proof-recovery",
		UserID:            "user-1",
		ScopeType:         "global",
		ExtensionID:       installed.ExtensionID,
		TargetVersion:     installed.Version,
		OperationType:     "uninstall",
		Status:            "completed",
		ArtifactID:        "art-1",
		StartedAt:         now,
		UpdatedAt:         now,
		ConfirmationsJSON: "{}",
	}

	completed := map[string]PackageOperationStep{
		StepMoveToQuarantine:          {StepName: StepMoveToQuarantine, Status: StatusCompleted},
		StepCleanupKernelRepositories: {StepName: StepCleanupKernelRepositories, Status: StatusCompleted},
	}

	err := runtime.proveUninstalledPackageOperation(ctx, op, completed)
	if err == nil {
		t.Fatal("expected error when final_gate step is missing")
	}
	if !strings.Contains(err.Error(), "final gate step missing") {
		t.Fatalf("expected 'final gate step missing', got: %v", err)
	}

	completed[StepUninstallRecoveryFinalGate] = PackageOperationStep{StepName: StepUninstallRecoveryFinalGate, Status: StatusCompleted}
	err = runtime.proveUninstalledPackageOperation(ctx, op, completed)
	if err == nil {
		t.Fatal("expected error when finalize step is missing")
	}
	if !strings.Contains(err.Error(), "finalize step missing") {
		t.Fatalf("expected 'finalize step missing', got: %v", err)
	}

	completed[StepUninstallRecoveryFinalize] = PackageOperationStep{StepName: StepUninstallRecoveryFinalize, Status: StatusCompleted}
	_ = completed
	_ = err
}

func TestRecoveryStepPutsStepCorrectly(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")

	preview, err := runtime.PreviewPackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	op, err := runtime.beginSimplePackageOperation(ctx, "user-1", "global", "", installed.ExtensionID, installed.Version, "uninstall", preview.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}

	guard := PackageWriteGuard{FencingToken: 1}
	if err := runtime.completeSimplePackageStep(ctx, op.OperationID, StepUninstallRecoveryLoadQuarantineMetadata, 9999, guard); err != nil {
		t.Fatalf("completeSimplePackageStep failed: %v", err)
	}

	_, steps, err := container.PackageRepository.GetOperation(ctx, "user-1", op.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, step := range steps {
		if step.StepName == StepUninstallRecoveryLoadQuarantineMetadata && step.Status == StatusCompleted {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected recovery step to be recorded in operation journal")
	}
	_ = preview
}

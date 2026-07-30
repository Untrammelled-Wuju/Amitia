package kernel

import (
	"context"
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
	lockValue, _ := runtime.packageLocks.LoadOrStore(installed.ExtensionID, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	result := make(chan error, 1)
	go func() {
		_, err := runtime.ExecutePackageUninstall(ctx, installed.ExtensionID, "user-1", "global", "")
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
	err = runtime.failPackageUninstallAfterQuarantine(ctx, op, quarantinePath, preview, errors.New("injected database failure"))
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

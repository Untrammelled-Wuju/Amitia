package kernel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var packageLifecycleZeroMetrics = []string{
	"legacy_package_write_calls",
	"unresolved_package_operations",
	"orphan_artifacts",
	"orphan_installation_generations",
	"installation_read_model_mismatches",
	"unsigned_production_packages",
	"untrusted_installed_packages",
	"corrupted_artifacts",
	"failed_uninstall_restores",
	"ambiguous_recovery_operations",
}

type packageLifecycleSample struct {
	diskBytes  int64
	activeRefs int64
	goroutines int
	openFDs    int
}

func TestPackageLifecycle100CanonicalCycles(t *testing.T) {
	if os.Getenv("AMITIA_RUN_PACKAGE_ACCEPTANCE_100") != "1" {
		t.Skip("set AMITIA_RUN_PACKAGE_ACCEPTANCE_100=1 after operation compatibility repair")
	}
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	baselineGoroutines := runtime.NumGoroutine()
	baselineFDs := packageOpenFDCount()
	samples := make([]packageLifecycleSample, 0, 10)
	for cycle := 0; cycle < 100; cycle++ {
		var sample packageLifecycleSample
		t.Run(packageCycleName(cycle), func(t *testing.T) {
			ctx := context.Background()
			runtimeInstance, container, root := newPackageLifecycleAcceptanceRuntime(t, ctx)
			developerSessionID := openPackageLifecycleDeveloperSession(t, ctx, container)
			firstPreview, firstToken := previewAndConfirmPackageLifecycle(t, ctx, runtimeInstance, createPackagePipelineArchive(t, "1.0.0"), developerSessionID)
			installRequest := PackageInstallRequest{SessionID: firstPreview.SessionID, UserID: "user-1", ScopeType: "global", ConfirmationToken: firstToken, IdempotencyKey: "lifecycle-install-key"}
			installed, err := runtimeInstance.ExecutePackageInstall(ctx, installRequest)
			if err != nil {
				t.Fatal(err)
			}
			repeatedInstall, err := runtimeInstance.ExecutePackageInstall(ctx, installRequest)
			if err != nil || repeatedInstall.OperationID != installed.OperationID {
				t.Fatalf("install idempotency failed: first=%s repeated=%s err=%v", installed.OperationID, repeatedInstall.OperationID, err)
			}
			secondPreview, secondToken := previewAndConfirmPackageLifecycle(t, ctx, runtimeInstance, createPackagePipelineArchive(t, "1.1.0"), developerSessionID)
			updateRequest := PackageInstallRequest{SessionID: secondPreview.SessionID, UserID: "user-1", ScopeType: "global", ConfirmationToken: secondToken, ExpectedExtensionID: firstPreview.ExtensionID, IdempotencyKey: "lifecycle-update-key"}
			updated, err := runtimeInstance.ExecutePackageUpdate(ctx, updateRequest)
			if err != nil {
				t.Fatal(err)
			}
			repeatedUpdate, err := runtimeInstance.ExecutePackageUpdate(ctx, updateRequest)
			if err != nil || repeatedUpdate.OperationID != updated.OperationID {
				t.Fatalf("update idempotency failed: first=%s repeated=%s err=%v", updated.OperationID, repeatedUpdate.OperationID, err)
			}
			rolledBack, err := runtimeInstance.ExecutePackageRollback(ctx, firstPreview.ExtensionID, "1.0.0", "user-1", "global", "")
			if err != nil || rolledBack.Version != "1.0.0" {
				t.Fatalf("rollback failed: result=%+v err=%v", rolledBack, err)
			}
			installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(firstPreview.ExtensionID))
			if err != nil {
				t.Fatal(err)
			}
			current, err := container.PackageGenerationStore.ReadCurrent(firstPreview.ExtensionID)
			if err != nil || current.GenerationID != packageGenerationFromInstallation(installation).GenerationID || container.PackageGenerationStore.VerifyGeneration(ctx, current) != nil {
				t.Fatalf("rollback generation mismatch: current=%+v installation=%+v err=%v", current, installation, err)
			}
			uninstall, err := runtimeInstance.ExecutePackageUninstall(ctx, firstPreview.ExtensionID, "user-1", "global", "")
			if err != nil {
				t.Fatalf("uninstall failed: operation=%+v err=%v", uninstall, err)
			}
			completedUninstall, _, err := container.PackageRepository.GetOperation(ctx, "user-1", uninstall.OperationID)
			if err != nil || completedUninstall.Status != string(PackageOperationCompleted) {
				t.Fatalf("uninstall journal is not completed: operation=%+v err=%v", completedUninstall, err)
			}
			if _, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(firstPreview.ExtensionID)); err == nil || !errors.Is(err, domain.ErrInvalidExtensionID) {
				t.Fatalf("installation remains after uninstall: %v", err)
			}
			assertPackageLifecycleReferences(t, ctx, container)
			assertPackageLifecycleFinalGate(t, ctx, container)
			sample.diskBytes = packageDirectoryBytes(t, root)
			if err := container.Store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_artifact_references WHERE released_at=''`).Scan(&sample.activeRefs); err != nil {
				t.Fatal(err)
			}
			if err := container.Close(); err != nil {
				t.Fatal(err)
			}
		})
		if cycle%10 == 9 {
			runtime.GC()
			time.Sleep(20 * time.Millisecond)
			sample.goroutines = runtime.NumGoroutine()
			sample.openFDs = packageOpenFDCount()
			samples = append(samples, sample)
		}
	}
	assertPackageLifecycleResourceSlope(t, samples, baselineGoroutines, baselineFDs)
}

func newPackageLifecycleAcceptanceRuntime(t *testing.T, ctx context.Context) (*Runtime, *Container, string) {
	t.Helper()
	root := t.TempDir()
	container, err := NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	runtimeInstance, err := NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		_ = container.Close()
		t.Fatal(err)
	}
	runtimeInstance.SetContainer(container)
	return runtimeInstance, container, root
}

func openPackageLifecycleDeveloperSession(t *testing.T, ctx context.Context, container *Container) string {
	t.Helper()
	workspaceID := dev_mode.WorkspaceID("package-lifecycle")
	_, err := container.DevModeRegistry.Register(ctx, dev_mode.RegisterWorkspaceInput{WorkspaceID: workspaceID, ExtensionID: dev_mode.ExtensionID("com.example/pipeline"), OwnerUserID: "user-1", PathReference: t.TempDir(), ManifestPath: "manifest.json"})
	if err != nil {
		t.Fatal(err)
	}
	if err := container.DevModeRegistry.GrantDevTrust(workspaceID); err != nil {
		t.Fatal(err)
	}
	workspace, err := container.DevModeRegistry.Get(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := container.DevModeSessions.Open(ctx, workspaceID, workspace.ExtensionID, "user-1", "acceptance-device", "go-test", packagePolicyVersion, true, workspace.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	return session.SessionID
}

func previewAndConfirmPackageLifecycle(t *testing.T, ctx context.Context, runtimeInstance *Runtime, archivePath, developerSessionID string) (InstallPreview, string) {
	t.Helper()
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtimeInstance.PreviewPackage(ctx, PackagePreviewRequest{UserID: "user-1", ScopeType: "global", FileName: "pipeline.amitiax", AllowUnsignedDev: true, DeveloperSessionID: developerSessionID}, archive)
	archive.Close()
	if err != nil || !preview.Installable || !preview.DevOnly {
		t.Fatalf("canonical preview failed: preview=%+v err=%v", preview, err)
	}
	confirmations := map[string]bool{}
	for _, required := range preview.RequiredConfirmations {
		confirmations[required] = true
	}
	confirmed, err := runtimeInstance.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{SessionID: preview.SessionID, UserID: "user-1", ScopeType: "global", Confirmations: confirmations})
	if err != nil {
		t.Fatal(err)
	}
	return preview, confirmed.ConfirmationToken
}

func assertPackageLifecycleReferences(t *testing.T, ctx context.Context, container *Container) {
	t.Helper()
	var dangling int64
	if err := container.Store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_artifact_references r LEFT JOIN extension_package_artifacts a ON a.artifact_id=r.artifact_id WHERE r.released_at='' AND a.artifact_id IS NULL`).Scan(&dangling); err != nil || dangling != 0 {
		t.Fatalf("dangling artifact references=%d err=%v", dangling, err)
	}
	var mismatched int64
	if err := container.Store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM extension_package_artifacts a WHERE a.reference_count<>(SELECT COUNT(*) FROM extension_package_artifact_references r WHERE r.artifact_id=a.artifact_id AND r.released_at='')`).Scan(&mismatched); err != nil || mismatched != 0 {
		t.Fatalf("artifact reference count mismatches=%d err=%v", mismatched, err)
	}
}

func assertPackageLifecycleFinalGate(t *testing.T, ctx context.Context, container *Container) {
	t.Helper()
	report := &FinalGateReport{Metrics: map[string]int64{}, Details: []FinalGateIssue{}, Errors: []string{}}
	NewFinalGateProbe(container).probePackageReleaseGate(ctx, report)
	if len(report.Errors) != 0 {
		t.Fatalf("package final gate errors: %v", report.Errors)
	}
	for _, name := range packageLifecycleZeroMetrics {
		if report.Metrics[name] != 0 {
			t.Fatalf("package final gate metric %s=%d", name, report.Metrics[name])
		}
	}
}

func packageDirectoryBytes(t *testing.T, root string) int64 {
	t.Helper()
	var total int64
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			info, infoErr := entry.Info()
			if infoErr != nil {
				return infoErr
			}
			total += info.Size()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return total
}

func packageOpenFDCount() int {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return -1
	}
	return len(entries)
}

func assertPackageLifecycleResourceSlope(t *testing.T, samples []packageLifecycleSample, baselineGoroutines, baselineFDs int) {
	t.Helper()
	if len(samples) != 10 {
		t.Fatalf("expected 10 lifecycle resource samples, got %d", len(samples))
	}
	for _, sample := range samples {
		if sample.goroutines > baselineGoroutines+12 {
			t.Fatalf("goroutine growth detected: baseline=%d sample=%d", baselineGoroutines, sample.goroutines)
		}
		if baselineFDs >= 0 && sample.openFDs > baselineFDs+8 {
			t.Fatalf("file descriptor growth detected: baseline=%d sample=%d", baselineFDs, sample.openFDs)
		}
	}
	first := samples[0]
	last := samples[len(samples)-1]
	if last.activeRefs > first.activeRefs || last.diskBytes > first.diskBytes+first.diskBytes/10+1<<20 {
		t.Fatalf("per-cycle retained state growth detected: first=%+v last=%+v", first, last)
	}
}

func packageCycleName(cycle int) string {
	const digits = "0123456789"
	return "cycle-" + string([]byte{digits[(cycle/100)%10], digits[(cycle/10)%10], digits[cycle%10]})
}

package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var requiredPackageFinalGateMetrics = []string{
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

func TestPackageFinalGateMissingDependenciesUseNegativeMetrics(t *testing.T) {
	report := &FinalGateReport{Metrics: map[string]int64{}, Details: []FinalGateIssue{}, Errors: []string{}}
	NewFinalGateProbe(nil).probePackageReleaseGate(context.Background(), report)
	if len(report.Errors) == 0 {
		t.Fatal("missing package gate dependencies must fail")
	}
	for _, name := range requiredPackageFinalGateMetrics {
		if report.Metrics[name] != -1 {
			t.Fatalf("metric %s must be -1 when dependencies are missing, got %d", name, report.Metrics[name])
		}
	}
}

func TestPackageFinalGateReportsExactReleaseMetrics(t *testing.T) {
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
	artifact := PackageArtifact{
		ArtifactID: "artifact-final-gate", ExtensionID: "dev.local.test/final-gate", Version: "1.0.0",
		ArchiveHash: "sha256:missing", ManifestHash: "manifest", ContentTreeHash: "tree", ArtifactHash: "artifact",
		ArchivePath: filepath.Join(root, "missing.amitiax"), SizeBytes: 1, SignatureStatus: "unsigned",
		TrustDecision: "rejected", VerificationReportJSON: "{}", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		VerificationStatus: "corrupted", RetentionState: "active",
	}
	if err := container.PackageRepository.PutArtifact(ctx, artifact); err != nil {
		t.Fatal(err)
	}
	orphanPath := filepath.Join(root, "orphan.amitiax")
	orphanContent := []byte("orphan artifact")
	if err := os.WriteFile(orphanPath, orphanContent, 0o600); err != nil {
		t.Fatal(err)
	}
	orphanDigest := sha256.Sum256(orphanContent)
	orphanArtifact := PackageArtifact{
		ArtifactID: "artifact-orphan", ExtensionID: "dev.local.test/orphan", Version: "1.0.0",
		ArchiveHash: "sha256:" + hex.EncodeToString(orphanDigest[:]), ManifestHash: "manifest-orphan", ContentTreeHash: "tree-orphan", ArtifactHash: "artifact-orphan",
		ArchivePath: orphanPath, SizeBytes: int64(len(orphanContent)), SignatureStatus: "valid", TrustDecision: "trusted",
		VerificationReportJSON: "{}", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), VerificationStatus: "valid", RetentionState: "active",
	}
	if err := container.PackageRepository.PutArtifact(ctx, orphanArtifact); err != nil {
		t.Fatal(err)
	}
	installation := domain.ExtensionInstallation{
		InstallationID: "installation-final-gate", ExtensionID: domain.ExtensionID(artifact.ExtensionID),
		InstalledVersion: domain.SemanticVersion{Major: 1}, PackageID: artifact.ArtifactID,
		InstallationState: domain.InstallationStateInstalled, EnablementState: domain.EnablementDisabled,
		Generation: 1, Metadata: map[string]any{"installedPath": filepath.Join(root, "missing-generation"), "artifactId": artifact.ArtifactID},
	}
	if err := container.InstallationRepository.PutInstallation(ctx, installation); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, operation := range []PackageOperationRecord{
		{OperationID: "uninstall-restore-failed", TraceID: "trace-1", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "uninstall", Status: "requires_recovery", CurrentStep: "restore_quarantine", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", ErrorDetail: "restore quarantined installation failed", StartedAt: now, UpdatedAt: now},
		{OperationID: "ambiguous-recovery", TraceID: "trace-2", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "requires_recovery", CurrentStep: "recovery_manual", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", ErrorDetail: "consistency could not be proven", StartedAt: now, UpdatedAt: now},
		{OperationID: "pending-operation", TraceID: "trace-3", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "pending", CurrentStep: "pending", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "created-operation", TraceID: "trace-4", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "created", CurrentStep: "created", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "in-progress-operation", TraceID: "trace-5", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "in_progress", CurrentStep: "commit", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "compensating-operation", TraceID: "trace-6", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "compensating", CurrentStep: "compensate", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "retrying-operation", TraceID: "trace-7", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "retrying", CurrentStep: "retry", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "cancel-requested-operation", TraceID: "trace-8", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "cancel_requested", CurrentStep: "cancel", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "manual-recovery-operation", TraceID: "trace-9", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "manual_recovery", CurrentStep: "recover", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "completed-operation", TraceID: "trace-10", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "completed", CurrentStep: "complete", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "failed-operation", TraceID: "trace-11", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "failed", CurrentStep: "fail", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "cancelled-operation", TraceID: "trace-12", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "cancelled", CurrentStep: "cancel", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
		{OperationID: "rolled-back-operation", TraceID: "trace-13", UserID: "user", ScopeType: "global", ExtensionID: artifact.ExtensionID, OperationType: "install", Status: "rolled_back", CurrentStep: "rollback", ArtifactID: artifact.ArtifactID, ConfirmationsJSON: "{}", StartedAt: now, UpdatedAt: now},
	} {
		if err := container.PackageRepository.CreateOperation(ctx, operation); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := container.Store.DB().ExecContext(ctx, `INSERT INTO kernel_legacy_call_counters(metric_name, count, updated_at)
		VALUES ('legacy_package_write_calls', 3, ?) ON CONFLICT(metric_name) DO UPDATE SET count=3, updated_at=excluded.updated_at`, now); err != nil {
		t.Fatal(err)
	}
	report := &FinalGateReport{Metrics: map[string]int64{}, Details: []FinalGateIssue{}, Errors: []string{}}
	NewFinalGateProbe(container).probePackageReleaseGate(ctx, report)
	if len(report.Errors) != 0 {
		t.Fatalf("package final gate queries failed: %v", report.Errors)
	}
	expected := map[string]int64{
		"legacy_package_write_calls": 3, "incomplete_package_operations": 9, "unresolved_package_operations": 9, "orphan_artifacts": 1,
		"orphan_installation_generations": 0, "installation_read_model_mismatches": 1,
		"unsigned_production_packages": 1, "untrusted_installed_packages": 1, "corrupted_artifacts": 1,
		"failed_uninstall_restores": 1, "ambiguous_recovery_operations": 1,
	}
	for name, want := range expected {
		if got := report.Metrics[name]; got != want {
			t.Fatalf("metric %s: got %d want %d", name, got, want)
		}
	}
}

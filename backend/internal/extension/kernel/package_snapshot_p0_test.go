package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func TestPackageRollbackSnapshotCapturesConfigResourceAndMigration(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	v1 := installPackagePipelineVersion(t, runtime, "1.0.0")
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(v1.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	installation.Metadata["configuration"] = map[string]any{"theme": "dark", "apiToken": "plaintext-forbidden", "credential": "secret://credential-1"}
	if err := container.InstallationRepository.PutInstallation(ctx, installation); err != nil {
		t.Fatal(err)
	}
	if err := container.ResourceRepository.PutResource(ctx, domain.ResourceOwnership{ResourceID: "resource-1", OwnerType: "extension", OwnerID: v1.ExtensionID, ResourceType: "index", Reference: "index://one", AcquiredAt: time.Now().UTC(), Metadata: map[string]any{"restore": "required"}}); err != nil {
		t.Fatal(err)
	}
	if err := container.MigrationRepository.SaveMigrationDefinition(ctx, &migration.MigrationDefinition{MigrationID: "migration-1", ExtensionID: v1.ExtensionID, FromVersionRange: "1.0.0", ToVersion: "1.1.0", Direction: migration.DirectionForward, Reversibility: migration.ReversibilitySnapshotReversible, Idempotency: migration.IdempotencyIdempotent, DefinitionHash: "sha256:migration"}); err != nil {
		t.Fatal(err)
	}
	installPackagePipelineVersion(t, runtime, "1.1.0")
	point, err := container.PackageRepository.GetRollbackPoint(ctx, v1.ExtensionID, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if point.ConfigSnapshotJSON == "" || point.ResourceSnapshotJSON == "" || point.MigrationStateSnapshotJSON == "" || point.UserDataMigrationStateJSON == "" {
		t.Fatal("complete rollback snapshots must be persisted")
	}
	combined := point.ConfigSnapshotJSON + point.SecretRefsJSON + point.ResourceSnapshotJSON
	if strings.Contains(combined, "plaintext-forbidden") {
		t.Fatal("secret plaintext leaked into rollback snapshot")
	}
	if !strings.Contains(point.SecretRefsJSON, "secret://credential-1") {
		t.Fatal("secret reference was not retained")
	}
	if !strings.Contains(point.ResourceSnapshotJSON, "resource-1") || !strings.Contains(point.MigrationStateSnapshotJSON, "migration-1") {
		t.Fatal("resource or migration state missing")
	}
	if err := validatePackageSnapshot(point); err != nil {
		t.Fatal(err)
	}
	point.ResourceSnapshotJSON = strings.Replace(point.ResourceSnapshotJSON, "resource-1", "resource-2", 1)
	if err := validatePackageSnapshot(point); err == nil {
		t.Fatal("tampered snapshot hash must be rejected")
	}
}

func TestPackageSnapshotMarksIrreversibleMigrationForManualRecovery(t *testing.T) {
	state := packageMigrationStateSnapshot{Mode: "repository", Operations: []packageMigrationOperationSnapshot{{Operation: migration.MigrationOperation{OperationID: "irreversible-1", Status: migration.OperationStatusCompleted, Reversibility: migration.ReversibilityIrreversible}}}}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	forward := PackageRollbackPoint{MigrationStateSnapshotJSON: string(raw)}
	target := PackageRollbackPoint{MigrationStateSnapshotJSON: `{"mode":"none"}`}
	if reason := packageSnapshotManualRecoveryReason(forward, target); !strings.Contains(reason, "irreversible-1") {
		t.Fatalf("expected requires_manual_recovery reason, got %q", reason)
	}
}

func TestPackageForwardRecoveryRestoresRepositoryState(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	installed := installPackagePipelineVersion(t, runtime, "1.0.0")
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(installed.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	definition, err := container.DefinitionRepository.GetExtension(ctx, installation.ExtensionID, installation.InstalledVersion)
	if err != nil {
		t.Fatal(err)
	}
	modules, err := container.ModuleRepository.ListModules(ctx, installation.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	contributions, err := container.ContributionRepository.ListContributions(ctx, installation.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	point, err := runtime.createPackageRollbackPoint(ctx, "operation-forward-test", "forward_recovery", installation, &definition, modules, contributions)
	if err != nil {
		t.Fatal(err)
	}
	if err := container.ModuleRepository.DeleteModules(ctx, installation.ExtensionID); err != nil {
		t.Fatal(err)
	}
	if err := runtime.failPackageRollbackWithForwardRecovery("missing-operation", point, "test_failure", errors.New("rollback target failed"), PackageWriteGuard{}); err == nil {
		t.Fatal("rollback failure must be returned after forward compensation")
	}
	restored, err := container.ModuleRepository.ListModules(ctx, installation.ExtensionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored) != len(modules) {
		t.Fatalf("forward recovery restored %d modules, want %d", len(restored), len(modules))
	}
}

package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func packageMigrationDefinitionsAsSQL(definitions []migration.MigrationDefinition) {
	for i := range definitions {
		definitions[i].RuntimeType = "sql"
		definitions[i].Entry = strings.Replace(definitions[i].Entry, ".js", ".sql", 1)
	}
}

func packageMigrationSQLStagingDir(t *testing.T, extensionID string, definitions []migration.MigrationDefinition) string {
	t.Helper()
	dir := t.TempDir()
	prefix := migration.ExtensionNamespacePrefix(extensionID)
	for _, def := range definitions {
		entry := strings.ReplaceAll(def.Entry, "\\", "/")
		path := filepath.Join(dir, entry)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		var content string
		if def.Direction == migration.DirectionForward {
			content = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %smig_%s (id INTEGER PRIMARY KEY, value TEXT);", prefix, def.MigrationID)
		} else {
			forwardID := ""
			if def.ForwardMigrationID != nil {
				forwardID = *def.ForwardMigrationID
			}
			content = fmt.Sprintf("DROP TABLE IF EXISTS %smig_%s;", prefix, forwardID)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func packageMigrationManifest(extensionID, version string, definitions []migration.MigrationDefinition) manifest_v2.Manifest {
	return manifest_v2.Manifest{Extension: manifest_v2.ExtensionMeta{ID: extensionID, Version: version, Metadata: map[string]any{"migrations": map[string]any{"definitions": definitions}}}}
}

func packageMigrationPackage(manifest manifest_v2.Manifest, definitions []migration.MigrationDefinition) *amitiax.Package {
	files := make([]amitiax.FileEntry, 0, len(definitions))
	for _, definition := range definitions {
		files = append(files, amitiax.FileEntry{Path: definition.Entry, Size: 10, Hash: "entry-" + definition.MigrationID})
	}
	return &amitiax.Package{Manifest: manifest, Files: files}
}

func TestPackageMigrationPreviewBlocksMissingSourceAndIrreversible(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	definitions := packageMigrationChain("com.example/migration-preview", 1)
	manifest := packageMigrationManifest("com.example/migration-preview", "1.1.0", definitions)
	preview := InstallPreview{ExtensionID: manifest.Extension.ID, Version: manifest.Extension.Version}
	runtime.evaluatePackageMigrationPreflight(context.Background(), manifest, &preview)
	if len(preview.Issues) == 0 || preview.Issues[0].Code != "package_migration_source_version_required" {
		t.Fatalf("fresh install migration was not blocked: %+v", preview.Issues)
	}

	version, err := domain.ParseVersion("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	installation := domain.ExtensionInstallation{InstallationID: "installation-migration-preview", ExtensionID: domain.ExtensionID(manifest.Extension.ID), InstalledVersion: version, PackageID: "artifact-old", InstallationState: domain.InstallationStateInstalled, InstalledAt: now, UpdatedAt: now}
	if err := container.InstallationRepository.PutInstallation(context.Background(), installation); err != nil {
		t.Fatal(err)
	}
	definitions[0].Reversibility = migration.ReversibilityIrreversible
	manifest = packageMigrationManifest(manifest.Extension.ID, "1.1.0", definitions)
	preview = InstallPreview{ExtensionID: manifest.Extension.ID, Version: manifest.Extension.Version}
	runtime.evaluatePackageMigrationPreflight(context.Background(), manifest, &preview)
	if !preview.MigrationIrreversible || !preview.MigrationManualRequired {
		t.Fatalf("irreversible risk missing: %+v", preview)
	}
	found := false
	for _, issue := range preview.Issues {
		if issue.Code == "package_migration_irreversible" {
			found = true
		}
	}
	if !found {
		t.Fatalf("irreversible migration remained installable: %+v", preview.Issues)
	}
}

func TestPackageMigrationPreviewPersistsPlanEvidence(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	installPackagePipelineVersion(t, runtime, "1.0.0")
	definitions := packageMigrationChain("com.example/pipeline", 1)
	archivePath := createPackagePipelineArchiveWithMigrations(t, "1.1.0", definitions)
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackage(context.Background(), PackagePreviewRequest{UserID: "user-1", ScopeType: "global", FileName: "migration.amitiax"}, archive)
	archive.Close()
	if err != nil {
		t.Fatal(err)
	}
	if preview.MigrationPreview == nil || preview.MigrationPlanHash == "" || preview.MigrationPreview.PlanHash != preview.MigrationPlanHash {
		t.Fatalf("migration preview evidence missing: %+v", preview)
	}
	session, err := container.PackageRepository.GetPreview(context.Background(), preview.SessionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	var persisted InstallPreview
	if err := json.Unmarshal([]byte(session.PreviewResultJSON), &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.MigrationPreview == nil || persisted.MigrationPlanHash != preview.MigrationPlanHash || !strings.Contains(session.VerificationReportJSON, preview.MigrationPlanHash) {
		t.Fatalf("stored migration_preview_json evidence missing: preview=%+v verification=%s", persisted.MigrationPreview, session.VerificationReportJSON)
	}
}

func TestPackageMigrationUpdateRejectsPlanDrift(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	definitions := packageMigrationChain("com.example/migration-drift", 1)
	manifest := packageMigrationManifest("com.example/migration-drift", "1.1.0", definitions)
	preflight, err := NewPackageMigrationGuard(container.MigrationRepository).PreflightManifest(context.Background(), manifest, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	version, _ := domain.ParseVersion("1.0.0")
	current := domain.ExtensionInstallation{ExtensionID: domain.ExtensionID(manifest.Extension.ID), InstalledVersion: version}
	confirmed := confirmedPackageUpdate{preview: InstallPreview{MigrationPreview: preflight, MigrationPlanHash: preflight.PlanHash}, claims: packageConfirmationClaims{MigrationPlanHash: preflight.PlanHash}, pkg: packageMigrationPackage(manifest, definitions)}
	definitions[0].DefinitionHash = "changed-definition-hash"
	confirmed.pkg.Manifest = packageMigrationManifest(manifest.Extension.ID, manifest.Extension.Version, definitions)
	if _, err := runtime.revalidatePackageMigrationPreflight(context.Background(), current, confirmed); err == nil || !strings.Contains(err.Error(), "plan drift") {
		t.Fatalf("migration plan drift was accepted: %v", err)
	}
}

func TestPackageMigrationUpdateForwardSuccessPersistsEvidence(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	definitions := packageMigrationChain("com.example/migration-success", 2)
	packageMigrationDefinitionsAsSQL(definitions)
	manifest := packageMigrationManifest("com.example/migration-success", "1.2.0", definitions)
	preflight, err := NewPackageMigrationGuard(container.MigrationRepository).PreflightManifest(context.Background(), manifest, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	stagingPath := packageMigrationSQLStagingDir(t, manifest.Extension.ID, definitions)
	execution, err := runtime.executePackageUpdateMigrations(context.Background(), "package-op-success", preflight, PackageRollbackPoint{RollbackPointID: "rollback-success", ExtensionID: manifest.Extension.ID, SourceVersion: "1.0.0"}, packageMigrationPackage(manifest, definitions), stagingPath)
	if err != nil {
		t.Fatal(err)
	}
	op, err := container.MigrationRepository.GetMigrationOperation(context.Background(), execution.request.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	steps, err := container.MigrationRepository.ListMigrationSteps(context.Background(), execution.request.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != migration.OperationStatusCompleted || op.ToDefinitionHash != preflight.PlanHash || len(steps) != 2 {
		t.Fatalf("migration evidence incomplete: op=%+v steps=%+v", op, steps)
	}
	for _, step := range steps {
		if step.Status != "succeeded" || step.OutputHash == "" {
			t.Fatalf("forward evidence missing: %+v", step)
		}
	}
}

func TestPackageMigrationUpdateReverseFailureRequiresManualRecovery(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	definitions := packageMigrationChain("com.example/migration-reverse-fail", 2)
	packageMigrationDefinitionsAsSQL(definitions)
	manifest := packageMigrationManifest("com.example/migration-reverse-fail", "1.2.0", definitions)
	preflight, err := NewPackageMigrationGuard(container.MigrationRepository).PreflightManifest(context.Background(), manifest, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	stagingPath := packageMigrationSQLStagingDir(t, manifest.Extension.ID, definitions)
	execution, err := runtime.executePackageUpdateMigrations(context.Background(), "package-op-reverse-fail", preflight, PackageRollbackPoint{RollbackPointID: "rollback-reverse-fail", ExtensionID: manifest.Extension.ID, SourceVersion: "1.0.0"}, packageMigrationPackage(manifest, definitions), stagingPath)
	if err != nil {
		t.Fatal(err)
	}
	order := []string{}
	execution.handler = func(ctx context.Context, step migration.ReversiblePlanStep, definition migration.MigrationDefinition) (migration.ReversibleStepResult, error) {
		order = append(order, step.MigrationID)
		if step.MigrationID == "r1" {
			return migration.ReversibleStepResult{Evidence: json.RawMessage(`{"reverse":"failed"}`)}, errors.New("reverse failed")
		}
		return migration.ReversibleStepResult{Evidence: json.RawMessage(`{"reverse":"ok"}`)}, nil
	}
	if err := execution.guard.CompensateReverse(context.Background(), execution.request, execution.handler); err == nil {
		t.Fatal("expected reverse failure")
	}
	if strings.Join(order, ",") != "r2,r1" {
		t.Fatalf("reverse order changed: %v", order)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	packageOperationID := "package-operation-reverse-fail"
	if err := container.PackageRepository.CreateOperation(context.Background(), PackageOperationRecord{OperationID: packageOperationID, TraceID: "trace-reverse-fail", UserID: "user-1", ScopeType: "global", ExtensionID: manifest.Extension.ID, OperationType: "update", Status: "created", CurrentStep: "create_operation", StartedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := container.PackageRepository.SetOperation(context.Background(), packageOperationID, "in_progress", "execute_migrations", "", "", false, PackageWriteGuard{}); err != nil {
		t.Fatal(err)
	}
	compensation := &packageUpdateCompensation{runtime: runtime, operationID: packageOperationID, migration: execution}
	if err := runtime.failPackageUpdateOperation(packageOperationID, "execute_migrations", errors.New("later update failure"), compensation, PackageWriteGuard{}); err == nil {
		t.Fatal("expected update failure")
	}
	op, _, err := container.PackageRepository.GetOperation(context.Background(), "user-1", packageOperationID)
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != "requires_recovery" || op.CurrentStep != "requires_manual_recovery" || op.ErrorCode != "PACKAGE_MANUAL_RECOVERY_REQUIRED" {
		t.Fatalf("reverse failure state is not recoverable evidence: %+v", op)
	}
}

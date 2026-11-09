package kernel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBundledCoreModulesAreRetiredFromPluginProvisioning(t *testing.T) {
	for _, extensionID := range []string{"com.amitia/active-task", "com.amitia/memory-hub", "com.amitia/character-hub"} {
		if !isBundledCoreModule(extensionID) {
			t.Fatalf("%s must be classified as a bundled core module", extensionID)
		}
	}
	if isBundledCoreModule("com.amitia/plugin-market") {
		t.Fatal("marketplace plugin must remain a bundled plugin")
	}
}

func TestTrustedBundledPreviewSkipsSignatureAndSameVersionContentChecks(t *testing.T) {
	ctx := context.Background()
	runtime, _ := newPackagePipelineRuntime(t)
	archivePath := createPackagePipelineArchive(t, "1.0.0")
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{SpaceID: "user-1", ScopeType: "global", FileName: "bundled.amitiax", TrustedBundled: true}, archive)
	archive.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Installable {
		t.Fatalf("trusted bundled package was rejected: %+v", preview.Issues)
	}
	confirmation, err := runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{SessionID: preview.SessionID, SpaceID: "user-1", ScopeType: "global", Confirmations: map[string]bool{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecutePackageInstall(ctx, PackageInstallRequest{SessionID: preview.SessionID, SpaceID: "user-1", ScopeType: "global", ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "bundled-preview-install"}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	modifiedPath := filepath.Join(t.TempDir(), "bundled-modified.amitiax")
	if err := os.WriteFile(modifiedPath, append(content, 0), 0600); err != nil {
		t.Fatal(err)
	}
	modified, err := os.Open(modifiedPath)
	if err != nil {
		t.Fatal(err)
	}
	modifiedPreview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{SpaceID: "user-1", ScopeType: "global", FileName: "bundled-modified.amitiax", TrustedBundled: true}, modified)
	modified.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !modifiedPreview.Installable {
		t.Fatalf("trusted bundled same-version package was rejected: %+v", modifiedPreview.Issues)
	}
}

func TestProvisionBundledPluginsQuarantinesOptionalPackageFailures(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	root := t.TempDir()
	validPath := createPackagePipelineArchiveWithMetadata(t, "1.0.0", map[string]any{
		"system.defaultInstall": true,
		"system.defaultEnabled": true,
	})
	valid, err := os.ReadFile(validPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "01-valid-package-1.0.0.amitiax"), valid, 0600); err != nil {
		t.Fatal(err)
	}
	brokenPath := filepath.Join(root, "00-broken-package-1.0.0.amitiax")
	if err := os.WriteFile(brokenPath, []byte("not a package"), 0600); err != nil {
		t.Fatal(err)
	}
	issues, err := ProvisionBundledPlugins(context.Background(), runtime, root, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Path != brokenPath {
		t.Fatalf("unexpected bundled plugin issues: %+v", issues)
	}
	if _, err := container.InstallationRepository.GetInstallation(context.Background(), "com.example/pipeline"); err != nil {
		t.Fatalf("valid bundled package was not processed: %v", err)
	}
}

func TestProvisionBundledPluginsUsesOnlyLatestVersionPerPackage(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	root := t.TempDir()
	metadata := map[string]any{"system.defaultInstall": true, "system.defaultEnabled": true}
	for _, version := range []string{"1.0.0", "2.0.0"} {
		archivePath := createPackagePipelineArchiveWithMetadata(t, version, metadata)
		archive, err := os.ReadFile(archivePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "com.example-pipeline-"+version+".amitiax"), archive, 0600); err != nil {
			t.Fatal(err)
		}
	}
	issues, err := ProvisionBundledPlugins(context.Background(), runtime, root, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("unexpected bundled plugin issues: %+v", issues)
	}
	installation, err := container.InstallationRepository.GetInstallation(context.Background(), "com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion.String() != "2.0.0" {
		t.Fatalf("installed version=%s", installation.InstalledVersion.String())
	}
}

func TestTrustedBundledUpdateAddsMandatoryUpdateConfirmation(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	root := t.TempDir()
	metadata := map[string]any{"system.defaultInstall": true}
	firstPath := createPackagePipelineArchiveWithMetadata(t, "1.0.0", metadata)
	first, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "com.example-pipeline-1.0.0.amitiax"), first, 0600); err != nil {
		t.Fatal(err)
	}
	issues, err := ProvisionBundledPlugins(context.Background(), runtime, root, "user-1")
	if err != nil || len(issues) != 0 {
		t.Fatalf("initial bundled install failed: issues=%+v err=%v", issues, err)
	}
	secondPath := createPackagePipelineArchiveWithMetadata(t, "1.1.0", metadata)
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "com.example-pipeline-1.1.0.amitiax"), second, 0600); err != nil {
		t.Fatal(err)
	}
	issues, err = ProvisionBundledPlugins(context.Background(), runtime, root, "user-1")
	if err != nil || len(issues) != 0 {
		t.Fatalf("bundled update failed: issues=%+v err=%v", issues, err)
	}
	installation, err := container.InstallationRepository.GetInstallation(context.Background(), "com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion.String() != "1.1.0" {
		t.Fatalf("installed version=%s", installation.InstalledVersion.String())
	}
}

func TestProvisionBundledPluginsRepairsMissingCurrentGeneration(t *testing.T) {
	runtime, container := newPackagePipelineRuntime(t)
	root := t.TempDir()
	metadata := map[string]any{"system.defaultInstall": true, "system.defaultEnabled": true}
	firstPath := createPackagePipelineArchiveWithMetadata(t, "1.0.0", metadata)
	first, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "com.example-pipeline-1.0.0.amitiax"), first, 0600); err != nil {
		t.Fatal(err)
	}
	issues, err := ProvisionBundledPlugins(context.Background(), runtime, root, "user-1")
	if err != nil || len(issues) != 0 {
		t.Fatalf("initial bundled install failed: issues=%+v err=%v", issues, err)
	}

	currentPath, err := container.PackageGenerationStore.statePath("com.example/pipeline", "current.json")
	if err != nil {
		t.Fatal(err)
	}
	broken := PackageGenerationCurrent{
		ExtensionID:  "com.example/pipeline",
		GenerationID: "generation-missing",
		Version:      "9.9.9",
		ArtifactID:   "artifact-missing",
		TreeHash:     "tree-missing",
		OperationID:  "operation-missing",
		FencingToken: 1,
		UpdatedAt:    time.Now().UTC(),
	}
	data, err := json.Marshal(broken)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, data, 0600); err != nil {
		t.Fatal(err)
	}

	secondPath := createPackagePipelineArchiveWithMetadata(t, "1.1.0", metadata)
	second, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "com.example-pipeline-1.1.0.amitiax"), second, 0600); err != nil {
		t.Fatal(err)
	}
	issues, err = ProvisionBundledPlugins(context.Background(), runtime, root, "user-1")
	if err != nil || len(issues) != 0 {
		t.Fatalf("bundled update after pointer repair failed: issues=%+v err=%v", issues, err)
	}
	installation, err := container.InstallationRepository.GetInstallation(context.Background(), "com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion.String() != "1.1.0" {
		t.Fatalf("installed version=%s", installation.InstalledVersion.String())
	}
	current, err := container.PackageGenerationStore.ReadCurrent("com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if current.Version != "1.1.0" {
		t.Fatalf("current version=%s", current.Version)
	}
}

func TestReconcileBundledPluginReadModelRepairsStaleVersionPointer(t *testing.T) {
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	root := t.TempDir()
	archivePath := createPackagePipelineArchiveWithMetadata(t, "1.0.0", map[string]any{
		"system.defaultInstall": true,
		"system.defaultEnabled": true,
	})
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "com.example-pipeline-1.0.0.amitiax"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	issues, err := ProvisionBundledPlugins(ctx, runtime, root, "user-1")
	if err != nil || len(issues) != 0 {
		t.Fatalf("initial bundled install failed: issues=%+v err=%v", issues, err)
	}
	currentVersion, err := container.PackageRepository.GetCurrentPackageVersion(ctx, "com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	_, err = container.PackageRepository.DB().ExecContext(ctx, `
		INSERT INTO package_versions (
			version_id, extension_id, version, artifact_id, generation_id,
			manifest_hash, content_tree_hash, version_state, retained_until, installed_at, created_at,
			install_operation_id, uninstall_operation_id, uninstalled_at,
			installed_path, installed_tree_hash, archive_hash
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "version-stale", "com.example/pipeline", "9.9.9", "artifact-stale", "generation-stale",
		"manifest-stale", "tree-stale", "retained", "", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano),
		"operation-stale", "", "", "/missing/generation-stale", "tree-stale", "archive-stale")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := container.PackageRepository.DB().ExecContext(ctx, `UPDATE package_versions SET version_state = 'retained' WHERE version_id = ?`, currentVersion.VersionID); err != nil {
		t.Fatal(err)
	}
	if _, err := container.PackageRepository.DB().ExecContext(ctx, `UPDATE package_versions SET version_state = 'current' WHERE version_id = 'version-stale'`); err != nil {
		t.Fatal(err)
	}
	if _, err := container.PackageRepository.DB().ExecContext(ctx, `UPDATE extension_installations SET current_version_id = 'version-stale', current_generation_id = 'generation-stale', current_artifact_id = 'artifact-stale' WHERE extension_id = 'com.example/pipeline'`); err != nil {
		t.Fatal(err)
	}

	if err := reconcileBundledPluginReadModel(ctx, runtime, "com.example/pipeline", "user-1", true); err != nil {
		t.Fatal(err)
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, "com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion.String() != "1.0.0" {
		t.Fatalf("installed version=%s", installation.InstalledVersion.String())
	}
	if installation.Metadata["ownerSpaceId"] != "user-1" || installation.Metadata["scopeType"] != "global" || installation.Metadata["scopeId"] != "" {
		t.Fatalf("reconciled installation scope metadata=%v", installation.Metadata)
	}
	repairedVersion, err := container.PackageRepository.GetCurrentPackageVersion(ctx, "com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if repairedVersion.VersionID != currentVersion.VersionID || repairedVersion.Version != "1.0.0" {
		t.Fatalf("unexpected repaired current version: %+v", repairedVersion)
	}
}

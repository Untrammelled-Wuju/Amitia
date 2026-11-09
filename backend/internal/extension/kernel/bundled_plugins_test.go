package kernel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

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

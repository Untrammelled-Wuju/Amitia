package kernel

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func setupR46ExportRuntime(t *testing.T) (*Runtime, *Container, context.Context, *domain.ExtensionInstallation) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	container, err := NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	runtime, err := NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)

	extensionID := "com.example/r46-export"
	artifactID := "artifact-r46-export"
	installedPath := filepath.Join(root, "r46-export-installed")
	if err := os.MkdirAll(installedPath, 0o755); err != nil {
		t.Fatalf("create installed path: %v", err)
	}

	putR46Artifact(t, ctx, container, PackageArtifact{
		ArtifactID: artifactID, ExtensionID: extensionID, Version: "1.0.0", RetentionState: "active",
	})

	versionRecord := PackageVersionRecord{
		VersionID:          "version-r46-export",
		ExtensionID:        extensionID,
		Version:            "1.0.0",
		ArtifactID:         artifactID,
		VersionState:       "current",
		GenerationID:       "generation-r46-export",
		InstallOperationID: "install-op-r46-export",
		InstalledPath:      installedPath,
		InstalledTreeHash:  "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		ManifestHash:       "manifest-r46-export",
		ContentTreeHash:    "content-r46-export",
		ArchiveHash:        "archive-r46-export",
	}
	if err := container.PackageRepository.PutPackageVersion(ctx, versionRecord); err != nil {
		t.Fatalf("put version record: %v", err)
	}

	installation := &domain.ExtensionInstallation{
		InstallationID:    "installation-r46-export",
		ExtensionID:       domain.ExtensionID(extensionID),
		InstalledVersion:  domain.SemanticVersion{Major: 1, Minor: 0, Patch: 0},
		PackageID:         artifactID,
		InstallationState: domain.InstallationStateInstalled,
		Generation:        1,
		EnablementState:   domain.EnablementDisabled,
		Metadata: map[string]any{
			"installedPath":     installedPath,
			"artifactId":        artifactID,
			"installedTreeHash": "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"lastOperationId":   "op-r46-export",
			"ownerUserId":       "user-1",
			"scopeType":         "global",
			"scopeId":           "",
		},
	}
	if err := container.InstallationRepository.PutInstallation(ctx, *installation); err != nil {
		t.Fatal(err)
	}

	return runtime, container, ctx, installation
}

func TestR46UninstallPreviewRejectsActiveExportLease(t *testing.T) {
	runtime, container, ctx, _ := setupR46ExportRuntime(t)
	extensionID := "com.example/r46-export"
	artifactID := "artifact-r46-export"

	exportTicket := PackageExportTicket{
		ExportID:    "export-r46",
		UserID:      "user-1",
		ExtensionID: extensionID,
		ArtifactID:  artifactID,
		FileName:    "export.zip",
		MIMEType:    "application/zip",
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := container.PackageRepository.PutExport(ctx, exportTicket); err != nil {
		t.Fatal(err)
	}

	preview, err := runtime.PreviewPackageUninstall(ctx, extensionID, "user-1", "global", "")
	if err == nil {
		t.Fatal("active export lease must reject uninstall preview")
	}

	if !IsPackageErrorCode(err, PackageErrCodeExportRetentionUnsupported) {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = preview
}

func TestR46UninstallPreviewDoesNotDowngradeExportError(t *testing.T) {
	runtime, container, ctx, _ := setupR46ExportRuntime(t)
	extensionID := "com.example/r46-export"
	artifactID := "artifact-r46-export"

	exportTicket := PackageExportTicket{
		ExportID:    "export-r46-downgrade",
		UserID:      "user-1",
		ExtensionID: extensionID,
		ArtifactID:  artifactID,
		FileName:    "export.zip",
		MIMEType:    "application/zip",
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := container.PackageRepository.PutExport(ctx, exportTicket); err != nil {
		t.Fatal(err)
	}

	preview, err := runtime.PreviewPackageUninstall(ctx, extensionID, "user-1", "global", "")
	if err == nil {
		t.Fatal("active export lease must reject uninstall preview")
	}

	if preview.ArtifactPolicy == ArtifactPolicyRetainArtifact {
		t.Fatal("export retention error must not downgrade to retainArtifact")
	}

	_ = err
}

func TestR46UninstallConfirmCannotBeCreatedForExportRetention(t *testing.T) {
	runtime, container, ctx, _ := setupR46ExportRuntime(t)
	extensionID := "com.example/r46-export"

	exportTicket := PackageExportTicket{
		ExportID:    "export-r46-confirm",
		UserID:      "user-1",
		ExtensionID: extensionID,
		ArtifactID:  "artifact-r46-export",
		FileName:    "export.zip",
		MIMEType:    "application/zip",
		ExpiresAt:   time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339Nano),
		CreatedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := container.PackageRepository.PutExport(ctx, exportTicket); err != nil {
		t.Fatal(err)
	}

	_, err := runtime.PreviewPackageUninstall(ctx, extensionID, "user-1", "global", "")
	if err == nil {
		t.Fatal("active export lease must reject uninstall preview, preventing confirm")
	}

	if !IsPackageErrorCode(err, PackageErrCodeExportRetentionUnsupported) {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = runtime
}

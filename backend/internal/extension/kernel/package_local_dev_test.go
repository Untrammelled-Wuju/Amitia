package kernel

import (
	"context"
	"os"
	"testing"
)

func TestLocalTrustedDevelopmentUnsignedPackageLifecycle(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	archivePath := createPackagePipelineArchive(t, "1.0.0")
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{
		SpaceID:            "user-1",
		ScopeType:          "global",
		FileName:           "pipeline.amitiax",
		AllowUnsignedDev:   true,
		AllowUnsignedLocal: true,
	}, archive)
	archive.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Installable || !preview.DevOnly || preview.DeveloperSessionID != localUnsignedDeveloperSessionID {
		t.Fatalf("unexpected local development preview: %+v", preview)
	}
	confirmation, err := runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
		SessionID:     preview.SessionID,
		SpaceID:       "user-1",
		ScopeType:     "global",
		Confirmations: map[string]bool{"confirm.unsigned_dev": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.ExecutePackageInstall(ctx, PackageInstallRequest{
		SessionID:         preview.SessionID,
		SpaceID:           "user-1",
		ScopeType:         "global",
		ConfirmationToken: confirmation.ConfirmationToken,
		IdempotencyKey:    "local-development-install",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExtensionID != preview.ExtensionID {
		t.Fatalf("installed extension = %q, want %q", result.ExtensionID, preview.ExtensionID)
	}
	if _, err := container.InstallationRepository.GetInstallation(ctx, "com.example/pipeline"); err != nil {
		t.Fatal(err)
	}
}

func TestLocalTrustedUnsignedPackageLifecycleWithoutDevelopmentMode(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "false")
	ctx := context.Background()
	runtime, _ := newPackagePipelineRuntime(t)
	archivePath := createPackagePipelineArchive(t, "1.0.0")
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{
		SpaceID:            "user-1",
		ScopeType:          "global",
		FileName:           "pipeline.amitiax",
		AllowUnsignedDev:   true,
		AllowUnsignedLocal: true,
	}, archive)
	archive.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Installable || !preview.DevOnly || preview.DeveloperSessionID != localUnsignedDeveloperSessionID {
		t.Fatalf("unexpected local unsigned preview: %+v", preview)
	}
	confirmation, err := runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{
		SessionID:     preview.SessionID,
		SpaceID:       "user-1",
		ScopeType:     "global",
		Confirmations: map[string]bool{"confirm.unsigned_dev": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := runtime.ExecutePackageInstall(ctx, PackageInstallRequest{
		SessionID:         preview.SessionID,
		SpaceID:           "user-1",
		ScopeType:         "global",
		ConfirmationToken: confirmation.ConfirmationToken,
		IdempotencyKey:    "local-trusted-install-without-development-mode",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExtensionID != preview.ExtensionID {
		t.Fatalf("installed extension = %q, want %q", result.ExtensionID, preview.ExtensionID)
	}
}

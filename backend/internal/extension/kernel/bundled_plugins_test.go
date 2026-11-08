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

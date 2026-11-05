package kernel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

func TestHostPluginPackageManagerPrepareAndInstall(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	path := createPackagePipelineArchive(t, "1.0.0")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	manager := NewHostPluginPackageManager(runtime, runtimeprofile.ProfileLocal)
	preview, err := manager.Prepare(ctx, PrepareHostPluginPackageRequest{
		SpaceID: "user-1", ExtensionID: "com.example/pipeline", Version: "1.0.0",
		PackageURI: (&url.URL{Scheme: "file", Path: path}).String(),
		SHA256:     hex.EncodeToString(sum[:]), FileName: "pipeline.amitiax",
		Source: PackageSourceProvenance{
			CatalogID: "com.example.catalog", SourceType: "community", EntryID: "pipeline",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Installable || preview.ExtensionID != "com.example/pipeline" || preview.Version != "1.0.0" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if preview.Source.CatalogID != "com.example.catalog" || preview.Source.URI == "" || preview.Source.ResolvedURL == "" {
		t.Fatalf("source provenance missing: %+v", preview.Source)
	}
	confirmations := make(map[string]bool, len(preview.RequiredConfirmations))
	for _, item := range preview.RequiredConfirmations {
		confirmations[item] = true
	}
	result, err := manager.Install(ctx, InstallHostPluginPackageRequest{
		SpaceID: "user-1", ExtensionID: preview.ExtensionID, Version: preview.Version,
		PreviewSessionID: preview.SessionID, Confirmations: confirmations, IdempotencyKey: "host-plugin-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExtensionID != preview.ExtensionID || result.OperationID == "" {
		t.Fatalf("unexpected install result: %+v", result)
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, "com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if installation.EnablementState != domain.EnablementEnabled {
		t.Fatalf("installed plugin enablement = %s, want enabled", installation.EnablementState)
	}
}

func TestHostPluginPackageManagerUsesUpdateTransactionForInstalledPlugin(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	manager := NewHostPluginPackageManager(runtime, runtimeprofile.ProfileLocal)
	installVersion := func(version, key string) KernelInstallResult {
		path := createPackagePipelineArchive(t, version)
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		preview, err := manager.Prepare(ctx, PrepareHostPluginPackageRequest{
			SpaceID: "user-1", ExtensionID: "com.example/pipeline", Version: version,
			PackageURI: (&url.URL{Scheme: "file", Path: path}).String(),
			SHA256:     hex.EncodeToString(sum[:]), FileName: "pipeline.amitiax",
		})
		if err != nil {
			t.Fatal(err)
		}
		confirmations := make(map[string]bool, len(preview.RequiredConfirmations))
		for _, item := range preview.RequiredConfirmations {
			confirmations[item] = true
		}
		result, err := manager.Install(ctx, InstallHostPluginPackageRequest{
			SpaceID: "user-1", ExtensionID: preview.ExtensionID, Version: preview.Version,
			PreviewSessionID: preview.SessionID, Confirmations: confirmations, IdempotencyKey: key,
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if result := installVersion("1.0.0", "host-plugin-install"); result.Operation != "install" {
		t.Fatalf("initial operation = %q, want install", result.Operation)
	}
	if err := runtime.Disable(ctx, "com.example/pipeline"); err != nil {
		t.Fatal(err)
	}
	if result := installVersion("2.0.0", "host-plugin-update"); result.Operation != "update" {
		t.Fatalf("installed plugin operation = %q, want update", result.Operation)
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, "com.example/pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if installation.InstalledVersion.String() != "2.0.0" {
		t.Fatalf("installed version = %s, want 2.0.0", installation.InstalledVersion.String())
	}
	if installation.EnablementState != domain.EnablementDisabled {
		t.Fatalf("updated plugin enablement = %s, want disabled", installation.EnablementState)
	}
}

func TestHostPluginPackageManagerRejectsHashMismatch(t *testing.T) {
	runtime, _ := newPackagePipelineRuntime(t)
	path := createPackagePipelineArchive(t, "1.0.0")
	manager := NewHostPluginPackageManager(runtime, runtimeprofile.ProfileLocal)
	_, err := manager.Prepare(context.Background(), PrepareHostPluginPackageRequest{
		SpaceID: "user-1", ExtensionID: "com.example/pipeline", Version: "1.0.0",
		PackageURI: (&url.URL{Scheme: "file", Path: path}).String(),
		SHA256:     "0000000000000000000000000000000000000000000000000000000000000000",
	})
	if err == nil {
		t.Fatal("expected hash mismatch")
	}
}

func TestSanitizePluginSourceURLDropsCredentialsAndQuery(t *testing.T) {
	got := sanitizePluginSourceURL("https://user:secret@plugins.example.com/tools.amitiax?token=private#fragment")
	if got != "https://plugins.example.com/tools.amitiax" {
		t.Fatalf("sanitized url = %q", got)
	}
}

func TestHostPluginPackageManagerFollowsSafeRedirectAndTracksResolvedURL(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/octet-stream")
		_, _ = writer.Write([]byte("plugin"))
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL+"/package.amitiax", http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	manager := NewHostPluginPackageManager(nil, runtimeprofile.ProfileLocal)
	reader, fileName, resolvedURL, err := manager.openPackage(context.Background(), redirect.URL+"/start", "plugin.amitiax")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "plugin" || fileName != "plugin.amitiax" {
		t.Fatalf("unexpected package response: file=%s body=%q", fileName, body)
	}
	if resolvedURL != target.URL+"/package.amitiax" {
		t.Fatalf("resolved URL = %q", resolvedURL)
	}
}

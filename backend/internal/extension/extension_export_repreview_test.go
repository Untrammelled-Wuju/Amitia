package extension

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type exportRepreviewFixture struct {
	service      *ExtensionReadModelService
	runtime      *kernelruntime.Runtime
	container    *kernelruntime.Container
	installation domain.ExtensionInstallation
	artifact     kernelruntime.PackageArtifact
	workspaceID  dev_mode.WorkspaceID
}

func newExportRepreviewFixture(t *testing.T, dependency bool) exportRepreviewFixture {
	t.Helper()
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	ctx := context.Background()
	root := t.TempDir()
	container, err := kernelruntime.NewContainerBuilder().WithDBPath(filepath.Join(root, "kernel.db")).WithExtensionRoot(filepath.Join(root, "extensions")).Build(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Close() })
	runtime, err := kernelruntime.NewRuntime(filepath.Join(root, "extensions"))
	if err != nil {
		t.Fatal(err)
	}
	runtime.SetContainer(container)
	if dependency {
		now := time.Now().UTC()
		if err := container.InstallationRepository.PutInstallation(ctx, domain.ExtensionInstallation{InstallationID: "dependency-installation", ExtensionID: domain.ExtensionID("com.example/dependency"), InstalledVersion: domain.SemanticVersion{Major: 1}, PackageID: "dependency-artifact", InstallationState: domain.InstallationStateInstalled, EnablementState: domain.EnablementEnabled, InstalledAt: now, UpdatedAt: now, Generation: 1}); err != nil {
			t.Fatal(err)
		}
	}
	workspaceID := dev_mode.WorkspaceID("export-repreview")
	if _, err := container.DevModeRegistry.Register(ctx, dev_mode.RegisterWorkspaceInput{WorkspaceID: workspaceID, ExtensionID: dev_mode.ExtensionID("com.example/export"), OwnerUserID: "user-1", PathReference: t.TempDir(), ManifestPath: "manifest.json"}); err != nil {
		t.Fatal(err)
	}
	if err := container.DevModeRegistry.GrantDevTrust(workspaceID); err != nil {
		t.Fatal(err)
	}
	workspace, err := container.DevModeRegistry.Get(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	developerSession, err := container.DevModeSessions.Open(ctx, workspaceID, workspace.ExtensionID, "user-1", "test-device", "go-test", kernelruntime.CurrentPackagePolicyVersion(), true, workspace.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	archivePath := createExportRepreviewArchive(t, dependency)
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackage(ctx, kernelruntime.PackagePreviewRequest{UserID: "user-1", ScopeType: "global", FileName: "export.amitiax", AllowUnsignedDev: true, DeveloperSessionID: developerSession.SessionID}, archive)
	archive.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Installable {
		t.Fatalf("initial preview rejected: %+v", preview)
	}
	artifact, err := container.PackageRepository.GetArtifact(ctx, preview.ArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if err := container.PackageRepository.CancelPreview(ctx, preview.SessionID, "user-1", "global", ""); err != nil {
		t.Fatal(err)
	}
	generationSource := t.TempDir()
	if err := os.WriteFile(filepath.Join(generationSource, "index.js"), []byte("export default {}"), 0o600); err != nil {
		t.Fatal(err)
	}
	prepared, err := container.PackageGenerationStore.PrepareGeneration(ctx, kernelruntime.PackageGenerationPrepareRequest{ExtensionID: preview.ExtensionID, GenerationID: "generation-1", Version: preview.Version, ArtifactID: artifact.ArtifactID, OperationID: "operation-export", SourcePath: generationSource})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err = container.PackageGenerationStore.CommitGeneration(ctx, prepared)
	if err != nil {
		t.Fatal(err)
	}
	if err := container.PackageGenerationStore.SwitchCurrent(preview.ExtensionID, "", prepared.Current); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	installation := domain.ExtensionInstallation{
		InstallationID:    "export-installation",
		ExtensionID:       domain.ExtensionID(preview.ExtensionID),
		InstalledVersion:  domain.SemanticVersion{Major: 1},
		PackageID:         artifact.ArtifactID,
		InstallationState: domain.InstallationStateInstalled,
		EnablementState:   domain.EnablementDisabled,
		InstalledAt:       now,
		UpdatedAt:         now,
		Generation:        1,
		Metadata: map[string]any{
			"artifactId": artifact.ArtifactID, "archiveHash": artifact.ArchiveHash,
			"manifestHash": artifact.ManifestHash, "contentTreeHash": artifact.ContentTreeHash,
			"artifactHash": artifact.ArtifactHash, "installedTreeHash": prepared.Current.TreeHash,
			"generationId": prepared.Current.GenerationID, "devOnly": true,
			"developerSessionId": developerSession.SessionID, "ownerUserId": "user-1",
			"scopeType": "global", "scopeId": "",
		},
	}
	if err := container.InstallationRepository.PutInstallation(ctx, installation); err != nil {
		t.Fatal(err)
	}
	service := NewExtensionReadModelService(runtime, nil)
	return exportRepreviewFixture{service: service, runtime: runtime, container: container, installation: installation, artifact: artifact, workspaceID: workspaceID}
}

func createExportRepreviewArchive(t *testing.T, dependency bool) string {
	t.Helper()
	manifest := map[string]any{
		"manifestVersion": 2,
		"extension":       map[string]any{"id": "com.example/export", "name": map[string]any{"default": "Export"}, "description": map[string]any{"default": "Export re-preview test"}, "version": "1.0.0"},
		"publisher":       map[string]any{"id": "com.example", "displayName": "Example"},
		"compatibility":   map[string]any{},
		"modules":         []any{map[string]any{"id": "main", "name": map[string]any{"default": "Main"}, "type": "javascript", "version": "1.0.0", "runtime": map[string]any{"type": "javascript", "entryPoint": "index.js"}, "contributions": []any{map[string]any{"id": "export-tool", "kind": "tool", "name": map[string]any{"default": "Export Tool"}, "version": "1.0.0"}}}},
		"integrity":       map[string]any{"algorithm": "sha256", "contentTreeHash": ""},
	}
	if dependency {
		manifest["dependencies"] = []any{map[string]any{"type": "extension", "id": "com.example/dependency", "version": "^1.0.0"}}
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"manifest.json": manifestRaw, "modules/main/index.js": []byte("export default {}")}
	entries := make(map[string]amitiax.FileEntry, len(files))
	list := make([]amitiax.FileEntry, 0, len(files))
	for name, content := range files {
		sum := sha256.Sum256(content)
		entry := amitiax.FileEntry{Path: name, Size: int64(len(content)), Hash: hex.EncodeToString(sum[:])}
		entries[name] = entry
		list = append(list, entry)
	}
	filesRaw, err := json.Marshal(amitiax.IntegrityFilesDoc{Algorithm: "sha256", Files: entries, GeneratedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	treeRaw, err := json.Marshal(amitiax.IntegrityTreeDoc{Algorithm: "sha256", TreeHash: amitiax.ComputeTreeHash(list), GeneratedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	files["integrity/files.json"] = filesRaw
	files["integrity/content-tree.json"] = treeRaw
	path := filepath.Join(t.TempDir(), "export.amitiax")
	writeExportRepreviewZip(t, path, files)
	return path
}

func writeExportRepreviewZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	out, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(out)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExportRepreviewRejectsInstallationTruthDrift(t *testing.T) {
	fixture := newExportRepreviewFixture(t, false)
	installation := fixture.installation
	installation.Metadata["manifestHash"] = "sha256:drift"
	if err := fixture.container.InstallationRepository.PutInstallation(context.Background(), installation); err != nil {
		t.Fatal(err)
	}
	_, ok, err := fixture.service.TryExport(context.Background(), "com.example/export", "1.0.0", "user-1", "global", "")
	if err == nil || ok || !strings.Contains(err.Error(), "manifestHash mismatch") {
		t.Fatalf("truth drift must fail closed: ok=%v err=%v", ok, err)
	}
}

func TestExportRepreviewRejectsCurrentTrustRevocation(t *testing.T) {
	fixture := newExportRepreviewFixture(t, false)
	if err := fixture.container.DevModeRegistry.RevokeDevTrust(fixture.workspaceID); err != nil {
		t.Fatal(err)
	}
	_, ok, err := fixture.service.TryExport(context.Background(), "com.example/export", "1.0.0", "user-1", "global", "")
	if err == nil || ok {
		t.Fatalf("revoked current trust must block export: ok=%v err=%v", ok, err)
	}
}

func TestExportRepreviewRejectsDependencyDrift(t *testing.T) {
	fixture := newExportRepreviewFixture(t, true)
	if err := fixture.container.InstallationRepository.DeleteInstallation(context.Background(), domain.ExtensionID("com.example/dependency")); err != nil {
		t.Fatal(err)
	}
	_, ok, err := fixture.service.TryExport(context.Background(), "com.example/export", "1.0.0", "user-1", "global", "")
	if err == nil || ok || !strings.Contains(err.Error(), "canonical export preview rejected") {
		t.Fatalf("dependency drift must block export: ok=%v err=%v", ok, err)
	}
}

func TestExportRepreviewBlocksUnsafeArchiveEntry(t *testing.T) {
	fixture := newExportRepreviewFixture(t, false)
	path := filepath.Join(t.TempDir(), "unsafe.amitiax")
	writeExportRepreviewZip(t, path, map[string][]byte{"../escape": []byte("blocked")})
	artifact := fixture.artifact
	artifact.ArchivePath = path
	if _, err := fixture.service.canonicalExportPreview(context.Background(), fixture.installation, artifact, "user-1", "global", ""); err == nil {
		t.Fatal("unsafe archive entry must be blocked")
	}
}

func TestExportRepreviewPreservesCanonicalIdentityAndLease(t *testing.T) {
	fixture := newExportRepreviewFixture(t, false)
	exported, ok, err := fixture.service.TryExport(context.Background(), "com.example/export", "1.0.0", "user-1", "global", "")
	if err != nil || !ok {
		t.Fatalf("canonical export failed: ok=%v err=%v", ok, err)
	}
	if exported.Hash != fixture.artifact.ArchiveHash || exported.Version != fixture.artifact.Version || exported.LocalPath != fixture.artifact.ArchivePath {
		t.Fatalf("export identity drifted: %+v artifact=%+v", exported, fixture.artifact)
	}
	ticket, err := fixture.container.PackageRepository.GetExport(context.Background(), exported.ExportID, "user-1", "com.example/export")
	if err != nil {
		t.Fatal(err)
	}
	if ticket.ArtifactID != fixture.artifact.ArtifactID || ticket.ExportID != exported.ExportID {
		t.Fatalf("export lease identity mismatch: %+v", ticket)
	}
}

package kernel

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

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
)

func newPackagePipelineRuntime(t *testing.T) (*Runtime, *Container) {
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
	return runtime, container
}

func createPackagePipelineArchive(t *testing.T, version string) string {
	return createPackagePipelineArchiveWithMigrations(t, version, nil)
}

func createPackagePipelineArchiveWithMigrations(t *testing.T, version string, definitions []migration.MigrationDefinition) string {
	t.Helper()
	extension := map[string]any{"id": "com.example/pipeline", "name": map[string]any{"default": "Pipeline"}, "description": map[string]any{"default": "Pipeline test"}, "version": version}
	if len(definitions) > 0 {
		extension["metadata"] = map[string]any{"migrations": map[string]any{"definitions": definitions}}
	}
	manifest := map[string]any{
		"manifestVersion": 2,
		"extension":       extension,
		"publisher":       map[string]any{"id": "com.example", "displayName": "Example"},
		"compatibility":   map[string]any{},
		"modules":         []any{map[string]any{"id": "main", "name": map[string]any{"default": "Main"}, "type": "javascript", "version": version, "runtime": map[string]any{"type": "javascript", "entryPoint": "index.js"}, "contributions": []any{map[string]any{"id": "pipeline-tool", "kind": "tool", "name": map[string]any{"default": "Pipeline Tool"}, "version": version}}}},
		"integrity":       map[string]any{"algorithm": "sha256", "contentTreeHash": ""},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"manifest.json": manifestRaw, "modules/main/index.js": []byte("export default {}")}
	for _, definition := range definitions {
		files[definition.Entry] = []byte("export default {}")
	}
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
	path := filepath.Join(t.TempDir(), "pipeline.amitiax")
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
		entry, createErr := w.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write(files[name]); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPackagePipelinePreviewInstallIsolationAndIdempotency(t *testing.T) {
	t.Setenv("AMITIA_EXTENSION_DEV_MODE", "true")
	ctx := context.Background()
	runtime, container := newPackagePipelineRuntime(t)
	archivePath := createPackagePipelineArchive(t, "1.0.0")
	productionArchive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	productionPreview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{UserID: "user-1", ScopeType: "global", FileName: "pipeline.amitiax"}, productionArchive)
	productionArchive.Close()
	if err != nil {
		t.Fatal(err)
	}
	if productionPreview.Installable || productionPreview.TrustDecision != "rejected" {
		t.Fatalf("unsigned production package must be rejected: %+v", productionPreview)
	}
	workspaceID := dev_mode.WorkspaceID("pipeline-test")
	_, err = container.DevModeRegistry.Register(ctx, dev_mode.RegisterWorkspaceInput{WorkspaceID: workspaceID, ExtensionID: dev_mode.ExtensionID("com.example/pipeline"), OwnerUserID: "user-1", PathReference: t.TempDir(), ManifestPath: "manifest.json"})
	if err != nil {
		t.Fatal(err)
	}
	if err := container.DevModeRegistry.GrantDevTrust(workspaceID); err != nil {
		t.Fatal(err)
	}
	workspace, err := container.DevModeRegistry.Get(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	developerSession, err := container.DevModeSessions.Open(ctx, workspaceID, dev_mode.ExtensionID("com.example/pipeline"), "user-1", "test-device", "go-test", packagePolicyVersion, true, workspace.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{UserID: "user-1", ScopeType: "global", FileName: "pipeline.amitiax", AllowUnsignedDev: true, DeveloperSessionID: developerSession.SessionID}, archive)
	archive.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Installable || !preview.DevOnly || len(preview.RequiredConfirmations) != 1 || preview.RequiredConfirmations[0] != "confirm.unsigned_dev" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	confirmation, err := runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{SessionID: preview.SessionID, UserID: "user-1", ScopeType: "global", Confirmations: map[string]bool{"confirm.unsigned_dev": true}})
	if err != nil {
		t.Fatal(err)
	}
	request := PackageInstallRequest{SessionID: preview.SessionID, UserID: "user-1", ScopeType: "global", ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "pipeline-install-key"}
	if _, err := runtime.ExecutePackageInstall(ctx, PackageInstallRequest{SessionID: preview.SessionID, UserID: "user-2", ScopeType: "global", ConfirmationToken: confirmation.ConfirmationToken, IdempotencyKey: "pipeline-cross-user-key"}); err == nil {
		t.Fatal("cross-user preview session must be rejected")
	}
	result, err := runtime.ExecutePackageInstall(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := runtime.ExecutePackageInstall(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.OperationID != result.OperationID {
		t.Fatalf("repeated install returned a different operation: %s != %s", repeated.OperationID, result.OperationID)
	}
	installation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(preview.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	if installation.EnablementState != domain.EnablementDisabled || installation.InstallationState != domain.InstallationStateInstalled {
		t.Fatalf("new installation must remain disabled and installed: %+v", installation)
	}
	installation.Metadata["userOverride"] = map[string]any{"theme": "dark"}
	installation.Metadata["apiSecretRef"] = "secret://extensions/pipeline/api"
	if err := container.InstallationRepository.PutInstallation(ctx, installation); err != nil {
		t.Fatal(err)
	}
	sameVersionArchive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	sameVersionPreview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{UserID: "user-1", ScopeType: "global", FileName: "pipeline.amitiax", AllowUnsignedDev: true, DeveloperSessionID: developerSession.SessionID}, sameVersionArchive)
	sameVersionArchive.Close()
	if err != nil {
		t.Fatal(err)
	}
	sameVersionConfirmations := map[string]bool{}
	for _, required := range sameVersionPreview.RequiredConfirmations {
		sameVersionConfirmations[required] = true
	}
	sameVersionConfirmation, err := runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{SessionID: sameVersionPreview.SessionID, UserID: "user-1", ScopeType: "global", Confirmations: sameVersionConfirmations})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecutePackageUpdate(ctx, PackageInstallRequest{SessionID: sameVersionPreview.SessionID, UserID: "user-1", ScopeType: "global", ConfirmationToken: sameVersionConfirmation.ConfirmationToken, ExpectedExtensionID: preview.ExtensionID, IdempotencyKey: "same-version-update-key"}); err == nil || !strings.Contains(err.Error(), "PACKAGE_UPDATE_TARGET_UNCHANGED") {
		t.Fatalf("same-version update must be rejected: %v", err)
	}
	operation, steps, err := container.PackageRepository.GetOperation(ctx, "user-1", result.OperationID)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != "completed" || len(steps) == 0 {
		t.Fatalf("operation journal incomplete: %+v %+v", operation, steps)
	}
	secondPath := createPackagePipelineArchive(t, "1.1.0")
	secondArchive, err := os.Open(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	secondPreview, err := runtime.PreviewPackage(ctx, PackagePreviewRequest{UserID: "user-1", ScopeType: "global", FileName: "pipeline.amitiax", AllowUnsignedDev: true, DeveloperSessionID: developerSession.SessionID}, secondArchive)
	secondArchive.Close()
	if err != nil {
		t.Fatal(err)
	}
	secondConfirmations := make(map[string]bool)
	for _, confirmation := range secondPreview.RequiredConfirmations {
		secondConfirmations[confirmation] = true
	}
	secondConfirmation, err := runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{SessionID: secondPreview.SessionID, UserID: "user-1", ScopeType: "global", Confirmations: secondConfirmations})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecutePackageInstall(ctx, PackageInstallRequest{SessionID: secondPreview.SessionID, UserID: "user-1", ScopeType: "global", ConfirmationToken: secondConfirmation.ConfirmationToken, ExpectedExtensionID: preview.ExtensionID, IdempotencyKey: "second-install-key"}); err == nil || !strings.Contains(err.Error(), "PACKAGE_ALREADY_INSTALLED") {
		t.Fatalf("install must reject an already installed extension: %v", err)
	}
	if err := container.DevModeRegistry.RevokeDevTrust(workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecutePackageUpdate(ctx, PackageInstallRequest{SessionID: secondPreview.SessionID, UserID: "user-1", ScopeType: "global", ConfirmationToken: secondConfirmation.ConfirmationToken, ExpectedExtensionID: preview.ExtensionID, IdempotencyKey: "trust-revoke-update-key"}); err == nil {
		t.Fatal("developer trust revocation must invalidate a confirmed install")
	}
	if err := container.DevModeRegistry.GrantDevTrust(workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecutePackageUpdate(ctx, PackageInstallRequest{SessionID: secondPreview.SessionID, UserID: "user-1", ScopeType: "global", ConfirmationToken: secondConfirmation.ConfirmationToken, ExpectedExtensionID: preview.ExtensionID, IdempotencyKey: "session-revoke-update-key"}); err == nil {
		t.Fatal("revoked developer session must not revive after trust is granted again")
	}
	workspace, err = container.DevModeRegistry.Get(workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	developerSession, err = container.DevModeSessions.Open(ctx, workspaceID, workspace.ExtensionID, "user-1", "test-device", "go-test", packagePolicyVersion, true, workspace.DevTrustVersion)
	if err != nil {
		t.Fatal(err)
	}
	secondArchive, err = os.Open(secondPath)
	if err != nil {
		t.Fatal(err)
	}
	secondPreview, err = runtime.PreviewPackage(ctx, PackagePreviewRequest{UserID: "user-1", ScopeType: "global", FileName: "pipeline.amitiax", AllowUnsignedDev: true, DeveloperSessionID: developerSession.SessionID}, secondArchive)
	secondArchive.Close()
	if err != nil {
		t.Fatal(err)
	}
	secondConfirmations = make(map[string]bool)
	for _, confirmation := range secondPreview.RequiredConfirmations {
		secondConfirmations[confirmation] = true
	}
	secondConfirmation, err = runtime.ConfirmPackagePreview(ctx, PackagePreviewConfirmationRequest{SessionID: secondPreview.SessionID, UserID: "user-1", ScopeType: "global", Confirmations: secondConfirmations})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.ExecutePackageUpdate(ctx, PackageInstallRequest{SessionID: secondPreview.SessionID, UserID: "user-1", ScopeType: "global", ConfirmationToken: secondConfirmation.ConfirmationToken, ExpectedExtensionID: preview.ExtensionID, IdempotencyKey: "final-update-key"}); err != nil {
		t.Fatal(err)
	}
	updatedInstallation, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(preview.ExtensionID))
	if err != nil {
		t.Fatal(err)
	}
	if updatedInstallation.Metadata["apiSecretRef"] != "secret://extensions/pipeline/api" || updatedInstallation.Metadata["userOverride"] == nil {
		t.Fatalf("update must preserve user configuration and secret references: %+v", updatedInstallation.Metadata)
	}
	rolledBack, err := runtime.ExecutePackageRollback(ctx, preview.ExtensionID, "1.0.0", "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.Version != "1.0.0" {
		t.Fatalf("rollback target mismatch: %+v", rolledBack)
	}
	artifact, err := container.PackageRepository.GetArtifactByVersion(ctx, preview.ExtensionID, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	originalArchive, err := os.ReadFile(artifact.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact.ArchivePath, append(originalArchive, byte(0)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.VerifyStoredPackage(ctx, artifact); err == nil {
		t.Fatal("tampered artifact must be rejected before export")
	}
	if err := os.WriteFile(artifact.ArchivePath, originalArchive, 0o600); err != nil {
		t.Fatal(err)
	}
	uninstallPreview, err := runtime.PreviewPackageUninstall(ctx, preview.ExtensionID, "user-1", "global", "")
	if err != nil || !uninstallPreview.Installable {
		t.Fatalf("uninstall preflight failed: %+v %v", uninstallPreview, err)
	}
	uninstallOperation, err := runtime.ExecutePackageUninstall(ctx, preview.ExtensionID, "user-1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	_, uninstallSteps, err := container.PackageRepository.GetOperation(ctx, "user-1", uninstallOperation.OperationID)
	if err != nil || len(uninstallSteps) < 3 {
		t.Fatalf("uninstall journal incomplete: %+v %v", uninstallSteps, err)
	}
	if _, err := container.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(preview.ExtensionID)); err == nil {
		t.Fatal("uninstall must remove installation record")
	}
	gateReport := &FinalGateReport{Metrics: map[string]int64{}, Details: []FinalGateIssue{}, Errors: []string{}}
	NewFinalGateProbe(container).probePackageReleaseGate(ctx, gateReport)
	if len(gateReport.Errors) != 0 {
		t.Fatalf("package final gate query failed: %+v", gateReport.Errors)
	}
	for name, count := range gateReport.Metrics {
		if count != 0 {
			t.Fatalf("package final gate metric %s is %d", name, count)
		}
	}
}

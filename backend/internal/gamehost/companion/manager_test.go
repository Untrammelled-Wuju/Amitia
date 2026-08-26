package artifact

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/gamehost/integration"
)

func TestResolveTargetRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := resolveTarget(root, "../mods/evil.jar"); err == nil {
		t.Fatal("resolveTarget() accepted traversal")
	}
}

func TestExtractZipAtomicRejectsZipSlip(t *testing.T) {
	root := t.TempDir()
	archive := filepath.Join(root, "bad.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractZipAtomic(archive, filepath.Join(root, "target")); err == nil {
		t.Fatal("extractZipAtomic() accepted zip-slip entry")
	}
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); !os.IsNotExist(err) {
		t.Fatalf("zip-slip wrote outside target: err=%v", err)
	}
}

func TestCopyTreeAtomicRejectsSymlinkSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(source, "link")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := copyTreeAtomic(source, filepath.Join(root, "target")); err == nil {
		t.Fatal("copyTreeAtomic() accepted symlink")
	}
}

type artifactTestSource struct {
	plugins []integration.KernelGamePlugin
}

func (s artifactTestSource) ListEnabledGamePlugins(ctx context.Context) ([]integration.KernelGamePlugin, error) {
	return s.plugins, nil
}

type artifactTestGenerationResolver struct {
	generation integration.InstalledGeneration
}

func (r *artifactTestGenerationResolver) ResolveInstalledGeneration(ctx context.Context, extensionID string) (integration.InstalledGeneration, error) {
	return r.generation, nil
}

func TestDeployRequiredToAuthorizedRootsRequiresGrantAndExplicitUpgradeRefresh(t *testing.T) {
	generationRoot := t.TempDir()
	sourcePath := filepath.Join(generationRoot, "artifacts", "required-companion.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("required companion payload"), 0o600); err != nil {
		t.Fatal(err)
	}

	const extensionID = "example/required-artifact"
	source := artifactTestSource{plugins: []integration.KernelGamePlugin{{
		Extension: kerneldomain.ExtensionDefinition{ID: kerneldomain.ExtensionID(extensionID)},
		Contribution: kerneldomain.ContributionDefinition{
			ID:          kerneldomain.ContributionID("required-game-plugin"),
			ExtensionID: kerneldomain.ExtensionID(extensionID),
			Kind:        kerneldomain.ContributionKindGamePlugin,
			Definition: map[string]any{
				"protocolVersion": "amitia-game-host/1",
				"runtimeModuleId": "runtime",
				"network":         map[string]any{"mode": "none"},
				"artifacts": []any{map[string]any{
					"id":       "required-companion",
					"type":     "file",
					"source":   "artifacts/required-companion.txt",
					"target":   "mods/required-companion.txt",
					"required": true,
				}},
			},
		},
	}}}
	generations := &artifactTestGenerationResolver{generation: integration.InstalledGeneration{
		Path: generationRoot, GenerationID: "generation-1", Version: "1.0.0",
	}}
	manager, err := NewArtifactManager(t.TempDir(), source, generations)
	if err != nil {
		t.Fatalf("NewArtifactManager() error = %v", err)
	}

	ctx := context.Background()
	if err := manager.DeployRequiredToAuthorizedRoots(ctx, extensionID); err == nil {
		t.Fatal("required artifact startup gate accepted an extension without an authorized target root")
	}

	targetRoot := t.TempDir()
	grant, err := manager.AuthorizeTargetRootForCompatibility(ctx, extensionID, targetRoot, "1.21.4")
	if err != nil {
		t.Fatalf("AuthorizeTargetRootForCompatibility() error = %v", err)
	}
	if grant.Generation != "generation-1" || grant.CompatibilityVersion != "1.21.4" {
		t.Fatalf("unexpected grant: %+v", grant)
	}
	if err := manager.DeployRequiredToAuthorizedRoots(ctx, extensionID); err != nil {
		t.Fatalf("DeployRequiredToAuthorizedRoots() error = %v", err)
	}
	deployed, err := os.ReadFile(filepath.Join(targetRoot, "mods", "required-companion.txt"))
	if err != nil {
		t.Fatalf("required artifact was not deployed: %v", err)
	}
	if string(deployed) != "required companion payload" {
		t.Fatalf("deployed payload = %q", deployed)
	}

	generations.generation = integration.InstalledGeneration{
		Path: generationRoot, GenerationID: "generation-2", Version: "2.0.0",
	}
	if err := manager.DeployRequiredToAuthorizedRoots(ctx, extensionID); err == nil {
		t.Fatal("stale generation grant was accepted before host lifecycle refresh")
	}
	if err := manager.RefreshAuthorizedTargetRootsForCurrentGeneration(ctx, extensionID); err != nil {
		t.Fatalf("RefreshAuthorizedTargetRootsForCurrentGeneration() error = %v", err)
	}
	grants, err := manager.ListAuthorizedTargetRoots(ctx, extensionID)
	if err != nil {
		t.Fatalf("ListAuthorizedTargetRoots() after refresh error = %v", err)
	}
	if len(grants) != 1 || grants[0].TargetRoot != filepath.Clean(targetRoot) || grants[0].Generation != "generation-2" {
		t.Fatalf("host refresh did not preserve the exact root on the new generation: %+v", grants)
	}
	if grants[0].CompatibilityVersion != "1.21.4" {
		t.Fatalf("host refresh changed compatibility version: %+v", grants[0])
	}
	if err := manager.DeployRequiredToAuthorizedRoots(ctx, extensionID); err != nil {
		t.Fatalf("DeployRequiredToAuthorizedRoots() after host refresh error = %v", err)
	}

	// Rollback is another confirmed package-generation switch. The host must be
	// able to rebind the same exact user-authorized root back to the restored
	// generation; otherwise required artifacts would self-lock runtime recovery.
	generations.generation = integration.InstalledGeneration{
		Path: generationRoot, GenerationID: "generation-rollback", Version: "1.0.0",
	}
	if err := manager.RefreshAuthorizedTargetRootsForCurrentGeneration(ctx, extensionID); err != nil {
		t.Fatalf("RefreshAuthorizedTargetRootsForCurrentGeneration() rollback error = %v", err)
	}
	grants, err = manager.ListAuthorizedTargetRoots(ctx, extensionID)
	if err != nil {
		t.Fatalf("ListAuthorizedTargetRoots() after rollback refresh error = %v", err)
	}
	if len(grants) != 1 || grants[0].TargetRoot != filepath.Clean(targetRoot) || grants[0].Generation != "generation-rollback" {
		t.Fatalf("host rollback refresh did not preserve the exact root: %+v", grants)
	}
	if grants[0].CompatibilityVersion != "1.21.4" {
		t.Fatalf("host rollback refresh changed compatibility version: %+v", grants[0])
	}
}

func TestDeployRequiredToAuthorizedRootsIgnoresRequiredArtifactsForOtherHosts(t *testing.T) {
	const extensionID = "example/cross-platform-artifact"
	source := artifactTestSource{plugins: []integration.KernelGamePlugin{{
		Extension: kerneldomain.ExtensionDefinition{ID: kerneldomain.ExtensionID(extensionID)},
		Contribution: kerneldomain.ContributionDefinition{
			ID:          kerneldomain.ContributionID("cross-platform-game-plugin"),
			ExtensionID: kerneldomain.ExtensionID(extensionID),
			Kind:        kerneldomain.ContributionKindGamePlugin,
			Definition: map[string]any{
				"protocolVersion": "amitia-game-host/1",
				"runtimeModuleId": "runtime",
				"network":         map[string]any{"mode": "none"},
				"artifacts": []any{map[string]any{
					"id":        "other-host-companion",
					"type":      "file",
					"source":    "artifacts/other-host.txt",
					"target":    "mods/other-host.txt",
					"required":  true,
					"platforms": []any{"never-current-host"},
				}},
			},
		},
	}}}
	manager, err := NewArtifactManager(t.TempDir(), source, &artifactTestGenerationResolver{generation: integration.InstalledGeneration{
		Path: t.TempDir(), GenerationID: "generation-1", Version: "1.0.0",
	}})
	if err != nil {
		t.Fatalf("NewArtifactManager() error = %v", err)
	}

	if err := manager.DeployRequiredToAuthorizedRoots(context.Background(), extensionID); err != nil {
		t.Fatalf("required artifact for another host must not block runtime start: %v", err)
	}
}

func TestRevokeAllTargetRootsPreventsGrantResurrectionAfterReinstall(t *testing.T) {
	const extensionID = "example/reinstall-safe"
	generations := &artifactTestGenerationResolver{generation: integration.InstalledGeneration{
		Path: t.TempDir(), GenerationID: "generation-old", Version: "1.0.0",
	}}
	manager, err := NewArtifactManager(t.TempDir(), artifactTestSource{}, generations)
	if err != nil {
		t.Fatalf("NewArtifactManager() error = %v", err)
	}
	ctx := context.Background()
	rootA := t.TempDir()
	rootB := t.TempDir()
	if _, err := manager.AuthorizeTargetRootForCompatibility(ctx, extensionID, rootA, "1.0"); err != nil {
		t.Fatalf("authorize root A: %v", err)
	}
	if _, err := manager.AuthorizeTargetRootForCompatibility(ctx, extensionID, rootB, "2.0"); err != nil {
		t.Fatalf("authorize root B: %v", err)
	}
	if err := manager.RevokeAllTargetRoots(ctx, extensionID); err != nil {
		t.Fatalf("RevokeAllTargetRoots() error = %v", err)
	}

	generations.generation = integration.InstalledGeneration{
		Path: t.TempDir(), GenerationID: "generation-reinstalled", Version: "1.0.0",
	}
	if err := manager.RefreshAuthorizedTargetRootsForCurrentGeneration(ctx, extensionID); err != nil {
		t.Fatalf("refresh after reinstall: %v", err)
	}
	grants, err := manager.ListAuthorizedTargetRoots(ctx, extensionID)
	if err != nil {
		t.Fatalf("ListAuthorizedTargetRoots() error = %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("uninstalled target-root grants were resurrected: %+v", grants)
	}

	data, err := os.ReadFile(manager.targetGrantPath)
	if err != nil {
		t.Fatalf("read persisted target grants: %v", err)
	}
	if strings.Contains(string(data), extensionID) {
		t.Fatalf("persisted target grants still contain uninstalled extension: %s", data)
	}
}

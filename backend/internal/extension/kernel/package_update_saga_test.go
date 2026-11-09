package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

func TestComputePackageUpdateDiffIncludesRemovedState(t *testing.T) {
	oldVersion, _ := domain.ParseVersion("1.0.0")
	oldDefinition := domain.ExtensionDefinition{ID: "com.example/update", Version: oldVersion, Integrity: domain.ExtensionIntegrity{FileHashes: map[string]string{"modules/old.js": "old", "migrations/001.json": "one"}}}
	oldModules := []domain.ModuleDefinition{{ID: "old", ExtensionID: oldDefinition.ID}}
	oldContributions := []domain.ContributionDefinition{{ID: "old-tool", ExtensionID: oldDefinition.ID, ModuleID: "old", RequiredScope: []string{"conversation"}}}
	oldRequirements := []sqlite.PermissionRequirement{{ExtensionID: oldDefinition.ID, PermissionName: "network"}}
	oldResources := []domain.ResourceOwnership{{ResourceID: "com.example/update/old-resource", OwnerID: string(oldDefinition.ID)}}
	target := plugincontract.Manifest{
		SchemaVersion: 1,
		Plugin: plugincontract.PluginMeta{
			ID:      string(oldDefinition.ID),
			Name:    plugincontract.LocalizedText{Default: "Update"},
			Version: "2.0.0",
			Publisher: plugincontract.Publisher{
				ID:          "com.example",
				DisplayName: "Example",
			},
		},
		Compatibility: plugincontract.Compatibility{MinHostVersion: "0.1.0"},
		Modules:       []plugincontract.Module{{ID: "new", Name: plugincontract.LocalizedText{Default: "New"}, Runtime: &plugincontract.RuntimeSpec{Adapter: "javascript"}}},
		Integrity:     plugincontract.Integrity{Algorithm: "sha256", FileHashes: map[string]string{"modules/new.js": "new", "migrations/002.json": "two"}},
	}
	diff := computePackageUpdateDiff(oldDefinition, oldModules, oldContributions, oldRequirements, oldResources, target, PackageArtifact{})
	assertPackageUpdateDiffContains(t, diff.ModulesRemoved, "old")
	assertPackageUpdateDiffContains(t, diff.ContributionsRemoved, "old-tool")
	assertPackageUpdateDiffContains(t, diff.PermissionsRemoved, "network")
	assertPackageUpdateDiffContains(t, diff.ScopesRemoved, "old-tool:conversation")
	assertPackageUpdateDiffContains(t, diff.ResourcesRemoved, "com.example/update/old-resource")
	assertPackageUpdateDiffContains(t, diff.FilesRemoved, "modules/old.js")
	assertPackageUpdateDiffContains(t, diff.MigrationsRemoved, "migrations/001.json")
	assertPackageUpdateDiffContains(t, diff.MigrationsAdded, "migrations/002.json")
}

func TestRetainPackagePermissionGrantsDropsRemovedPermissions(t *testing.T) {
	extensionID := domain.ExtensionID("com.example/update")
	grants := []sqlite.PermissionGrant{{ExtensionID: extensionID, PermissionName: "network", State: "granted"}, {ExtensionID: extensionID, PermissionName: "filesystem", State: "granted"}}
	requirements := []sqlite.PermissionRequirement{{ExtensionID: extensionID, PermissionName: "network"}}
	retained := retainPackagePermissionGrants(grants, requirements)
	if len(retained) != 1 || retained[0].PermissionName != "network" {
		t.Fatalf("unexpected retained grants: %+v", retained)
	}
}

func TestClonePackageMetadataPreservesOverridesAndSecretRefs(t *testing.T) {
	original := map[string]any{"userOverride": map[string]any{"theme": "dark"}, "apiSecretRef": "secret://extensions/update/api"}
	cloned := clonePackageMetadata(original)
	cloned["artifactId"] = "target"
	if cloned["apiSecretRef"] != original["apiSecretRef"] || cloned["userOverride"] == nil {
		t.Fatalf("metadata was not preserved: %+v", cloned)
	}
	if _, exists := original["artifactId"]; exists {
		t.Fatal("metadata clone mutated source")
	}
}

func assertPackageUpdateDiffContains(t *testing.T, values []string, expected string) {
	t.Helper()
	for _, value := range values {
		if value == expected {
			return
		}
	}
	t.Fatalf("missing %s in %+v", expected, values)
}

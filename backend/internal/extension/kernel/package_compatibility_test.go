package kernel

import (
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestPackageCompatibilityUsesManifestRuntimeSourceOfTruth(t *testing.T) {
	service := manifest_v2.ModuleMeta{
		ID:   "native-runtime",
		Type: "service",
		Runtime: &manifest_v2.RuntimeMeta{
			Type:       "service",
			EntryPoint: "bin/plugin",
		},
	}
	if !packageModuleSupported(service, "linux", "1.0.0") {
		t.Fatal("service module/runtime accepted by Manifest v2 must be installable by canonical package preview")
	}

	native := service
	native.Type = "native"
	if !packageModuleSupported(native, "windows", "1.0.0") {
		t.Fatal("native module with service runtime must be installable by canonical package preview")
	}

	unknown := service
	unknown.Runtime = &manifest_v2.RuntimeMeta{Type: "plugin_service"}
	if packageModuleSupported(unknown, "linux", "1.0.0") {
		t.Fatal("unknown runtime type must remain unsupported")
	}
}

func TestPackagePlatformAliases(t *testing.T) {
	cases := []struct {
		values   []string
		platform string
		want     bool
	}{
		{[]string{"macos"}, "darwin", true},
		{[]string{"darwin"}, "macos", true},
		{[]string{"win32"}, "windows", true},
		{[]string{"windows"}, "win32", true},
		{[]string{"linux"}, "linux", true},
		{[]string{"all"}, "darwin", true},
		{[]string{"linux"}, "windows", false},
	}
	for _, tc := range cases {
		if got := packagePlatformMatches(tc.values, tc.platform); got != tc.want {
			t.Fatalf("packagePlatformMatches(%v, %q) = %v, want %v", tc.values, tc.platform, got, tc.want)
		}
	}
}

func TestPackageModuleCompatibilityChecksMinimumHostVersion(t *testing.T) {
	mod := manifest_v2.ModuleMeta{
		ID:   "runtime",
		Type: "service",
		Runtime: &manifest_v2.RuntimeMeta{
			Type: "service",
		},
		Compatibility: &manifest_v2.ModuleCompatibility{MinHostVersion: "2.0.0"},
	}
	if packageModuleSupported(mod, "linux", "1.9.9") {
		t.Fatal("module requiring newer host version was accepted")
	}
	if !packageModuleSupported(mod, "linux", "2.0.0") {
		t.Fatal("module minimum host version should accept equal host version")
	}
}

func TestPackageGamePluginNetworkPreflightMatchesRuntimePermissionBoundary(t *testing.T) {
	_, code, err := packageGamePluginNetworkPolicy(&gameprotocol.PluginNetworkPolicy{
		Mode:           "restricted",
		AllowedDomains: []string{"example.com"},
		AllowedPorts:   []int{443},
	}, nil)
	if err == nil || code != "game_plugin_network_permission_required" {
		t.Fatalf("restricted network without permission = code %q err %v", code, err)
	}

	policy, code, err := packageGamePluginNetworkPolicy(&gameprotocol.PluginNetworkPolicy{Mode: "none"}, nil)
	if err != nil || code != "" {
		t.Fatalf("deny-all network policy should preflight on supported hosts: code %q err %v", code, err)
	}
	if !policy.Enforce || policy.Mode != "none" {
		t.Fatalf("unexpected deny-all network policy: %+v", policy)
	}
}

func TestPackageGamePluginArtifactSourcesMustExistInArchive(t *testing.T) {
	pkg := &amitiax.Package{
		Manifest: manifest_v2.Manifest{Modules: []manifest_v2.ModuleMeta{{
			ID:   "runtime",
			Type: "service",
			Contributions: []manifest_v2.ContributionMeta{{
				ID:   "game-plugin",
				Kind: "game_plugin",
				Spec: map[string]any{
					"protocolVersion": "amitia-game-host/1",
					"runtimeModuleId": "runtime",
					"network":         map[string]any{"mode": "none"},
					"artifacts": []any{
						map[string]any{
							"id": "missing-file", "type": "file",
							"source": "artifacts/missing.bin", "target": "mods/missing.bin",
						},
						map[string]any{
							"id": "present-directory", "type": "directory",
							"source": "artifacts/config", "target": "config/plugin",
						},
					},
				},
			}},
		}}},
		Files: []amitiax.FileEntry{{Path: "artifacts/config/default.json"}},
	}
	preview := InstallPreview{}
	appendGamePluginArtifactPackageIssues(pkg, &preview)

	if len(preview.Issues) != 1 {
		t.Fatalf("artifact source issues = %+v, want exactly one missing source", preview.Issues)
	}
	if issue := preview.Issues[0]; issue.Code != "game_plugin_artifact_source_missing" || issue.Path != "modules[0].contributions[0].spec.artifacts[0].source" {
		t.Fatalf("unexpected artifact source issue: %+v", issue)
	}
}

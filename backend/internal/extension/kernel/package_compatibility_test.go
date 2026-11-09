package kernel

import (
	"context"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func TestPackageCompatibilityUsesManifestRuntimeSourceOfTruth(t *testing.T) {
	service := plugincontract.Module{
		ID: "native-runtime",
		Runtime: &plugincontract.RuntimeSpec{
			Adapter: "service",
			Entry:   "bin/plugin",
		},
	}
	if !packageModuleSupported(service, "linux", "1.0.0") {
		t.Fatal("service module/runtime accepted by Manifest v1 must be installable by canonical package preview")
	}

	native := service
	native.Runtime = &plugincontract.RuntimeSpec{Adapter: "native", Entry: "bin/plugin"}
	if !packageModuleSupported(native, "windows", "1.0.0") {
		t.Fatal("native module with service runtime must be installable by canonical package preview")
	}

	unknown := service
	unknown.Runtime = &plugincontract.RuntimeSpec{Adapter: "plugin_service"}
	if packageModuleSupported(unknown, "linux", "1.0.0") {
		t.Fatal("unknown runtime type must remain unsupported")
	}
}

func TestPackagePlatformUsesCanonicalValues(t *testing.T) {
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
		{[]string{"darwin"}, "darwin", true},
		{[]string{"windows"}, "windows", true},
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
	mod := plugincontract.Module{
		ID: "runtime",
		Runtime: &plugincontract.RuntimeSpec{
			Adapter: "service",
		},
		Compatibility: &plugincontract.ModuleCompatibility{MinHostVersion: "2.0.0"},
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
	if err == nil || code != "gamex_network_permission_required" {
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

func TestPackageGamePluginNetworkPreflightRejectsMissingHostSandboxPrerequisite(t *testing.T) {
	manifest := plugincontract.Manifest{Modules: []plugincontract.Module{{
		ID: "runtime",
		Contributions: []plugincontract.ContributionDescriptor{{
			ID:      "game-plugin",
			Kind:    "gamex",
			Adapter: "game",
			Spec: map[string]any{
				"protocolVersion": "amitia-game-host/1",
				"runtimeModuleId": "runtime",
				"network":         map[string]any{"mode": "none"},
			},
		}},
	}}}
	preview := InstallPreview{}
	appendGamePluginNetworkCompatibilityIssuesWithHostValidator(manifest, &preview, func(policy trusted_service.ServiceNetworkPolicy) error {
		if !policy.Enforce || policy.Mode != "none" {
			t.Fatalf("unexpected canonical policy passed to host validator: %+v", policy)
		}
		return trusted_service.ErrNetworkSandboxUnavailable
	})
	if len(preview.Issues) != 1 {
		t.Fatalf("network prerequisite issues = %+v, want exactly one", preview.Issues)
	}
	issue := preview.Issues[0]
	if issue.Category != PreviewNotInstallable || issue.Code != "gamex_network_sandbox_unavailable" {
		t.Fatalf("unexpected prerequisite issue: %+v", issue)
	}
	if !strings.Contains(issue.Message, trusted_service.ErrNetworkSandboxUnavailable.Error()) {
		t.Fatalf("missing prerequisite failure detail: %+v", issue)
	}
	if issue.Path != "modules[0].contributions[0].spec.network" {
		t.Fatalf("unexpected prerequisite issue path: %q", issue.Path)
	}
}

func TestPackageGamePluginNoneNetworkUsesNodePermissionIsolation(t *testing.T) {
	manifest := plugincontract.Manifest{Modules: []plugincontract.Module{{
		ID: "runtime",
		Runtime: &plugincontract.RuntimeSpec{
			Adapter: "javascript",
			Entry:   "dist/index.js",
		},
		Contributions: []plugincontract.ContributionDescriptor{{
			ID:      "game-plugin",
			Kind:    "gamex",
			Adapter: "game",
			Spec: map[string]any{
				"protocolVersion": "amitia-game-host/1",
				"runtimeModuleId": "runtime",
				"services": []any{
					map[string]any{"id": "runtime-service", "moduleId": "runtime", "kind": "process"},
				},
				"network": map[string]any{"mode": "none"},
			},
		}},
	}}}
	preview := InstallPreview{}
	hostValidatorCalled := false
	appendGamePluginNetworkCompatibilityIssuesWithHostValidator(manifest, &preview, func(policy trusted_service.ServiceNetworkPolicy) error {
		hostValidatorCalled = true
		return trusted_service.ErrNetworkSandboxUnavailable
	})
	if len(preview.Issues) != 0 {
		t.Fatalf("Node permission isolation should not require bubblewrap: %+v", preview.Issues)
	}
	if hostValidatorCalled {
		t.Fatal("bubblewrap prerequisite validator was called for JavaScript deny-all plugin")
	}
}

func TestPackageCompatibilityExecuteRecheckUsesHostSandboxPrerequisites(t *testing.T) {
	pkg := &amitiax.Package{Manifest: plugincontract.Manifest{Modules: []plugincontract.Module{{
		ID: "runtime",
		Contributions: []plugincontract.ContributionDescriptor{{
			ID:      "game-plugin",
			Kind:    "gamex",
			Adapter: "game",
			Spec: map[string]any{
				"protocolVersion": "amitia-game-host/1",
				"runtimeModuleId": "runtime",
				"network":         map[string]any{"mode": "none"},
			},
		}},
	}}}}
	preview := InstallPreview{}
	runtime := &Runtime{}
	runtime.evaluatePackageCompatibilityAndDependenciesWithHostValidator(context.Background(), pkg, &preview, func(policy trusted_service.ServiceNetworkPolicy) error {
		return trusted_service.ErrNetworkSandboxUnavailable
	})
	found := false
	for _, issue := range preview.Issues {
		if issue.Code == "gamex_network_sandbox_unavailable" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("execute-time compatibility recheck did not include host sandbox prerequisite issue: %+v", preview.Issues)
	}
}

func TestPackageGamePluginArtifactSourcesMustExistInArchive(t *testing.T) {
	pkg := &amitiax.Package{
		Manifest: plugincontract.Manifest{Modules: []plugincontract.Module{{
			ID: "runtime",
			Contributions: []plugincontract.ContributionDescriptor{{
				ID:      "game-plugin",
				Kind:    "gamex",
				Adapter: "game",
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
	if issue := preview.Issues[0]; issue.Code != "gamex_artifact_source_missing" || issue.Path != "modules[0].contributions[0].spec.artifacts[0].source" {
		t.Fatalf("unexpected artifact source issue: %+v", issue)
	}
}

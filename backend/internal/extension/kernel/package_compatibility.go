package kernel

import (
	"errors"
	"fmt"
	"path"
	"runtime"
	"strings"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/extension/platformcompat"
	gamehostnetworkpolicy "github.com/u-ai/backend/internal/gamehost/networkpolicy"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func currentPackageHostVersion() string {
	return config.AppVersion()
}

func packagePlatformMatches(values []string, platform string) bool {
	return platformcompat.Supports(values, platform)
}

func packageVersionMatches(actual, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return true
	}
	actualVersion, actualErr := domain.ParseVersion(strings.TrimSpace(actual))
	requiredVersion, requiredErr := domain.ParseVersion(required)
	return actualErr == nil && requiredErr == nil && actualVersion.Compare(requiredVersion) >= 0
}

func packageVersionAtMost(actual, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return true
	}
	actualVersion, actualErr := domain.ParseVersion(strings.TrimSpace(actual))
	requiredVersion, requiredErr := domain.ParseVersion(required)
	return actualErr == nil && requiredErr == nil && actualVersion.Compare(requiredVersion) <= 0
}

func appendPackageHostCompatibilityIssues(manifest plugincontract.Manifest, preview *InstallPreview) {
	if preview == nil {
		return
	}
	platform := runtime.GOOS
	hostVersion := currentPackageHostVersion()
	if !packagePlatformMatches(manifest.Compatibility.Platforms, platform) {
		preview.Issues = append(preview.Issues, PreviewIssue{
			Category: PreviewNotInstallable,
			Code:     "host_platform_unsupported",
			Message:  fmt.Sprintf("extension does not support host platform %s", platform),
			Path:     "compatibility.platforms",
		})
	}
	if !packageVersionMatches(hostVersion, manifest.Compatibility.MinHostVersion) {
		preview.Issues = append(preview.Issues, PreviewIssue{
			Category: PreviewNotInstallable,
			Code:     "host_version_too_old",
			Message:  fmt.Sprintf("host version %s does not match extension version %s", hostVersion, manifest.Compatibility.MinHostVersion),
			Path:     "compatibility.minHostVersion",
		})
	}
	if strings.TrimSpace(manifest.Compatibility.MaxHostVersion) != "" && !packageVersionAtMost(hostVersion, manifest.Compatibility.MaxHostVersion) {
		preview.Issues = append(preview.Issues, PreviewIssue{
			Category: PreviewNotInstallable,
			Code:     "host_version_too_new",
			Message:  fmt.Sprintf("host version %s does not match extension version %s", hostVersion, manifest.Compatibility.MaxHostVersion),
			Path:     "compatibility.maxHostVersion",
		})
	}
}

func packageModuleSupported(mod plugincontract.Module, platform, hostVersion string) bool {
	if !plugincontract.IsSupportedModuleType(mod.EffectiveType()) {
		return false
	}
	if mod.Runtime != nil && strings.TrimSpace(mod.Runtime.Adapter) != "" && !plugincontract.IsSupportedRuntimeAdapter(mod.Runtime.Adapter) {
		return false
	}
	if mod.Compatibility == nil {
		return true
	}
	if !packagePlatformMatches(mod.Compatibility.Platforms, platform) {
		return false
	}
	return packageVersionMatches(hostVersion, mod.Compatibility.MinHostVersion)
}

func packageGamePluginNetworkPolicy(spec *gameprotocol.PluginNetworkPolicy, requiredPermissions []string) (trusted_service.ServiceNetworkPolicy, string, error) {
	policy, err := gamehostnetworkpolicy.Build(spec, requiredPermissions)
	if err == nil {
		return policy, "", nil
	}
	switch {
	case errors.Is(err, gamehostnetworkpolicy.ErrPermissionRequired):
		return trusted_service.ServiceNetworkPolicy{}, "gamex_network_permission_required", err
	case errors.Is(err, gamehostnetworkpolicy.ErrPlatformUnsupported):
		return trusted_service.ServiceNetworkPolicy{}, "gamex_network_platform_unsupported", err
	default:
		return trusted_service.ServiceNetworkPolicy{}, "gamex_network_policy_invalid", err
	}
}

func appendGamePluginNetworkCompatibilityIssues(manifest plugincontract.Manifest, preview *InstallPreview) {
	appendGamePluginNetworkCompatibilityIssuesWithHostValidator(manifest, preview, trusted_service.ValidateNetworkSandboxPrerequisites)
}

func appendGamePluginNetworkCompatibilityIssuesWithHostValidator(manifest plugincontract.Manifest, preview *InstallPreview, validateHost func(trusted_service.ServiceNetworkPolicy) error) {
	if preview == nil {
		return
	}
	devMode := packageDevelopmentModeEnabled()
	for moduleIndex, mod := range manifest.Modules {
		for contributionIndex, contribution := range mod.Contributions {
			if contribution.EffectiveAdapter() != domain.ContributionAdapterGame {
				continue
			}
			spec, err := gameprotocol.ParsePluginHostSpec(contribution.Spec)
			if err != nil {
				continue // Manifest validation owns malformed contribution specs.
			}
			policy, code, policyErr := packageGamePluginNetworkPolicy(spec.Network, contribution.RequiredPermissions)
			if policyErr == nil && validateHost != nil && !devMode {
				var hostErr error
				if gamePluginUsesNodePermissionIsolation(manifest, spec) && strings.EqualFold(strings.TrimSpace(policy.Mode), "none") {
					hostErr = trusted_service.ValidateNetworkPolicySupport(policy)
				} else {
					hostErr = validateHost(policy)
				}
				if hostErr != nil {
					code = "gamex_network_sandbox_unavailable"
					policyErr = fmt.Errorf("game plugin network sandbox prerequisites are unavailable on this host: %w", hostErr)
				}
			}
			if policyErr == nil {
				continue
			}
			preview.Issues = append(preview.Issues, PreviewIssue{
				Category: PreviewNotInstallable,
				Code:     code,
				Message:  policyErr.Error(),
				Path:     fmt.Sprintf("modules[%d].contributions[%d].spec.network", moduleIndex, contributionIndex),
			})
		}
	}
}

func gamePluginUsesNodePermissionIsolation(manifest plugincontract.Manifest, spec gameprotocol.PluginHostSpec) bool {
	if len(spec.Services) == 0 {
		return false
	}
	modules := make(map[string]plugincontract.Module, len(manifest.Modules))
	for _, module := range manifest.Modules {
		modules[module.ID] = module
	}
	for _, service := range spec.Services {
		moduleID := strings.TrimSpace(service.ModuleID)
		if moduleID == "" {
			moduleID = strings.TrimSpace(service.ID)
		}
		module, ok := modules[moduleID]
		if !ok || module.Runtime == nil || !strings.EqualFold(strings.TrimSpace(module.Runtime.Adapter), "javascript") {
			return false
		}
	}
	return true
}

func appendGamePluginArtifactPackageIssues(pkg *amitiax.Package, preview *InstallPreview) {
	if pkg == nil || preview == nil {
		return
	}

	files := make(map[string]bool, len(pkg.Files))
	directories := make(map[string]bool, len(pkg.Files))
	for _, file := range pkg.Files {
		normalized := normalizePackageEntryPath(file.Path)
		if normalized == "" {
			continue
		}
		if file.IsDir {
			directories[strings.TrimSuffix(normalized, "/")] = true
			continue
		}
		files[normalized] = true
	}

	for moduleIndex, mod := range pkg.Manifest.Modules {
		for contributionIndex, contribution := range mod.Contributions {
			if contribution.EffectiveAdapter() != domain.ContributionAdapterGame {
				continue
			}
			spec, err := gameprotocol.ParsePluginHostSpec(contribution.Spec)
			if err != nil || spec.Validate() != nil {
				continue // Manifest/game-plugin validation owns malformed contribution specs.
			}
			for artifactIndex, artifact := range spec.Artifacts {
				source := normalizePackageEntryPath(artifact.Source)
				if source == "" || packageArtifactSourceExists(source, artifact.Type, files, directories) {
					continue
				}
				preview.Issues = append(preview.Issues, PreviewIssue{
					Category: PreviewNotInstallable,
					Code:     "gamex_artifact_source_missing",
					Message:  fmt.Sprintf("game plugin artifact %q source %q is missing from the package", artifact.ID, artifact.Source),
					Path:     fmt.Sprintf("modules[%d].contributions[%d].spec.artifacts[%d].source", moduleIndex, contributionIndex, artifactIndex),
				})
			}
		}
	}
}

func normalizePackageEntryPath(value string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	normalized = strings.TrimPrefix(normalized, "./")
	if normalized == "" {
		return ""
	}
	return strings.TrimPrefix(path.Clean(normalized), "/")
}

func packageArtifactSourceExists(source, artifactType string, files, directories map[string]bool) bool {
	source = strings.TrimSuffix(source, "/")
	if source == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(artifactType)) {
	case gameprotocol.PluginArtifactTypeDirectory:
		if directories[source] {
			return true
		}
		prefix := source + "/"
		for file := range files {
			if strings.HasPrefix(file, prefix) {
				return true
			}
		}
		return false
	default:
		return files[source]
	}
}

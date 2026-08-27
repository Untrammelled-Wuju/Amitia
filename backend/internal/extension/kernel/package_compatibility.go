package kernel

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	gamehostnetworkpolicy "github.com/u-ai/backend/internal/gamehost/networkpolicy"
	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
)

func currentPackageHostVersion() string {
	if value := strings.TrimSpace(os.Getenv("AMITIA_VERSION")); value != "" {
		return value
	}
	// The production extension runtime is currently constructed with 1.0.0.
	// Release builds can override the compatibility identity via AMITIA_VERSION.
	return "1.0.0"
}

func normalizePackagePlatform(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "windows", "win32", "win64":
		return "windows"
	case "darwin", "macos", "osx", "mac":
		return "darwin"
	case "linux":
		return "linux"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func packagePlatformMatches(values []string, platform string) bool {
	if len(values) == 0 {
		return true
	}
	target := normalizePackagePlatform(platform)
	for _, value := range values {
		normalized := normalizePackagePlatform(value)
		if normalized == "all" || normalized == "*" || normalized == target {
			return true
		}
	}
	return false
}

func packageVersionAtLeast(actual, minimum string) bool {
	minimum = strings.TrimSpace(minimum)
	if minimum == "" {
		return true
	}
	actualVersion, actualErr := domain.ParseVersion(strings.TrimSpace(actual))
	minimumVersion, minimumErr := domain.ParseVersion(minimum)
	return actualErr == nil && minimumErr == nil && actualVersion.Compare(minimumVersion) >= 0
}

func packageVersionAtMost(actual, maximum string) bool {
	maximum = strings.TrimSpace(maximum)
	if maximum == "" {
		return true
	}
	actualVersion, actualErr := domain.ParseVersion(strings.TrimSpace(actual))
	maximumVersion, maximumErr := domain.ParseVersion(maximum)
	return actualErr == nil && maximumErr == nil && actualVersion.Compare(maximumVersion) <= 0
}

func appendPackageHostCompatibilityIssues(manifest manifest_v2.Manifest, preview *InstallPreview) {
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
	if !packageVersionAtLeast(hostVersion, manifest.Compatibility.MinHostVersion) {
		preview.Issues = append(preview.Issues, PreviewIssue{
			Category: PreviewNotInstallable,
			Code:     "host_version_too_old",
			Message:  fmt.Sprintf("host version %s is below extension minimum %s", hostVersion, manifest.Compatibility.MinHostVersion),
			Path:     "compatibility.minHostVersion",
		})
	}
	if !packageVersionAtMost(hostVersion, manifest.Compatibility.MaxHostVersion) {
		preview.Issues = append(preview.Issues, PreviewIssue{
			Category: PreviewNotInstallable,
			Code:     "host_version_too_new",
			Message:  fmt.Sprintf("host version %s is above extension maximum %s", hostVersion, manifest.Compatibility.MaxHostVersion),
			Path:     "compatibility.maxHostVersion",
		})
	}
}

func packageModuleSupported(mod manifest_v2.ModuleMeta, platform, hostVersion string) bool {
	if !manifest_v2.IsSupportedModuleType(mod.Type) {
		return false
	}
	if mod.Runtime != nil && strings.TrimSpace(mod.Runtime.Type) != "" && !manifest_v2.IsSupportedRuntimeType(mod.Runtime.Type) {
		return false
	}
	if mod.Compatibility == nil {
		return true
	}
	if !packagePlatformMatches(mod.Compatibility.Platforms, platform) {
		return false
	}
	return packageVersionAtLeast(hostVersion, mod.Compatibility.MinHostVersion)
}

func packageGamePluginNetworkPolicy(spec *gameprotocol.PluginNetworkPolicy, requiredPermissions []string) (trusted_service.ServiceNetworkPolicy, string, error) {
	policy, err := gamehostnetworkpolicy.Build(spec, requiredPermissions)
	if err == nil {
		return policy, "", nil
	}
	switch {
	case errors.Is(err, gamehostnetworkpolicy.ErrPermissionRequired):
		return trusted_service.ServiceNetworkPolicy{}, "game_plugin_network_permission_required", err
	case errors.Is(err, gamehostnetworkpolicy.ErrPlatformUnsupported):
		return trusted_service.ServiceNetworkPolicy{}, "game_plugin_network_platform_unsupported", err
	default:
		return trusted_service.ServiceNetworkPolicy{}, "game_plugin_network_policy_invalid", err
	}
}

func appendGamePluginNetworkCompatibilityIssues(manifest manifest_v2.Manifest, preview *InstallPreview) {
	appendGamePluginNetworkCompatibilityIssuesWithHostValidator(manifest, preview, trusted_service.ValidateNetworkSandboxPrerequisites)
}

func appendGamePluginNetworkCompatibilityIssuesWithHostValidator(manifest manifest_v2.Manifest, preview *InstallPreview, validateHost func(trusted_service.ServiceNetworkPolicy) error) {
	if preview == nil {
		return
	}
	for moduleIndex, mod := range manifest.Modules {
		for contributionIndex, contribution := range mod.Contributions {
			if strings.TrimSpace(string(contribution.Kind)) != "game_plugin" {
				continue
			}
			spec, err := gameprotocol.ParsePluginHostSpec(contribution.Spec)
			if err != nil {
				continue // Manifest validation owns malformed contribution specs.
			}
			policy, code, policyErr := packageGamePluginNetworkPolicy(spec.Network, contribution.RequiredPermissions)
			if policyErr == nil && validateHost != nil {
				if hostErr := validateHost(policy); hostErr != nil {
					code = "game_plugin_network_sandbox_unavailable"
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
			if strings.TrimSpace(string(contribution.Kind)) != "game_plugin" {
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
					Code:     "game_plugin_artifact_source_missing",
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

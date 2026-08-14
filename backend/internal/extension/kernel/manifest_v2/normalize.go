package manifest_v2

import (
	"fmt"
	"regexp"
	"strings"
)

var semVerPattern = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-[0-9A-Za-z-.]+)?(?:\+[0-9A-Za-z-.]+)?$`)

var platformAliases = map[string]string{
	"macos":   "darwin",
	"mac":     "darwin",
	"win":     "windows",
	"win32":   "windows",
	"win64":   "windows",
	"linux":   "linux",
	"android": "android",
	"ios":     "ios",
}

var knownPlatforms = map[string]bool{
	"darwin":  true,
	"windows": true,
	"linux":   true,
	"android": true,
	"ios":     true,
}

var architectureAliases = map[string]string{
	"x86_64":    "amd64",
	"x64":       "amd64",
	"amd64":     "amd64",
	"aarch64":   "arm64",
	"arm64-v8a": "arm64",
	"arm64":     "arm64",
	"armv7":     "arm",
	"arm":       "arm",
	"386":       "386",
	"amd32":     "386",
}

var knownArchitectures = map[string]bool{
	"amd64": true,
	"arm64": true,
	"arm":   true,
	"386":   true,
}

var capabilityIDPattern = regexp.MustCompile(`^[a-z0-9/._-]{1,256}$`)
var providerIDPattern = regexp.MustCompile(`^[a-z0-9/._-]{1,256}$`)
var labelKeyPattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}$`)
var requiredFeaturePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`)

const maxMetadataBytes = 1 << 20
const maxProvidedCapabilities = 256
const maxProviderLabels = 64

func normalizeStringList(values []string, lower bool) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if lower {
			v = strings.ToLower(v)
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		result = append(result, v)
	}
	return result
}

func normalizePlatform(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if alias, ok := platformAliases[v]; ok {
		return alias
	}
	return v
}

func normalizeArchitecture(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if alias, ok := architectureAliases[v]; ok {
		return alias
	}
	return v
}

func isExecutableModule(mod ModuleMeta) bool {
	switch mod.Type {
	case "builtin", "javascript", "wasm", "native", "service":
		return true
	case "data_only":
		return false
	}
	return false
}

func (m Manifest) NormalizeCompatibility() (Manifest, ValidationReport) {
	report := ValidationReport{}
	clone := m

	if m.Placement == "" {
		report.AddWarningCode("", "placement_implicit", "extension placement not declared, will be inferred from modules")
	}

	hasCloudModule := false
	hasDeviceModule := false
	for i := range clone.Modules {
		mod := &clone.Modules[i]
		if !isExecutableModule(*mod) {
			continue
		}
		if mod.Placement == "" {
			mod.Placement = "cloud"
			report.AddWarningCode(
				fmt.Sprintf("modules[%d].placement", i),
				"module_placement_defaulted_to_cloud",
				fmt.Sprintf("executable module %q placement defaulted to cloud", mod.ID),
			)
		}
		if mod.Placement == "cloud" {
			hasCloudModule = true
		}
		if mod.Placement == "device" {
			hasDeviceModule = true
		}
	}

	if clone.Placement == "" {
		switch {
		case hasCloudModule && hasDeviceModule:
			clone.Placement = "hybrid"
		case hasDeviceModule:
			clone.Placement = "device"
		default:
			clone.Placement = "cloud"
		}
		report.AddWarningCode("", "placement_defaulted_to_cloud",
			fmt.Sprintf("extension placement defaulted to %q", clone.Placement))
	}

	if clone.Placement == "hybrid" {
		for i := range clone.Modules {
			mod := &clone.Modules[i]
			if !isExecutableModule(*mod) {
				continue
			}
			if mod.Placement == "" {
				report.AddError(
					fmt.Sprintf("modules[%d].placement", i),
					"hybrid_module_placement_required",
					fmt.Sprintf("hybrid extension module %q must declare placement", mod.ID),
				)
			}
		}
	}

	for i := range clone.Modules {
		mod := &clone.Modules[i]
		if mod.Runtime != nil && len(mod.Runtime.Capabilities) > 0 && len(mod.ProvidedCapabilities) == 0 {
			for capName, enabled := range mod.Runtime.Capabilities {
				if !enabled {
					continue
				}
				mod.ProvidedCapabilities = append(mod.ProvidedCapabilities, ProvidedCapabilityMeta{
					ID:      capName,
					Version: mod.Version,
				})
			}
			report.AddWarningCode(
				fmt.Sprintf("modules[%d].runtime.capabilities", i),
				"legacy_runtime_capabilities_mapped",
				"runtime.capabilities=true mapped to providedCapabilities",
			)
		} else if mod.Runtime != nil && len(mod.Runtime.Capabilities) > 0 && len(mod.ProvidedCapabilities) > 0 {
			report.AddWarningCode(
				fmt.Sprintf("modules[%d].runtime.capabilities", i),
				"legacy_runtime_capabilities_ignored_for_provider_declaration",
				"runtime.capabilities ignored because providedCapabilities already declared",
			)
		}

		if mod.Provider != nil && len(mod.Contributions) > 0 {
			hasProviderContribution := false
			for _, c := range mod.Contributions {
				if c.Kind == "provider" {
					hasProviderContribution = true
					break
				}
			}
			if hasProviderContribution {
				report.AddWarningCode(
					fmt.Sprintf("modules[%d].provider", i),
					"legacy_provider_contribution_present",
					"provider contribution kind is deprecated in favor of provider metadata",
				)
			}
		}

		if mod.Compatibility != nil && mod.DeviceRequirements != nil {
			report.AddWarningCode(
				fmt.Sprintf("modules[%d]", i),
				"module_compatibility_and_device_requirements_both_declared",
				"both compatibility and deviceRequirements declared; deviceRequirements takes precedence for device modules",
			)
		}
	}

	return clone, report
}

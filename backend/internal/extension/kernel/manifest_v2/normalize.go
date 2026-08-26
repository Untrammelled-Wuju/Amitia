package manifest_v2

import (
	"fmt"
	"regexp"
	"strings"

	gameprotocol "github.com/u-ai/backend/pkg/gameplugin/protocol"
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

func normalizeContributionKind(kind string) string {
	switch kind {
	case "background_service":
		return "background_task"
	default:
		return kind
	}
}

func normalizeProviderContributions(modIdx int, mod *ModuleMeta, report *ValidationReport) bool {
	var providerContribs []int
	for idx, c := range mod.Contributions {
		if c.Kind == "provider" {
			providerContribs = append(providerContribs, idx)
		}
	}
	if len(providerContribs) == 0 {
		return true
	}

	canonicalID := ""
	canonicalPriority := 0
	canonicalLabels := map[string]string{}
	canonicalMetadata := map[string]any{}
	var capabilityIDs []string
	conflict := false

	for _, idx := range providerContribs {
		c := mod.Contributions[idx]
		spec := c.Spec

		provID := ""
		if spec != nil {
			if v, ok := spec["id"].(string); ok {
				provID = strings.ToLower(strings.TrimSpace(v))
			}
		}
		if provID == "" {
			provID = strings.ToLower(strings.TrimSpace(c.ID))
		}
		if provID != "" {
			if canonicalID == "" {
				canonicalID = provID
			} else if canonicalID != provID {
				report.AddWarningCode(
					fmt.Sprintf("modules[%d].contributions[%d].kind", modIdx, idx),
					"provider_contribution_id_conflict",
					fmt.Sprintf("multiple provider contributions with different ids: %q vs %q", canonicalID, provID),
				)
				conflict = true
			}
		}

		provPriority := 0
		if spec != nil {
			if v, ok := spec["priority"].(float64); ok {
				provPriority = int(v)
			}
		}
		if canonicalPriority == 0 && provPriority != 0 {
			canonicalPriority = provPriority
		}

		if spec != nil {
			if labels, ok := spec["labels"].(map[string]any); ok {
				for k, v := range labels {
					if vs, ok := v.(string); ok {
						canonicalLabels[k] = vs
					}
				}
			}
		}

		if spec != nil {
			if caps, ok := spec["capabilities"].([]any); ok {
				for _, cap := range caps {
					if capID, ok := cap.(string); ok {
						capabilityIDs = append(capabilityIDs, strings.ToLower(strings.TrimSpace(capID)))
					} else if capMap, ok := cap.(map[string]any); ok {
						if v, ok := capMap["id"].(string); ok {
							capabilityIDs = append(capabilityIDs, strings.ToLower(strings.TrimSpace(v)))
						}
					}
				}
			}
		}
	}

	if conflict {
		return false
	}

	if mod.Provider != nil && mod.Provider.ID != "" && canonicalID != "" && mod.Provider.ID != canonicalID {
		report.AddWarningCode(
			fmt.Sprintf("modules[%d].provider", modIdx),
			"provider_contribution_conflicts_with_existing",
			fmt.Sprintf("provider contribution id %q conflicts with existing module.provider.id %q", canonicalID, mod.Provider.ID),
		)
		return false
	}

	if canonicalID != "" {
		if mod.Provider == nil {
			mod.Provider = &ProviderMetadataMeta{ID: canonicalID}
		} else if mod.Provider.ID == "" {
			mod.Provider.ID = canonicalID
		}
	}
	if canonicalPriority != 0 && mod.Provider != nil {
		mod.Provider.Priority = canonicalPriority
	}
	if len(canonicalLabels) > 0 && mod.Provider != nil {
		if mod.Provider.Labels == nil {
			mod.Provider.Labels = map[string]string{}
		}
		for k, v := range canonicalLabels {
			mod.Provider.Labels[k] = v
		}
	}
	if len(canonicalMetadata) > 0 && mod.Provider != nil {
		if mod.Provider.Metadata == nil {
			mod.Provider.Metadata = map[string]any{}
		}
		for k, v := range canonicalMetadata {
			mod.Provider.Metadata[k] = v
		}
	}

	seenCaps := map[string]bool{}
	for _, pc := range mod.ProvidedCapabilities {
		seenCaps[strings.ToLower(strings.TrimSpace(pc.ID))] = true
	}
	for _, capID := range capabilityIDs {
		if !seenCaps[capID] {
			mod.ProvidedCapabilities = append(mod.ProvidedCapabilities, ProvidedCapabilityMeta{ID: capID})
			seenCaps[capID] = true
		}
	}

	newContribs := make([]ContributionMeta, 0, len(mod.Contributions))
	for idx, c := range mod.Contributions {
		isProvider := false
		for _, pi := range providerContribs {
			if idx == pi {
				isProvider = true
				break
			}
		}
		if !isProvider {
			newContribs = append(newContribs, c)
		}
	}
	mod.Contributions = newContribs

	report.AddWarningCode(
		fmt.Sprintf("modules[%d]", modIdx),
		"provider_contribution_normalized",
		"legacy provider contribution normalized to module.provider + providedCapabilities",
	)

	return true
}

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
	return IsExecutableModuleType(mod.Type)
}

// gamePluginRuntimeModuleIDs discovers executable modules owned by game_plugin
// contributions before placement normalization. The contribution may live in a
// different module and may reference a module declared later in the manifest,
// so placement inference must be order-independent. Invalid specs are ignored
// here and reported by the normal validation pass.
func gamePluginRuntimeModuleIDs(m Manifest) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, mod := range m.Modules {
		for _, contribution := range mod.Contributions {
			if normalizeContributionKind(contribution.Kind) != "game_plugin" {
				continue
			}
			spec, err := gameprotocol.ParsePluginHostSpec(contribution.Spec)
			if err != nil {
				continue
			}
			if moduleID := strings.TrimSpace(spec.RuntimeModuleID); moduleID != "" {
				ids[moduleID] = struct{}{}
			}
			for _, service := range spec.Services {
				if moduleID := strings.TrimSpace(service.ModuleID); moduleID != "" {
					ids[moduleID] = struct{}{}
				}
			}
		}
	}
	return ids
}

func (m Manifest) NormalizeCompatibility() (Manifest, ValidationReport) {
	report := ValidationReport{}
	clone := m

	if m.Placement == "" {
		report.AddWarningCode("", "placement_implicit", "extension placement not declared, will be inferred from modules")
	}

	gameRuntimeModules := gamePluginRuntimeModuleIDs(clone)
	hasCloudModule := false
	hasDeviceModule := false
	for i := range clone.Modules {
		mod := &clone.Modules[i]
		if !isExecutableModule(*mod) {
			continue
		}
		if mod.Placement == "" {
			if _, gameRuntime := gameRuntimeModules[mod.ID]; gameRuntime {
				mod.Placement = "device"
				report.AddWarningCode(
					fmt.Sprintf("modules[%d].placement", i),
					"game_plugin_runtime_placement_defaulted_to_device",
					fmt.Sprintf("game plugin runtime module %q placement defaulted to device", mod.ID),
				)
			} else {
				mod.Placement = "cloud"
				report.AddWarningCode(
					fmt.Sprintf("modules[%d].placement", i),
					"module_placement_defaulted_to_cloud",
					fmt.Sprintf("executable module %q placement defaulted to cloud", mod.ID),
				)
			}
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
			for _, c := range mod.Contributions {
				if c.Kind == "provider" {
					report.AddWarningCode(
						fmt.Sprintf("modules[%d].provider", i),
						"provider_contribution_deprecated",
						"provider contribution kind is deprecated in favor of module.provider + providedCapabilities",
					)
					break
				}
			}
		}

		if normalized := normalizeProviderContributions(i, mod, &report); !normalized {
			report.AddError(
				fmt.Sprintf("modules[%d].contributions", i),
				"provider_contribution_deprecated_invalid",
				"legacy provider contribution cannot be unambiguously mapped to module.provider + providedCapabilities",
			)
		}

		for j := range mod.Contributions {
			normalizedKind := normalizeContributionKind(mod.Contributions[j].Kind)
			if normalizedKind != mod.Contributions[j].Kind {
				oldKind := mod.Contributions[j].Kind
				mod.Contributions[j].Kind = normalizedKind
				report.AddWarningCode(
					fmt.Sprintf("modules[%d].contributions[%d].kind", i, j),
					"contribution_kind_canonicalized",
					fmt.Sprintf("contribution kind %q canonicalized to %q", oldKind, normalizedKind),
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

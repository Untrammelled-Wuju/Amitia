package manifest_v2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

func validateExtensionPlacement(placement string, report *ValidationReport, path string) {
	if placement == "" {
		return
	}
	if !domain.IsKnownExtensionPlacement(domain.ExtensionPlacement(placement)) {
		report.AddError(path, "invalid_extension_placement",
			fmt.Sprintf("unknown extension placement: %q (expected cloud, device, or hybrid)", placement))
	}
}

func validateModulePlacement(placement string, report *ValidationReport, path string) {
	if placement == "" {
		return
	}
	if !domain.IsKnownModulePlacement(domain.ModulePlacement(placement)) {
		report.AddError(path, "invalid_module_placement",
			fmt.Sprintf("unknown module placement: %q (expected cloud or device)", placement))
	}
}

func checkExtensionModulePlacementConsistency(m Manifest, report *ValidationReport) {
	extPlacement := m.Placement
	if extPlacement == "" {
		return
	}

	hasCloud := false
	hasDevice := false
	for i, mod := range m.Modules {
		if !isExecutableModule(mod) {
			continue
		}
		modPath := fmt.Sprintf("modules[%d].placement", i)
		switch mod.Placement {
		case "cloud":
			hasCloud = true
			if extPlacement == "device" {
				report.AddError(modPath, "device_extension_contains_cloud_module",
					fmt.Sprintf("device extension contains cloud module %q", mod.ID))
			}
		case "device":
			hasDevice = true
			if extPlacement == "cloud" {
				report.AddError(modPath, "cloud_extension_contains_device_module",
					fmt.Sprintf("cloud extension contains device module %q", mod.ID))
			}
		}
	}

	if extPlacement == "hybrid" {
		if !hasCloud || !hasDevice {
			report.AddError("", "hybrid_requires_cloud_and_device_modules",
				"hybrid extension must have at least one cloud and one device executable module")
		}
	}
}

func validateDeviceRequirements(mod ModuleMeta, modPath string, report *ValidationReport) {
	if mod.DeviceRequirements == nil {
		return
	}
	if mod.Placement != "device" {
		report.AddError(modPath+".deviceRequirements", "device_requirements_on_non_device_module",
			"deviceRequirements only allowed on device modules")
	}
	for i, p := range mod.DeviceRequirements.Platforms {
		normalized := normalizePlatform(p)
		if !knownPlatforms[normalized] {
			report.AddError(fmt.Sprintf("%s.deviceRequirements.platforms[%d]", modPath, i),
				"unknown_device_platform", fmt.Sprintf("unknown platform: %q", p))
		}
	}
	for i, a := range mod.DeviceRequirements.Architectures {
		normalized := normalizeArchitecture(a)
		if !knownArchitectures[normalized] {
			report.AddError(fmt.Sprintf("%s.deviceRequirements.architectures[%d]", modPath, i),
				"unknown_device_architecture", fmt.Sprintf("unknown architecture: %q", a))
		}
	}
	if mod.DeviceRequirements.MinAppVersion != "" {
		if !semVerPattern.MatchString(mod.DeviceRequirements.MinAppVersion) {
			report.AddError(modPath+".deviceRequirements.minAppVersion",
				"invalid_min_app_version",
				fmt.Sprintf("invalid semver: %q", mod.DeviceRequirements.MinAppVersion))
		}
	}
	if mod.DeviceRequirements.MinRuntimeVersion != "" {
		if !semVerPattern.MatchString(mod.DeviceRequirements.MinRuntimeVersion) {
			report.AddError(modPath+".deviceRequirements.minRuntimeVersion",
				"invalid_min_runtime_version",
				fmt.Sprintf("invalid semver: %q", mod.DeviceRequirements.MinRuntimeVersion))
		}
	}
	for i, f := range mod.DeviceRequirements.RequiredFeatures {
		if !requiredFeaturePattern.MatchString(f) {
			report.AddError(fmt.Sprintf("%s.deviceRequirements.requiredFeatures[%d]", modPath, i),
				"invalid_required_feature", fmt.Sprintf("invalid feature format: %q", f))
		}
	}
}

func validateProvidedCapabilities(mod ModuleMeta, modPath string, report *ValidationReport) {
	if len(mod.ProvidedCapabilities) > maxProvidedCapabilities {
		report.AddError(modPath+".providedCapabilities", "too_many_provided_capabilities",
			fmt.Sprintf("too many provided capabilities: %d (max %d)", len(mod.ProvidedCapabilities), maxProvidedCapabilities))
	}
	seen := make(map[string]bool)
	for i, pc := range mod.ProvidedCapabilities {
		pcPath := fmt.Sprintf("%s.providedCapabilities[%d]", modPath, i)
		normalizedID := strings.ToLower(strings.TrimSpace(pc.ID))
		if normalizedID == "" {
			report.AddError(pcPath+".id", "invalid_provided_capability_id", "capability id required")
		} else if !capabilityIDPattern.MatchString(normalizedID) {
			report.AddError(pcPath+".id", "invalid_provided_capability_id",
				fmt.Sprintf("invalid capability id: %q", pc.ID))
		}
		if seen[normalizedID] {
			report.AddError(pcPath+".id", "duplicate_provided_capability",
				fmt.Sprintf("duplicate capability id: %q", normalizedID))
		}
		seen[normalizedID] = true
		if pc.Version != "" && !semVerPattern.MatchString(pc.Version) {
			report.AddError(pcPath+".version", "invalid_provided_capability_version",
				fmt.Sprintf("invalid semver: %q", pc.Version))
		}
	}
}

func validateProviderMetadata(mod ModuleMeta, modPath string, report *ValidationReport) {
	if mod.Provider == nil {
		return
	}
	pPath := modPath + ".provider"
	if mod.Provider.ID != "" {
		normalizedID := strings.ToLower(strings.TrimSpace(mod.Provider.ID))
		if !providerIDPattern.MatchString(normalizedID) {
			report.AddError(pPath+".id", "invalid_provider_id",
				fmt.Sprintf("invalid provider id: %q", mod.Provider.ID))
		}
	}
	if mod.Provider.Priority < -100000 || mod.Provider.Priority > 100000 {
		report.AddError(pPath+".priority", "provider_priority_out_of_range",
			fmt.Sprintf("priority %d out of range [-100000, 100000]", mod.Provider.Priority))
	}
	if len(mod.Provider.Labels) > maxProviderLabels {
		report.AddError(pPath+".labels", "too_many_provider_labels",
			fmt.Sprintf("too many provider labels: %d (max %d)", len(mod.Provider.Labels), maxProviderLabels))
	}
	for k := range mod.Provider.Labels {
		normalizedKey := strings.ToLower(strings.TrimSpace(k))
		if !labelKeyPattern.MatchString(normalizedKey) {
			report.AddError(pPath+".labels."+k, "invalid_provider_label",
				fmt.Sprintf("invalid label key: %q", k))
		}
	}
	if len(mod.ProvidedCapabilities) == 0 {
		report.AddError(pPath, "provider_without_capability",
			"provider declared but no providedCapabilities")
	}
}

func checkMetadataSize(meta map[string]any, path string, report *ValidationReport) {
	if len(meta) == 0 {
		return
	}
	data, err := jsonMarshal(meta)
	if err != nil {
		return
	}
	if len(data) > maxMetadataBytes {
		report.AddError(path, "metadata_too_large",
			fmt.Sprintf("metadata exceeds %d bytes", maxMetadataBytes))
	}
}

func jsonMarshal(v any) ([]byte, error) {
	return jsonMarshalIndent(v, "", "")
}

func jsonMarshalIndent(v any, prefix, indent string) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

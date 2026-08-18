package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	UIAgentExtensionID         = domain.ExtensionID("com.amitia.builtin.uiagent")
	UIAgentModuleID            = domain.ModuleID("uiagent-runtime")
	UIAgentInspectCapabilityID = capability.CapabilityID("ui.inspect")
	UIAgentModifyCapabilityID  = capability.CapabilityID("ui.modify")
	UIAgentCreateCapabilityID  = capability.CapabilityID("ui.create")
	UIAgentPreviewCapabilityID = capability.CapabilityID("ui.preview")
	UIAgentProviderID          = capability.ProviderID("com.amitia.builtin.uiagent.provider")
)

func BuildUIAgentExtension(version string) Definition {
	ver, err := domain.ParseVersion(version)
	if err != nil {
		ver = domain.SemanticVersion{Major: 0, Minor: 1, Patch: 0}
	}

	extDef := domain.ExtensionDefinition{
		ID:   UIAgentExtensionID,
		Name: domain.LocalizedText{Default: "UI Agent"},
		Description: domain.LocalizedText{
			Default: "Provides UI inspection, modification, and creation capabilities for workspace files.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGeneral,
		Placement:       domain.ExtensionPlacementCloud,
		Publisher: domain.PublisherReference{
			PublisherID: "com.amitia",
			DisplayName: "Amitia",
			TrustLevel:  "system",
		},
		Package: domain.PackageReference{
			PackageID:       "builtin-uiagent",
			ManifestVersion: 1,
		},
		Modules: []domain.ModuleDefinition{
			{
				ID:          UIAgentModuleID,
				ExtensionID: UIAgentExtensionID,
				Name:        domain.LocalizedText{Default: "UI Agent Runtime"},
				Description: domain.LocalizedText{
					Default: "Built-in module providing UI agent inspection, modification, and creation.",
				},
				Type:    domain.ModuleTypeBuiltin,
				Version: version,
				Runtime: &domain.RuntimeDefinition{
					Type:        domain.RuntimeTypeBuiltin,
					EntryPoint:  "uiagent.runtime",
					WorkerCount: 2,
				},
				Contributions: buildUIAgentContributions(UIAgentExtensionID, UIAgentModuleID),
				ProvidedCapabilities: []domain.ProvidedCapability{
					{ID: string(UIAgentInspectCapabilityID), Version: version},
					{ID: string(UIAgentModifyCapabilityID), Version: version},
					{ID: string(UIAgentCreateCapabilityID), Version: version},
					{ID: string(UIAgentPreviewCapabilityID), Version: version},
				},
				Provider: &domain.ProviderMetadata{
					ID:       string(UIAgentProviderID),
					Priority: 90,
					Labels: map[string]string{
						"component": "uiagent",
					},
				},
				Placement: domain.ModulePlacementCloud,
				Compatibility: domain.ModuleCompatibility{
					Platforms: []string{"windows", "linux", "darwin"},
				},
				Policies: domain.ModulePolicies{
					NetworkAccess:    false,
					FileSystemAccess: true,
				},
			},
		},
		Compatibility: domain.ExtensionCompatibility{
			Platforms: []string{"windows", "linux", "darwin"},
		},
		Policies: domain.ExtensionPolicies{},
	}

	return Definition{
		Extension:         extDef,
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

func buildUIAgentContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	return []domain.ContributionDefinition{}
}

package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	GameHostExtensionID = domain.ExtensionID("com.amitia.builtin.game-host")
	GameHostModuleID    = domain.ModuleID("game-host-runtime")
)

// BuildGameHostExtension registers only the built-in plugin-host runtime. It
// intentionally exposes no game-specific Agent capabilities: concrete game
// packages own detection, state, control, tooling and any companion logic.
func BuildGameHostExtension(version string) Definition {
	ver := parseBuiltinVersion(version)
	_ = ver

	extDef := domain.ExtensionDefinition{
		ID:   GameHostExtensionID,
		Name: domain.LocalizedText{Default: "Game Plugin Host"},
		Description: domain.LocalizedText{
			Default: "Provides the isolated runtime, IPC, lifecycle and permission substrate used by installed game plugins.",
		},
		Version:         parseBuiltinVersion(version),
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGame,
		Placement:       domain.ExtensionPlacementDevice,
		Publisher:       domain.PublisherReference{PublisherID: "com.amitia", DisplayName: "Amitia", TrustLevel: "system"},
		Package:         domain.PackageReference{PackageID: "builtin-game-host", ManifestVersion: 1},
		Modules: []domain.ModuleDefinition{{
			ID: GameHostModuleID, ExtensionID: GameHostExtensionID,
			Name:        domain.LocalizedText{Default: "Game Plugin Host Runtime"},
			Description: domain.LocalizedText{Default: "Built-in infrastructure for hosting independent game plugins."},
			Type:        domain.ModuleTypeBuiltin, Version: version,
			Runtime:            &domain.RuntimeDefinition{Type: domain.RuntimeTypeBuiltin, EntryPoint: "gamehost.runtime", WorkerCount: 2},
			Contributions:      buildGameHostContributions(GameHostExtensionID, GameHostModuleID),
			Placement:          domain.ModulePlacementDevice,
			DeviceRequirements: &domain.DeviceRequirements{Platforms: []string{"windows", "linux", "darwin"}},
			Compatibility:      domain.ModuleCompatibility{Platforms: []string{"windows", "linux", "darwin"}},
			Policies:           domain.ModulePolicies{NetworkAccess: false, FileSystemAccess: false},
		}},
		Compatibility: domain.ExtensionCompatibility{Platforms: []string{"windows", "linux", "darwin"}},
		Policies:      domain.ExtensionPolicies{NetworkAccess: false},
	}
	return Definition{Extension: extDef, SystemManaged: true, Required: false, DisableAllowed: true, BootstrapRevision: 2}
}

func buildGameHostContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	_ = extID
	_ = modID
	return nil
}

const (
	DesktopPetExtensionID = domain.ExtensionID("com.amitia.builtin.desktop-pet")
	DesktopPetModuleID    = domain.ModuleID("desktop-pet-runtime")
	DesktopPetProviderID  = capability.ProviderID("com.amitia.builtin.desktop-pet.provider")

	DesktopPetCapRender   capability.CapabilityID = "desktoppet.render"
	DesktopPetCapInteract capability.CapabilityID = "desktoppet.interact"
	DesktopPetCapNotify   capability.CapabilityID = "desktoppet.notify"
	DesktopPetCapAnimate  capability.CapabilityID = "desktoppet.animate"
)

// BuildDesktopPetExtension constructs a Built-in Extension definition for the Desktop Pet.
//
//	Extension ID: com.amitia.builtin.desktop-pet
//	Module: desktop-pet-runtime
//	Provider Capabilities: desktoppet.render, desktoppet.interact,
//	                       desktoppet.notify, desktoppet.animate
func BuildDesktopPetExtension(version string) Definition {
	ver := parseBuiltinVersion(version)

	extDef := domain.ExtensionDefinition{
		ID:   DesktopPetExtensionID,
		Name: domain.LocalizedText{Default: "Desktop Pet"},
		Description: domain.LocalizedText{
			Default: "Provides desktop pet rendering, interaction, notification, and animation capabilities.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainDesktopPet,
		Placement:       domain.ExtensionPlacementDevice,
		Publisher: domain.PublisherReference{
			PublisherID: "com.amitia",
			DisplayName: "Amitia",
			TrustLevel:  "system",
		},
		Package: domain.PackageReference{
			PackageID:       "builtin-desktop-pet",
			ManifestVersion: 1,
		},
		Modules: []domain.ModuleDefinition{
			{
				ID:          DesktopPetModuleID,
				ExtensionID: DesktopPetExtensionID,
				Name:        domain.LocalizedText{Default: "Desktop Pet Runtime"},
				Description: domain.LocalizedText{
					Default: "Built-in module providing desktop pet rendering, interaction, notification, and animation.",
				},
				Type:    domain.ModuleTypeBuiltin,
				Version: version,
				Runtime: &domain.RuntimeDefinition{
					Type:        domain.RuntimeTypeBuiltin,
					EntryPoint:  "desktoppet.render",
					WorkerCount: 1,
				},
				Contributions: buildDesktopPetContributions(DesktopPetExtensionID, DesktopPetModuleID),
				ProvidedCapabilities: []domain.ProvidedCapability{
					{ID: string(DesktopPetCapRender), Version: ver.String()},
					{ID: string(DesktopPetCapInteract), Version: ver.String()},
					{ID: string(DesktopPetCapNotify), Version: ver.String()},
					{ID: string(DesktopPetCapAnimate), Version: ver.String()},
				},
				Provider: &domain.ProviderMetadata{
					ID:       string(DesktopPetProviderID),
					Priority: 70,
					Labels: map[string]string{
						"component": "desktop-pet",
					},
				},
				Placement: domain.ModulePlacementDevice,
				DeviceRequirements: &domain.DeviceRequirements{
					Platforms: []string{"windows", "linux", "darwin"},
				},
				Compatibility: domain.ModuleCompatibility{
					Platforms: []string{"windows", "linux", "darwin"},
				},
				Policies: domain.ModulePolicies{
					NetworkAccess:    false,
					FileSystemAccess: false,
				},
			},
		},
		Compatibility: domain.ExtensionCompatibility{
			Platforms: []string{"windows", "linux", "darwin"},
		},
		Policies: domain.ExtensionPolicies{
			NetworkAccess: false,
		},
	}

	return Definition{
		Extension:         extDef,
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

func buildDesktopPetContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	return []domain.ContributionDefinition{
		{
			ID:          domain.ContributionID("desktop_pet_plugin"),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindDesktopPetPlugin,
			Name:        domain.LocalizedText{Default: "Desktop Pet Plugin"},
			Description: domain.LocalizedText{
				Default: "Desktop pet plugin contribution providing rendering, interaction, notification, and animation for desktop pets.",
			},
		},
	}
}

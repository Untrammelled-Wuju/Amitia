package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	GameHostExtensionID = domain.ExtensionID("com.amitia.builtin.game-host")
	GameHostModuleID    = domain.ModuleID("game-host-runtime")
	GameHostProviderID  = capability.ProviderID("com.amitia.builtin.game-host.provider")

	GameHostCapSessionManage   capability.CapabilityID = "game.session.manage"
	GameHostCapSessionTransfer capability.CapabilityID = "game.session.transfer"
	GameHostCapAssetManage     capability.CapabilityID = "game.asset.manage"
	GameHostCapPluginManage    capability.CapabilityID = "game.plugin.manage"
	GameHostCapScriptExecute   capability.CapabilityID = "game.script.execute"
	GameHostCapDeviceManage    capability.CapabilityID = "game.device.manage"
)

// BuildGameHostExtension constructs a Built-in Extension definition for the Game Host.
//
//	Extension ID: com.amitia.builtin.game-host
//	Module: game-host-runtime
//	Provider Capabilities: game.session.manage, game.session.transfer,
//	                       game.asset.manage, game.plugin.manage,
//	                       game.script.execute, game.device.manage
func BuildGameHostExtension(version string) Definition {
	ver := parseBuiltinVersion(version)

	extDef := domain.ExtensionDefinition{
		ID:   GameHostExtensionID,
		Name: domain.LocalizedText{Default: "Game Host"},
		Description: domain.LocalizedText{
			Default: "Provides game session management, asset management, plugin hosting, script execution, and device management for game runtimes.",
		},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainGame,
		Placement:       domain.ExtensionPlacementDevice,
		Publisher: domain.PublisherReference{
			PublisherID: "com.amitia",
			DisplayName: "Amitia",
			TrustLevel:  "system",
		},
		Package: domain.PackageReference{
			PackageID:       "builtin-game-host",
			ManifestVersion: 1,
		},
		Modules: []domain.ModuleDefinition{
			{
				ID:          GameHostModuleID,
				ExtensionID: GameHostExtensionID,
				Name:        domain.LocalizedText{Default: "Game Host Runtime"},
				Description: domain.LocalizedText{
					Default: "Built-in module providing game session, asset, plugin, script, and device management.",
				},
				Type:    domain.ModuleTypeBuiltin,
				Version: version,
				Runtime: &domain.RuntimeDefinition{
					Type:        domain.RuntimeTypeBuiltin,
					EntryPoint:  "game.session.manage",
					WorkerCount: 2,
				},
				Contributions: buildGameHostContributions(GameHostExtensionID, GameHostModuleID),
				ProvidedCapabilities: []domain.ProvidedCapability{
					{ID: string(GameHostCapSessionManage), Version: ver.String()},
					{ID: string(GameHostCapSessionTransfer), Version: ver.String()},
					{ID: string(GameHostCapAssetManage), Version: ver.String()},
					{ID: string(GameHostCapPluginManage), Version: ver.String()},
					{ID: string(GameHostCapScriptExecute), Version: ver.String()},
					{ID: string(GameHostCapDeviceManage), Version: ver.String()},
				},
				Provider: &domain.ProviderMetadata{
					ID:       string(GameHostProviderID),
					Priority: 80,
					Labels: map[string]string{
						"component": "game-host",
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
					NetworkAccess:    true,
					FileSystemAccess: true,
				},
			},
		},
		Compatibility: domain.ExtensionCompatibility{
			Platforms: []string{"windows", "linux", "darwin"},
		},
		Policies: domain.ExtensionPolicies{
			NetworkAccess: true,
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

func buildGameHostContributions(extID domain.ExtensionID, modID domain.ModuleID) []domain.ContributionDefinition {
	_ = extID
	_ = modID
	// GameHost is host infrastructure, not a game_plugin contribution. Concrete games
	// (Minecraft, Stardew Valley, etc.) are installed as independent .amitiax packages.
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

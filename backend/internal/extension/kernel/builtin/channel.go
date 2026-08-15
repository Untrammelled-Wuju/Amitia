package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	moduleIDWebChannel    domain.ModuleID = "channel-web"
	moduleIDQQChannel     domain.ModuleID = "channel-qq"
	moduleIDWechatChannel domain.ModuleID = "channel-wechat"

	providerIDWebChannel    = "web"
	providerIDQQChannel     = "qq"
	providerIDWechatChannel = "wechat"

	capabilityDeliverWeb    capability.CapabilityID = "channel.deliver.web"
	capabilityDeliverQQ     capability.CapabilityID = "channel.deliver.qq"
	capabilityDeliverWechat capability.CapabilityID = "channel.deliver.wechat"
)

func BuildWebChannelExtension(version string) Definition {
	ver, _ := domain.ParseVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   domain.ExtensionID(PrefixBuiltin + "channel.web"),
			Name: domain.LocalizedText{Default: "Web Channel"},
			Description: domain.LocalizedText{
				Default: "Built-in web channel delivery provider for sending messages through web interfaces.",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainSystem,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Built-in",
			},
			Policies: domain.ExtensionPolicies{
				AutoUpdate:    false,
				NetworkAccess: true,
			},
			Compatibility: domain.ExtensionCompatibility{
				Platforms: []string{"web"},
			},
			Modules: []ModuleDefinition{
				buildChannelModule(
					moduleIDWebChannel,
					providerIDWebChannel,
					capabilityDeliverWeb,
				),
			},
		},
		SystemManaged:  true,
		Required:       true,
		DisableAllowed: false,
	}
}

func BuildQQChannelExtension(version string) Definition {
	ver, _ := domain.ParseVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   domain.ExtensionID(PrefixBuiltin + "channel.qq"),
			Name: domain.LocalizedText{Default: "QQ Channel"},
			Description: domain.LocalizedText{
				Default: "Built-in QQ channel delivery provider for sending messages through QQ sidecar.",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainSystem,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Built-in",
			},
			Policies: domain.ExtensionPolicies{
				AutoUpdate:    false,
				NetworkAccess: true,
			},
			Compatibility: domain.ExtensionCompatibility{
				Platforms: []string{"qq"},
			},
			Modules: []ModuleDefinition{
				buildChannelModule(
					moduleIDQQChannel,
					providerIDQQChannel,
					capabilityDeliverQQ,
				),
			},
		},
		SystemManaged:  true,
		Required:       false,
		DisableAllowed: true,
	}
}

func BuildWechatChannelExtension(version string) Definition {
	ver, _ := domain.ParseVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   domain.ExtensionID(PrefixBuiltin + "channel.wechat"),
			Name: domain.LocalizedText{Default: "Wechat Channel"},
			Description: domain.LocalizedText{
				Default: "Built-in Wechat channel delivery provider for sending messages through Wechat sidecar.",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainSystem,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Built-in",
			},
			Policies: domain.ExtensionPolicies{
				AutoUpdate:    false,
				NetworkAccess: true,
			},
			Compatibility: domain.ExtensionCompatibility{
				Platforms: []string{"wechat"},
			},
			Modules: []ModuleDefinition{
				buildChannelModule(
					moduleIDWechatChannel,
					providerIDWechatChannel,
					capabilityDeliverWechat,
				),
			},
		},
		SystemManaged:  true,
		Required:       false,
		DisableAllowed: true,
	}
}

func buildChannelModule(id domain.ModuleID, providerName string, capID capability.CapabilityID) ModuleDefinition {
	extID := domain.ExtensionID(PrefixBuiltin + "channel." + providerName)
	modID := "channel-" + domain.ModuleID(providerName)

	return ModuleDefinition{
		ID:          modID,
		ExtensionID: extID,
		Name: domain.LocalizedText{
			Default: "Channel Provider - " + providerName,
		},
		Description: domain.LocalizedText{
			Default: "Provides channel delivery capability for " + providerName,
		},
		Type:    ModuleTypeBuiltin,
		Version: "1.0.0",
		Runtime: &RuntimeDefinition{
			Type:       domain.RuntimeTypeBuiltin,
			EntryPoint: "deliver",
		},
		Contributions: []ContributionDefinition{
			{
				ID:          ContributionID(capID),
				ModuleID:    modID,
				ExtensionID: extID,
				Kind:        ContributionKindProvider,
				Name: domain.LocalizedText{
					Default: "Channel Provider - " + providerName,
				},
			},
		},
		Placement: ModulePlacementCloud,
		ProvidedCapabilities: []domain.ProvidedCapability{
			{
				ID:      string(capID),
				Version: "1.0.0",
			},
		},
		Provider: &domain.ProviderMetadata{
			ID:       providerName,
			Priority: 100,
			Metadata: map[string]any{
				"channel": providerName,
			},
		},
	}
}

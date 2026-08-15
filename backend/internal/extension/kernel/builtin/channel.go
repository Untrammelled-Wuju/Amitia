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

// BuildWebChannelExtension constructs a Built-in Extension definition for the Web channel.
//
//	Extension ID: com.amitia.builtin.channel.web
//	Provider Capability: channel.deliver.web
func BuildWebChannelExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "channel.web")
	moduleID := moduleIDWebChannel
	contributionID := domain.ContributionID(PrefixBuiltin + "channel.web/deliver")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "Web Channel"},
			Description: domain.LocalizedText{
				Default: "Built-in web channel delivery provider for sending messages through web interfaces.",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-channel-web",
			},
			Modules: []domain.ModuleDefinition{
				buildChannelModule(extID, moduleID, contributionID, providerIDWebChannel, capabilityDeliverWeb, ver),
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:    true,
		Required:         true,
		DisableAllowed:   false,
		BootstrapRevision: 1,
	}
}

// BuildQQChannelExtension constructs a Built-in Extension definition for the QQ channel.
//
//	Extension ID: com.amitia.builtin.channel.qq
//	Provider Capability: channel.deliver.qq
func BuildQQChannelExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "channel.qq")
	moduleID := moduleIDQQChannel
	contributionID := domain.ContributionID(PrefixBuiltin + "channel.qq/deliver")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "QQ Channel"},
			Description: domain.LocalizedText{
				Default: "Built-in QQ channel delivery provider for sending messages through QQ sidecar.",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-channel-qq",
			},
			Modules: []domain.ModuleDefinition{
				buildChannelModule(extID, moduleID, contributionID, providerIDQQChannel, capabilityDeliverQQ, ver),
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:    true,
		Required:         false,
		DisableAllowed:   true,
		BootstrapRevision: 1,
	}
}

// BuildWechatChannelExtension constructs a Built-in Extension definition for the Wechat channel.
//
//	Extension ID: com.amitia.builtin.channel.wechat
//	Provider Capability: channel.deliver.wechat
func BuildWechatChannelExtension(version string) Definition {
	extID := domain.ExtensionID(PrefixBuiltin + "channel.wechat")
	moduleID := moduleIDWechatChannel
	contributionID := domain.ContributionID(PrefixBuiltin + "channel.wechat/deliver")
	ver := parseBuiltinVersion(version)

	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:   extID,
			Name: domain.LocalizedText{Default: "Wechat Channel"},
			Description: domain.LocalizedText{
				Default: "Built-in Wechat channel delivery provider for sending messages through Wechat sidecar.",
			},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementCloud,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia",
				DisplayName: "Amitia",
			},
			Package: domain.PackageReference{
				PackageID: "builtin-channel-wechat",
			},
			Modules: []domain.ModuleDefinition{
				buildChannelModule(extID, moduleID, contributionID, providerIDWechatChannel, capabilityDeliverWechat, ver),
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:    true,
		Required:         false,
		DisableAllowed:   true,
		BootstrapRevision: 1,
	}
}

func buildChannelModule(
	extID domain.ExtensionID,
	modID domain.ModuleID,
	contributionID domain.ContributionID,
	providerName string,
	capID capability.CapabilityID,
	ver domain.SemanticVersion,
) domain.ModuleDefinition {
	return domain.ModuleDefinition{
		ID:          modID,
		ExtensionID: extID,
		Name: domain.LocalizedText{
			Default: "Channel Provider - " + providerName,
		},
		Description: domain.LocalizedText{
			Default: "Provides channel delivery capability for " + providerName,
		},
		Type: domain.ModuleTypeBuiltin,
		Runtime: &domain.RuntimeDefinition{
			Type: domain.RuntimeTypeBuiltin,
		},
		Contributions: []domain.ContributionDefinition{
			{
				ID:          contributionID,
				ModuleID:    modID,
				ExtensionID: extID,
				Kind:        domain.ContributionKindProvider,
				Name: domain.LocalizedText{
					Default: "Channel Deliver - " + providerName,
				},
				Description: domain.LocalizedText{
					Default: "Deliver messages through the " + providerName + " channel",
				},
				RuntimeBinding: &domain.RuntimeBinding{
					RuntimeType: domain.RuntimeTypeBuiltin,
					InstanceID:  string(capID),
				},
			},
		},
		ProvidedCapabilities: []domain.ProvidedCapability{
			{ID: string(capID), Version: ver.String()},
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

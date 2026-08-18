package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

const (
	moduleIDWebChannel    domain.ModuleID = "channel-web"
	moduleIDQQChannel     domain.ModuleID = "channel-qq"
	moduleIDWechatChannel domain.ModuleID = "channel-wechat"

	providerIDWebChannel    = "builtin.channel.web"
	providerIDQQChannel     = "builtin.channel.qq"
	providerIDWechatChannel = "builtin.channel.wechat"

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
				buildChannelModule(extID, moduleID, providerIDWebChannel, capabilityDeliverWeb, ver),
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          true,
		DisableAllowed:    false,
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
				buildChannelModule(extID, moduleID, providerIDQQChannel, capabilityDeliverQQ, ver),
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
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
				buildChannelModule(extID, moduleID, providerIDWechatChannel, capabilityDeliverWechat, ver),
			},
			Compatibility: domain.ExtensionCompatibility{},
			Integrity:     domain.ExtensionIntegrity{},
			Policies:      domain.ExtensionPolicies{},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

func buildChannelModule(
	extID domain.ExtensionID,
	modID domain.ModuleID,
	providerID string,
	capID capability.CapabilityID,
	ver domain.SemanticVersion,
) domain.ModuleDefinition {
	return domain.ModuleDefinition{
		ID:          modID,
		ExtensionID: extID,
		Name: domain.LocalizedText{
			Default: "Channel Provider - " + providerID,
		},
		Description: domain.LocalizedText{
			Default: "Provides channel delivery capability for " + providerID,
		},
		Type: domain.ModuleTypeBuiltin,
		Runtime: &domain.RuntimeDefinition{
			Type: domain.RuntimeTypeBuiltin,
		},
		Contributions: buildChannelContributions(extID, modID, capID),
		ProvidedCapabilities: []domain.ProvidedCapability{
			{ID: string(capID), Version: ver.String()},
		},
		Provider: &domain.ProviderMetadata{
			ID:       providerID,
			Priority: 100,
			Metadata: map[string]any{
				"channel": providerID,
			},
		},
	}
}

func buildChannelContributions(extID domain.ExtensionID, modID domain.ModuleID, capID capability.CapabilityID) []domain.ContributionDefinition {
	contributionID := "channel_deliver_" + string(capID)
	modelName := "channel.deliver." + string(capID)
	return []domain.ContributionDefinition{
		{
			ID:          domain.ContributionID(contributionID),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "Channel Deliver - " + string(capID)},
			Description: domain.LocalizedText{Default: "Send a message through the " + string(capID) + " channel"},
			Definition: map[string]any{
				"capabilityId": string(capID),
				"modelName":    modelName,
				"inputSchema":  `{"type":"object","properties":{"channel":{"type":"string","description":"Target channel name"},"peerId":{"type":"string","description":"Recipient peer ID"},"contentType":{"type":"string","enum":["text","emote","image","file"],"description":"Content type of the message"},"payload":{"type":"object","description":"Message payload"}},"required":["channel","peerId","payload"],"additionalProperties":false}`,
				"outputSchema": `{"type":"object","properties":{"intentId":{"type":"string","description":"Delivery intent ID"},"status":{"type":"string","description":"Initial delivery status"}}}`,
				"riskLevel":    "medium",
				"sideEffect":   "write",
				"permissions":  []map[string]any{{"capability": string(capID), "risk": "medium"}},
				"timeoutMs":    int64(30000),
				"idempotent":   false,
				"retryable":    true,
				"runtime": map[string]any{
					"runtimeType": "channel",
					"runtimeId":   "default",
					"handlerName": string(capID),
				},
			},
			Metadata: map[string]any{
				"system.builtin": true,
			},
		},
	}
}

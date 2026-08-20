package ui_provider

func builtin(providerID string, capability Capability) ProviderDefinition {
	return ProviderDefinition{
		ProviderID:  providerID,
		ExtensionID: "builtin.amitia.ui",
		ModuleID:    "builtin-ui",
		Capability:  capability,
		Mode:        ModeReplace,
		Priority:    -1000,
		Platforms:   []string{"*"},
		Entries:     map[string]Entry{"*": {Type: EntryBuiltinNative, ExportName: providerID}},
		TrustLevel:  "system",
		Enabled:     true,
		Builtin:     true,
	}
}

func BuiltinProviders() []ProviderDefinition {
	return []ProviderDefinition{
		builtin("builtin.amitia.app-shell", CapabilityAppShell),
		builtin("builtin.amitia.navigation", CapabilityAppNavigation),
		builtin("builtin.amitia.workspace", CapabilityAppWorkspace),
		builtin("builtin.amitia.routes", CapabilityRouteRegistry),
		builtin("builtin.amitia.page", CapabilityPageProvider),
		builtin("builtin.amitia.conversation", CapabilityConversationShell),
		builtin("builtin.amitia.conversation-header", CapabilityConversationHeader),
		builtin("builtin.amitia.conversation-messages", CapabilityConversationMessages),
		builtin("builtin.amitia.message-renderer", CapabilityConversationMessageRenderer),
		builtin("builtin.amitia.conversation-sidebar", CapabilityConversationSidebar),
		builtin("builtin.amitia.composer", CapabilityConversationComposer),
		builtin("builtin.amitia.conversation-overlay", CapabilityConversationOverlay),
		builtin("builtin.amitia.character", CapabilityCharacterShell),
		builtin("builtin.amitia.character-detail", CapabilityCharacterDetail),
		builtin("builtin.amitia.memory", CapabilityMemoryShell),
		builtin("builtin.amitia.memory-detail", CapabilityMemoryDetail),
		builtin("builtin.amitia.settings", CapabilitySettingsShell),
		builtin("builtin.amitia.settings-section", CapabilitySettingsSection),
		builtin("builtin.amitia.extension-center", CapabilityExtensionCenter),
		builtin("builtin.amitia.extension-page", CapabilityExtensionPage),
		builtin("builtin.amitia.theme", CapabilityTheme),
		builtin("builtin.amitia.tokens", CapabilityTokens),
		builtin("builtin.amitia.icons", CapabilityIcons),
		builtin("builtin.amitia.components", CapabilityComponents),
	}
}

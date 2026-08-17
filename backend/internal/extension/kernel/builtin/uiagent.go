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
	UIAgentProviderID          = capability.ProviderID("com.amitia.builtin.uiagent.provider")
)

func buildUIAgentInspectInputSchema() string {
	return `{"type":"object","additionalProperties":false,"required":["workspaceId"],"properties":{"workspaceId":{"type":"string"},"paths":{"type":"array","items":{"type":"string"}},"includeSymbols":{"type":"boolean"},"includeFramework":{"type":"boolean"}}}`
}

func buildUIAgentInspectOutputSchema() string {
	return `{"type":"object","additionalProperties":false,"properties":{"workspaceId":{"type":"string"},"totalFiles":{"type":"integer"},"hasEntrypoint":{"type":"boolean"},"uiFilePaths":{"type":"array","items":{"type":"string"}},"framework":{"type":"string"},"symbols":{"type":"array","items":{"type":"object"}},"fileHashes":{"type":"object"},"editable":{"type":"boolean"}}}`
}

func buildUIAgentModifyInputSchema() string {
	return `{"type":"object","additionalProperties":false,"required":["workspaceId","operations"],"properties":{"workspaceId":{"type":"string"},"operations":{"type":"array","items":{"type":"object","required":["path"],"properties":{"path":{"type":"string"},"patch":{"type":"string"},"oldText":{"type":"string"},"newText":{"type":"string"},"baseSha256":{"type":"string"}},"additionalProperties":false}},"transaction":{"type":"boolean"},"preview":{"type":"boolean"}}}`
}

func buildUIAgentModifyOutputSchema() string {
	return `{"type":"object","additionalProperties":false,"properties":{"success":{"type":"boolean"},"appliedOperations":{"type":"integer"},"transactionToken":{"type":"string"},"changedFiles":{"type":"array","items":{"type":"string"}},"previewRef":{"type":"string"}}}`
}

func buildUIAgentCreateInputSchema() string {
	return `{"type":"object","additionalProperties":false,"required":["workspaceId"],"properties":{"workspaceId":{"type":"string"},"mode":{"type":"string","enum":["source","schema"]},"description":{"type":"string"},"targetPath":{"type":"string"}}}`
}

func buildUIAgentCreateOutputSchema() string {
	return `{"type":"object","additionalProperties":false,"properties":{"success":{"type":"boolean"},"createdFiles":{"type":"array","items":{"type":"string"}},"schemaId":{"type":"string"}}}`
}

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
	return []domain.ContributionDefinition{
		{
			ID:          domain.ContributionID("uiagent_inspect"),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "UI Inspect"},
			Description: domain.LocalizedText{Default: "Inspect UI workspace files, symbols, and structure."},
			Definition: map[string]any{
				"capabilityId": string(UIAgentInspectCapabilityID),
				"modelName":    "uiagent.inspect",
				"inputSchema":  buildUIAgentInspectInputSchema(),
				"outputSchema": buildUIAgentInspectOutputSchema(),
				"runtime": map[string]any{
					"runtimeType": "uiagent",
					"runtimeId":   "default",
					"handlerName": "uiagent.inspect",
				},
			},
			Metadata: map[string]any{"system.builtin": true},
		},
		{
			ID:          domain.ContributionID("uiagent_modify"),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "UI Modify"},
			Description: domain.LocalizedText{Default: "Modify UI workspace files with transaction support."},
			Definition: map[string]any{
				"capabilityId": string(UIAgentModifyCapabilityID),
				"modelName":    "uiagent.modify",
				"inputSchema":  buildUIAgentModifyInputSchema(),
				"outputSchema": buildUIAgentModifyOutputSchema(),
				"runtime": map[string]any{
					"runtimeType": "uiagent",
					"runtimeId":   "default",
					"handlerName": "uiagent.modify",
				},
			},
			Metadata: map[string]any{"system.builtin": true},
		},
		{
			ID:          domain.ContributionID("uiagent_create"),
			ModuleID:    modID,
			ExtensionID: extID,
			Kind:        domain.ContributionKindTool,
			Name:        domain.LocalizedText{Default: "UI Create"},
			Description: domain.LocalizedText{Default: "Create new UI workspace files or schema drafts."},
			Definition: map[string]any{
				"capabilityId": string(UIAgentCreateCapabilityID),
				"modelName":    "uiagent.create",
				"inputSchema":  buildUIAgentCreateInputSchema(),
				"outputSchema": buildUIAgentCreateOutputSchema(),
				"runtime": map[string]any{
					"runtimeType": "uiagent",
					"runtimeId":   "default",
					"handlerName": "uiagent.create",
				},
			},
			Metadata: map[string]any{"system.builtin": true},
		},
	}
}

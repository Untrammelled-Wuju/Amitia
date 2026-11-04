package builtin

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/pluginmarket"
)

//go:embed plugin_market_schema.json
var pluginMarketSchema []byte

func BuildPluginMarketExtension(version string) Definition {
	ver := parseBuiltinVersion(version)
	extID := domain.ExtensionID(pluginmarket.ExtensionID)
	runtimeModuleID := domain.ModuleID(pluginmarket.RuntimeModuleID)
	uiModuleID := domain.ModuleID("plugin-market-ui")
	schema := pluginMarketSchemaDocument()
	pageContributionID := domain.ContributionID("plugin-market-page")
	mobileContributionID := domain.ContributionID("plugin-market-page-mobile")
	return Definition{
		Extension: domain.ExtensionDefinition{
			ID:              extID,
			Name:            domain.LocalizedText{Default: "Amitia 插件市场"},
			Description:     domain.LocalizedText{Default: "作为普通插件提供插件目录、来源审查、权限确认和安装入口。"},
			Version:         ver,
			ManifestVersion: 1,
			Domain:          domain.ExtensionDomainGeneral,
			Placement:       domain.ExtensionPlacementHybrid,
			Publisher: domain.PublisherReference{
				PublisherID: "com.amitia", DisplayName: "Amitia", TrustLevel: "official",
			},
			Package: domain.PackageReference{PackageID: "builtin-plugin-market", ManifestVersion: 1},
			Modules: []domain.ModuleDefinition{
				{
					ID: runtimeModuleID, ExtensionID: extID,
					Name: domain.LocalizedText{Default: "插件市场运行时"}, Type: domain.ModuleTypeBuiltin,
					Version: version, Placement: domain.ModulePlacementCloud,
					Runtime:              &domain.RuntimeDefinition{Type: domain.RuntimeTypeBuiltin, EntryPoint: pluginmarket.HandlerName},
					Contributions:        []domain.ContributionDefinition{pluginMarketToolContribution(extID, runtimeModuleID, version)},
					ProvidedCapabilities: []domain.ProvidedCapability{{ID: "plugin.market", Version: version}},
				},
				{
					ID: uiModuleID, ExtensionID: extID,
					Name: domain.LocalizedText{Default: "插件市场界面"}, Type: domain.ModuleTypeDataOnly,
					Version: version, Placement: domain.ModulePlacementCloud,
					Contributions: []domain.ContributionDefinition{
						pluginMarketPageContribution(extID, uiModuleID, pageContributionID, schema),
						pluginMarketPageContribution(extID, uiModuleID, mobileContributionID, schema),
						pluginMarketRouteContribution(extID, uiModuleID),
						pluginMarketPageProviderContribution(extID, uiModuleID, pageContributionID, mobileContributionID),
					},
				},
			},
			Compatibility: domain.ExtensionCompatibility{Platforms: []string{"windows", "linux", "darwin", "android", "ios"}},
			Metadata:      map[string]any{"system.defaultEnabled": true},
		},
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

func pluginMarketToolContribution(extID domain.ExtensionID, modID domain.ModuleID, version string) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID: "plugin-market-command", ModuleID: modID, ExtensionID: extID,
		Kind: domain.ContributionKindTool, Version: version,
		Name:        domain.LocalizedText{Default: "插件市场命令"},
		Description: domain.LocalizedText{Default: "读取插件目录、准备安装审查并确认安装普通插件。"},
		RequiredPermissions: []string{
			"tool.invoke", "service.network.request", "extensions.install",
		},
		Definition: map[string]any{
			"toolId": pluginmarket.ToolID, "modelName": "amitia_plugin_market",
			"handlerName": pluginmarket.HandlerName,
			"inputSchema": map[string]any{
				"type": "object", "required": []string{"action"},
				"properties": map[string]any{
					"action":                map[string]any{"type": "string", "enum": []string{"catalog", "prepare", "install"}},
					"q":                     map[string]any{"type": "string"},
					"category":              map[string]any{"type": "string"},
					"catalogUrl":            map[string]any{"type": "string"},
					"extensionId":           map[string]any{"type": "string"},
					"version":               map[string]any{"type": "string"},
					"packageUri":            map[string]any{"type": "string"},
					"sha256":                map[string]any{"type": "string"},
					"fileName":              map[string]any{"type": "string"},
					"publisher":             map[string]any{"type": "string"},
					"trustLevel":            map[string]any{"type": "string"},
					"entryId":               map[string]any{"type": "string"},
					"requiredConfirmations": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"confirmations":         map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "boolean"}},
					"confirmed":             map[string]any{"type": "boolean"},
					"sessionId":             map[string]any{"type": "string"},
				},
			},
			"outputSchema": map[string]any{"type": "object", "additionalProperties": true},
			"riskLevel":    "high", "sideEffect": "external",
			"runtime": map[string]any{
				"runtimeType": "builtin", "handlerName": pluginmarket.HandlerName,
			},
			"permissions": []map[string]any{
				{"capability": "service.network.request", "risk": "medium"},
				{"capability": "extensions.install", "risk": "high"},
			},
		},
		Metadata: map[string]any{"system.builtin": true},
	}
}

func pluginMarketPageContribution(extID domain.ExtensionID, modID domain.ModuleID, contributionID domain.ContributionID, schema map[string]any) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID: contributionID, ModuleID: modID, ExtensionID: extID,
		Kind:    domain.ContributionKind("com.amitia.plugin-market/page"),
		Adapter: domain.ContributionAdapterUI, ContractVersion: 1,
		Schema:              json.RawMessage(`{"type":"object","required":["kind","slot"]}`),
		Name:                domain.LocalizedText{Default: "插件市场页面"},
		RequiredPermissions: []string{"tool.invoke"},
		Definition: map[string]any{
			"contribution_id": string(contributionID), "extension_id": string(extID), "module_id": string(modID),
			"kind":             "schema_page",
			"slot":             map[string]any{"slot_id": "provider.page.provider", "contract_version": 1},
			"contract_version": 1,
			"display": map[string]any{
				"title":       map[string]any{"default": "插件市场"},
				"description": map[string]any{"default": "从管理端目录发现并安装经过宿主验证的普通插件。"},
				"icon":        "extension",
			},
			"entry": map[string]any{
				"type": "schema_renderer", "content_hash": "plugin-market-inline-v1",
			},
			"schema":  schema,
			"sandbox": map[string]any{"type": "schema_renderer"},
			"actions": []map[string]any{{
				"action_id":  "command",
				"title":      map[string]any{"default": "执行插件市场命令"},
				"target":     map[string]any{"type": "tool", "tool_id": pluginmarket.ToolID},
				"risk_level": "high",
			}},
			"integrity": map[string]any{"definition_hash": "plugin-market-inline-v1"},
		},
		Metadata: map[string]any{"system.builtin": true},
	}
}

func pluginMarketRouteContribution(extID domain.ExtensionID, modID domain.ModuleID) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID: "plugin-market-routes", ModuleID: modID, ExtensionID: extID,
		Kind: domain.ContributionKindUIProvider,
		Name: domain.LocalizedText{Default: "插件市场路由"},
		Definition: map[string]any{
			"providerId": "plugin-market-routes", "capability": "route.registry",
			"mode": "replace", "priority": 260, "trustLevel": "official", "placement": "cloud",
			"entries": platformDeclarativeEntries(),
			"metadata": map[string]any{
				"routes": []map[string]any{{
					"id": "plugin-market", "path": "/extensions/market", "providerId": "plugin-market-page-provider",
					"capability": "page.provider", "title": "插件市场", "priority": 260,
				}},
				"navigationItems": []map[string]any{{
					"id": "plugin-market", "label": "插件市场", "route": "/extensions/market",
					"icon": "extension", "group": "extensions", "groupLabel": "扩展",
					"groupIcon": "extension", "order": 10, "mobile": true, "panel": "main",
				}},
			},
		},
		Metadata: map[string]any{"system.builtin": true},
	}
}

func pluginMarketPageProviderContribution(extID domain.ExtensionID, modID domain.ModuleID, pageID, mobilePageID domain.ContributionID) domain.ContributionDefinition {
	return domain.ContributionDefinition{
		ID: "plugin-market-page-provider", ModuleID: modID, ExtensionID: extID,
		Kind: domain.ContributionKindUIProvider,
		Name: domain.LocalizedText{Default: "插件市场页面 Provider"},
		Definition: map[string]any{
			"providerId": "plugin-market-page-provider", "capability": "page.provider",
			"mode": "replace", "priority": 260, "trustLevel": "official", "placement": "cloud",
			"entries":  platformSchemaEntries(string(pageID), string(mobilePageID)),
			"metadata": map[string]any{"routes": []string{"/extensions/market"}},
		},
		Metadata: map[string]any{"system.builtin": true},
	}
}

func platformDeclarativeEntries() map[string]any {
	entries := make(map[string]any)
	for _, platform := range []string{"web", "mobile", "electron_windows", "electron_macos", "electron_linux"} {
		entries[platform] = map[string]any{"type": "declarative"}
	}
	return entries
}

func platformSchemaEntries(contributionID, mobileContributionID string) map[string]any {
	entries := make(map[string]any)
	for _, platform := range []string{"web", "electron_windows", "electron_macos", "electron_linux"} {
		entries[platform] = map[string]any{"type": "schema_renderer", "contributionId": contributionID}
	}
	entries["mobile"] = map[string]any{"type": "schema_renderer", "contributionId": mobileContributionID}
	return entries
}

func pluginMarketSchemaDocument() map[string]any {
	var schema map[string]any
	if err := json.Unmarshal(pluginMarketSchema, &schema); err != nil {
		panic(fmt.Errorf("decode plugin market schema: %w", err))
	}
	return schema
}

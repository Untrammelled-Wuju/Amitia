package builtin

import (
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/runtimegate"
)

func BuildEmotionExtension(version string) Definition {
	return buildCharacterFeatureExtension(
		runtimegate.EmotionExtensionID,
		version,
		"情绪系统",
		"为角色提供情绪状态、心理状态和情绪融合能力。",
		"emotion-core",
		"host.character.psyche",
		"心理状态",
		"psyche",
		210,
		[]string{"emotion.state.read", "emotion.state.write", "emotion.fusion"},
	)
}

func BuildProactiveExtension(version string) Definition {
	return buildCharacterFeatureExtension(
		runtimegate.ProactiveExtensionID,
		version,
		"主动消息",
		"为角色提供主动消息规则、调度、发送和历史记录能力。",
		"proactive-core",
		"host.character.proactive",
		"主动消息",
		"proactive",
		200,
		[]string{"proactive.rules.read", "proactive.rules.write", "proactive.dispatch"},
	)
}

func BuildLifestyleExtension(version string) Definition {
	return buildCharacterFeatureExtension(
		runtimegate.LifestyleExtensionID,
		version,
		"生活系统",
		"管理角色的日程、生活状态、提醒和运行情况。",
		"lifestyle-core",
		"host.character.lifestyle",
		"生活系统",
		"life-system",
		220,
		[]string{"lifestyle.state.read", "lifestyle.schedule.manage", "lifestyle.debug.read"},
	)
}

func buildCharacterFeatureExtension(
	extensionID string,
	version string,
	displayName string,
	description string,
	moduleID string,
	runtimeID string,
	tabTitle string,
	tabKey string,
	order int,
	capabilities []string,
	defaultEnabled ...bool,
) Definition {
	ver := parseBuiltinVersion(version)
	extID := domain.ExtensionID(extensionID)
	modID := domain.ModuleID(moduleID)
	def := domain.ExtensionDefinition{
		ID:              extID,
		Name:            domain.LocalizedText{Default: displayName},
		Description:     domain.LocalizedText{Default: description},
		Version:         ver,
		ManifestVersion: 1,
		Domain:          domain.ExtensionDomainCompanion,
		Placement:       domain.ExtensionPlacementDevice,
		Metadata:        map[string]any{"system.defaultEnabled": len(defaultEnabled) > 0 && defaultEnabled[0]},
		Modules: []domain.ModuleDefinition{
			{
				ID:          modID,
				ExtensionID: extID,
				Name:        domain.LocalizedText{Default: displayName},
				Description: domain.LocalizedText{Default: description},
				Type:        domain.ModuleTypeBuiltin,
				Version:     ver.String(),
				Runtime: &domain.RuntimeDefinition{
					Type:       domain.RuntimeTypeBuiltin,
					EntryPoint: runtimeID,
				},
				Contributions: []domain.ContributionDefinition{
					buildCharacterTabContribution(extID, modID, displayName, tabTitle, tabKey, runtimeID, order),
				},
				Placement: domain.ModulePlacementDevice,
				ProvidedCapabilities: func() []domain.ProvidedCapability {
					out := make([]domain.ProvidedCapability, 0, len(capabilities))
					for _, item := range capabilities {
						out = append(out, domain.ProvidedCapability{ID: item, Version: ver.String()})
					}
					return out
				}(),
			},
		},
		Compatibility: domain.ExtensionCompatibility{
			Platforms: []string{"windows", "linux", "darwin"},
		},
	}
	return Definition{
		Extension:         def,
		SystemManaged:     true,
		Required:          false,
		DisableAllowed:    true,
		BootstrapRevision: 1,
	}
}

func buildCharacterTabContribution(
	extID domain.ExtensionID,
	modID domain.ModuleID,
	displayName string,
	tabTitle string,
	tabKey string,
	runtimeID string,
	order int,
) domain.ContributionDefinition {
	contributionID := domain.ContributionID(string(extID) + ".character-tab")
	return domain.ContributionDefinition{
		ID:          contributionID,
		ModuleID:    modID,
		ExtensionID: extID,
		Kind:        domain.ContributionKindUIPage,
		Name:        domain.LocalizedText{Default: displayName},
		Description: domain.LocalizedText{Default: tabTitle},
		Definition: map[string]any{
			"contribution_id": string(contributionID),
			"extension_id":    string(extID),
			"module_id":       string(modID),
			"kind":            "schema_page",
			"slot": map[string]any{
				"slot_id":          "character.detail.tab",
				"contract_version": 1,
			},
			"contract_version": 1,
			"display": map[string]any{
				"title": map[string]any{"default": tabTitle},
				"icon":  tabKey,
			},
			"entry": map[string]any{
				"type":         "host_native",
				"path":         "builtin://" + string(extID),
				"runtime_id":   runtimeID,
				"content_hash": "builtin",
			},
			"sandbox": map[string]any{"type": "host_native"},
			"ordering": map[string]any{
				"priority": order,
			},
			"dispatch": map[string]any{
				"entry_key": tabKey,
				"cell_id":   tabKey,
			},
			"integrity": map[string]any{
				"definition_hash": "builtin-" + string(extID) + "-v1",
			},
		},
	}
}

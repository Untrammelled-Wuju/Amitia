package migration

import (
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func LegacySkillToTool(def extension.SkillDefinition) capability.ToolDefinition {
	namespace := extractNamespace(def.ID)
	name := extractToolName(def.ID)

	return capability.ToolDefinition{
		ID:        capability.BuildToolID(capability.ToolSourceLegacy, namespace, name),
		ModelName: def.ModelName,
		Source:    capability.ToolSourceLegacy,
		Name:      def.Name,
		Description: def.Description,
		Version:     def.Version,
		InputSchema:  json.RawMessage(append([]byte(nil), def.InputSchema...)),
		OutputSchema: json.RawMessage(append([]byte(nil), def.OutputSchema...)),
		Permissions:  mapCapabilities(def.Capabilities),
		RiskLevel:     capability.RiskLow,
		SideEffect:    mapSideEffect(def.HasSideEffects),
		Enabled:       def.Enabled,
		Compatible:    def.Compatible,
		Internal:      def.Internal,
		HasSideEffects: def.HasSideEffects,
		Idempotent:    def.Idempotent,
		Retryable:     def.Retryable,
		TimeoutMS:     def.TimeoutMS,
		Metadata: map[string]any{
			"legacy_id":     def.ID,
			"legacy_source": string(def.Source),
			"author":        def.Author,
			"license":       def.License,
		},
	}
}

func BuiltinSkillToTool(def extension.SkillDefinition) capability.ToolDefinition {
	namespace := extractNamespace(def.ID)
	name := extractToolName(def.ID)

	return capability.ToolDefinition{
		ID:        capability.BuildToolID(capability.ToolSourceBuiltin, namespace, name),
		ModelName: def.ModelName,
		Source:    capability.ToolSourceBuiltin,
		Name:      def.Name,
		Description: def.Description,
		Version:     def.Version,
		InputSchema:  json.RawMessage(append([]byte(nil), def.InputSchema...)),
		OutputSchema: json.RawMessage(append([]byte(nil), def.OutputSchema...)),
		Permissions:  mapCapabilities(def.Capabilities),
		RiskLevel:     capability.RiskLow,
		SideEffect:    mapSideEffect(def.HasSideEffects),
		Enabled:       def.Enabled,
		Compatible:    def.Compatible,
		Internal:      def.Internal,
		HasSideEffects: def.HasSideEffects,
		Idempotent:    def.Idempotent,
		Retryable:     def.Retryable,
		TimeoutMS:     def.TimeoutMS,
		Metadata: map[string]any{
			"legacy_id":     def.ID,
			"legacy_source": string(def.Source),
			"author":        def.Author,
			"license":       def.License,
		},
	}
}

func PluginSkillToTool(def extension.SkillDefinition, pluginID string) capability.ToolDefinition {
	name := extractToolName(def.ID)

	return capability.ToolDefinition{
		ID:        capability.BuildToolID(capability.ToolSourcePlugin, pluginID, name),
		ModelName: def.ModelName,
		Source:    capability.ToolSourcePlugin,
		Name:      def.Name,
		Description: def.Description,
		Version:     def.Version,
		ExtensionID: pluginID,
		InputSchema:  json.RawMessage(append([]byte(nil), def.InputSchema...)),
		OutputSchema: json.RawMessage(append([]byte(nil), def.OutputSchema...)),
		Permissions:  mapCapabilities(def.Capabilities),
		RiskLevel:     capability.RiskMedium,
		SideEffect:    mapSideEffect(def.HasSideEffects),
		Enabled:       def.Enabled,
		Compatible:    def.Compatible,
		HasSideEffects: def.HasSideEffects,
		Idempotent:    def.Idempotent,
		Retryable:     def.Retryable,
		TimeoutMS:     def.TimeoutMS,
		Metadata: map[string]any{
			"legacy_id":     def.ID,
			"legacy_source": string(def.Source),
			"plugin_id":     pluginID,
			"author":        def.Author,
			"license":       def.License,
		},
	}
}

func InternalSkillToTool(def extension.SkillDefinition) capability.ToolDefinition {
	namespace := "agent-skill"
	name := formatInternalToolName(def.Name)

	return capability.ToolDefinition{
		ID:        capability.BuildToolID(capability.ToolSourceInternal, namespace, name),
		ModelName: def.ModelName,
		Source:    capability.ToolSourceInternal,
		Name:      def.Name,
		Description: def.Description,
		Version:     "1.0.0",
		InputSchema:  json.RawMessage(append([]byte(nil), def.InputSchema...)),
		OutputSchema: json.RawMessage(append([]byte(nil), def.OutputSchema...)),
		Enabled:    true,
		Compatible: true,
		Internal:   true,
		HasSideEffects: false,
		Idempotent:     true,
		TimeoutMS:      def.TimeoutMS,
		Metadata: map[string]any{
			"legacy_id":     def.ID,
			"legacy_source": string(def.Source),
		},
	}
}

func extractNamespace(legacyID string) string {
	parts := strings.Split(legacyID, ".")
	if len(parts) >= 4 {
		return parts[2]
	}
	return "amitia"
}

func extractToolName(legacyID string) string {
	parts := strings.Split(legacyID, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return legacyID
}

func formatInternalToolName(raw string) string {
	name := strings.ToLower(raw)
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func mapCapabilities(caps []string) []capability.PermissionRequirement {
	result := make([]capability.PermissionRequirement, 0, len(caps))
	for _, c := range caps {
		risk := "low"
		if strings.Contains(c, "write") || strings.Contains(c, "manage") || strings.Contains(c, "send") || strings.Contains(c, "action") {
			risk = "medium"
		}
		if strings.Contains(c, "delete") || strings.Contains(c, "financial") {
			risk = "high"
		}
		result = append(result, capability.PermissionRequirement{
			Capability:  c,
			Description: "",
			Risk:        risk,
		})
	}
	return result
}

func mapSideEffect(hasSE bool) capability.SideEffectLevel {
	if hasSE {
		return capability.SideEffectWrite
	}
	return capability.SideEffectNone
}

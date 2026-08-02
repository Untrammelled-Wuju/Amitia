package migration

import (
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func MCPSkillToTool(def extension.SkillDefinition, serverID string) capability.ToolDefinition {
	prefix := "mcp:" + serverID
	toolName := extractMCPToolName(def.ModelName, serverID)

	return capability.ToolDefinition{
		ID:             capability.BuildToolID(capability.ToolSourceMCP, prefix, toolName),
		ModelName:      def.ModelName,
		Source:         capability.ToolSourceMCP,
		Name:           def.Name,
		Description:    def.Description,
		Version:        "1.0.0",
		InputSchema:    json.RawMessage(append([]byte(nil), def.InputSchema...)),
		OutputSchema:   json.RawMessage(append([]byte(nil), def.OutputSchema...)),
		Permissions:    mapMCPSideEffects(def.Capabilities, def.HasSideEffects),
		RiskLevel:      mcpRiskLevel(def.HasSideEffects, def.Capabilities),
		SideEffect:     mapMCPSideEffectLevel(def.HasSideEffects),
		Scope:          capability.ScopeRule{Type: "mcp_server", ID: serverID},
		Enabled:        def.Enabled,
		Compatible:     def.Compatible,
		HasSideEffects: def.HasSideEffects,
		Idempotent:     def.Idempotent,
		Retryable:      def.Retryable,
		TimeoutMS:      def.TimeoutMS,
		Metadata: map[string]any{
			"legacy_id":     def.ID,
			"mcp_server_id": serverID,
			"entry_name":    def.Entry.Name,
			"author":        def.Author,
			"license":       def.License,
		},
	}
}

func extractMCPToolName(modelName string, serverID string) string {
	prefix := "mcp_" + normalizeID(serverID) + "_"
	if strings.HasPrefix(modelName, prefix) {
		return modelName[len(prefix):]
	}
	return modelName
}

func normalizeID(id string) string {
	result := strings.Builder{}
	for _, r := range strings.ToLower(strings.TrimSpace(id)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else if result.Len() > 0 && !strings.HasSuffix(result.String(), "_") {
			result.WriteByte('_')
		}
	}
	return strings.Trim(result.String(), "_")
}

func mapMCPSideEffects(caps []string, hasSE bool) []capability.PermissionRequirement {
	if !hasSE {
		return nil
	}
	result := make([]capability.PermissionRequirement, 0, len(caps))
	for _, c := range caps {
		if c == "mcp.invoke" || strings.HasPrefix(c, "mcp.server.") || strings.HasPrefix(c, "mcp.tool.") {
			continue
		}
		risk := "low"
		if strings.Contains(c, "write") || strings.Contains(c, "send") || strings.Contains(c, "manage") || strings.Contains(c, "publish") {
			risk = "medium"
		}
		if strings.Contains(c, "delete") || strings.Contains(c, "remove") || strings.Contains(c, "financial") || strings.Contains(c, "transfer") || strings.Contains(c, "purchase") || strings.Contains(c, "pay") {
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

func mapMCPSideEffectLevel(hasSE bool) capability.SideEffectLevel {
	if hasSE {
		return capability.SideEffectWrite
	}
	return capability.SideEffectReadOnly
}

func mcpRiskLevel(hasSE bool, caps []string) capability.RiskLevel {
	for _, c := range caps {
		if strings.Contains(c, "financial") || strings.Contains(c, "delete") || strings.Contains(c, "remove") {
			return capability.RiskHigh
		}
	}
	if hasSE {
		return capability.RiskMedium
	}
	return capability.RiskLow
}

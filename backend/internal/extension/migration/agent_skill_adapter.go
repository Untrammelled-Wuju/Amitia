package migration

import (
	"github.com/u-ai/backend/internal/extension"
	kas "github.com/u-ai/backend/internal/extension/kernel/agent_skill"
)

func AgentSkillToDefinition(old extension.AgentSkillDefinition) kas.AgentSkillDefinition {
	resources := make([]kas.SkillResourceDescriptor, 0, len(old.Resources))
	for _, r := range old.Resources {
		resources = append(resources, kas.SkillResourceDescriptor{
			Path:         r.Path,
			Kind:         kas.SkillResourceKind(r.Kind),
			MIMEType:     r.MIMEType,
			Size:         r.Size,
			TextReadable: r.TextReadable,
		})
	}

	toolMappings := make([]map[string]any, 0, len(old.ToolMappings))
	for _, tm := range old.ToolMappings {
		toolMappings = append(toolMappings, map[string]any{
			"sourceTool":    tm.SourceTool,
			"targetSkillId": tm.TargetSkillID,
			"status":        tm.Status,
			"reason":        tm.Reason,
		})
	}

	metadata := make(map[string]any)
	for k, v := range old.Metadata {
		metadata[k] = v
	}
	metadata["artifact_id"] = old.ArtifactID
	metadata["content_hash"] = old.ContentHash
	metadata["source"] = string(old.Source)

	tokenBudget := 0
	if len(old.Body) > 0 {
		tokenBudget = len(old.Body) / 4
		if tokenBudget > 32768 {
			tokenBudget = 32768
		}
	}

	mode := kas.ActivationManual
	return kas.AgentSkillDefinition{
		ID:          old.ExtensionID,
		ExtensionID: old.ExtensionID,
		Name:        old.Name,
		Description: old.Description,
		DisplayName: old.DisplayName,
		Instructions: kas.SkillInstructionRef{
			Text:       old.Body,
			TokenCount: tokenBudget,
		},
		Activation: kas.ActivationRule{
			Mode:     mode,
			Priority: 0,
		},
		Resources:  resources,
		TokenPolicy: kas.SkillTokenPolicy{
			MaxInstructionTokens: tokenBudget,
		},
		Scope:      kas.AgentSkillScope(old.Scope),
		ScopeID:    old.ScopeID,
		Enabled:    old.Enabled,
		Compatible: old.CompatibilityStatus == extension.AgentSkillCompatible || old.CompatibilityStatus == extension.AgentSkillCompatibleWarnings,
		Source:     string(old.Source),
		Version:    old.Compatibility,
		License:    old.License,
		Compatibility: kas.SkillCompatibility{
			Status:   string(old.CompatibilityStatus),
		},
		Metadata:      metadata,
	}
}

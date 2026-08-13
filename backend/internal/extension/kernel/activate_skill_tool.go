package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
)

const ActivateSkillToolName = "activate_skill"

func buildActivateSkillTool(names []string) tool.Tool {
	return tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        ActivateSkillToolName,
			Description: "Activate an Agent Skill by name. Use this tool when you need specialized capabilities from an available skill.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"name": {
						Type:        "string",
						Description: "The name of the skill to activate.",
						Enum:        names,
					},
				},
				Required: []string{"name"},
			},
		},
	}
}

type activateSkillInput struct {
	Name string `json:"name"`
}

func (f *ToolFacade) handleActivateSkill(ctx context.Context, input json.RawMessage, scope LegacyScope) (LegacyToolResult, error) {
	var req activateSkillInput
	if err := json.Unmarshal(input, &req); err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("invalid activate_skill input: %v", err),
			Error:       &LegacyToolError{Code: "INVALID_INPUT", Message: err.Error()},
		}, err
	}

	if strings.TrimSpace(req.Name) == "" {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: "skill name is required",
			Error:       &LegacyToolError{Code: "INVALID_INPUT", Message: "skill name is required"},
		}, fmt.Errorf("skill name is required")
	}

	result, err := f.agentSkillBackend.Activate(ctx, scope, req.Name, true)
	if err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("failed to activate skill %q: %v", req.Name, err),
			Error:       &LegacyToolError{Code: "ACTIVATION_FAILED", Message: err.Error()},
		}, err
	}

	output, _ := json.Marshal(map[string]interface{}{
		"activationId":        result.ActivationID,
		"extensionId":         result.ExtensionID,
		"name":                result.Name,
		"scope":               result.Scope,
		"compatibilityStatus": result.CompatibilityStatus,
		"tokens":              result.Tokens,
		"contentHash":         result.ContentHash,
		"explicit":            result.Explicit,
		"status":              "activated",
	})

	return LegacyToolResult{
		RunID:       result.ActivationID,
		Status:      "SUCCESS",
		Output:      output,
		VisibleText: fmt.Sprintf("Agent Skill activated: %s", result.Name),
	}, nil
}

func (f *ToolFacade) resolveVisibleSkillNames(ctx context.Context, scope LegacyScope) ([]string, error) {
	if f.agentSkillBackend == nil {
		return nil, nil
	}
	catalog, err := f.agentSkillBackend.ResolveCatalog(ctx, scope)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(catalog))
	for _, entry := range catalog {
		names = append(names, entry.Name)
	}
	return names, nil
}

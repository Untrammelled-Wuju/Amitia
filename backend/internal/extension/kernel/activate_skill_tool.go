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

func (f *ToolFacade) handleActivateSkill(ctx context.Context, input json.RawMessage, scope InvocationScope) (ToolDispatchResult, error) {
	var req activateSkillInput
	if err := json.Unmarshal(input, &req); err != nil {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("invalid activate_skill input: %v", err),
			Error:       &ToolDispatchError{Code: "INVALID_INPUT", Message: err.Error()},
		}, err
	}

	if strings.TrimSpace(req.Name) == "" {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: "skill name is required",
			Error:       &ToolDispatchError{Code: "INVALID_INPUT", Message: "skill name is required"},
		}, fmt.Errorf("skill name is required")
	}

	if f.agentSkillRuntime == nil {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: "agent skill runtime unavailable",
			Error:       &ToolDispatchError{Code: "ACTIVATION_UNAVAILABLE", Message: "agent skill runtime unavailable"},
		}, fmt.Errorf("agent skill runtime unavailable")
	}
	result, err := f.agentSkillRuntime.Activate(ctx, scope, req.Name, true)
	if err != nil {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("failed to activate skill %q: %v", req.Name, err),
			Error:       &ToolDispatchError{Code: "ACTIVATION_FAILED", Message: err.Error()},
		}, err
	}

	output, _ := json.Marshal(map[string]interface{}{
		"activationId":        result.ActivationID,
		"extensionId":         result.ExtensionID,
		"name":                result.Name,
		"scope":               result.Scope,
		"compatibilityStatus": result.CompatibilityStatus,
		"tokens":              result.BodyTokens,
		"contentHash":         result.ContentHash,
		"explicit":            result.Explicit,
		"status":              "activated",
	})

	return ToolDispatchResult{
		RunID:       result.ActivationID,
		Status:      "SUCCESS",
		Output:      output,
		VisibleText: fmt.Sprintf("Agent Skill activated: %s", result.Name),
	}, nil
}

func (f *ToolFacade) resolveVisibleSkillNames(ctx context.Context, scope InvocationScope) ([]string, error) {
	if f.agentSkillRuntime == nil {
		return nil, nil
	}
	catalog := f.agentSkillRuntime.Available(ctx, scope)
	names := make([]string, 0, len(catalog))
	for _, entry := range catalog {
		names = append(names, entry.Name)
	}
	return names, nil
}

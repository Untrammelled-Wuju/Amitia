package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

const userWorkflowToolPrefix = "workflow/user/"

var workflowModelNameInvalid = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func UserWorkflowToolID(workflowID string) string {
	return userWorkflowToolPrefix + strings.TrimSpace(workflowID)
}

// SyncUserWorkflowAgentTool mirrors a user workflow into the canonical ToolRegistry.
// It is intentionally idempotent and removes the model tool whenever the workflow is
// disabled or no longer callable by the agent.
func SyncUserWorkflowAgentTool(ctx context.Context, registry *capability.ToolRegistry, def workflow.WorkflowDefinition) error {
	if registry == nil || strings.TrimSpace(def.ID) == "" {
		return nil
	}
	toolID := UserWorkflowToolID(def.ID)
	if def.Source != "user" || !def.Enabled || !def.CallableByAgent {
		if _, ok := registry.Get(ctx, toolID); ok {
			return registry.Unregister(ctx, toolID)
		}
		return nil
	}

	toolDef, err := BuildUserWorkflowAgentTool(def)
	if err != nil {
		return err
	}
	return registry.Replace(ctx, toolDef)
}

func RemoveUserWorkflowAgentTool(ctx context.Context, registry *capability.ToolRegistry, workflowID string) error {
	if registry == nil || strings.TrimSpace(workflowID) == "" {
		return nil
	}
	toolID := UserWorkflowToolID(workflowID)
	if _, ok := registry.Get(ctx, toolID); !ok {
		return nil
	}
	return registry.Unregister(ctx, toolID)
}

func BuildUserWorkflowAgentTool(def workflow.WorkflowDefinition) (capability.ToolDefinition, error) {
	if def.Source != "user" {
		return capability.ToolDefinition{}, fmt.Errorf("workflow %s is not a user workflow", def.ID)
	}
	ownerUserID := ""
	if def.Metadata != nil {
		ownerUserID = strings.TrimSpace(fmt.Sprint(def.Metadata["ownerUserId"]))
	}
	if ownerUserID == "" {
		return capability.ToolDefinition{}, fmt.Errorf("workflow %s has no owner user", def.ID)
	}

	inputSchema := normalizedWorkflowObjectSchema(def.InputSchema)
	outputSchema := normalizedWorkflowOutputSchema(def.OutputSchema)
	modelName := normalizeWorkflowModelName(def.AgentTool.Name)
	if modelName == "" {
		modelName = normalizeWorkflowModelName("workflow_" + def.Name)
	}
	if modelName == "" {
		modelName = normalizeWorkflowModelName("workflow_" + def.ID)
	}
	description := strings.TrimSpace(def.AgentTool.Description)
	if description == "" {
		description = strings.TrimSpace(def.Description)
	}
	if description == "" {
		description = "Run the user workflow " + def.Name
	}

	permissions := make([]capability.PermissionRequirement, 0, len(def.Permissions))
	for _, permission := range def.Permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		permissions = append(permissions, capability.PermissionRequirement{Capability: permission})
	}

	risk := capability.RiskLow
	sideEffect := capability.SideEffectNone
	if def.HasSideEffects {
		risk = capability.RiskMedium
		sideEffect = capability.SideEffectExternal
	}
	timeout := time.Duration(def.Limits.MaxExecutionDurationMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	maxConcurrency := def.Limits.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	}

	return capability.ToolDefinition{
		ID:             UserWorkflowToolID(def.ID),
		ModelName:      modelName,
		Source:         capability.ToolSourceWorkflow,
		Name:           def.Name,
		Description:    description,
		Version:        def.Version,
		InputSchema:    inputSchema,
		OutputSchema:   outputSchema,
		Permissions:    permissions,
		RiskLevel:      risk,
		SideEffect:     sideEffect,
		Enabled:        true,
		Compatible:     true,
		Internal:       false,
		HasSideEffects: def.HasSideEffects,
		Idempotent:     def.Idempotent,
		Retryable:      def.Idempotent,
		TimeoutMS:      timeout.Milliseconds(),
		Metadata: map[string]any{
			"ownerUserId":  ownerUserID,
			"workflowId":   def.ID,
			"userWorkflow": true,
		},
		ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: def.DefinitionHash},
		ModelExposure: capability.ModelExposureRule{
			ExposedByDefault: true,
			MaxPromptTokens:  4096,
			Priority:         50,
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:         timeout,
			MaxConcurrency:  maxConcurrency,
			Idempotent:      def.Idempotent,
			AllowBackground: true,
			MaxDepth:        def.Limits.MaxSkillCallDepth,
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeWorkflow,
			RuntimeID:   def.ID,
		},
	}, nil
}

func normalizeWorkflowModelName(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = workflowModelNameInvalid.ReplaceAllString(raw, "_")
	raw = strings.Trim(raw, "_-")
	if len(raw) > 64 {
		raw = strings.TrimRight(raw[:64], "_-")
	}
	return raw
}

func normalizedWorkflowObjectSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	var schema map[string]any
	if json.Unmarshal(raw, &schema) != nil {
		return json.RawMessage(`{"type":"object","properties":{}}`)
	}
	if schema["type"] == nil {
		schema["type"] = "object"
	}
	if schema["properties"] == nil {
		schema["properties"] = map[string]any{}
	}
	out, _ := json.Marshal(schema)
	return out
}

func normalizedWorkflowOutputSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{}`)
	}
	if json.Valid(raw) {
		return raw
	}
	return json.RawMessage(`{}`)
}

func workflowToolOwnerUserID(def capability.ToolDefinition) string {
	if def.Source != capability.ToolSourceWorkflow || def.Metadata == nil {
		return ""
	}
	if flag, ok := def.Metadata["userWorkflow"].(bool); !ok || !flag {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(def.Metadata["ownerUserId"]))
}

func workflowToolAllowedForUser(def capability.ToolDefinition, userID string) bool {
	owner := workflowToolOwnerUserID(def)
	return owner == "" || (strings.TrimSpace(userID) != "" && owner == strings.TrimSpace(userID))
}

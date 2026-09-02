package kernel

import (
	"context"
	"encoding/json"
	"time"

	androidinteraction "github.com/u-ai/backend/internal/androidnative/interaction"
	androiduitree "github.com/u-ai/backend/internal/androidnative/uitree"
	_ "github.com/u-ai/backend/internal/androiduiagent"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func registerAndroidUIAgentTool(ctx context.Context, registry *capability.ToolRegistry) error {
	if registry == nil {
		return nil
	}
	definition := capability.ToolDefinition{
		ID:          "android.ui.agent.run",
		ModelName:   "android.ui.agent.run",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Android UI Agent",
		Description: "Run a bounded autonomous Android UI sub-agent. It repeatedly captures a structured UI tree, asks the configured model for exactly one constrained next action, executes only typed Android capabilities, re-observes the screen, and replans until the goal is complete or the step/timeout budget is exhausted.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["goal"],
			"additionalProperties":false,
			"properties":{
				"goal":{"type":"string","minLength":1,"maxLength":4096},
				"maxSteps":{"type":"integer","minimum":1,"maximum":30,"default":12},
				"timeoutMs":{"type":"integer","minimum":5000,"maximum":180000,"default":90000},
				"allowedApps":{"type":"array","maxItems":32,"items":{"type":"string","minLength":1,"maxLength":255}},
				"allowAdbFallback":{"type":"boolean","default":false},
				"allowRootFallback":{"type":"boolean","default":false}
			}
		}`),
		OutputSchema: json.RawMessage(`{
			"type":"object",
			"required":["success","steps","stepCount"],
			"properties":{
				"success":{"type":"boolean"},
				"result":{"type":"string"},
				"finalState":{"type":"string"},
				"steps":{"type":"array"},
				"stepCount":{"type":"integer"}
			}
		}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: androiduitree.PermissionUITreeRead, Risk: "medium"},
			{Capability: androidinteraction.PermissionInteractionReadVisual, Risk: "medium"},
			{Capability: androidinteraction.PermissionInteractionClick, Risk: "high"},
			{Capability: androidinteraction.PermissionInteractionInput, Risk: "high"},
			{Capability: androidinteraction.PermissionInteractionGesture, Risk: "medium"},
			{Capability: "android.interaction.global", Risk: "medium"},
			{Capability: "android.app.launch", Risk: "medium"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectWrite,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		Enabled:        true,
		Compatible:     true,
		TimeoutMS:      180000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "android-ui-agent-v2-reliability"},
		ModelExposure: capability.ModelExposureRule{
			ExposedByDefault: true,
			Categories:       []string{"android", "automation", "agent"},
			Priority:         75,
		},
		Metadata: map[string]any{
			"autonomousAgent":  true,
			"bounded":          true,
			"observer":         "android.ui_tree.snapshot",
			"planner":          "configured_llm",
			"typedActionsOnly": true,
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:          180 * time.Second,
			MaxConcurrency:   1,
			Idempotent:       false,
			ApprovalRequired: true,
			AllowBackground:  false,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 256 * 1024,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBuiltin,
			RuntimeID:   "android_ui_agent",
			HandlerName: "android.ui.agent.run",
		},
	}
	return registry.Register(ctx, definition)
}

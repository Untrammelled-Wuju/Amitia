package kernel

import (
	"context"
	"encoding/json"
	"time"

	_ "github.com/u-ai/backend/internal/browseragent"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func registerBrowserAgentTool(ctx context.Context, registry *capability.ToolRegistry) error {
	if registry == nil {
		return nil
	}
	definition := capability.ToolDefinition{
		ID:          "browser.agent.run",
		ModelName:   "browser.agent.run",
		Source:      capability.ToolSourceBuiltin,
		Name:        "Browser Agent",
		Description: "Run a bounded autonomous browser sub-agent that repeatedly captures DOM state, plans exactly one constrained action, executes only typed browser capabilities, re-observes, and replans until completion or the step/timeout budget is exhausted.",
		InputSchema: json.RawMessage(`{
			"type":"object",
			"required":["goal"],
			"additionalProperties":false,
			"properties":{
				"goal":{"type":"string","minLength":1,"maxLength":4096},
				"startUrl":{"type":"string","maxLength":4096},
				"sessionId":{"type":"string","maxLength":256},
				"tabId":{"type":"string","maxLength":256},
				"maxSteps":{"type":"integer","minimum":1,"maximum":30,"default":12},
				"timeoutMs":{"type":"integer","minimum":5000,"maximum":180000,"default":90000},
				"allowedHosts":{"type":"array","maxItems":64,"items":{"type":"string","minLength":1,"maxLength":253}},
				"keepSession":{"type":"boolean","default":false}
			}
		}`),
		OutputSchema: json.RawMessage(`{
			"type":"object",
			"required":["success","steps","stepCount"],
			"properties":{
				"success":{"type":"boolean"},
				"result":{"type":"string"},
				"finalState":{"type":"string"},
				"sessionId":{"type":"string"},
				"tabId":{"type":"string"},
				"steps":{"type":"array"},
				"stepCount":{"type":"integer"}
			}
		}`),
		Permissions: []capability.PermissionRequirement{
			{Capability: "browser.runtime", Risk: "high"},
			{Capability: "browser.session.create", Risk: "medium"},
			{Capability: "browser.session.manage", Risk: "medium"},
			{Capability: "browser.tab.manage", Risk: "medium"},
			{Capability: "browser.navigate", Risk: "high"},
			{Capability: "browser.dom.read", Risk: "medium"},
			{Capability: "browser.interact", Risk: "high"},
		},
		RiskLevel:      capability.RiskHigh,
		SideEffect:     capability.SideEffectExternal,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		Enabled:        true,
		Compatible:     true,
		TimeoutMS:      180000,
		ToolVersion:    capability.ToolVersion{SchemaVersion: 1, Revision: "browser-agent-v1"},
		ModelExposure: capability.ModelExposureRule{
			ExposedByDefault: true,
			Categories:       []string{"browser", "automation", "agent"},
			Priority:         74,
		},
		Metadata: map[string]any{
			"autonomousAgent":  true,
			"bounded":          true,
			"observer":         "browser_dom_snapshot",
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
			RuntimeID:   "browser_agent",
			HandlerName: "browser.agent.run",
		},
	}
	return registry.Register(ctx, definition)
}

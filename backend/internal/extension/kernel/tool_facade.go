package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
)

type ToolFacadeConfig struct {
	PreferKernel bool
	FallbackOnError bool
}

func DefaultToolFacadeConfig() ToolFacadeConfig {
	return ToolFacadeConfig{
		PreferKernel:   true,
		FallbackOnError: false,
	}
}

type ToolFacade struct {
	toolRegistry    *capability.ToolRegistry
	executionKernel *execution.ExecutionPipeline
	legacy          LegacyToolDispatcher
	hookService     *hook.Service
	counters        *ToolFacadeCounters
	config          ToolFacadeConfig
}

func NewToolFacade(toolRegistry *capability.ToolRegistry, executionKernel *execution.ExecutionPipeline, legacy LegacyToolDispatcher, config ToolFacadeConfig) *ToolFacade {
	return &ToolFacade{
		toolRegistry:    toolRegistry,
		executionKernel: executionKernel,
		legacy:          legacy,
		counters:        NewToolFacadeCounters(),
		config:          config,
	}
}

func (f *ToolFacade) Counters() *ToolFacadeCounters {
	return f.counters
}

func (f *ToolFacade) SetLegacyDispatcher(dispatcher LegacyToolDispatcher) {
	f.legacy = dispatcher
}

func (f *ToolFacade) SetHookService(svc *hook.Service) {
	f.hookService = svc
}

func (f *ToolFacade) PrepareAgentSkillPrompt(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string) {
	f.counters.IncPrepareAgentSkillPrompt()
	if f.hookService != nil {
		return "", nil, nil
	}
	if f.legacy != nil {
		f.counters.IncLegacyFallback("prepare_agent_skill_prompt")
		return f.legacy.PrepareAgentSkillPrompt(ctx, scope, message)
	}
	return "", nil, nil
}

func (f *ToolFacade) EndAgentSkillRound(scope LegacyScope) {
	f.counters.IncEndAgentSkillRound()
	if f.hookService != nil {
		return
	}
	if f.legacy != nil {
		f.counters.IncLegacyFallback("end_agent_skill_round")
		f.legacy.EndAgentSkillRound(scope)
	}
}

func (f *ToolFacade) BeforePrompt(ctx context.Context, scope LegacyScope) []LegacyContextContribution {
	f.counters.IncBeforePrompt()
	if f.hookService != nil && f.hookService.Integrator != nil {
		hookCtx := f.buildHookContext(scope)
		payload := f.buildBeforePromptPayload(scope)
		result, blocked, err := f.hookService.Integrator.InvokePromptBeforeAssemble(ctx, payload, hookCtx)
		if err != nil {
			f.counters.IncPipelineFailure("before_prompt_hook")
		}
		if blocked {
			return nil
		}
		return f.parseContextContributions(result)
	}
	if f.legacy != nil {
		f.counters.IncLegacyFallback("before_prompt")
		return f.legacy.BeforePrompt(ctx, scope)
	}
	return nil
}

func (f *ToolFacade) ModelTools(ctx context.Context, scope LegacyScope) ([]tool.Tool, error) {
	f.counters.IncModelTools()
	if f.config.PreferKernel && f.toolRegistry != nil {
		tools, err := f.buildKernelModelTools(ctx, scope)
		if err == nil && len(tools) > 0 {
			return tools, nil
		}
		if err != nil {
			f.counters.IncPipelineFailure("model_tools_build")
		}
		if !f.config.FallbackOnError && err != nil {
			return nil, err
		}
	}
	if f.legacy != nil {
		f.counters.IncLegacyFallback("model_tools")
		return f.legacy.ModelTools(ctx, scope)
	}
	return nil, nil
}

func (f *ToolFacade) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope LegacyScope, idempotencyKey string) (LegacyToolResult, bool) {
	f.counters.IncExecuteModelTool()
	if f.config.PreferKernel && f.toolRegistry != nil {
		def, ok := f.toolRegistry.GetByModelName(ctx, modelName)
		if !ok {
			def, ok = f.toolRegistry.Get(ctx, modelName)
		}
		if ok {
			f.counters.IncPipelineExecution()
			result := f.executeKernelTool(ctx, def, input, scope, idempotencyKey)
			if result.Status == "success" || !f.config.FallbackOnError {
				return result, true
			}
			f.counters.IncPipelineFailure("execute_kernel_tool")
		}
	}
	if f.legacy != nil {
		f.counters.IncLegacyFallback("execute_model_tool")
		return f.legacy.ExecuteModelTool(ctx, modelName, input, scope, idempotencyKey)
	}
	return LegacyToolResult{Status: "FAILED", VisibleText: "tool runtime not configured", Error: &LegacyToolError{Code: "TOOL_RUNTIME_UNAVAILABLE"}}, false
}

func (f *ToolFacade) AfterReply(scope LegacyScope, reply LegacyReplyView) bool {
	f.counters.IncAfterReply()
	if f.hookService != nil && f.hookService.Integrator != nil {
		hookCtx := f.buildHookContext(scope)
		payload := f.buildAfterReplyPayload(reply)
		_, blocked, err := f.hookService.Integrator.InvokeModelAfterResponse(context.Background(), payload, hookCtx)
		if err != nil {
			f.counters.IncPipelineFailure("after_reply_hook")
		}
		return !blocked
	}
	if f.legacy != nil {
		f.counters.IncLegacyFallback("after_reply")
		return f.legacy.AfterReply(scope, reply)
	}
	return false
}

func (f *ToolFacade) buildHookContext(scope LegacyScope) hook.HookContextSnapshot {
	var charID *string
	if scope.CharacterID != "" {
		c := scope.CharacterID
		charID = &c
	}
	var convID *string
	if scope.ConversationID != "" {
		c := scope.ConversationID
		convID = &c
	}
	return hook.HookContextSnapshot{
		TraceID:        scope.TraceID,
		OperationID:    scope.RequestID,
		InvocationID:   scope.ToolCallID,
		CharacterID:    charID,
		ConversationID: convID,
		Platform:       scope.Channel,
		Timestamp:      time.Now().UTC(),
		Depth:          0,
	}
}

func (f *ToolFacade) buildBeforePromptPayload(scope LegacyScope) json.RawMessage {
	payload := map[string]interface{}{
		"sections": map[string]interface{}{},
		"context": map[string]interface{}{
			"userId":         scope.UserID,
			"characterId":    scope.CharacterID,
			"conversationId": scope.ConversationID,
			"channel":        scope.Channel,
			"sessionId":      scope.SessionID,
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func (f *ToolFacade) buildAfterReplyPayload(reply LegacyReplyView) json.RawMessage {
	payload := map[string]interface{}{
		"response": map[string]interface{}{
			"content":      reply.Content,
			"finishReason": "stop",
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func (f *ToolFacade) parseContextContributions(result json.RawMessage) []LegacyContextContribution {
	if len(result) == 0 {
		return nil
	}
	var parsed struct {
		Decision string                 `json:"decision"`
		Patch    []map[string]any       `json:"patch"`
		Sections map[string]interface{} `json:"sections"`
	}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return nil
	}
	contributions := make([]LegacyContextContribution, 0)
	if parsed.Sections != nil {
		if extCtx, ok := parsed.Sections["extension_context"].(map[string]interface{}); ok {
			for source, val := range extCtx {
				if content, ok := val.(string); ok {
					contributions = append(contributions, LegacyContextContribution{
						Source:  source,
						Content: content,
					})
				} else if obj, ok := val.(map[string]interface{}); ok {
					if content, ok := obj["content"].(string); ok {
						priority := 0
						if p, ok := obj["priority"].(float64); ok {
							priority = int(p)
						}
						contributions = append(contributions, LegacyContextContribution{
							Source:   source,
							Content:  content,
							Priority: priority,
						})
					}
				}
			}
		}
	}
	return contributions
}

func (f *ToolFacade) buildKernelModelTools(ctx context.Context, scope LegacyScope) ([]tool.Tool, error) {
	defs := f.toolRegistry.List(ctx, capability.ToolFilter{Enabled: boolPtr(true)})
	tools := make([]tool.Tool, 0, len(defs))
	for _, def := range defs {
		if !def.Enabled {
			continue
		}
		if def.ModelName == "" {
			continue
		}
		params, err := schemaToParameters(def.InputSchema)
		if err != nil {
			continue
		}
		tools = append(tools, tool.Tool{
			Type: "function",
			Function: tool.Function{
				Name:        def.ModelName,
				Description: def.Description,
				Parameters:  params,
			},
		})
	}
	return tools, nil
}

func (f *ToolFacade) executeKernelTool(ctx context.Context, def capability.ToolDefinition, input json.RawMessage, scope LegacyScope, idempotencyKey string) LegacyToolResult {
	if f.executionKernel == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "execution kernel not configured", Error: &LegacyToolError{Code: "EXECUTION_KERNEL_UNAVAILABLE"}}
	}
	invocation := capability.ToolInvocationContext{
		InvocationID:   fmt.Sprintf("toolfacade-%d-%d", time.Now().UnixNano(), f.counters.Snapshot()["pipeline_executions"]),
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		ExtensionID:    def.ExtensionID,
		ModuleID:       def.ModuleID,
		Source:         capability.InvocationSourceModel,
		IdempotencyKey: idempotencyKey,
		TraceID:        scope.TraceID,
	}
	req := execution.ToolExecutionRequest{
		ToolID:     capability.CapabilityID(def.ID),
		Input:      input,
		Invocation: invocation,
	}
	result := f.executionKernel.Execute(ctx, req)
	return unifiedResultToLegacy(result, invocation.InvocationID)
}

func unifiedResultToLegacy(result capability.UnifiedToolResult, invocationID string) LegacyToolResult {
	legacy := LegacyToolResult{
		RunID:      invocationID,
		Status:     string(result.Status),
		Output:     result.Structured,
		DurationMS: 0,
	}
	if len(result.Content) > 0 {
		text := ""
		for _, c := range result.Content {
			if c.Type == capability.ToolContentText && c.Text != "" {
				if text == "" {
					text = c.Text
				} else {
					text += "\n" + c.Text
				}
			}
		}
		legacy.VisibleText = text
	}
	if result.Error != nil {
		legacy.Error = &LegacyToolError{
			Code:      result.Error.Code,
			Message:   result.Error.Message,
			Retryable: result.Error.Retryable,
		}
	}
	return legacy
}

func schemaToParameters(raw json.RawMessage) (tool.Parameters, error) {
	var params tool.Parameters
	if len(raw) == 0 {
		return params, fmt.Errorf("empty schema")
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return params, err
	}
	return params, nil
}

func boolPtr(v bool) *bool {
	return &v
}

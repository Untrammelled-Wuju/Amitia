package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
)

type ToolFacadeConfig struct {
	PreferKernel bool
	FallbackOnError bool
}

func DefaultToolFacadeConfig() ToolFacadeConfig {
	return ToolFacadeConfig{
		PreferKernel: true,
		FallbackOnError: true,
	}
}

type ToolFacade struct {
	toolRegistry    *capability.ToolRegistry
	executionKernel *execution.ExecutionPipeline
	legacy          LegacyToolDispatcher
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

func (f *ToolFacade) PrepareAgentSkillPrompt(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string) {
	f.counters.IncPrepareAgentSkillPrompt()
	if f.legacy != nil {
		f.counters.IncLegacyFallback("prepare_agent_skill_prompt")
		return f.legacy.PrepareAgentSkillPrompt(ctx, scope, message)
	}
	return "", nil, nil
}

func (f *ToolFacade) EndAgentSkillRound(scope LegacyScope) {
	f.counters.IncEndAgentSkillRound()
	if f.legacy != nil {
		f.counters.IncLegacyFallback("end_agent_skill_round")
		f.legacy.EndAgentSkillRound(scope)
	}
}

func (f *ToolFacade) BeforePrompt(ctx context.Context, scope LegacyScope) []LegacyContextContribution {
	f.counters.IncBeforePrompt()
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
	if f.legacy != nil {
		f.counters.IncLegacyFallback("after_reply")
		return f.legacy.AfterReply(scope, reply)
	}
	return false
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

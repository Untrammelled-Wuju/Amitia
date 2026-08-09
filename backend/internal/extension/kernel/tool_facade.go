package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
)

type ToolFacadeConfig struct {
	PreferKernel    bool
	FallbackOnError bool
}

func DefaultToolFacadeConfig() ToolFacadeConfig {
	return ToolFacadeConfig{
		PreferKernel:    true,
		FallbackOnError: false,
	}
}

type ToolFacade struct {
	toolRegistry      *capability.ToolRegistry
	executionKernel   *execution.ExecutionPipeline
	hookService       *hook.Service
	agentSkillCatalog *agent_skill.AgentSkillCatalog
	activationService *agent_skill.ActivationService
	counters          *ToolFacadeCounters
	config            ToolFacadeConfig
}

func NewToolFacade(toolRegistry *capability.ToolRegistry, executionKernel *execution.ExecutionPipeline, args ...any) *ToolFacade {
	config := DefaultToolFacadeConfig()
	for _, arg := range args {
		if value, ok := arg.(ToolFacadeConfig); ok {
			config = value
		}
	}
	return &ToolFacade{
		toolRegistry:    toolRegistry,
		executionKernel: executionKernel,
		counters:        NewToolFacadeCounters(),
		config:          config,
	}
}

func (f *ToolFacade) Counters() *ToolFacadeCounters {
	return f.counters
}

func (f *ToolFacade) SetHookService(svc *hook.Service) {
	f.hookService = svc
}

func (f *ToolFacade) SetAgentSkillCatalog(catalog *agent_skill.AgentSkillCatalog) {
	f.agentSkillCatalog = catalog
	if catalog != nil {
		f.activationService = agent_skill.NewActivationService(catalog)
	}
}

func (f *ToolFacade) PrepareAgentSkillPrompt(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string) {
	f.counters.IncPrepareAgentSkillPrompt()
	if f.agentSkillCatalog == nil {
		return "", nil, nil
	}
	return f.buildAgentSkillPrompt(ctx, scope, message)
}

func (f *ToolFacade) EndAgentSkillRound(scope LegacyScope) {
	f.counters.IncEndAgentSkillRound()
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
	return nil
}

func (f *ToolFacade) ModelTools(ctx context.Context, scope LegacyScope) ([]tool.Tool, error) {
	f.counters.IncModelTools()
	if f.toolRegistry == nil {
		return nil, nil
	}
	return f.buildKernelModelTools(ctx, scope)
}

type ResolvedToolReference struct {
	ID          capability.CapabilityID
	ModelName   string
	ExtensionID string
	ModuleID    string
	Generation  int64
}

func (f *ToolFacade) ResolveModelTool(modelName string) (ResolvedToolReference, error) {
	if f.toolRegistry == nil {
		return ResolvedToolReference{}, fmt.Errorf("tool registry not configured")
	}
	def, ok := f.toolRegistry.GetByModelName(context.Background(), modelName)
	if !ok {
		return ResolvedToolReference{}, fmt.Errorf("tool not found: %s", modelName)
	}
	return ResolvedToolReference{
		ID:          capability.CapabilityID(def.ID),
		ModelName:   def.ModelName,
		ExtensionID: def.ExtensionID,
		ModuleID:    def.ModuleID,
		Generation:  0,
	}, nil
}

func (f *ToolFacade) ExecuteTool(ctx context.Context, toolID capability.CapabilityID, input json.RawMessage, scope LegacyScope, externalCallID string, idempotencyKey string) (LegacyToolResult, bool) {
	f.counters.IncExecuteModelTool()
	if f.toolRegistry == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "tool registry not configured", Error: &LegacyToolError{Code: "TOOL_REGISTRY_UNAVAILABLE"}}, false
	}
	def, ok := f.toolRegistry.Get(ctx, string(toolID))
	if !ok {
		return LegacyToolResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", toolID), Error: &LegacyToolError{Code: "TOOL_NOT_FOUND", Message: string(toolID)}}, false
	}
	f.counters.IncPipelineExecution()
	return f.executeResolvedTool(ctx, def, input, scope, externalCallID, idempotencyKey), true
}

func (f *ToolFacade) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope LegacyScope, idempotencyKey string) (LegacyToolResult, bool) {
	f.counters.IncExecuteModelTool()
	if f.toolRegistry == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "tool registry not configured", Error: &LegacyToolError{Code: "TOOL_REGISTRY_UNAVAILABLE"}}, false
	}
	def, ok := f.toolRegistry.GetByModelName(ctx, modelName)
	if !ok {
		return LegacyToolResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", modelName), Error: &LegacyToolError{Code: "TOOL_NOT_FOUND", Message: modelName}}, false
	}
	f.counters.IncPipelineExecution()
	return f.executeResolvedTool(ctx, def, input, scope, scope.ToolCallID, idempotencyKey), true
}

func (f *ToolFacade) AfterReply(scope LegacyScope, reply LegacyReplyView) bool {
	f.counters.IncAfterReply()
	if f.hookService != nil && f.hookService.Integrator != nil {
		hookCtx := f.buildHookContext(scope)
		payload := f.buildAfterReplyPayload(reply)
		_, blocked, err := f.hookService.Integrator.InvokePromptAfterAssemble(context.Background(), payload, hookCtx)
		if err != nil {
			f.counters.IncPipelineFailure("after_reply_hook")
		}
		return !blocked
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
		params, err := tool.ParseParametersSchema(def.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("tool %s input schema: %w", def.ID, err)
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

func (f *ToolFacade) executeResolvedTool(ctx context.Context, def capability.ToolDefinition, input json.RawMessage, scope LegacyScope, externalCallID string, idempotencyKey string) LegacyToolResult {
	if f.executionKernel == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "execution kernel not configured", Error: &LegacyToolError{Code: "EXECUTION_KERNEL_UNAVAILABLE"}}
	}
	invocation := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		ExternalCallID: externalCallID,
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Channel:        scope.Channel,
		SessionID:      scope.SessionID,
		ExtensionID:    def.ExtensionID,
		ModuleID:       def.ModuleID,
		Source:         capability.InvocationSourceModel,
		IdempotencyKey: idempotencyKey,
		TraceID:        scope.TraceID,
		OperationID:    scope.RequestID,
	})
	req := execution.ToolExecutionRequest{
		ToolID:     capability.CapabilityID(def.ID),
		Input:      input,
		Invocation: invocation,
	}
	result := f.executionKernel.Execute(ctx, req)
	return unifiedResultToLegacy(result)
}

func (f *ToolFacade) ExecuteModelToolStream(ctx context.Context, modelName string, input json.RawMessage, scope LegacyScope, idempotencyKey string, sink capability.ToolStreamSink) (LegacyToolResult, bool, error) {
	if f.toolRegistry == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "tool registry not configured", Error: &LegacyToolError{Code: "TOOL_REGISTRY_UNAVAILABLE"}}, false, nil
	}
	if sink == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "stream sink is required", Error: &LegacyToolError{Code: "STREAM_SINK_REQUIRED"}}, false, fmt.Errorf("stream sink is nil")
	}

	def, ok := f.toolRegistry.GetByModelName(ctx, modelName)
	if !ok {
		def, ok = f.toolRegistry.Get(ctx, modelName)
	}
	if !ok {
		return LegacyToolResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", modelName), Error: &LegacyToolError{Code: "TOOL_NOT_FOUND", Message: modelName}}, false, nil
	}

	var kernelInterface interface{} = f.executionKernel
	streamingKernel, ok := kernelInterface.(execution.StreamingExecutionSecurityKernel)
	if !ok {
		result := f.executeResolvedTool(ctx, def, input, scope, scope.ToolCallID, idempotencyKey)
		return result, false, nil
	}

	invocation := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		ExternalCallID: scope.ToolCallID,
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Channel:        scope.Channel,
		SessionID:      scope.SessionID,
		ExtensionID:    def.ExtensionID,
		ModuleID:       def.ModuleID,
		Source:         capability.InvocationSourceModel,
		IdempotencyKey: idempotencyKey,
		TraceID:        scope.TraceID,
		OperationID:    scope.RequestID,
	})
	req := execution.ToolExecutionRequest{
		ToolID:     capability.CapabilityID(def.ID),
		Input:      input,
		Invocation: invocation,
	}

	f.counters.IncPipelineExecution()
	result, err := streamingKernel.ExecuteStream(ctx, req, sink)
	legacy := unifiedResultToLegacy(result)
	return legacy, true, err
}

func unifiedResultToLegacy(result capability.UnifiedToolResult) LegacyToolResult {
	legacy := LegacyToolResult{
		RunID:      result.InvocationID,
		Status:     string(result.Status),
		Output:     result.Structured,
		DurationMS: result.DurationMS,
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

func boolPtr(v bool) *bool {
	return &v
}

func (f *ToolFacade) CancelInvocation(ctx context.Context, invocationID string) execution.CancellationResult {
	reason := capability.ToolCancellationReason{
		Code: capability.CancellationReasonUserRequested,
	}
	var kernelInterface interface{} = f.executionKernel
	if cancellable, ok := kernelInterface.(execution.CancellableExecutionSecurityKernel); ok {
		return cancellable.CancelInvocation(ctx, invocationID, reason)
	}
	return execution.CancellationResult{Requested: false, TargetInvocationID: invocationID}
}

func (f *ToolFacade) CancelModelTool(ctx context.Context, scope LegacyScope, toolCallID string) execution.CancellationResult {
	reason := capability.ToolCancellationReason{
		Code: capability.CancellationReasonUserRequested,
	}
	var kernelInterface interface{} = f.executionKernel
	if cancellable, ok := kernelInterface.(execution.CancellableExecutionSecurityKernel); ok {
		externalScope := capability.CancellationExternalScope{
			UserID:         scope.UserID,
			CharacterID:    scope.CharacterID,
			ConversationID: scope.ConversationID,
			SessionID:      scope.SessionID,
		}
		return cancellable.CancelExternalCall(ctx, externalScope, toolCallID, reason)
	}
	return execution.CancellationResult{Requested: false}
}

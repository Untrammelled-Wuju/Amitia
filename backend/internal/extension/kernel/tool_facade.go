package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
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
	toolRegistry          *capability.ToolRegistry
	executionKernel       *execution.ExecutionPipeline
	capabilityResolver    capability.CapabilityResolver
	hookService           *hook.Service
	agentSkillCatalog     *agent_skill.AgentSkillCatalog
	activationService     *agent_skill.ActivationService
	agentSkillBackend     AgentSkillBackend
	runSkillScriptHandler RunSkillScriptHandler
	skillResourceHandler  SkillResourceHandler
	acquisitionBridge     *acquisition.AgentCapabilityBridge
	recoveryService       *acquisition.RecoveryService
	counters              *ToolFacadeCounters
	config                ToolFacadeConfig
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

func (f *ToolFacade) SetAgentSkillBackend(backend AgentSkillBackend) {
	f.agentSkillBackend = backend
}

func (f *ToolFacade) SetRunSkillScriptHandler(handler RunSkillScriptHandler) {
	f.runSkillScriptHandler = handler
}

func (f *ToolFacade) SetSkillResourceHandler(handler SkillResourceHandler) {
	f.skillResourceHandler = handler
}

func (f *ToolFacade) SetAcquisitionBridge(bridge *acquisition.AgentCapabilityBridge) {
	f.acquisitionBridge = bridge
}

func (f *ToolFacade) SetCapabilityResolver(resolver capability.CapabilityResolver) {
	f.capabilityResolver = resolver
}

func (f *ToolFacade) SetRecoveryService(svc *acquisition.RecoveryService) {
	f.recoveryService = svc
}

func (f *ToolFacade) TryRecoverMissingCapability(ctx context.Context, err error, invocation capability.ToolInvocationContext) (*acquisition.AcquisitionResult, error) {
	if f.recoveryService == nil {
		return nil, fmt.Errorf("recovery service not configured")
	}
	return f.recoveryService.RecoverFromError(ctx, err, invocation)
}

func (f *ToolFacade) PrepareAgentSkillPrompt(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string) {
	f.counters.IncPrepareAgentSkillPrompt()
	if f.agentSkillBackend != nil {
		return f.prepareAgentSkillPromptFromBackend(ctx, scope, message)
	}
	if f.agentSkillCatalog == nil {
		return "", nil, nil
	}
	return f.buildAgentSkillPrompt(ctx, scope, message)
}

func (f *ToolFacade) EndAgentSkillRound(scope LegacyScope) {
	f.counters.IncEndAgentSkillRound()
	if f.agentSkillBackend != nil {
		f.agentSkillBackend.EndRound(scope)
	}
}

func (f *ToolFacade) prepareAgentSkillPromptFromBackend(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string) {
	catalog, err := f.agentSkillBackend.ResolveCatalog(ctx, scope)
	if err != nil {
		return "", nil, []string{err.Error()}
	}

	errorsList := []string{}
	activated := []LegacyActivatedSkill{}

	explicitNames := parseExplicitSkillNames(message)
	for _, name := range explicitNames {
		result, activateErr := f.agentSkillBackend.Activate(ctx, scope, name, true)
		if activateErr != nil {
			errorsList = append(errorsList, activateErr.Error())
			continue
		}
		prompts, promptErr := f.agentSkillBackend.ActivePrompts(ctx, scope)
		if promptErr != nil {
			errorsList = append(errorsList, promptErr.Error())
			continue
		}
		for _, p := range prompts {
			if p.ActivationID == result.ActivationID {
				activated = append(activated, LegacyActivatedSkill{
					ActivationID:        p.ActivationID,
					ExtensionID:         p.ExtensionID,
					Name:                p.Name,
					Source:              p.Source,
					Scope:               p.Scope,
					CompatibilityStatus: p.CompatibilityStatus,
					Prompt:              p.Body,
					BodyTokens:          p.BodyTokens,
					Explicit:            p.Explicit,
				})
				break
			}
		}
	}

	catalogSection := renderSkillCatalogFromEntries(catalog)
	return catalogSection, activated, errorsList
}

func renderSkillCatalogFromEntries(catalog []SkillCatalogEntry) string {
	if len(catalog) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("# Available Agent Skills\n\n")
	for _, s := range catalog {
		sb.WriteString(fmt.Sprintf("- **%s**", s.Name))
		sb.WriteString(fmt.Sprintf(": %s\n", s.Description))
	}
	sb.WriteString("\nTo activate a skill, use the activate_skill tool or include $skill-name in your message.\n")
	return sb.String()
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
	tools, err := f.buildKernelModelTools(ctx, scope)
	if err != nil {
		return nil, err
	}
	if f.agentSkillBackend != nil {
		names, namesErr := f.resolveVisibleSkillNames(ctx, scope)
		if namesErr == nil && len(names) > 0 {
			tools = append(tools, buildActivateSkillTool(names))
		}
	}
	if f.agentSkillBackend != nil || f.acquisitionBridge != nil {
		tools = append(tools, buildUsePackageTool())
	}
	if f.runSkillScriptHandler != nil {
		scriptNames, scriptErr := f.resolveScriptCapableSkillNames(ctx, scope)
		if scriptErr == nil && len(scriptNames) > 0 {
			tools = append(tools, buildRunSkillScriptTool(scriptNames))
		}
	}
	if f.skillResourceHandler != nil {
		resourceNames, resourceErr := f.resolveResourceCapableSkillNames(ctx, scope)
		if resourceErr == nil && len(resourceNames) > 0 {
			tools = append(tools, buildListSkillResourcesTool(), buildReadSkillResourceTool(), buildMaterializeSkillResourceTool())
		}
	}
	return tools, nil
}

type ResolvedToolReference struct {
	ID              capability.CapabilityID
	ModelName       string
	ExtensionID     string
	ModuleID        string
	RuntimeType     capability.RuntimeType
	RuntimeID       string
	AllowBackground bool
	ToolVersion     string
	Generation      int64
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
		ID:              capability.CapabilityID(def.ID),
		ModelName:       def.ModelName,
		ExtensionID:     def.ExtensionID,
		ModuleID:        def.ModuleID,
		RuntimeType:     def.Runtime.RuntimeType,
		RuntimeID:       def.Runtime.RuntimeID,
		AllowBackground: def.ExecutionPolicy.AllowBackground,
		ToolVersion:     def.Version,
		Generation:      0,
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
	if !workflowToolAllowedForUser(def, scope.UserID) {
		return LegacyToolResult{Status: "FAILED", VisibleText: "workflow tool is not available for this user", Error: &LegacyToolError{Code: "TOOL_NOT_FOUND", Message: string(toolID)}}, false
	}
	f.counters.IncPipelineExecution()
	return f.executeResolvedTool(ctx, def, input, scope, externalCallID, idempotencyKey), true
}

func (f *ToolFacade) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope LegacyScope, idempotencyKey string) (LegacyToolResult, bool) {
	f.counters.IncExecuteModelTool()
	if modelName == ActivateSkillToolName && f.agentSkillBackend != nil {
		result, _ := f.handleActivateSkill(ctx, input, scope)
		return result, true
	}
	if modelName == UsePackageToolName && (f.agentSkillBackend != nil || f.acquisitionBridge != nil) {
		result, _ := f.handleUsePackage(ctx, input, scope)
		return result, true
	}
	if modelName == RunSkillScriptToolName && f.runSkillScriptHandler != nil {
		result, _ := f.handleRunSkillScript(ctx, input, scope)
		return result, true
	}
	if modelName == ListSkillResourcesToolName && f.skillResourceHandler != nil {
		result, _ := f.handleListSkillResources(ctx, input, scope)
		return result, true
	}
	if modelName == ReadSkillResourceToolName && f.skillResourceHandler != nil {
		result, _ := f.handleReadSkillResource(ctx, input, scope)
		return result, true
	}
	if modelName == MaterializeSkillResourceToolName && f.skillResourceHandler != nil {
		result, _ := f.handleMaterializeSkillResource(ctx, input, scope)
		return result, true
	}
	if modelName == acquisition.FindCapabilitiesToolID && f.acquisitionBridge != nil {
		result, _ := f.handleFindCapability(ctx, input, scope)
		return result, true
	}
	if modelName == acquisition.AcquireCapabilityToolID && f.acquisitionBridge != nil {
		result, _ := f.handleAcquireCapability(ctx, input, scope)
		return result, true
	}
	if f.toolRegistry == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "tool registry not configured", Error: &LegacyToolError{Code: "TOOL_REGISTRY_UNAVAILABLE"}}, false
	}
	def, ok := f.toolRegistry.GetByModelName(ctx, modelName)
	if !ok {
		return LegacyToolResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", modelName), Error: &LegacyToolError{Code: "TOOL_NOT_FOUND", Message: modelName}}, false
	}
	if !workflowToolAllowedForUser(def, scope.UserID) {
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

func (f *ToolFacade) handleFindCapability(ctx context.Context, input json.RawMessage, scope LegacyScope) (LegacyToolResult, error) {
	var req acquisition.FindCapabilitiesInput
	if err := json.Unmarshal(input, &req); err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("invalid find_capability input: %v", err),
			Error:       &LegacyToolError{Code: "INVALID_INPUT", Message: err.Error()},
		}, err
	}
	output, err := f.acquisitionBridge.FindCapabilities(ctx, req, scope.UserID)
	if err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("find_capability failed: %v", err),
			Error:       &LegacyToolError{Code: "FIND_CAPABILITY_FAILED", Message: err.Error()},
		}, err
	}
	resultJSON, _ := json.Marshal(output)
	return LegacyToolResult{
		Status:      "SUCCESS",
		Output:      resultJSON,
		VisibleText: fmt.Sprintf("Found %d candidate(s) for capability %s", output.TotalFound, req.CapabilityID),
	}, nil
}

func (f *ToolFacade) handleAcquireCapability(ctx context.Context, input json.RawMessage, scope LegacyScope) (LegacyToolResult, error) {
	var req acquisition.AcquireInput
	if err := json.Unmarshal(input, &req); err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("invalid acquire_capability input: %v", err),
			Error:       &LegacyToolError{Code: "INVALID_INPUT", Message: err.Error()},
		}, err
	}
	output, err := f.acquisitionBridge.AcquireCapability(ctx, req, scope.UserID, scope.ExecContext)
	if err != nil {
		return LegacyToolResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("acquire_capability failed: %v", err),
			Error:       &LegacyToolError{Code: "ACQUIRE_CAPABILITY_FAILED", Message: err.Error()},
		}, err
	}
	resultJSON, _ := json.Marshal(output)
	visibleText := fmt.Sprintf("Capability acquisition state: %s", output.State)
	if output.Success {
		visibleText = fmt.Sprintf("Capability %s acquired successfully", output.CapabilityID)
	}
	return LegacyToolResult{
		Status:      "SUCCESS",
		Output:      resultJSON,
		VisibleText: visibleText,
	}, nil
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
		if !workflowToolAllowedForUser(def, scope.UserID) {
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

type resolvedExecution struct {
	target            capability.InvocationExecutionTarget
	legacyUnresolved  bool
	missingCapability capability.CapabilityID
}

func (f *ToolFacade) SetCapabilityService(svc *capability.CapabilityService) {
	if svc != nil {
		f.capabilityResolver = svc
	}
}

func (f *ToolFacade) resolveExecutionTarget(ctx context.Context, def capability.ToolDefinition) resolvedExecution {
	if f.capabilityResolver == nil {
		return resolvedExecution{legacyUnresolved: true}
	}
	capID := def.CapabilityID
	if capID == "" {
		capID = capability.CapabilityID(def.ID)
	}
	req := capability.CapabilityResolutionRequest{
		CapabilityID:       capID,
		ExtensionID:        def.ExtensionID,
		ModuleID:           def.ModuleID,
		AllowCore:          true,
		AllowDevice:        true,
		PreferredPlacement: capability.ProviderPlacementCore,
	}
	if def.RoutingMode == capability.RoutingModeProviderRequired || def.RoutingMode == capability.RoutingModeProviderPreferred {
		if def.ProviderID != "" {
			req.PreferredProviderID = capability.ProviderID(def.ProviderID)
		}
	}
	result, err := f.capabilityResolver.Resolve(req)
	if err != nil {
		if def.CapabilityID != "" {
			return resolvedExecution{missingCapability: def.CapabilityID}
		}
		return resolvedExecution{legacyUnresolved: true}
	}
	if !result.HasResult() {
		if def.CapabilityID != "" {
			return resolvedExecution{missingCapability: def.CapabilityID}
		}
		return resolvedExecution{legacyUnresolved: true}
	}
	return resolvedExecution{target: result.ExecutionTarget}
}

func (f *ToolFacade) executeResolvedTool(ctx context.Context, def capability.ToolDefinition, input json.RawMessage, scope LegacyScope, externalCallID string, idempotencyKey string) LegacyToolResult {
	if f.executionKernel == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "execution kernel not configured", Error: &LegacyToolError{Code: "EXECUTION_KERNEL_UNAVAILABLE"}}
	}
	isBackground := def.Runtime.RuntimeType == capability.RuntimeTypeTask && def.ExecutionPolicy.AllowBackground
	resolved := f.resolveExecutionTarget(ctx, def)

	if resolved.missingCapability != "" {
		if f.recoveryService != nil {
			metadata := map[string]any{
				"capabilityId": string(resolved.missingCapability),
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
				IsBackground:   isBackground,
				ExecContext:    scope.ExecContext,
				Metadata:       metadata,
			})
			resolutionFailure := capability.ResolutionFailureCapabilityNotRegistered
			recoveryResult, recoverErr := f.recoveryService.RecoverFromResolution(ctx, resolutionFailure, invocation)
			if recoverErr != nil {
				if errors.Is(recoverErr, acquisition.ErrApprovalRequired) && recoveryResult != nil {
					payload, _ := json.Marshal(recoveryResult)
					return LegacyToolResult{
						Status:      "WAITING_APPROVAL",
						Output:      payload,
						VisibleText: fmt.Sprintf("approval required to acquire capability %s", resolved.missingCapability),
						Error: &LegacyToolError{
							Code:      "CAPABILITY_APPROVAL_REQUIRED",
							Message:   string(resolved.missingCapability),
							Detail:    recoveryResult.ResumeToken,
							Retryable: true,
						},
					}
				}
				return LegacyToolResult{
					Status:      "FAILED",
					VisibleText: fmt.Sprintf("capability recovery failed: %s", recoverErr),
					Error: &LegacyToolError{
						Code:    "CAPABILITY_RECOVERY_FAILED",
						Message: string(resolved.missingCapability),
						Detail:  recoverErr.Error(),
					},
				}
			}
			resolved = f.resolveExecutionTarget(ctx, def)
		}

		if resolved.missingCapability != "" {
			return LegacyToolResult{
				Status:      "FAILED",
				VisibleText: fmt.Sprintf("capability not available: %s", resolved.missingCapability),
				Error: &LegacyToolError{
					Code:    "CAPABILITY_NOT_REGISTERED",
					Message: string(resolved.missingCapability),
					Detail:  fmt.Sprintf("capability %s has no executable provider", resolved.missingCapability),
				},
			}
		}
	}

	metadata := map[string]any{}
	if resolved.legacyUnresolved {
		metadata["execution_mode"] = "legacy_unresolved_provider"
	}
	invocation := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		ExternalCallID:  externalCallID,
		UserID:          scope.UserID,
		CharacterID:     scope.CharacterID,
		ConversationID:  scope.ConversationID,
		Channel:         scope.Channel,
		SessionID:       scope.SessionID,
		ExtensionID:     def.ExtensionID,
		ModuleID:        def.ModuleID,
		Source:          capability.InvocationSourceModel,
		IdempotencyKey:  idempotencyKey,
		TraceID:         scope.TraceID,
		OperationID:     scope.RequestID,
		IsBackground:    isBackground,
		ExecContext:     scope.ExecContext,
		ExecutionTarget: resolved.target,
		Metadata:        metadata,
	})
	execToolID := capability.CapabilityID(def.ID)
	req := execution.ToolExecutionRequest{
		ToolID:     execToolID,
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
	if !ok || !workflowToolAllowedForUser(def, scope.UserID) {
		return LegacyToolResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", modelName), Error: &LegacyToolError{Code: "TOOL_NOT_FOUND", Message: modelName}}, false, nil
	}

	var kernelInterface interface{} = f.executionKernel
	streamingKernel, ok := kernelInterface.(execution.StreamingExecutionSecurityKernel)
	if !ok {
		result := f.executeResolvedTool(ctx, def, input, scope, scope.ToolCallID, idempotencyKey)
		return result, false, nil
	}

	isBackground := def.Runtime.RuntimeType == capability.RuntimeTypeTask && def.ExecutionPolicy.AllowBackground
	resolved := f.resolveExecutionTarget(ctx, def)
	streamMetadata := map[string]any{}
	if resolved.legacyUnresolved {
		streamMetadata["execution_mode"] = "legacy_unresolved_provider"
	}
	invocation := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		ExternalCallID:  scope.ToolCallID,
		UserID:          scope.UserID,
		CharacterID:     scope.CharacterID,
		ConversationID:  scope.ConversationID,
		Channel:         scope.Channel,
		SessionID:       scope.SessionID,
		ExtensionID:     def.ExtensionID,
		ModuleID:        def.ModuleID,
		Source:          capability.InvocationSourceModel,
		IdempotencyKey:  idempotencyKey,
		TraceID:         scope.TraceID,
		OperationID:     scope.RequestID,
		IsBackground:    isBackground,
		ExecContext:     scope.ExecContext,
		ExecutionTarget: resolved.target,
		Metadata:        streamMetadata,
	})
	execToolID := capability.CapabilityID(def.ID)
	req := execution.ToolExecutionRequest{
		ToolID:     execToolID,
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

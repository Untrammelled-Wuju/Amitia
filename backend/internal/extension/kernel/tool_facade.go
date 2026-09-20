package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/agentpermission"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
	"github.com/u-ai/backend/internal/runtimeidentity"
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

const (
	contextProviderSlotsMetadataKey    = "amitia.context.provides"
	contextProviderPriorityMetadataKey = "amitia.context.priority"
	contextProviderKeyMetadataKey      = "amitia.context.key"
	messageOutputProviderKey           = "amitia.message.outputs"
	messageOutputProviderPriority      = "amitia.message.priority"
)

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

func (f *ToolFacade) ListContextProviders(ctx context.Context, slot string) []capability.ToolDefinition {
	if f == nil || f.toolRegistry == nil {
		return nil
	}
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return nil
	}
	definitions := f.toolRegistry.List(ctx, capability.ToolFilter{Enabled: boolPtr(true), IncludeInternal: true})
	result := make([]capability.ToolDefinition, 0)
	for _, definition := range definitions {
		if !contextProviderProvides(definition, slot) {
			continue
		}
		result = append(result, definition)
	}
	sort.SliceStable(result, func(i, j int) bool {
		leftPriority := ContextProviderPriority(result[i])
		rightPriority := ContextProviderPriority(result[j])
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		leftSource := ContextProviderSource(result[i])
		rightSource := ContextProviderSource(result[j])
		if leftSource != rightSource {
			return leftSource < rightSource
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func ContextProviderPriority(definition capability.ToolDefinition) int {
	switch value := definition.Metadata[contextProviderPriorityMetadataKey].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	}
	return 0
}

func ContextProviderSource(definition capability.ToolDefinition) string {
	key := strings.TrimSpace(metadataString(definition.Metadata[contextProviderKeyMetadataKey]))
	switch {
	case definition.ExtensionID != "" && key != "":
		return definition.ExtensionID + ":" + key
	case definition.ExtensionID != "":
		return definition.ExtensionID
	case key != "":
		return key
	default:
		return definition.ID
	}
}

func contextProviderProvides(definition capability.ToolDefinition, slot string) bool {
	switch value := definition.Metadata[contextProviderSlotsMetadataKey].(type) {
	case string:
		return strings.TrimSpace(value) == slot
	case []string:
		for _, candidate := range value {
			if strings.TrimSpace(candidate) == slot {
				return true
			}
		}
	case []any:
		for _, candidate := range value {
			if strings.TrimSpace(metadataString(candidate)) == slot {
				return true
			}
		}
	}
	return false
}

func metadataString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func (f *ToolFacade) ListMessageOutputProviders(ctx context.Context) []capability.ToolDefinition {
	if f == nil || f.toolRegistry == nil {
		return nil
	}
	definitions := f.toolRegistry.List(ctx, capability.ToolFilter{Enabled: boolPtr(true), IncludeInternal: true})
	result := make([]capability.ToolDefinition, 0)
	for _, definition := range definitions {
		if !messageOutputProviderEnabled(definition) {
			continue
		}
		result = append(result, definition)
	}
	sort.SliceStable(result, func(i, j int) bool {
		leftPriority := MessageOutputProviderPriority(result[i])
		rightPriority := MessageOutputProviderPriority(result[j])
		if leftPriority != rightPriority {
			return leftPriority > rightPriority
		}
		leftSource := ContextProviderSource(result[i])
		rightSource := ContextProviderSource(result[j])
		if leftSource != rightSource {
			return leftSource < rightSource
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func MessageOutputProviderPriority(definition capability.ToolDefinition) int {
	switch value := definition.Metadata[messageOutputProviderPriority].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, _ := value.Int64()
		return int(parsed)
	}
	return 0
}

func messageOutputProviderEnabled(definition capability.ToolDefinition) bool {
	switch value := definition.Metadata[messageOutputProviderKey].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	}
	return false
}

type MessageOutputProviderResult struct {
	Provider capability.ToolDefinition
	Result   ToolDispatchResult
}

func (f *ToolFacade) ExecuteMessageOutputProviders(ctx context.Context, scope InvocationScope, input json.RawMessage) []MessageOutputProviderResult {
	providers := f.ListMessageOutputProviders(ctx)
	results := make([]MessageOutputProviderResult, 0, len(providers))
	for _, provider := range providers {
		result, found := f.ExecuteTool(
			ctx,
			capability.CapabilityID(provider.ID),
			input,
			scope,
			fmt.Sprintf("message-output-%s-%s", provider.ID, scope.RequestID),
			fmt.Sprintf("message-output:%s:%s", provider.ID, scope.RequestID),
		)
		if !found {
			continue
		}
		results = append(results, MessageOutputProviderResult{Provider: provider, Result: result})
	}
	return results
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

func (f *ToolFacade) PrepareAgentSkillPrompt(ctx context.Context, scope InvocationScope, message string) (string, []ActivatedSkill, []string) {
	f.counters.IncPrepareAgentSkillPrompt()
	if f.agentSkillBackend != nil {
		return f.prepareAgentSkillPromptFromBackend(ctx, scope, message)
	}
	if f.agentSkillCatalog == nil {
		return "", nil, nil
	}
	return f.buildAgentSkillPrompt(ctx, scope, message)
}

func (f *ToolFacade) EndAgentSkillRound(scope InvocationScope) {
	f.counters.IncEndAgentSkillRound()
	if f.agentSkillBackend != nil {
		f.agentSkillBackend.EndRound(scope)
	}
}

func (f *ToolFacade) prepareAgentSkillPromptFromBackend(ctx context.Context, scope InvocationScope, message string) (string, []ActivatedSkill, []string) {
	catalog, err := f.agentSkillBackend.ResolveCatalog(ctx, scope)
	if err != nil {
		return "", nil, []string{err.Error()}
	}

	errorsList := []string{}
	activated := []ActivatedSkill{}

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
				activated = append(activated, ActivatedSkill{
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

func (f *ToolFacade) BeforePrompt(ctx context.Context, scope InvocationScope) []ContextContribution {
	f.counters.IncBeforePrompt()
	contributions := make([]ContextContribution, 0, 4)
	if workspaceContribution, ok := workspacePromptContribution(scope); ok {
		contributions = append(contributions, workspaceContribution)
	}
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
		contributions = append(contributions, f.parseContextContributions(result)...)
	}
	return contributions
}

func workspacePromptContribution(scope InvocationScope) (ContextContribution, bool) {
	if scope.ExecContext == nil || strings.TrimSpace(scope.ExecContext.WorkspaceID) == "" {
		return ContextContribution{}, false
	}
	workspaceID := strings.TrimSpace(scope.ExecContext.WorkspaceID)
	rootURI := "amitia://workspace/@" + workspaceID + "/"
	name := ""
	kind := ""
	deviceID := ""
	if metadata := scope.ExecContext.Metadata; metadata != nil {
		if value, ok := metadata["workspaceRootUri"].(string); ok && strings.TrimSpace(value) != "" {
			rootURI = strings.TrimSpace(value)
		}
		if value, ok := metadata["workspaceName"].(string); ok {
			name = strings.TrimSpace(value)
		}
		if value, ok := metadata["workspaceKind"].(string); ok {
			kind = strings.TrimSpace(value)
		}
		if value, ok := metadata["workspaceDeviceId"].(string); ok {
			deviceID = strings.TrimSpace(value)
		}
	}
	content := fmt.Sprintf("Current conversation workspace is bound to workspaceId=%s, rootUri=%s.", workspaceID, rootURI)
	if name != "" {
		content += " Display name: " + name + "."
	}
	content += " Treat relative project paths as relative to this workspace. Use this workspace for file/search/edit operations unless the user explicitly changes the chat workspace. Do not access another workspace ID."
	return ContextContribution{
		Source:     "conversation_workspace",
		Priority:   95,
		Content:    content,
		TokenLimit: 220,
		Metadata: map[string]string{
			"workspaceId": workspaceID,
			"rootUri":     rootURI,
			"name":        name,
			"kind":        kind,
			"deviceId":    deviceID,
		},
	}, true
}

func (f *ToolFacade) ModelTools(ctx context.Context, scope InvocationScope) ([]tool.Tool, error) {
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

func (f *ToolFacade) IsModelToolParallelSafe(ctx context.Context, modelName string, scope InvocationScope) bool {
	if f == nil || f.toolRegistry == nil {
		return false
	}
	def, ok := f.toolRegistry.GetByModelName(ctx, modelName)
	if !ok {
		return false
	}
	if def.ExecutionPolicy.MaxConcurrency == 1 || !def.Idempotent {
		return false
	}
	switch def.SideEffect {
	case capability.SideEffectNone, capability.SideEffectReadOnly:
		return true
	default:
		return false
	}
}

func (f *ToolFacade) ExecuteTool(ctx context.Context, toolID capability.CapabilityID, input json.RawMessage, scope InvocationScope, externalCallID string, idempotencyKey string) (ToolDispatchResult, bool) {
	f.counters.IncExecuteModelTool()
	if f.toolRegistry == nil {
		return ToolDispatchResult{Status: "FAILED", VisibleText: "tool registry not configured", Error: &ToolDispatchError{Code: "TOOL_REGISTRY_UNAVAILABLE"}}, false
	}
	def, ok := f.toolRegistry.Get(ctx, string(toolID))
	if !ok {
		return ToolDispatchResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", toolID), Error: &ToolDispatchError{Code: "TOOL_NOT_FOUND", Message: string(toolID)}}, false
	}
	if !workflowToolAllowedForSpace(def, scope.SpaceID) {
		return ToolDispatchResult{Status: "FAILED", VisibleText: "workflow tool is not available for this Space", Error: &ToolDispatchError{Code: "TOOL_NOT_FOUND", Message: string(toolID)}}, false
	}
	f.counters.IncPipelineExecution()
	return f.executeResolvedTool(ctx, def, input, scope, externalCallID, idempotencyKey), true
}

func (f *ToolFacade) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope InvocationScope, idempotencyKey string) (ToolDispatchResult, bool) {
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
		return ToolDispatchResult{Status: "FAILED", VisibleText: "tool registry not configured", Error: &ToolDispatchError{Code: "TOOL_REGISTRY_UNAVAILABLE"}}, false
	}
	def, ok := f.toolRegistry.GetByModelName(ctx, modelName)
	if !ok {
		return ToolDispatchResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", modelName), Error: &ToolDispatchError{Code: "TOOL_NOT_FOUND", Message: modelName}}, false
	}
	if !workflowToolAllowedForSpace(def, scope.SpaceID) {
		return ToolDispatchResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", modelName), Error: &ToolDispatchError{Code: "TOOL_NOT_FOUND", Message: modelName}}, false
	}
	f.counters.IncPipelineExecution()
	return f.executeResolvedTool(ctx, def, input, scope, scope.ToolCallID, idempotencyKey), true
}

func (f *ToolFacade) AfterReply(scope InvocationScope, reply ReplyView) bool {
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

func (f *ToolFacade) handleFindCapability(ctx context.Context, input json.RawMessage, scope InvocationScope) (ToolDispatchResult, error) {
	var req acquisition.FindCapabilitiesInput
	if err := json.Unmarshal(input, &req); err != nil {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("invalid find_capability input: %v", err),
			Error:       &ToolDispatchError{Code: "INVALID_INPUT", Message: err.Error()},
		}, err
	}
	output, err := f.acquisitionBridge.FindCapabilities(ctx, req, scope.SpaceID)
	if err != nil {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("find_capability failed: %v", err),
			Error:       &ToolDispatchError{Code: "FIND_CAPABILITY_FAILED", Message: err.Error()},
		}, err
	}
	resultJSON, _ := json.Marshal(output)
	return ToolDispatchResult{
		Status:      "SUCCESS",
		Output:      resultJSON,
		VisibleText: fmt.Sprintf("Found %d candidate(s) for capability %s", output.TotalFound, req.CapabilityID),
	}, nil
}

func (f *ToolFacade) handleAcquireCapability(ctx context.Context, input json.RawMessage, scope InvocationScope) (ToolDispatchResult, error) {
	var req acquisition.AcquireInput
	if err := json.Unmarshal(input, &req); err != nil {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("invalid acquire_capability input: %v", err),
			Error:       &ToolDispatchError{Code: "INVALID_INPUT", Message: err.Error()},
		}, err
	}
	output, err := f.acquisitionBridge.AcquireCapability(ctx, req, scope.SpaceID, scope.ExecContext)
	if err != nil {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: fmt.Sprintf("acquire_capability failed: %v", err),
			Error:       &ToolDispatchError{Code: "ACQUIRE_CAPABILITY_FAILED", Message: err.Error()},
		}, err
	}
	resultJSON, _ := json.Marshal(output)
	visibleText := fmt.Sprintf("Capability acquisition state: %s", output.State)
	if output.Success {
		visibleText = fmt.Sprintf("Capability %s acquired successfully", output.CapabilityID)
	}
	return ToolDispatchResult{
		Status:      "SUCCESS",
		Output:      resultJSON,
		VisibleText: visibleText,
	}, nil
}

func (f *ToolFacade) buildHookContext(scope InvocationScope) hook.HookContextSnapshot {
	deviceID := strings.TrimSpace(scope.DeviceID)
	runtimeID := strings.TrimSpace(scope.RuntimeID)
	if scope.ExecContext != nil && scope.ExecContext.RuntimeTarget != nil {
		if deviceID == "" {
			deviceID = string(scope.ExecContext.RuntimeTarget.DeviceID)
		}
		if runtimeID == "" {
			runtimeID = string(scope.ExecContext.RuntimeTarget.RuntimeID)
		}
	}
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
		SpaceID:        strings.TrimSpace(scope.SpaceID),
		DeviceID:       deviceID,
		RuntimeID:      runtimeID,
		PrincipalType:  strings.TrimSpace(scope.PrincipalType),
		CharacterID:    charID,
		ConversationID: convID,
		Platform:       scope.Channel,
		Timestamp:      time.Now().UTC(),
		Depth:          0,
	}
}

func (f *ToolFacade) buildBeforePromptPayload(scope InvocationScope) json.RawMessage {
	payload := map[string]interface{}{
		"sections": map[string]interface{}{},
		"context": map[string]interface{}{
			"spaceId":        scope.SpaceID,
			"characterId":    scope.CharacterID,
			"conversationId": scope.ConversationID,
			"channel":        scope.Channel,
			"sessionId":      scope.SessionID,
			"message":        scope.Message,
			"source":         scope.Source,
			"internal":       scope.IsInternal,
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func (f *ToolFacade) buildAfterReplyPayload(reply ReplyView) json.RawMessage {
	payload := map[string]interface{}{
		"response": map[string]interface{}{
			"content":      reply.Content,
			"finishReason": "stop",
		},
	}
	b, _ := json.Marshal(payload)
	return b
}

func (f *ToolFacade) parseContextContributions(result json.RawMessage) []ContextContribution {
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
	contributions := make([]ContextContribution, 0)
	if parsed.Sections != nil {
		if extCtx, ok := parsed.Sections["extension_context"].(map[string]interface{}); ok {
			for source, val := range extCtx {
				if content, ok := val.(string); ok {
					contributions = append(contributions, ContextContribution{
						Source:  source,
						Content: content,
					})
				} else if obj, ok := val.(map[string]interface{}); ok {
					if content, ok := obj["content"].(string); ok {
						priority := 0
						if p, ok := obj["priority"].(float64); ok {
							priority = int(p)
						}
						contributions = append(contributions, ContextContribution{
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

func (f *ToolFacade) buildKernelModelTools(ctx context.Context, scope InvocationScope) ([]tool.Tool, error) {
	defs := f.toolRegistry.List(ctx, capability.ToolFilter{Enabled: boolPtr(true)})
	tools := make([]tool.Tool, 0, len(defs))
	for _, def := range defs {
		if !def.Enabled {
			continue
		}
		if !workflowToolAllowedForSpace(def, scope.SpaceID) {
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
	target              capability.InvocationExecutionTarget
	missingCapability   capability.CapabilityID
	resolverUnavailable bool
}

func (f *ToolFacade) SetCapabilityService(svc *capability.CapabilityService) {
	if svc != nil {
		f.capabilityResolver = svc
	}
}

func (f *ToolFacade) resolveExecutionTarget(ctx context.Context, def capability.ToolDefinition, scope InvocationScope) resolvedExecution {
	if def.Runtime.RuntimeType == capability.RuntimeTypeGameHost {
		return resolvedExecution{}
	}
	if strings.TrimSpace(string(def.CapabilityID)) == "" && def.Runtime.RuntimeType != "" {
		return resolvedExecution{}
	}
	if f.capabilityResolver == nil {
		return resolvedExecution{resolverUnavailable: true}
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
	if def.ExtensionID == "com.amitia.builtin.workspace" && scope.ExecContext != nil && scope.ExecContext.RuntimeTarget != nil {
		deviceID := scope.ExecContext.RuntimeTarget.DeviceID
		if deviceID != "" && strings.EqualFold(strings.TrimSpace(scope.ExecContext.RuntimeTarget.Placement), "device") {
			req.ModuleID = ""
			req.RequiredPlacement = capability.ProviderPlacementDevice
			req.PreferredPlacement = capability.ProviderPlacementDevice
			req.RequiredDeviceID = runtimeidentity.DeviceID(deviceID)
			req.PreferredDeviceID = runtimeidentity.DeviceID(deviceID)
			req.AllowCore = false
		}
	}
	if def.RoutingMode == capability.RoutingModeProviderRequired || def.RoutingMode == capability.RoutingModeProviderPreferred {
		if def.ProviderID != "" {
			req.PreferredProviderID = capability.ProviderID(def.ProviderID)
		}
	}
	result, err := f.capabilityResolver.Resolve(req)
	if err != nil || !result.HasResult() {
		return resolvedExecution{missingCapability: capID}
	}
	return resolvedExecution{target: result.ExecutionTarget}
}

func (f *ToolFacade) executeResolvedTool(ctx context.Context, def capability.ToolDefinition, input json.RawMessage, scope InvocationScope, externalCallID string, idempotencyKey string) ToolDispatchResult {
	if f.executionKernel == nil {
		return ToolDispatchResult{Status: "FAILED", VisibleText: "execution kernel not configured", Error: &ToolDispatchError{Code: "EXECUTION_KERNEL_UNAVAILABLE"}}
	}
	isBackground := def.Runtime.RuntimeType == capability.RuntimeTypeTask && def.ExecutionPolicy.AllowBackground
	resolved := f.resolveExecutionTarget(ctx, def, scope)
	if resolved.resolverUnavailable {
		return ToolDispatchResult{
			Status:      "FAILED",
			VisibleText: "capability resolver unavailable",
			Error: &ToolDispatchError{
				Code:    "CAPABILITY_RESOLVER_UNAVAILABLE",
				Message: string(def.ID),
			},
		}
	}

	if resolved.missingCapability != "" {
		if f.recoveryService != nil {
			metadata := map[string]any{
				"capabilityId": string(resolved.missingCapability),
			}
			invocation := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
				ExternalCallID: externalCallID,
				SpaceID:        scope.SpaceID,
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
					return ToolDispatchResult{
						Status:      "WAITING_APPROVAL",
						Output:      payload,
						VisibleText: fmt.Sprintf("approval required to acquire capability %s", resolved.missingCapability),
						Error: &ToolDispatchError{
							Code:      "CAPABILITY_APPROVAL_REQUIRED",
							Message:   string(resolved.missingCapability),
							Detail:    recoveryResult.ResumeToken,
							Retryable: true,
						},
					}
				}
				return ToolDispatchResult{
					Status:      "FAILED",
					VisibleText: fmt.Sprintf("capability recovery failed: %s", recoverErr),
					Error: &ToolDispatchError{
						Code:    "CAPABILITY_RECOVERY_FAILED",
						Message: string(resolved.missingCapability),
						Detail:  recoverErr.Error(),
					},
				}
			}
			resolved = f.resolveExecutionTarget(ctx, def, scope)
		}

		if resolved.resolverUnavailable {
			return ToolDispatchResult{
				Status:      "FAILED",
				VisibleText: "capability resolver unavailable",
				Error:       &ToolDispatchError{Code: "CAPABILITY_RESOLVER_UNAVAILABLE", Message: string(def.ID)},
			}
		}
		if resolved.missingCapability != "" {
			return ToolDispatchResult{
				Status:      "FAILED",
				VisibleText: fmt.Sprintf("capability not available: %s", resolved.missingCapability),
				Error: &ToolDispatchError{
					Code:    "CAPABILITY_NOT_REGISTERED",
					Message: string(resolved.missingCapability),
					Detail:  fmt.Sprintf("capability %s has no executable provider", resolved.missingCapability),
				},
			}
		}
	}

	metadata := map[string]any{"execution_mode": "capability_resolved"}
	approvalMode := capabilityApprovalMode(scope.PermissionMode)
	invocation := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		ExternalCallID:  externalCallID,
		SpaceID:         scope.SpaceID,
		CharacterID:     scope.CharacterID,
		ConversationID:  scope.ConversationID,
		Channel:         scope.Channel,
		SessionID:       scope.SessionID,
		ExtensionID:     def.ExtensionID,
		ModuleID:        def.ModuleID,
		Source:          capability.InvocationSourceModel,
		ApprovalMode:    approvalMode,
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
	return unifiedResultToDispatch(result)
}

func (f *ToolFacade) ExecuteModelToolStream(ctx context.Context, modelName string, input json.RawMessage, scope InvocationScope, idempotencyKey string, sink capability.ToolStreamSink) (ToolDispatchResult, bool, error) {
	if f.toolRegistry == nil {
		return ToolDispatchResult{Status: "FAILED", VisibleText: "tool registry not configured", Error: &ToolDispatchError{Code: "TOOL_REGISTRY_UNAVAILABLE"}}, false, nil
	}
	if sink == nil {
		return ToolDispatchResult{Status: "FAILED", VisibleText: "stream sink is required", Error: &ToolDispatchError{Code: "STREAM_SINK_REQUIRED"}}, false, fmt.Errorf("stream sink is nil")
	}

	def, ok := f.toolRegistry.GetByModelName(ctx, modelName)
	if !ok {
		def, ok = f.toolRegistry.Get(ctx, modelName)
	}
	if !ok || !workflowToolAllowedForSpace(def, scope.SpaceID) {
		return ToolDispatchResult{Status: "FAILED", VisibleText: fmt.Sprintf("tool %s not found in kernel registry", modelName), Error: &ToolDispatchError{Code: "TOOL_NOT_FOUND", Message: modelName}}, false, nil
	}

	var kernelInterface interface{} = f.executionKernel
	streamingKernel, ok := kernelInterface.(execution.StreamingExecutionSecurityKernel)
	if !ok {
		result := f.executeResolvedTool(ctx, def, input, scope, scope.ToolCallID, idempotencyKey)
		return result, false, nil
	}

	isBackground := def.Runtime.RuntimeType == capability.RuntimeTypeTask && def.ExecutionPolicy.AllowBackground
	resolved := f.resolveExecutionTarget(ctx, def, scope)
	if resolved.resolverUnavailable {
		return ToolDispatchResult{Status: "FAILED", VisibleText: "capability resolver unavailable", Error: &ToolDispatchError{Code: "CAPABILITY_RESOLVER_UNAVAILABLE", Message: string(def.ID)}}, true, nil
	}
	if resolved.missingCapability != "" {
		return ToolDispatchResult{Status: "FAILED", VisibleText: fmt.Sprintf("capability not available: %s", resolved.missingCapability), Error: &ToolDispatchError{Code: "CAPABILITY_NOT_REGISTERED", Message: string(resolved.missingCapability)}}, true, nil
	}
	streamMetadata := map[string]any{"execution_mode": "capability_resolved"}
	approvalMode := capabilityApprovalMode(scope.PermissionMode)
	invocation := capability.NewToolInvocationContext(capability.ToolInvocationOptions{
		ExternalCallID:  scope.ToolCallID,
		SpaceID:         scope.SpaceID,
		CharacterID:     scope.CharacterID,
		ConversationID:  scope.ConversationID,
		Channel:         scope.Channel,
		SessionID:       scope.SessionID,
		ExtensionID:     def.ExtensionID,
		ModuleID:        def.ModuleID,
		Source:          capability.InvocationSourceModel,
		ApprovalMode:    approvalMode,
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
	legacy := unifiedResultToDispatch(result)
	return legacy, true, err
}

func capabilityApprovalMode(mode string) capability.ApprovalMode {
	switch agentpermission.Normalize(mode) {
	case agentpermission.FullAccess:
		return capability.ApprovalModeAuto
	default:
		return capability.ApprovalModeManual
	}
}

func unifiedResultToDispatch(result capability.UnifiedToolResult) ToolDispatchResult {
	dispatch := ToolDispatchResult{
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
		dispatch.VisibleText = text
	}
	if result.Error != nil {
		dispatch.Error = &ToolDispatchError{
			Code:      result.Error.Code,
			Message:   result.Error.Message,
			Retryable: result.Error.Retryable,
		}
	}
	return dispatch
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

func (f *ToolFacade) CancelModelTool(ctx context.Context, scope InvocationScope, toolCallID string) execution.CancellationResult {
	reason := capability.ToolCancellationReason{
		Code: capability.CancellationReasonUserRequested,
	}
	var kernelInterface interface{} = f.executionKernel
	if cancellable, ok := kernelInterface.(execution.CancellableExecutionSecurityKernel); ok {
		externalScope := capability.CancellationExternalScope{
			SpaceID:        scope.SpaceID,
			CharacterID:    scope.CharacterID,
			ConversationID: scope.ConversationID,
			SessionID:      scope.SessionID,
		}
		return cancellable.CancelExternalCall(ctx, externalScope, toolCallID, reason)
	}
	return execution.CancellationResult{Requested: false}
}

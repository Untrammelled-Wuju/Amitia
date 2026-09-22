package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/mcp"
)

type chatToolRuntimeAdapter struct {
	facade *kernel.ToolFacade
}

var _ chat.ModelToolRuntime = (*chatToolRuntimeAdapter)(nil)

func newChatToolRuntimeAdapter(facade *kernel.ToolFacade) *chatToolRuntimeAdapter {
	return &chatToolRuntimeAdapter{facade: facade}
}

func (a *chatToolRuntimeAdapter) toInvocationScope(scope chat.SkillScope) kernel.InvocationScope {
	deviceID, runtimeID := "", ""
	if scope.ExecContext != nil && scope.ExecContext.RuntimeTarget != nil {
		deviceID = string(scope.ExecContext.RuntimeTarget.DeviceID)
		runtimeID = string(scope.ExecContext.RuntimeTarget.RuntimeID)
	}
	return kernel.InvocationScope{
		SpaceID:        scope.SpaceID,
		DeviceID:       deviceID,
		RuntimeID:      runtimeID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Channel:        scope.Channel,
		SessionID:      scope.SessionID,
		Message:        scope.Message,
		Source:         scope.Source,
		IsInternal:     scope.IsInternal,
		Trigger:        scope.Trigger,
		TraceID:        scope.TraceID,
		RequestID:      scope.RequestID,
		ToolCallID:     scope.ToolCallID,
		CorrelationID:  scope.CorrelationID,
		CausationID:    scope.CausationID,
		PermissionMode: scope.PermissionMode,
		ExecContext:    scope.ExecContext,
	}
}

func (a *chatToolRuntimeAdapter) toChatActivated(items []kernel.ActivatedSkill) []chat.ActivatedSkill {
	result := make([]chat.ActivatedSkill, 0, len(items))
	for _, item := range items {
		result = append(result, chat.ActivatedSkill{
			ActivationID:        item.ActivationID,
			ExtensionID:         item.ExtensionID,
			Name:                item.Name,
			Source:              item.Source,
			Scope:               item.Scope,
			CompatibilityStatus: item.CompatibilityStatus,
			Prompt:              item.Prompt,
			BodyTokens:          item.BodyTokens,
			Explicit:            item.Explicit,
			ToolMappings:        item.ToolMappings,
		})
	}
	return result
}

func (a *chatToolRuntimeAdapter) toChatContributions(items []kernel.ContextContribution) []chat.ContextContribution {
	result := make([]chat.ContextContribution, 0, len(items))
	for _, c := range items {
		result = append(result, chat.ContextContribution{
			Source:     c.Source,
			Priority:   c.Priority,
			Content:    c.Content,
			TokenLimit: c.TokenLimit,
			ExpiresAt:  c.ExpiresAt,
			Metadata:   c.Metadata,
		})
	}
	return result
}

func (a *chatToolRuntimeAdapter) toChatResult(r kernel.ToolDispatchResult) chat.ToolResult {
	result := chat.ToolResult{
		RunID:       r.RunID,
		Status:      r.Status,
		Output:      r.Output,
		DurationMS:  r.DurationMS,
		VisibleText: r.VisibleText,
		ForceVoice:  r.ForceVoice,
	}
	if r.Error != nil {
		result.Error = &chat.ToolError{
			Code:      r.Error.Code,
			Message:   r.Error.Message,
			Detail:    r.Error.Detail,
			Retryable: r.Error.Retryable,
		}
	}
	return result
}

func (a *chatToolRuntimeAdapter) PrepareAgentSkillPrompt(ctx context.Context, scope chat.SkillScope, message string) (string, []chat.ActivatedSkill, []string) {
	catalog, activated, errs := a.facade.PrepareAgentSkillPrompt(ctx, a.toInvocationScope(scope), message)
	return catalog, a.toChatActivated(activated), errs
}

func (a *chatToolRuntimeAdapter) EndAgentSkillRound(scope chat.SkillScope) {
	a.facade.EndAgentSkillRound(a.toInvocationScope(scope))
}

func (a *chatToolRuntimeAdapter) BeforePrompt(ctx context.Context, scope chat.SkillScope) []chat.ContextContribution {
	return a.toChatContributions(a.facade.BeforePrompt(ctx, a.toInvocationScope(scope)))
}

func (a *chatToolRuntimeAdapter) ModelTools(ctx context.Context, scope chat.SkillScope) ([]tool.Tool, error) {
	return a.facade.ModelTools(ctx, a.toInvocationScope(scope))
}

func (a *chatToolRuntimeAdapter) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope chat.SkillScope, idempotencyKey string) (chat.ToolResult, bool) {
	result, found := a.facade.ExecuteModelTool(ctx, modelName, input, a.toInvocationScope(scope), idempotencyKey)
	return a.toChatResult(result), found
}

type chatToolProgressSink struct {
	emit func(context.Context, chat.ToolProgressEvent) error
}

func (s chatToolProgressSink) Emit(ctx context.Context, event capability.ToolStreamEvent) error {
	if s.emit == nil || event.Type != capability.ToolStreamEventProgress || event.Progress == nil {
		return nil
	}
	return s.emit(ctx, chat.ToolProgressEvent{
		Fraction:      event.Progress.Fraction,
		Indeterminate: event.Progress.Indeterminate,
		Message:       event.Progress.Message,
		Metadata:      event.Metadata,
	})
}

func (a *chatToolRuntimeAdapter) ExecuteModelToolWithProgress(ctx context.Context, modelName string, input json.RawMessage, scope chat.SkillScope, idempotencyKey string, emit func(context.Context, chat.ToolProgressEvent) error) (chat.ToolResult, bool, error) {
	result, streamed, err := a.facade.ExecuteModelToolStream(ctx, modelName, input, a.toInvocationScope(scope), idempotencyKey, chatToolProgressSink{emit: emit})
	found := streamed
	if !found && (result.Error == nil || result.Error.Code != "TOOL_NOT_FOUND") {
		found = true
	}
	return a.toChatResult(result), found, err
}

func (a *chatToolRuntimeAdapter) IsModelToolParallelSafe(ctx context.Context, modelName string, scope chat.SkillScope) bool {
	return a.facade.IsModelToolParallelSafe(ctx, modelName, a.toInvocationScope(scope))
}

func (a *chatToolRuntimeAdapter) AfterReply(scope chat.SkillScope, reply chat.ReplyView) bool {
	return a.facade.AfterReply(a.toInvocationScope(scope), kernel.ReplyView{
		MessageID:      reply.MessageID,
		CharacterID:    reply.CharacterID,
		ConversationID: reply.ConversationID,
		Channel:        reply.Channel,
		Content:        reply.Content,
		CreatedAt:      reply.CreatedAt,
	})
}

func (a *chatToolRuntimeAdapter) PlanMessageOutputs(ctx context.Context, scope chat.SkillScope, event *chat.MessageOutputPlanningEvent) ([]chat.MessageOutput, error) {
	if a == nil || a.facade == nil || event == nil {
		return nil, nil
	}
	input, err := json.Marshal(map[string]any{
		"conversationId": event.ConversationID,
		"characterId":    event.CharacterID,
		"channel":        event.Channel,
		"source":         event.Source,
		"userMessage":    event.UserMessage,
		"reply":          event.Reply,
		"spaceId":        event.SpaceID,
		"peerId":         event.PeerID,
		"requestId":      event.RequestID,
		"forceVoice":     event.ForceVoice,
	})
	if err != nil {
		return nil, err
	}
	results := a.facade.ExecuteMessageOutputProviders(ctx, a.toInvocationScope(scope), input)
	outputs := make([]chat.MessageOutput, 0)
	for _, providerResult := range results {
		if providerResult.Result.Error != nil || !strings.EqualFold(providerResult.Result.Status, "SUCCESS") || len(providerResult.Result.Output) == 0 {
			continue
		}
		var envelope struct {
			Outputs []chat.MessageOutput `json:"outputs"`
		}
		if err := json.Unmarshal(providerResult.Result.Output, &envelope); err != nil {
			var direct []chat.MessageOutput
			if directErr := json.Unmarshal(providerResult.Result.Output, &direct); directErr != nil {
				continue
			}
			envelope.Outputs = direct
		}
		for _, output := range envelope.Outputs {
			if output.ExtensionID == "" {
				output.ExtensionID = providerResult.Provider.ExtensionID
			}
			if output.OutputID == "" {
				output.OutputID = providerResult.Provider.ID
			}
			outputs = append(outputs, output)
		}
	}
	return outputs, nil
}

type mcpDuplicateMetricAdapter struct {
	store *mcp.DuplicateStore
}

var _ kernel.MCPDuplicateMetricProvider = (*mcpDuplicateMetricAdapter)(nil)

func (a *mcpDuplicateMetricAdapter) CountUnresolved(ctx context.Context) (int64, error) {
	return a.store.CountUnresolved(ctx)
}

func (a *mcpDuplicateMetricAdapter) ListUnresolved(ctx context.Context) ([]kernel.MCPDuplicateDetail, error) {
	records, err := a.store.ListUnresolved(ctx)
	if err != nil {
		return nil, err
	}
	details := make([]kernel.MCPDuplicateDetail, len(records))
	for i, r := range records {
		details[i] = kernel.MCPDuplicateDetail{
			ToolID:     r.ToolID,
			ServerID:   r.ServerID,
			Owner:      r.Owner,
			Generation: r.Generation,
			DetectedAt: r.DetectedAt,
		}
	}
	return details, nil
}

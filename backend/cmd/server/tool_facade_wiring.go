package main

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/mcp"
)

type chatToolRuntimeAdapter struct {
	facade *kernel.ToolFacade
}

var _ chat.ModelToolRuntime = (*chatToolRuntimeAdapter)(nil)

func newChatToolRuntimeAdapter(facade *kernel.ToolFacade) *chatToolRuntimeAdapter {
	return &chatToolRuntimeAdapter{facade: facade}
}

func (a *chatToolRuntimeAdapter) toLegacyScope(scope chat.SkillScope) kernel.LegacyScope {
	return kernel.LegacyScope{
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Channel:        scope.Channel,
		SessionID:      scope.SessionID,
		Trigger:        scope.Trigger,
		TraceID:        scope.TraceID,
		RequestID:      scope.RequestID,
		ToolCallID:     scope.ToolCallID,
		CorrelationID:  scope.CorrelationID,
		CausationID:    scope.CausationID,
		ExecContext:    scope.ExecContext,
	}
}

func (a *chatToolRuntimeAdapter) toChatActivated(items []kernel.LegacyActivatedSkill) []chat.ActivatedSkill {
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

func (a *chatToolRuntimeAdapter) toChatContributions(items []kernel.LegacyContextContribution) []chat.ContextContribution {
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

func (a *chatToolRuntimeAdapter) toChatResult(r kernel.LegacyToolResult) chat.ToolResult {
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
	catalog, activated, errs := a.facade.PrepareAgentSkillPrompt(ctx, a.toLegacyScope(scope), message)
	return catalog, a.toChatActivated(activated), errs
}

func (a *chatToolRuntimeAdapter) EndAgentSkillRound(scope chat.SkillScope) {
	a.facade.EndAgentSkillRound(a.toLegacyScope(scope))
}

func (a *chatToolRuntimeAdapter) BeforePrompt(ctx context.Context, scope chat.SkillScope) []chat.ContextContribution {
	return a.toChatContributions(a.facade.BeforePrompt(ctx, a.toLegacyScope(scope)))
}

func (a *chatToolRuntimeAdapter) ModelTools(ctx context.Context, scope chat.SkillScope) ([]tool.Tool, error) {
	return a.facade.ModelTools(ctx, a.toLegacyScope(scope))
}

func (a *chatToolRuntimeAdapter) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope chat.SkillScope, idempotencyKey string) (chat.ToolResult, bool) {
	result, found := a.facade.ExecuteModelTool(ctx, modelName, input, a.toLegacyScope(scope), idempotencyKey)
	return a.toChatResult(result), found
}

func (a *chatToolRuntimeAdapter) AfterReply(scope chat.SkillScope, reply chat.ReplyView) bool {
	return a.facade.AfterReply(a.toLegacyScope(scope), kernel.LegacyReplyView{
		MessageID:      reply.MessageID,
		CharacterID:    reply.CharacterID,
		ConversationID: reply.ConversationID,
		Channel:        reply.Channel,
		Content:        reply.Content,
		CreatedAt:      reply.CreatedAt,
	})
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

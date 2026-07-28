package kernel

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
)

type LegacyScope struct {
	UserID         string
	CharacterID    string
	ConversationID string
	Channel        string
	SessionID      string
	Trigger        string
	TraceID        string
	RequestID      string
	ToolCallID     string
	CorrelationID  string
	CausationID    string
}

type LegacyContextContribution struct {
	Source     string
	Priority   int
	Content    string
	TokenLimit int
	ExpiresAt  *time.Time
	Metadata   map[string]string
}

type LegacyReplyView struct {
	MessageID      string
	CharacterID    string
	ConversationID string
	Channel        string
	Content        string
	CreatedAt      time.Time
}

type LegacyToolError struct {
	Code      string
	Message   string
	Detail    string
	Retryable bool
}

type LegacyToolResult struct {
	RunID       string
	Status      string
	Output      json.RawMessage
	Error       *LegacyToolError
	DurationMS  int64
	VisibleText string
	ForceVoice  bool
}

type LegacyActivatedSkill struct {
	ActivationID        string
	ExtensionID         string
	Name                string
	Source              string
	Scope               string
	CompatibilityStatus string
	Prompt              string
	BodyTokens          int
	Explicit            bool
	ToolMappings        []map[string]any
}

type LegacyToolDispatcher interface {
	PrepareAgentSkillPrompt(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string)
	EndAgentSkillRound(scope LegacyScope)
	BeforePrompt(ctx context.Context, scope LegacyScope) []LegacyContextContribution
	ModelTools(ctx context.Context, scope LegacyScope) ([]tool.Tool, error)
	ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope LegacyScope, idempotencyKey string) (LegacyToolResult, bool)
	AfterReply(scope LegacyScope, reply LegacyReplyView) bool
}

type LegacyToolDispatcherFuncs struct {
	PrepareAgentSkillPromptFn func(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string)
	EndAgentSkillRoundFn      func(scope LegacyScope)
	BeforePromptFn            func(ctx context.Context, scope LegacyScope) []LegacyContextContribution
	ModelToolsFn              func(ctx context.Context, scope LegacyScope) ([]tool.Tool, error)
	ExecuteModelToolFn        func(ctx context.Context, modelName string, input json.RawMessage, scope LegacyScope, idempotencyKey string) (LegacyToolResult, bool)
	AfterReplyFn              func(scope LegacyScope, reply LegacyReplyView) bool
}

var _ LegacyToolDispatcher = (*LegacyToolDispatcherFuncs)(nil)

func (f *LegacyToolDispatcherFuncs) PrepareAgentSkillPrompt(ctx context.Context, scope LegacyScope, message string) (string, []LegacyActivatedSkill, []string) {
	if f == nil || f.PrepareAgentSkillPromptFn == nil {
		return "", nil, nil
	}
	return f.PrepareAgentSkillPromptFn(ctx, scope, message)
}

func (f *LegacyToolDispatcherFuncs) EndAgentSkillRound(scope LegacyScope) {
	if f == nil || f.EndAgentSkillRoundFn == nil {
		return
	}
	f.EndAgentSkillRoundFn(scope)
}

func (f *LegacyToolDispatcherFuncs) BeforePrompt(ctx context.Context, scope LegacyScope) []LegacyContextContribution {
	if f == nil || f.BeforePromptFn == nil {
		return nil
	}
	return f.BeforePromptFn(ctx, scope)
}

func (f *LegacyToolDispatcherFuncs) ModelTools(ctx context.Context, scope LegacyScope) ([]tool.Tool, error) {
	if f == nil || f.ModelToolsFn == nil {
		return nil, nil
	}
	return f.ModelToolsFn(ctx, scope)
}

func (f *LegacyToolDispatcherFuncs) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope LegacyScope, idempotencyKey string) (LegacyToolResult, bool) {
	if f == nil || f.ExecuteModelToolFn == nil {
		return LegacyToolResult{Status: "FAILED", VisibleText: "legacy dispatcher not configured", Error: &LegacyToolError{Code: "LEGACY_DISPATCHER_UNAVAILABLE"}}, false
	}
	return f.ExecuteModelToolFn(ctx, modelName, input, scope, idempotencyKey)
}

func (f *LegacyToolDispatcherFuncs) AfterReply(scope LegacyScope, reply LegacyReplyView) bool {
	if f == nil || f.AfterReplyFn == nil {
		return false
	}
	return f.AfterReplyFn(scope, reply)
}

package kernel

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	coreexec "github.com/u-ai/backend/internal/execution"
)

type InvocationScope struct {
	SpaceID        string
	DeviceID       string
	RuntimeID      string
	PrincipalType  string
	CharacterID    string
	ConversationID string
	Channel        string
	SessionID      string
	Message        string
	Source         string
	IsInternal     bool
	Trigger        string
	TraceID        string
	RequestID      string
	ToolCallID     string
	CorrelationID  string
	CausationID    string
	PermissionMode string
	ExecContext    *coreexec.ExecutionContext
}

type ContextContribution struct {
	Source     string
	Priority   int
	Content    string
	TokenLimit int
	ExpiresAt  *time.Time
	Metadata   map[string]string
}

type ReplyView struct {
	MessageID      string
	CharacterID    string
	ConversationID string
	Channel        string
	Content        string
	CreatedAt      time.Time
}

type ToolDispatchError struct {
	Code      string
	Message   string
	Detail    string
	Retryable bool
}

type ToolDispatchResult struct {
	RunID       string
	Status      string
	Output      json.RawMessage
	Error       *ToolDispatchError
	DurationMS  int64
	VisibleText string
	ForceVoice  bool
}

type ActivatedSkill struct {
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

type CapabilityDispatcher interface {
	PrepareAgentSkillPrompt(ctx context.Context, scope InvocationScope, message string) (string, []ActivatedSkill, []string)
	EndAgentSkillRound(scope InvocationScope)
	BeforePrompt(ctx context.Context, scope InvocationScope) []ContextContribution
	ModelTools(ctx context.Context, scope InvocationScope) ([]tool.Tool, error)
	ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope InvocationScope, idempotencyKey string) (ToolDispatchResult, bool)
	AfterReply(scope InvocationScope, reply ReplyView) bool
}

type CapabilityDispatcherFuncs struct {
	PrepareAgentSkillPromptFn func(ctx context.Context, scope InvocationScope, message string) (string, []ActivatedSkill, []string)
	EndAgentSkillRoundFn      func(scope InvocationScope)
	BeforePromptFn            func(ctx context.Context, scope InvocationScope) []ContextContribution
	ModelToolsFn              func(ctx context.Context, scope InvocationScope) ([]tool.Tool, error)
	ExecuteModelToolFn        func(ctx context.Context, modelName string, input json.RawMessage, scope InvocationScope, idempotencyKey string) (ToolDispatchResult, bool)
	AfterReplyFn              func(scope InvocationScope, reply ReplyView) bool
}

var _ CapabilityDispatcher = (*CapabilityDispatcherFuncs)(nil)

func (f *CapabilityDispatcherFuncs) PrepareAgentSkillPrompt(ctx context.Context, scope InvocationScope, message string) (string, []ActivatedSkill, []string) {
	if f == nil || f.PrepareAgentSkillPromptFn == nil {
		return "", nil, nil
	}
	return f.PrepareAgentSkillPromptFn(ctx, scope, message)
}

func (f *CapabilityDispatcherFuncs) EndAgentSkillRound(scope InvocationScope) {
	if f == nil || f.EndAgentSkillRoundFn == nil {
		return
	}
	f.EndAgentSkillRoundFn(scope)
}

func (f *CapabilityDispatcherFuncs) BeforePrompt(ctx context.Context, scope InvocationScope) []ContextContribution {
	if f == nil || f.BeforePromptFn == nil {
		return nil
	}
	return f.BeforePromptFn(ctx, scope)
}

func (f *CapabilityDispatcherFuncs) ModelTools(ctx context.Context, scope InvocationScope) ([]tool.Tool, error) {
	if f == nil || f.ModelToolsFn == nil {
		return nil, nil
	}
	return f.ModelToolsFn(ctx, scope)
}

func (f *CapabilityDispatcherFuncs) ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope InvocationScope, idempotencyKey string) (ToolDispatchResult, bool) {
	if f == nil || f.ExecuteModelToolFn == nil {
		return ToolDispatchResult{Status: "FAILED", VisibleText: "capability dispatcher not configured", Error: &ToolDispatchError{Code: "CAPABILITY_DISPATCHER_UNAVAILABLE"}}, false
	}
	return f.ExecuteModelToolFn(ctx, modelName, input, scope, idempotencyKey)
}

func (f *CapabilityDispatcherFuncs) AfterReply(scope InvocationScope, reply ReplyView) bool {
	if f == nil || f.AfterReplyFn == nil {
		return false
	}
	return f.AfterReplyFn(scope, reply)
}

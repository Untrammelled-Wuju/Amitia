package chat

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension"
)

type SkillScope struct {
	SpaceID        string
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

type ToolError struct {
	Code      string
	Message   string
	Detail    string
	Retryable bool
}

type ToolResult struct {
	RunID       string
	Status      string
	Output      json.RawMessage
	Error       *ToolError
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

type ModelToolRuntime interface {
	PrepareAgentSkillPrompt(ctx context.Context, scope SkillScope, message string) (string, []ActivatedSkill, []string)
	EndAgentSkillRound(scope SkillScope)
	BeforePrompt(ctx context.Context, scope SkillScope) []ContextContribution
	ModelTools(ctx context.Context, scope SkillScope) ([]tool.Tool, error)
	ExecuteModelTool(ctx context.Context, modelName string, input json.RawMessage, scope SkillScope, idempotencyKey string) (ToolResult, bool)
	AfterReply(scope SkillScope, reply ReplyView) bool
}

func toolScopeFromExtension(es extension.ExecutionScope) SkillScope {
	return SkillScope{
		SpaceID:        es.SpaceID,
		CharacterID:    es.CharacterID,
		ConversationID: es.ConversationID,
		Channel:        es.Channel,
		SessionID:      es.SessionID,
		Trigger:        string(es.Trigger),
		TraceID:        es.TraceID,
		RequestID:      es.RequestID,
		ToolCallID:     es.ToolCallID,
		CorrelationID:  es.CorrelationID,
		CausationID:    es.CausationID,
		ExecContext:    es.ExecContext,
	}
}

type toolExecOutcome struct {
	VisibleText string
	Status      string
	ForceVoice  bool
	ErrorCode   string
	Output      json.RawMessage
	HasError    bool
	Found       bool
}

func toolResultToOutcome(r ToolResult, found bool) toolExecOutcome {
	out := toolExecOutcome{
		VisibleText: r.VisibleText,
		Status:      r.Status,
		ForceVoice:  r.ForceVoice,
		Output:      r.Output,
		Found:       found,
	}
	if r.Error != nil {
		out.ErrorCode = r.Error.Code
		out.HasError = true
	}
	return out
}

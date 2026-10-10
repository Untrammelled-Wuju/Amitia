package chat

import (
	"context"
	"encoding/json"
	"strings"
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

type ToolProgressEvent struct {
	Fraction      float64
	Indeterminate bool
	Message       string
	Metadata      map[string]any
}

type ModelToolProgressRuntime interface {
	ExecuteModelToolWithProgress(ctx context.Context, modelName string, input json.RawMessage, scope SkillScope, idempotencyKey string, emit func(context.Context, ToolProgressEvent) error) (ToolResult, bool, error)
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
	VisibleText  string
	Status       string
	ForceVoice   bool
	ErrorCode    string
	ErrorMessage string
	Output       json.RawMessage
	HasError     bool
	Found        bool
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
		out.ErrorMessage = r.Error.Message
		out.HasError = true
	}
	switch strings.ToUpper(strings.TrimSpace(r.Status)) {
	case "FAILED", "FAILURE", "ERROR", "DENIED", "REJECTED", "CANCELLED", "CANCELED", "TIMED_OUT", "TIMEOUT", "NOT_AVAILABLE", "UNAVAILABLE", "UNKNOWN", "UNCERTAIN":
		out.HasError = true
		if out.ErrorCode == "" {
			out.ErrorCode = "TOOL_" + strings.ToUpper(strings.TrimSpace(r.Status))
		}
		if out.ErrorMessage == "" {
			out.ErrorMessage = strings.TrimSpace(r.VisibleText)
		}
	}
	if found && len(r.Output) > 0 {
		var evidence struct {
			ExitCode *int   `json:"exitCode"`
			TimedOut bool   `json:"timedOut"`
			Stderr   string `json:"stderr"`
		}
		if json.Unmarshal(r.Output, &evidence) == nil {
			if evidence.ExitCode != nil && *evidence.ExitCode != 0 {
				out.HasError = true
				if out.ErrorCode == "" {
					out.ErrorCode = "TOOL_NONZERO_EXIT"
				}
				if out.ErrorMessage == "" {
					out.ErrorMessage = strings.TrimSpace(evidence.Stderr)
					if out.ErrorMessage == "" {
						out.ErrorMessage = "command exited with nonzero status"
					}
				}
			}
			if evidence.TimedOut {
				out.HasError = true
				out.ErrorCode = "TOOL_TIMED_OUT"
				if out.ErrorMessage == "" {
					out.ErrorMessage = "command exceeded its time limit"
				}
			}
		}
	}
	if !found {
		out.HasError = true
		if out.ErrorCode == "" {
			out.ErrorCode = "TOOL_NOT_FOUND"
		}
	}
	return out
}

func toolResultContent(toolName string, outcome toolExecOutcome) string {
	if outcome.HasError || !outcome.Found {
		code := strings.TrimSpace(outcome.ErrorCode)
		message := strings.TrimSpace(outcome.ErrorMessage)
		if message == "" {
			message = strings.TrimSpace(outcome.VisibleText)
		}
		if code == "" && message == "" {
			return "工具执行失败"
		}
		if code == "" {
			return "工具执行失败：" + message
		}
		if message == "" {
			return "工具执行失败：" + code
		}
		return "工具执行失败：" + code + ": " + message
	}
	if structuredToolResult(toolName) {
		if payload := strings.TrimSpace(string(outcome.Output)); payload != "" {
			return payload
		}
	}
	if strings.TrimSpace(outcome.VisibleText) != "" {
		return outcome.VisibleText
	}
	if !outcome.HasError && outcome.Found {
		return ""
	}
	if code := strings.TrimSpace(outcome.ErrorCode); code != "" {
		if message := strings.TrimSpace(outcome.ErrorMessage); message != "" {
			return "工具执行失败：" + code + ": " + message
		}
		return "工具执行失败：" + code
	}
	return "工具执行失败"
}

func structuredToolResult(toolName string) bool {
	switch toolName {
	case "web_run",
		"web.run",
		"harness_multi_agent_delegate",
		"harness_multi_agent_status",
		"harness_multi_agent_wait",
		"execute_host_command",
		"execute_terminal",
		"execute_in_terminal_session_streaming",
		"media_image_generate",
		"media.image.generate",
		"send_attachment",
		"find_capability",
		"acquire_capability",
		"use_package",
		"run_skill_script",
		"list_skill_resources",
		"read_skill_resource",
		"materialize_skill_resource":
		return true
	default:
		return strings.HasPrefix(toolName, "android_ui_tree_") ||
			strings.HasPrefix(toolName, "android_interaction_") ||
			strings.HasPrefix(toolName, "android_virtual_display_") ||
			strings.HasPrefix(toolName, "android_display_") ||
			strings.HasPrefix(toolName, "android_accessibility_")
	}
}

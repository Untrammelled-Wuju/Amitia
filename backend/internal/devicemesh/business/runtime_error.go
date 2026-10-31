package business

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

type RuntimeError struct {
	ErrorCode       string `json:"errorCode"`
	UserMessage     string `json:"userMessage"`
	InternalMessage string `json:"internalMessage,omitempty"`
	Stage           string `json:"stage,omitempty"`
	ToolName        string `json:"toolName,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Retryable       bool   `json:"retryable"`
}

func DescribeRuntimeError(err error) RuntimeError {
	if err == nil {
		return RuntimeError{ErrorCode: "task_failed", UserMessage: "nil error", Stage: "execution"}
	}
	raw := boundedRuntimeMessage(err.Error())
	lower := strings.ToLower(raw)
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		return RuntimeError{ErrorCode: "task_timeout", UserMessage: raw, InternalMessage: raw, Stage: "execution", Retryable: true}
	case errors.Is(err, ErrInterrupted) || errors.Is(err, context.Canceled):
		return RuntimeError{ErrorCode: "task_interrupted", UserMessage: raw, InternalMessage: raw, Stage: "execution", Retryable: true}
	case strings.Contains(lower, "typed browser tool is unavailable") || strings.Contains(lower, "browser runtime is disabled") || strings.Contains(lower, "browser agent runner is not configured") || strings.Contains(lower, "browser_agent_unavailable"):
		return RuntimeError{ErrorCode: "browser_runtime_unavailable", UserMessage: raw, InternalMessage: raw, Stage: "tool", ToolName: "browser.agent.run", Provider: "browser", Retryable: true}
	case strings.Contains(lower, "context exceeds") || strings.Contains(lower, "context window") || strings.Contains(lower, "too many tokens"):
		return RuntimeError{ErrorCode: "task_context_exceeded", UserMessage: raw, InternalMessage: raw, Stage: "model", Provider: "model", Retryable: true}
	case strings.Contains(lower, "tool loop") || strings.Contains(lower, "repeated tool failure"):
		return RuntimeError{ErrorCode: "task_tool_loop_limit", UserMessage: raw, InternalMessage: raw, Stage: "tool", Retryable: true}
	case strings.Contains(lower, "network") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "tls") || strings.Contains(lower, "fetch failed") || strings.Contains(lower, "unexpected eof") || strings.Contains(lower, "broken pipe"):
		return RuntimeError{ErrorCode: "model_network_error", UserMessage: raw, InternalMessage: raw, Stage: "model", Provider: "model", Retryable: true}
	case strings.Contains(lower, "workspace changes were applied but no successful") || strings.Contains(lower, "verification"):
		return RuntimeError{ErrorCode: "task_verification_failed", UserMessage: raw, InternalMessage: raw, Stage: "verification", Retryable: true}
	}
	return RuntimeError{ErrorCode: "task_execution_failed", UserMessage: raw, InternalMessage: raw, Stage: "execution", Retryable: true}
}

func boundedRuntimeMessage(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= 500 {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:500])) + "…"
}

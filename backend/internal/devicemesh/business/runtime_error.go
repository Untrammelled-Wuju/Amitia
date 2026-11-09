package business

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type RuntimeError struct {
	ErrorCode       string `json:"errorCode"`
	UserMessage     string `json:"userMessage"`
	InternalMessage string `json:"internalMessage,omitempty"`
	Message         string `json:"message,omitempty"`
	Error           string `json:"error,omitempty"`
	Detail          string `json:"detail,omitempty"`
	Cause           string `json:"cause,omitempty"`
	RawError        string `json:"rawError,omitempty"`
	Stage           string `json:"stage,omitempty"`
	ToolName        string `json:"toolName,omitempty"`
	Provider        string `json:"provider,omitempty"`
	Retryable       bool   `json:"retryable"`
}

func DescribeRuntimeError(err error) RuntimeError {
	if err == nil {
		return RuntimeError{ErrorCode: "task_failed", UserMessage: "nil error", Stage: "execution"}
	}
	raw := err.Error()
	detail := fmt.Sprintf("%+v", err)
	result := RuntimeError{
		ErrorCode:       "task_execution_failed",
		UserMessage:     raw,
		InternalMessage: raw,
		Message:         raw,
		Error:           raw,
		Detail:          detail,
		Cause:           errorCauses(err),
		RawError:        raw,
		Stage:           "execution",
		Retryable:       true,
	}
	lower := strings.ToLower(raw)
	switch {
	case errors.Is(err, ErrInvalidRequest):
		result.ErrorCode = "invalid_request"
		result.Stage = "validation"
		result.Retryable = false
	case errors.Is(err, ErrRequestInFlight):
		result.ErrorCode = "request_in_progress"
		result.Stage = "admission"
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		result.ErrorCode = "task_timeout"
	case errors.Is(err, ErrInterrupted) || errors.Is(err, context.Canceled):
		result.ErrorCode = "task_interrupted"
	case strings.Contains(lower, "typed browser tool is unavailable") || strings.Contains(lower, "browser runtime is disabled") || strings.Contains(lower, "browser agent runner is not configured") || strings.Contains(lower, "browser_agent_unavailable"):
		result.ErrorCode = "browser_runtime_unavailable"
		result.Stage = "tool"
		result.ToolName = "browser.agent.run"
		result.Provider = "browser"
	case strings.Contains(lower, "context exceeds") || strings.Contains(lower, "context window") || strings.Contains(lower, "too many tokens"):
		result.ErrorCode = "task_context_exceeded"
		result.Stage = "model"
		result.Provider = "model"
	case strings.Contains(lower, "没有可用的模型配置"):
		result.ErrorCode = "model_not_configured"
		result.Stage = "model"
		result.Provider = "model"
		result.Retryable = false
	case strings.Contains(lower, "tool loop") || strings.Contains(lower, "repeated tool failure"):
		result.ErrorCode = "task_tool_loop_limit"
		result.Stage = "tool"
	case strings.Contains(lower, "network") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "tls") || strings.Contains(lower, "fetch failed") || strings.Contains(lower, "unexpected eof") || strings.Contains(lower, "broken pipe"):
		result.ErrorCode = "model_network_error"
		result.Stage = "model"
		result.Provider = "model"
	case strings.Contains(lower, "workspace changes were applied but no successful") || strings.Contains(lower, "verification"):
		result.ErrorCode = "task_verification_failed"
		result.Stage = "verification"
	}
	return result
}

func errorCauses(err error) string {
	if err == nil {
		return ""
	}
	causes := make([]string, 0, 4)
	var collect func(error)
	collect = func(current error) {
		if current == nil {
			return
		}
		message := strings.TrimSpace(current.Error())
		if message != "" && (len(causes) == 0 || causes[len(causes)-1] != message) {
			causes = append(causes, message)
		}
		switch wrapped := current.(type) {
		case interface{ Unwrap() []error }:
			for _, child := range wrapped.Unwrap() {
				collect(child)
			}
		case interface{ Unwrap() error }:
			collect(wrapped.Unwrap())
		}
	}
	collect(err)
	return strings.Join(causes, "\n")
}

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
		return RuntimeError{ErrorCode: "task_failed", UserMessage: "任务执行失败，未返回具体原因。", Stage: "execution"}
	}
	raw := strings.TrimSpace(err.Error())
	lower := strings.ToLower(raw)
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		return RuntimeError{ErrorCode: "task_timeout", UserMessage: "任务执行超时，尚未完成。请缩小任务范围后重试。", InternalMessage: boundedRuntimeMessage(raw), Stage: "execution", Retryable: true}
	case errors.Is(err, ErrInterrupted) || errors.Is(err, context.Canceled):
		return RuntimeError{ErrorCode: "task_interrupted", UserMessage: "任务已停止。", InternalMessage: boundedRuntimeMessage(raw), Stage: "execution", Retryable: true}
	case strings.Contains(lower, "typed browser tool is unavailable") || strings.Contains(lower, "browser runtime is disabled") || strings.Contains(lower, "browser agent runner is not configured") || strings.Contains(lower, "browser_agent_unavailable"):
		return RuntimeError{ErrorCode: "browser_runtime_unavailable", UserMessage: "浏览器运行时未启用或不可用，无法执行网页自动化任务。请启用浏览器运行时后重试。", InternalMessage: boundedRuntimeMessage(raw), Stage: "tool", ToolName: "browser.agent.run", Provider: "browser", Retryable: true}
	case strings.Contains(lower, "context exceeds") || strings.Contains(lower, "context window") || strings.Contains(lower, "too many tokens"):
		return RuntimeError{ErrorCode: "task_context_exceeded", UserMessage: "任务上下文超出模型容量，尚未完成。请缩小任务范围或新开会话后重试。", InternalMessage: boundedRuntimeMessage(raw), Stage: "model", Provider: "model", Retryable: true}
	case strings.Contains(lower, "tool loop") || strings.Contains(lower, "repeated tool failure"):
		return RuntimeError{ErrorCode: "task_tool_loop_limit", UserMessage: "工具连续执行未取得有效进展，任务已停止以避免无限重试。请根据当前结果调整任务后重新发起。", InternalMessage: boundedRuntimeMessage(raw), Stage: "tool", Retryable: true}
	case strings.Contains(lower, "network") || strings.Contains(lower, "connection reset") || strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host") || strings.Contains(lower, "tls") || strings.Contains(lower, "fetch failed") || strings.Contains(lower, "unexpected eof") || strings.Contains(lower, "broken pipe"):
		return RuntimeError{ErrorCode: "model_network_error", UserMessage: "模型服务网络连接中断，任务尚未完成。请检查网络后重试。", InternalMessage: boundedRuntimeMessage(raw), Stage: "model", Provider: "model", Retryable: true}
	case strings.Contains(lower, "workspace changes were applied but no successful") || strings.Contains(lower, "verification"):
		return RuntimeError{ErrorCode: "task_verification_failed", UserMessage: "任务产生了工作区变更，但缺少成功的测试或构建证据，因此未标记完成。", InternalMessage: boundedRuntimeMessage(raw), Stage: "verification", Retryable: true}
	}
	userMessage := boundedRuntimeMessage(raw)
	if userMessage == "" {
		userMessage = "任务执行失败，未返回具体原因。"
	} else {
		userMessage = "任务执行失败：" + userMessage
	}
	return RuntimeError{ErrorCode: "task_execution_failed", UserMessage: userMessage, InternalMessage: boundedRuntimeMessage(raw), Stage: "execution", Retryable: true}
}

func boundedRuntimeMessage(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= 500 {
		return value
	}
	runes := []rune(value)
	return strings.TrimSpace(string(runes[:500])) + "…"
}

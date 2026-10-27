package business

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDescribeRuntimeErrorPrioritizesActionableFailures(t *testing.T) {
	cases := []struct {
		err       error
		errorCode string
		contains  string
		retryable bool
	}{
		{err: errors.New("typed browser tool is unavailable: browser_session_create"), errorCode: "browser_runtime_unavailable", contains: "浏览器运行时", retryable: true},
		{err: context.DeadlineExceeded, errorCode: "task_timeout", contains: "超时", retryable: true},
		{err: errors.New("tool loop repeated tool failure: browser.agent.run returned browser_agent_failed 3 times"), errorCode: "task_tool_loop_limit", contains: "避免无限重试", retryable: true},
		{err: errors.New("read model stream: connection reset by peer"), errorCode: "model_network_error", contains: "网络连接中断", retryable: true},
		{err: errors.New("agent context exceeds usable context 128000 after reserving output"), errorCode: "task_context_exceeded", contains: "上下文", retryable: true},
		{err: errors.New("tool exploded"), errorCode: "task_execution_failed", contains: "tool exploded", retryable: true},
	}
	for _, item := range cases {
		got := DescribeRuntimeError(item.err)
		if got.ErrorCode != item.errorCode || !strings.Contains(got.UserMessage, item.contains) || got.Retryable != item.retryable {
			t.Fatalf("DescribeRuntimeError(%v) = %+v", item.err, got)
		}
	}
}

func TestDescribeRuntimeErrorBoundsInternalMessage(t *testing.T) {
	got := DescribeRuntimeError(errors.New(strings.Repeat("连", 700)))
	if len([]rune(got.InternalMessage)) != 501 {
		t.Fatalf("internal message rune count = %d", len([]rune(got.InternalMessage)))
	}
}

package capability

import (
	"context"
	"encoding/json"
	"time"
)

type BrowserCallFunc func(ctx context.Context, handlerName string, invocation ToolInvocationContext, input json.RawMessage) (json.RawMessage, error)
type BrowserHealthFunc func(ctx context.Context) HealthStatus

type BrowserRuntimeAdapter struct {
	call   BrowserCallFunc
	health BrowserHealthFunc
}

func NewBrowserRuntimeAdapter(call BrowserCallFunc, health BrowserHealthFunc) *BrowserRuntimeAdapter {
	return &BrowserRuntimeAdapter{call: call, health: health}
}

func (a *BrowserRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeBrowser
}

func (a *BrowserRuntimeAdapter) Execute(
	ctx context.Context,
	binding RuntimeBinding,
	invocation ToolInvocationContext,
	input json.RawMessage,
) UnifiedToolResult {
	if a.call == nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeRuntimeUnavailable,
				Message:     "browser runtime caller not configured",
				UserVisible: false,
			},
		}
	}

	start := time.Now()
	output, err := a.call(ctx, binding.HandlerName, invocation, input)
	duration := time.Since(start)

	if err != nil {
		toolErr := mapBrowserErrorToToolError(err)
		result := UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error:        toolErr,
			DurationMS:   duration.Milliseconds(),
		}
		return result
	}

	return UnifiedToolResult{
		InvocationID: invocation.InvocationID,
		Status:       ToolResultStatusSuccess,
		Content: []ToolContent{
			{Type: ToolContentText, Text: string(output)},
		},
		DurationMS: duration.Milliseconds(),
	}
}

func (a *BrowserRuntimeAdapter) Health(_ context.Context, _ RuntimeBinding) HealthStatus {
	if a.health != nil {
		return a.health(context.Background())
	}
	return HealthUnknown
}

func mapBrowserErrorToToolError(err error) *ToolError {
	if err == nil {
		return nil
	}
	if toolErr, ok := err.(*ToolError); ok {
		return toolErr
	}
	return &ToolError{
		Code:        ErrorCodeExecutionFailed,
		Message:     err.Error(),
		UserVisible: false,
		Retryable:   false,
	}
}

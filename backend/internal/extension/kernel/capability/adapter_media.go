package capability

import (
	"context"
	"encoding/json"
)

type MediaCallFunc func(
	ctx context.Context,
	handlerName string,
	invocation ToolInvocationContext,
	input json.RawMessage,
) (json.RawMessage, error)

type MediaHealthFunc func(
	ctx context.Context,
) HealthStatus

type MediaRuntimeAdapter struct {
	call   MediaCallFunc
	health MediaHealthFunc
}

func NewMediaRuntimeAdapter(call MediaCallFunc, health MediaHealthFunc) *MediaRuntimeAdapter {
	return &MediaRuntimeAdapter{
		call:   call,
		health: health,
	}
}

func (a *MediaRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeMedia
}

func (a *MediaRuntimeAdapter) Execute(
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
				Code:        ErrorCodeExecutionFailed,
				Message:     "media runtime not configured",
				UserVisible: false,
			},
		}
	}
	output, err := a.call(ctx, binding.HandlerName, invocation, input)
	if err != nil {
		if toolErr, ok := err.(*ToolError); ok {
			return UnifiedToolResult{
				InvocationID: invocation.InvocationID,
				Status:       ToolResultStatusFailed,
				Error:        toolErr,
			}
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeExecutionFailed,
				Message:     err.Error(),
				UserVisible: true,
			},
		}
	}
	return UnifiedToolResult{
		InvocationID: invocation.InvocationID,
		Status:       ToolResultStatusSuccess,
		Structured:   output,
	}
}

func (a *MediaRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health == nil {
		return HealthUnknown
	}
	return a.health(ctx)
}

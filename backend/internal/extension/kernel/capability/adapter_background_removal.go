package capability

import (
	"context"
	"encoding/json"
)

type BackgroundRemovalCallFunc func(
	ctx context.Context,
	input json.RawMessage,
) (json.RawMessage, error)

type BackgroundRemovalRuntimeAdapter struct {
	call   BackgroundRemovalCallFunc
	health func(ctx context.Context) HealthStatus
}

func NewBackgroundRemovalRuntimeAdapter(
	call BackgroundRemovalCallFunc,
	health func(ctx context.Context) HealthStatus,
) *BackgroundRemovalRuntimeAdapter {
	return &BackgroundRemovalRuntimeAdapter{
		call:   call,
		health: health,
	}
}

func (a *BackgroundRemovalRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeBackgroundRemoval
}

func (a *BackgroundRemovalRuntimeAdapter) Execute(
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
				Message:     "background removal runtime not configured",
				UserVisible: false,
			},
		}
	}

	output, err := a.call(ctx, input)
	if err != nil {
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
		Content: []ToolContent{
			{Type: ToolContentText, Text: string(output)},
		},
	}
}

func (a *BackgroundRemovalRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health == nil {
		return HealthUnknown
	}
	return a.health(ctx)
}

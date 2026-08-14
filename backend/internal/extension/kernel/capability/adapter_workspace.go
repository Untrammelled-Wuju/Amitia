package capability

import (
	"context"
	"encoding/json"
)

type WorkspaceCallFunc func(
	ctx context.Context,
	handlerName string,
	invocation ToolInvocationContext,
	input json.RawMessage,
) (json.RawMessage, error)

type WorkspaceHealthFunc func(
	ctx context.Context,
) HealthStatus

type WorkspaceRuntimeAdapter struct {
	call   WorkspaceCallFunc
	health WorkspaceHealthFunc
}

func NewWorkspaceRuntimeAdapter(call WorkspaceCallFunc, health WorkspaceHealthFunc) *WorkspaceRuntimeAdapter {
	return &WorkspaceRuntimeAdapter{
		call:   call,
		health: health,
	}
}

func (a *WorkspaceRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeWorkspace
}

func (a *WorkspaceRuntimeAdapter) Execute(
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
				Message:     "workspace runtime not configured",
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

func (a *WorkspaceRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health == nil {
		return HealthUnknown
	}
	return a.health(ctx)
}

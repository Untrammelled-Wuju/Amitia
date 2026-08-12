package capability

import (
	"context"
	"encoding/json"
)

type SearchCallFunc func(
	ctx context.Context,
	providerID string,
	handlerName string,
	invocation ToolInvocationContext,
	input json.RawMessage,
) (json.RawMessage, error)

type SearchHealthFunc func(
	ctx context.Context,
	providerID string,
) HealthStatus

type SearchRuntimeAdapter struct {
	call   SearchCallFunc
	health SearchHealthFunc
}

func NewSearchRuntimeAdapter(call SearchCallFunc, health SearchHealthFunc) *SearchRuntimeAdapter {
	return &SearchRuntimeAdapter{
		call:   call,
		health: health,
	}
}

func (a *SearchRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeSearch
}

func (a *SearchRuntimeAdapter) Execute(
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
				Message:     "search runtime not configured",
				UserVisible: false,
			},
		}
	}
	providerID := binding.RuntimeID
	if providerID == "" {
		providerID = "default"
	}
	output, err := a.call(ctx, providerID, binding.HandlerName, invocation, input)
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

func (a *SearchRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health == nil {
		return HealthUnknown
	}
	providerID := binding.RuntimeID
	if providerID == "" {
		providerID = "default"
	}
	return a.health(ctx, providerID)
}

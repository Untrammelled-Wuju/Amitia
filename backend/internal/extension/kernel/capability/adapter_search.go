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

type SearchStreamCallFunc func(
	ctx context.Context,
	providerID string,
	handlerName string,
	invocation ToolInvocationContext,
	input json.RawMessage,
	emitter ToolStreamEmitter,
) (json.RawMessage, error)

type SearchHealthFunc func(
	ctx context.Context,
	providerID string,
) HealthStatus

type SearchRuntimeAdapter struct {
	call       SearchCallFunc
	streamCall SearchStreamCallFunc
	health     SearchHealthFunc
}

func NewSearchRuntimeAdapter(call SearchCallFunc, health SearchHealthFunc) *SearchRuntimeAdapter {
	return &SearchRuntimeAdapter{
		call:   call,
		health: health,
	}
}

func NewSearchRuntimeAdapterWithStream(call SearchCallFunc, streamCall SearchStreamCallFunc, health SearchHealthFunc) *SearchRuntimeAdapter {
	return &SearchRuntimeAdapter{
		call:       call,
		streamCall: streamCall,
		health:     health,
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
		return searchToolFailure(invocation.InvocationID, err)
	}
	return UnifiedToolResult{
		InvocationID: invocation.InvocationID,
		Status:       ToolResultStatusSuccess,
		Structured:   output,
	}
}

func (a *SearchRuntimeAdapter) ExecuteStream(
	ctx context.Context,
	binding RuntimeBinding,
	invocation ToolInvocationContext,
	input json.RawMessage,
	emitter ToolStreamEmitter,
) UnifiedToolResult {
	if a.streamCall == nil {
		return a.Execute(ctx, binding, invocation, input)
	}
	providerID := binding.RuntimeID
	if providerID == "" {
		providerID = "default"
	}
	output, err := a.streamCall(ctx, providerID, binding.HandlerName, invocation, input, emitter)
	if err != nil {
		return searchToolFailure(invocation.InvocationID, err)
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

func searchToolFailure(invocationID string, err error) UnifiedToolResult {
	if toolErr, ok := err.(*ToolError); ok {
		return UnifiedToolResult{
			InvocationID: invocationID,
			Status:       ToolResultStatusFailed,
			Error:        toolErr,
		}
	}
	return UnifiedToolResult{
		InvocationID: invocationID,
		Status:       ToolResultStatusFailed,
		Error: &ToolError{
			Code:        ErrorCodeExecutionFailed,
			Message:     err.Error(),
			UserVisible: true,
		},
	}
}

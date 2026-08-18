package capability

import (
	"context"
	"encoding/json"
)

type DispatchFunc func(ctx context.Context, handlerName string, input json.RawMessage, invocation ToolInvocationContext) (json.RawMessage, error)

type HandlerVerifier func(handlerName string) bool

type BuiltinRuntimeAdapter struct {
	dispatcher      DispatchFunc
	handlerVerifier HandlerVerifier
}

func NewBuiltinRuntimeAdapter(dispatcher DispatchFunc) *BuiltinRuntimeAdapter {
	return &BuiltinRuntimeAdapter{dispatcher: dispatcher}
}

func NewBuiltinRuntimeAdapterWithVerifier(dispatcher DispatchFunc, verifier HandlerVerifier) *BuiltinRuntimeAdapter {
	return &BuiltinRuntimeAdapter{dispatcher: dispatcher, handlerVerifier: verifier}
}

func (a *BuiltinRuntimeAdapter) SetHandlerVerifier(v HandlerVerifier) {
	a.handlerVerifier = v
}

func (a *BuiltinRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeBuiltin
}

func (a *BuiltinRuntimeAdapter) Execute(
	ctx context.Context,
	binding RuntimeBinding,
	invocation ToolInvocationContext,
	input json.RawMessage,
) UnifiedToolResult {
	if a.dispatcher == nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeInternalError,
				Message:     "builtin dispatcher not configured",
				UserVisible: false,
			},
		}
	}

	output, err := a.dispatcher(ctx, binding.HandlerName, input, invocation)
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

func (a *BuiltinRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if binding.HandlerName == "" {
		return HealthUnhealthy
	}
	if a.dispatcher == nil {
		return HealthUnhealthy
	}
	if a.handlerVerifier != nil {
		if !a.handlerVerifier(binding.HandlerName) {
			return HealthUnhealthy
		}
	}
	return HealthReady
}

package capability

import (
	"context"
	"encoding/json"
)

type DispatchFunc func(ctx context.Context, handlerName string, input json.RawMessage) (json.RawMessage, error)

type BuiltinRuntimeAdapter struct {
	dispatcher DispatchFunc
}

func NewBuiltinRuntimeAdapter(dispatcher DispatchFunc) *BuiltinRuntimeAdapter {
	return &BuiltinRuntimeAdapter{dispatcher: dispatcher}
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

	output, err := a.dispatcher(ctx, binding.HandlerName, input)
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
	return HealthReady
}

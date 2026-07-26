package capability

import (
	"context"
	"encoding/json"
)

type InternalCallFunc func(ctx context.Context, handlerName string, input json.RawMessage) (json.RawMessage, error)

type InternalRuntimeAdapter struct {
	dispatcher InternalCallFunc
}

func NewInternalRuntimeAdapter(dispatcher InternalCallFunc) *InternalRuntimeAdapter {
	return &InternalRuntimeAdapter{dispatcher: dispatcher}
}

func (a *InternalRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeInternal
}

func (a *InternalRuntimeAdapter) Execute(
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
				Message:     "internal dispatcher not configured",
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

func (a *InternalRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	return HealthReady
}

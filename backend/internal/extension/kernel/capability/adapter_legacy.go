package capability

import (
	"context"
	"encoding/json"
)

type LegacyDispatchFunc func(ctx context.Context, handlerName string, input json.RawMessage) (json.RawMessage, error)

type LegacyRuntimeAdapter struct {
	dispatcher LegacyDispatchFunc
}

func NewLegacyRuntimeAdapter(dispatcher LegacyDispatchFunc) *LegacyRuntimeAdapter {
	return &LegacyRuntimeAdapter{dispatcher: dispatcher}
}

func (a *LegacyRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeLegacy
}

func (a *LegacyRuntimeAdapter) Execute(
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
				Message:     "legacy dispatcher not configured",
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

func (a *LegacyRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	return HealthReady
}

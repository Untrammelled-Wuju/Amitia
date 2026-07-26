package capability

import (
	"context"
	"encoding/json"
)

type PluginCallFunc func(ctx context.Context, pluginID string, handlerName string, input json.RawMessage) (json.RawMessage, error)
type PluginHealthFunc func(ctx context.Context, pluginID string) HealthStatus

type PluginRuntimeAdapter struct {
	caller PluginCallFunc
	health PluginHealthFunc
}

func NewPluginRuntimeAdapter(caller PluginCallFunc, health PluginHealthFunc) *PluginRuntimeAdapter {
	return &PluginRuntimeAdapter{caller: caller, health: health}
}

func (a *PluginRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypePluginJS || binding.RuntimeType == RuntimeTypePluginService
}

func (a *PluginRuntimeAdapter) Execute(
	ctx context.Context,
	binding RuntimeBinding,
	invocation ToolInvocationContext,
	input json.RawMessage,
) UnifiedToolResult {
	if a.caller == nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeRuntimeUnavailable,
				Message:     "plugin caller not configured",
				UserVisible: false,
			},
		}
	}

	output, err := a.caller(ctx, binding.RuntimeID, binding.HandlerName, input)
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

func (a *PluginRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health != nil {
		return a.health(ctx, binding.RuntimeID)
	}
	return HealthUnknown
}

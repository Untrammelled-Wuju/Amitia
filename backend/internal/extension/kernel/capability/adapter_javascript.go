package capability

import (
	"context"
	"encoding/json"
	"fmt"
)

type JavaScriptCallFunc func(ctx context.Context, extensionID string, moduleID string, handlerName string, input json.RawMessage) (json.RawMessage, error)
type JavaScriptHealthFunc func(ctx context.Context, extensionID string, moduleID string) HealthStatus

type JavaScriptRuntimeAdapter struct {
	caller JavaScriptCallFunc
	health JavaScriptHealthFunc
}

func NewJavaScriptRuntimeAdapter(caller JavaScriptCallFunc, health JavaScriptHealthFunc) *JavaScriptRuntimeAdapter {
	return &JavaScriptRuntimeAdapter{caller: caller, health: health}
}

func (a *JavaScriptRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeJavaScript || binding.RuntimeType == RuntimeTypePluginJS
}

func (a *JavaScriptRuntimeAdapter) Execute(
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
				Message:     "javascript caller not configured",
				UserVisible: false,
			},
		}
	}

	extID := invocation.ExtensionID
	modID := invocation.ModuleID
	if extID == "" && binding.Metadata != nil {
		if v, ok := binding.Metadata["extensionId"].(string); ok {
			extID = v
		}
	}
	if modID == "" && binding.Metadata != nil {
		if v, ok := binding.Metadata["moduleId"].(string); ok {
			modID = v
		}
	}

	output, err := a.caller(ctx, extID, modID, binding.HandlerName, input)
	if err != nil {
		code := ErrorCodeExecutionFailed
		switch {
		case contains(err.Error(), "timeout"):
			code = ErrorCodeTimeout
		case contains(err.Error(), "cancelled"):
			code = ErrorCodeCancelled
		case contains(err.Error(), "not found"):
			code = ErrorCodeRuntimeUnavailable
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        code,
				Message:     fmt.Sprintf("javascript runtime: %s", err.Error()),
				UserVisible: false,
				Retryable:   code == ErrorCodeTimeout,
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

func (a *JavaScriptRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health != nil {
		extID := ""
		modID := ""
		if binding.Metadata != nil {
			if v, ok := binding.Metadata["extensionId"].(string); ok {
				extID = v
			}
			if v, ok := binding.Metadata["moduleId"].(string); ok {
				modID = v
			}
		}
		return a.health(ctx, extID, modID)
	}
	return HealthUnknown
}

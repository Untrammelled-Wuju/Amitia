package capability

import (
	"context"
	"encoding/json"
	"fmt"
)

type WASMCallFunc func(ctx context.Context, moduleHash string, exportName string, input json.RawMessage) (json.RawMessage, error)
type WASMHealthFunc func(ctx context.Context, moduleHash string) HealthStatus

type WASMRuntimeAdapter struct {
	caller WASMCallFunc
	health WASMHealthFunc
}

func NewWASMRuntimeAdapter(caller WASMCallFunc, health WASMHealthFunc) *WASMRuntimeAdapter {
	return &WASMRuntimeAdapter{caller: caller, health: health}
}

func (a *WASMRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeWASM
}

func (a *WASMRuntimeAdapter) Execute(
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
				Message:     "wasm caller not configured",
				UserVisible: false,
			},
		}
	}

	moduleHash := binding.RuntimeID
	exportName := binding.HandlerName
	if moduleHash == "" && binding.Metadata != nil {
		if v, ok := binding.Metadata["moduleHash"].(string); ok {
			moduleHash = v
		}
	}

	output, err := a.caller(ctx, moduleHash, exportName, input)
	if err != nil {
		code := ErrorCodeExecutionFailed
		switch {
		case contains(err.Error(), "timeout"):
			code = ErrorCodeTimeout
		case contains(err.Error(), "abi"):
			code = ErrorCodeInternalError
		case contains(err.Error(), "not found"):
			code = ErrorCodeRuntimeUnavailable
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        code,
				Message:     fmt.Sprintf("wasm runtime: %s", err.Error()),
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

func (a *WASMRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health != nil {
		return a.health(ctx, binding.RuntimeID)
	}
	return HealthUnknown
}

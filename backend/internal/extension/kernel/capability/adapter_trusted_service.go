package capability

import (
	"context"
	"encoding/json"
	"fmt"
)

type TrustedServiceCallFunc func(ctx context.Context, serviceID string, handlerName string, input json.RawMessage) (json.RawMessage, error)
type TrustedServiceHealthFunc func(ctx context.Context, serviceID string) HealthStatus

type TrustedServiceRuntimeAdapter struct {
	caller TrustedServiceCallFunc
	health TrustedServiceHealthFunc
}

func NewTrustedServiceRuntimeAdapter(caller TrustedServiceCallFunc, health TrustedServiceHealthFunc) *TrustedServiceRuntimeAdapter {
	return &TrustedServiceRuntimeAdapter{caller: caller, health: health}
}

func (a *TrustedServiceRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeTrustedService || binding.RuntimeType == RuntimeTypePluginService
}

func (a *TrustedServiceRuntimeAdapter) Execute(
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
				Message:     "trusted service caller not configured",
				UserVisible: false,
			},
		}
	}

	serviceID := binding.RuntimeID
	handlerName := binding.HandlerName

	output, err := a.caller(ctx, serviceID, handlerName, input)
	if err != nil {
		code := ErrorCodeExecutionFailed
		switch {
		case contains(err.Error(), "timeout"):
			code = ErrorCodeTimeout
		case contains(err.Error(), "platform"):
			code = ErrorCodeNotAvailable
		case contains(err.Error(), "trust"):
			code = ErrorCodePermissionDenied
		case contains(err.Error(), "not found"):
			code = ErrorCodeRuntimeUnavailable
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        code,
				Message:     fmt.Sprintf("trusted service: %s", err.Error()),
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

func (a *TrustedServiceRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health != nil {
		return a.health(ctx, binding.RuntimeID)
	}
	return HealthUnknown
}

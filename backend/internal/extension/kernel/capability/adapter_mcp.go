package capability

import (
	"context"
	"encoding/json"
)

type MCPCallFunc func(ctx context.Context, serverID string, toolName string, input json.RawMessage) (json.RawMessage, error)
type MCPHealthFunc func(ctx context.Context, serverID string) HealthStatus

type MCPRuntimeAdapter struct {
	caller MCPCallFunc
	health MCPHealthFunc
}

func NewMCPRuntimeAdapter(caller MCPCallFunc, health MCPHealthFunc) *MCPRuntimeAdapter {
	return &MCPRuntimeAdapter{caller: caller, health: health}
}

func (a *MCPRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeMCP
}

func (a *MCPRuntimeAdapter) Execute(
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
				Message:     "MCP caller not configured",
				UserVisible: false,
			},
		}
	}

	output, err := a.caller(ctx, binding.RuntimeID, binding.HandlerName, input)
	if err != nil {
		code := ErrorCodeExecutionFailed
		userVisible := false
		switch {
		case contains(err.Error(), "connection"):
			code = ErrorCodeConnectionLost
		case contains(err.Error(), "timeout"):
			code = ErrorCodeTimeout
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        code,
				Message:     err.Error(),
				UserVisible: userVisible,
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

func (a *MCPRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.health != nil {
		return a.health(ctx, binding.RuntimeID)
	}
	return HealthUnknown
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

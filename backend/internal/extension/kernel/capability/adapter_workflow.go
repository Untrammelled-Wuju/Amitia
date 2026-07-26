package capability

import (
	"context"
	"encoding/json"
)

type WorkflowCallFunc func(ctx context.Context, workflowID string, input json.RawMessage) (json.RawMessage, error)

type WorkflowRuntimeAdapter struct {
	caller WorkflowCallFunc
}

func NewWorkflowRuntimeAdapter(caller WorkflowCallFunc) *WorkflowRuntimeAdapter {
	return &WorkflowRuntimeAdapter{caller: caller}
}

func (a *WorkflowRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeWorkflow
}

func (a *WorkflowRuntimeAdapter) Execute(
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
				Message:     "workflow caller not configured",
				UserVisible: false,
			},
		}
	}

	output, err := a.caller(ctx, binding.RuntimeID, input)
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

func (a *WorkflowRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	return HealthReady
}

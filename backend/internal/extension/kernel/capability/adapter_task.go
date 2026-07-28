package capability

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type TaskEnqueueFunc func(ctx context.Context, taskDefinitionID string, input json.RawMessage) (taskRunID string, err error)
type TaskStatusFunc func(ctx context.Context, taskRunID string) (TaskRunStatus, error)

type TaskRunStatus struct {
	State    string
	Output   json.RawMessage
	Error    string
	Finished bool
}

type TaskRuntimeAdapter struct {
	enqueue TaskEnqueueFunc
	status  TaskStatusFunc
}

func NewTaskRuntimeAdapter(enqueue TaskEnqueueFunc, status TaskStatusFunc) *TaskRuntimeAdapter {
	return &TaskRuntimeAdapter{enqueue: enqueue, status: status}
}

func (a *TaskRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType == RuntimeTypeTask
}

func (a *TaskRuntimeAdapter) Execute(
	ctx context.Context,
	binding RuntimeBinding,
	invocation ToolInvocationContext,
	input json.RawMessage,
) UnifiedToolResult {
	if a.enqueue == nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeRuntimeUnavailable,
				Message:     "task enqueue function not configured",
				UserVisible: false,
			},
		}
	}

	taskDefID := binding.RuntimeID
	if taskDefID == "" {
		taskDefID = binding.HandlerName
	}

	taskRunID, err := a.enqueue(ctx, taskDefID, input)
	if err != nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:        ErrorCodeExecutionFailed,
				Message:     fmt.Sprintf("task enqueue: %s", err.Error()),
				UserVisible: false,
			},
		}
	}

	acceptedOutput, _ := json.Marshal(map[string]any{
		"accepted":   true,
		"taskRunId":  taskRunID,
		"taskDefId":  taskDefID,
		"invocationId": invocation.InvocationID,
	})

	return UnifiedToolResult{
		InvocationID: invocation.InvocationID,
		Status:       ToolResultStatusSuccess,
		Content: []ToolContent{
			{Type: ToolContentText, Text: string(acceptedOutput)},
		},
		Structured: acceptedOutput,
		Metadata: map[string]any{
			"taskRunId": taskRunID,
			"async":     true,
		},
	}
}

func (a *TaskRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.enqueue == nil {
		return HealthUnhealthy
	}
	return HealthReady
}

func (a *TaskRuntimeAdapter) WaitForCompletion(ctx context.Context, taskRunID string, timeout time.Duration) (TaskRunStatus, error) {
	if a.status == nil {
		return TaskRunStatus{}, fmt.Errorf("task status function not configured")
	}

	deadline := time.Now().Add(timeout)
	pollInterval := 500 * time.Millisecond

	for time.Now().Before(deadline) {
		status, err := a.status(ctx, taskRunID)
		if err != nil {
			return TaskRunStatus{}, err
		}
		if status.Finished {
			return status, nil
		}
		select {
		case <-ctx.Done():
			return TaskRunStatus{}, ctx.Err()
		case <-time.After(pollInterval):
		}
	}

	return TaskRunStatus{State: "timeout"}, fmt.Errorf("task %s did not complete within %s", taskRunID, timeout)
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/agent"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type taskRuntimeAdapter struct {
	svc *task_runtime.TaskRuntimeService
}

func NewTaskRuntimeExecutor(svc *task_runtime.TaskRuntimeService) agent.TaskRuntimeExecutor {
	return &taskRuntimeAdapter{svc: svc}
}

func (a *taskRuntimeAdapter) Execute(ctx context.Context, taskType string, input map[string]interface{}) (json.RawMessage, error) {
	if a.svc == nil {
		return nil, fmt.Errorf("task runtime service not configured")
	}

	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode task input: %w", err)
	}

	def, err := a.svc.GetTaskDefinition(ctx, taskType)
	if err != nil {
		return nil, fmt.Errorf("task definition not found: %s: %w", taskType, err)
	}

	// Device workers are the execution plane. Never recursively re-dispatch a
	// device-targeted definition back through the remote executor.
	def.ExecutionPlacement = task_runtime.TaskExecutionPlacementLocal

	result, err := a.svc.Enqueue(ctx, task_runtime.EnqueueTaskRequest{
		TaskDefinitionID:   def.TaskID,
		ExtensionID:        def.ExtensionID,
		ModuleID:           def.ModuleID,
		Input:              inputBytes,
		ExecutionPlacement: def.ExecutionPlacement,
		InvocationID:       "mesh-" + uuid.NewString(),
	}, def)
	if err != nil {
		return nil, fmt.Errorf("enqueue task: %w", err)
	}

	return a.waitForResult(ctx, result.TaskRunID)
}

func (a *taskRuntimeAdapter) waitForResult(ctx context.Context, taskRunID string) (json.RawMessage, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("task execution timeout: %s", taskRunID)
		case <-ticker.C:
			runResult, err := a.svc.GetResult(ctx, taskRunID)
			if err == nil && runResult != nil && runResult.ResultJSON != nil {
				return runResult.ResultJSON, nil
			}

			run, runErr := a.svc.GetTaskRun(ctx, taskRunID)
			if runErr != nil || run == nil || !run.Status.IsTerminal() {
				continue
			}
			if run.Status == task_runtime.RunStatusSucceeded {
				return json.RawMessage(`null`), nil
			}
			message := string(run.Status)
			if run.ErrorMessage != nil && *run.ErrorMessage != "" {
				message = *run.ErrorMessage
			}
			return nil, fmt.Errorf("task %s finished with status %s: %s", taskRunID, run.Status, message)
		}
	}
}

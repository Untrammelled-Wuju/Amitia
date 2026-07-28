package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type HostAPIToolExecutorAdapter struct {
	Gateway host_api.Gateway
}

func NewHostAPIToolExecutorAdapter(gateway host_api.Gateway) *HostAPIToolExecutorAdapter {
	return &HostAPIToolExecutorAdapter{Gateway: gateway}
}

func (a *HostAPIToolExecutorAdapter) ExecuteTool(ctx context.Context, toolID string, input []byte, operationID string) (*schedule.ToolExecutionResult, error) {
	if a == nil || a.Gateway == nil {
		return &schedule.ToolExecutionResult{
			ErrorCode:    schedule.ErrCodeTargetNotFound,
			ErrorMessage: "tool executor not configured",
		}, nil
	}

	inputPayload := input
	if len(inputPayload) == 0 {
		inputPayload = []byte(`{}`)
	}
	requestBody, err := json.Marshal(map[string]any{
		"toolId": toolID,
		"input":  json.RawMessage(inputPayload),
	})
	if err != nil {
		return &schedule.ToolExecutionResult{
			ErrorCode:    schedule.ErrCodeTargetExecutionFailed,
			ErrorMessage: fmt.Sprintf("marshal tool request: %v", err),
		}, nil
	}

	callID := fmt.Sprintf("sched-tool-%s", operationID)
	if callID == "sched-tool-" {
		callID = fmt.Sprintf("sched-tool-%s", uuid.NewString())
	}

	result := a.Gateway.Call(ctx, host_api.CallRequest{
		CallID:       callID,
		Method:       host_api.MethodToolExecute,
		Version:      1,
		Input:        requestBody,
		InvocationID: operationID,
	})

	if result.Error != nil {
		return &schedule.ToolExecutionResult{
			ErrorCode:    result.Error.Code,
			ErrorMessage: result.Error.Message,
		}, nil
	}

	return &schedule.ToolExecutionResult{
		ResultJSON: result.Output,
	}, nil
}

type KernelWorkflowFacadeAdapter struct{}

func NewKernelWorkflowFacadeAdapter() *KernelWorkflowFacadeAdapter {
	return &KernelWorkflowFacadeAdapter{}
}

func (a *KernelWorkflowFacadeAdapter) ExecuteWorkflow(ctx context.Context, workflowID string, input []byte, operationID string) (*schedule.WorkflowExecutionResult, error) {
	return &schedule.WorkflowExecutionResult{
		ErrorCode:    "WORKFLOW_NOT_IMPLEMENTED",
		ErrorMessage: "workflow executor not yet migrated to kernel facade",
	}, nil
}

type TaskRuntimeEnqueueAdapter struct {
	Service *task_runtime.TaskRuntimeService
}

func NewTaskRuntimeEnqueueAdapter(service *task_runtime.TaskRuntimeService) *TaskRuntimeEnqueueAdapter {
	return &TaskRuntimeEnqueueAdapter{Service: service}
}

func (a *TaskRuntimeEnqueueAdapter) Enqueue(ctx context.Context, def *schedule.ScheduleContributionDefinition, trigger *schedule.ScheduleTriggerRecord) (*schedule.TaskEnqueueResult, error) {
	if a == nil || a.Service == nil {
		return &schedule.TaskEnqueueResult{
			ErrorCode:    schedule.ErrCodeTargetNotFound,
			ErrorMessage: "task runtime service not configured",
		}, nil
	}

	input := def.Target.InputTemplate
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}

	operationID := ""
	if trigger.OperationID != nil {
		operationID = *trigger.OperationID
	}
	if operationID == "" {
		operationID = fmt.Sprintf("sched-%s-%s", trigger.ScheduleID, trigger.TriggerID)
	}

	taskDef := &task_runtime.TaskDefinition{
		TaskID:        def.Target.TargetID,
		ExtensionID:   def.ExtensionID,
		ModuleID:      def.ModuleID,
		ContributionID: def.ContributionID,
		RuntimeType:   "javascript",
		Entry:         def.Target.TargetID,
	}

	enqueueReq := task_runtime.EnqueueTaskRequest{
		TaskDefinitionID:    def.Target.TargetID,
		ExtensionID:         def.ExtensionID,
		ModuleID:            def.ModuleID,
		Input:               input,
		Priority:            0,
		OperationID:         operationID,
		ScopeSnapshotID:     trigger.ScopeSnapshotID,
		PermissionSnapshotID: trigger.PermissionSnapshotID,
	}

	result, err := a.Service.Enqueue(ctx, enqueueReq, taskDef)
	if err != nil {
		return &schedule.TaskEnqueueResult{
			ErrorCode:    schedule.ErrCodeTargetExecutionFailed,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &schedule.TaskEnqueueResult{
		TaskRunID:   result.TaskRunID,
		OperationID: operationID,
	}, nil
}

type SupervisorRuntimeHandlerAdapter struct {
	Supervisor runtime_supervisor.Supervisor
}

func NewSupervisorRuntimeHandlerAdapter(supervisor runtime_supervisor.Supervisor) *SupervisorRuntimeHandlerAdapter {
	return &SupervisorRuntimeHandlerAdapter{Supervisor: supervisor}
}

func (a *SupervisorRuntimeHandlerAdapter) Invoke(ctx context.Context, handlerID string, input []byte, scheduleCtx schedule.ScheduleHandlerContext) (*schedule.RuntimeHandlerResult, error) {
	if a == nil || a.Supervisor == nil {
		return &schedule.RuntimeHandlerResult{
			ErrorCode:    schedule.ErrCodeRuntimeHandlerMissing,
			ErrorMessage: "runtime supervisor not configured",
		}, nil
	}

	invocationID := scheduleCtx.InvocationID
	if invocationID == "" {
		invocationID = scheduleCtx.TriggerID
	}

	req := runtime_supervisor.InvocationRequest{
		InstanceID:   handlerID,
		TraceID:      scheduleCtx.TriggerID,
		InvocationID: invocationID,
		Operation:    handlerID,
		Input:        input,
		Generation:   0,
	}

	if !scheduleCtx.ScheduledAt.IsZero() {
		req.Deadline = scheduleCtx.ScheduledAt.Add(30 * time.Minute)
	}

	result := a.Supervisor.Invoke(ctx, req)

	if result.Error != nil {
		return &schedule.RuntimeHandlerResult{
			ErrorCode:    schedule.ErrCodeTargetExecutionFailed,
			ErrorMessage: result.Error.Error(),
		}, nil
	}

	if result.Status != "" && result.Status != "success" && result.Status != "completed" {
		return &schedule.RuntimeHandlerResult{
			ErrorCode:    schedule.ErrCodeTargetExecutionFailed,
			ErrorMessage: fmt.Sprintf("runtime handler returned status: %s", result.Status),
		}, nil
	}

	return &schedule.RuntimeHandlerResult{
		ResultJSON: result.Output,
	}, nil
}

func BuildScheduleTaskEnqueueFunc(service *task_runtime.TaskRuntimeService) schedule.TaskEnqueueFunc {
	adapter := NewTaskRuntimeEnqueueAdapter(service)
	return adapter.Enqueue
}

func BuildScheduleRuntimeHandlerFn(supervisor runtime_supervisor.Supervisor) schedule.RuntimeHandlerInvokeFunc {
	adapter := NewSupervisorRuntimeHandlerAdapter(supervisor)
	return adapter.Invoke
}

var _ schedule.ToolExecutor = (*HostAPIToolExecutorAdapter)(nil)
var _ schedule.WorkflowExecutor = (*KernelWorkflowFacadeAdapter)(nil)

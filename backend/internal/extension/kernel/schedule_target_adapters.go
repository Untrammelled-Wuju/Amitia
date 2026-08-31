package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type HostAPIToolExecutorAdapter struct {
	Gateway host_api.Gateway
}

func NewHostAPIToolExecutorAdapter(gateway host_api.Gateway) *HostAPIToolExecutorAdapter {
	return &HostAPIToolExecutorAdapter{Gateway: gateway}
}

func (a *HostAPIToolExecutorAdapter) ExecuteTool(ctx context.Context, toolID string, input []byte, operationID string, scheduleCtx schedule.ScheduleToolContext) (*schedule.ToolExecutionResult, error) {
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

	invocationID := operationID
	if invocationID == "" {
		invocationID = scheduleCtx.TraceID
	}
	if invocationID == "" {
		invocationID = fmt.Sprintf("sched-tool-%s", uuid.NewString())
	}

	result := a.Gateway.Call(ctx, host_api.CallRequest{
		CallID: callID,
		RuntimeIdentity: runtime_supervisor.RuntimeIdentity{
			ExtensionID: domain.ExtensionID(scheduleCtx.ExtensionID),
			ModuleID:    domain.ModuleID(scheduleCtx.ModuleID),
			Generation:  scheduleCtx.Generation,
		},
		Method:          host_api.MethodToolExecute,
		Version:         1,
		Input:           requestBody,
		ScopeSnapshotID: scheduleCtx.ScopeSnapshotID,
		TraceID:         scheduleCtx.TraceID,
		InvocationID:    invocationID,
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

type KernelToolExecutorAdapter struct {
	Kernel *execution.ExecutionPipeline
}

func NewKernelToolExecutorAdapter(kernel *execution.ExecutionPipeline) *KernelToolExecutorAdapter {
	return &KernelToolExecutorAdapter{Kernel: kernel}
}

func (a *KernelToolExecutorAdapter) ExecuteTool(ctx context.Context, toolID string, input []byte, operationID string, scheduleCtx schedule.ScheduleToolContext) (*schedule.ToolExecutionResult, error) {
	if a == nil || a.Kernel == nil {
		return &schedule.ToolExecutionResult{
			ErrorCode:    schedule.ErrCodeTargetNotFound,
			ErrorMessage: "execution kernel not configured",
		}, nil
	}

	inputPayload := input
	if len(inputPayload) == 0 {
		inputPayload = []byte(`{}`)
	}

	invocationID := operationID
	if invocationID == "" {
		invocationID = scheduleCtx.TraceID
	}
	if invocationID == "" {
		invocationID = fmt.Sprintf("sched-tool-%s", uuid.NewString())
	}

	invocation := capability.ToolInvocationContext{
		InvocationID:         invocationID,
		Source:               capability.InvocationSourceScheduledTask,
		TraceID:              scheduleCtx.TraceID,
		ExtensionID:          scheduleCtx.ExtensionID,
		ModuleID:             scheduleCtx.ModuleID,
		Generation:           scheduleCtx.Generation,
		IdempotencyKey:       scheduleCtx.IdempotencyKey,
		ScheduleID:           scheduleCtx.ScheduleID,
		TriggerID:            scheduleCtx.TriggerID,
		OperationID:          operationID,
		ScopeSnapshotID:      scheduleCtx.ScopeSnapshotID,
		PermissionSnapshotID: scheduleCtx.PermissionSnapshotID,
	}

	req := execution.ToolExecutionRequest{
		ToolID:     capability.CapabilityID(toolID),
		Input:      json.RawMessage(inputPayload),
		Invocation: invocation,
	}

	result := a.Kernel.Execute(ctx, req)

	if result.Status != capability.ToolResultStatusSuccess {
		errCode := schedule.ErrCodeTargetExecutionFailed
		errMsg := "tool execution failed"
		if result.Error != nil {
			if result.Error.Code != "" {
				errCode = result.Error.Code
			}
			if result.Error.Message != "" {
				errMsg = result.Error.Message
			}
		}
		return &schedule.ToolExecutionResult{
			ErrorCode:    errCode,
			ErrorMessage: errMsg,
		}, nil
	}

	var outputBytes []byte
	if len(result.Structured) > 0 {
		outputBytes = result.Structured
	} else if len(result.Content) > 0 {
		for _, c := range result.Content {
			if c.Type == capability.ToolContentText && c.Text != "" {
				outputBytes = []byte(c.Text)
				break
			}
		}
	}
	if len(outputBytes) == 0 {
		outputBytes = []byte(`{}`)
	}

	return &schedule.ToolExecutionResult{
		ResultJSON: outputBytes,
	}, nil
}

type KernelWorkflowFacadeAdapter struct {
	executor           *workflow.WorkflowExecutor
	generationResolver func(context.Context, string) (int64, error)
}

func NewKernelWorkflowFacadeAdapter(executor *workflow.WorkflowExecutor, generationResolver ...func(context.Context, string) (int64, error)) *KernelWorkflowFacadeAdapter {
	adapter := &KernelWorkflowFacadeAdapter{executor: executor}
	if len(generationResolver) > 0 {
		adapter.generationResolver = generationResolver[0]
	}
	return adapter
}

func (a *KernelWorkflowFacadeAdapter) ExecuteWorkflow(ctx context.Context, workflowID string, input []byte, scheduleContext schedule.WorkflowScheduleContext) (*schedule.WorkflowExecutionResult, error) {
	if a == nil || a.executor == nil {
		return &schedule.WorkflowExecutionResult{
			ErrorCode:    "WORKFLOW_NOT_CONFIGURED",
			ErrorMessage: "workflow executor not configured",
		}, nil
	}

	inputPayload := input
	if len(inputPayload) == 0 {
		inputPayload = []byte(`{}`)
	}

	invocationID := scheduleContext.InvocationID
	if invocationID == "" {
		invocationID = fmt.Sprintf("sched-wf-%s", uuid.NewString())
	}
	generation := scheduleContext.Generation
	if scheduleContext.ExtensionID == "" {
		generation = 0
	}
	if a.generationResolver != nil && scheduleContext.ExtensionID != "" {
		resolvedGeneration, resolveErr := a.generationResolver(ctx, scheduleContext.ExtensionID)
		if resolveErr != nil {
			return &schedule.WorkflowExecutionResult{OperationID: scheduleContext.OperationID, InvocationID: invocationID, ErrorCode: schedule.ErrCodeGenerationMismatch, ErrorMessage: resolveErr.Error()}, nil
		}
		generation = resolvedGeneration
	}

	req := workflow.ExecuteRequest{
		WorkflowID: workflowID,
		Input:      json.RawMessage(inputPayload),
		Context: workflow.ExecutionContext{
			ExtensionID:      scheduleContext.ExtensionID,
			ModuleID:         scheduleContext.ModuleID,
			OperationID:      scheduleContext.OperationID,
			InvocationID:     invocationID,
			ScopeSnapshotID:  scheduleContext.ScopeSnapshotID,
			PermissionSnapID: scheduleContext.PermissionSnapshotID,
			Generation:       generation,
			ScheduleID:       scheduleContext.ScheduleID,
			TriggerID:        scheduleContext.TriggerID,
			TraceID:          scheduleContext.TraceID,
			IdempotencyKey:   scheduleContext.IdempotencyKey,
		},
	}

	result, err := a.executor.Execute(ctx, req)
	if err != nil {
		return &schedule.WorkflowExecutionResult{
			OperationID:  scheduleContext.OperationID,
			InvocationID: invocationID,
			ErrorCode:    schedule.ErrCodeTargetExecutionFailed,
			ErrorMessage: err.Error(),
		}, nil
	}

	if !result.Success {
		return &schedule.WorkflowExecutionResult{
			OperationID:  scheduleContext.OperationID,
			InvocationID: invocationID,
			ErrorCode:    schedule.ErrCodeTargetExecutionFailed,
			ErrorMessage: result.Error,
		}, nil
	}

	outputBytes := result.Output
	if len(outputBytes) == 0 {
		outputBytes = []byte(`{}`)
	}

	return &schedule.WorkflowExecutionResult{
		OperationID:  scheduleContext.OperationID,
		InvocationID: invocationID,
		Status:       string(result.Status),
		Accepted:     result.Accepted,
		ResultJSON:   outputBytes,
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
		TaskID:         def.Target.TargetID,
		ExtensionID:    def.ExtensionID,
		ModuleID:       def.ModuleID,
		ContributionID: def.ContributionID,
		RuntimeType:    "javascript",
		Entry:          def.Target.TargetID,
	}

	enqueueReq := task_runtime.EnqueueTaskRequest{
		TaskDefinitionID:     def.Target.TargetID,
		ExtensionID:          def.ExtensionID,
		ModuleID:             def.ModuleID,
		Input:                input,
		Priority:             0,
		OperationID:          operationID,
		ScopeSnapshotID:      trigger.ScopeSnapshotID,
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
var _ schedule.ToolExecutor = (*KernelToolExecutorAdapter)(nil)
var _ schedule.WorkflowExecutor = (*KernelWorkflowFacadeAdapter)(nil)

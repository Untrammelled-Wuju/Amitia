package schedule

import (
	"context"
	"fmt"
	"time"
)

type TargetAdapter interface {
	Execute(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord) (*TargetExecutionResult, error)
	Type() TargetType
}

type TargetExecutionResult struct {
	Success      bool
	OperationID  string
	InvocationID string
	TaskRunID    string
	ResultJSON   []byte
	ErrorCode    string
	ErrorMessage string
}

type ToolTargetAdapter struct {
	executor ToolExecutor
}

type ScheduleToolContext struct {
	ScheduleID           string
	TriggerID            string
	ExtensionID          string
	ModuleID             string
	Generation           int64
	ScopeSnapshotID      string
	PermissionSnapshotID string
	IdempotencyKey       string
	TraceID              string
}

type ToolExecutor interface {
	ExecuteTool(ctx context.Context, toolID string, input []byte, operationID string, scheduleCtx ScheduleToolContext) (*ToolExecutionResult, error)
}

type ToolExecutionResult struct {
	ResultJSON   []byte
	ErrorCode    string
	ErrorMessage string
}

func NewToolTargetAdapter(executor ToolExecutor) *ToolTargetAdapter {
	return &ToolTargetAdapter{executor: executor}
}

func (a *ToolTargetAdapter) Type() TargetType { return TargetTypeTool }

func (a *ToolTargetAdapter) Execute(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord) (*TargetExecutionResult, error) {
	if a.executor == nil {
		return &TargetExecutionResult{
			Success:      false,
			ErrorCode:    ErrCodeTargetNotFound,
			ErrorMessage: "tool executor not configured",
		}, nil
	}
	opID := ""
	if trigger.OperationID != nil {
		opID = *trigger.OperationID
	}
	invocationID := ""
	if trigger.InvocationID != nil {
		invocationID = *trigger.InvocationID
	}
	scheduleCtx := ScheduleToolContext{
		ScheduleID:           def.ScheduleID,
		TriggerID:            trigger.TriggerID,
		ExtensionID:          def.ExtensionID,
		ModuleID:             def.ModuleID,
		Generation:           trigger.Generation,
		ScopeSnapshotID:      trigger.ScopeSnapshotID,
		PermissionSnapshotID: trigger.PermissionSnapshotID,
		IdempotencyKey:       trigger.IdempotencyKey,
		TraceID:              invocationID,
	}
	result, err := a.executor.ExecuteTool(ctx, def.Target.TargetID, def.Target.InputTemplate, opID, scheduleCtx)
	if err != nil {
		return &TargetExecutionResult{
			Success:      false,
			ErrorCode:    ErrCodeTargetExecutionFailed,
			ErrorMessage: err.Error(),
		}, nil
	}
	return &TargetExecutionResult{
		Success:      result.ErrorCode == "",
		OperationID:  opID,
		ResultJSON:   result.ResultJSON,
		ErrorCode:    result.ErrorCode,
		ErrorMessage: result.ErrorMessage,
	}, nil
}

type WorkflowTargetAdapter struct {
	executor WorkflowExecutor
}

type WorkflowExecutor interface {
	ExecuteWorkflow(ctx context.Context, workflowID string, input []byte, scheduleContext WorkflowScheduleContext) (*WorkflowExecutionResult, error)
}

type WorkflowScheduleContext struct {
	ScheduleID           string
	TriggerID            string
	OperationID          string
	InvocationID         string
	ScopeSnapshotID      string
	PermissionSnapshotID string
	ExtensionID          string
	ModuleID             string
	Generation           int64
	TraceID              string
	IdempotencyKey       string
}

type WorkflowExecutionResult struct {
	OperationID  string
	InvocationID string
	Status       string
	Accepted     bool
	ResultJSON   []byte
	ErrorCode    string
	ErrorMessage string
}

func NewWorkflowTargetAdapter(executor WorkflowExecutor) *WorkflowTargetAdapter {
	return &WorkflowTargetAdapter{executor: executor}
}

func (a *WorkflowTargetAdapter) Type() TargetType { return TargetTypeWorkflow }

func (a *WorkflowTargetAdapter) Execute(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord) (*TargetExecutionResult, error) {
	if a.executor == nil {
		return &TargetExecutionResult{
			Success:      false,
			ErrorCode:    ErrCodeTargetNotFound,
			ErrorMessage: "workflow executor not configured",
		}, nil
	}
	opID := ""
	if trigger.OperationID != nil {
		opID = *trigger.OperationID
	}
	invocationID := fmt.Sprintf("sched-wf-%s", trigger.TriggerID)
	if trigger.InvocationID != nil && *trigger.InvocationID != "" {
		invocationID = *trigger.InvocationID
	}
	traceID := opID
	if traceID == "" {
		traceID = trigger.TriggerID
	}
	result, err := a.executor.ExecuteWorkflow(ctx, def.Target.TargetID, def.Target.InputTemplate, WorkflowScheduleContext{
		ScheduleID:           def.ScheduleID,
		TriggerID:            trigger.TriggerID,
		OperationID:          opID,
		InvocationID:         invocationID,
		ScopeSnapshotID:      trigger.ScopeSnapshotID,
		PermissionSnapshotID: trigger.PermissionSnapshotID,
		ExtensionID:          def.ExtensionID,
		ModuleID:             def.ModuleID,
		Generation:           trigger.Generation,
		TraceID:              traceID,
		IdempotencyKey:       trigger.IdempotencyKey,
	})
	if err != nil {
		return &TargetExecutionResult{
			Success:      false,
			ErrorCode:    ErrCodeTargetExecutionFailed,
			ErrorMessage: err.Error(),
		}, nil
	}
	return &TargetExecutionResult{
		Success:      result.ErrorCode == "",
		OperationID:  result.OperationID,
		InvocationID: result.InvocationID,
		ResultJSON:   result.ResultJSON,
		ErrorCode:    result.ErrorCode,
		ErrorMessage: result.ErrorMessage,
	}, nil
}

type TaskTargetAdapter struct {
	enqueueFn TaskEnqueueFunc
}

type TaskEnqueueFunc func(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord) (*TaskEnqueueResult, error)

type TaskEnqueueResult struct {
	TaskRunID    string
	OperationID  string
	ErrorCode    string
	ErrorMessage string
}

func NewTaskTargetAdapter(fn TaskEnqueueFunc) *TaskTargetAdapter {
	return &TaskTargetAdapter{enqueueFn: fn}
}

func (a *TaskTargetAdapter) Type() TargetType { return TargetTypeTask }

func (a *TaskTargetAdapter) Execute(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord) (*TargetExecutionResult, error) {
	if a.enqueueFn == nil {
		return &TargetExecutionResult{
			Success:      false,
			ErrorCode:    ErrCodeTargetNotFound,
			ErrorMessage: "task enqueue function not configured",
		}, nil
	}
	result, err := a.enqueueFn(ctx, def, trigger)
	if err != nil {
		return &TargetExecutionResult{
			Success:      false,
			ErrorCode:    ErrCodeTargetExecutionFailed,
			ErrorMessage: err.Error(),
		}, nil
	}
	return &TargetExecutionResult{
		Success:      result.ErrorCode == "",
		TaskRunID:    result.TaskRunID,
		OperationID:  result.OperationID,
		ErrorCode:    result.ErrorCode,
		ErrorMessage: result.ErrorMessage,
	}, nil
}

type RuntimeHandlerTargetAdapter struct {
	invokeFn RuntimeHandlerInvokeFunc
}

type RuntimeHandlerInvokeFunc func(ctx context.Context, handlerID string, input []byte, scheduleContext ScheduleHandlerContext) (*RuntimeHandlerResult, error)

type RuntimeHandlerResult struct {
	ResultJSON   []byte
	ErrorCode    string
	ErrorMessage string
}

type ScheduleHandlerContext struct {
	ScheduleID           string
	TriggerID            string
	ScheduledAt          time.Time
	EffectiveAt          time.Time
	Attempt              int
	OperationID          string
	InvocationID         string
	ScopeSnapshotID      string
	PermissionSnapshotID string
}

func NewRuntimeHandlerTargetAdapter(fn RuntimeHandlerInvokeFunc) *RuntimeHandlerTargetAdapter {
	return &RuntimeHandlerTargetAdapter{invokeFn: fn}
}

func (a *RuntimeHandlerTargetAdapter) Type() TargetType { return TargetTypeRuntimeHandler }

func (a *RuntimeHandlerTargetAdapter) Execute(ctx context.Context, def *ScheduleContributionDefinition, trigger *ScheduleTriggerRecord) (*TargetExecutionResult, error) {
	if a.invokeFn == nil {
		return &TargetExecutionResult{
			Success:      false,
			ErrorCode:    ErrCodeRuntimeHandlerMissing,
			ErrorMessage: "runtime handler invoke function not configured",
		}, nil
	}
	opID := ""
	invID := ""
	if trigger.OperationID != nil {
		opID = *trigger.OperationID
	}
	if trigger.InvocationID != nil {
		invID = *trigger.InvocationID
	}
	scheduleCtx := ScheduleHandlerContext{
		ScheduleID:           def.ScheduleID,
		TriggerID:            trigger.TriggerID,
		ScheduledAt:          trigger.ScheduledAt,
		EffectiveAt:          trigger.EffectiveAt,
		Attempt:              trigger.Attempt,
		OperationID:          opID,
		InvocationID:         invID,
		ScopeSnapshotID:      trigger.ScopeSnapshotID,
		PermissionSnapshotID: trigger.PermissionSnapshotID,
	}
	result, err := a.invokeFn(ctx, def.Target.TargetID, def.Target.InputTemplate, scheduleCtx)
	if err != nil {
		return &TargetExecutionResult{
			Success:      false,
			ErrorCode:    ErrCodeTargetExecutionFailed,
			ErrorMessage: err.Error(),
		}, nil
	}
	return &TargetExecutionResult{
		Success:      result.ErrorCode == "",
		OperationID:  opID,
		InvocationID: invID,
		ResultJSON:   result.ResultJSON,
		ErrorCode:    result.ErrorCode,
		ErrorMessage: result.ErrorMessage,
	}, nil
}

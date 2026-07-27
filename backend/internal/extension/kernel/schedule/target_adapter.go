package schedule

import (
	"context"
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

type ToolExecutor interface {
	ExecuteTool(ctx context.Context, toolID string, input []byte, operationID string) (*ToolExecutionResult, error)
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
	result, err := a.executor.ExecuteTool(ctx, def.Target.TargetID, def.Target.InputTemplate, opID)
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
	ExecuteWorkflow(ctx context.Context, workflowID string, input []byte, operationID string) (*WorkflowExecutionResult, error)
}

type WorkflowExecutionResult struct {
	OperationID  string
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
	result, err := a.executor.ExecuteWorkflow(ctx, def.Target.TargetID, def.Target.InputTemplate, opID)
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
	ScheduleID    string
	TriggerID     string
	ScheduledAt   time.Time
	EffectiveAt   time.Time
	Attempt       int
	OperationID   string
	InvocationID  string
	ScopeSnapshotID string
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

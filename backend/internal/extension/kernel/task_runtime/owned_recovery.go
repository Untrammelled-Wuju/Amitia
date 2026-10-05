package task_runtime

import (
	"context"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *TaskRuntimeService) HandleRemoteUnconfirmed(ctx context.Context, taskID, attemptID, leaseID string) error {
	if err := s.validateRemoteAttemptLease(ctx, taskID, attemptID, leaseID); err != nil {
		return err
	}
	run, err := s.store.GetTaskRun(ctx, taskID)
	if err != nil || run == nil || run.EffectiveExecutionPlacement() != TaskExecutionPlacementDevice || run.Status.IsTerminal() {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "未确认结果缺少有效的设备任务")
	}
	guarded, finish, owned, err := s.callbackAuthority(ctx, run, attemptID, run.Generation)
	if err != nil {
		return err
	}
	defer finish()
	if !owned {
		return NewTaskError(ErrTaskScopeDenied, "普通任务不能使用所有者结果未知回执")
	}
	if err := coordination.ValidateCurrent(guarded); err != nil {
		return err
	}
	return s.markTaskRecoveryUnknown(guarded, run)
}

func (s *TaskRuntimeService) markTaskRecoveryUnknown(ctx context.Context, run *TaskRun) error {
	if run == nil || run.Status.IsTerminal() {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务恢复状态无效")
	}
	next := CloneTaskRun(run)
	next.Status = RunStatusRecoveryRequired
	next.FinishedAt = nil
	next.ErrorCode = strPtr("task_execution_unconfirmed")
	message := "设备任务执行结果尚未确认，请核对数据所有者处的结果或确认检查点后再恢复"
	next.ErrorMessage = &message
	next.Revision = NextRevision(run.Revision)
	return s.mutateTaskRun(ctx, taskMutationParams{next: next, expected: run.Status, generation: run.Generation, revision: run.Revision, removeQ: true, eventType: TaskEventRecoveryRequired, eventMsg: message, eventCode: "task_execution_unconfirmed"})
}

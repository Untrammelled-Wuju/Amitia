package task_runtime

import (
	"context"
	"time"
)

func (s *TaskRuntimeService) finalizeCommittedOwnerOutcome(ctx context.Context, run *TaskRun, committed *CommittedTaskOutcome) (*TaskRun, error) {
	if run == nil || committed == nil || (run.Status != RunStatusRecoveryRequired && run.Status != RunStatusManualIntervention) {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "原任务不处于可对账的恢复状态")
	}
	next := CloneTaskRun(run)
	var event TaskDomainEventType
	switch committed.Status {
	case "succeeded":
		if committed.Result == nil || committed.Result.TaskRunID != run.TaskRunID || committed.Result.ResultHash == "" {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者确认的成功结果缺少有效引用")
		}
		next.Status, event = RunStatusSucceeded, TaskEventSucceeded
		if committed.Result.ResultType == ResultArtifact {
			id := committed.Result.ArtifactID
			next.ResultArtifactID = &id
		}
		next.ErrorCode, next.ErrorMessage = nil, nil
	case "failed":
		next.Status, event = RunStatusFailed, TaskEventFailed
	case "cancelled":
		next.Status, event = RunStatusCancelled, TaskEventCancelled
	case "timed_out":
		next.Status, event = RunStatusTimedOut, TaskEventTimedOut
	case "manual_intervention":
		next.Status, event = RunStatusManualIntervention, TaskEventFailed
	default:
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者返回未知终态")
	}
	if committed.Status != "succeeded" {
		if committed.Result != nil {
			return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "失败终态包含意外结果")
		}
		code := "task_" + committed.Status
		message := "任务已停止，详细结果保存在数据所有者处"
		next.ErrorCode, next.ErrorMessage = &code, &message
	}
	finishedAt := time.Now().UTC()
	if committed.Result != nil && !committed.Result.CreatedAt.IsZero() {
		finishedAt = committed.Result.CreatedAt
	}
	next.FinishedAt = &finishedAt
	next.Revision = NextRevision(run.Revision)
	if err := s.withManagementTaskTx(ctx, func(txCtx context.Context) error {
		ok, err := s.store.UpdateTaskRunCAS(txCtx, next, run.Status, run.Generation, run.Revision)
		if err != nil {
			return err
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "任务终态对账时发生并发变化")
		}
		if committed.Result != nil {
			metadata := *committed.Result
			metadata.ResultJSON = nil
			if err := s.store.PutResult(txCtx, &metadata); err != nil {
				return err
			}
		}
		if err := s.store.RemoveFromQueue(txCtx, run.TaskRunID); err != nil {
			return err
		}
		return s.publishTaskEvent(txCtx, event, next, "owner_outcome_reconciled", "")
	}); err != nil {
		return nil, err
	}
	return next, nil
}

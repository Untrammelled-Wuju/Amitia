package task_runtime

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *TaskRuntimeService) confirmRemotePaused(ctx context.Context, expected *TaskRun) error {
	if expected == nil || expected.Status != RunStatusPaused || expected.PausedAt == nil {
		return NewTaskError(ErrTaskPauseInProgress, "远端任务尚未取得持久暂停状态")
	}
	executor, supported := s.remoteExecutor.(interface {
		ConfirmPauseStopped(context.Context, *TaskRun) bool
	})
	if !supported {
		return NewTaskError(ErrTaskPauseInProgress, "远端执行器不支持停止凭证核验")
	}
	confirmation, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if !executor.ConfirmPauseStopped(confirmation, expected) {
		return NewTaskError(ErrTaskPauseInProgress, "原设备执行停止尚未确认")
	}
	latest, err := s.store.GetTaskRun(ctx, expected.TaskRunID)
	if err != nil {
		return err
	}
	if latest == nil || latest.TaskRunID != expected.TaskRunID || latest.ScopeSnapshotID != expected.ScopeSnapshotID || latest.Status != RunStatusPaused || latest.Revision != expected.Revision ||
		latest.Generation != expected.Generation || latest.ExecutionAttemptID != expected.ExecutionAttemptID ||
		latest.LeaseID != expected.LeaseID || latest.PausedAt == nil {
		return NewTaskError(ErrTaskResumeStaleGeneration, "设备停止确认期间任务状态或执行代次已变化")
	}
	return coordination.ValidateCurrent(ctx)
}

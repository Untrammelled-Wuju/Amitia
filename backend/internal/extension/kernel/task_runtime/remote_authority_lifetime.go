package task_runtime

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *TaskRuntimeService) waitOwnedRemoteExecution(ctx context.Context, run *TaskRun, executor TaskExecutorPort) error {
	if _, owned := coordination.FromContext(ctx); !owned {
		return errors.New("远端任务缺少设备授权范围")
	}
	if run.DeadlineAt != nil {
		var stop context.CancelFunc
		ctx, stop = context.WithDeadline(ctx, *run.DeadlineAt)
		defer stop()
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	cancelRemote := func() {
		remote, ok := executor.(interface {
			Cancel(context.Context, *TaskRun) error
		})
		if !ok {
			return
		}
		grace := s.config.CancelGracePeriod
		if grace <= 0 || grace > 10*time.Second {
			grace = 10 * time.Second
		}
		cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
		defer cancel()
		if waiter, ok := executor.(interface {
			CancelAndWait(context.Context, *TaskRun, time.Duration) error
		}); ok {
			_ = waiter.CancelAndWait(cancelCtx, run, grace)
		} else {
			_ = remote.Cancel(cancelCtx, run)
		}
	}
	for {
		if err := coordination.ValidateCurrent(ctx); err != nil {
			cancelRemote()
			return err
		}
		current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
		if err != nil || current == nil {
			cancelRemote()
			return NewTaskError(ErrTaskScopeDenied, "任务结果来源尚未确认")
		}
		if current.Generation != run.Generation || current.ExecutionAttemptID != run.ExecutionAttemptID || current.ScopeSnapshotID != run.ScopeSnapshotID || current.TaskDefinitionID != run.TaskDefinitionID || current.ExtensionID != run.ExtensionID || current.ModuleID != run.ModuleID || current.InvocationID != run.InvocationID || current.InputHash != run.InputHash {
			cancelRemote()
			return NewTaskError(ErrTaskScopeDenied, "远端任务执行代次已变化")
		}
		if current.Status.IsTerminal() {
			if err := coordination.ValidateCurrent(ctx); err != nil {
				cancelRemote()
				return err
			}
			return nil
		}
		select {
		case <-ctx.Done():
			cancelRemote()
			return context.Cause(ctx)
		case <-ticker.C:
		}
	}
}

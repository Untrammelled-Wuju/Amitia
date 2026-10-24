package task_runtime

import (
	"context"
	"time"
)

func (s *TaskRuntimeService) CancelAndWait(ctx context.Context, taskRunID, reason string) error {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil || current == nil {
		return NewTaskError(ErrTaskNotFound, "待取消的任务不存在")
	}
	if current.Status.IsTerminal() && current.Status != RunStatusCancelled {
		return NewTaskError(ErrTaskNotCancelable, "任务已终结但未确认取消: "+string(current.Status))
	}
	if !current.Status.IsTerminal() {
		if err := s.Cancel(ctx, taskRunID, reason); err != nil {
			return err
		}
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.RLock()
		host := s.activeHosts[taskRunID]
		s.mu.RUnlock()
		if host != nil {
			select {
			case <-host.Done():
				live, err := s.store.GetTaskRun(ctx, taskRunID)
				if err != nil || live == nil {
					return NewTaskError(ErrTaskNotFound, "任务取消结果尚未确认")
				}
				if live.Generation != current.Generation || live.ExecutionAttemptID != current.ExecutionAttemptID {
					return NewTaskError(ErrTaskExecutionAttemptInvalid, "取消期间任务执行代次已变化")
				}
				if live.Status == RunStatusCancelling {
					s.handleFinished(ctx, live, "cancelled", nil, "", "", "任务已取消")
				}
				confirmed, err := s.store.GetTaskRun(ctx, taskRunID)
				if err != nil || confirmed == nil {
					return NewTaskError(ErrTaskExecutionAttemptInvalid, "进程已停止，取消状态尚未确认")
				}
				if confirmed.Generation != current.Generation || confirmed.ExecutionAttemptID != current.ExecutionAttemptID {
					return NewTaskError(ErrTaskExecutionAttemptInvalid, "取消确认期间执行代次已变化")
				}
				if confirmed.Status != RunStatusCancelled {
					return NewTaskError(ErrTaskExecutionAttemptInvalid, "进程停止后任务未确认取消: "+string(confirmed.Status))
				}
				return nil
			case <-ctx.Done():
				host.ForceStop()
				return ctx.Err()
			}
		}
		live, err := s.store.GetTaskRun(ctx, taskRunID)
		if err != nil || live == nil {
			return NewTaskError(ErrTaskNotFound, "任务取消结果尚未确认")
		}
		if live.Generation != current.Generation || live.ExecutionAttemptID != current.ExecutionAttemptID {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "取消期间任务执行代次已变化")
		}
		if live.Status.IsTerminal() {
			if live.Status != RunStatusCancelled {
				return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务终态并非已确认取消: "+string(live.Status))
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

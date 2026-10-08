package task_runtime

import (
	"context"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"time"
)

func (e *MeshRemoteTaskExecutor) RequestPause(ctx context.Context, run *TaskRun, reason string) error {
	if run == nil || e.hub == nil || e.PendingTasks == nil || run.EffectiveExecutionPlacement() != TaskExecutionPlacementDevice {
		return NewTaskError(ErrTaskPauseUnsupported, "设备任务暂停通道不可用")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	target := run.ExecutionTarget
	if !e.PendingTasks.ValidateBound(run.TaskRunID, run.ExecutionAttemptID.String(), run.LeaseID, target.RuntimeSessionID.String(), target.ConnectionGeneration) {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务暂停租约已失效")
	}
	request := protocol.TaskPausePayload{TaskRunID: run.TaskRunID, AttemptID: run.ExecutionAttemptID.String(), LeaseID: run.LeaseID, Reason: reason, RuntimeSessionID: target.RuntimeSessionID, ConnectionGeneration: target.ConnectionGeneration, SentAt: time.Now().UTC()}
	if !e.hub.SendEnvelope(target.RuntimeSessionID, target.ConnectionGeneration, protocol.MessageTypeTaskPause, request) {
		return NewTaskError(ErrTaskPauseUnsupported, "任务暂停请求尚未送达设备")
	}
	return nil
}

func (e *MeshRemoteTaskExecutor) ConfirmPauseStopped(ctx context.Context, run *TaskRun) bool {
	if run == nil || e.PendingTasks == nil || run.Status != RunStatusPaused || run.PausedAt == nil || run.ScopeSnapshotID == "" || run.Generation < 1 || run.ExecutionAttemptID == "" || run.LeaseID == "" || run.ExecutionTarget.RuntimeSessionID == "" || run.ExecutionTarget.ConnectionGeneration < 1 {
		return false
	}
	if authority, owned := coordination.FromContext(ctx); !owned || run.ExecutionTarget.SpaceID.String() != authority.SpaceID || run.ExecutionTarget.DeviceID.String() != authority.TargetDeviceID || context.Cause(ctx) != nil {
		return false
	}
	if pending, exists := e.PendingTasks.Get(run.TaskRunID); exists {
		if pending.AttemptID != run.ExecutionAttemptID.String() || pending.LeaseID != run.LeaseID || pending.SessionID != run.ExecutionTarget.RuntimeSessionID.String() || pending.Generation != run.ExecutionTarget.ConnectionGeneration {
			return false
		}
		return e.PendingTasks.WaitForCancelAck(ctx, run.TaskRunID)
	}
	return true
}

func (s *TaskRuntimeService) HandleRemotePaused(ctx context.Context, taskID, attemptID, leaseID string, version int64) error {
	unlock := s.lockTaskOwner(taskID)
	defer unlock()
	if version < 1 || version > 9007199254740991 {
		return NewTaskError(ErrTaskCheckpointIncompatible, "暂停检查点版本无效")
	}
	if err := s.validateRemoteAttemptLease(ctx, taskID, attemptID, leaseID); err != nil {
		return err
	}
	run, err := s.store.GetTaskRun(ctx, taskID)
	if err != nil || run == nil || run.EffectiveExecutionPlacement() != TaskExecutionPlacementDevice || run.Status != RunStatusPausing && run.Status != RunStatusPaused {
		return NewTaskError(ErrTaskPauseUnsupported, "设备任务当前没有等待暂停确认")
	}
	guarded, finish, owned, err := s.callbackAuthority(ctx, run, attemptID, run.Generation)
	if err != nil {
		return err
	}
	defer finish()
	if !owned {
		return NewTaskError(ErrTaskScopeDenied, "设备任务暂停缺少所有者授权")
	}
	definition, err := s.store.GetTaskDefinition(guarded, run.TaskDefinitionID)
	if err != nil {
		return err
	}
	checkpoint, err := s.readTaskCheckpoint(guarded, run)
	if err != nil || !definition.Checkpoint || checkpoint == nil || run.CheckpointID == nil || checkpoint.CheckpointID != *run.CheckpointID || checkpoint.Version != version || checkpoint.DefinitionHash != definition.DefinitionHash || checkpoint.InputHash != run.InputHash || checkpoint.PayloadHash != hashBytes(checkpoint.Payload) {
		return NewTaskError(ErrTaskCheckpointIncompatible, "设备暂停检查点尚未获得数据所有者确认")
	}
	if err := coordination.ValidateCurrent(guarded); err != nil {
		return err
	}
	if run.Status == RunStatusPaused {
		if run.PausedAt == nil {
			return NewTaskError(ErrTaskPauseInProgress, "设备停止凭证缺失，不能确认旧暂停状态")
		}
		return nil
	}
	now := time.Now().UTC()
	next := CloneTaskRun(run)
	next.Status, next.PausedAt = RunStatusPaused, &now
	next.Revision = NextRevision(run.Revision)
	return s.mutateTaskRun(guarded, taskMutationParams{next: next, expected: run.Status, generation: run.Generation, revision: run.Revision, removeQ: true, eventType: TaskEventPaused})
}

func (s *TaskRuntimeService) pauseRemoteTask(ctx context.Context, run *TaskRun, reason string) error {
	executor, supported := s.remoteExecutor.(interface {
		RequestPause(context.Context, *TaskRun, string) error
		ConfirmPauseStopped(context.Context, *TaskRun) bool
	})
	if !supported {
		return s.failPause(ctx, run, "设备执行器不支持已确认暂停")
	}
	if err := executor.RequestPause(ctx, run, reason); err != nil {
		return s.failPause(ctx, run, "设备暂停请求未送达，执行结果需人工核对")
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	deadline, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	for {
		current, err := s.store.GetTaskRun(deadline, run.TaskRunID)
		if err != nil {
			return err
		}
		if current == nil || current.Generation != run.Generation || current.ExecutionAttemptID != run.ExecutionAttemptID {
			return NewTaskError(ErrTaskResumeStaleGeneration, "暂停期间设备执行代次已变化")
		}
		if current.Status == RunStatusPaused {
			if executor.ConfirmPauseStopped(deadline, current) {
				return coordination.ValidateCurrent(ctx)
			}
			return NewTaskError(ErrTaskPauseInProgress, "设备停止确认尚未完成，暂时不能恢复")
		}
		if current.Status != RunStatusPausing {
			return NewTaskError(ErrTaskPauseUnsupported, "设备任务暂停未确认")
		}
		select {
		case <-deadline.Done():
			cleanup, finish := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer finish()
			_ = s.remoteExecutor.Cancel(cleanup, current)
			return s.failPause(cleanup, current, "设备暂停超时，停止结果尚未确认")
		case <-ticker.C:
		}
	}
}

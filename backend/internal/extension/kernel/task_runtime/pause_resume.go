package task_runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type PauseTaskRequest struct {
	TaskRunID  string `json:"taskRunId"`
	Reason     string `json:"reason,omitempty"`
	Generation int64  `json:"generation"`
}

type PauseTaskResponse struct {
	Paused    bool   `json:"Paused"`
	NewStatus string `json:"newStatus"`
}

type ResumeTaskRequest struct {
	TaskRunID  string `json:"taskRunId"`
	Generation int64  `json:"generation"`
	ResumeKind string `json:"resumeKind,omitempty"`
}

type ResumeTaskResponse struct {
	Resumed   bool   `json:"resumed"`
	NewStatus string `json:"newStatus"`
}

const (
	ResumeKindResume     = "resume"
	ResumeKindResumeFrom = "resume_from_checkpoint"
)

func (s *TaskRuntimeService) PauseTask(ctx context.Context, req PauseTaskRequest) error {
	unlock := s.lockTaskOwner(req.TaskRunID)
	defer unlock()
	current, err := s.store.GetTaskRun(ctx, req.TaskRunID)
	if err != nil {
		return NewTaskError(ErrTaskNotFound, err.Error())
	}
	guarded, finish, err := s.restoreTaskAuthority(ctx, current)
	if err != nil {
		return err
	}
	defer finish()
	ctx = guarded

	def, err := s.store.GetTaskDefinition(ctx, current.TaskDefinitionID)
	if err != nil {
		return NewTaskError(ErrTaskDefinitionInvalid, err.Error())
	}

	if !def.Checkpoint {
		return NewTaskError(ErrTaskPauseUnsupported, "pause not supported for this task definition")
	}

	if req.Generation != 0 && req.Generation != current.Generation {
		return NewTaskError(ErrTaskResumeStaleGeneration, "stale generation")
	}

	if current.Status == RunStatusPaused {
		return nil
	}

	if current.Status != RunStatusRunning && current.Status != RunStatusCheckpointing {
		return NewTaskError(ErrTaskPauseUnsupported, "cannot pause in status: "+string(current.Status))
	}

	if placement := current.EffectiveExecutionPlacement(); placement != TaskExecutionPlacementLocal && placement != TaskExecutionPlacementDevice {
		return NewTaskError(ErrTaskPauseUnsupported, "当前执行位置不支持已确认暂停")
	}

	now := time.Now().UTC()
	pausedRun := cloneTaskRun(current)
	pausedRun.Status = RunStatusPausing
	reason := req.Reason
	pausedRun.PauseReason = &reason
	pausedRun.PauseRequestedAt = &now
	pausedRun.Revision = NextRevision(current.Revision)

	if err := s.mutateTaskRun(ctx, taskMutationParams{
		next:       pausedRun,
		expected:   current.Status,
		generation: current.Generation,
		revision:   current.Revision,
		eventType:  TaskEventPausing,
		eventMsg:   reason,
	}); err != nil {
		return err
	}
	if current.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice {
		unlock()
		return s.pauseRemoteTask(ctx, pausedRun, reason)
	}
	unlock()

	s.mu.RLock()
	host := s.activeHosts[req.TaskRunID]
	s.mu.RUnlock()

	if host == nil {
		return s.failPause(ctx, pausedRun, "任务执行进程不可用，暂停未确认")
	}
	pauseCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	version, err := host.Pause(pauseCtx)
	if err != nil {
		stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer stopCancel()
		host.ForceStop()
		select {
		case <-host.Done():
			return s.failPause(stopCtx, pausedRun, "任务进程暂停未确认")
		case <-stopCtx.Done():
			return NewTaskError(ErrTaskPauseInProgress, "任务进程停止尚未确认")
		}
	}
	latest, err := s.store.GetTaskRun(ctx, req.TaskRunID)
	if err != nil {
		return err
	}
	cp, err := s.readTaskCheckpoint(ctx, latest)
	if err != nil || cp == nil || cp.Version != version || cp.DefinitionHash != def.DefinitionHash || cp.InputHash != latest.InputHash || cp.PayloadHash != hashBytes(cp.Payload) {
		return s.failPause(ctx, pausedRun, "暂停检查点未完成持久化确认")
	}
	if latest.Status != RunStatusPausing || latest.Generation != current.Generation || latest.ExecutionAttemptID != current.ExecutionAttemptID {
		return NewTaskError(ErrTaskResumeStaleGeneration, "暂停期间任务执行已变化")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	finished := time.Now().UTC()
	next := cloneTaskRun(latest)
	next.Status, next.PausedAt = RunStatusPaused, &finished
	next.Revision = NextRevision(latest.Revision)
	return s.mutateTaskRun(ctx, taskMutationParams{next: next, expected: latest.Status, generation: latest.Generation, revision: latest.Revision, removeQ: true, eventType: TaskEventPaused, eventMsg: reason})
}

func (s *TaskRuntimeService) failPause(ctx context.Context, expected *TaskRun, message string) error {
	current, err := s.store.GetTaskRun(ctx, expected.TaskRunID)
	if err != nil {
		return err
	}
	if current != nil && current.Status == RunStatusPausing && current.Generation == expected.Generation && current.ExecutionAttemptID == expected.ExecutionAttemptID {
		next := cloneTaskRun(current)
		next.Status = RunStatusRecoveryRequired
		next.ErrorMessage = &message
		next.Revision = NextRevision(current.Revision)
		if err := s.mutateTaskRun(ctx, taskMutationParams{next: next, expected: current.Status, generation: current.Generation, revision: current.Revision, removeQ: true, eventType: TaskEventRecoveryRequired, eventMsg: message}); err != nil {
			return err
		}
	}
	return NewTaskError(ErrTaskPauseUnsupported, message)
}

func (s *TaskRuntimeService) ResumeTask(ctx context.Context, req ResumeTaskRequest) error {
	current, err := s.store.GetTaskRun(ctx, req.TaskRunID)
	if err != nil {
		return NewTaskError(ErrTaskNotFound, err.Error())
	}

	if current.Status != RunStatusPaused {
		return NewTaskError(ErrTaskNotPaused, "task not paused")
	}

	if req.Generation != 0 && req.Generation != current.Generation {
		return NewTaskError(ErrTaskResumeStaleGeneration, "stale generation")
	}

	if placement := current.EffectiveExecutionPlacement(); placement != TaskExecutionPlacementLocal && placement != TaskExecutionPlacementDevice {
		return NewTaskError(ErrTaskResumeIncompatible, "当前执行位置不支持检查点恢复")
	}

	if req.ResumeKind == "" {
		req.ResumeKind = ResumeKindResume
	}

	if req.ResumeKind != ResumeKindResume && req.ResumeKind != ResumeKindResumeFrom {
		return NewTaskError(ErrTaskResumeIncompatible, "恢复方式无效")
	}
	s.mu.RLock()
	host := s.activeHosts[req.TaskRunID]
	s.mu.RUnlock()
	if host != nil {
		return NewTaskError(ErrTaskPauseInProgress, "旧执行进程尚未完成清理")
	}
	def, err := s.store.GetTaskDefinition(ctx, current.TaskDefinitionID)
	if err != nil {
		return err
	}
	restored, finish, err := s.restoreTaskAuthority(ctx, current)
	if err != nil {
		return err
	}
	defer finish()
	cp, err := s.readTaskCheckpoint(restored, current)
	if err != nil || cp == nil || current.CheckpointID == nil || cp.CheckpointID != *current.CheckpointID || cp.DefinitionHash != def.DefinitionHash || cp.InputHash != current.InputHash || cp.PayloadHash != hashBytes(cp.Payload) {
		return NewTaskError(ErrTaskCheckpointIncompatible, "恢复检查点缺失或与任务不匹配")
	}
	_, owned := coordination.FromContext(restored)
	if err := validateTaskDefinition(owned, current, def); err != nil {
		return err
	}
	if err := coordination.ValidateCurrent(restored); err != nil {
		return err
	}
	if current.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice {
		executor, supported := s.remoteExecutor.(interface {
			ConfirmPauseStopped(context.Context, *TaskRun) bool
		})
		confirmation, cancel := context.WithTimeout(restored, time.Second)
		defer cancel()
		if !supported || !executor.ConfirmPauseStopped(confirmation, current) {
			return NewTaskError(ErrTaskPauseInProgress, "旧设备执行停止尚未确认，不能恢复")
		}
	}
	if err := coordination.ValidateCurrent(restored); err != nil {
		return err
	}
	now := time.Now().UTC()
	if current.DeadlineAt != nil && !current.DeadlineAt.After(now) {
		return NewTaskError(ErrTaskResumeIncompatible, "任务执行期限已过")
	}
	resumedRun := cloneTaskRun(current)
	resumedRun.Status = RunStatusQueued
	resumedRun.Generation++
	resumedRun.ExecutionAttemptID = ""
	resumedRun.LeaseID = ""
	resumedRun.LeaseExpiresAt, resumedRun.LastHeartbeatAt = nil, nil
	resumedRun.RuntimeInstanceID = nil
	resumedRun.ResumedAt, resumedRun.QueuedAt = &now, &now
	resumedRun.Revision = NextRevision(current.Revision)
	if owned && resumedRun.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice && len(def.PermissionRequirements)+len(def.PermissionRequirementStrings) > 0 {
		if s.config.OwnedInputs == nil || s.config.OwnedTargetDefinitions == nil {
			return NewTaskError(ErrTaskPermissionDenied, "恢复任务缺少所有者输入和设备版本端口")
		}
		input, err := s.config.OwnedInputs.Input(restored, current)
		if err != nil {
			return err
		}
		pin, err := s.config.OwnedTargetDefinitions.Prepare(restored, current, def)
		if err != nil {
			return err
		}
		if err := s.prepareOwnedTargetPermissions(restored, resumedRun, def, pin, input); err != nil {
			return err
		}
	}
	if err := s.withManagementTaskTx(restored, func(txCtx context.Context) error {
		ok, err := s.store.UpdateTaskRunCAS(txCtx, resumedRun, current.Status, current.Generation, current.Revision)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("恢复任务前执行状态已变化")
		}
		if err := s.queue.Enqueue(txCtx, resumedRun); err != nil {
			return err
		}
		return s.publishTaskEvent(txCtx, TaskEventResumed, resumedRun, "", "")
	}); err != nil {
		return err
	}
	go s.tryDispatch()
	return nil
}

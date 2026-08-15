package task_runtime

import (
	"context"
	"fmt"
	"time"
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
	current, err := s.store.GetTaskRun(ctx, req.TaskRunID)
	if err != nil {
		return NewTaskError(ErrTaskNotFound, err.Error())
	}

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

	if current.EffectiveExecutionPlacement() != TaskExecutionPlacementLocal {
		return NewTaskError(ErrTaskPauseUnsupported, "only local task execution can be paused in G13")
	}

	pausingRun := cloneTaskRun(current)
	pausingRun.Status = RunStatusPausing
	reason := req.Reason
	pausingRun.PauseReason = &reason
	now := time.Now().UTC()
	pausingRun.PauseRequestedAt = &now

	if ok, casErr := s.store.UpdateTaskRunCAS(ctx, pausingRun, current.Status, current.Generation, current.Revision); casErr != nil {
		return fmt.Errorf("task_runtime: pause cas: %w", casErr)
	} else if !ok {
		return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry pause")
	}

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		pausedRun := cloneTaskRun(pausingRun)
		pausedRun.Status = RunStatusPaused
		pausedRun.PausedAt = &now
		pausedRun.Revision = pausingRun.Revision
		if err := s.store.PutTaskRun(txCtx, pausedRun); err != nil {
			return err
		}
		return s.publishTaskEvent(txCtx, TaskEventPaused, pausedRun, reason, "")
	}); err != nil {
		return err
	}

	s.mu.RLock()
	host, ok := s.activeHosts[req.TaskRunID]
	s.mu.RUnlock()

	if ok {
		statusNotify := make(chan string, 1)
		cancelCh := host.CancelCh()
		_ = cancelCh

		select {
		case statusNotify <- "pausing":
		default:
		}
		_ = statusNotify
	}

	return nil
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

	if current.EffectiveExecutionPlacement() != TaskExecutionPlacementLocal {
		return NewTaskError(ErrTaskResumeIncompatible, "only local task execution can be resumed in G13")
	}

	if req.ResumeKind == "" {
		req.ResumeKind = ResumeKindResume
	}

	resumingRun := cloneTaskRun(current)
	resumingRun.Status = RunStatusResuming
	now := time.Now().UTC()
	resumingRun.ResumedAt = &now

	if ok, casErr := s.store.UpdateTaskRunCAS(ctx, resumingRun, current.Status, current.Generation, current.Revision); casErr != nil {
		return fmt.Errorf("task_runtime: resume cas: %w", casErr)
	} else if !ok {
		return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry resume")
	}

	nextStatus := RunStatusRunning
	var failMsg string
	if req.ResumeKind == ResumeKindResumeFrom {
		if current.CheckpointID == nil || *current.CheckpointID == "" {
			failMsg = "resume_from_checkpoint but no checkpoint available"
			nextStatus = RunStatusFailed
		} else {
			cp, err := s.store.GetLatestCheckpoint(ctx, current.TaskRunID)
			if err != nil || cp == nil {
				failMsg = "resume_from_checkpoint but checkpoint not found"
				nextStatus = RunStatusFailed
			}
		}
	}

	next := cloneTaskRun(resumingRun)
	next.Status = nextStatus
	if failMsg != "" {
		next.ErrorMessage = &failMsg
	}
	next.Revision = resumingRun.Revision

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if err := s.store.PutTaskRun(txCtx, next); err != nil {
			return err
		}
		if failMsg != "" {
			return s.publishTaskEvent(txCtx, TaskEventFailed, next, failMsg, "")
		}
		return s.publishTaskEvent(txCtx, TaskEventResumed, next, "", "")
	}); err != nil {
		return err
	}

	return nil
}

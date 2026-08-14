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
	run, err := s.store.GetTaskRun(ctx, req.TaskRunID)
	if err != nil {
		return NewTaskError(ErrTaskNotFound, err.Error())
	}

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return NewTaskError(ErrTaskDefinitionInvalid, err.Error())
	}

	if !def.Checkpoint {
		return NewTaskError(ErrTaskPauseUnsupported, "pause not supported for this task definition")
	}

	if req.Generation != 0 && req.Generation != run.Generation {
		return NewTaskError(ErrTaskResumeStaleGeneration, "stale generation")
	}

	if run.Status == RunStatusPaused {
		return nil
	}

	if run.Status != RunStatusRunning && run.Status != RunStatusCheckpointing {
		return NewTaskError(ErrTaskPauseUnsupported, "cannot pause in status: "+string(run.Status))
	}

	if run.EffectiveExecutionPlacement() != TaskExecutionPlacementLocal {
		return NewTaskError(ErrTaskPauseUnsupported, "only local task execution can be paused in G13")
	}

	previousStatus := run.Status
	run.Status = RunStatusPausing
	reason := req.Reason
	run.PauseReason = &reason
	now := time.Now().UTC()
	run.PauseRequestedAt = &now

	if ok, casErr := s.store.UpdateTaskRunCAS(ctx, run, previousStatus, run.Generation); casErr != nil {
		return fmt.Errorf("task_runtime: pause cas: %w", casErr)
	} else if !ok {
		return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry pause")
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
	run, err := s.store.GetTaskRun(ctx, req.TaskRunID)
	if err != nil {
		return NewTaskError(ErrTaskNotFound, err.Error())
	}

	if run.Status != RunStatusPaused {
		return NewTaskError(ErrTaskNotPaused, "task not paused")
	}

	if req.Generation != 0 && req.Generation != run.Generation {
		return NewTaskError(ErrTaskResumeStaleGeneration, "stale generation")
	}

	if run.EffectiveExecutionPlacement() != TaskExecutionPlacementLocal {
		return NewTaskError(ErrTaskResumeIncompatible, "only local task execution can be resumed in G13")
	}

	if req.ResumeKind == "" {
		req.ResumeKind = ResumeKindResume
	}

	previousStatus := run.Status
	run.Status = RunStatusResuming
	now := time.Now().UTC()
	run.ResumedAt = &now

	if ok, casErr := s.store.UpdateTaskRunCAS(ctx, run, previousStatus, run.Generation); casErr != nil {
		return fmt.Errorf("task_runtime: resume cas: %w", casErr)
	} else if !ok {
		return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry resume")
	}

	run.Status = RunStatusRunning
	if req.ResumeKind == ResumeKindResumeFrom {
		if run.CheckpointID == nil || *run.CheckpointID == "" {
			run.Status = RunStatusFailed
			msg := "resume_from_checkpoint but no checkpoint available"
			run.ErrorMessage = &msg
			_ = s.store.PutTaskRun(ctx, run)
			return NewTaskError(ErrTaskResumeFailed, msg)
		}
		cp, err := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
		if err != nil || cp == nil {
			run.Status = RunStatusFailed
			msg := "resume_from_checkpoint but checkpoint not found"
			run.ErrorMessage = &msg
			_ = s.store.PutTaskRun(ctx, run)
			return NewTaskError(ErrTaskResumeFailed, msg)
		}
	}

	_ = s.store.PutTaskRun(ctx, run)

	return nil
}

package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type PauseTaskResult struct {
	TaskRunID        string        `json:"taskRunId"`
	Status           TaskRunStatus `json:"status"`
	CheckpointID     string        `json:"checkpointId,omitempty"`
	CheckpointVersion int64        `json:"checkpointVersion,omitempty"`
	Generation       int64         `json:"generation"`
}

type TaskPauseAck struct {
	TaskRunID        string `json:"task_run_id"`
	CheckpointVersion int64 `json:"checkpoint_version"`
	CheckpointHash   string `json:"checkpoint_hash"`
	Paused           bool   `json:"paused"`
}

func (s *TaskRuntimeService) Pause(ctx context.Context, taskRunID string, reason string) (*TaskRun, error) {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, NewTaskError(ErrTaskNotFound, err.Error())
	}

	if run.Status == RunStatusPaused {
		return run, nil
	}
	if run.Status.IsTerminal() {
		return nil, NewTaskError(ErrTaskPauseUnsupported, "task already terminal: "+string(run.Status))
	}
	if run.Status == RunStatusPausing {
		return nil, NewTaskError(ErrTaskPauseInProgress, "pause already in progress")
	}
	if run.Status == RunStatusCancelled || run.Status == RunStatusCancelling {
		return nil, NewTaskError(ErrTaskPauseUnsupported, "task is being cancelled")
	}

	if run.Status == RunStatusQueued {
		return s.pauseQueued(ctx, run, reason)
	}

	if run.Status == RunStatusRunning || run.Status == RunStatusCheckpointing || run.Status == RunStatusResuming {
		return s.pauseRunning(ctx, run, reason)
	}

	return nil, NewTaskError(ErrTaskPauseUnsupported, "cannot pause from status: "+string(run.Status))
}

func (s *TaskRuntimeService) pauseQueued(ctx context.Context, run *TaskRun, reason string) (*TaskRun, error) {
	expectedGeneration := run.Generation
	now := time.Now().UTC()
	run.Status = RunStatusPausing
	run.PauseReason = &reason
	run.PauseRequestedAt = &now

	if ok, err := s.store.UpdateTaskRunCAS(ctx, run, RunStatusQueued, expectedGeneration); err != nil {
		return nil, fmt.Errorf("task_runtime: pause queued cas: %w", err)
	} else if !ok {
		return nil, NewTaskError(ErrTaskPauseInProgress, "concurrent state change")
	}

	if err := s.queue.Remove(ctx, run.TaskRunID); err != nil {
		return nil, fmt.Errorf("task_runtime: remove queued task: %w", err)
	}

	run.Status = RunStatusPaused
	run.PausedAt = &now
	if ok, err := s.store.UpdateTaskRunCAS(ctx, run, RunStatusPausing, expectedGeneration); err != nil {
		return nil, fmt.Errorf("task_runtime: finalize paused: %w", err)
	} else if !ok {
		return nil, NewTaskError(ErrTaskPauseInProgress, "concurrent state change during finalize")
	}

	s.emitPauseEvent(ctx, run, "task.paused")
	return run, nil
}

func (s *TaskRuntimeService) pauseRunning(ctx context.Context, run *TaskRun, reason string) (*TaskRun, error) {
	expectedGeneration := run.Generation
	previousStatus := run.Status

	if !s.canPauseRunning(run) {
		return nil, NewTaskError(ErrTaskPauseUnsupported, "task recoverability does not support checkpoint-resume pause")
	}

	now := time.Now().UTC()
	run.Status = RunStatusPausing
	run.PauseReason = &reason
	run.PauseRequestedAt = &now

	if ok, err := s.store.UpdateTaskRunCAS(ctx, run, previousStatus, expectedGeneration); err != nil {
		return nil, fmt.Errorf("task_runtime: pause running cas: %w", err)
	} else if !ok {
		return nil, NewTaskError(ErrTaskPauseInProgress, "concurrent state change")
	}

	s.mu.RLock()
	host, ok := s.activeHosts[run.TaskRunID]
	s.mu.RUnlock()

	if !ok {
		run.Status = RunStatusRecoveryRequired
		_ = s.store.PutTaskRun(ctx, run)
		return nil, NewTaskError(ErrTaskPauseTimeout, "no active host found for running task")
	}

	pauseTimeout := s.config.PauseGracePeriod
	if pauseTimeout <= 0 {
		pauseTimeout = 30 * time.Second
	}

	pauseCtx, cancel := context.WithTimeout(ctx, pauseTimeout)
	defer cancel()

	if err := host.RequestPause(pauseTimeout); err != nil {
		run.Status = RunStatusRunning
		_ = s.store.PutTaskRun(ctx, run)
		return nil, NewTaskError(ErrTaskPauseTimeout, "pause request failed: "+err.Error())
	}

	select {
	case ack := <-host.PauseAck():
		return s.handlePauseAck(pauseCtx, run, ack)
	case <-host.Done():
		return s.handlePauseHostExit(pauseCtx, run)
	case <-pauseCtx.Done():
		run.Status = RunStatusRunning
		_ = s.store.PutTaskRun(ctx, run)
		return nil, NewTaskError(ErrTaskPauseTimeout, "pause timed out waiting for checkpoint ack")
	}
}

func (s *TaskRuntimeService) canPauseRunning(run *TaskRun) bool {
	def, err := s.store.GetTaskDefinition(context.Background(), run.TaskDefinitionID)
	if err != nil {
		return false
	}

	recoverability := def.Recoverability
	if recoverability == "" {
		if def.Recoverable {
			recoverability = CheckpointRecoverable
		} else {
			recoverability = NotRecoverable
		}
	}

	return recoverability == CheckpointRecoverable
}

func (s *TaskRuntimeService) handlePauseAck(ctx context.Context, run *TaskRun, ack *TaskPauseAck) (*TaskRun, error) {
	cp, err := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
	if err != nil {
		return nil, NewTaskError(ErrTaskCheckpointInvalid, "failed to load checkpoint after ack")
	}
	if cp == nil {
		run.Status = RunStatusRecoveryRequired
		_ = s.store.PutTaskRun(ctx, run)
		return nil, NewTaskError(ErrTaskCheckpointInvalid, "no checkpoint available after pause ack")
	}

	if ack.CheckpointVersion != cp.Version {
		run.Status = RunStatusRecoveryRequired
		_ = s.store.PutTaskRun(ctx, run)
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "checkpoint version mismatch")
	}
	if ack.CheckpointHash != "" && ack.CheckpointHash != cp.PayloadHash {
		run.Status = RunStatusRecoveryRequired
		_ = s.store.PutTaskRun(ctx, run)
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "checkpoint hash mismatch")
	}

	generation := run.Generation
	run.Status = RunStatusPaused
	now := time.Now().UTC()
	run.PausedAt = &now
	run.RuntimeInstanceID = nil

	if ok, err := s.store.UpdateTaskRunCAS(ctx, run, RunStatusPausing, generation); err != nil {
		return nil, fmt.Errorf("task_runtime: finalize paused cas: %w", err)
	} else if !ok {
		return nil, NewTaskError(ErrTaskPauseInProgress, "concurrent state change during finalize")
	}

	s.removeActiveHost(run.TaskRunID)

	s.emitPauseEvent(ctx, run, "task.paused")
	return run, nil
}

func (s *TaskRuntimeService) handlePauseHostExit(ctx context.Context, run *TaskRun) (*TaskRun, error) {
	cp, err := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
	if err != nil {
		run.Status = RunStatusRecoveryRequired
		_ = s.store.PutTaskRun(ctx, run)
		return nil, NewTaskError(ErrTaskRecoveryRequired, "host exited during pause, checkpoint may be invalid")
	}
	if cp != nil && run.Status == RunStatusPausing {
		generation := run.Generation
		run.Status = RunStatusPaused
		now := time.Now().UTC()
		run.PausedAt = &now
		run.RuntimeInstanceID = nil
		if ok, _ := s.store.UpdateTaskRunCAS(ctx, run, RunStatusPausing, generation); ok {
			s.emitPauseEvent(ctx, run, "task.paused")
			return run, nil
		}
	}

	run.Status = RunStatusRecoveryRequired
	_ = s.store.PutTaskRun(ctx, run)
	return nil, NewTaskError(ErrTaskRecoveryRequired, "host exited during pause with no valid checkpoint")
}

func (s *TaskRuntimeService) Resume(ctx context.Context, taskRunID string) (*TaskRun, error) {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, NewTaskError(ErrTaskNotFound, err.Error())
	}

	if run.Status == RunStatusRunning || run.Status == RunStatusStarting {
		return run, nil
	}
	if run.Status != RunStatusPaused {
		return nil, NewTaskError(ErrTaskNotPaused, "task not paused: "+string(run.Status))
	}

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, err.Error())
	}

	cp, _ := s.store.GetLatestCheckpoint(ctx, taskRunID)
	if cp != nil {
		if cp.DefinitionHash != def.DefinitionHash {
			return nil, NewTaskError(ErrTaskResumeIncompatible, "definition hash mismatch")
		}
		if cp.InputHash != run.InputHash {
			return nil, NewTaskError(ErrTaskResumeIncompatible, "input hash mismatch")
		}
		cpID := cp.CheckpointID
		run.CheckpointID = &cpID
	}

	expectedGeneration := run.Generation
	now := time.Now().UTC()
	run.Status = RunStatusResuming
	run.Generation++
	run.ResumedAt = &now
	run.PauseReason = nil
	run.PauseRequestedAt = nil
	run.PausedAt = nil
	run.RuntimeInstanceID = nil

	if ok, err := s.store.UpdateTaskRunCAS(ctx, run, RunStatusPaused, expectedGeneration); err != nil {
		return nil, fmt.Errorf("task_runtime: resume cas: %w", err)
	} else if !ok {
		return nil, NewTaskError(ErrTaskResumeStaleGeneration, "concurrent resume or state change")
	}

	if err := s.queue.Enqueue(ctx, run); err != nil {
		return nil, fmt.Errorf("task_runtime: enqueue resume: %w", err)
	}

	s.emitPauseEvent(ctx, run, "task.resumed")

	go s.tryDispatch()
	return run, nil
}

func (s *TaskRuntimeService) removeActiveHost(taskRunID string) {
	s.mu.Lock()
	delete(s.activeHosts, taskRunID)
	s.mu.Unlock()
}

func (s *TaskRuntimeService) emitPauseEvent(ctx context.Context, run *TaskRun, eventType string) {
	if s.eventEmitter == nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"taskRunId":    run.TaskRunID,
		"taskDefinitionId": run.TaskDefinitionID,
		"extensionId":  run.ExtensionID,
		"moduleId":     run.ModuleID,
		"status":       string(run.Status),
		"generation":   run.Generation,
		"checkpointId": run.CheckpointID,
		"pauseReason":  run.PauseReason,
	})
	opts := event.PublishOptions{
		AggregateType: "task_run",
		AggregateID:   run.TaskRunID,
	}
	switch eventType {
	case "task.paused":
		_, _ = s.eventEmitter.EmitTaskPaused(ctx, payload, opts)
	case "task.resumed":
		_, _ = s.eventEmitter.EmitTaskResumed(ctx, payload, opts)
	}
}

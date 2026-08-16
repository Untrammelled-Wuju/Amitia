package task_runtime

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

func (s *TaskRuntimeService) HandleExternalFinish(ctx context.Context, taskRunID string, status string, result json.RawMessage, artifactID, errCode, errMsg string) error {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if current.Status.IsTerminal() {
		return nil
	}
	next := cloneTaskRun(current)
	now := time.Now().UTC()
	next.FinishedAt = &now

	var eventType TaskDomainEventType
	var runResult *TaskRunResult
	switch status {
	case "succeeded":
		next.Status = RunStatusSucceeded
		eventType = TaskEventSucceeded
		resultType := ResultInlineJSON
		if artifactID != "" || len(result) > s.config.MaxInlineResultBytes {
			resultType = ResultArtifact
		}
		runResult = &TaskRunResult{
			TaskRunID:  next.TaskRunID,
			ResultType: resultType,
			ResultJSON: result,
			ArtifactID: artifactID,
			ResultHash: hashBytes(result),
			CreatedAt:  now,
		}
		if artifactID != "" {
			next.ResultArtifactID = &artifactID
		}
	case "failed":
		next.Status = RunStatusFailed
		eventType = TaskEventFailed
		if errCode != "" {
			next.ErrorCode = &errCode
		}
		if errMsg != "" {
			next.ErrorMessage = &errMsg
		}
	case "cancelled":
		next.Status = RunStatusCancelled
		eventType = TaskEventCancelled
		if errMsg != "" {
			next.ErrorMessage = &errMsg
		}
	default:
		return errors.New("unknown finish status: " + status)
	}

	next.Revision = NextRevision(current.Revision)
	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if runResult != nil {
			if err := s.store.PutResult(txCtx, runResult); err != nil {
				return err
			}
		}
		if err := s.store.PutTaskRun(txCtx, next); err != nil {
			return err
		}
		return s.store.RemoveFromQueue(txCtx, next.TaskRunID)
	}); err != nil {
		return err
	}
	return s.publishTaskEvent(ctx, eventType, next, "", "")
}

func (s *TaskRuntimeService) HandleExternalProgress(ctx context.Context, taskRunID string, completedUnits, totalUnits int64, phase string) error {
	s.progressMu.Lock()
	last, ok := s.progressLast[taskRunID]
	now := time.Now()
	if ok && time.Since(last) < time.Second/time.Duration(s.config.MaxProgressPerSecond) {
		s.progressMu.Unlock()
		return nil
	}
	s.progressLast[taskRunID] = now
	s.progressMu.Unlock()

	completedFloat := float64(completedUnits)
	totalFloat := float64(totalUnits)
	seq := now.UnixNano()
	prog := TaskRunProgress{
		TaskRunID: taskRunID,
		Sequence:  seq,
		Current:   &completedFloat,
		Total:     &totalFloat,
		Stage:     phase,
		Message:   phase,
		UpdatedAt: now.UTC(),
	}
	progJSON, _ := json.Marshal(prog)
	return s.store.PutProgress(ctx, taskRunID, seq, progJSON)
}

func (s *TaskRuntimeService) HandleExternalCheckpoint(ctx context.Context, taskRunID string, lastUnit int64, phase string, payload json.RawMessage) error {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if current.Status.IsTerminal() {
		return nil
	}
	if len(payload) > s.config.MaxCheckpointBytes {
		return errors.New("checkpoint payload exceeds maximum size")
	}

	actualHash := hashBytes(payload)
	cp := &TaskCheckpoint{
		CheckpointID: "cp-" + uuid.NewString(),
		TaskRunID:    current.TaskRunID,
		Version:      NextRevision(current.Revision),
		Payload:      payload,
		PayloadHash:  actualHash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.store.PutCheckpoint(ctx, cp); err != nil {
		return err
	}

	cpID := cp.CheckpointID
	current.CheckpointID = &cpID
	if err := s.store.PutTaskRun(ctx, current); err != nil {
		return err
	}
	return nil
}

func (s *TaskRuntimeService) ClearLatestCheckpoint(ctx context.Context, taskRunID string) error {
	cp, err := s.store.GetLatestCheckpoint(ctx, taskRunID)
	if err != nil {
		return err
	}
	if cp == nil {
		return nil
	}
	return s.store.RemoveCheckpoint(ctx, cp.CheckpointID)
}

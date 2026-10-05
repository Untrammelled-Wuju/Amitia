package task_runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *TaskRuntimeService) HandleRemoteProgress(ctx context.Context, taskRunID, attemptID string, seq int64, current, total, percentage *float64, stage, message string) error {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return NewTaskError(ErrTaskNotFound, "task not found")
	}
	guarded, finish, owned, err := s.callbackAuthority(ctx, run, attemptID, run.Generation)
	if err != nil {
		return err
	}
	defer finish()
	if owned {
		return s.handleProgress(guarded, taskRunID, seq, current, total, percentage, stage, message, run)
	}
	if run.ExecutionAttemptID.String() != attemptID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "attempt ID mismatch")
	}

	s.progressMu.Lock()
	last, ok := s.progressLast[taskRunID]
	if ok && time.Since(last) < time.Second/time.Duration(s.config.MaxProgressPerSecond) {
		s.progressMu.Unlock()
		return nil
	}
	s.progressLast[taskRunID] = time.Now()
	s.progressMu.Unlock()

	prog := TaskRunProgress{
		TaskRunID:  taskRunID,
		Sequence:   seq,
		Current:    current,
		Total:      total,
		Percentage: percentage,
		Stage:      stage,
		Message:    message,
		UpdatedAt:  time.Now().UTC(),
	}
	progJSON, err := json.Marshal(prog)
	if err != nil {
		return err
	}
	return s.store.PutProgress(ctx, taskRunID, seq, progJSON)
}

func (s *TaskRuntimeService) HandleRemoteCheckpoint(ctx context.Context, taskRunID, attemptID, checkpointID string, version int64, payload json.RawMessage, payloadHash string) error {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return NewTaskError(ErrTaskNotFound, "task not found")
	}
	guarded, finish, owned, err := s.callbackAuthority(ctx, run, attemptID, run.Generation)
	if err != nil {
		return err
	}
	defer finish()
	if owned {
		def, err := s.store.GetTaskDefinition(guarded, run.TaskDefinitionID)
		if err != nil {
			return err
		}
		return s.handleCheckpoint(guarded, run, def, payload, payloadHash, version)
	}
	if run.ExecutionAttemptID.String() != attemptID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "attempt ID mismatch")
	}

	if len(payload) > s.config.MaxCheckpointBytes {
		return NewTaskError(ErrTaskCheckpointTooLarge, "checkpoint payload exceeds maximum size")
	}

	actualHash := hashBytes(payload)
	if payloadHash != "" && payloadHash != actualHash {
		return NewTaskError(ErrTaskCheckpointHashMismatch, "checkpoint payload hash mismatch")
	}

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return err
	}

	cp := &TaskCheckpoint{
		CheckpointID:   checkpointID,
		TaskRunID:      taskRunID,
		Version:        version,
		Payload:        payload,
		PayloadHash:    actualHash,
		DefinitionHash: def.DefinitionHash,
		InputHash:      run.InputHash,
		CreatedAt:      time.Now().UTC(),
	}

	return s.store.PutCheckpoint(ctx, cp)
}

func (s *TaskRuntimeService) UpdateLease(ctx context.Context, taskRunID, leaseID string, extension time.Duration) error {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if current == nil {
		return NewTaskError(ErrTaskNotFound, "task not found")
	}
	guarded, finish, _, err := s.callbackAuthority(ctx, current, current.ExecutionAttemptID.String(), current.Generation)
	if err != nil {
		return err
	}
	defer finish()
	ctx = guarded
	if current.Status != RunStatusRunning || extension <= 0 || extension > 5*time.Minute {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务状态或续租时长无效")
	}
	if current.LeaseID != leaseID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "lease ID mismatch")
	}

	next := cloneTaskRun(current)
	now := time.Now().UTC()
	next.LeaseExpiresAt = ptrTime(now.Add(extension))
	next.LastHeartbeatAt = ptrTime(now)
	next.Revision = NextRevision(current.Revision)

	return s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		if casErr != nil {
			return casErr
		}
		if !ok {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "concurrent lease update")
		}
		return nil
	})
}

func (s *TaskRuntimeService) ReclaimExpiredLeases(ctx context.Context) (int, error) {
	runs, err := s.store.ListTaskRunsByStatus(ctx, string(RunStatusRunning))
	if err != nil {
		return 0, err
	}

	reclaimed := 0
	now := time.Now().UTC()
	for _, run := range runs {
		if run.LeaseExpiresAt != nil && run.LeaseExpiresAt.Before(now) && run.LeaseID != "" {
			if err := s.recoverStaleTask(ctx, run); err != nil {
				continue
			}
			reclaimed++
		}
	}
	return reclaimed, nil
}

func (s *TaskRuntimeService) recoverStaleTask(ctx context.Context, run *TaskRun) error {
	if run.ScopeSnapshotID != "" {
		return s.markTaskRecoveryUnknown(ctx, run)
	}
	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return err
	}

	idempotency := def.Idempotency
	if idempotency == "" {
		if def.Idempotent {
			idempotency = Idempotent
		} else {
			idempotency = NonIdempotent
		}
	}

	next := cloneTaskRun(run)
	now := time.Now().UTC()
	next.FinishedAt = &now
	errMsg := fmt.Sprintf("lease expired at %s", run.LeaseExpiresAt.Format(time.RFC3339))
	next.ErrorMessage = &errMsg

	if idempotency == Idempotent && run.Attempt < run.MaxAttempts {
		next.Status = RunStatusRecoveryRequired
		next.ErrorCode = strPtr("lease_expired")
	} else {
		next.Status = RunStatusFailed
		next.ErrorCode = strPtr("lease_expired")
	}
	next.Revision = NextRevision(run.Revision)

	return s.mutateTaskRun(ctx, taskMutationParams{
		next:       next,
		expected:   run.Status,
		generation: run.Generation,
		revision:   run.Revision,
		removeQ:    true,
		eventType:  TaskEventFailed,
		eventMsg:   errMsg,
		eventCode:  "lease_expired",
	})
}

func (s *TaskRuntimeService) ValidateRemoteCompletion(ctx context.Context, taskRunID, attemptID, leaseID string) error {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return NewTaskError(ErrTaskNotFound, "task not found")
	}

	if run.ExecutionAttemptID.String() != attemptID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "attempt ID mismatch, late result rejected")
	}

	if run.LeaseID != "" && run.LeaseID != leaseID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "lease ID mismatch, result rejected")
	}

	if run.Generation > 0 && run.ExecutionTarget.ConnectionGeneration > 0 {
		if run.Status.IsTerminal() {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "task already terminal, late result rejected")
		}
	}

	return nil
}

func (s *TaskRuntimeService) ApplyRemoteCompletion(ctx context.Context, taskRunID, attemptID, leaseID string, success bool, result json.RawMessage, errMsg string, artifactIDs ...string) error {
	artifactID := ""
	if len(artifactIDs) > 1 || len(artifactIDs) == 1 && (!success || len(result) != 0) {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务产物结果与终态不一致")
	}
	if len(artifactIDs) == 1 {
		artifactID = artifactIDs[0]
	}
	completed, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil || completed == nil {
		return NewTaskError(ErrTaskNotFound, "任务结果来源不存在")
	}
	if completed.Status.IsTerminal() {
		guarded, finish, _, err := s.callbackAuthority(ctx, completed, attemptID, completed.Generation)
		if err != nil {
			return err
		}
		defer finish()
		if completed.ExecutionAttemptID.String() != attemptID || completed.LeaseID != "" && completed.LeaseID != leaseID {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "重复结果的执行身份不一致")
		}
		if success {
			metadata, err := s.store.GetResult(guarded, taskRunID)
			matches := metadata != nil && (artifactID == "" && metadata.ResultHash == hashBytes(result) || artifactID != "" && metadata.ArtifactID == artifactID && metadata.ResultType == ResultArtifact)
			if err != nil || completed.Status != RunStatusSucceeded || !matches {
				return NewTaskError(ErrTaskExecutionAttemptInvalid, "重复结果与已确认结果不一致")
			}
			if artifactID != "" {
				if s.config.OwnedArtifacts == nil {
					return NewTaskError(ErrTaskScopeDenied, "重复任务产物缺少所有者复核端口")
				}
				_, hash, err := s.config.OwnedArtifacts.Result(guarded, completed, artifactID)
				if err != nil || hash != metadata.ResultHash {
					return NewTaskError(ErrTaskExecutionAttemptInvalid, "重复结果的所有者产物已变化或失效")
				}
			}
		} else if completed.Status != RunStatusFailed && completed.Status != RunStatusCancelled || len(result) != 0 {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "重复终态与已确认状态不一致")
		}
		return coordination.ValidateCurrent(guarded)
	}
	if err := s.ValidateRemoteCompletion(ctx, taskRunID, attemptID, leaseID); err != nil {
		return err
	}

	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	guarded, finish, owned, err := s.callbackAuthority(ctx, current, attemptID, current.Generation)
	if err != nil {
		return err
	}
	defer finish()
	if owned {
		status, code := "succeeded", ""
		if !success {
			status, code = "failed", "remote_completed"
		}
		return s.handleFinished(guarded, current, status, result, artifactID, code, errMsg)
	}
	if artifactID != "" {
		return NewTaskError(ErrTaskScopeDenied, "普通远端任务不能提交所有者产物引用")
	}

	next := cloneTaskRun(current)
	now := time.Now().UTC()
	next.FinishedAt = &now

	var eventType TaskDomainEventType
	var runResult *TaskRunResult
	if success {
		next.Status = RunStatusSucceeded
		eventType = TaskEventSucceeded
		resultType := ResultInlineJSON
		if len(result) > s.config.MaxInlineResultBytes {
			resultType = ResultArtifact
		}
		runResult = &TaskRunResult{
			TaskRunID:  next.TaskRunID,
			ResultType: resultType,
			ResultJSON: result,
			ResultHash: hashBytes(result),
			CreatedAt:  now,
		}
		next.ErrorCode = nil
		next.ErrorMessage = nil
	} else if current.Status == RunStatusCancelling {
		// A completion emitted after a remote cancel is the worker's cancel ACK,
		// not a task failure. Preserve cancellation semantics end-to-end.
		next.Status = RunStatusCancelled
		eventType = TaskEventCancelled
		next.ErrorCode = nil
		if errMsg != "" {
			next.ErrorMessage = &errMsg
		}
	} else {
		next.Status = RunStatusFailed
		eventType = TaskEventFailed
		next.ErrorCode = strPtr("remote_completed")
		if errMsg != "" {
			next.ErrorMessage = &errMsg
		}
	}
	next.Revision = NextRevision(current.Revision)

	return s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if runResult != nil {
			if err := s.store.PutResult(txCtx, runResult); err != nil {
				return err
			}
		}
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		if casErr != nil {
			return casErr
		}
		if !ok {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "concurrent remote completion")
		}
		if err := s.store.RemoveFromQueue(txCtx, next.TaskRunID); err != nil {
			return err
		}
		return s.publishTaskEvent(txCtx, eventType, next, "", "remote_completed")
	})
}

func (s *TaskRuntimeService) HandleRemoteClaim(ctx context.Context, taskRunID, attemptID, leaseID string, leaseExpiresAt time.Time) error {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if current == nil {
		return NewTaskError(ErrTaskNotFound, "task not found")
	}
	guarded, finish, _, err := s.callbackAuthority(ctx, current, attemptID, current.Generation)
	if err != nil {
		return err
	}
	defer finish()
	ctx = guarded
	if leaseID == "" || len(leaseID) > 256 || !leaseExpiresAt.After(time.Now()) || leaseExpiresAt.After(time.Now().Add(5*time.Minute+time.Second)) {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "远端任务租约无效")
	}
	if current.ExecutionAttemptID.String() != attemptID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "attempt ID mismatch")
	}
	if current.Status.IsTerminal() {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "task already terminal")
	}
	if current.Status == RunStatusRunning {
		if current.LeaseID != "" && current.LeaseID != leaseID {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "lease ID mismatch")
		}
		return nil
	}

	next := cloneTaskRun(current)
	now := time.Now().UTC()
	next.Status = RunStatusRunning
	if next.StartedAt == nil {
		next.StartedAt = &now
	}
	next.LeaseID = leaseID
	next.LeaseExpiresAt = &leaseExpiresAt
	next.LastHeartbeatAt = &now
	next.Revision = NextRevision(current.Revision)

	return s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		if casErr != nil {
			return casErr
		}
		if !ok {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "concurrent remote claim state change")
		}
		return s.publishTaskEvent(txCtx, TaskEventRunning, next, "", "remote_claimed")
	})
}

func (s *TaskRuntimeService) HeartbeatRemoteTask(ctx context.Context, taskRunID, attemptID, leaseID string, extendDuration time.Duration) error {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if current == nil {
		return NewTaskError(ErrTaskNotFound, "task not found")
	}
	guarded, finish, _, err := s.callbackAuthority(ctx, current, attemptID, current.Generation)
	if err != nil {
		return err
	}
	defer finish()
	ctx = guarded
	if extendDuration <= 0 || extendDuration > 5*time.Minute {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "远端任务续租时长无效")
	}
	if current.ExecutionAttemptID.String() != attemptID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "attempt ID mismatch")
	}
	if current.LeaseID != leaseID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "lease ID mismatch")
	}
	if current.Status != RunStatusRunning {
		return NewTaskError(ErrTaskStateTransitionInvalid, "task not running")
	}

	next := cloneTaskRun(current)
	now := time.Now().UTC()
	next.LeaseExpiresAt = ptrTime(now.Add(extendDuration))
	next.LastHeartbeatAt = ptrTime(now)
	next.Revision = NextRevision(current.Revision)

	return s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		if casErr != nil {
			return casErr
		}
		if !ok {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "concurrent remote heartbeat")
		}
		return nil
	})
}

func (s *TaskRuntimeService) HandleProgress(ctx context.Context, taskRunID, attemptID, leaseID string, seq int64, current, total, percentage *float64, stage, message string) error {
	if err := s.validateRemoteAttemptLease(ctx, taskRunID, attemptID, leaseID); err != nil {
		return err
	}
	return s.HandleRemoteProgress(ctx, taskRunID, attemptID, seq, current, total, percentage, stage, message)
}

func (s *TaskRuntimeService) HandleCheckpoint(ctx context.Context, taskRunID, attemptID, leaseID, checkpointID string, version int64, payload json.RawMessage, payloadHash string) error {
	if err := s.validateRemoteAttemptLease(ctx, taskRunID, attemptID, leaseID); err != nil {
		return err
	}
	return s.HandleRemoteCheckpoint(ctx, taskRunID, attemptID, checkpointID, version, payload, payloadHash)
}

func (s *TaskRuntimeService) HandleCompletion(ctx context.Context, taskRunID, attemptID, leaseID string, success bool, result json.RawMessage, errMsg string) error {
	return s.ApplyRemoteCompletion(ctx, taskRunID, attemptID, leaseID, success, result, errMsg)
}

func (s *TaskRuntimeService) HandleCompletionArtifact(ctx context.Context, taskRunID, attemptID, leaseID string, success bool, result json.RawMessage, errMsg string, artifactIDs ...string) error {
	return s.ApplyRemoteCompletion(ctx, taskRunID, attemptID, leaseID, success, result, errMsg, artifactIDs...)
}

func (s *TaskRuntimeService) validateRemoteAttemptLease(ctx context.Context, taskRunID, attemptID, leaseID string) error {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return err
	}
	if run == nil {
		return NewTaskError(ErrTaskNotFound, "task not found")
	}
	if run.ExecutionAttemptID.String() != attemptID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "attempt ID mismatch")
	}
	if run.LeaseID == "" || run.LeaseID != leaseID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "lease ID mismatch")
	}
	if run.Status.IsTerminal() {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "task already terminal")
	}
	return nil
}

func (s *TaskRuntimeService) remoteLeaseExpiryLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.dispatchCtx.Done():
			return
		case <-ticker.C:
			if _, err := s.ReclaimExpiredLeases(s.dispatchCtx); err != nil {
				log.Printf("task_runtime: reclaim expired leases failed: %v", err)
			}
		}
	}
}

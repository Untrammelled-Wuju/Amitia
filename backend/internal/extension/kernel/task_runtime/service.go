package task_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type TaskRuntimeService struct {
	store   TaskStore
	queue   *TaskQueue
	limiter *ConcurrencyLimiter
	config  TaskRuntimeConfig

	eventEmitter event.HostEventEmitter

	mu          sync.RWMutex
	activeHosts map[string]*TaskProcessHost

	dispatchCtx    context.Context
	dispatchCancel context.CancelFunc
	dispatchWg     sync.WaitGroup
	dispatching    int32

	progressSeq  map[string]*int64
	progressMu   sync.Mutex
	progressLast map[string]time.Time

	closed bool
}

func NewTaskRuntimeService(store TaskStore, config TaskRuntimeConfig) *TaskRuntimeService {
	queue := NewTaskQueue(store, "amitia-task-runtime", config.LeaseDuration)
	limiter := NewConcurrencyLimiter(store, config)
	return &TaskRuntimeService{
		store:         store,
		queue:         queue,
		limiter:       limiter,
		config:        config,
		activeHosts:   make(map[string]*TaskProcessHost),
		progressSeq:   make(map[string]*int64),
		progressLast:  make(map[string]time.Time),
	}
}

func (s *TaskRuntimeService) SetEventEmitter(emitter event.HostEventEmitter) {
	s.eventEmitter = emitter
}

func (s *TaskRuntimeService) GetTaskDefinition(ctx context.Context, defID string) (*TaskDefinition, error) {
	return s.store.GetTaskDefinition(ctx, defID)
}

func (s *TaskRuntimeService) PutTaskDefinition(ctx context.Context, def *TaskDefinition) error {
	return s.store.PutTaskDefinition(ctx, def)
}

func (s *TaskRuntimeService) ListTaskDefinitions(ctx context.Context, extensionID string) ([]*TaskDefinition, error) {
	return s.store.ListTaskDefinitions(ctx, extensionID)
}

func (s *TaskRuntimeService) Start(ctx context.Context) {
	s.dispatchCtx, s.dispatchCancel = context.WithCancel(ctx)
	go s.dispatchLoop()
	go s.leaseReclaimLoop()
}

func (s *TaskRuntimeService) Shutdown(ctx context.Context) {
	s.mu.Lock()
	s.closed = true
	hosts := make([]*TaskProcessHost, 0, len(s.activeHosts))
	for _, h := range s.activeHosts {
		hosts = append(hosts, h)
	}
	s.mu.Unlock()

	for _, h := range hosts {
		_ = h.Cancel(ctx, "application_shutdown")
	}

	if s.dispatchCancel != nil {
		s.dispatchCancel()
	}
	s.dispatchWg.Wait()

	done := make(chan struct{})
	go func() {
		for _, h := range hosts {
			<-h.Done()
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		for _, h := range hosts {
			h.ForceStop()
		}
	}
}

func (s *TaskRuntimeService) Enqueue(ctx context.Context, req EnqueueTaskRequest, def *TaskDefinition) (*EnqueueTaskResult, error) {
	inputHash := hashBytes(req.Input)
	runID := "tr-" + uuid.NewString()
	now := time.Now().UTC()

	deadline := now.Add(s.config.DefaultTimeout)
	if def.TimeoutPolicy.DefaultTimeout > 0 {
		deadline = now.Add(def.TimeoutPolicy.DefaultTimeout)
	}

	maxAttempts := def.RetryPolicy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	run := &TaskRun{
		TaskRunID:             runID,
		OperationID:           req.OperationID,
		TaskDefinitionID:      def.TaskID,
		ExtensionID:           req.ExtensionID,
		ModuleID:              req.ModuleID,
		Status:                RunStatusQueued,
		Priority:              req.Priority,
		Input:                 req.Input,
		InputHash:             inputHash,
		ScopeSnapshotID:       req.ScopeSnapshotID,
		PermissionSnapshotID:  req.PermissionSnapshotID,
		Attempt:               1,
		MaxAttempts:           maxAttempts,
		CreatedAt:             now,
		QueuedAt:              &now,
		DeadlineAt:            &deadline,
		Generation:            1,
	}

	if err := s.store.PutTaskRun(ctx, run); err != nil {
		return nil, fmt.Errorf("task_runtime: persist run: %w", err)
	}

	if err := s.queue.Enqueue(ctx, run); err != nil {
		return nil, fmt.Errorf("task_runtime: enqueue: %w", err)
	}

	result := &EnqueueTaskResult{
		TaskRunID: runID,
		Status:    RunStatusQueued,
		Queued:    true,
	}

	go s.tryDispatch()

	return result, nil
}

func (s *TaskRuntimeService) tryDispatch() {
	if !atomic.CompareAndSwapInt32(&s.dispatching, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&s.dispatching, 0)
	ctx := s.dispatchCtx
	if ctx == nil {
		ctx = context.Background()
	}
	s.dispatchOnce(ctx)
}

func (s *TaskRuntimeService) dispatchLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.dispatchCtx.Done():
			return
		case <-ticker.C:
			s.dispatchOnce(s.dispatchCtx)
		}
	}
}

func (s *TaskRuntimeService) dispatchOnce(ctx context.Context) {
	for {
		entry, err := s.queue.Dequeue(ctx)
		if err != nil {
			return
		}
		if entry == nil {
			return
		}

		run, err := s.store.GetTaskRun(ctx, entry.TaskRunID)
		if err != nil || run.Status.IsTerminal() {
			_ = s.queue.Remove(ctx, entry.TaskRunID)
			continue
		}

		canStart, reason, err := s.limiter.CanStart(ctx, run)
		if err != nil || !canStart {
			_ = s.queue.ReenqueueWithDelay(ctx, run, 5*time.Second)
			_ = reason
			return
		}

		s.dispatchWg.Add(1)
		go func(r *TaskRun) {
			defer s.dispatchWg.Done()
			s.executeTaskRun(ctx, r)
		}(run)
	}
}

func (s *TaskRuntimeService) executeTaskRun(ctx context.Context, run *TaskRun) {
	defer s.tryDispatch()

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		s.failRun(ctx, run, ErrTaskDefinitionInvalid, fmt.Sprintf("definition not found: %v", err))
		return
	}

	workspace, err := createTaskWorkspace(s.config.WorkspaceRoot, run.TaskRunID)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("workspace: %v", err))
		return
	}

	now := time.Now().UTC()
	run.Status = RunStatusStarting
	run.StartedAt = &now
	run.RuntimeInstanceID = strPtr("ri-" + uuid.NewString())
	if err := s.store.PutTaskRun(ctx, run); err != nil {
		return
	}

	var checkpointPayload json.RawMessage
	if run.CheckpointID != nil && *run.CheckpointID != "" {
		cp, err := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
		if err == nil && cp != nil {
			checkpointPayload = cp.Payload
			run.Status = RunStatusResuming
			s.store.PutTaskRun(ctx, run)
		}
	}

	run.Status = RunStatusRunning
	s.store.PutTaskRun(ctx, run)

	instanceID := "ri-" + uuid.NewString()
	hostCfg := ProcessHostConfig{
		InstanceID:  instanceID,
		TaskRunID:   run.TaskRunID,
		ExtensionID: run.ExtensionID,
		ModuleID:    run.ModuleID,
		DefHash:     def.DefinitionHash,
		NodePath:    s.config.NodePath,
		HostPath:    s.config.TaskHostPath,
		WorkDir:     workspace,
		EntryPath:   def.Entry,
	}

	host, err := NewTaskProcessHost(hostCfg)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, err.Error())
		s.cleanupWorkspace(run.TaskRunID, workspace)
		return
	}

	s.mu.Lock()
	s.activeHosts[run.TaskRunID] = host
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.activeHosts, run.TaskRunID)
		s.mu.Unlock()
		s.cleanupWorkspace(run.TaskRunID, workspace)
	}()

	callbacks := ProcessCallbacks{
		OnProgress: func(seq int64, current, total, percentage *float64, stage, message string) {
			s.handleProgress(ctx, run.TaskRunID, seq, current, total, percentage, stage, message)
		},
		OnCheckpoint: func(version int64, payload json.RawMessage, hash string) {
			s.handleCheckpoint(ctx, run, def, payload, hash, version)
		},
		OnLog: func(level, message string, fields map[string]interface{}) {
		},
		OnFinished: func(status string, result json.RawMessage, artifactID string, errCode, errMsg string) {
			s.handleFinished(ctx, run, status, result, artifactID, errCode, errMsg)
		},
	}

	taskCtx, taskCancel := context.WithCancel(ctx)
	defer taskCancel()

	if run.DeadlineAt != nil {
		taskCtx, taskCancel = context.WithDeadline(ctx, *run.DeadlineAt)
		defer taskCancel()
	}

	if err := host.Start(taskCtx, run.Input, checkpointPayload, run.DeadlineAt, run.Attempt, run.MaxAttempts, callbacks); err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, err.Error())
		return
	}

	exitCode, _ := host.Wait()

	if run.Status.IsTerminal() {
		return
	}

	if exitCode != 0 && !run.Status.IsTerminal() {
		s.handleCrash(ctx, run, def, exitCode)
	}
}

func (s *TaskRuntimeService) handleProgress(ctx context.Context, taskRunID string, seq int64, current, total, percentage *float64, stage, message string) {
	s.progressMu.Lock()
	last, ok := s.progressLast[taskRunID]
	if ok && time.Since(last) < time.Second/time.Duration(s.config.MaxProgressPerSecond) {
		s.progressMu.Unlock()
		return
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
	progJSON, _ := json.Marshal(prog)
	_ = s.store.PutProgress(ctx, taskRunID, seq, progJSON)
}

func (s *TaskRuntimeService) handleCheckpoint(ctx context.Context, run *TaskRun, def *TaskDefinition, payload json.RawMessage, hash string, version int64) {
	if len(payload) > s.config.MaxCheckpointBytes {
		return
	}

	actualHash := hashBytes(payload)
	if hash != "" && hash != actualHash {
		return
	}

	cp := &TaskCheckpoint{
		CheckpointID:   "cp-" + uuid.NewString(),
		TaskRunID:      run.TaskRunID,
		Version:        version,
		Payload:        payload,
		PayloadHash:    actualHash,
		DefinitionHash: def.DefinitionHash,
		InputHash:      run.InputHash,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.PutCheckpoint(ctx, cp); err != nil {
		return
	}

	cpID := cp.CheckpointID
	run.CheckpointID = &cpID
	run.Status = RunStatusCheckpointing
	_ = s.store.PutTaskRun(ctx, run)
	run.Status = RunStatusRunning
	_ = s.store.PutTaskRun(ctx, run)
}

func (s *TaskRuntimeService) handleFinished(ctx context.Context, run *TaskRun, status string, result json.RawMessage, artifactID string, errCode, errMsg string) {
	now := time.Now().UTC()
	run.FinishedAt = &now

	switch status {
	case "succeeded":
		run.Status = RunStatusSucceeded
		resultType := ResultInlineJSON
		if artifactID != "" || len(result) > s.config.MaxInlineResultBytes {
			resultType = ResultArtifact
		}
		runResult := &TaskRunResult{
			TaskRunID:  run.TaskRunID,
			ResultType: resultType,
			ResultJSON: result,
			ArtifactID: artifactID,
			ResultHash: hashBytes(result),
			CreatedAt:  now,
		}
		_ = s.store.PutResult(ctx, runResult)
		if artifactID != "" {
			run.ResultArtifactID = &artifactID
		}
	case "failed":
		run.Status = RunStatusFailed
		if errCode != "" {
			run.ErrorCode = &errCode
		}
		if errMsg != "" {
			run.ErrorMessage = &errMsg
		}
	case "cancelled":
		run.Status = RunStatusCancelled
		if errMsg != "" {
			run.ErrorMessage = &errMsg
		}
	}

	_ = s.store.PutTaskRun(ctx, run)
	_ = s.queue.Remove(ctx, run.TaskRunID)

	if s.eventEmitter != nil {
		taskPayload, _ := json.Marshal(map[string]interface{}{
			"taskRunId":        run.TaskRunID,
			"taskDefinitionId": run.TaskDefinitionID,
			"extensionId":      run.ExtensionID,
			"moduleId":         run.ModuleID,
			"status":           string(run.Status),
			"operationId":      run.OperationID,
			"attempt":          run.Attempt,
			"finishedAt":       now.Format(time.RFC3339),
		})
		taskOpts := event.PublishOptions{
			AggregateType: "task_run",
			AggregateID:   run.TaskRunID,
			OperationID:   run.OperationID,
		}
		_, _ = s.eventEmitter.EmitTaskCompleted(ctx, taskPayload, taskOpts)
	}
}

func (s *TaskRuntimeService) handleCrash(ctx context.Context, run *TaskRun, def *TaskDefinition, exitCode int) {
	now := time.Now().UTC()
	run.FinishedAt = &now
	errMsg := fmt.Sprintf("task process crashed with exit code %d", exitCode)
	run.ErrorMessage = &errMsg

	recoverability := def.Recoverability
	if recoverability == "" {
		if def.Recoverable {
			recoverability = CheckpointRecoverable
		} else {
			recoverability = NotRecoverable
		}
	}

	idempotency := def.Idempotency
	if idempotency == "" {
		if def.Idempotent {
			idempotency = Idempotent
		} else {
			idempotency = NonIdempotent
		}
	}

	switch recoverability {
	case CheckpointRecoverable:
		cp, _ := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
		if cp != nil {
			run.Status = RunStatusRecoveryRequired
		} else {
			run.Status = RunStatusFailed
		}
	case RestartableFromBeginning:
		if idempotency == Idempotent && run.Attempt < run.MaxAttempts {
			run.Status = RunStatusRecoveryRequired
		} else {
			run.Status = RunStatusFailed
		}
	case ManualRecovery:
		run.Status = RunStatusManualIntervention
	default:
		if idempotency == NonIdempotent {
			run.Status = RunStatusManualIntervention
		} else {
			run.Status = RunStatusFailed
		}
	}

	code := string(ErrTaskRuntimeCrashed)
	run.ErrorCode = &code
	_ = s.store.PutTaskRun(ctx, run)
	_ = s.queue.Remove(ctx, run.TaskRunID)
}

func (s *TaskRuntimeService) failRun(ctx context.Context, run *TaskRun, code TaskErrorCode, message string) {
	now := time.Now().UTC()
	run.Status = RunStatusFailed
	run.FinishedAt = &now
	c := string(code)
	run.ErrorCode = &c
	run.ErrorMessage = &message
	_ = s.store.PutTaskRun(ctx, run)
	_ = s.queue.Remove(ctx, run.TaskRunID)
}

func (s *TaskRuntimeService) Cancel(ctx context.Context, taskRunID, reason string) error {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return NewTaskError(ErrTaskNotFound, err.Error())
	}

	if run.Status.IsTerminal() {
		return NewTaskError(ErrTaskNotCancelable, "task already terminal: "+string(run.Status))
	}

	now := time.Now().UTC()
	run.CancelRequestedAt = &now
	run.Status = RunStatusCancelling
	_ = s.store.PutTaskRun(ctx, run)

	if run.Status == RunStatusQueued {
		_ = s.queue.Remove(ctx, taskRunID)
		now := time.Now().UTC()
		run.Status = RunStatusCancelled
		run.FinishedAt = &now
		_ = s.store.PutTaskRun(ctx, run)
		return nil
	}

	s.mu.RLock()
	host, ok := s.activeHosts[taskRunID]
	s.mu.RUnlock()

	if ok {
		_ = host.Cancel(ctx, reason)
	}

	return nil
}

func (s *TaskRuntimeService) Retry(ctx context.Context, taskRunID string) (*TaskRun, error) {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, NewTaskError(ErrTaskNotFound, err.Error())
	}

	if !run.Status.IsTerminal() {
		return nil, NewTaskError(ErrTaskNotRetryable, "task not terminal")
	}

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, err.Error())
	}

	idempotency := def.Idempotency
	if idempotency == "" {
		if def.Idempotent {
			idempotency = Idempotent
		} else {
			idempotency = NonIdempotent
		}
	}

	if idempotency == NonIdempotent {
		return nil, NewTaskError(ErrTaskNotRetryable, "non-idempotent task cannot be retried")
	}

	if run.Attempt >= run.MaxAttempts {
		return nil, NewTaskError(ErrTaskNotRetryable, "max attempts exceeded")
	}

	newRun := &TaskRun{
		TaskRunID:        "tr-" + uuid.NewString(),
		OperationID:      run.OperationID,
		TaskDefinitionID: run.TaskDefinitionID,
		ExtensionID:      run.ExtensionID,
		ModuleID:         run.ModuleID,
		Status:           RunStatusQueued,
		Priority:         run.Priority,
		Input:            run.Input,
		InputHash:        run.InputHash,
		Attempt:          run.Attempt + 1,
		MaxAttempts:      run.MaxAttempts,
		CreatedAt:        time.Now().UTC(),
		QueuedAt:         ptrTime(time.Now().UTC()),
		DeadlineAt:       run.DeadlineAt,
		Generation:       run.Generation + 1,
	}

	if err := s.store.PutTaskRun(ctx, newRun); err != nil {
		return nil, fmt.Errorf("task_runtime: persist retry run: %w", err)
	}
	if err := s.queue.Enqueue(ctx, newRun); err != nil {
		return nil, fmt.Errorf("task_runtime: enqueue retry: %w", err)
	}

	go s.tryDispatch()
	return newRun, nil
}

func (s *TaskRuntimeService) Recover(ctx context.Context, taskRunID string) (*TaskRun, error) {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, NewTaskError(ErrTaskNotFound, err.Error())
	}

	if run.Status != RunStatusRecoveryRequired && run.Status != RunStatusManualIntervention {
		return nil, NewTaskError(ErrTaskStateTransitionInvalid, "task not in recovery state")
	}

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, err.Error())
	}

	cp, _ := s.store.GetLatestCheckpoint(ctx, taskRunID)
	if cp != nil {
		if cp.DefinitionHash != def.DefinitionHash {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "definition hash mismatch")
		}
		if cp.InputHash != run.InputHash {
			return nil, NewTaskError(ErrTaskCheckpointIncompatible, "input hash mismatch")
		}
		cpID := cp.CheckpointID
		run.CheckpointID = &cpID
	}

	run.Status = RunStatusQueued
	now := time.Now().UTC()
	run.QueuedAt = &now
	run.Generation++
	_ = s.store.PutTaskRun(ctx, run)
	_ = s.queue.Enqueue(ctx, run)

	go s.tryDispatch()
	return run, nil
}

func (s *TaskRuntimeService) GetTaskRun(ctx context.Context, taskRunID string) (*TaskRun, error) {
	return s.store.GetTaskRun(ctx, taskRunID)
}

func (s *TaskRuntimeService) ListTaskRuns(ctx context.Context, filter ListTasksFilter) ([]*TaskRun, error) {
	return s.store.ListTaskRuns(ctx, filter)
}

func (s *TaskRuntimeService) GetProgress(ctx context.Context, taskRunID string) (*TaskRunProgress, error) {
	return s.store.GetProgress(ctx, taskRunID)
}

func (s *TaskRuntimeService) GetResult(ctx context.Context, taskRunID string) (*TaskRunResult, error) {
	return s.store.GetResult(ctx, taskRunID)
}

func (s *TaskRuntimeService) GetLatestCheckpoint(ctx context.Context, taskRunID string) (*TaskCheckpoint, error) {
	return s.store.GetLatestCheckpoint(ctx, taskRunID)
}

func (s *TaskRuntimeService) StartupRecovery(ctx context.Context) error {
	statuses := []string{
		string(RunStatusStarting), string(RunStatusRunning),
		string(RunStatusCheckpointing), string(RunStatusCancelling),
		string(RunStatusPausing), string(RunStatusPaused),
		string(RunStatusResuming),
	}

	for _, status := range statuses {
		runs, err := s.store.ListTaskRunsByStatus(ctx, status)
		if err != nil {
			return fmt.Errorf("task_runtime: recovery list %s: %w", status, err)
		}
		for _, run := range runs {
			s.recoverRun(ctx, run)
		}
	}

	_, _ = s.queue.ReclaimExpired(ctx)
	return nil
}

func (s *TaskRuntimeService) recoverRun(ctx context.Context, run *TaskRun) {
	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		run.Status = RunStatusManualIntervention
		msg := "definition not found during recovery"
		run.ErrorMessage = &msg
		_ = s.store.PutTaskRun(ctx, run)
		return
	}

	recoverability := def.Recoverability
	if recoverability == "" {
		if def.Recoverable {
			recoverability = CheckpointRecoverable
		} else {
			recoverability = NotRecoverable
		}
	}

	idempotency := def.Idempotency
	if idempotency == "" {
		if def.Idempotent {
			idempotency = Idempotent
		} else {
			idempotency = NonIdempotent
		}
	}

	switch recoverability {
	case CheckpointRecoverable:
		cp, _ := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
		if cp != nil && cp.DefinitionHash == def.DefinitionHash && cp.InputHash == run.InputHash {
			cpID := cp.CheckpointID
			run.CheckpointID = &cpID
			run.Status = RunStatusRecoveryRequired
		} else {
			run.Status = RunStatusManualIntervention
		}
	case RestartableFromBeginning:
		if idempotency == Idempotent {
			run.Status = RunStatusQueued
			now := time.Now().UTC()
			run.QueuedAt = &now
			_ = s.queue.Enqueue(ctx, run)
		} else {
			run.Status = RunStatusManualIntervention
		}
	case ManualRecovery:
		run.Status = RunStatusManualIntervention
	default:
		if idempotency == NonIdempotent {
			run.Status = RunStatusManualIntervention
		} else {
			run.Status = RunStatusFailed
			msg := "task not recoverable"
			run.ErrorMessage = &msg
		}
	}

	_ = s.store.PutTaskRun(ctx, run)
}

func (s *TaskRuntimeService) leaseReclaimLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.dispatchCtx.Done():
			return
		case <-ticker.C:
			s.queue.ReclaimExpired(s.dispatchCtx)
		}
	}
}

func (s *TaskRuntimeService) cleanupWorkspace(taskRunID, workspace string) {
	_ = os.RemoveAll(workspace)
	_ = filepath.Clean(workspace)
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func strPtr(s string) *string { return &s }

func ptrTime(t time.Time) *time.Time { return &t }

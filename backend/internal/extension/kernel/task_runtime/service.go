package task_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/u-ai/backend/internal/extension/kernel/script_host"
)

type TaskRuntimeService struct {
	store   TaskStore
	queue   *TaskQueue
	limiter *ConcurrencyLimiter
	config  TaskRuntimeConfig

	events TaskEventSink

	mu          sync.RWMutex
	activeHosts map[string]*TaskProcessHost

	localExecutor  TaskExecutorPort
	remoteExecutor RemoteTaskExecutor

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
	if config.NodeEnvironmentResolver == nil {
		config.NodeEnvironmentResolver = script_host.UnavailableNodeResolver()
	}
	if config.HostArtifactResolver == nil {
		config.HostArtifactResolver = script_host.UnavailableArtifactResolver()
	}
	queue := NewTaskQueue(store, "amitia-task-runtime", config.LeaseDuration)
	limiter := NewConcurrencyLimiter(store, config)
	svc := &TaskRuntimeService{
		store:        store,
		queue:        queue,
		limiter:      limiter,
		config:       config,
		activeHosts:  make(map[string]*TaskProcessHost),
		progressSeq:  make(map[string]*int64),
		progressLast: make(map[string]time.Time),
	}
	svc.localExecutor = NewLocalTaskExecutor(svc)
	svc.remoteExecutor = UnavailableRemoteTaskExecutor{}
	return svc
}

func (s *TaskRuntimeService) SetEventSink(sink TaskEventSink) {
	s.events = sink
}

func (s *TaskRuntimeService) SetRemoteExecutor(executor RemoteTaskExecutor) {
	s.remoteExecutor = executor
}

func (s *TaskRuntimeService) RemoteExecutor() RemoteTaskExecutor {
	return s.remoteExecutor
}

func (s *TaskRuntimeService) publishTaskEvent(ctx context.Context, eventType TaskDomainEventType, run *TaskRun, reason, errorCode string) error {
	if s.events == nil {
		return nil
	}
	event := TaskDomainEvent{
		Type:       eventType,
		Run:        *run,
		Reason:     reason,
		ErrorCode:  errorCode,
		OccurredAt: time.Now().UTC(),
	}
	if err := s.events.TaskEvent(ctx, event); err != nil {
		return fmt.Errorf("task_runtime: publish event %s: %w", eventType, err)
	}
	return nil
}

type taskMutationParams struct {
	current    *TaskRun
	next       *TaskRun
	expected   TaskRunStatus
	generation int64
	revision   int64
	removeQ    bool
	eventType  TaskDomainEventType
	eventMsg   string
	eventCode  string
}

func (s *TaskRuntimeService) mutateTaskRun(ctx context.Context, p taskMutationParams) error {
	return s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, p.next, p.expected, p.generation, p.revision)
		if casErr != nil {
			return casErr
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "concurrent state change")
		}
		if p.removeQ {
			if err := s.store.RemoveFromQueue(txCtx, p.next.TaskRunID); err != nil {
				return err
			}
		}
		if p.eventType != "" {
			return s.publishTaskEvent(txCtx, p.eventType, p.next, p.eventMsg, p.eventCode)
		}
		return nil
	})
}

func (s *TaskRuntimeService) GetTaskDefinition(ctx context.Context, defID string) (*TaskDefinition, error) {
	return s.store.GetTaskDefinition(ctx, defID)
}

func (s *TaskRuntimeService) PutTaskDefinition(ctx context.Context, def *TaskDefinition) error {
	return s.store.PutTaskDefinition(ctx, def)
}

func (s *TaskRuntimeService) DeleteTaskDefinition(ctx context.Context, defID string) error {
	return s.store.DeleteTaskDefinition(ctx, defID)
}

func (s *TaskRuntimeService) DeleteByExtension(ctx context.Context, extensionID string) error {
	return s.store.DeleteByExtension(ctx, extensionID)
}

func (s *TaskRuntimeService) ListTaskDefinitions(ctx context.Context, extensionID string) ([]*TaskDefinition, error) {
	return s.store.ListTaskDefinitions(ctx, extensionID)
}

func (s *TaskRuntimeService) Start(ctx context.Context) {
	s.dispatchCtx, s.dispatchCancel = context.WithCancel(ctx)
	go s.dispatchLoop()
	go s.leaseReclaimLoop()
	go s.remoteLeaseExpiryLoop()
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

	placement, err := ResolveRequestedPlacement(req.ExecutionPlacement, def.ExecutionPlacement)
	if err != nil {
		return nil, err
	}

	deadline := now.Add(s.config.DefaultTimeout)
	if def.TimeoutPolicy.DefaultTimeout > 0 {
		deadline = now.Add(def.TimeoutPolicy.DefaultTimeout)
	}

	maxAttempts := def.RetryPolicy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	run := &TaskRun{
		TaskRunID:            runID,
		OperationID:          req.OperationID,
		InvocationID:         req.InvocationID,
		TaskDefinitionID:     def.TaskID,
		ExtensionID:          def.ExtensionID,
		ModuleID:             def.ModuleID,
		Status:               RunStatusQueued,
		Priority:             req.Priority,
		ExecutionPlacement:   placement,
		Input:                req.Input,
		InputHash:            inputHash,
		TraceID:              req.TraceID,
		CorrelationID:        req.CorrelationID,
		CausationID:          req.CausationID,
		Source:               req.Source,
		ScopeSnapshotID:      req.ScopeSnapshotID,
		PermissionSnapshotID: req.PermissionSnapshotID,
		Attempt:              1,
		MaxAttempts:          maxAttempts,
		CreatedAt:            now,
		QueuedAt:             &now,
		DeadlineAt:           &deadline,
		Generation:           1,
		Revision:             1,
	}

	if err := s.store.WithinTaskTx(ctx, func(ctx context.Context) error {
		if err := s.store.PutTaskRun(ctx, run); err != nil {
			return fmt.Errorf("task_runtime: persist run: %w", err)
		}
		if err := s.queue.Enqueue(ctx, run); err != nil {
			return fmt.Errorf("task_runtime: enqueue: %w", err)
		}
		return s.publishTaskEvent(ctx, TaskEventQueued, run, "", "")
	}); err != nil {
		return nil, err
	}

	result := &EnqueueTaskResult{
		TaskRunID: runID,
		Status:    RunStatusQueued,
		Queued:    true,
	}

	go s.tryDispatch()

	return result, nil
}

func (s *TaskRuntimeService) BindExecutionTarget(
	ctx context.Context,
	taskRunID string,
	request TrustedExecutionTargetRequest,
) (*TaskRun, error) {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, NewTaskError(ErrTaskNotFound, err.Error())
	}

	if current.Status != RunStatusCreated && current.Status != RunStatusQueued && current.Status != RunStatusRecoveryRequired {
		return nil, NewTaskError(ErrTaskExecutionTargetConflict, "execution target can only be bound in created/queued/recovery_required state")
	}

	decision := TaskPlacementDecision{
		Placement: request.Placement,
		Target:    request.Target,
		Resolved:  true,
	}

	now := time.Now().UTC()
	existingRevision := current.Revision
	next := cloneTaskRun(current)
	if err := next.BindExecutionTarget(decision, request.ResolvedBy, now); err != nil {
		return nil, err
	}

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if err := s.store.UpdateExecutionTarget(txCtx, next.TaskRunID, next.ExecutionPlacement, next.ExecutionTarget, *next.ExecutionResolvedAt, next.ExecutionResolvedBy, NextRevision(existingRevision), existingRevision); err != nil {
			return fmt.Errorf("task_runtime: update execution target: %w", err)
		}
		next.Revision = NextRevision(existingRevision)
		if err := s.publishTaskEvent(txCtx, TaskEventExecutionTargetBound, next, "", ""); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return next, nil
}

func (s *TaskRuntimeService) ClearExecutionConnectionBinding(
	ctx context.Context,
	taskRunID string,
) error {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return NewTaskError(ErrTaskNotFound, err.Error())
	}

	if current.EffectiveExecutionPlacement() != TaskExecutionPlacementDevice {
		return NewTaskError(ErrTaskExecutionPlacementInvalid, "connection binding only applies to device placement")
	}

	if current.ExecutionTarget.RuntimeSessionID == "" && current.ExecutionTarget.ConnectionGeneration == 0 {
		return nil
	}

	existingRevision := current.Revision
	next := cloneTaskRun(current)
	next.ClearTransientConnectionBinding()

	return s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		now := time.Now().UTC()
		if err := s.store.UpdateExecutionConnectionBinding(txCtx, taskRunID, emptyRuntimeSessionID(""), 0, now, NextRevision(existingRevision), existingRevision); err != nil {
			return err
		}
		next.Revision = NextRevision(existingRevision)
		return s.publishTaskEvent(txCtx, TaskEventConnectionBindingChanged, next, "", "")
	})
}

type emptyRuntimeSessionID string

func (s emptyRuntimeSessionID) String() string { return string(s) }

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
			if removeErr := s.queue.Remove(ctx, entry.TaskRunID); removeErr != nil {
				return
			}
			continue
		}

		canStart, _, err := s.limiter.CanStart(ctx, run)
		if err != nil || !canStart {
			if reenqueueErr := s.queue.ReenqueueWithDelay(ctx, run, 5*time.Second); reenqueueErr != nil {
				return
			}
			return
		}

		s.dispatchWg.Add(1)
		go func(r *TaskRun) {
			defer s.dispatchWg.Done()
			s.executeTaskRun(ctx, r)
		}(run)
	}
}

func (s *TaskRuntimeService) executorFor(placement TaskExecutionPlacement) (TaskExecutorPort, error) {
	switch placement {
	case TaskExecutionPlacementLocal:
		return s.localExecutor, nil
	case TaskExecutionPlacementCloud, TaskExecutionPlacementDevice:
		if s.remoteExecutor != nil && s.remoteExecutor.SupportsPlacement(placement) {
			return s.remoteExecutor, nil
		}
		return nil, NewTaskError(ErrRemoteTaskExecutorUnavailable, "no remote executor available for placement: "+string(placement))
	}
	return nil, NewTaskError(ErrTaskExecutionPlacementInvalid, "unknown placement: "+string(placement))
}

func (s *TaskRuntimeService) persistExecutionAttempt(
	ctx context.Context,
	run *TaskRun,
	attemptID TaskExecutionAttemptID,
	runtimeInstanceID string,
) error {
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil {
		return err
	}
	existingRevision := current.Revision

	return s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		now := time.Now().UTC()
		if err := s.store.UpdateExecutionAttempt(txCtx, run.TaskRunID, attemptID, runtimeInstanceID, now, NextRevision(existingRevision), existingRevision); err != nil {
			return err
		}
		next := cloneTaskRun(current)
		next.ExecutionAttemptID = attemptID
		next.RuntimeInstanceID = strPtr(runtimeInstanceID)
		next.Revision = NextRevision(existingRevision)
		CopyCommittedTaskRun(run, next)
		return s.publishTaskEvent(txCtx, TaskEventAttemptStarted, next, "", "")
	})
}

func (s *TaskRuntimeService) executeTaskRun(ctx context.Context, run *TaskRun) {
	defer s.tryDispatch()

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		s.failRun(ctx, run, ErrTaskDefinitionInvalid, fmt.Sprintf("definition not found: %v", err))
		return
	}

	attemptID := NewTaskExecutionAttemptID()
	run.ExecutionAttemptID = attemptID

	if run.EffectiveExecutionPlacement() != TaskExecutionPlacementLocal {
		if !run.HasResolvedExecutionTarget() {
			s.failRun(ctx, run, ErrTaskExecutionTargetUnresolved, "remote task execution target is not resolved")
			return
		}

		if err := s.persistExecutionAttempt(ctx, run, attemptID, ""); err != nil {
			s.failRun(ctx, run, ErrTaskExecutionAttemptInvalid, fmt.Sprintf("persist attempt: %v", err))
			return
		}

		// Remote execution has the same lifecycle gate as local execution: queued
		// -> starting -> running. The device claim callback owns the transition to
		// running after it has persisted the authoritative lease.
		current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
		if err != nil {
			s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("reload remote run: %v", err))
			return
		}
		startingRun := cloneTaskRun(current)
		now := time.Now().UTC()
		startingRun.Status = RunStatusStarting
		startingRun.StartedAt = &now
		startingRun.Revision = NextRevision(current.Revision)
		if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
			ok, casErr := s.store.UpdateTaskRunCAS(txCtx, startingRun, current.Status, current.Generation, current.Revision)
			if casErr != nil {
				return casErr
			}
			if !ok {
				return NewTaskError(ErrTaskExecutionAttemptInvalid, "concurrent remote start state change")
			}
			return s.publishTaskEvent(txCtx, TaskEventStarting, startingRun, "", "remote_dispatch")
		}); err != nil {
			s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("persist remote starting: %v", err))
			return
		}
		CopyCommittedTaskRun(run, startingRun)

		executor, err := s.executorFor(run.EffectiveExecutionPlacement())
		if err != nil {
			s.failRun(ctx, run, ErrRemoteTaskExecutorUnavailable, err.Error())
			return
		}

		outcome := s.runRemoteExecution(ctx, run, def, executor)
		s.applyExecutionOutcome(ctx, run, def, outcome)
		return
	}

	if err := s.persistExecutionAttempt(ctx, run, attemptID, ""); err != nil {
		s.failRun(ctx, run, ErrTaskExecutionAttemptInvalid, fmt.Sprintf("persist attempt: %v", err))
		return
	}

	workspace, err := s.createTaskWorkspace(run.TaskRunID)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("workspace: %v", err))
		return
	}

	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("reload run: %v", err))
		return
	}
	existingRevision := current.Revision
	startingRun := cloneTaskRun(current)
	now := time.Now().UTC()
	startingRun.Status = RunStatusStarting
	startingRun.StartedAt = &now
	startingRun.RuntimeInstanceID = strPtr("ri-" + uuid.NewString())
	startingRun.Revision = NextRevision(existingRevision)

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, startingRun, RunStatusQueued, current.Generation, current.Revision)
		if casErr != nil {
			return fmt.Errorf("task_runtime: starting cas: %w", casErr)
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry start")
		}
		return s.publishTaskEvent(txCtx, TaskEventStarting, startingRun, "", "")
	}); err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("persist starting: %v", err))
		return
	}
	CopyCommittedTaskRun(run, startingRun)

	var checkpointPayload json.RawMessage
	if run.CheckpointID != nil && *run.CheckpointID != "" {
		cp, err := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
		if err == nil && cp != nil {
			checkpointPayload = cp.Payload
			resumingRun := cloneTaskRun(startingRun)
			resumingRun.Status = RunStatusResuming
			resumingRun.Revision = NextRevision(startingRun.Revision)
			if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
				ok, casErr := s.store.UpdateTaskRunCAS(txCtx, resumingRun, RunStatusStarting, startingRun.Generation, startingRun.Revision)
				if casErr != nil {
					return fmt.Errorf("task_runtime: resuming cas: %w", casErr)
				}
				if !ok {
					return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry resume")
				}
				return s.publishTaskEvent(txCtx, TaskEventResuming, resumingRun, "", "")
			}); err != nil {
				s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("persist resuming: %v", err))
				return
			}
			CopyCommittedTaskRun(run, resumingRun)
		}
	}

	runningRun := cloneTaskRun(run)
	runningRun.Status = RunStatusRunning
	runningRun.Revision = NextRevision(run.Revision)
	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, runningRun, RunStatusResuming, run.Generation, run.Revision)
		if casErr != nil {
			return fmt.Errorf("task_runtime: running cas: %w", casErr)
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry running")
		}
		return s.publishTaskEvent(txCtx, TaskEventRunning, runningRun, "", "")
	}); err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("persist running: %v", err))
		return
	}
	run.Status = RunStatusRunning

	instanceID := "ri-" + uuid.NewString()

	nodeEnv, err := s.config.NodeEnvironmentResolver.Resolve(ctx)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, "node unavailable: "+err.Error())
		s.cleanupWorkspace(run.TaskRunID, workspace)
		return
	}

	hostArtifact, err := s.config.HostArtifactResolver.Resolve(ctx, script_host.KindTaskHost)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, "task host unavailable: "+err.Error())
		s.cleanupWorkspace(run.TaskRunID, workspace)
		return
	}

	hostCfg := ProcessHostConfig{
		InstanceID:  instanceID,
		TaskRunID:   run.TaskRunID,
		ExtensionID: run.ExtensionID,
		ModuleID:    run.ModuleID,
		DefHash:     def.DefinitionHash,
		NodePath:    nodeEnv.NodeBinary,
		HostPath:    hostArtifact.EntryPath,
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
		finalRun, _ := s.store.GetTaskRun(ctx, run.TaskRunID)
		if finalRun != nil && (finalRun.Status == RunStatusPaused || finalRun.Status == RunStatusPausing) {
			return
		}
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

func (s *TaskRuntimeService) runRemoteExecution(ctx context.Context, run *TaskRun, def *TaskDefinition, executor TaskExecutorPort) TaskExecutionOutcome {
	request := TaskExecutionRequest{
		Run:        run,
		Definition: def,
		AttemptID:  run.ExecutionAttemptID,
		Placement:  run.EffectiveExecutionPlacement(),
		Target:     run.ExecutionTarget,
	}
	outcome, _ := executor.Execute(ctx, request)
	return outcome
}

func (s *TaskRuntimeService) applyExecutionOutcome(ctx context.Context, run *TaskRun, def *TaskDefinition, outcome TaskExecutionOutcome) {
	now := time.Now().UTC()
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil {
		return
	}

	// A device result may race the executor returning its non-terminal claim
	// outcome. Never let a late Running outcome overwrite a terminal result.
	if current.Status.IsTerminal() && !outcome.Status.IsTerminal() {
		CopyCommittedTaskRun(run, current)
		return
	}
	if outcome.Status == RunStatusRunning && current.Status == RunStatusRunning {
		CopyCommittedTaskRun(run, current)
		return
	}

	next := cloneTaskRun(current)
	next.Status = outcome.Status
	if outcome.Status.IsTerminal() {
		next.FinishedAt = &now
	}
	next.Revision = NextRevision(current.Revision)
	if outcome.ErrorCode != "" {
		ec := outcome.ErrorCode
		next.ErrorCode = &ec
	}
	if outcome.ErrorMessage != "" {
		em := outcome.ErrorMessage
		next.ErrorMessage = &em
	}
	if outcome.LeaseID != "" {
		next.LeaseID = outcome.LeaseID
	}
	if outcome.LeaseExpiresAt != nil {
		le := *outcome.LeaseExpiresAt
		next.LeaseExpiresAt = &le
	}

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if outcome.Result != nil && outcome.Status.IsTerminal() {
			if err := s.store.PutResult(txCtx, outcome.Result); err != nil {
				return fmt.Errorf("task_runtime: put result: %w", err)
			}
		}
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		if casErr != nil {
			return fmt.Errorf("task_runtime: apply outcome cas: %w", casErr)
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry outcome")
		}
		if next.Status.IsTerminal() {
			if err := s.store.RemoveFromQueue(txCtx, next.TaskRunID); err != nil {
				return err
			}
		}
		return s.publishExecutionOutcomeEvent(txCtx, next)
	}); err != nil {
		return
	}
	CopyCommittedTaskRun(run, next)
}

func (s *TaskRuntimeService) publishExecutionOutcomeEvent(ctx context.Context, run *TaskRun) error {
	switch run.Status {
	case RunStatusSucceeded:
		return s.publishTaskEvent(ctx, TaskEventSucceeded, run, "", "")
	case RunStatusFailed:
		errCode := ""
		if run.ErrorCode != nil {
			errCode = *run.ErrorCode
		}
		return s.publishTaskEvent(ctx, TaskEventFailed, run, "", errCode)
	case RunStatusCancelled:
		return s.publishTaskEvent(ctx, TaskEventCancelled, run, "", "")
	case RunStatusTimedOut:
		return s.publishTaskEvent(ctx, TaskEventTimedOut, run, "", "")
	default:
		return nil
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
	progJSON, err := json.Marshal(prog)
	if err != nil {
		return
	}
	if err := s.store.PutProgress(ctx, taskRunID, seq, progJSON); err != nil {
		return
	}
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

	cpID := cp.CheckpointID
	run.CheckpointID = &cpID

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if err := s.store.PutCheckpoint(txCtx, cp); err != nil {
			return err
		}
		return s.store.PutTaskRun(txCtx, run)
	}); err != nil {
		return
	}
}

func (s *TaskRuntimeService) handleFinished(ctx context.Context, run *TaskRun, status string, result json.RawMessage, artifactID string, errCode, errMsg string) {
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil {
		return
	}
	existingRevision := current.Revision
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
		eventType = TaskEventFailed
	}

	next.Revision = NextRevision(existingRevision)

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if runResult != nil {
			if err := s.store.PutResult(txCtx, runResult); err != nil {
				return err
			}
		}
		if err := s.store.PutTaskRun(txCtx, next); err != nil {
			return err
		}
		if err := s.store.RemoveFromQueue(txCtx, next.TaskRunID); err != nil {
			return err
		}
		return s.publishTaskEvent(txCtx, eventType, next, "", errCode)
	}); err != nil {
		return
	}

	run.Status = next.Status
	run.FinishedAt = next.FinishedAt
	run.Revision = next.Revision
	run.ErrorCode = next.ErrorCode
	run.ErrorMessage = next.ErrorMessage
	run.ResultArtifactID = next.ResultArtifactID
}

func (s *TaskRuntimeService) handleCrash(ctx context.Context, run *TaskRun, def *TaskDefinition, exitCode int) {
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil {
		return
	}
	existingRevision := current.Revision
	next := cloneTaskRun(current)
	now := time.Now().UTC()
	next.FinishedAt = &now
	errMsg := fmt.Sprintf("task process crashed with exit code %d", exitCode)
	next.ErrorMessage = &errMsg

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

	var eventType TaskDomainEventType
	switch recoverability {
	case CheckpointRecoverable:
		cp, _ := s.store.GetLatestCheckpoint(ctx, next.TaskRunID)
		if cp != nil {
			next.Status = RunStatusRecoveryRequired
			eventType = TaskEventRecoveryRequired
		} else {
			next.Status = RunStatusFailed
			eventType = TaskEventFailed
		}
	case RestartableFromBeginning:
		if idempotency == Idempotent && next.Attempt < next.MaxAttempts {
			next.Status = RunStatusRecoveryRequired
			eventType = TaskEventRecoveryRequired
		} else {
			next.Status = RunStatusFailed
			eventType = TaskEventFailed
		}
	case ManualRecovery:
		next.Status = RunStatusManualIntervention
		eventType = TaskEventFailed
	default:
		if idempotency == NonIdempotent {
			next.Status = RunStatusManualIntervention
			eventType = TaskEventFailed
		} else {
			next.Status = RunStatusFailed
			eventType = TaskEventFailed
		}
	}

	code := string(ErrTaskRuntimeCrashed)
	next.ErrorCode = &code
	next.Revision = NextRevision(existingRevision)

	if err := s.mutateTaskRun(ctx, taskMutationParams{
		next:       next,
		expected:   current.Status,
		generation: current.Generation,
		revision:   current.Revision,
		removeQ:    true,
		eventType:  eventType,
		eventMsg:   errMsg,
		eventCode:  code,
	}); err != nil {
		return
	}

	run.Status = next.Status
	run.FinishedAt = next.FinishedAt
	run.Revision = next.Revision
	run.ErrorCode = next.ErrorCode
	run.ErrorMessage = next.ErrorMessage
}

func (s *TaskRuntimeService) failRun(ctx context.Context, run *TaskRun, code TaskErrorCode, message string) {
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil {
		return
	}
	next := cloneTaskRun(current)
	now := time.Now().UTC()
	next.Status = RunStatusFailed
	next.FinishedAt = &now
	c := string(code)
	next.ErrorCode = &c
	next.ErrorMessage = &message
	next.Revision = NextRevision(current.Revision)

	if err := s.mutateTaskRun(ctx, taskMutationParams{
		next:       next,
		expected:   current.Status,
		generation: current.Generation,
		revision:   current.Revision,
		removeQ:    true,
		eventType:  TaskEventFailed,
		eventMsg:   message,
		eventCode:  string(code),
	}); err != nil {
		return
	}

	run.Status = next.Status
	run.FinishedAt = next.FinishedAt
	run.Revision = next.Revision
	run.ErrorCode = next.ErrorCode
	run.ErrorMessage = next.ErrorMessage
}

func (s *TaskRuntimeService) Cancel(ctx context.Context, taskRunID, reason string) error {
	current, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return NewTaskError(ErrTaskNotFound, err.Error())
	}

	if current.Status.IsTerminal() {
		return NewTaskError(ErrTaskNotCancelable, "task already terminal: "+string(current.Status))
	}

	now := time.Now().UTC()
	placement := current.EffectiveExecutionPlacement()
	if placement != TaskExecutionPlacementLocal {
		// A queued remote task has not been handed to a worker yet; cancel it
		// locally without manufacturing a remote acknowledgement.
		if current.Status == RunStatusQueued {
			cancelledRun := cloneTaskRun(current)
			cancelledRun.Status = RunStatusCancelled
			cancelledRun.CancelRequestedAt = &now
			cancelledRun.FinishedAt = &now
			cancelledRun.Revision = NextRevision(current.Revision)
			return s.mutateTaskRun(ctx, taskMutationParams{
				next:       cancelledRun,
				expected:   current.Status,
				generation: current.Generation,
				revision:   current.Revision,
				removeQ:    true,
				eventType:  TaskEventCancelled,
				eventMsg:   reason,
			})
		}
		if s.remoteExecutor == nil {
			return NewTaskError(ErrRemoteTaskExecutorUnavailable, "remote cancellation not available")
		}
		if current.Status != RunStatusRunning && current.Status != RunStatusCancelling {
			return NewTaskError(ErrTaskNotCancelable, "remote task is not running: "+string(current.Status))
		}

		if current.Status == RunStatusRunning {
			cancelling := cloneTaskRun(current)
			cancelling.Status = RunStatusCancelling
			cancelling.CancelRequestedAt = &now
			cancelling.Revision = NextRevision(current.Revision)
			if err := s.mutateTaskRun(ctx, taskMutationParams{
				next:       cancelling,
				expected:   current.Status,
				generation: current.Generation,
				revision:   current.Revision,
			}); err != nil {
				return err
			}
			current = cancelling
		}

		cancelTimeout := s.config.CancelGracePeriod
		if cancelTimeout <= 0 {
			cancelTimeout = 10 * time.Second
		}
		if waiter, ok := s.remoteExecutor.(interface {
			CancelAndWait(ctx context.Context, run *TaskRun, timeout time.Duration) error
		}); ok {
			if err := waiter.CancelAndWait(ctx, current, cancelTimeout); err != nil {
				return err
			}
		} else if err := s.remoteExecutor.Cancel(ctx, current); err != nil {
			return err
		}

		latest, err := s.store.GetTaskRun(ctx, taskRunID)
		if err != nil {
			return err
		}
		if latest.Status == RunStatusCancelled {
			return nil
		}
		if latest.Status.IsTerminal() {
			return NewTaskError(ErrTaskNotCancelable, "remote cancel completed with status: "+string(latest.Status))
		}
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "remote cancel acknowledged without terminal cancellation state")
	}

	current.CancelRequestedAt = &now

	if current.Status == RunStatusQueued || current.Status == RunStatusPaused || current.Status == RunStatusPausing {
		cancelledRun := cloneTaskRun(current)
		cancelledRun.Status = RunStatusCancelled
		cancelledRun.FinishedAt = &now
		cancelledRun.Revision = NextRevision(current.Revision)
		if err := s.mutateTaskRun(ctx, taskMutationParams{
			next:       cancelledRun,
			expected:   current.Status,
			generation: current.Generation,
			revision:   current.Revision,
			removeQ:    true,
			eventType:  TaskEventCancelled,
			eventMsg:   reason,
		}); err != nil {
			return err
		}
		return nil
	}

	next := cloneTaskRun(current)
	next.Status = RunStatusCancelling
	next.Revision = NextRevision(current.Revision)
	if err := s.mutateTaskRun(ctx, taskMutationParams{
		next:       next,
		expected:   RunStatusRunning,
		generation: current.Generation,
		revision:   current.Revision,
	}); err != nil {
		return err
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

	now := time.Now().UTC()
	newRun := &TaskRun{
		TaskRunID:          "tr-" + uuid.NewString(),
		OperationID:        run.OperationID,
		TaskDefinitionID:   run.TaskDefinitionID,
		ExtensionID:        run.ExtensionID,
		ModuleID:           run.ModuleID,
		Status:             RunStatusQueued,
		Priority:           run.Priority,
		ExecutionPlacement: run.ExecutionPlacement,
		ExecutionTarget: TaskExecutionTarget{
			ProviderID:         run.ExecutionTarget.ProviderID,
			ProviderInstanceID: run.ExecutionTarget.ProviderInstanceID,
			UserID:             run.ExecutionTarget.UserID,
			DeviceID:           run.ExecutionTarget.DeviceID,
			RuntimeID:          run.ExecutionTarget.RuntimeID,
			RuntimeInstanceID:  run.ExecutionTarget.RuntimeInstanceID,
		},
		ExecutionResolvedAt: &now,
		ExecutionResolvedBy: "retry-inherit",
		Input:               run.Input,
		InputHash:           run.InputHash,
		Attempt:             run.Attempt + 1,
		MaxAttempts:         run.MaxAttempts,
		CreatedAt:           now,
		QueuedAt:            ptrTime(now),
		DeadlineAt:          run.DeadlineAt,
		Generation:          run.Generation + 1,
		Revision:            1,
	}

	queueEntry := &TaskQueueEntry{
		TaskRunID:   newRun.TaskRunID,
		Priority:    newRun.Priority,
		AvailableAt: now,
		CreatedAt:   now,
	}

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if err := s.store.PutTaskRun(txCtx, newRun); err != nil {
			return fmt.Errorf("task_runtime: persist retry run: %w", err)
		}
		if err := s.store.EnqueueTask(txCtx, queueEntry); err != nil {
			return fmt.Errorf("task_runtime: enqueue retry: %w", err)
		}
		return s.publishTaskEvent(txCtx, TaskEventQueued, newRun, "", "")
	}); err != nil {
		return nil, err
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

	previousStatus := run.Status
	run.Status = RunStatusQueued
	now := time.Now().UTC()
	run.QueuedAt = &now
	run.Generation++
	run.Revision = NextRevision(run.Revision)

	if run.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice {
		run.ClearTransientConnectionBinding()
	}

	run.ExecutionAttemptID = ""
	run.RuntimeInstanceID = nil

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, run, previousStatus, run.Generation-1, run.Revision-1)
		if casErr != nil {
			return fmt.Errorf("task_runtime: recover cas: %w", casErr)
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry recover")
		}
		if err := s.queue.Enqueue(txCtx, run); err != nil {
			return err
		}
		return s.publishTaskEvent(txCtx, TaskEventQueued, run, "", "")
	}); err != nil {
		return nil, err
	}

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

func (s *TaskRuntimeService) GetTaskResult(ctx context.Context, taskRunID string) (*TaskRunResult, error) {
	return s.GetResult(ctx, taskRunID)
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
			if err := s.recoverRun(ctx, run); err != nil {
				return err
			}
		}
	}

	if _, reclaimErr := s.queue.ReclaimExpired(ctx); reclaimErr != nil {
		return reclaimErr
	}
	return nil
}

func (s *TaskRuntimeService) recoverRun(ctx context.Context, run *TaskRun) error {
	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		run.Status = RunStatusManualIntervention
		msg := "definition not found during recovery"
		run.ErrorMessage = &msg
		return s.store.PutTaskRun(ctx, run)
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

	var enqueueNeeded bool
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
			enqueueNeeded = true
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

	return s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		if err := s.store.PutTaskRun(txCtx, run); err != nil {
			return err
		}
		if enqueueNeeded {
			return s.queue.Enqueue(txCtx, run)
		}
		return nil
	})
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
	if err := os.RemoveAll(workspace); err != nil {
		log.Printf("task_runtime: cleanup workspace failed for %s: %v", taskRunID, err)
	}
}

func (s *TaskRuntimeService) createTaskWorkspace(taskRunID string) (string, error) {
	base := s.config.WorkspaceRoot
	if base == "" {
		base = os.TempDir()
	}
	workspace := filepath.Join(base, "task-workspace-"+taskRunID)
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		return "", err
	}
	return workspace, nil
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func strPtr(s string) *string { return &s }

func ptrTime(t time.Time) *time.Time { return &t }

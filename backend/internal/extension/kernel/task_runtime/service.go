package task_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"

	"github.com/u-ai/backend/internal/extension/kernel/script_host"
)

type TaskRuntimeService struct {
	store   TaskStore
	queue   *TaskQueue
	limiter *ConcurrencyLimiter
	config  TaskRuntimeConfig

	eventMu        sync.RWMutex
	events         TaskEventSink
	eventObservers []TaskEventSink

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
	s.eventMu.Lock()
	s.events = sink
	s.eventMu.Unlock()
}

// AddEventSink adds a best-effort observer while preserving the primary
// durable event sink configured by the task runtime.
func (s *TaskRuntimeService) AddEventSink(sink TaskEventSink) {
	if sink == nil {
		return
	}
	s.eventMu.Lock()
	s.eventObservers = append(s.eventObservers, sink)
	s.eventMu.Unlock()
}

func (s *TaskRuntimeService) SetRemoteExecutor(executor RemoteTaskExecutor) {
	s.remoteExecutor = executor
}

func (s *TaskRuntimeService) RemoteExecutor() RemoteTaskExecutor {
	return s.remoteExecutor
}

func (s *TaskRuntimeService) publishTaskEvent(ctx context.Context, eventType TaskDomainEventType, run *TaskRun, reason, errorCode string) error {
	s.eventMu.RLock()
	primary := s.events
	observers := append([]TaskEventSink(nil), s.eventObservers...)
	s.eventMu.RUnlock()
	if primary == nil && len(observers) == 0 {
		return nil
	}
	event := TaskDomainEvent{
		Type:       eventType,
		Run:        *run,
		Reason:     reason,
		ErrorCode:  errorCode,
		OccurredAt: time.Now().UTC(),
	}
	if primary != nil {
		if err := primary.TaskEvent(ctx, event); err != nil {
			return fmt.Errorf("task_runtime: publish event %s: %w", eventType, err)
		}
	}
	for _, observer := range observers {
		if observer != nil {
			_ = observer.TaskEvent(ctx, event)
		}
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
	definitionFingerprint, err := taskDefinitionFingerprint(def)
	if err != nil {
		return nil, err
	}
	if err := s.validateEnqueueAuthority(ctx, req, def); err != nil {
		return nil, err
	}
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
	var deadlineAt *time.Time
	if duration := timeoutpolicy.Duration(deadline.Sub(now)); duration > 0 {
		deadline = now.Add(duration)
		deadlineAt = &deadline
	}

	maxAttempts := def.RetryPolicy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	run := &TaskRun{
		TaskRunID:             runID,
		OperationID:           req.OperationID,
		InvocationID:          req.InvocationID,
		TaskDefinitionID:      def.TaskID,
		DefinitionFingerprint: definitionFingerprint,
		ExtensionID:           def.ExtensionID,
		ModuleID:              def.ModuleID,
		Status:                RunStatusQueued,
		Priority:              req.Priority,
		ExecutionPlacement:    placement,
		Input:                 req.Input,
		InputHash:             inputHash,
		TraceID:               req.TraceID,
		CorrelationID:         req.CorrelationID,
		CausationID:           req.CausationID,
		Source:                req.Source,
		ScopeSnapshotID:       req.ScopeSnapshotID,
		PermissionSnapshotID:  req.PermissionSnapshotID,
		Attempt:               1,
		MaxAttempts:           maxAttempts,
		CreatedAt:             now,
		QueuedAt:              &now,
		DeadlineAt:            deadlineAt,
		Generation:            1,
		Revision:              1,
	}

	// Workflow and other trusted coordinators may already have resolved a
	// stable remote target. Bind it before the run is persisted/enqueued so
	// dispatch can never race ahead and observe an unresolved remote target.
	if req.TrustedExecutionTarget != nil {
		targetReq := *req.TrustedExecutionTarget
		if targetReq.Placement == "" {
			targetReq.Placement = placement
		}
		if targetReq.Placement != placement {
			return nil, NewTaskError(ErrTaskExecutionPlacementInvalid, "trusted execution target placement does not match enqueue placement")
		}
		if err := run.BindExecutionTarget(TaskPlacementDecision{
			Placement: targetReq.Placement,
			Target:    targetReq.Target,
			Resolved:  true,
		}, targetReq.ResolvedBy, now); err != nil {
			return nil, err
		}
	}

	if _, owned := coordination.FromContext(ctx); owned {
		if s.config.OwnedInputs == nil {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者输入端口不可用")
		}
		if err := s.config.OwnedInputs.SaveInput(ctx, run); err != nil {
			return nil, err
		}
		run.Input = nil
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
	guarded, finish, authorityErr := s.restoreTaskAuthority(ctx, run)
	if authorityErr != nil {
		s.failRun(ctx, run, ErrTaskScopeDenied, authorityErr.Error())
		return
	}
	defer finish()
	ctx = guarded

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		s.failRun(ctx, run, ErrTaskDefinitionInvalid, fmt.Sprintf("definition not found: %v", err))
		return
	}
	_, owned := coordination.FromContext(ctx)
	if err := validateTaskDefinition(owned, run, def); err != nil {
		s.failRun(ctx, run, ErrTaskDefinitionInvalid, err.Error())
		return
	}
	input := append(json.RawMessage(nil), run.Input...)
	if owned {
		ctx = s.guardOwnedTaskDefinition(ctx, run)
		if s.config.OwnedInputs == nil {
			s.failRun(ctx, run, ErrTaskScopeDenied, "任务所有者输入端口不可用")
			return
		}
		input, err = s.config.OwnedInputs.Input(ctx, run)
		if err != nil {
			s.failRun(ctx, run, ErrTaskScopeDenied, err.Error())
			return
		}
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

		remoteRun := CloneTaskRun(run)
		remoteRun.Input = input
		outcome := s.runRemoteExecution(ctx, remoteRun, def, executor)
		s.applyExecutionOutcome(ctx, run, def, outcome)
		if _, owned := coordination.FromContext(ctx); owned && !run.Status.IsTerminal() {
			if err := s.waitOwnedRemoteExecution(ctx, run, executor); err != nil {
				log.Printf("task_runtime: owned remote task requires result confirmation: %s", run.TaskRunID)
			}
		}
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
		cp, err := s.readTaskCheckpoint(ctx, run)
		if err != nil || cp == nil || cp.CheckpointID != *run.CheckpointID || cp.DefinitionHash != def.DefinitionHash || cp.InputHash != run.InputHash || cp.PayloadHash != hashBytes(cp.Payload) {
			s.failRun(ctx, run, ErrTaskCheckpointIncompatible, "执行检查点缺失或与任务不匹配")
			s.cleanupWorkspace(run.TaskRunID, workspace)
			return
		}
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
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, runningRun, run.Status, run.Generation, run.Revision)
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
	CopyCommittedTaskRun(run, runningRun)

	instanceID := "ri-" + uuid.NewString()
	if run.RuntimeInstanceID != nil && *run.RuntimeInstanceID != "" {
		instanceID = *run.RuntimeInstanceID
	}

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

	if s.config.EntryResolver == nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, "任务入口解析器不可用")
		s.cleanupWorkspace(run.TaskRunID, workspace)
		return
	}
	entryPath, err := s.config.EntryResolver(ctx, def)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, err.Error())
		s.cleanupWorkspace(run.TaskRunID, workspace)
		return
	}
	hostCfg := ProcessHostConfig{
		Generation:  run.Generation,
		InstanceID:  instanceID,
		TaskRunID:   run.TaskRunID,
		ExtensionID: run.ExtensionID,
		ModuleID:    run.ModuleID,
		DefHash:     def.DefinitionHash,
		NodePath:    nodeEnv.NodeBinary,
		HostPath:    hostArtifact.EntryPath,
		WorkDir:     workspace,
		EntryPath:   entryPath,
		EntryHash:   def.EntryHash,
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
		finalRun, _ := s.store.GetTaskRun(ctx, run.TaskRunID)
		if finalRun == nil || finalRun.Status != RunStatusPaused && finalRun.Status != RunStatusPausing {
			s.cleanupWorkspace(run.TaskRunID, workspace)
		}
		s.mu.Lock()
		if s.activeHosts[run.TaskRunID] == host {
			delete(s.activeHosts, run.TaskRunID)
		}
		s.mu.Unlock()
	}()

	previousProgress, err := s.store.GetProgress(ctx, run.TaskRunID)
	if err != nil {
		host.ForceStop()
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, "任务进度版本不可用")
		return
	}
	var progressBase int64
	if previousProgress != nil {
		progressBase = previousProgress.Sequence
	}
	callbacks := ProcessCallbacks{
		OnRequest: func(requestCtx context.Context, requestID, method string, params json.RawMessage) (json.RawMessage, error) {
			if _, owned := coordination.FromContext(requestCtx); !owned {
				return nil, NewTaskError(ErrTaskScopeDenied, "任务存储缺少数据所有者执行端口")
			}
			live, err := s.store.GetTaskRun(requestCtx, run.TaskRunID)
			if err != nil || live == nil || live.Generation != run.Generation || live.ExecutionAttemptID != run.ExecutionAttemptID || live.Status != RunStatusRunning && live.Status != RunStatusCheckpointing {
				return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "任务存储所属执行状态已变化")
			}
			var response json.RawMessage
			if strings.HasPrefix(method, "task.artifact.") {
				if s.config.OwnedArtifacts == nil {
					return nil, NewTaskError(ErrTaskScopeDenied, "任务产物所有者端口不可用")
				}
				response, err = s.config.OwnedArtifacts.Call(requestCtx, live, requestID, method, params)
			} else {
				if s.config.OwnedStorage == nil {
					return nil, NewTaskError(ErrTaskScopeDenied, "任务存储所有者端口不可用")
				}
				response, err = s.config.OwnedStorage.Call(requestCtx, live, requestID, method, params)
			}
			if err != nil {
				return nil, err
			}
			latest, err := s.store.GetTaskRun(requestCtx, run.TaskRunID)
			if err != nil || latest == nil || latest.Generation != live.Generation || latest.ExecutionAttemptID != live.ExecutionAttemptID || latest.Status != live.Status || latest.Revision != live.Revision {
				return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "任务存储确认期间执行状态已变化")
			}
			if err := coordination.ValidateCurrent(requestCtx); err != nil {
				return nil, err
			}
			return response, nil
		},
		OnProgress: func(seq int64, current, total, percentage *float64, stage, message string) {
			if seq < 1 || progressBase < 0 || seq > int64(^uint64(0)>>1)-progressBase {
				return
			}
			s.handleProgress(ctx, run.TaskRunID, seq+progressBase, current, total, percentage, stage, message, run)
		},
		OnCheckpoint: func(version int64, payload json.RawMessage, hash string) {
			s.handleCheckpoint(ctx, run, def, payload, hash, version)
		},
		OnCheckpointConfirmed: func(version int64, payload json.RawMessage, hash string) error {
			return s.handleCheckpoint(ctx, run, def, payload, hash, version)
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
	beforeStart, err := s.store.GetTaskRun(taskCtx, run.TaskRunID)
	if err != nil || beforeStart == nil || beforeStart.Generation != run.Generation || beforeStart.ExecutionAttemptID != run.ExecutionAttemptID || beforeStart.Status != RunStatusRunning {
		_ = host.Cancel(context.WithoutCancel(taskCtx), "任务启动前状态已变化")
		if err == nil && beforeStart != nil && beforeStart.Status == RunStatusCancelling {
			s.handleFinished(ctx, run, "cancelled", nil, "", "", "任务已取消")
		}
		return
	}
	if err := coordination.ValidateCurrent(taskCtx); err != nil {
		host.ForceStop()
		return
	}

	if err := host.Start(taskCtx, input, checkpointPayload, run.DeadlineAt, run.Attempt, run.MaxAttempts, callbacks); err != nil {
		if host.State() == "cancelled" {
			s.handleFinished(ctx, run, "cancelled", nil, "", "", "任务已取消")
			return
		}
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, err.Error())
		return
	}

	exitCode, _ := host.Wait()

	if run.Status.IsTerminal() {
		return
	}

	if exitCode != 0 && !run.Status.IsTerminal() {
		s.handleCrash(ctx, run, def, exitCode)
	} else if run.ScopeSnapshotID != "" && run.Status != RunStatusPaused && run.Status != RunStatusPausing {
		latest, err := s.store.GetTaskRun(context.WithoutCancel(ctx), run.TaskRunID)
		if err == nil && latest != nil && latest.Generation == run.Generation && latest.ExecutionAttemptID == run.ExecutionAttemptID && !latest.Status.IsTerminal() {
			_ = s.markTaskRecoveryUnknown(context.WithoutCancel(ctx), latest)
		}
	}
}

func (s *TaskRuntimeService) runRemoteExecution(ctx context.Context, run *TaskRun, def *TaskDefinition, executor TaskExecutorPort) TaskExecutionOutcome {
	var pin *TargetTaskDefinitionPin
	if _, owned := coordination.FromContext(ctx); owned && run.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice {
		if s.config.OwnedTargetDefinitions == nil {
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired, ErrorCode: string(ErrTaskScopeDenied), ErrorMessage: "目标任务版本端口尚未就绪"}
		}
		prepared, err := s.config.OwnedTargetDefinitions.Prepare(ctx, run, def)
		if err != nil {
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired, ErrorCode: string(ErrTaskDefinitionInvalid), ErrorMessage: err.Error()}
		}
		pin = &prepared
	}
	request := TaskExecutionRequest{
		Run:                 run,
		Definition:          def,
		AttemptID:           run.ExecutionAttemptID,
		Placement:           run.EffectiveExecutionPlacement(),
		Target:              run.ExecutionTarget,
		TargetDefinitionPin: pin,
	}
	if _, owned := coordination.FromContext(ctx); owned && run.CheckpointID != nil {
		checkpoint, err := s.readTaskCheckpoint(ctx, run)
		if err != nil || checkpoint == nil || checkpoint.CheckpointID != *run.CheckpointID {
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired, ErrorCode: string(ErrTaskCheckpointIncompatible), ErrorMessage: "设备任务缺少所有者确认的恢复检查点"}
		}
		request.ResumeCheckpoint = checkpoint
	}
	if _, owned := coordination.FromContext(ctx); owned {
		previous, err := s.store.GetProgress(ctx, run.TaskRunID)
		if err != nil {
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired, ErrorCode: string(ErrTaskExecutionAttemptInvalid), ErrorMessage: "任务进度基础版本不可用"}
		}
		if previous != nil {
			request.ProgressBase = previous.Sequence
		}
	}
	outcome, _ := executor.Execute(ctx, request)
	return outcome
}

func (s *TaskRuntimeService) applyExecutionOutcome(ctx context.Context, run *TaskRun, def *TaskDefinition, outcome TaskExecutionOutcome) {
	now := time.Now().UTC()
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil || current == nil || current.Generation != run.Generation || current.ExecutionAttemptID != run.ExecutionAttemptID {
		return
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return
	}
	if _, owned := coordination.FromContext(ctx); owned {
		if outcome.Status.IsTerminal() {
			var result json.RawMessage
			artifactID := ""
			if outcome.Result != nil {
				result, artifactID = outcome.Result.ResultJSON, outcome.Result.ArtifactID
			}
			s.handleFinished(ctx, run, string(outcome.Status), result, artifactID, outcome.ErrorCode, outcome.ErrorMessage)
			return
		}
		outcome.Result = nil
		if outcome.ErrorMessage != "" {
			outcome.ErrorMessage = "任务执行状态尚未确认，请检查数据所有者处的任务状态"
			outcome.ErrorCode = "task_execution_unconfirmed"
		}
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
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		if casErr != nil {
			return fmt.Errorf("task_runtime: apply outcome cas: %w", casErr)
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry outcome")
		}
		if outcome.Result != nil && outcome.Status.IsTerminal() {
			if err := s.store.PutResult(txCtx, outcome.Result); err != nil {
				return fmt.Errorf("task_runtime: put result: %w", err)
			}
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

func (s *TaskRuntimeService) handleProgress(ctx context.Context, taskRunID string, seq int64, current, total, percentage *float64, stage, message string, expected ...*TaskRun) error {
	return s.persistTaskProgress(ctx, taskRunID, seq, current, total, percentage, stage, message, false, expected...)
}

func (s *TaskRuntimeService) persistTaskProgress(ctx context.Context, taskRunID string, seq int64, current, total, percentage *float64, stage, message string, requireConfirmation bool, expected ...*TaskRun) error {
	if seq < 1 || len(stage) > 512 || len(message) > 32<<10 {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务进度格式无效")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	live, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil || live == nil || live.Status != RunStatusRunning && live.Status != RunStatusCheckpointing {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务进度所属状态无效")
	}
	if len(expected) > 0 && (expected[0] == nil || live.Generation != expected[0].Generation || live.ExecutionAttemptID != expected[0].ExecutionAttemptID) {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务进度所属执行已变化")
	}
	if _, owned := coordination.FromContext(ctx); owned && len(expected) == 0 {
		return NewTaskError(ErrTaskScopeDenied, "任务进度缺少执行身份")
	}
	rate := s.config.MaxProgressPerSecond
	if rate < 1 {
		rate = 5
	}
	s.progressMu.Lock()
	last, ok := s.progressLast[taskRunID]
	if !requireConfirmation && ok && time.Since(last) < time.Second/time.Duration(rate) {
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
	if _, owned := coordination.FromContext(ctx); owned {
		if s.config.OwnedProgress == nil {
			return NewTaskError(ErrTaskScopeDenied, "任务进度所有者存储不可用")
		}
		prog, err = s.config.OwnedProgress.SaveProgress(ctx, live, prog)
		if err != nil {
			return err
		}
	}
	progJSON, err := json.Marshal(prog)
	if err != nil {
		return err
	}
	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		latest, err := s.store.GetTaskRun(txCtx, taskRunID)
		if err != nil {
			return err
		}
		if latest == nil || latest.Generation != live.Generation || latest.ExecutionAttemptID != live.ExecutionAttemptID || latest.Revision != live.Revision || latest.Status != live.Status {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务进度所属执行已变化")
		}
		return s.store.PutProgress(txCtx, taskRunID, seq, progJSON)
	}); err != nil {
		return err
	}
	return nil
}

func (s *TaskRuntimeService) handleCheckpoint(ctx context.Context, run *TaskRun, def *TaskDefinition, payload json.RawMessage, hash string, version int64) error {
	if version < 1 || len(payload) > s.config.MaxCheckpointBytes || !json.Valid(payload) {
		return NewTaskError(ErrTaskCheckpointTooLarge, "检查点格式或大小无效")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil || current == nil || current.Generation != run.Generation || current.ExecutionAttemptID != run.ExecutionAttemptID || current.Status != RunStatusRunning && current.Status != RunStatusCheckpointing && current.Status != RunStatusPausing {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "检查点所属执行状态已变化")
	}
	previous, err := s.store.GetLatestCheckpoint(ctx, run.TaskRunID)
	if err != nil || previous != nil && previous.Version >= version {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "检查点版本已失效")
	}

	actualHash := hashBytes(payload)
	if hash != "" && hash != actualHash {
		return NewTaskError(ErrTaskCheckpointHashMismatch, "检查点摘要不匹配")
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
	next := CloneTaskRun(current)
	next.CheckpointID = &cpID
	next.Revision = NextRevision(current.Revision)
	if _, owned := coordination.FromContext(ctx); owned {
		if s.config.OwnedCheckpoints == nil {
			return NewTaskError(ErrTaskScopeDenied, "检查点所有者存储不可用")
		}
		if err := s.config.OwnedCheckpoints.SaveCheckpoint(ctx, current, cp); err != nil {
			return err
		}
		metadata := *cp
		metadata.Payload = nil
		cp = &metadata
	}

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, err := s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		if err != nil {
			return err
		}
		if !ok {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "检查点保存前任务已变化")
		}
		if err := s.store.PutCheckpoint(txCtx, cp); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	run.CheckpointID = &cpID
	run.Revision = next.Revision
	return nil
}

func (s *TaskRuntimeService) handleFinished(ctx context.Context, run *TaskRun, status string, result json.RawMessage, artifactID string, errCode, errMsg string) error {
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil || current == nil || current.Status.IsTerminal() || current.Status == RunStatusPaused || current.Status == RunStatusPausing || current.Generation != run.Generation || current.ExecutionAttemptID != run.ExecutionAttemptID {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务结果所属执行状态已变化")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if current.Status == RunStatusCancelling {
		status, result, artifactID, errCode, errMsg = "cancelled", nil, "", "", "任务已取消"
	}
	if _, owned := coordination.FromContext(ctx); owned && status == "succeeded" && artifactID == "" && len(result) > 64<<10 {
		if s.config.OwnedArtifacts == nil || len(result) > 1<<20 || !json.Valid(result) {
			return NewTaskError(ErrTaskScopeDenied, "大型任务结果需要所有者产物端口")
		}
		params, err := json.Marshal(map[string]any{"task_run_id": run.TaskRunID, "name": "result.json", "data": result})
		if err != nil {
			return err
		}
		response, err := s.config.OwnedArtifacts.Call(ctx, current, "task-final/"+run.ExecutionAttemptID.String(), "task.artifact.saveData", params)
		if err != nil {
			return err
		}
		var metadata ownedTaskArtifactMetadata
		if json.Unmarshal(response, &metadata) != nil || metadata.ArtifactID == "" {
			return NewTaskError(ErrTaskScopeDenied, "任务产物确认引用无效")
		}
		artifactID, result = metadata.ArtifactID, nil
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
	case "timed_out":
		next.Status = RunStatusTimedOut
		eventType = TaskEventTimedOut
		next.ErrorCode, next.ErrorMessage = &errCode, &errMsg
	case "manual_intervention":
		next.Status = RunStatusManualIntervention
		eventType = TaskEventFailed
		next.ErrorCode, next.ErrorMessage = &errCode, &errMsg
	default:
		return NewTaskError(ErrTaskStateTransitionInvalid, "任务终态无效")
	}

	next.Revision = NextRevision(existingRevision)
	if _, owned := coordination.FromContext(ctx); owned {
		if s.config.OwnedOutcomes == nil {
			return NewTaskError(ErrTaskScopeDenied, "任务结果所有者存储不可用")
		}
		if runResult != nil && runResult.ResultType == ResultArtifact {
			if s.config.OwnedArtifacts == nil {
				return NewTaskError(ErrTaskScopeDenied, "任务产物结果端口不可用")
			}
			_, hash, err := s.config.OwnedArtifacts.Result(ctx, current, runResult.ArtifactID)
			if err != nil {
				return err
			}
			runResult.ResultJSON, runResult.ResultHash = nil, hash
		}
		if err := s.config.OwnedOutcomes.SaveOutcome(ctx, current, status, runResult, errCode, errMsg); err != nil {
			return err
		}
		if runResult != nil {
			metadata := *runResult
			metadata.ResultJSON = nil
			runResult = &metadata
		}
		if status != "succeeded" {
			errCode = "task_" + status
			next.ErrorCode = &errCode
			message := "任务已停止，详细结果保存在数据所有者处"
			next.ErrorMessage = &message
		}
	}

	if err := s.store.WithinTaskTx(ctx, func(txCtx context.Context) error {
		ok, err := s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		if err != nil {
			return err
		}
		if !ok {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务完成前状态已变化")
		}
		if runResult != nil {
			if err := s.store.PutResult(txCtx, runResult); err != nil {
				return err
			}
		}
		if err := s.store.RemoveFromQueue(txCtx, next.TaskRunID); err != nil {
			return err
		}
		return s.publishTaskEvent(txCtx, eventType, next, "", errCode)
	}); err != nil {
		return err
	}

	run.Status = next.Status
	run.FinishedAt = next.FinishedAt
	run.Revision = next.Revision
	run.ErrorCode = next.ErrorCode
	run.ErrorMessage = next.ErrorMessage
	run.ResultArtifactID = next.ResultArtifactID
	return nil
}

func (s *TaskRuntimeService) handleCrash(ctx context.Context, run *TaskRun, def *TaskDefinition, exitCode int) {
	current, err := s.store.GetTaskRun(ctx, run.TaskRunID)
	if err != nil {
		return
	}
	if current == nil || current.Status.IsTerminal() || current.Status == RunStatusPaused || current.Status == RunStatusPausing || current.Generation != run.Generation || current.ExecutionAttemptID != run.ExecutionAttemptID {
		return
	}
	if current.Status == RunStatusCancelling {
		s.handleFinished(ctx, run, "cancelled", nil, "", "", "任务已取消")
		return
	}
	if run.ScopeSnapshotID != "" {
		_, owned, authorityErr := s.taskAuthority(ctx, run.ScopeSnapshotID, run.InvocationID, run.ExtensionID, run.ModuleID)
		if owned || authorityErr != nil {
			_ = s.markTaskRecoveryUnknown(context.WithoutCancel(ctx), current)
			return
		}
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
	if current == nil || current.Status.IsTerminal() || current.Status == RunStatusPaused || current.Status == RunStatusPausing || current.Generation != run.Generation || current.ExecutionAttemptID != run.ExecutionAttemptID {
		return
	}
	if run.ScopeSnapshotID != "" {
		_, owned, authorityErr := s.taskAuthority(ctx, run.ScopeSnapshotID, run.InvocationID, run.ExtensionID, run.ModuleID)
		if owned || authorityErr != nil {
			if _, scoped := coordination.FromContext(ctx); scoped && authorityErr == nil && current.ExecutionAttemptID != "" {
				if err := s.handleFinished(ctx, run, "failed", nil, "", string(code), message); err == nil {
					return
				}
			}
			_ = s.markTaskRecoveryUnknown(context.WithoutCancel(ctx), current)
			return
		}
	}
	if current.Status == RunStatusCancelling {
		s.handleFinished(ctx, run, "cancelled", nil, "", "", "任务已取消")
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

	if current.Status == RunStatusQueued || current.Status == RunStatusPaused {
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
	if current.Status == RunStatusCancelling {
		return nil
	}
	if current.Status != RunStatusRunning && current.Status != RunStatusStarting && current.Status != RunStatusResuming && current.Status != RunStatusCheckpointing && current.Status != RunStatusPausing {
		return NewTaskError(ErrTaskNotCancelable, "任务当前状态不允许取消")
	}
	next.CancelRequestedAt = &now
	next.Revision = NextRevision(current.Revision)
	if err := s.mutateTaskRun(ctx, taskMutationParams{
		next:       next,
		expected:   current.Status,
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
	guarded, finish, err := s.restoreTaskAuthority(ctx, run)
	if err != nil {
		return nil, err
	}
	defer finish()
	ctx = guarded
	_, owned := coordination.FromContext(ctx)
	if owned && run.Status == RunStatusSucceeded {
		return nil, NewTaskError(ErrTaskNotRetryable, "已完成的设备任务不能重复执行")
	}

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, err.Error())
	}
	if err := validateTaskDefinition(owned, run, def); err != nil {
		return nil, err
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
		TaskRunID:             "tr-" + uuid.NewString(),
		OperationID:           run.OperationID,
		InvocationID:          run.InvocationID,
		DefinitionFingerprint: run.DefinitionFingerprint,
		ScopeSnapshotID:       run.ScopeSnapshotID,
		PermissionSnapshotID:  run.PermissionSnapshotID,
		DependencySnapshotID:  run.DependencySnapshotID,
		TraceID:               run.TraceID,
		CorrelationID:         run.CorrelationID,
		CausationID:           run.CausationID,
		Source:                run.Source,
		TaskDefinitionID:      run.TaskDefinitionID,
		ExtensionID:           run.ExtensionID,
		ModuleID:              run.ModuleID,
		Status:                RunStatusQueued,
		Priority:              run.Priority,
		ExecutionPlacement:    run.ExecutionPlacement,
		ExecutionTarget: TaskExecutionTarget{
			ProviderID:         run.ExecutionTarget.ProviderID,
			ProviderInstanceID: run.ExecutionTarget.ProviderInstanceID,
			SpaceID:            run.ExecutionTarget.SpaceID,
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
	if owned {
		if s.config.OwnedInputs == nil {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务所有者输入端口不可用")
		}
		newRun.Input, err = s.config.OwnedInputs.Input(ctx, run)
		if err != nil {
			return nil, err
		}
		if err := s.config.OwnedInputs.SaveInput(ctx, newRun); err != nil {
			return nil, err
		}
		newRun.Input = nil
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
	guarded, finish, err := s.restoreTaskAuthority(ctx, run)
	if err != nil {
		return nil, err
	}
	defer finish()
	ctx = guarded

	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil {
		return nil, NewTaskError(ErrTaskDefinitionInvalid, err.Error())
	}
	_, owned := coordination.FromContext(ctx)
	if err := validateTaskDefinition(owned, run, def); err != nil {
		return nil, err
	}

	cp, _ := s.readTaskCheckpoint(ctx, run)
	if owned && (cp == nil || run.CheckpointID == nil || cp.CheckpointID != *run.CheckpointID || cp.PayloadHash != hashBytes(cp.Payload)) {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "设备任务结果未知，缺少已确认检查点，拒绝从头执行")
	}
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
	metadata, err := s.store.GetProgress(ctx, taskRunID)
	if err != nil || metadata == nil {
		return metadata, err
	}
	if _, owned := coordination.FromContext(ctx); !owned {
		return metadata, nil
	}
	if s.config.OwnedProgress == nil {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务进度所有者端口不可用")
	}
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, err
	}
	if err := s.validateTaskReadScope(ctx, run); err != nil {
		return nil, err
	}
	return s.config.OwnedProgress.Progress(ctx, run, metadata)
}

func (s *TaskRuntimeService) GetResult(ctx context.Context, taskRunID string) (*TaskRunResult, error) {
	metadata, err := s.store.GetResult(ctx, taskRunID)
	if err != nil || metadata == nil {
		return metadata, err
	}
	if _, owned := coordination.FromContext(ctx); !owned {
		return metadata, nil
	}
	if s.config.OwnedOutcomes == nil {
		return nil, NewTaskError(ErrTaskScopeDenied, "任务结果所有者端口不可用")
	}
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, err
	}
	if err := s.validateTaskReadScope(ctx, run); err != nil {
		return nil, err
	}
	return s.config.OwnedOutcomes.Result(ctx, run, metadata)
}

func (s *TaskRuntimeService) GetTaskResult(ctx context.Context, taskRunID string) (*TaskRunResult, error) {
	return s.GetResult(ctx, taskRunID)
}

func (s *TaskRuntimeService) GetLatestCheckpoint(ctx context.Context, taskRunID string) (*TaskCheckpoint, error) {
	if _, owned := coordination.FromContext(ctx); !owned {
		return s.store.GetLatestCheckpoint(ctx, taskRunID)
	}
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, err
	}
	if err := s.validateTaskReadScope(ctx, run); err != nil {
		return nil, err
	}
	return s.readTaskCheckpoint(ctx, run)
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
	if run.ScopeSnapshotID != "" {
		return s.markTaskRecoveryUnknown(ctx, run)
	}
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

package task_runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	"github.com/u-ai/backend/internal/devicemesh/executionjournal"

	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

type TaskRuntimeService struct {
	store   TaskStore
	queue   *TaskQueue
	limiter *ConcurrencyLimiter
	config  TaskRuntimeConfig

	eventMu        sync.RWMutex
	events         TaskEventSink
	eventObservers []TaskEventSink

	mu                   sync.RWMutex
	activeHosts          map[string]*TaskProcessHost
	sourceHosts          sync.Map
	sourceEventPublisher func(context.Context, *TaskRun, *TaskDefinition, string, TaskHostNativeCall) (json.RawMessage, error)
	sourceApprovals      *SourceTaskApprovalLedger
	processBudget        taskProcessBudget
	ownerLocks           [64]sync.Mutex

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
		store:           store,
		queue:           queue,
		limiter:         limiter,
		config:          config,
		activeHosts:     make(map[string]*TaskProcessHost),
		sourceApprovals: NewSourceTaskApprovalLedger(),
		progressSeq:     make(map[string]*int64),
		progressLast:    make(map[string]time.Time),
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
			if err := observer.TaskEvent(ctx, event); err != nil {
				log.Printf("[original-error] %v", err)
			}
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
	return s.withManagementTaskTx(ctx, func(txCtx context.Context) error {
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
	return coordination.CommitCurrent(ctx, func() error { return s.store.PutTaskDefinition(ctx, def) })
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
	if s.sourceApprovals != nil {
		s.sourceApprovals.Close()
	}
	hosts := make([]*TaskProcessHost, 0, len(s.activeHosts))
	for _, h := range s.activeHosts {
		hosts = append(hosts, h)
	}
	s.sourceHosts.Range(func(_, value any) bool {
		hosts = append(hosts, value.(*sourceTaskProcess).host)
		return true
	})
	s.mu.Unlock()

	for _, h := range hosts {
		if err := h.Cancel(ctx, "application_shutdown"); err != nil {
			log.Printf("[original-error] %v", err)
		}
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
	var creator TaskRunCreationStore
	if req.DeduplicateOwnedRequest {
		authority, owned := coordination.FromContext(ctx)
		if !owned {
			return nil, coordination.ErrWrongOwner
		}
		runID, err = OwnedRequestTaskRunID(authority)
		if err != nil {
			return nil, err
		}
		creator, _ = s.store.(TaskRunCreationStore)
		if creator == nil {
			return nil, NewTaskError(ErrTaskScopeDenied, "任务存储不支持原子请求去重")
		}
	}
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

	if req.DeduplicateOwnedRequest {
		previous, err := s.existingOwnedEnqueue(ctx, run)
		if err != nil || previous != nil {
			return previous, err
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
	if err := s.withManagementTaskTx(ctx, func(ctx context.Context) error {
		if creator != nil {
			created, err := creator.CreateTaskRun(ctx, run)
			if err != nil {
				return err
			}
			if !created {
				return errOwnedTaskAlreadyCreated
			}
		} else if err := s.store.PutTaskRun(ctx, run); err != nil {
			return fmt.Errorf("task_runtime: persist run: %w", err)
		}
		if err := s.queue.Enqueue(ctx, run); err != nil {
			return fmt.Errorf("task_runtime: enqueue: %w", err)
		}
		return s.publishTaskEvent(ctx, TaskEventQueued, run, "", "")
	}); err != nil {
		if errors.Is(err, errOwnedTaskAlreadyCreated) {
			previous, readErr := s.existingOwnedEnqueue(ctx, run)
			if readErr == nil && previous == nil {
				readErr = coordination.ErrRequestConflict
			}
			return previous, readErr
		}
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
			s.tryDispatch()
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
		if err != nil || run == nil || run.Status != RunStatusQueued {
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
	if current == nil || current.Status != RunStatusQueued || current.Generation != run.Generation || current.Revision != run.Revision {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "任务已被领取或排队版本发生变化")
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

	releaseProcess, capacityErr := s.reserveTaskProcess(ctx, def)
	if capacityErr != nil {
		if errors.Is(capacityErr, ErrTaskProcessCapacity) {
			if err := s.queue.ReenqueueWithDelay(ctx, run, 5*time.Second); err != nil {
				s.failRun(ctx, run, ErrTaskRuntimeStartFailed, "任务进程容量等待入队失败")
			}
		} else {
			s.failRun(ctx, run, ErrTaskRuntimeStartFailed, capacityErr.Error())
		}
		return
	}
	defer releaseProcess()
	if err := s.persistExecutionAttempt(ctx, run, attemptID, ""); err != nil {
		s.failRun(ctx, run, ErrTaskExecutionAttemptInvalid, fmt.Sprintf("persist attempt: %v", err))
		return
	}

	workspace, err := s.createTaskWorkspace(run.TaskRunID)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("workspace: %v", err))
		return
	}

	defer func() {
		finalRun, readErr := s.store.GetTaskRun(context.WithoutCancel(ctx), run.TaskRunID)
		if readErr != nil || finalRun == nil || finalRun.Status != RunStatusPaused && finalRun.Status != RunStatusPausing {
			s.cleanupWorkspace(run.TaskRunID, workspace)
		}
	}()

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
	var entryPath string
	if s.config.InstalledExecutionLease != nil && def.BundleHash != "" {
		root, release, leaseErr := s.config.InstalledExecutionLease(ctx, def)
		if leaseErr != nil || release == nil || root == "" {
			if release != nil {
				release()
			}
			s.failRun(ctx, run, ErrTaskRuntimeStartFailed, fmt.Sprintf("任务安装读取租约未确认: %v", leaseErr))
			s.cleanupWorkspace(run.TaskRunID, workspace)
			return
		}
		defer release()
		entryPath, err = ResolveTaskEntry(ctx, root, def)
	} else {
		entryPath, err = s.config.EntryResolver(ctx, def)
	}
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, err.Error())
		s.cleanupWorkspace(run.TaskRunID, workspace)
		return
	}
	bundleRoot, err := TaskBundleRoot(entryPath, def)
	if err != nil {
		s.failRun(ctx, run, ErrTaskRuntimeStartFailed, err.Error())
		s.cleanupWorkspace(run.TaskRunID, workspace)
		return
	}
	hostCfg := ProcessHostConfig{
		Generation:   run.Generation,
		InstanceID:   instanceID,
		TaskRunID:    run.TaskRunID,
		ExtensionID:  run.ExtensionID,
		ModuleID:     run.ModuleID,
		DefHash:      def.DefinitionHash,
		NodePath:     nodeEnv.NodeBinary,
		HostPath:     hostArtifact.EntryPath,
		WorkDir:      workspace,
		EntryPath:    entryPath,
		EntryHash:    def.EntryHash,
		BundleRoot:   bundleRoot,
		BundleHash:   def.BundleHash,
		NativeLimits: taskProcessLimits(def),
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
		if err := host.Cancel(context.WithoutCancel(taskCtx), "任务启动前状态已变化"); err != nil {
			log.Printf("[original-error] %v", err)
		}
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
			if err := s.markTaskRecoveryUnknown(context.WithoutCancel(ctx), latest); err != nil {
				log.Printf("[original-error] %v", err)
			}
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
		if err := s.prepareOwnedTargetPermissions(ctx, run, def, prepared, run.Input); err != nil {
			return TaskExecutionOutcome{Status: RunStatusRecoveryRequired, ErrorCode: string(ErrTaskPermissionDenied), ErrorMessage: err.Error()}
		}
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
	unlock := s.lockTaskOwner(run.TaskRunID)
	defer unlock()
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
	if outcome.Status == RunStatusRunning && current.Status != RunStatusStarting {
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
	if err != nil || live == nil || !taskOwnerExecutionActive(live.Status) {
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
		if live.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice {
			writer, ok := s.store.(RemoteLeaseWriteStore)
			if !ok {
				return NewTaskError(ErrTaskExecutionAttemptInvalid, "remote progress requires atomic lease write fencing")
			}
			written, err := writer.PutRemoteProgressCAS(txCtx, live, seq, progJSON)
			if err != nil {
				return err
			}
			if !written {
				return NewTaskError(ErrTaskExecutionAttemptInvalid, "remote progress lease expired or taken over")
			}
			return nil
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
	if err != nil {
		return err
	}

	if err := validateCheckpointPayloadVersion(payload, version); err != nil {
		return err
	}
	actualHash := hashBytes(payload)
	if hash != "" && hash != actualHash {
		return NewTaskError(ErrTaskCheckpointHashMismatch, "检查点摘要不匹配")
	}
	if previous != nil {

		if current.CheckpointID == nil || *current.CheckpointID != previous.CheckpointID {
			return NewTaskError(ErrTaskCheckpointIncompatible, "最新检查点与任务已确认的引用不一致")
		}
		if previous.Version == version {
			if previous.TaskRunID != current.TaskRunID || previous.InputHash != current.InputHash ||
				previous.DefinitionHash != def.DefinitionHash || previous.PayloadHash != actualHash ||
				previous.CheckpointID == "" {
				return NewTaskError(ErrTaskExecutionAttemptInvalid, "检查点版本已失效或已确认不同内容")
			}

			if _, owned := coordination.FromContext(ctx); owned {
				if current.ExecutionAttemptID == "" || current.Generation < 1 ||
					previous.CheckpointID != ownedStableCheckpointID(current, version) ||
					s.config.OwnedCheckpoints == nil {
					return NewTaskError(ErrTaskCheckpointIncompatible, "重复检查点缺少原执行身份或所有者端口")
				}
				verified, readErr := s.config.OwnedCheckpoints.Checkpoint(ctx, current, previous)
				if readErr != nil || verified == nil || string(verified.Payload) != string(payload) ||
					verified.PayloadHash != actualHash {
					return NewTaskError(ErrTaskCheckpointIncompatible, "所有者检查点重放正文无法验证")
				}
			} else if string(previous.Payload) != string(payload) || !json.Valid(previous.Payload) {
				return NewTaskError(ErrTaskCheckpointIncompatible, "本地检查点重放正文无法验证")
			}
			if err := coordination.ValidateCurrent(ctx); err != nil {
				return err
			}
			run.CheckpointID = current.CheckpointID
			run.Revision = current.Revision
			return nil
		}
		if version <= previous.Version || previous.Version == int64(^uint64(0)>>1) || version != previous.Version+1 {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "检查点版本不能倒退或跳号")
		}

		previousVerified, verifyErr := s.readTaskCheckpoint(ctx, current)
		if verifyErr != nil {
			return NewTaskError(ErrTaskCheckpointIncompatible, "前序检查点所有者验证失败: "+verifyErr.Error())
		}
		_, owned := coordination.FromContext(ctx)
		if err := validateRecoveryCheckpoint(current, def, previousVerified, owned); err != nil {
			return err
		}
		if previousVerified == nil || previousVerified.Version != previous.Version ||
			previousVerified.CheckpointID != previous.CheckpointID {
			return NewTaskError(ErrTaskCheckpointIncompatible, "前序检查点版本与任务引用不一致")
		}
	} else if current.CheckpointID != nil {
		return NewTaskError(ErrTaskCheckpointIncompatible, "任务已有检查点引用，但检查点存储缺失")
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

	if _, owned := coordination.FromContext(ctx); owned {

		if current.ExecutionAttemptID == "" || current.Generation < 1 {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "所有者检查点缺少原执行身份")
		}
		cp.CheckpointID = ownedStableCheckpointID(current, version)
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
		var ok bool
		var err error
		if current.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice {
			writer, available := s.store.(RemoteLeaseTaskMutationStore)
			if !available {
				return NewTaskError(ErrTaskExecutionAttemptInvalid, "remote checkpoint requires atomic task lease fencing")
			}
			ok, err = writer.UpdateRemoteTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		} else {
			ok, err = s.store.UpdateTaskRunCAS(txCtx, next, current.Status, current.Generation, current.Revision)
		}
		if err != nil {
			return err
		}
		if !ok {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "检查点保存前任务已变化或租约失效")
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
			if err := s.markTaskRecoveryUnknown(context.WithoutCancel(ctx), current); err != nil {
				log.Printf("[original-error] %v", err)
			}
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
		cp, checkpointErr := s.readTaskCheckpoint(ctx, next)
		switch {
		case checkpointErr != nil:

			next.Status = RunStatusManualIntervention
			eventType = TaskEventFailed
			errMsg = "task checkpoint could not be verified after crash: " + checkpointErr.Error()
			next.ErrorMessage = &errMsg
		case cp != nil && next.CheckpointID != nil && validateRecoveryCheckpoint(next, def, cp, false) == nil:
			next.Status = RunStatusRecoveryRequired
			eventType = TaskEventRecoveryRequired
		case cp != nil || next.CheckpointID != nil:
			next.Status = RunStatusManualIntervention
			eventType = TaskEventFailed
			errMsg = "task checkpoint reference or contents are incompatible after crash"
			next.ErrorMessage = &errMsg
		default:
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
			if err := s.markTaskRecoveryUnknown(context.WithoutCancel(ctx), current); err != nil {
				log.Printf("[original-error] %v", err)
			}
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
	if err != nil || current == nil {
		return NewTaskError(ErrTaskNotFound, "任务不存在")
	}
	guarded, finish, err := s.restoreTaskAuthority(ctx, current)
	if err != nil {
		return err
	}
	defer finish()
	ctx = guarded

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
		if current.Status == RunStatusPaused && placement == TaskExecutionPlacementDevice {
			return s.cancelConfirmedPausedRemote(ctx, current, reason, now)
		}
		if s.remoteExecutor == nil {
			return NewTaskError(ErrRemoteTaskExecutorUnavailable, "remote cancellation not available")
		}
		if !remoteCancellationAllowed(current.Status) {
			return NewTaskError(ErrTaskNotCancelable, "remote task is not cancellable: "+string(current.Status))
		}

		if current.Status != RunStatusCancelling {
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
		if latest == nil {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "remote cancellation result unavailable")
		}
		if latest.Generation != current.Generation || latest.ExecutionAttemptID != current.ExecutionAttemptID || latest.TaskRunID != current.TaskRunID {
			return NewTaskError(ErrTaskExecutionAttemptInvalid, "remote cancellation execution attempt changed")
		}
		if cancellationConfirmedForAttempt(current, latest) {
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
		if err := host.Cancel(ctx, reason); err != nil {
			log.Printf("[original-error] %v", err)
		}
	}

	return nil
}

func (s *TaskRuntimeService) Retry(ctx context.Context, taskRunID string) (*TaskRun, error) {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil || run == nil {
		return nil, NewTaskError(ErrTaskNotFound, "任务不存在")
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
	if run.DeadlineAt != nil && !run.DeadlineAt.After(time.Now().UTC()) {
		return nil, NewTaskError(ErrTaskNotRetryable, "任务截止时间已过，不得重新排队")
	}
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
	if owned {
		requirements, err := sourceTaskPermissionRequirements(def)
		if err != nil {
			return nil, err
		}
		for _, permission := range requirements {
			if permission.PermissionID == "service.tool.execute" || permission.PermissionID == "event.emit" {
				return nil, NewTaskError(ErrTaskNotRetryable, "带有外部副作用权限的设备任务不得以新的TaskRunID重试，请在原任务检查点完成效果对账后恢复")
			}
		}
	}

	if run.Attempt >= run.MaxAttempts {
		return nil, NewTaskError(ErrTaskNotRetryable, "max attempts exceeded")
	}

	existing, err := s.existingRetry(ctx, run)
	if err != nil || existing != nil {
		return existing, err
	}

	now := time.Now().UTC()
	newRun := &TaskRun{
		TaskRunID:             retryTaskRunID(run.TaskRunID),
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
			SourceTaskDefinitionID: run.ExecutionTarget.SourceTaskDefinitionID,
			ProviderID:             run.ExecutionTarget.ProviderID,
			ProviderInstanceID:     run.ExecutionTarget.ProviderInstanceID,
			SpaceID:                run.ExecutionTarget.SpaceID,
			DeviceID:               run.ExecutionTarget.DeviceID,
			RuntimeID:              run.ExecutionTarget.RuntimeID,
			RuntimeInstanceID:      run.ExecutionTarget.RuntimeInstanceID,
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

	created := false
	if err := s.withManagementTaskTx(ctx, func(txCtx context.Context) error {
		creator, supported := s.store.(RetryTaskCreationStore)
		if !supported {
			return NewTaskError(ErrTaskDependencyUnavailable, "重试需要持久存储原子创建能力")
		}
		inserted, err := creator.CreateRetryTaskRun(txCtx, newRun)
		if err != nil {
			return fmt.Errorf("task_runtime: persist retry run: %w", err)
		}
		if !inserted {
			return nil
		}
		if err := s.store.EnqueueTask(txCtx, queueEntry); err != nil {
			return fmt.Errorf("task_runtime: enqueue retry: %w", err)
		}
		if err := s.publishTaskEvent(txCtx, TaskEventQueued, newRun, "", ""); err != nil {
			return err
		}
		created = true
		return nil
	}); err != nil {
		return nil, err
	}
	if !created {
		existing, err := s.existingRetry(ctx, run)
		if err != nil {
			return nil, err
		}
		if existing == nil {
			return nil, NewTaskError(ErrTaskStaleWrite, "重试身份已占用但无法读取权威任务")
		}
		return existing, nil
	}
	go s.tryDispatch()
	return newRun, nil
}

func (s *TaskRuntimeService) Recover(ctx context.Context, taskRunID string) (*TaskRun, error) {
	run, err := s.store.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return nil, NewTaskError(ErrTaskNotFound, err.Error())
	}
	if run == nil {
		return nil, NewTaskError(ErrTaskNotFound, "任务不存在")
	}
	if run.Status != RunStatusRecoveryRequired && run.Status != RunStatusManualIntervention {
		return nil, NewTaskError(ErrTaskStateTransitionInvalid, "task not in recovery state")
	}
	s.mu.RLock()
	host := s.activeHosts[taskRunID]
	s.mu.RUnlock()
	if host != nil {
		return nil, NewTaskError(ErrTaskPauseInProgress, "旧任务进程尚未退出，不能开启新的执行代次")
	}
	if _, active := s.sourceHosts.Load(taskRunID); active {
		return nil, NewTaskError(ErrTaskPauseInProgress, "旧设备任务进程仍在运行，不能开启新的执行代次")
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

	if owned {
		inspector, ok := s.config.OwnedOutcomes.(interface {
			InspectOutcome(context.Context, *TaskRun) (*CommittedTaskOutcome, error)
		})
		if !ok {
			return nil, NewTaskError(ErrTaskDependencyUnavailable, "所有者终态对账端口未就绪")
		}
		committed, inspectErr := inspector.InspectOutcome(ctx, run)
		if inspectErr != nil {
			return nil, inspectErr
		}
		if committed != nil {
			return s.finalizeCommittedOwnerOutcome(ctx, run, committed)
		}
	}

	if owned {
		mayCallNative, permissionErr := taskMayCallNativeEffects(def)
		if permissionErr != nil {
			return nil, permissionErr
		}
		if mayCallNative {
			inspector, ok := s.config.OwnedHost.(interface {
				InspectPendingNativeEffects(context.Context, *TaskRun) ([]string, error)
			})
			if !ok || inspector == nil {
				return nil, NewTaskError(ErrTaskDependencyUnavailable, "Owner Native执行端口缺少可信未决效果核查能力")
			}
			pending, inspectionErr := inspector.InspectPendingNativeEffects(ctx, run)
			if inspectionErr != nil {
				return nil, inspectionErr
			}
			if len(pending) > 0 {
				return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "Owner仍存在未确认Native工具效果，必须先对账，禁止开启新执行代次")
			}
		}
	}

	if run.DeadlineAt != nil && !run.DeadlineAt.After(time.Now().UTC()) {
		return nil, NewTaskError(ErrTaskResumeIncompatible, "任务执行期限已过")
	}
	if run.EffectiveExecutionPlacement() == TaskExecutionPlacementCloud {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "远端云任务缺少可信停止证明，不能直接重派")
	}
	if run.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice &&
		(run.Generation < 1 || run.ExecutionAttemptID == "" || run.LeaseID == "" || run.ExecutionTarget.RuntimeSessionID == "" || run.ExecutionTarget.ConnectionGeneration < 1) {
		return nil, NewTaskError(ErrTaskExecutionAttemptInvalid, "设备历史执行缺少完整原代次身份，禁止直接重派")
	}
	if run.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice && run.LeaseExpiresAt != nil && run.LeaseExpiresAt.After(time.Now().UTC()) {
		return nil, NewTaskError(ErrTaskPauseInProgress, "旧设备执行租约仍在有效期内，禁止开启新代次")
	}
	cp, err := s.readTaskCheckpoint(ctx, run)
	if err != nil {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "恢复检查点读取或所有者验证失败: "+err.Error())
	}

	if cp != nil || run.CheckpointID != nil || !owned {
		if err := validateRecoveryCheckpoint(run, def, cp, owned); err != nil {
			return nil, err
		}
	}

	if owned {
		run, cp, err = s.reconcilePendingOwnerCheckpoint(ctx, run, def, cp)
		if err != nil {
			return nil, err
		}
	}
	if err := validateRecoveryCheckpoint(run, def, cp, owned); err != nil {
		return nil, err
	}
	if cp == nil && (def.Recoverability != RestartableFromBeginning || (def.Idempotency != Idempotent && !(def.Idempotency == "" && def.Idempotent))) {
		return nil, NewTaskError(ErrTaskCheckpointIncompatible, "缺少检查点的任务只有在显式声明可从头恢复且具有幂等性时才能重试")
	}

	if run.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice && run.ExecutionAttemptID != "" {
		if err := s.confirmRecoveryStop(ctx, run, def, cp, owned); err != nil {
			return nil, err
		}
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}

	resumedRun := CloneTaskRun(run)
	if cp != nil {
		cpID := cp.CheckpointID
		resumedRun.CheckpointID = &cpID
	}
	previousStatus := run.Status
	resumedRun.Status = RunStatusQueued
	now := time.Now().UTC()
	resumedRun.QueuedAt = &now
	resumedRun.Generation++
	resumedRun.Revision = NextRevision(run.Revision)
	if resumedRun.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice {
		resumedRun.ClearTransientConnectionBinding()
	}
	resumedRun.ExecutionAttemptID = ""
	resumedRun.RuntimeInstanceID = nil
	resumedRun.LeaseID = ""
	resumedRun.LeaseExpiresAt, resumedRun.LastHeartbeatAt = nil, nil

	if err := s.withManagementTaskTx(ctx, func(txCtx context.Context) error {
		if owned {
			inspector, ok := s.config.OwnedCheckpoints.(pendingOwnerCheckpointInspector)
			if !ok {
				return NewTaskError(ErrTaskDependencyUnavailable, "Owner检查点恢复端口不支持最终入队核验")
			}
			if err := verifyOwnerRecoveryQueuePosition(txCtx, inspector, run, def, cp); err != nil {
				return err
			}
		}
		ok, casErr := s.store.UpdateTaskRunCAS(txCtx, resumedRun, previousStatus, run.Generation, run.Revision)
		if casErr != nil {
			return fmt.Errorf("task_runtime: recover cas: %w", casErr)
		}
		if !ok {
			return NewTaskError(ErrTaskPauseInProgress, "concurrent state change, retry recover")
		}
		if err := s.queue.Enqueue(txCtx, resumedRun); err != nil {
			return err
		}
		return s.publishTaskEvent(txCtx, TaskEventQueued, resumedRun, "", "")
	}); err != nil {
		return nil, err
	}

	go s.tryDispatch()
	return resumedRun, nil
}

func (s *TaskRuntimeService) confirmRecoveryStop(ctx context.Context, run *TaskRun, def *TaskDefinition, checkpoint *TaskCheckpoint, owned bool) error {
	owner, hasOwner := coordination.FromContext(ctx)
	if !owned || !hasOwner || !owner.Coordinated || run == nil || def == nil ||
		owner.SpaceID != run.ExecutionTarget.SpaceID.String() || owner.TargetDeviceID != run.ExecutionTarget.DeviceID.String() {
		return NewTaskError(ErrTaskScopeDenied, "设备任务恢复必须绑定原所有者和设备执行范围")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}

	allowReplay := owned && checkpoint != nil && run.CheckpointID != nil && *run.CheckpointID == checkpoint.CheckpointID &&
		def.Recoverability == CheckpointRecoverable && def.Checkpoint &&
		(def.Idempotency == Idempotent || (def.Idempotency == "" && def.Idempotent))
	if !allowReplay {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "设备任务缺少 Owner 确认检查点和显式幂等恢复保证，禁止重新执行")
	}
	verifier, supported := s.remoteExecutor.(interface {
		ConfirmStoppedAttempt(context.Context, *TaskRun) bool
	})
	if supported && verifier.ConfirmStoppedAttempt(ctx, run) {
		return nil
	}
	inspector, supported := s.remoteExecutor.(interface {
		InspectExecutionJournal(context.Context, *TaskRun) (executionjournal.Inspection, error)
	})
	if !supported {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "原设备执行缺少可验证的停止证明，不能开启新执行代次")
	}
	inspection, err := inspector.InspectExecutionJournal(ctx, run)
	if err != nil {
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "设备历史执行状态无法可信核对，禁止恢复")
	}
	switch inspection.State {
	case "stopped_performed":
		return nil
	case "stopped_unknown":
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "设备已停止但副作用结果不明，必须先对账")
	case "performed", "completed":
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "设备已有执行返回但缺少可信停止和 Owner 结果确认")
	case "started":
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "设备历史执行仍可能在运行，禁止重复派发")
	case "absent":
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "设备未发现历史记录不等于任务未产生副作用")
	default:
		return NewTaskError(ErrTaskExecutionAttemptInvalid, "设备返回未知执行状态，禁止恢复")
	}
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
	if run == nil {
		return NewTaskError(ErrTaskNotFound, "恢复任务不存在")
	}
	if run.ScopeSnapshotID != "" {
		if run.Status == RunStatusPaused && run.PausedAt != nil && run.CheckpointID != nil && *run.CheckpointID != "" && run.Generation > 0 && run.ExecutionAttemptID != "" && run.LeaseID != "" && run.EffectiveExecutionPlacement() == TaskExecutionPlacementDevice && run.ExecutionTarget.RuntimeSessionID != "" && run.ExecutionTarget.ConnectionGeneration > 0 {
			return nil
		}
		return s.markTaskRecoveryUnknown(ctx, run)
	}
	def, err := s.store.GetTaskDefinition(ctx, run.TaskDefinitionID)
	if err != nil || def == nil {
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

	var cp *TaskCheckpoint
	if recoverability == CheckpointRecoverable || run.CheckpointID != nil || run.Status == RunStatusPaused {
		cp, err = s.readTaskCheckpoint(ctx, run)
		if err != nil {
			return fmt.Errorf("task_runtime: startup checkpoint read %s: %w", run.TaskRunID, err)
		}
	}
	checkpointValid := cp != nil && validateRecoveryCheckpoint(run, def, cp, false) == nil
	if run.Status == RunStatusPaused && run.PausedAt != nil && run.EffectiveExecutionPlacement() == TaskExecutionPlacementLocal && checkpointValid && run.CheckpointID != nil {

		return nil
	}

	var enqueueNeeded bool
	switch recoverability {
	case CheckpointRecoverable:
		if checkpointValid {
			cpID := cp.CheckpointID
			run.CheckpointID = &cpID
			run.Status = RunStatusRecoveryRequired
		} else {
			run.Status = RunStatusManualIntervention
		}
	case RestartableFromBeginning:

		if run.EffectiveExecutionPlacement() != TaskExecutionPlacementLocal {
			run.Status = RunStatusRecoveryRequired
		} else if idempotency == Idempotent && run.CheckpointID == nil {
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

func (s *TaskRuntimeService) taskWorkspacePath(taskRunID string) (string, error) {
	if len(taskRunID) == 0 || len(taskRunID) > 128 || taskRunID == "." || taskRunID == ".." {
		return "", NewTaskError(ErrTaskInputInvalid, "任务工作目录标识无效")
	}
	for _, c := range taskRunID {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-' {
			continue
		}
		return "", NewTaskError(ErrTaskInputInvalid, "任务工作目录标识包含非法字符")
	}
	base, err := hostWorkspaceRoot(runtimeprofile.CurrentProcessProfile(), s.config.WorkspaceRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "task-workspace-"+taskRunID), nil
}

func (s *TaskRuntimeService) validateTaskWorkspaceRoot(create bool) error {
	root, err := hostWorkspaceRoot(runtimeprofile.CurrentProcessProfile(), s.config.WorkspaceRoot)
	if err != nil {
		return err
	}
	strict := runtimeprofile.CurrentProcessProfile() == runtimeprofile.ProfileCloudCore
	if strict {

		if err := verifyExistingWorkspaceAncestors(root); err != nil {
			return err
		}
	}
	if create {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
	}
	return verifyWorkspaceRoot(root, strict)
}

func (s *TaskRuntimeService) cleanupWorkspace(taskRunID, workspace string) {
	expected, err := s.taskWorkspacePath(taskRunID)
	if err != nil || workspace != expected {
		log.Printf("task_runtime: refuse unsafe workspace cleanup for %q", taskRunID)
		return
	}
	if err := s.validateTaskWorkspaceRoot(false); err != nil {
		log.Printf("task_runtime: refuse workspace cleanup through invalid root for %q: %v", taskRunID, err)
		return
	}

	info, err := os.Lstat(workspace)
	if os.IsNotExist(err) {
		return
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		log.Printf("task_runtime: refuse cleanup of non-private workspace %q: %v", workspace, err)
		return
	}
	if err := os.RemoveAll(workspace); err != nil {
		log.Printf("task_runtime: cleanup workspace failed for %s: %v", taskRunID, err)
	}
}

func (s *TaskRuntimeService) createTaskWorkspace(taskRunID string) (string, error) {
	workspace, err := s.taskWorkspacePath(taskRunID)
	if err != nil {
		return "", err
	}
	if err := s.validateTaskWorkspaceRoot(true); err != nil {
		return "", err
	}
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		return "", err
	}

	info, err := os.Lstat(workspace)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
		return "", NewTaskError(ErrTaskScopeDenied, "任务工作目录不是私有真实目录")
	}
	return workspace, nil
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func strPtr(s string) *string { return &s }

func ptrTime(t time.Time) *time.Time { return &t }

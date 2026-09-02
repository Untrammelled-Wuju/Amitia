package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type StepHandler interface {
	Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error)
}

type stepHandlerFunc func(context.Context, WorkflowNode, json.RawMessage) (json.RawMessage, error)

func (f stepHandlerFunc) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	return f(ctx, node, input)
}

func wrapPostconditionHandler(base StepHandler, nodeID string, expr *WorkflowExpression, workflowInput json.RawMessage, runtime map[string]any, outputs func() map[string]json.RawMessage) StepHandler {
	if base == nil || expr == nil {
		return base
	}
	var inputMap map[string]any
	_ = json.Unmarshal(workflowInput, &inputMap)
	if inputMap == nil {
		inputMap = make(map[string]any)
	}
	return stepHandlerFunc(func(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
		output, err := base.Execute(ctx, node, input)
		if err != nil {
			return output, err
		}
		snapshot := make(map[string]json.RawMessage)
		if outputs != nil {
			for key, value := range outputs() {
				snapshot[key] = value
			}
		}
		snapshot[nodeID] = append(json.RawMessage(nil), output...)
		ok, evalErr := EvaluateExpression(expr, ExpressionEvalConfig{Input: inputMap, Runtime: runtime, Outputs: snapshot})
		if evalErr != nil {
			return output, fmt.Errorf("workflow postcondition evaluation failed for node %s: %w", nodeID, evalErr)
		}
		if !ok {
			return output, fmt.Errorf("workflow postcondition failed for node %s", nodeID)
		}
		return output, nil
	})
}

type WorkflowExecutor struct {
	registry      *WorkflowRegistry
	handlers      map[string]StepHandler
	checkpoint    CheckpointStore
	compensation  *CompensationManager
	retryMax      int
	runStore      RunStore
	guard         StepGuard
	activeMu      sync.Mutex
	active        map[string]context.CancelFunc
	pauseMu       sync.Mutex
	pauseControls map[string]*WorkflowExecutionControl
	remoteMu      sync.RWMutex
	remoteRunner  RemoteWorkflowRunner
	runEventMu    sync.RWMutex
	runEventSink  WorkflowRunLifecycleSink
	concurrencyMu sync.Mutex
	revisionMu    sync.RWMutex
	revisionBind  WorkflowRevisionBinder
}

// WorkflowRevisionBinder resolves (and, when necessary, creates/promotes) the
// immutable published revision associated with a definition before a new run
// is persisted. Keeping this as a callback avoids coupling the workflow core
// to a concrete persistence implementation while making every entry point
// (HTTP, trigger, subworkflow, schedule, UI action, device mesh) bind the same
// revision semantics.
type WorkflowRevisionBinder func(context.Context, string, WorkflowDefinition) (string, error)

type WorkflowRunLifecycleEvent struct {
	Type           string
	WorkflowID     string
	ExecutionID    string
	InstallationID string
	UserID         string
	DeviceID       string
	Status         RunStatus
	Generation     int64
	Error          string
	Timestamp      time.Time
}

type WorkflowRunLifecycleSink func(context.Context, WorkflowRunLifecycleEvent)

type WorkflowExecutionControl struct {
	executionID    string
	pauseRequested chan struct{}
	pauseSignal    chan struct{}
	pauseOnce      sync.Once
	paused         chan struct{}
}

type StepGuard interface {
	Check(ctx context.Context, definition WorkflowDefinition, node WorkflowNode, execution ExecutionContext) error
}

type ExecuteRequest struct {
	WorkflowID string
	Input      json.RawMessage
	Context    ExecutionContext
	Options    ExecutionOptions
}

type ExecutionContext struct {
	UserID             string              `json:"userId,omitempty"`
	WorkflowID         string              `json:"workflowId,omitempty"`
	InstallationID     string              `json:"installationId,omitempty"`
	DeviceID           string              `json:"deviceId,omitempty"`
	CallStack          []WorkflowCallFrame `json:"callStack,omitempty"`
	RootID             string              `json:"rootId,omitempty"`
	ExtensionID        string              `json:"extensionId,omitempty"`
	CharacterID        string              `json:"characterId,omitempty"`
	ConversationID     string              `json:"conversationId,omitempty"`
	OperationID        string              `json:"operationId,omitempty"`
	InvocationID       string              `json:"invocationId,omitempty"`
	ScopeSnapshotID    string              `json:"scopeSnapshotId,omitempty"`
	PermissionSnapID   string              `json:"permissionSnapshotId,omitempty"`
	ExecutionOptions   ExecutionOptions    `json:"executionOptions,omitempty"`
	Generation         int64               `json:"generation,omitempty"`
	NodeID             string              `json:"nodeId,omitempty"`
	LogicalAttempt     int                 `json:"logicalAttempt,omitempty"`
	FencingToken       int64               `json:"fencingToken,omitempty"`
	ModuleID           string              `json:"moduleId,omitempty"`
	ScheduleID         string              `json:"scheduleId,omitempty"`
	TriggerID          string              `json:"triggerId,omitempty"`
	TraceID            string              `json:"traceId,omitempty"`
	IdempotencyKey     string              `json:"idempotencyKey,omitempty"`
	RevisionID         string              `json:"revisionId,omitempty"`
	DefinitionHash     string              `json:"definitionHash,omitempty"`
	DefinitionSnapshot json.RawMessage     `json:"definitionSnapshot,omitempty"`
	Depth              int                 `json:"depth,omitempty"`
	Recovery           bool                `json:"recovery,omitempty"`
}

type ExecuteResult struct {
	ExecutionID           string               `json:"executionId"`
	WorkflowID            string               `json:"workflowId"`
	Status                RunStatus            `json:"status"`
	Accepted              bool                 `json:"accepted"`
	Output                json.RawMessage      `json:"output,omitempty"`
	Steps                 []StepResult         `json:"steps,omitempty"`
	Success               bool                 `json:"success"`
	Error                 string               `json:"error,omitempty"`
	Duration              time.Duration        `json:"duration"`
	CompensationResults   []CompensationResult `json:"compensationResults,omitempty"`
	ExecutionMode         ExecutionMode        `json:"executionMode,omitempty"`
	RequiredConfirmations []string             `json:"requiredConfirmations,omitempty"`
}

type StepResult struct {
	NodeID   string          `json:"nodeId"`
	Status   string          `json:"status"`
	Output   json.RawMessage `json:"output,omitempty"`
	Error    string          `json:"error,omitempty"`
	Duration time.Duration   `json:"duration"`
	Attempt  int             `json:"attempt"`
}

func NewWorkflowExecutor(registry *WorkflowRegistry) *WorkflowExecutor {
	return &WorkflowExecutor{
		registry:      registry,
		handlers:      make(map[string]StepHandler),
		retryMax:      3,
		active:        make(map[string]context.CancelFunc),
		pauseControls: make(map[string]*WorkflowExecutionControl),
	}
}

func (e *WorkflowExecutor) RegisterHandler(nodeType string, handler StepHandler) {
	e.handlers[nodeType] = handler
}

func (e *WorkflowExecutor) SetCheckpointStore(store CheckpointStore) {
	e.checkpoint = store
}

func (e *WorkflowExecutor) SetCompensationManager(cm *CompensationManager) {
	e.compensation = cm
}

func (e *WorkflowExecutor) SetRetryMax(max int) {
	e.retryMax = max
}

func (e *WorkflowExecutor) SetRunStore(store RunStore) {
	e.runStore = store
}

func (e *WorkflowExecutor) SetRevisionBinder(binder WorkflowRevisionBinder) {
	e.revisionMu.Lock()
	e.revisionBind = binder
	e.revisionMu.Unlock()
}

func (e *WorkflowExecutor) bindRevision(ctx context.Context, userID string, def WorkflowDefinition) (string, error) {
	e.revisionMu.RLock()
	binder := e.revisionBind
	e.revisionMu.RUnlock()
	if binder == nil {
		return "", nil
	}
	return binder(ctx, strings.TrimSpace(userID), def)
}

func (e *WorkflowExecutor) RunStore() RunStore {
	return e.runStore
}

func (e *WorkflowExecutor) CheckpointStore() CheckpointStore {
	return e.checkpoint
}

func (e *WorkflowExecutor) SetStepGuard(guard StepGuard) {
	e.guard = guard
}

func (e *WorkflowExecutor) SetRunLifecycleSink(sink WorkflowRunLifecycleSink) {
	e.runEventMu.Lock()
	e.runEventSink = sink
	e.runEventMu.Unlock()
}

func (e *WorkflowExecutor) emitRunLifecycle(ctx context.Context, event WorkflowRunLifecycleEvent) {
	e.runEventMu.RLock()
	sink := e.runEventSink
	e.runEventMu.RUnlock()
	if sink == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	sink(ctx, event)
}

func runLifecycleEvent(kind string, run WorkflowRun) WorkflowRunLifecycleEvent {
	return WorkflowRunLifecycleEvent{
		Type:           kind,
		WorkflowID:     run.WorkflowID,
		ExecutionID:    run.ExecutionID,
		InstallationID: run.Context.InstallationID,
		UserID:         run.Context.UserID,
		DeviceID:       run.Context.DeviceID,
		Status:         run.Status,
		Generation:     run.Generation,
		Error:          run.Error,
		Timestamp:      run.UpdatedAt,
	}
}

func (e *WorkflowExecutor) SetRemoteWorkflowRunner(runner RemoteWorkflowRunner) {
	e.remoteMu.Lock()
	e.remoteRunner = runner
	e.remoteMu.Unlock()
}

func (e *WorkflowExecutor) RemoteWorkflowRunner() RemoteWorkflowRunner {
	e.remoteMu.RLock()
	defer e.remoteMu.RUnlock()
	return e.remoteRunner
}

func (e *WorkflowExecutor) Recover(ctx context.Context, limit int) error {
	if e.runStore == nil {
		return nil
	}
	runs, err := e.runStore.ListRecoverable(ctx, limit)
	if err != nil {
		return err
	}
	for i := range runs {
		run := runs[i]
		e.activeMu.Lock()
		_, locallyActive := e.active[run.ExecutionID]
		e.activeMu.Unlock()
		if locallyActive {
			continue
		}

		updated := run
		updated.Generation = run.Generation + 1
		updated.Context.Generation = updated.Generation
		updated.Context.Recovery = true
		updated.UpdatedAt = time.Now().UTC()

		switch run.Status {
		case RunStatusCompensating:
			// Compensation is a separate durable reverse phase. Never pass a
			// compensating run back through Execute(), otherwise a process restart
			// can re-run forward side effects before the Saga is resumed.
			updated.Status = RunStatusCompensating
			ok, casErr := e.runStore.UpdateStateCAS(ctx, updated, run.Status)
			if casErr != nil {
				return casErr
			}
			if !ok {
				continue
			}
			e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
			e.executeCompensationRecoveryAsync(updated)
		case RunStatusRunning, RunStatusResuming:
			updated.Status = RunStatusResuming
			ok, casErr := e.runStore.UpdateStateCAS(ctx, updated, run.Status)
			if casErr != nil {
				return casErr
			}
			if !ok {
				continue
			}
			e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
			e.executeRecoveryAsync(updated)
		}
	}
	return nil
}

func (e *WorkflowExecutor) Pause(ctx context.Context, executionID, reason string) (*WorkflowRun, error) {
	run, err := e.runStore.Get(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if run.Status == RunStatusPaused {
		return run, nil
	}
	if run.Status == RunStatusPausing {
		return run, nil
	}
	if !run.Status.IsActive() && run.Status != RunStatusCompensating {
		return nil, fmt.Errorf("workflow: cannot pause from status %s", run.Status)
	}

	now := time.Now().UTC()
	updated := *run
	updated.Status = RunStatusPausing
	updated.PauseReason = reason
	updated.PauseRequestedAt = &now
	updated.UpdatedAt = now

	ok, err := e.runStore.UpdateStateCAS(ctx, updated, run.Status)
	if err != nil {
		return nil, fmt.Errorf("workflow pause cas: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("workflow pause: concurrent state change")
	}

	ctrl := e.ensureExecutionControl(executionID)
	ctrl.pauseOnce.Do(func() { close(ctrl.pauseSignal) })
	select {
	case ctrl.pauseRequested <- struct{}{}:
	default:
	}
	e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))

	return &updated, nil
}

func (e *WorkflowExecutor) Resume(ctx context.Context, executionID string) (*WorkflowRun, error) {
	run, err := e.runStore.Get(ctx, executionID)
	if err != nil {
		return nil, err
	}
	if run.Status == RunStatusRunning || run.Status == RunStatusResuming {
		return run, nil
	}
	if run.Status != RunStatusPaused {
		return nil, fmt.Errorf("workflow: cannot resume from status %s", run.Status)
	}

	e.pauseMu.Lock()
	if ctrl, ok := e.pauseControls[executionID]; ok {
		select {
		case <-ctrl.paused:
		default:
		}
		delete(e.pauseControls, executionID)
	}
	e.pauseMu.Unlock()

	now := time.Now().UTC()
	updated := *run
	updated.Status = RunStatusResuming
	updated.Generation = run.Generation + 1
	updated.PausedAt = nil
	updated.Context.Generation = updated.Generation
	updated.Context.Recovery = true
	updated.UpdatedAt = now

	ok, err := e.runStore.UpdateStateCAS(ctx, updated, RunStatusPaused)
	if err != nil {
		return nil, fmt.Errorf("workflow resume cas: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("workflow resume: concurrent state change")
	}

	e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
	e.executeRecoveryAsync(updated)

	return &updated, nil
}

func (e *WorkflowExecutor) ensureExecutionControl(executionID string) *WorkflowExecutionControl {
	e.pauseMu.Lock()
	defer e.pauseMu.Unlock()
	if ctrl, ok := e.pauseControls[executionID]; ok && ctrl != nil {
		return ctrl
	}
	ctrl := &WorkflowExecutionControl{
		executionID:    executionID,
		pauseRequested: make(chan struct{}, 1),
		pauseSignal:    make(chan struct{}),
		paused:         make(chan struct{}),
	}
	e.pauseControls[executionID] = ctrl
	return ctrl
}

func (e *WorkflowExecutor) getExecutionControl(executionID string) *WorkflowExecutionControl {
	e.pauseMu.Lock()
	defer e.pauseMu.Unlock()
	return e.pauseControls[executionID]
}

func (e *WorkflowExecutor) removeExecutionControl(executionID string) {
	e.pauseMu.Lock()
	delete(e.pauseControls, executionID)
	e.pauseMu.Unlock()
}

func (e *WorkflowExecutor) finalisePaused(ctx context.Context, executionID string, gen int64) {
	e.pauseMu.Lock()
	ctrl, ok := e.pauseControls[executionID]
	e.pauseMu.Unlock()

	if !ok {
		return
	}

	run, err := e.runStore.Get(ctx, executionID)
	if err != nil || run == nil {
		return
	}
	if run.Status != RunStatusPausing || run.Generation != gen {
		return
	}

	now := time.Now().UTC()
	updated := *run
	updated.Status = RunStatusPaused
	updated.PausedAt = &now
	updated.UpdatedAt = now

	if ok, err := e.runStore.UpdateStateCAS(ctx, updated, RunStatusPausing); err == nil && ok {
		e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
		close(ctrl.paused)
	}
}

func (e *WorkflowExecutor) markWaitingDevice(ctx context.Context, executionID, deviceID string) error {
	if e.runStore == nil {
		return fmt.Errorf("workflow: waiting_device requires a durable run store")
	}
	run, err := e.runStore.Get(ctx, executionID)
	if err != nil {
		return err
	}
	if run == nil {
		return fmt.Errorf("workflow: execution %s not found while waiting for device", executionID)
	}
	now := time.Now().UTC()
	updated := *run
	updated.Status = RunStatusWaitingDevice
	updated.PauseReason = "waiting_device:" + strings.TrimSpace(deviceID)
	updated.UpdatedAt = now
	ok, err := e.runStore.UpdateStateCAS(context.WithoutCancel(ctx), updated, run.Status)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("workflow: waiting_device concurrent state change")
	}
	e.emitRunLifecycle(context.WithoutCancel(ctx), runLifecycleEvent("updated", updated))
	return nil
}

// ResumeWaitingDevice re-enters waiting runs on the same execution ID. Completed
// nodes are restored from checkpoints, while Generation is incremented so
// attempts after reconnect remain distinguishable from pre-disconnect attempts.
func (e *WorkflowExecutor) ResumeWaitingDevice(ctx context.Context, userID, deviceID string) (int, error) {
	store, ok := e.runStore.(WaitingDeviceRunStore)
	if !ok || store == nil {
		return 0, nil
	}
	runs, err := store.ListWaitingDevice(ctx, strings.TrimSpace(userID), strings.TrimSpace(deviceID), 100)
	if err != nil {
		return 0, err
	}
	resumed := 0
	for i := range runs {
		run := runs[i]
		if run.Status != RunStatusWaitingDevice {
			continue
		}
		now := time.Now().UTC()
		updated := run
		updated.Status = RunStatusResuming
		updated.Generation = run.Generation + 1
		updated.Context.Generation = updated.Generation
		updated.Context.Recovery = true
		updated.PauseReason = ""
		updated.UpdatedAt = now
		ok, casErr := e.runStore.UpdateStateCAS(ctx, updated, RunStatusWaitingDevice)
		if casErr != nil {
			return resumed, casErr
		}
		if !ok {
			continue
		}
		e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
		DefaultWorkflowReliabilityMetrics.Inc(MetricWorkflowDeviceReconnectTotal)
		resumed++
		e.executeRecoveryAsync(updated)
	}
	return resumed, nil
}

func (e *WorkflowExecutor) restorePausedWaitNode(ctx context.Context, executionID string, node WorkflowNode, recovery bool) WorkflowNode {
	if e == nil || !recovery || !strings.EqualFold(strings.TrimSpace(node.Type), "wait") {
		return node
	}
	store, ok := e.runStore.(StepProgressStore)
	if !ok || store == nil {
		return node
	}
	step, err := store.GetStep(ctx, executionID, node.ID)
	if err != nil || step == nil || step.Status != "paused" || len(step.Output) == 0 {
		return node
	}
	var progress waitPauseProgress
	if err := json.Unmarshal(step.Output, &progress); err != nil || progress.RemainingMS < 0 {
		return node
	}
	metadata := make(map[string]any, len(node.Runtime.Metadata)+1)
	for key, value := range node.Runtime.Metadata {
		metadata[key] = value
	}
	metadata["durationMs"] = float64(progress.RemainingMS)
	node.Runtime.Metadata = metadata
	return node
}

// executeRecoveryAsync re-enters a durable paused/waiting execution and makes
// sure failures that happen before Execute reaches RunStore.Start do not leave
// the persisted run permanently stuck in resuming.
func (e *WorkflowExecutor) executeRecoveryAsync(run WorkflowRun) {
	go func() {
		_, err := e.Execute(context.Background(), ExecuteRequest{
			WorkflowID: run.WorkflowID,
			Input:      run.Input,
			Context:    run.Context,
		})
		if err == nil || e.runStore == nil {
			return
		}

		ctx := context.Background()
		current, getErr := e.runStore.Get(ctx, run.ExecutionID)
		if getErr != nil || current == nil || current.Status != RunStatusResuming {
			return
		}
		now := time.Now().UTC()
		failed := *current
		failed.Status = RunStatusFailed
		failed.Error = err.Error()
		failed.FinishedAt = &now
		failed.UpdatedAt = now
		ok, casErr := e.runStore.UpdateStateCAS(ctx, failed, RunStatusResuming)
		if casErr != nil || !ok {
			return
		}
		if finishErr := e.runStore.Finish(ctx, failed); finishErr == nil {
			e.emitRunLifecycle(ctx, runLifecycleEvent("finished", failed))
		}
	}()
}

func (e *WorkflowExecutor) Cancel(executionID string) bool {
	e.activeMu.Lock()
	cancel, ok := e.active[executionID]
	e.activeMu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// CancelRun persists the cancellation intent before signalling the in-memory
// execution. This keeps cancel_request distinct from cancel_complete and lets a
// reaper finish an orphaned cancellation after a process/device failure.
func (e *WorkflowExecutor) CancelRun(ctx context.Context, executionID string) (bool, error) {
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return false, fmt.Errorf("workflow: execution id is required")
	}
	if e.runStore == nil {
		return e.Cancel(executionID), nil
	}

	run, err := e.runStore.Get(ctx, executionID)
	if err != nil {
		return false, err
	}
	if run.Status.IsTerminal() {
		return false, nil
	}
	if run.Status != RunStatusCancelRequested && run.Status != RunStatusCancelling {
		now := time.Now().UTC()
		requested := *run
		requested.Status = RunStatusCancelRequested
		requested.UpdatedAt = now
		ok, casErr := e.runStore.UpdateStateCAS(ctx, requested, run.Status)
		if casErr != nil {
			return false, casErr
		}
		if !ok {
			return false, fmt.Errorf("workflow cancel: concurrent state change")
		}
		run = &requested
		e.emitRunLifecycle(ctx, runLifecycleEvent("updated", requested))
	}

	if e.Cancel(executionID) {
		if run.Status == RunStatusCancelRequested {
			now := time.Now().UTC()
			cancelling := *run
			cancelling.Status = RunStatusCancelling
			cancelling.UpdatedAt = now
			if ok, casErr := e.runStore.UpdateStateCAS(ctx, cancelling, RunStatusCancelRequested); casErr == nil && ok {
				e.emitRunLifecycle(ctx, runLifecycleEvent("updated", cancelling))
			}
		}
		return true, nil
	}

	// No in-memory execution owns this run. It is safe to complete cancellation
	// immediately for paused/waiting/orphaned runs because no handler is active.
	now := time.Now().UTC()
	cancelled := *run
	cancelled.Status = RunStatusCancelled
	cancelled.Error = "cancelled"
	cancelled.FinishedAt = &now
	cancelled.UpdatedAt = now
	expected := run.Status
	if ok, casErr := e.runStore.UpdateStateCAS(ctx, cancelled, expected); casErr != nil || !ok {
		return ok, casErr
	}
	if err := e.runStore.Finish(ctx, cancelled); err != nil {
		return false, err
	}
	e.emitRunLifecycle(ctx, runLifecycleEvent("finished", cancelled))
	go e.drainQueuedWorkflow(context.Background(), cancelled.WorkflowID)
	return true, nil
}

func (e *WorkflowExecutor) concurrencyQueueRun(ctx context.Context, req ExecuteRequest, executionID string, at time.Time) (*ExecuteResult, error) {
	store, ok := e.runStore.(WorkflowConcurrencyStore)
	if !ok || store == nil {
		return nil, errors.New("workflow concurrency policy requires durable concurrency store")
	}
	run := WorkflowRun{
		ExecutionID: executionID,
		WorkflowID:  req.WorkflowID,
		Status:      RunStatusQueued,
		Input:       req.Input,
		Context:     req.Context,
		Attempt:     0,
		Generation:  req.Context.Generation,
		StartedAt:   at.UTC(),
		UpdatedAt:   at.UTC(),
	}
	if err := store.EnqueueWorkflowRun(ctx, run); err != nil {
		return nil, err
	}
	e.emitRunLifecycle(ctx, runLifecycleEvent("queued", run))
	return &ExecuteResult{
		ExecutionID: executionID,
		WorkflowID:  req.WorkflowID,
		Status:      RunStatusQueued,
		Accepted:    true,
		Success:     true,
	}, nil
}

func (e *WorkflowExecutor) concurrencyDropRun(ctx context.Context, req ExecuteRequest, executionID, reason string, at time.Time) (*ExecuteResult, error) {
	run := WorkflowRun{
		ExecutionID: executionID,
		WorkflowID:  req.WorkflowID,
		Status:      RunStatusDropped,
		Input:       req.Input,
		Error:       reason,
		Context:     req.Context,
		Attempt:     0,
		Generation:  req.Context.Generation,
		StartedAt:   at.UTC(),
		UpdatedAt:   at.UTC(),
	}
	if e.runStore != nil {
		// Start creates the durable execution row using the normal idempotency
		// path, then Finish immediately seals it as a terminal dropped run.
		if existing, created, err := e.runStore.Start(ctx, WorkflowRun{
			ExecutionID: executionID,
			WorkflowID:  req.WorkflowID,
			Status:      RunStatusRunning,
			Input:       req.Input,
			Context:     req.Context,
			Attempt:     1,
			Generation:  req.Context.Generation,
			StartedAt:   at.UTC(),
			UpdatedAt:   at.UTC(),
		}); err != nil {
			return nil, err
		} else if !created && existing != nil {
			return &ExecuteResult{ExecutionID: existing.ExecutionID, WorkflowID: existing.WorkflowID, Status: existing.Status, Accepted: true, Success: existing.Status == RunStatusSucceeded, Output: existing.Output, Steps: existing.Steps}, nil
		}
		finishedAt := at.UTC()
		run.FinishedAt = &finishedAt
		if err := e.runStore.Finish(ctx, run); err != nil {
			return nil, err
		}
	}
	e.emitRunLifecycle(ctx, runLifecycleEvent("finished", run))
	return &ExecuteResult{ExecutionID: executionID, WorkflowID: req.WorkflowID, Status: RunStatusDropped, Error: reason, Duration: 0}, nil
}

func (e *WorkflowExecutor) applyConcurrencyPolicy(ctx context.Context, wf WorkflowDefinition, req ExecuteRequest, executionID string, at time.Time) (*ExecuteResult, bool, error) {
	if req.Context.Recovery {
		return nil, false, nil
	}
	policy := wf.ConcurrencyPolicy.Normalize()
	if err := policy.Validate(); err != nil {
		return nil, false, err
	}
	if policy.Mode == WorkflowConcurrencyAllow {
		return nil, false, nil
	}
	store, ok := e.runStore.(WorkflowConcurrencyStore)
	if !ok || store == nil {
		return nil, false, errors.New("workflow concurrency policy requires durable concurrency store")
	}
	active, err := store.ListActiveWorkflowRuns(ctx, req.WorkflowID, 500)
	if err != nil {
		return nil, false, err
	}

	switch policy.Mode {
	case WorkflowConcurrencySingleton:
		if len(active) > 0 {
			result, err := e.concurrencyDropRun(ctx, req, executionID, "singleton workflow already has an active run", at)
			return result, true, err
		}
	case WorkflowConcurrencyDrop:
		if len(active) > 0 {
			result, err := e.concurrencyDropRun(ctx, req, executionID, "dropped by workflow concurrency policy", at)
			return result, true, err
		}
	case WorkflowConcurrencyQueue:
		if len(active) > 0 {
			result, err := e.concurrencyQueueRun(ctx, req, executionID, at)
			return result, true, err
		}
	case WorkflowConcurrencyMaxN:
		if len(active) >= policy.MaxN {
			result, err := e.concurrencyQueueRun(ctx, req, executionID, at)
			return result, true, err
		}
	case WorkflowConcurrencyReplace:
		if _, err := store.DropQueuedWorkflowRuns(ctx, req.WorkflowID, "replaced by newer workflow run", at); err != nil {
			return nil, false, err
		}
		if len(active) > 0 {
			for i := range active {
				if active[i].ExecutionID == executionID {
					continue
				}
				if _, cancelErr := e.CancelRun(ctx, active[i].ExecutionID); cancelErr != nil {
					return nil, false, cancelErr
				}
			}
			result, err := e.concurrencyQueueRun(ctx, req, executionID, at)
			return result, true, err
		}
	}
	return nil, false, nil
}

func (e *WorkflowExecutor) drainQueuedWorkflow(ctx context.Context, workflowID string) {
	e.concurrencyMu.Lock()
	defer e.concurrencyMu.Unlock()
	store, ok := e.runStore.(WorkflowConcurrencyStore)
	if !ok || store == nil || e.runStore == nil || e.registry == nil {
		return
	}
	wf, ok := e.registry.Get(workflowID)
	if !ok {
		return
	}
	policy := wf.ConcurrencyPolicy.Normalize()
	active, err := store.ListActiveWorkflowRuns(ctx, workflowID, 500)
	if err != nil {
		return
	}
	capacity := 1
	switch policy.Mode {
	case WorkflowConcurrencyAllow:
		capacity = 500
	case WorkflowConcurrencyMaxN:
		capacity = policy.MaxN
	default:
		capacity = 1
	}
	if capacity < 1 || len(active) >= capacity {
		return
	}
	queued, err := store.ListQueuedWorkflowRuns(ctx, workflowID, capacity-len(active))
	if err != nil {
		return
	}
	for i := range queued {
		run := queued[i]
		now := time.Now().UTC()
		promoted := run
		promoted.Status = RunStatusResuming
		promoted.Context.Recovery = true
		promoted.Context.Generation = promoted.Generation
		promoted.UpdatedAt = now
		ok, casErr := e.runStore.UpdateStateCAS(ctx, promoted, RunStatusQueued)
		if casErr != nil || !ok {
			continue
		}
		e.emitRunLifecycle(ctx, runLifecycleEvent("updated", promoted))
		e.executeRecoveryAsync(promoted)
	}
}

const (
	DefaultWorkflowRunHeartbeatInterval = 15 * time.Second
	DefaultWorkflowRunStaleAfter        = 90 * time.Second
	DefaultWorkflowWaitingDeviceTimeout = 24 * time.Hour
)

func (e *WorkflowExecutor) startRunHeartbeat(ctx context.Context, executionID string) func() {
	store, ok := e.runStore.(WorkflowRunHeartbeatStore)
	if !ok || store == nil || strings.TrimSpace(executionID) == "" {
		return func() {}
	}
	hbCtx, cancel := context.WithCancel(ctx)
	writeHeartbeat := func() {
		persistCtx, persistCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer persistCancel()
		_ = store.HeartbeatRun(persistCtx, executionID, time.Now().UTC())
	}
	writeHeartbeat()
	go func() {
		ticker := time.NewTicker(DefaultWorkflowRunHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				writeHeartbeat()
			}
		}
	}()
	return cancel
}

// ReapStuck resolves durable runs whose owner stopped heartbeating. It never
// steals an execution that is still active in this process. Running/resuming
// runs are recovered with a new generation, stale pause requests are completed,
// and orphaned cancellation requests terminate deterministically.
func (e *WorkflowExecutor) ReapStuck(ctx context.Context, staleAfter, waitingDeviceTimeout time.Duration, limit int) (int, error) {
	store, ok := e.runStore.(WorkflowRunHeartbeatStore)
	if !ok || store == nil || e.runStore == nil {
		return 0, nil
	}
	if staleAfter <= 0 {
		staleAfter = DefaultWorkflowRunStaleAfter
	}
	if waitingDeviceTimeout <= 0 {
		waitingDeviceTimeout = DefaultWorkflowWaitingDeviceTimeout
	}
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	runs, err := store.ListStuckRuns(ctx, now.Add(-staleAfter), limit)
	if err != nil {
		return 0, err
	}
	reaped := 0
	for i := range runs {
		run := runs[i]
		e.activeMu.Lock()
		_, locallyActive := e.active[run.ExecutionID]
		e.activeMu.Unlock()
		if locallyActive {
			_ = store.HeartbeatRun(context.WithoutCancel(ctx), run.ExecutionID, now)
			continue
		}

		switch run.Status {
		case RunStatusRunning, RunStatusResuming:
			updated := run
			updated.Status = RunStatusResuming
			updated.Generation = run.Generation + 1
			updated.Context.Generation = updated.Generation
			updated.Context.Recovery = true
			updated.UpdatedAt = now
			ok, casErr := e.runStore.UpdateStateCAS(ctx, updated, run.Status)
			if casErr != nil {
				return reaped, casErr
			}
			if !ok {
				continue
			}
			e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
			e.executeRecoveryAsync(updated)
			reaped++
		case RunStatusPausing:
			updated := run
			updated.Status = RunStatusPaused
			updated.PausedAt = &now
			updated.UpdatedAt = now
			ok, casErr := e.runStore.UpdateStateCAS(ctx, updated, RunStatusPausing)
			if casErr != nil {
				return reaped, casErr
			}
			if ok {
				e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
				reaped++
			}
		case RunStatusCancelRequested, RunStatusCancelling:
			updated := run
			updated.Status = RunStatusCancelTimeout
			updated.Error = "cancel timeout: execution owner stopped heartbeating"
			updated.FinishedAt = &now
			updated.UpdatedAt = now
			ok, casErr := e.runStore.UpdateStateCAS(ctx, updated, run.Status)
			if casErr != nil {
				return reaped, casErr
			}
			if !ok {
				continue
			}
			if finishErr := e.runStore.Finish(ctx, updated); finishErr != nil {
				return reaped, finishErr
			}
			e.emitRunLifecycle(ctx, runLifecycleEvent("finished", updated))
			e.drainQueuedWorkflow(context.WithoutCancel(ctx), updated.WorkflowID)
			reaped++
		case RunStatusWaitingDevice:
			if now.Sub(run.UpdatedAt) < waitingDeviceTimeout {
				continue
			}
			updated := run
			updated.Status = RunStatusFailed
			updated.Error = "waiting_device timeout"
			updated.FinishedAt = &now
			updated.UpdatedAt = now
			ok, casErr := e.runStore.UpdateStateCAS(ctx, updated, RunStatusWaitingDevice)
			if casErr != nil {
				return reaped, casErr
			}
			if !ok {
				continue
			}
			if finishErr := e.runStore.Finish(ctx, updated); finishErr != nil {
				return reaped, finishErr
			}
			e.emitRunLifecycle(ctx, runLifecycleEvent("finished", updated))
			e.drainQueuedWorkflow(context.WithoutCancel(ctx), updated.WorkflowID)
			reaped++
		case RunStatusCompensating:
			// Compensation is itself a durable execution phase. A dead owner must
			// not turn a recoverable Saga into an ordinary failed run. Fence the
			// old generation, persist the new recovery context and resume only the
			// reverse steps from their durable records.
			updated := run
			updated.Generation = run.Generation + 1
			updated.Context.Generation = updated.Generation
			updated.Context.Recovery = true
			updated.UpdatedAt = now
			ok, casErr := e.runStore.UpdateStateCAS(ctx, updated, RunStatusCompensating)
			if casErr != nil {
				return reaped, casErr
			}
			if !ok {
				continue
			}
			e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
			e.executeCompensationRecoveryAsync(updated)
			reaped++
		}
	}
	return reaped, nil
}

func (e *WorkflowExecutor) ConfirmControlledRun(ctx context.Context, executionID string, approved []string) (*WorkflowRun, []string, error) {
	if e == nil || e.runStore == nil {
		return nil, nil, fmt.Errorf("workflow: durable run store unavailable")
	}
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return nil, nil, fmt.Errorf("workflow: execution id is required")
	}
	run, err := e.runStore.Get(ctx, executionID)
	if err != nil {
		return nil, nil, err
	}
	if run.Status != RunStatusWaitingConfirmation {
		return run, nil, fmt.Errorf("workflow: run %s is not waiting for confirmation", executionID)
	}
	if len(run.Context.DefinitionSnapshot) == 0 {
		return run, nil, fmt.Errorf("workflow: controlled run is missing immutable definition snapshot")
	}
	var def WorkflowDefinition
	if err := json.Unmarshal(run.Context.DefinitionSnapshot, &def); err != nil {
		return run, nil, fmt.Errorf("workflow: restore controlled run definition: %w", err)
	}
	opts := run.Context.ExecutionOptions
	opts.ApprovedSideEffects = append(opts.ApprovedSideEffects, approved...)
	opts, err = opts.Normalize()
	if err != nil {
		return run, nil, err
	}
	if opts.Mode != ExecutionModeControlled {
		return run, nil, fmt.Errorf("workflow: run %s is not controlled_live", executionID)
	}
	run.Context.ExecutionOptions = opts
	missing := opts.MissingControlledApprovals(def.Nodes)
	now := time.Now().UTC()
	run.UpdatedAt = now
	if len(missing) > 0 {
		ok, updateErr := e.runStore.UpdateStateCAS(ctx, *run, RunStatusWaitingConfirmation)
		if updateErr != nil {
			return run, missing, updateErr
		}
		if !ok {
			return run, missing, fmt.Errorf("workflow: confirmation state changed concurrently")
		}
		return run, missing, nil
	}
	run.Generation++
	run.Context.Generation = run.Generation
	run.Context.Recovery = true
	run.Status = RunStatusResuming
	ok, err := e.runStore.UpdateStateCAS(ctx, *run, RunStatusWaitingConfirmation)
	if err != nil {
		return run, nil, err
	}
	if !ok {
		return run, nil, fmt.Errorf("workflow: confirmation state changed concurrently")
	}
	e.emitRunLifecycle(ctx, runLifecycleEvent("updated", *run))
	resume := *run
	go func() {
		_, _ = e.Execute(context.Background(), ExecuteRequest{
			WorkflowID: resume.WorkflowID,
			Input:      resume.Input,
			Context:    resume.Context,
			Options:    resume.Context.ExecutionOptions,
		})
	}()
	return run, nil, nil
}

func (e *WorkflowExecutor) Execute(ctx context.Context, req ExecuteRequest) (resultOut *ExecuteResult, errOut error) {
	start := time.Now()
	DefaultWorkflowReliabilityMetrics.Inc(MetricWorkflowRunTotal)
	defer func() {
		DefaultWorkflowReliabilityMetrics.Observe(MetricWorkflowRunLatencyMS, float64(time.Since(start).Microseconds())/1000.0)
		if errOut != nil || resultOut == nil {
			DefaultWorkflowReliabilityMetrics.Inc(MetricWorkflowRunFailedTotal)
			return
		}
		if resultOut.Success || resultOut.Status == RunStatusSucceeded {
			DefaultWorkflowReliabilityMetrics.Inc(MetricWorkflowRunSuccessTotal)
		} else if resultOut.Status != RunStatusPaused && resultOut.Status != RunStatusWaitingDevice && resultOut.Status != RunStatusWaitingConfirmation {
			DefaultWorkflowReliabilityMetrics.Inc(MetricWorkflowRunFailedTotal)
		}
		if resultOut.Status == RunStatusWaitingDevice {
			DefaultWorkflowReliabilityMetrics.Inc(MetricWorkflowDeviceWaitTotal)
		}
		for _, step := range resultOut.Steps {
			if step.Attempt > 1 {
				DefaultWorkflowReliabilityMetrics.Add(MetricWorkflowNodeRetryTotal, int64(step.Attempt-1))
			}
			if step.Attempt == 0 && step.Status == "succeeded" {
				DefaultWorkflowReliabilityMetrics.Inc(MetricWorkflowCheckpointResumeTotal)
			}
			if strings.Contains(strings.ToLower(step.Error), "timeout") || strings.Contains(strings.ToLower(step.Error), "deadline exceeded") {
				DefaultWorkflowReliabilityMetrics.Inc(MetricWorkflowNodeTimeoutTotal)
			}
		}
		if len(resultOut.CompensationResults) > 0 {
			DefaultWorkflowReliabilityMetrics.Add(MetricWorkflowCompensationTotal, int64(len(resultOut.CompensationResults)))
		}
	}()

	var wf WorkflowDefinition
	if req.Context.Recovery && len(req.Context.DefinitionSnapshot) > 0 {
		if err := json.Unmarshal(req.Context.DefinitionSnapshot, &wf); err != nil {
			return nil, fmt.Errorf("workflow: restore immutable definition snapshot: %w", err)
		}
		if wf.ID != req.WorkflowID {
			return nil, fmt.Errorf("workflow: definition snapshot id %q does not match requested workflow %q", wf.ID, req.WorkflowID)
		}
	} else {
		var ok bool
		wf, ok = e.registry.Get(req.WorkflowID)
		if !ok {
			return nil, ErrWorkflowNotFound
		}
	}
	currentFrame := WorkflowCallFrame{InstallationID: req.Context.InstallationID, WorkflowID: req.WorkflowID, DeviceID: req.Context.DeviceID}
	// Durable resume/recovery reuses the persisted execution context, which
	// already contains this workflow's frame. Do not append the same frame a
	// second time or a legitimate resume would be mistaken for recursion. New
	// nested invocations still append and validate normally.
	if !(req.Context.Recovery && len(req.Context.CallStack) > 0 && req.Context.CallStack[len(req.Context.CallStack)-1].key() == currentFrame.key()) {
		callStack, err := AppendWorkflowCallFrame(req.Context.CallStack, currentFrame)
		if err != nil {
			return nil, err
		}
		req.Context.CallStack = callStack
	}
	req.Context.WorkflowID = req.WorkflowID
	definitionHash := ComputeDefinitionHash(wf)
	if req.Context.Recovery && req.Context.DefinitionHash != "" && req.Context.DefinitionHash != definitionHash {
		return nil, fmt.Errorf("workflow: immutable definition hash mismatch during recovery: persisted=%s snapshot=%s", req.Context.DefinitionHash, definitionHash)
	}
	if len(req.Context.DefinitionSnapshot) == 0 {
		snapshot, snapshotErr := json.Marshal(wf)
		if snapshotErr != nil {
			return nil, fmt.Errorf("workflow: snapshot definition for run: %w", snapshotErr)
		}
		req.Context.DefinitionSnapshot = snapshot
	}
	var stackErr error
	ctx, stackErr = pushWorkflowCall(ctx, req.WorkflowID)
	if stackErr != nil {
		return nil, stackErr
	}
	var edgeConditionErr error
	wf, edgeConditionErr = MaterializeEdgeConditions(wf)
	if edgeConditionErr != nil {
		return nil, edgeConditionErr
	}
	if req.Context.UserID == "" && wf.Metadata != nil {
		if owner, ok := wf.Metadata["ownerUserId"].(string); ok {
			req.Context.UserID = strings.TrimSpace(owner)
		}
	}
	if req.Options.Mode == "" && req.Context.ExecutionOptions.Mode != "" {
		req.Options = req.Context.ExecutionOptions
	}
	normalizedOptions, optionsErr := req.Options.Normalize()
	if optionsErr != nil {
		return nil, optionsErr
	}
	req.Options = normalizedOptions
	req.Context.ExecutionOptions = normalizedOptions
	if !req.Context.Recovery && strings.TrimSpace(req.Context.RevisionID) == "" {
		revisionID, bindErr := e.bindRevision(ctx, req.Context.UserID, wf)
		if bindErr != nil {
			return nil, fmt.Errorf("workflow: bind immutable revision: %w", bindErr)
		}
		req.Context.RevisionID = strings.TrimSpace(revisionID)
	}
	req.Context.DefinitionHash = definitionHash

	if !wf.Enabled {
		return nil, ErrWorkflowDisabled
	}

	if err := ValidateDAG(wf.Nodes); err != nil {
		return nil, err
	}

	levels, err := ComputeLevels(wf.Nodes)
	if err != nil {
		return nil, err
	}

	totalNodes := 0
	for _, level := range levels {
		totalNodes += len(level)
	}

	if wf.Limits.MaxSteps > 0 && totalNodes > wf.Limits.MaxSteps {
		return nil, ErrMaxStepsExceeded
	}
	if wf.Limits.MaxInputBytes > 0 && int64(len(req.Input)) > wf.Limits.MaxInputBytes {
		return nil, ErrOutputLimitExceeded
	}
	if req.Context.ExtensionID == "" {
		req.Context.ExtensionID = wf.ExtensionID
	}
	if req.Context.ModuleID == "" {
		req.Context.ModuleID = wf.ModuleID
	}
	if wf.Limits.MaxSkillCallDepth > 0 && req.Context.Depth > wf.Limits.MaxSkillCallDepth {
		return nil, ErrDepthExceeded
	}

	execCtx := ctx
	var execCancel context.CancelFunc
	if wf.Limits.MaxExecutionDurationMS > 0 {
		execCtx, execCancel = context.WithTimeout(ctx, time.Duration(wf.Limits.MaxExecutionDurationMS)*time.Millisecond)
		defer execCancel()
	}

	executionID := req.Context.InvocationID
	if executionID == "" {
		executionID = fmt.Sprintf("%s-%d", req.WorkflowID, start.UnixNano())
		req.Context.InvocationID = executionID
	}
	if req.Context.RootID == "" {
		req.Context.RootID = executionID
	}
	if req.Context.IdempotencyKey == "" {
		req.Context.IdempotencyKey = executionID
	}
	var journal *SideEffectJournal
	if store, ok := e.runStore.(SideEffectStore); ok && store != nil {
		journal = NewPersistentSideEffectJournal(context.WithoutCancel(execCtx), store, SideEffectJournalScope{
			ExecutionID: executionID,
			WorkflowID:  req.WorkflowID,
			Generation:  req.Context.Generation,
			DeviceID:    req.Context.DeviceID,
		})
	}
	if req.Context.OperationID == "" {
		req.Context.OperationID = "wf-op-" + executionID
	}
	if req.Context.TraceID == "" {
		req.Context.TraceID = "wf-trace-" + executionID
	}
	if req.Context.IdempotencyKey == "" {
		req.Context.IdempotencyKey = executionID
	}
	if missing := req.Options.MissingControlledApprovals(wf.Nodes); len(missing) > 0 {
		if e.runStore == nil {
			return nil, fmt.Errorf("workflow: controlled_live confirmation requires a durable run store")
		}
		now := time.Now().UTC()
		waitingRun := WorkflowRun{
			ExecutionID: executionID, WorkflowID: req.WorkflowID, Status: RunStatusWaitingConfirmation,
			Input: req.Input, Context: req.Context, Generation: req.Context.Generation,
			StartedAt: now, UpdatedAt: now,
		}
		existing, created, startErr := e.runStore.Start(context.WithoutCancel(execCtx), waitingRun)
		if startErr != nil {
			return nil, startErr
		}
		if !created && existing != nil {
			waitingRun.StartedAt = existing.StartedAt
		}
		e.emitRunLifecycle(context.WithoutCancel(execCtx), runLifecycleEvent("waiting_confirmation", waitingRun))
		return &ExecuteResult{
			ExecutionID: executionID, WorkflowID: req.WorkflowID, Status: RunStatusWaitingConfirmation,
			Accepted: true, Success: false, Duration: time.Since(start), ExecutionMode: req.Options.Mode,
			RequiredConfirmations: append([]string(nil), missing...),
		}, nil
	}
	concurrencyLocked := false
	if !req.Context.Recovery && wf.ConcurrencyPolicy.Normalize().Mode != WorkflowConcurrencyAllow {
		e.concurrencyMu.Lock()
		concurrencyLocked = true
		defer func() {
			if concurrencyLocked {
				e.concurrencyMu.Unlock()
			}
		}()
	}
	if concurrencyResult, handled, concurrencyErr := e.applyConcurrencyPolicy(execCtx, wf, req, executionID, start); handled || concurrencyErr != nil {
		return concurrencyResult, concurrencyErr
	}
	execCtx, runCancel := context.WithCancel(execCtx)
	e.activeMu.Lock()
	if _, running := e.active[executionID]; running && !req.Context.Recovery {
		e.activeMu.Unlock()
		runCancel()
		return &ExecuteResult{ExecutionID: executionID, WorkflowID: req.WorkflowID, Status: RunStatusRunning, Accepted: true, Success: true, Duration: time.Since(start)}, nil
	}
	e.active[executionID] = runCancel
	e.activeMu.Unlock()
	control := e.ensureExecutionControl(executionID)
	execCtx = withWorkflowPauseSignal(execCtx, control.pauseSignal)
	defer func() {
		runCancel()
		e.activeMu.Lock()
		delete(e.active, executionID)
		e.activeMu.Unlock()
		e.removeExecutionControl(executionID)
	}()

	result := &ExecuteResult{
		ExecutionID:   executionID,
		WorkflowID:    req.WorkflowID,
		Status:        RunStatusRunning,
		Steps:         make([]StepResult, 0, totalNodes),
		ExecutionMode: req.Options.Mode,
	}
	defer func() {
		if result.Status == RunStatusPaused || result.Status == RunStatusPausing || result.Status == RunStatusWaitingDevice {
			return
		}
		if result.Accepted && result.Status == RunStatusRunning {
			return
		}
		result.Status = workflowResultTerminalStatus(result)
	}()

	startedAt := start.UTC()
	if e.runStore != nil {
		existing, created, startErr := e.runStore.Start(execCtx, WorkflowRun{
			ExecutionID: executionID,
			WorkflowID:  req.WorkflowID,
			Status:      RunStatusRunning,
			Input:       req.Input,
			Context:     req.Context,
			Attempt:     1,
			StartedAt:   startedAt,
			UpdatedAt:   startedAt,
		})
		if startErr != nil {
			return nil, startErr
		}
		if !created && existing != nil {
			if existing.Status == RunStatusSucceeded {
				return &ExecuteResult{ExecutionID: existing.ExecutionID, WorkflowID: existing.WorkflowID, Status: existing.Status, Output: existing.Output, Steps: existing.Steps, Success: true, Duration: time.Since(start)}, nil
			}
			if existing.Status == RunStatusRunning && !req.Context.Recovery {
				return &ExecuteResult{ExecutionID: existing.ExecutionID, WorkflowID: existing.WorkflowID, Status: existing.Status, Accepted: true, Success: true, Duration: time.Since(start)}, nil
			}
			startedAt = existing.StartedAt
		}
		startedRun := WorkflowRun{
			ExecutionID: executionID, WorkflowID: req.WorkflowID, Status: RunStatusRunning,
			Input: req.Input, Context: req.Context, Generation: req.Context.Generation,
			StartedAt: startedAt, UpdatedAt: time.Now().UTC(),
		}
		startEventKind := "started"
		if !created {
			startEventKind = "updated"
		}
		e.emitRunLifecycle(context.WithoutCancel(execCtx), runLifecycleEvent(startEventKind, startedRun))
		defer func() {
			if result.Status == RunStatusPaused || result.Status == RunStatusPausing || result.Status == RunStatusWaitingDevice {
				return
			}
			finishedAt := time.Now().UTC()
			status := workflowResultTerminalStatus(result)
			finishedRun := WorkflowRun{
				ExecutionID:         executionID,
				WorkflowID:          req.WorkflowID,
				Status:              status,
				Input:               req.Input,
				Output:              result.Output,
				Error:               result.Error,
				Context:             req.Context,
				Steps:               result.Steps,
				CompensationResults: result.CompensationResults,
				Generation:          req.Context.Generation,
				StartedAt:           startedAt,
				FinishedAt:          &finishedAt,
				UpdatedAt:           finishedAt,
			}
			if finishErr := e.runStore.Finish(context.Background(), finishedRun); finishErr == nil {
				e.emitRunLifecycle(context.Background(), runLifecycleEvent("finished", finishedRun))
				e.drainQueuedWorkflow(context.Background(), req.WorkflowID)
			}
		}()
	}
	if concurrencyLocked {
		e.concurrencyMu.Unlock()
		concurrencyLocked = false
	}

	if req.Options.IsDryRun() {
		dag, compileErr := NewCompiler().Compile(wf, DefaultCompileOptions())
		if compileErr != nil {
			result.Status = RunStatusFailed
			result.Error = compileErr.Error()
			result.Duration = time.Since(start)
			return result, nil
		}
		dry := ExecuteDryRun(execCtx, dag)
		for _, id := range dry.WouldExecute {
			result.Steps = append(result.Steps, StepResult{NodeID: id, Status: "succeeded"})
		}
		for _, id := range dry.WouldSkip {
			result.Steps = append(result.Steps, StepResult{NodeID: id, Status: "skipped"})
		}
		result.Status = RunStatusSucceeded
		result.Success = true
		result.Duration = time.Since(start)
		return result, nil
	}

	stopRunHeartbeat := e.startRunHeartbeat(execCtx, executionID)
	defer stopRunHeartbeat()

	outputs := make(map[string]json.RawMessage)
	outputsMu := sync.RWMutex{}
	nodeMap := make(map[string]WorkflowNode, len(wf.Nodes))
	for _, node := range wf.Nodes {
		nodeMap[node.ID] = node
	}

	failed := false
	var failError string

	currentGeneration := req.Context.Generation
	for _, level := range levels {
		if failed {
			break
		}

		select {
		case <-execCtx.Done():
			result.Success = false
			if execCtx.Err() == context.DeadlineExceeded {
				result.Error = ErrExecutionTimeout.Error()
			} else {
				result.Error = execCtx.Err().Error()
			}
			// The forward execution context is already cancelled here. Saga reverse
			// steps get a fresh parent and retain their own per-step timeouts, so a
			// workflow-level timeout/cancel cannot strand committed side effects.
			if workflowNeedsDeclaredCompensation(wf, result.Steps) {
				compCtx := context.WithoutCancel(ctx)
				_ = e.markRunCompensating(compCtx, executionID)
				result.CompensationResults = e.compensateDeclaredWorkflow(compCtx, wf, result.Steps, req.Input, req.Context, journal)
			}
			result.Duration = time.Since(start)
			return result, nil
		default:
		}

		if ctrl := e.getExecutionControl(executionID); ctrl != nil {
			select {
			case <-ctrl.pauseRequested:
				result.Success = false
				result.Accepted = true
				result.Status = RunStatusPaused
				result.Error = ""
				result.Duration = time.Since(start)
				e.finalisePaused(context.Background(), executionID, currentGeneration)
				return result, nil
			default:
			}
		}

		type nodeExecResult struct {
			nodeID   string
			step     StepResult
			input    json.RawMessage
			restored bool
		}

		resultChan := make(chan nodeExecResult, len(level))
		var wg sync.WaitGroup

		maxConcurrency := wf.Limits.MaxConcurrency
		if maxConcurrency <= 0 || maxConcurrency > len(level) {
			maxConcurrency = len(level)
		}
		semaphore := make(chan struct{}, maxConcurrency)
		for _, nodeID := range level {
			wg.Add(1)
			go func(nid string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				node := nodeMap[nid]

				if e.checkpoint != nil {
					cp, cpErr := e.checkpoint.Load(execCtx, executionID, nid)
					if cpErr == nil && cp != nil {
						resultChan <- nodeExecResult{
							nodeID:   nid,
							input:    cp.Input,
							restored: true,
							step: StepResult{
								NodeID:  nid,
								Status:  "succeeded",
								Output:  cp.Output,
								Attempt: 0,
							},
						}
						return
					}
				}

				node = e.restorePausedWaitNode(execCtx, executionID, node, req.Context.Recovery)

				input := req.Input
				if len(node.Step.Input) > 0 {
					if wf.SchemaVersion == UserWorkflowSchemaVersion {
						var inputTemplate map[string]any
						if err := json.Unmarshal(node.Step.Input, &inputTemplate); err == nil && inputTemplate != nil {
							resolved := e.resolveInputTemplate(inputTemplate, req.Input, outputs, &outputsMu, req.Context)
							if resolvedBytes, marshalErr := json.Marshal(resolved); marshalErr == nil {
								input = resolvedBytes
							} else {
								input = node.Step.Input
							}
						} else {
							input = node.Step.Input
						}
					} else {
						input = node.Step.Input
					}
				} else if len(node.DependsOn) > 0 {
					outputsMu.RLock()
					merged := make(map[string]json.RawMessage)
					for _, dep := range node.DependsOn {
						if out, depOK := outputs[dep]; depOK {
							merged[dep] = out
						}
					}
					outputsMu.RUnlock()
					if len(merged) > 0 {
						if mergedBytes, mErr := json.Marshal(merged); mErr == nil {
							input = mergedBytes
						}
					}
				}

				if node.Step.When != nil && len(*node.Step.When) > 0 {
					shouldExecute := true
					var conditionErr error
					if wf.SchemaVersion == UserWorkflowSchemaVersion {
						expr, compileErr := CompileExpression(*node.Step.When)
						if compileErr != nil {
							conditionErr = compileErr
						} else {
							var inputMap map[string]any
							_ = json.Unmarshal(req.Input, &inputMap)
							if inputMap == nil {
								inputMap = make(map[string]any)
							}
							outputsMu.RLock()
							outputSnapshot := make(map[string]json.RawMessage, len(outputs))
							for key, value := range outputs {
								outputSnapshot[key] = value
							}
							outputsMu.RUnlock()
							shouldExecute, conditionErr = EvaluateExpression(expr, ExpressionEvalConfig{Input: inputMap, Runtime: workflowRuntimeValues(req.Context), Outputs: outputSnapshot})
						}
					} else {
						shouldExecute, conditionErr = evaluateWhen(*node.Step.When)
					}
					if conditionErr != nil {
						resultChan <- nodeExecResult{nodeID: nid, input: input, step: StepResult{NodeID: nid, Status: "failed", Error: conditionErr.Error(), Attempt: 0}}
						return
					}
					if !shouldExecute {
						resultChan <- nodeExecResult{
							nodeID: nid,
							input:  input,
							step: StepResult{
								NodeID: nid,
								Status: "skipped",
							},
						}
						return
					}
				}

				if mockOutput, mockErr, mocked := req.Options.EffectiveMockOutput(nid); mocked {
					step := StepResult{NodeID: nid, Status: "succeeded", Output: mockOutput, Attempt: 1}
					if strings.TrimSpace(mockErr) != "" {
						step.Status = "failed"
						step.Error = mockErr
					}
					resultChan <- nodeExecResult{nodeID: nid, input: input, step: step}
					return
				}
				if req.Options.IsMocked() {
					if _, sideEffecting := sideEffectKindForNode(node); sideEffecting {
						resultChan <- nodeExecResult{nodeID: nid, input: input, step: StepResult{NodeID: nid, Status: "failed", Error: "mocked execution requires an explicit mock for side-effecting node", Attempt: 0}}
						return
					}
				}

				handler, hOK := e.handlers[node.Type]
				if !hOK {
					handler, hOK = e.handlers[strings.ToLower(node.Type)]
				}
				if !hOK {
					resultChan <- nodeExecResult{
						nodeID: nid,
						step: StepResult{
							NodeID: nid,
							Status: "failed",
							Error:  ErrHandlerNotFound.Error(),
						},
					}
					return
				}

				if e.guard != nil {
					if guardErr := e.guard.Check(execCtx, wf, node, req.Context); guardErr != nil {
						resultChan <- nodeExecResult{nodeID: nid, input: input, step: StepResult{NodeID: nid, Status: "failed", Error: guardErr.Error()}}
						return
					}
				}

				if node.Step.Postcondition != nil && len(*node.Step.Postcondition) > 0 {
					expr, compileErr := CompileExpression(*node.Step.Postcondition)
					if compileErr != nil {
						resultChan <- nodeExecResult{nodeID: nid, input: input, step: StepResult{NodeID: nid, Status: "failed", Error: compileErr.Error()}}
						return
					}
					handler = wrapPostconditionHandler(handler, nid, expr, req.Input, workflowRuntimeValues(req.Context), func() map[string]json.RawMessage {
						outputsMu.RLock()
						defer outputsMu.RUnlock()
						snapshot := make(map[string]json.RawMessage, len(outputs))
						for key, value := range outputs {
							snapshot[key] = value
						}
						return snapshot
					})
				}

				stepResult := e.executeStep(withExecutionContext(execCtx, req.Context), handler, node, input, wf.Limits, journal, req.WorkflowID)
				stepResult.NodeID = nid
				resultChan <- nodeExecResult{
					nodeID: nid,
					step:   stepResult,
					input:  input,
				}
			}(nodeID)
		}

		wg.Wait()
		close(resultChan)

		levelPaused := false
		waitingDeviceID := ""
		waitingDeviceError := ""
		for ner := range resultChan {
			node := nodeMap[ner.nodeID]
			if ner.step.Status == "failed" && node.Step.OnError.Mode == "use_default" && len(node.Step.OnError.Default) > 0 {
				ner.step.Status = "defaulted"
				ner.step.Output = node.Step.OnError.Default
				ner.step.Error = ""
			}
			result.Steps = append(result.Steps, ner.step)
			if e.runStore != nil && !ner.restored {
				finishedAt := time.Now().UTC()
				runtimeID := strings.TrimSpace(node.ExecutionTarget.RuntimeID)
				if runtimeID == "" {
					runtimeID = strings.TrimSpace(node.Runtime.RuntimeID)
				}
				deviceID := strings.TrimSpace(req.Context.DeviceID)
				if deviceID == "" {
					deviceID = strings.TrimSpace(node.ExecutionTarget.DeviceID)
				}
				logicalAttempt := int(req.Context.Generation) + 1
				if logicalAttempt < 1 {
					logicalAttempt = 1
				}
				idempotencyKey := BuildStepIdempotencyKey(req.WorkflowID, executionID, ner.nodeID, logicalAttempt)
				toolCallID := ""
				if strings.TrimSpace(node.TargetID) != "" || runtimeID != "" {
					toolCallID = fmt.Sprintf("%s/%s", executionID, ner.nodeID)
				}
				attemptID := ""
				if ner.step.Attempt > 0 {
					attemptID = fmt.Sprintf("%s/%s/g%d/a%d", executionID, ner.nodeID, req.Context.Generation, ner.step.Attempt)
				}
				_ = e.runStore.SaveStep(execCtx, StepRun{ExecutionID: executionID, WorkflowID: req.WorkflowID, NodeID: ner.nodeID, TraceID: req.Context.TraceID, AttemptID: attemptID, DeviceID: deviceID, RuntimeID: runtimeID, ToolCallID: toolCallID, IdempotencyKey: idempotencyKey, Status: ner.step.Status, Input: ner.input, Output: ner.step.Output, Error: ner.step.Error, Attempt: ner.step.Attempt, StartedAt: finishedAt.Add(-ner.step.Duration), FinishedAt: &finishedAt})
			}

			if ner.step.Status == "paused" {
				// A pausable node (currently the built-in wait node) can stop while
				// other nodes in the same DAG level are already completing. Do not
				// return here: persist/checkpoint every sibling first so Resume cannot
				// replay side effects that finished before the pause boundary.
				levelPaused = true
				continue
			}

			if ner.step.Status == "waiting_device" {
				// Like pause, waiting_device is a level boundary. Finish recording
				// sibling outcomes before transitioning the run so reconnect recovery
				// can restore completed siblings instead of executing them twice.
				if waitingDeviceID == "" {
					waitingDeviceID = strings.TrimSpace(node.ExecutionTarget.DeviceID)
					waitingDeviceError = ner.step.Error
				}
				continue
			}

			if ner.step.Status == "succeeded" || ner.step.Status == "defaulted" {
				outputsMu.Lock()
				outputs[ner.nodeID] = ner.step.Output
				outputsMu.Unlock()

				if e.checkpoint != nil && !ner.restored {
					_ = e.checkpoint.Save(execCtx, Checkpoint{
						WorkflowID:  req.WorkflowID,
						ExecutionID: executionID,
						NodeID:      ner.nodeID,
						Input:       ner.input,
						Output:      ner.step.Output,
						CompletedAt: time.Now(),
					})
				}

				if wf.Limits.MaxOutputBytes > 0 && int64(len(ner.step.Output)) > wf.Limits.MaxOutputBytes {
					result.Success = false
					result.Error = ErrOutputLimitExceeded.Error()
					result.Duration = time.Since(start)
					return result, nil
				}
			} else if ner.step.Status == "skipped" {
				if len(node.Step.OnError.Default) > 0 {
					outputsMu.Lock()
					outputs[ner.nodeID] = node.Step.OnError.Default
					outputsMu.Unlock()
				}
			} else if ner.step.Status == "cancelled" {
				result.Success = false
				result.Error = ner.step.Error
				result.Duration = time.Since(start)
				return result, nil
			} else {
				switch node.Step.OnError.Mode {
				case "continue":
					if len(node.Step.OnError.Default) > 0 {
						outputsMu.Lock()
						outputs[ner.nodeID] = node.Step.OnError.Default
						outputsMu.Unlock()
					}
				case "retry":
					if len(node.Step.OnError.Default) > 0 {
						outputsMu.Lock()
						outputs[ner.nodeID] = node.Step.OnError.Default
						outputsMu.Unlock()
					} else {
						failed = true
						if failError == "" {
							failError = ner.step.Error
						}
					}
				default:
					failed = true
					if failError == "" {
						failError = ner.step.Error
					}
				}
			}
		}

		if levelPaused {
			result.Success = false
			result.Accepted = true
			result.Status = RunStatusPaused
			result.Error = ""
			result.Duration = time.Since(start)
			e.finalisePaused(context.Background(), executionID, currentGeneration)
			return result, nil
		}

		if waitingDeviceID != "" {
			if err := e.markWaitingDevice(execCtx, executionID, waitingDeviceID); err != nil {
				result.Success = false
				result.Error = err.Error()
				result.Duration = time.Since(start)
				return result, nil
			}
			result.Success = false
			result.Accepted = true
			result.Status = RunStatusWaitingDevice
			result.Error = waitingDeviceError
			result.Duration = time.Since(start)
			return result, nil
		}

		// Pause requested while this level was executing but no node supported
		// cooperative interruption. The current level is now fully durable, so
		// stop before advancing (or before reporting success for the final level).
		if ctrl := e.getExecutionControl(executionID); ctrl != nil {
			select {
			case <-ctrl.pauseRequested:
				result.Success = false
				result.Accepted = true
				result.Status = RunStatusPaused
				result.Error = ""
				result.Duration = time.Since(start)
				e.finalisePaused(context.Background(), executionID, currentGeneration)
				return result, nil
			default:
			}
		}
	}

	if failed {
		result.Success = false
		result.Error = failError

		if workflowNeedsDeclaredCompensation(wf, result.Steps) {
			_ = e.markRunCompensating(context.WithoutCancel(execCtx), executionID)
			result.CompensationResults = e.compensateDeclaredWorkflow(execCtx, wf, result.Steps, req.Input, req.Context, journal)
		} else if e.compensation != nil {
			result.CompensationResults = e.compensation.Compensate(ctx, result.Steps)
		}

		result.Duration = time.Since(start)
		return result, nil
	}

	var lastOutput json.RawMessage = req.Input
	if len(result.Steps) > 0 {
		for i := len(result.Steps) - 1; i >= 0; i-- {
			if (result.Steps[i].Status == "succeeded" || result.Steps[i].Status == "defaulted") && len(result.Steps[i].Output) > 0 {
				lastOutput = result.Steps[i].Output
				break
			}
		}
	}

	result.Output = lastOutput
	result.Success = true
	result.Status = RunStatusSucceeded
	result.Duration = time.Since(start)
	return result, nil
}

type workflowCallStackKey struct{}

func pushWorkflowCall(ctx context.Context, workflowID string) (context.Context, error) {
	stack, _ := ctx.Value(workflowCallStackKey{}).([]string)
	for _, item := range stack {
		if item == workflowID {
			path := append(append([]string(nil), stack...), workflowID)
			return ctx, fmt.Errorf("workflow: nested workflow cycle detected: %s", strings.Join(path, " -> "))
		}
	}
	next := append(append([]string(nil), stack...), workflowID)
	return context.WithValue(ctx, workflowCallStackKey{}, next), nil
}

type executionContextKey struct{}

func withExecutionContext(ctx context.Context, execution ExecutionContext) context.Context {
	return context.WithValue(ctx, executionContextKey{}, execution)
}

func ExecutionContextFromContext(ctx context.Context) (ExecutionContext, bool) {
	execution, ok := ctx.Value(executionContextKey{}).(ExecutionContext)
	return execution, ok
}

func workflowHasDeclaredCompensation(def WorkflowDefinition) bool {
	for _, node := range def.Nodes {
		if node.Compensation != nil {
			return true
		}
	}
	return false
}

func workflowNeedsDeclaredCompensation(def WorkflowDefinition, completedSteps []StepResult) bool {
	if !workflowHasDeclaredCompensation(def) {
		return false
	}
	declared := make(map[string]struct{}, len(def.Nodes))
	for _, node := range def.Nodes {
		if node.Compensation != nil {
			declared[node.ID] = struct{}{}
		}
	}
	for _, step := range completedSteps {
		if step.Status != "succeeded" {
			continue
		}
		if _, ok := declared[step.NodeID]; ok {
			return true
		}
	}
	return false
}

func compensationCoversRequiredSteps(def WorkflowDefinition, completedSteps []StepResult, results []CompensationResult) bool {
	required := make(map[string]struct{})
	for _, node := range def.Nodes {
		if node.Compensation != nil {
			required[node.ID] = struct{}{}
		}
	}
	expected := make(map[string]struct{})
	for _, step := range completedSteps {
		if step.Status == "succeeded" {
			if _, ok := required[step.NodeID]; ok {
				expected[step.NodeID] = struct{}{}
			}
		}
	}
	if len(expected) == 0 {
		return true
	}
	for _, result := range results {
		if result.Status == "succeeded" {
			delete(expected, result.NodeID)
		}
	}
	return len(expected) == 0
}

func (e *WorkflowExecutor) executeCompensationRecoveryAsync(run WorkflowRun) {
	go func() {
		if err := e.recoverDeclaredCompensation(context.Background(), run); err != nil {
			e.finishCompensationRecoverySetupFailure(run.ExecutionID, err)
		}
	}()
}

func (e *WorkflowExecutor) recoverDeclaredCompensation(ctx context.Context, run WorkflowRun) error {
	if e == nil || e.runStore == nil {
		return errors.New("workflow: compensation recovery requires a run store")
	}
	store, ok := e.runStore.(CompensationStore)
	if !ok || store == nil {
		return errors.New("workflow: compensation recovery requires a durable compensation store")
	}
	stepStore, ok := e.runStore.(CompensationStepRunStore)
	if !ok || stepStore == nil {
		return errors.New("workflow: compensation recovery requires durable step history")
	}
	if len(run.Context.DefinitionSnapshot) == 0 {
		return errors.New("workflow: compensation recovery requires immutable definition snapshot")
	}

	var def WorkflowDefinition
	if err := json.Unmarshal(run.Context.DefinitionSnapshot, &def); err != nil {
		return fmt.Errorf("workflow: restore compensation definition snapshot: %w", err)
	}
	if def.ID != run.WorkflowID {
		return fmt.Errorf("workflow: compensation snapshot id %q does not match run workflow %q", def.ID, run.WorkflowID)
	}
	// DefinitionHash authenticates the exact immutable snapshot that the run
	// started with. Verify before any in-memory schema migration changes the
	// representation used for execution.
	if run.Context.DefinitionHash != "" && ComputeDefinitionHash(def) != run.Context.DefinitionHash {
		return errors.New("workflow: compensation definition snapshot hash mismatch")
	}
	migrated, err := MigrateDefinition(def)
	if err != nil {
		return fmt.Errorf("workflow: migrate compensation definition snapshot: %w", err)
	}
	def = migrated

	stepRuns, err := stepStore.ListStepRuns(ctx, run.ExecutionID)
	if err != nil {
		return fmt.Errorf("workflow: list durable step history for compensation: %w", err)
	}
	completed := make([]StepResult, 0, len(stepRuns))
	for _, step := range stepRuns {
		completed = append(completed, StepResult{
			NodeID:  step.NodeID,
			Status:  step.Status,
			Output:  append(json.RawMessage(nil), step.Output...),
			Error:   step.Error,
			Attempt: step.Attempt,
		})
	}

	execution := run.Context
	execution.InvocationID = run.ExecutionID
	execution.WorkflowID = run.WorkflowID
	execution.Generation = run.Generation
	execution.Recovery = true

	var journal *SideEffectJournal
	if sideEffects, ok := e.runStore.(SideEffectStore); ok && sideEffects != nil {
		journal = NewPersistentSideEffectJournal(context.WithoutCancel(ctx), sideEffects, SideEffectJournalScope{
			ExecutionID: run.ExecutionID,
			WorkflowID:  run.WorkflowID,
			Generation:  run.Generation,
			DeviceID:    execution.DeviceID,
		})
	}

	execCtx, cancel := context.WithCancel(ctx)
	e.activeMu.Lock()
	if _, exists := e.active[run.ExecutionID]; exists {
		e.activeMu.Unlock()
		cancel()
		return nil
	}
	e.active[run.ExecutionID] = cancel
	e.activeMu.Unlock()
	stopHeartbeat := e.startRunHeartbeat(execCtx, run.ExecutionID)
	defer func() {
		stopHeartbeat()
		cancel()
		e.activeMu.Lock()
		delete(e.active, run.ExecutionID)
		e.activeMu.Unlock()
	}()

	results := e.compensateDeclaredWorkflow(execCtx, def, completed, run.Input, execution, journal)
	now := time.Now().UTC()
	current, err := e.runStore.Get(context.WithoutCancel(ctx), run.ExecutionID)
	if err != nil || current == nil {
		if err != nil {
			return err
		}
		return ErrWorkflowRunNotFound
	}
	if current.Status != RunStatusCompensating || current.Generation != run.Generation {
		return nil
	}
	finished := *current
	finished.CompensationResults = results
	if compensationCoversRequiredSteps(def, completed, results) {
		finished.Status = RunStatusCompensated
	} else {
		finished.Status = RunStatusCompensationFailed
		if finished.Error == "" {
			finished.Error = "one or more Saga compensation steps failed"
		}
	}
	finished.FinishedAt = &now
	finished.UpdatedAt = now
	if err := e.runStore.Finish(context.WithoutCancel(ctx), finished); err != nil {
		return err
	}
	e.emitRunLifecycle(context.WithoutCancel(ctx), runLifecycleEvent("finished", finished))
	go e.drainQueuedWorkflow(context.Background(), finished.WorkflowID)
	return nil
}

func (e *WorkflowExecutor) finishCompensationRecoverySetupFailure(executionID string, cause error) {
	if e == nil || e.runStore == nil || cause == nil {
		return
	}
	ctx := context.Background()
	run, err := e.runStore.Get(ctx, executionID)
	if err != nil || run == nil || run.Status != RunStatusCompensating {
		return
	}
	now := time.Now().UTC()
	failed := *run
	failed.Status = RunStatusManualInterventionRequired
	failed.Error = "compensation recovery requires manual intervention: " + cause.Error()
	failed.FinishedAt = &now
	failed.UpdatedAt = now
	ok, err := e.runStore.UpdateStateCAS(ctx, failed, RunStatusCompensating)
	if err != nil || !ok {
		return
	}
	if err := e.runStore.Finish(ctx, failed); err == nil {
		e.emitRunLifecycle(ctx, runLifecycleEvent("finished", failed))
		go e.drainQueuedWorkflow(context.Background(), failed.WorkflowID)
	}
}

func (e *WorkflowExecutor) markRunCompensating(ctx context.Context, executionID string) error {
	if e == nil || e.runStore == nil || strings.TrimSpace(executionID) == "" {
		return nil
	}
	run, err := e.runStore.Get(ctx, executionID)
	if err != nil || run == nil {
		return err
	}
	if run.Status == RunStatusCompensating {
		return nil
	}
	if run.Status != RunStatusRunning && run.Status != RunStatusResuming {
		return fmt.Errorf("workflow: cannot enter compensation from status %s", run.Status)
	}
	now := time.Now().UTC()
	updated := *run
	updated.Status = RunStatusCompensating
	updated.UpdatedAt = now
	ok, err := e.runStore.UpdateStateCAS(ctx, updated, run.Status)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("workflow: compensation state changed concurrently")
	}
	e.emitRunLifecycle(ctx, runLifecycleEvent("updated", updated))
	return nil
}

func (e *WorkflowExecutor) compensateDeclaredWorkflow(ctx context.Context, def WorkflowDefinition, completedSteps []StepResult, workflowInput json.RawMessage, execution ExecutionContext, journal *SideEffectJournal) []CompensationResult {
	if e == nil {
		return nil
	}
	nodes := make(map[string]WorkflowNode, len(def.Nodes))
	outputs := make(map[string]json.RawMessage, len(completedSteps))
	for _, node := range def.Nodes {
		nodes[node.ID] = node
	}
	for _, step := range completedSteps {
		if step.Status == "succeeded" || step.Status == "defaulted" {
			outputs[step.NodeID] = append(json.RawMessage(nil), step.Output...)
		}
	}
	var outputsMu sync.RWMutex
	store, _ := e.runStore.(CompensationStore)
	results := make([]CompensationResult, 0)
	persistCtx := context.WithoutCancel(ctx)

	for i := len(completedSteps) - 1; i >= 0; i-- {
		step := completedSteps[i]
		if step.Status != "succeeded" {
			continue
		}
		original, ok := nodes[step.NodeID]
		if !ok || original.Compensation == nil {
			continue
		}

		if store != nil {
			if existing, err := store.GetCompensation(persistCtx, execution.InvocationID, original.ID); err == nil && existing != nil && existing.Status == "succeeded" {
				results = append(results, CompensationResult{NodeID: original.ID, Status: "succeeded", ExecutedAt: existing.UpdatedAt})
				continue
			}
		}

		compNode, err := BuildCompensationNode(original)
		if err != nil {
			results = append(results, CompensationResult{NodeID: original.ID, Status: "failed", Error: err.Error(), ExecutedAt: time.Now().UTC()})
			continue
		}
		handler := e.handlers[strings.ToLower(strings.TrimSpace(compNode.Type))]
		if handler == nil {
			err := fmt.Errorf("workflow: compensation handler not registered for %s", compNode.Type)
			results = append(results, CompensationResult{NodeID: original.ID, Status: "failed", Error: err.Error(), ExecutedAt: time.Now().UTC()})
			continue
		}
		if e.guard != nil {
			if guardErr := e.guard.Check(ctx, def, compNode, execution); guardErr != nil {
				results = append(results, CompensationResult{NodeID: original.ID, Status: "failed", Error: guardErr.Error(), ExecutedAt: time.Now().UTC()})
				continue
			}
		}

		input := append(json.RawMessage(nil), step.Output...)
		if len(compNode.Step.Input) > 0 {
			input = append(json.RawMessage(nil), compNode.Step.Input...)
			if def.SchemaVersion == UserWorkflowSchemaVersion {
				var inputTemplate map[string]any
				if err := json.Unmarshal(compNode.Step.Input, &inputTemplate); err == nil && inputTemplate != nil {
					resolved := e.resolveInputTemplate(inputTemplate, workflowInput, outputs, &outputsMu, execution)
					if resolvedBytes, marshalErr := json.Marshal(resolved); marshalErr == nil {
						input = resolvedBytes
					}
				}
			}
		}

		idemKey := BuildCompensationIdempotencyKey(def.ID, execution.InvocationID, compNode.ID)
		startedAt := time.Now().UTC()
		if store != nil {
			_ = store.SaveCompensation(persistCtx, CompensationRecord{
				ExecutionID: execution.InvocationID, WorkflowID: def.ID, NodeID: original.ID,
				Generation: execution.Generation, Status: "running", IdempotencyKey: idemKey,
				Input: input, StartedAt: startedAt, UpdatedAt: startedAt,
			})
		}

		compExecution := execution
		compExecution.NodeID = compNode.ID
		compExecution.IdempotencyKey = idemKey
		stepCtx := withExecutionContext(ctx, compExecution)
		compStep := e.executeStep(stepCtx, handler, compNode, input, def.Limits, journal, def.ID)
		now := time.Now().UTC()
		status := "succeeded"
		errText := ""
		if compStep.Status != "succeeded" {
			status = "failed"
			errText = compStep.Error
		}
		if store != nil {
			completed := now
			_ = store.SaveCompensation(persistCtx, CompensationRecord{
				ExecutionID: execution.InvocationID, WorkflowID: def.ID, NodeID: original.ID,
				Generation: execution.Generation, Status: status, Attempt: compStep.Attempt, IdempotencyKey: idemKey,
				Input: input, Output: compStep.Output, Error: errText, StartedAt: startedAt, CompletedAt: &completed, UpdatedAt: now,
			})
		}
		results = append(results, CompensationResult{NodeID: original.ID, Status: status, Error: errText, ExecutedAt: now})
	}
	return results
}

func workflowResultTerminalStatus(result *ExecuteResult) RunStatus {
	if result == nil {
		return RunStatusFailed
	}
	if result.Success {
		return RunStatusSucceeded
	}
	if compensationSucceeded(result.CompensationResults) {
		return RunStatusCompensated
	}
	if len(result.CompensationResults) > 0 {
		return RunStatusCompensationFailed
	}
	if strings.Contains(strings.ToLower(result.Error), "canceled") {
		return RunStatusCancelled
	}
	return RunStatusFailed
}

func compensationSucceeded(results []CompensationResult) bool {
	if len(results) == 0 {
		return false
	}
	for _, result := range results {
		if result.Status != "succeeded" {
			return false
		}
	}
	return true
}

func (e *WorkflowExecutor) saveAttempt(ctx context.Context, workflowID string, node WorkflowNode, input json.RawMessage, attempt int, status string, output json.RawMessage, err error, nextBackoff time.Duration, startedAt, finishedAt time.Time) {
	store, ok := e.runStore.(StepAttemptStore)
	if !ok || store == nil || workflowID == "" || node.ID == "" || attempt <= 0 {
		return
	}
	execution, _ := ExecutionContextFromContext(ctx)
	if execution.InvocationID == "" {
		return
	}
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	persistCtx := context.WithoutCancel(ctx)
	traceMetadata := ExecutionTraceMetadataSnapshot(ctx)
	runtimeID := strings.TrimSpace(traceMetadata.RuntimeID)
	if runtimeID == "" {
		runtimeID = strings.TrimSpace(node.ExecutionTarget.RuntimeID)
	}
	if runtimeID == "" {
		runtimeID = strings.TrimSpace(node.Runtime.RuntimeID)
	}
	deviceID := strings.TrimSpace(traceMetadata.DeviceID)
	if deviceID == "" {
		deviceID = strings.TrimSpace(execution.DeviceID)
	}
	if deviceID == "" {
		deviceID = strings.TrimSpace(node.ExecutionTarget.DeviceID)
	}
	toolCallID := strings.TrimSpace(traceMetadata.ToolCallID)
	if toolCallID == "" && (strings.TrimSpace(node.TargetID) != "" || runtimeID != "") {
		toolCallID = fmt.Sprintf("%s/%s", execution.InvocationID, node.ID)
	}
	attemptID := fmt.Sprintf("%s/%s/g%d/a%d", execution.InvocationID, node.ID, execution.Generation, attempt)
	_ = store.SaveAttempt(persistCtx, StepAttemptRun{
		ExecutionID:    execution.InvocationID,
		WorkflowID:     workflowID,
		NodeID:         node.ID,
		TraceID:        execution.TraceID,
		AttemptID:      attemptID,
		DeviceID:       deviceID,
		RuntimeID:      runtimeID,
		ToolCallID:     toolCallID,
		FencingToken:   execution.FencingToken,
		IdempotencyKey: execution.IdempotencyKey,
		Attempt:        attempt,
		Generation:     execution.Generation,
		Status:         status,
		Input:          input,
		Output:         output,
		Error:          errorMessage,
		NextBackoffMS:  nextBackoff.Milliseconds(),
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
	})
}

func (e *WorkflowExecutor) executeStep(ctx context.Context, handler StepHandler, node WorkflowNode, input json.RawMessage, limits WorkflowLimits, journal *SideEffectJournal, workflowID string) StepResult {
	start := time.Now()

	var retryPolicy *WorkflowRetryPolicy
	if node.Retry == nil && node.Step.OnError.Mode == "retry" {
		// Preserve the compatibility executor's historical SetRetryMax semantics:
		// retryMax counts retries in addition to the first attempt.
		retryPolicy = DefaultRetryPolicy()
		retryPolicy.MaxAttempts = e.retryMax + 1
	} else {
		var retryErr error
		retryPolicy, retryErr = NewCompiler().compileRetry(node)
		if retryErr != nil {
			return StepResult{Status: "failed", Error: retryErr.Error(), Duration: time.Since(start), Attempt: 0}
		}
	}
	retryPolicy = retryPolicyNormalize(retryPolicy)
	maxAttempts := retryPolicy.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return StepResult{Status: "cancelled", Error: ctx.Err().Error(), Duration: time.Since(start), Attempt: attempt - 1}
			}
			return StepResult{Status: "failed", Error: ErrExecutionTimeout.Error(), Duration: time.Since(start), Attempt: attempt - 1}
		default:
		}

		stepCtx := ctx
		var cancel context.CancelFunc
		timeout := time.Duration(0)
		if node.TimeoutMS > 0 {
			timeout = time.Duration(node.TimeoutMS) * time.Millisecond
			if limits.MaxStepDurationMS > 0 {
				maxTimeout := time.Duration(limits.MaxStepDurationMS) * time.Millisecond
				if timeout > maxTimeout {
					timeout = maxTimeout
				}
			}
		} else if limits.MaxStepDurationMS > 0 {
			timeout = time.Duration(limits.MaxStepDurationMS) * time.Millisecond
		}
		if timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, timeout)
		}

		attemptStarted := time.Now().UTC()
		attemptExecution, _ := ExecutionContextFromContext(ctx)
		attemptExecution.NodeID = node.ID
		logicalAttempt := int(attemptExecution.Generation) + 1
		if logicalAttempt < 1 {
			logicalAttempt = 1
		}
		attemptExecution.LogicalAttempt = logicalAttempt
		if strings.HasSuffix(node.ID, CompensationNodeSuffix) {
			attemptExecution.IdempotencyKey = BuildCompensationIdempotencyKey(workflowID, attemptExecution.InvocationID, node.ID)
		} else {
			attemptExecution.IdempotencyKey = BuildStepIdempotencyKey(workflowID, attemptExecution.InvocationID, node.ID, logicalAttempt)
		}

		handlerCtx := stepCtx
		var lease workflowExecutionLeaseHandle
		lease, leaseErr := e.acquireStepExecutionLease(stepCtx, attemptExecution, node, timeout)
		if leaseErr == nil && lease.active {
			attemptExecution.FencingToken = lease.lease.FencingToken
			handlerCtx = lease.ctx
		}
		traceMetadata := &ExecutionTraceMetadata{
			DeviceID:  strings.TrimSpace(attemptExecution.DeviceID),
			RuntimeID: strings.TrimSpace(node.ExecutionTarget.RuntimeID),
		}
		if traceMetadata.DeviceID == "" {
			traceMetadata.DeviceID = strings.TrimSpace(node.ExecutionTarget.DeviceID)
		}
		if traceMetadata.RuntimeID == "" {
			traceMetadata.RuntimeID = strings.TrimSpace(node.Runtime.RuntimeID)
		}
		if strings.TrimSpace(node.TargetID) != "" || traceMetadata.RuntimeID != "" {
			traceMetadata.ToolCallID = fmt.Sprintf("%s/%s", attemptExecution.InvocationID, node.ID)
		}
		attemptCtx := WithExecutionTraceMetadata(withExecutionContext(handlerCtx, attemptExecution), traceMetadata)
		var output json.RawMessage
		var err error
		if leaseErr != nil {
			err = leaseErr
		} else {
			output, err = handler.Execute(attemptCtx, node, input)
		}
		stepErr := handlerCtx.Err()
		if lease.active {
			lease.close()
		}
		attemptFinished := time.Now().UTC()
		if journal != nil {
			if kind, sideEffecting := sideEffectKindForNode(node); sideEffecting {
				errText := ""
				if err != nil {
					errText = err.Error()
				} else if stepErr != nil {
					errText = stepErr.Error()
				}
				journal.RecordAttempt(node.ID, attempt, attemptExecution.IdempotencyKey, kind, node.TargetID, input, output, errText, attemptFinished.Sub(attemptStarted))
			}
		}
		if cancel != nil {
			cancel()
		}

		var pauseErr *WorkflowPauseError
		if err != nil && errors.As(err, &pauseErr) {
			e.saveAttempt(attemptCtx, workflowID, node, input, attempt, "paused", output, nil, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "paused", Output: output, Duration: time.Since(start), Attempt: attempt}
		}

		if stepErr == nil && err == nil {
			e.saveAttempt(attemptCtx, workflowID, node, input, attempt, "succeeded", output, nil, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "succeeded", Output: output, Duration: time.Since(start), Attempt: attempt}
		}

		lastErr = err
		attemptStatus := "failed"
		if stepErr == context.DeadlineExceeded {
			lastErr = ErrStepTimeout
			attemptStatus = "timed_out"
		} else if stepErr == context.Canceled && ctx.Err() != nil {
			lastErr = ctx.Err()
			attemptStatus = "cancelled"
			e.saveAttempt(attemptCtx, workflowID, node, input, attempt, attemptStatus, nil, lastErr, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "cancelled", Error: lastErr.Error(), Duration: time.Since(start), Attempt: attempt}
		}

		var unavailable *WorkflowDeviceUnavailableError
		if err != nil && errors.As(err, &unavailable) {
			e.saveAttempt(attemptCtx, workflowID, node, input, attempt, "waiting_device", output, err, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "waiting_device", Error: err.Error(), Duration: time.Since(start), Attempt: attempt}
		}

		nextBackoff := time.Duration(0)
		if attempt < maxAttempts && retryPolicy.IsRetryable("") {
			nextBackoff = retryPolicy.ComputeBackoff(attempt)
		}
		e.saveAttempt(attemptCtx, workflowID, node, input, attempt, attemptStatus, output, lastErr, nextBackoff, attemptStarted, attemptFinished)

		if nextBackoff > 0 {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.Canceled {
					return StepResult{Status: "cancelled", Error: ctx.Err().Error(), Duration: time.Since(start), Attempt: attempt}
				}
				return StepResult{Status: "failed", Error: ErrExecutionTimeout.Error(), Duration: time.Since(start), Attempt: attempt}
			case <-time.After(nextBackoff):
			}
		}
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}

	return StepResult{Status: "failed", Error: errMsg, Duration: time.Since(start), Attempt: maxAttempts}
}

func evaluateWhen(when json.RawMessage) (bool, error) {
	if len(when) == 0 {
		return true, nil
	}

	var condition any
	if err := json.Unmarshal(when, &condition); err != nil {
		return true, nil
	}

	switch v := condition.(type) {
	case bool:
		return v, nil
	case string:
		return v != "false" && v != "" && v != "0", nil
	case float64:
		return v != 0, nil
	case nil:
		return false, nil
	default:
		return true, nil
	}
}

type CompiledExecuteRequest struct {
	DAG     *CompiledWorkflowDAG
	Input   json.RawMessage
	Context ExecutionContext
	Opts    ExecutionOptions
	Journal *SideEffectJournal
}

func workflowDefinitionFromCompiled(dag *CompiledWorkflowDAG) WorkflowDefinition {
	if dag == nil {
		return WorkflowDefinition{}
	}
	def := WorkflowDefinition{
		ID:                dag.WorkflowID,
		SchemaVersion:     UserWorkflowSchemaVersion,
		Limits:            dag.Limits,
		ConcurrencyPolicy: dag.ConcurrencyPolicy,
		DefinitionHash:    dag.DefinitionHash,
		Nodes:             make([]WorkflowNode, 0, len(dag.TopologicalOrder)),
	}
	seen := make(map[string]struct{}, len(dag.Nodes))
	appendNode := func(cnode CompiledWorkflowNode) {
		if _, ok := seen[cnode.ID]; ok {
			return
		}
		seen[cnode.ID] = struct{}{}
		node := WorkflowNode{
			ID:                   cnode.ID,
			Type:                 cnode.Type,
			DependsOn:            append([]string(nil), cnode.DependsOn...),
			TargetID:             cnode.TargetID,
			ExecutionTarget:      cnode.ExecutionTarget,
			Permissions:          append([]string(nil), cnode.Permissions...),
			RequiredCapabilities: append([]string(nil), cnode.RequiredCapabilities...),
			Scope:                cnode.Scope,
			Compensation:         cnode.Compensation,
			Step:                 WorkflowStepInput{OnError: WorkflowOnError{Mode: string(cnode.OnError.Mode)}},
		}
		if len(cnode.Runtime) > 0 {
			_ = json.Unmarshal(cnode.Runtime, &node.Runtime)
		}
		if cnode.Postcondition != nil {
			if raw, err := json.Marshal(cnode.Postcondition); err == nil {
				postcondition := json.RawMessage(raw)
				node.Step.Postcondition = &postcondition
			}
		}
		def.Nodes = append(def.Nodes, node)
	}
	for _, id := range dag.TopologicalOrder {
		if cnode, ok := dag.Nodes[id]; ok {
			appendNode(cnode)
		}
	}
	for _, cnode := range dag.Nodes {
		appendNode(cnode)
	}
	return def
}

func (e *WorkflowExecutor) compensateCompiledFailure(ctx context.Context, req CompiledExecuteRequest, result *ExecuteResult) {
	if e == nil || result == nil || result.Success || req.DAG == nil {
		return
	}
	def := workflowDefinitionFromCompiled(req.DAG)
	if !workflowNeedsDeclaredCompensation(def, result.Steps) {
		return
	}
	_ = e.markRunCompensating(context.WithoutCancel(ctx), result.ExecutionID)
	result.CompensationResults = e.compensateDeclaredWorkflow(ctx, def, result.Steps, req.Input, req.Context, req.Journal)
	result.Status = workflowResultTerminalStatus(result)
}

func (e *WorkflowExecutor) ExecuteCompiled(ctx context.Context, req CompiledExecuteRequest) (*ExecuteResult, error) {
	start := time.Now()

	if req.DAG == nil {
		return nil, fmt.Errorf("workflow: missing compiled DAG")
	}
	if req.DAG.WorkflowID == "" {
		return nil, fmt.Errorf("workflow: missing compiled workflow id")
	}
	normalizedOpts, normalizeErr := req.Opts.Normalize()
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	req.Opts = normalizedOpts
	if req.Context.ExecutionOptions.Mode == "" {
		req.Context.ExecutionOptions = normalizedOpts
	}
	if req.Opts.Mode == ExecutionModeControlled {
		compiledDef := workflowDefinitionFromCompiled(req.DAG)
		if missing := req.Opts.MissingControlledApprovals(compiledDef.Nodes); len(missing) > 0 {
			return &ExecuteResult{
				ExecutionID: req.Context.InvocationID, WorkflowID: req.DAG.WorkflowID,
				Status: RunStatusWaitingConfirmation, Accepted: true, ExecutionMode: req.Opts.Mode,
				RequiredConfirmations: append([]string(nil), missing...), Duration: time.Since(start),
			}, nil
		}
	}

	if req.Opts.IsDryRun() {
		dryResult := ExecuteDryRun(ctx, req.DAG)
		result := &ExecuteResult{
			ExecutionID: req.Context.InvocationID,
			WorkflowID:  req.DAG.WorkflowID,
			Status:      RunStatusSucceeded,
			Success:     true,
			Duration:    time.Since(start),
			Steps:       make([]StepResult, 0),
		}
		for _, id := range dryResult.WouldExecute {
			result.Steps = append(result.Steps, StepResult{NodeID: id, Status: "succeeded"})
		}
		for _, id := range dryResult.WouldSkip {
			result.Steps = append(result.Steps, StepResult{NodeID: id, Status: "skipped"})
		}
		return result, nil
	}

	execCtx := ctx
	var execCancel context.CancelFunc
	limits := req.DAG.Limits
	if limits.MaxExecutionDurationMS > 0 {
		execCtx, execCancel = context.WithTimeout(ctx, time.Duration(limits.MaxExecutionDurationMS)*time.Millisecond)
		if execCancel != nil {
			defer execCancel()
		}
	}

	executionID := req.Context.InvocationID
	if executionID == "" {
		executionID = fmt.Sprintf("%s-%d", req.DAG.WorkflowID, start.UnixNano())
		req.Context.InvocationID = executionID
	}
	if req.Context.RootID == "" {
		req.Context.RootID = executionID
	}
	if req.Context.WorkflowID == "" {
		req.Context.WorkflowID = req.DAG.WorkflowID
	}
	if req.Context.IdempotencyKey == "" {
		req.Context.IdempotencyKey = executionID
	}
	if req.Context.OperationID == "" {
		req.Context.OperationID = "wf-op-" + executionID
	}
	if req.Context.TraceID == "" {
		req.Context.TraceID = "wf-trace-" + executionID
	}
	if req.Journal == nil {
		if store, ok := e.runStore.(SideEffectStore); ok && store != nil {
			req.Journal = NewPersistentSideEffectJournal(context.WithoutCancel(execCtx), store, SideEffectJournalScope{
				ExecutionID: executionID,
				WorkflowID:  req.DAG.WorkflowID,
				Generation:  req.Context.Generation,
				DeviceID:    req.Context.DeviceID,
			})
		}
	}
	execCtx, runCancel := context.WithCancel(execCtx)
	e.activeMu.Lock()
	e.active[executionID] = runCancel
	e.activeMu.Unlock()
	control := e.ensureExecutionControl(executionID)
	execCtx = withWorkflowPauseSignal(execCtx, control.pauseSignal)
	defer func() {
		runCancel()
		e.activeMu.Lock()
		delete(e.active, executionID)
		e.activeMu.Unlock()
		e.removeExecutionControl(executionID)
	}()

	states := make(map[string]NodeState)
	for _, id := range req.DAG.TopologicalOrder {
		if len(req.DAG.DependedOnBy[id]) == 0 {
			states[id] = NodeStateReady
		} else {
			states[id] = NodeStateBlocked
		}
	}

	outputs := make(map[string]json.RawMessage)
	outputsMu := sync.RWMutex{}
	stepResults := make([]StepResult, 0, len(req.DAG.TopologicalOrder))
	var resultMu sync.Mutex

	mockLookup := BuildMockLookup(req.Opts.Mocks)
	totalRecorded := 0

	runReadySet := func(ready []string) (bool, string) {
		if len(ready) == 0 {
			return false, ""
		}

		type execResult struct {
			nodeID string
			step   StepResult
		}

		resultChan := make(chan execResult, len(ready))
		var wg sync.WaitGroup

		maxConcurrency := limits.MaxConcurrency
		if maxConcurrency <= 0 {
			maxConcurrency = 8
		}
		if maxConcurrency > len(ready) {
			maxConcurrency = len(ready)
		}
		semaphore := make(chan struct{}, maxConcurrency)

		for _, nid := range ready {
			wg.Add(1)
			go func(nodeID string) {
				defer wg.Done()
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				cnode := req.DAG.Nodes[nodeID]
				node := WorkflowNode{
					ID:              cnode.ID,
					Type:            cnode.Type,
					DependsOn:       cnode.DependsOn,
					TargetID:        cnode.TargetID,
					ExecutionTarget: cnode.ExecutionTarget,
					Permissions:     append([]string(nil), cnode.Permissions...),
					Scope:           cnode.Scope,
					Step: WorkflowStepInput{
						Input: json.RawMessage{},
					},
				}
				if len(cnode.Runtime) > 0 {
					if err := json.Unmarshal(cnode.Runtime, &node.Runtime); err != nil {
						resultChan <- execResult{
							nodeID: nodeID,
							step:   StepResult{NodeID: nodeID, Status: "failed", Error: "invalid compiled runtime binding: " + err.Error()},
						}
						return
					}
				}

				if mockOutput, mockErr, isMocked := req.Opts.EffectiveMockOutput(nodeID); isMocked {
					step := StepResult{NodeID: nodeID}
					if mockErr != "" {
						step.Status = "failed"
						step.Error = mockErr
						step.Attempt = 1
					} else {
						step.Status = "succeeded"
						step.Output = mockOutput
						step.Attempt = 1
					}
					resultChan <- execResult{nodeID: nodeID, step: step}
					return
				}
				if req.Opts.IsMocked() {
					if _, sideEffecting := sideEffectKindForNode(node); sideEffecting {
						resultChan <- execResult{nodeID: nodeID, step: StepResult{NodeID: nodeID, Status: "failed", Error: "mocked execution requires an explicit mock for side-effecting node"}}
						return
					}
				}

				var inputJSON json.RawMessage = req.Input
				if len(cnode.Input) > 0 {
					resolved := e.resolveInputTemplate(cnode.Input, req.Input, outputs, &outputsMu, req.Context)
					if merged, err := json.Marshal(resolved); err == nil {
						inputJSON = merged
					}
				}

				if cnode.When != nil {
					shouldRun, err := EvaluateExpression(cnode.When, ExpressionEvalConfig{
						Input: func() map[string]any {
							var m map[string]any
							_ = json.Unmarshal(req.Input, &m)
							if m == nil {
								m = make(map[string]any)
							}
							return m
						}(),
						Runtime: workflowRuntimeValues(req.Context),
						Outputs: func() map[string]json.RawMessage {
							outputsMu.RLock()
							defer outputsMu.RUnlock()
							cp := make(map[string]json.RawMessage, len(outputs))
							for k, v := range outputs {
								cp[k] = v
							}
							return cp
						}(),
					})
					if err != nil || !shouldRun {
						resultChan <- execResult{
							nodeID: nodeID,
							step:   StepResult{NodeID: nodeID, Status: "skipped"},
						}
						return
					}
				}

				handler, hOK := e.handlers[node.Type]
				if !hOK {
					handler, hOK = e.handlers[strings.ToLower(node.Type)]
				}
				if !hOK {
					resultChan <- execResult{
						nodeID: nodeID,
						step:   StepResult{NodeID: nodeID, Status: "failed", Error: ErrHandlerNotFound.Error()},
					}
					return
				}

				retryPolicy := cnode.Retry
				if retryPolicy == nil {
					retryPolicy = DefaultRetryPolicy()
				}
				normalizedRetry := retryPolicyNormalize(retryPolicy)

				if cnode.Postcondition != nil {
					handler = wrapPostconditionHandler(handler, nodeID, cnode.Postcondition, req.Input, workflowRuntimeValues(req.Context), func() map[string]json.RawMessage {
						outputsMu.RLock()
						defer outputsMu.RUnlock()
						snapshot := make(map[string]json.RawMessage, len(outputs))
						for key, value := range outputs {
							snapshot[key] = value
						}
						return snapshot
					})
				}

				stepResult := e.executeStepCompiled(withExecutionContext(execCtx, req.Context), handler, node, inputJSON, limits, cnode.Timeout, normalizedRetry, req.Journal, req.DAG.WorkflowID)
				stepResult.NodeID = nodeID

				if cnode.OnError.Mode == WorkflowErrorModeUseDefault && stepResult.Status == "failed" && len(cnode.OnError.Default) > 0 {
					stepResult.Status = "defaulted"
					stepResult.Output = cnode.OnError.Default
					stepResult.Error = ""
				}

				resultChan <- execResult{nodeID: nodeID, step: stepResult}
			}(nid)
		}

		wg.Wait()
		close(resultChan)

		aborted := false
		var abortReason string
		for er := range resultChan {
			resultMu.Lock()
			stepResults = append(stepResults, er.step)
			resultMu.Unlock()

			switch er.step.Status {
			case "succeeded", "defaulted":
				outputsMu.Lock()
				outputs[er.nodeID] = er.step.Output
				outputsMu.Unlock()
				states[er.nodeID] = NodeStateSucceeded
			case "skipped":
				states[er.nodeID] = NodeStateSkipped
			case "cancelled":
				states[er.nodeID] = NodeStateCancelled
				aborted = true
				abortReason = er.step.Error
			default:
				cnode := req.DAG.Nodes[er.nodeID]
				states[er.nodeID] = NodeStateFailed
				if cnode.OnError.Mode == WorkflowErrorModeContinue {
					if len(cnode.OnError.Default) > 0 {
						outputsMu.Lock()
						outputs[er.nodeID] = cnode.OnError.Default
						outputsMu.Unlock()
					}
				} else {
					aborted = true
					if abortReason == "" {
						abortReason = er.step.Error
					}
				}
			}
		}

		if req.Journal != nil {
			totalRecorded = req.Journal.Count()
		}
		_ = mockLookup
		_ = totalRecorded

		return aborted, abortReason
	}

	for {
		select {
		case <-execCtx.Done():
			result := &ExecuteResult{
				ExecutionID: executionID,
				WorkflowID:  req.DAG.WorkflowID,
				Status:      RunStatusFailed,
				Success:     false,
				Steps:       stepResults,
				Duration:    time.Since(start),
			}
			if execCtx.Err() == context.DeadlineExceeded {
				result.Error = ErrExecutionTimeout.Error()
			} else {
				result.Error = execCtx.Err().Error()
			}
			e.compensateCompiledFailure(context.WithoutCancel(ctx), req, result)
			return result, nil
		default:
		}

		ready := ReadyNodes(req.DAG, states)
		if len(ready) == 0 {
			allTerminal := true
			anyFailed := false
			for _, s := range states {
				if !s.IsTerminal() {
					allTerminal = false
					break
				}
				if s == NodeStateFailed {
					anyFailed = true
				}
			}
			if allTerminal {
				result := &ExecuteResult{
					ExecutionID: executionID,
					WorkflowID:  req.DAG.WorkflowID,
					Steps:       stepResults,
					Duration:    time.Since(start),
				}
				if anyFailed {
					result.Status = RunStatusFailed
					result.Success = false
					e.compensateCompiledFailure(context.WithoutCancel(ctx), req, result)
				} else {
					result.Status = RunStatusSucceeded
					result.Success = true
					for i := len(stepResults) - 1; i >= 0; i-- {
						if stepResults[i].Status == "succeeded" && len(stepResults[i].Output) > 0 {
							result.Output = stepResults[i].Output
							break
						}
					}
				}
				return result, nil
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}

		for _, id := range ready {
			states[id] = NodeStateRunning
		}

		aborted, abortReason := runReadySet(ready)
		if aborted {
			result := &ExecuteResult{
				ExecutionID: executionID,
				WorkflowID:  req.DAG.WorkflowID,
				Status:      RunStatusFailed,
				Success:     false,
				Error:       abortReason,
				Steps:       stepResults,
				Duration:    time.Since(start),
			}
			e.compensateCompiledFailure(context.WithoutCancel(ctx), req, result)
			return result, nil
		}
	}
}

func workflowRuntimeValues(execution ExecutionContext) map[string]any {
	return map[string]any{
		"userId":           execution.UserID,
		"rootId":           execution.RootID,
		"characterId":      execution.CharacterID,
		"conversationId":   execution.ConversationID,
		"operationId":      execution.OperationID,
		"invocationId":     execution.InvocationID,
		"scheduleId":       execution.ScheduleID,
		"triggerId":        execution.TriggerID,
		"traceId":          execution.TraceID,
		"idempotencyKey":   execution.IdempotencyKey,
		"nodeId":           execution.NodeID,
		"logicalAttempt":   execution.LogicalAttempt,
		"fencingToken":     execution.FencingToken,
		"depth":            execution.Depth,
		"recovery":         execution.Recovery,
		"scopeSnapshotId":  execution.ScopeSnapshotID,
		"permissionSnapId": execution.PermissionSnapID,
	}
}

func (e *WorkflowExecutor) resolveInputTemplate(template map[string]any, input json.RawMessage, outputs map[string]json.RawMessage, outputsMu *sync.RWMutex, execution ExecutionContext) map[string]any {
	var inputMap map[string]any
	_ = json.Unmarshal(input, &inputMap)
	if inputMap == nil {
		inputMap = make(map[string]any)
	}

	runtimeMap := workflowRuntimeValues(execution)

	resolved := resolveWorkflowTemplateValue(template, inputMap, runtimeMap, outputs, outputsMu)
	if mapped, ok := resolved.(map[string]any); ok {
		return mapped
	}
	return map[string]any{"input": resolved}
}

func resolveWorkflowTemplateValue(value any, inputMap, runtimeMap map[string]any, outputs map[string]json.RawMessage, outputsMu *sync.RWMutex) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = resolveWorkflowTemplateValue(child, inputMap, runtimeMap, outputs, outputsMu)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = resolveWorkflowTemplateValue(child, inputMap, runtimeMap, outputs, outputsMu)
		}
		return out
	case string:
		ref, err := ParseWorkflowValueRef(typed)
		if err != nil {
			return typed
		}
		switch ref.Source {
		case RefSourceInput:
			return traversePath(inputMap, ref.Path)
		case RefSourceRuntime:
			return traversePath(runtimeMap, ref.Path)
		case RefSourceNodeOutput:
			outputsMu.RLock()
			raw, ok := outputs[ref.NodeID]
			outputsMu.RUnlock()
			if !ok || len(raw) == 0 {
				return nil
			}
			var parsed any
			if err := json.Unmarshal(raw, &parsed); err != nil {
				return nil
			}
			return traversePath(parsed, ref.Path)
		case RefSourceLiteral:
			if len(ref.Path) > 0 {
				return ref.Path[0]
			}
			return ""
		default:
			// Kernel workflow-v2 does not have a workflow-level config object.
			// Preserve unknown/config refs verbatim instead of silently erasing data.
			return typed
		}
	default:
		return value
	}
}

func (e *WorkflowExecutor) executeStepCompiled(ctx context.Context, handler StepHandler, node WorkflowNode, input json.RawMessage, limits WorkflowLimits, nodeTimeout *time.Duration, retry *WorkflowRetryPolicy, journal *SideEffectJournal, workflowID string) StepResult {
	start := time.Now()

	retry = retryPolicyNormalize(retry)
	maxAttempts := retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return StepResult{Status: "cancelled", Error: ctx.Err().Error(), Duration: time.Since(start), Attempt: attempt - 1}
			}
			return StepResult{Status: "failed", Error: ErrExecutionTimeout.Error(), Duration: time.Since(start), Attempt: attempt - 1}
		default:
		}

		stepCtx := ctx
		var cancel context.CancelFunc
		timeout := time.Duration(0)
		if nodeTimeout != nil && *nodeTimeout > 0 {
			timeout = *nodeTimeout
			if limits.MaxStepDurationMS > 0 {
				maxTimeout := time.Duration(limits.MaxStepDurationMS) * time.Millisecond
				if timeout > maxTimeout {
					timeout = maxTimeout
				}
			}
		} else if limits.MaxStepDurationMS > 0 {
			timeout = time.Duration(limits.MaxStepDurationMS) * time.Millisecond
		}
		if timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, timeout)
		}

		attemptStarted := time.Now().UTC()
		attemptExecution, _ := ExecutionContextFromContext(ctx)
		attemptExecution.NodeID = node.ID
		logicalAttempt := int(attemptExecution.Generation) + 1
		if logicalAttempt < 1 {
			logicalAttempt = 1
		}
		attemptExecution.LogicalAttempt = logicalAttempt
		attemptExecution.IdempotencyKey = BuildStepIdempotencyKey(workflowID, attemptExecution.InvocationID, node.ID, logicalAttempt)

		handlerCtx := stepCtx
		lease, leaseErr := e.acquireStepExecutionLease(stepCtx, attemptExecution, node, timeout)
		if leaseErr == nil && lease.active {
			attemptExecution.FencingToken = lease.lease.FencingToken
			handlerCtx = lease.ctx
		}
		traceMetadata := &ExecutionTraceMetadata{
			DeviceID:  strings.TrimSpace(attemptExecution.DeviceID),
			RuntimeID: strings.TrimSpace(node.ExecutionTarget.RuntimeID),
		}
		if traceMetadata.DeviceID == "" {
			traceMetadata.DeviceID = strings.TrimSpace(node.ExecutionTarget.DeviceID)
		}
		if traceMetadata.RuntimeID == "" {
			traceMetadata.RuntimeID = strings.TrimSpace(node.Runtime.RuntimeID)
		}
		if strings.TrimSpace(node.TargetID) != "" || traceMetadata.RuntimeID != "" {
			traceMetadata.ToolCallID = fmt.Sprintf("%s/%s", attemptExecution.InvocationID, node.ID)
		}
		attemptCtx := WithExecutionTraceMetadata(withExecutionContext(handlerCtx, attemptExecution), traceMetadata)

		var output json.RawMessage
		var err error
		if leaseErr != nil {
			err = leaseErr
		} else {
			output, err = handler.Execute(attemptCtx, node, input)
		}
		stepErr := handlerCtx.Err()
		if lease.active {
			lease.close()
		}
		attemptFinished := time.Now().UTC()

		if journal != nil {
			if kind, sideEffecting := sideEffectKindForNode(node); sideEffecting {
				errText := ""
				if err != nil {
					errText = err.Error()
				} else if stepErr != nil {
					errText = stepErr.Error()
				}
				journal.RecordAttempt(node.ID, attempt, attemptExecution.IdempotencyKey, kind, node.TargetID, input, output, errText, attemptFinished.Sub(attemptStarted))
			}
		}
		if cancel != nil {
			cancel()
		}

		var pauseErr *WorkflowPauseError
		if err != nil && errors.As(err, &pauseErr) {
			e.saveAttempt(attemptCtx, workflowID, node, input, attempt, "paused", output, nil, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "paused", Output: output, Duration: time.Since(start), Attempt: attempt}
		}

		if stepErr == nil && err == nil {
			e.saveAttempt(attemptCtx, workflowID, node, input, attempt, "succeeded", output, nil, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "succeeded", Output: output, Duration: time.Since(start), Attempt: attempt}
		}

		lastErr = err
		attemptStatus := "failed"
		if stepErr == context.DeadlineExceeded {
			lastErr = ErrStepTimeout
			attemptStatus = "timed_out"
		} else if stepErr == context.Canceled && ctx.Err() != nil {
			lastErr = ctx.Err()
			attemptStatus = "cancelled"
			e.saveAttempt(attemptCtx, workflowID, node, input, attempt, attemptStatus, output, lastErr, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "cancelled", Output: output, Error: lastErr.Error(), Duration: time.Since(start), Attempt: attempt}
		} else if stepErr == context.Canceled && lease.active {
			lastErr = fmt.Errorf("workflow execution lease lost for node %s", node.ID)
		}
		if lastErr == nil {
			lastErr = errors.New("workflow step failed")
		}

		nextBackoff := time.Duration(0)
		if attempt < maxAttempts && retry.IsRetryable("") {
			nextBackoff = retry.ComputeBackoff(attempt)
		}
		e.saveAttempt(attemptCtx, workflowID, node, input, attempt, attemptStatus, output, lastErr, nextBackoff, attemptStarted, attemptFinished)
		if nextBackoff > 0 {
			select {
			case <-ctx.Done():
				return StepResult{Status: "cancelled", Error: ctx.Err().Error(), Duration: time.Since(start), Attempt: attempt}
			case <-time.After(nextBackoff):
			}
		}
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	return StepResult{Status: "failed", Error: errMsg, Duration: time.Since(start), Attempt: maxAttempts}
}

type workflowExecutionLeaseHandle struct {
	active bool
	ctx    context.Context
	cancel context.CancelFunc
	store  ExecutionLeaseStore
	lease  ExecutionLease
	stop   chan struct{}
}

func (h workflowExecutionLeaseHandle) close() {
	if !h.active {
		return
	}
	if h.stop != nil {
		select {
		case <-h.stop:
		default:
			close(h.stop)
		}
	}
	if h.cancel != nil {
		h.cancel()
	}
	if h.store != nil {
		_ = h.store.ReleaseExecutionLease(context.Background(), h.lease)
	}
}

func (e *WorkflowExecutor) acquireStepExecutionLease(ctx context.Context, execution ExecutionContext, node WorkflowNode, timeout time.Duration) (workflowExecutionLeaseHandle, error) {
	placement := node.ExecutionTarget.Placement
	if placement != WorkflowExecutionDevice && placement != WorkflowExecutionAuto {
		return workflowExecutionLeaseHandle{ctx: ctx}, nil
	}
	store, ok := e.runStore.(ExecutionLeaseStore)
	if !ok || store == nil {
		return workflowExecutionLeaseHandle{}, fmt.Errorf("workflow distributed execution lease store unavailable")
	}
	ownerDeviceID := strings.TrimSpace(node.ExecutionTarget.DeviceID)
	if ownerDeviceID == "" {
		ownerDeviceID = "auto"
	}
	ttl := DefaultExecutionLeaseTTL
	if timeout > 0 && timeout+30*time.Second > ttl {
		ttl = timeout + 30*time.Second
	}
	lease, err := store.AcquireExecutionLease(ctx, execution.InvocationID, node.ID, ownerDeviceID, execution.Generation, ttl)
	if err != nil {
		return workflowExecutionLeaseHandle{}, err
	}
	leaseCtx, cancel := context.WithCancel(ctx)
	handle := workflowExecutionLeaseHandle{
		active: true,
		ctx:    leaseCtx,
		cancel: cancel,
		store:  store,
		lease:  lease,
		stop:   make(chan struct{}),
	}
	interval := ttl / 3
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-handle.stop:
				return
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				if _, renewErr := store.RenewExecutionLease(context.Background(), lease, ttl); renewErr != nil {
					cancel()
					return
				}
			}
		}
	}()
	return handle, nil
}

func BuildStepIdempotencyKey(workflowID, executionID, nodeID string, logicalAttempt int) string {
	workflowID = strings.TrimSpace(workflowID)
	executionID = strings.TrimSpace(executionID)
	nodeID = strings.TrimSpace(nodeID)
	if logicalAttempt < 1 {
		logicalAttempt = 1
	}
	return fmt.Sprintf("wf:%s:run:%s:node:%s:attempt:%d", workflowID, executionID, nodeID, logicalAttempt)
}

func sideEffectKindForNode(node WorkflowNode) (SideEffectKind, bool) {
	switch strings.ToLower(strings.TrimSpace(node.Type)) {
	case "tool", "task", "mcp", "javascript", "wasm", "trusted_service", "trusted service", "runtime_handler":
		return SideEffectToolCall, true
	case "skill", "call_skill", "nested_workflow", "nested workflow":
		return SideEffectSkillInvoke, true
	case "http", "http_request":
		return SideEffectHTTPCall, true
	case "notification":
		return SideEffectNotification, true
	case "schedule":
		return SideEffectScheduleCreate, true
	case "memory_candidate", "memory_write":
		return SideEffectMemoryWrite, true
	case "artifact", "artifact_write", "file_write":
		return SideEffectArtifactWrite, true
	case "context_contribution", "context_write":
		return SideEffectContextWrite, true
	default:
		return "", false
	}
}

func retryPolicyNormalize(p *WorkflowRetryPolicy) *WorkflowRetryPolicy {
	if p == nil {
		return DefaultRetryPolicy()
	}
	return p.Normalize()
}

func (e *WorkflowExecutor) ExecuteWithOptions(ctx context.Context, req CompiledExecuteRequest) (*ExecuteResult, error) {
	return e.ExecuteCompiled(ctx, req)
}

func (e *WorkflowExecutor) CompileAndExecute(ctx context.Context, def WorkflowDefinition, input json.RawMessage, execContext ExecutionContext, opts CompileOptions) (*ExecuteResult, error) {
	compiler := NewCompiler()
	dag, err := compiler.Compile(def, opts)
	if err != nil {
		return nil, err
	}
	req := CompiledExecuteRequest{
		DAG:     dag,
		Input:   input,
		Context: execContext,
		Opts:    DefaultExecutionOptions(),
	}
	return e.ExecuteCompiled(ctx, req)
}

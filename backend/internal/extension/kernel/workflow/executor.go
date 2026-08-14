package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type StepHandler interface {
	Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error)
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
}

type WorkflowExecutionControl struct {
	executionID    string
	pauseRequested chan struct{}
	paused         chan struct{}
}

type StepGuard interface {
	Check(ctx context.Context, definition WorkflowDefinition, node WorkflowNode, execution ExecutionContext) error
}

type ExecuteRequest struct {
	WorkflowID string
	Input      json.RawMessage
	Context    ExecutionContext
}

type ExecutionContext struct {
	ExtensionID      string
	CharacterID      string
	ConversationID   string
	OperationID      string
	InvocationID     string
	ScopeSnapshotID  string
	PermissionSnapID string
	Generation       int64
	ModuleID         string
	ScheduleID       string
	TriggerID        string
	TraceID          string
	IdempotencyKey   string
	Depth            int
	Recovery         bool
}

type ExecuteResult struct {
	ExecutionID         string
	WorkflowID          string
	Status              RunStatus
	Accepted            bool
	Output              json.RawMessage
	Steps               []StepResult
	Success             bool
	Error               string
	Duration            time.Duration
	CompensationResults []CompensationResult
}

type StepResult struct {
	NodeID   string
	Status   string
	Output   json.RawMessage
	Error    string
	Duration time.Duration
	Attempt  int
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

func (e *WorkflowExecutor) RunStore() RunStore {
	return e.runStore
}

func (e *WorkflowExecutor) CheckpointStore() CheckpointStore {
	return e.checkpoint
}

func (e *WorkflowExecutor) SetStepGuard(guard StepGuard) {
	e.guard = guard
}

func (e *WorkflowExecutor) Recover(ctx context.Context, limit int) error {
	if e.runStore == nil {
		return nil
	}
	runs, err := e.runStore.ListRecoverable(ctx, limit)
	if err != nil {
		return err
	}
	for _, run := range runs {
		run.Context.Recovery = true
		run.Generation++
		if _, err := e.Execute(ctx, ExecuteRequest{WorkflowID: run.WorkflowID, Input: run.Input, Context: run.Context}); err != nil {
			finishedAt := time.Now().UTC()
			run.Status = RunStatusFailed
			run.Error = err.Error()
			run.FinishedAt = &finishedAt
			run.UpdatedAt = finishedAt
			_ = e.runStore.Finish(context.Background(), run)
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

	e.pauseMu.Lock()
	ctrl, exists := e.pauseControls[executionID]
	if !exists {
		ctrl = &WorkflowExecutionControl{
			executionID:    executionID,
			pauseRequested: make(chan struct{}, 1),
			paused:         make(chan struct{}),
		}
		e.pauseControls[executionID] = ctrl
	}
	select {
	case ctrl.pauseRequested <- struct{}{}:
	default:
	}
	e.pauseMu.Unlock()

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
	updated.UpdatedAt = now

	ok, err := e.runStore.UpdateStateCAS(ctx, updated, RunStatusPaused)
	if err != nil {
		return nil, fmt.Errorf("workflow resume cas: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("workflow resume: concurrent state change")
	}

	go func() {
		_, _ = e.Execute(context.Background(), ExecuteRequest{
			WorkflowID: updated.WorkflowID,
			Input:      updated.Input,
			Context:    updated.Context,
		})
	}()

	return &updated, nil
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
		close(ctrl.paused)
	}
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

func (e *WorkflowExecutor) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	start := time.Now()

	wf, ok := e.registry.Get(req.WorkflowID)
	if !ok {
		return nil, ErrWorkflowNotFound
	}

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
	defer func() {
		runCancel()
		e.activeMu.Lock()
		delete(e.active, executionID)
		e.activeMu.Unlock()
		e.removeExecutionControl(executionID)
	}()

	result := &ExecuteResult{
		ExecutionID: executionID,
		WorkflowID:  req.WorkflowID,
		Status:      RunStatusRunning,
		Steps:       make([]StepResult, 0, totalNodes),
	}
	defer func() {
		if result.Accepted && result.Status == RunStatusRunning {
			return
		}
		if result.Success {
			result.Status = RunStatusSucceeded
		} else if compensationSucceeded(result.CompensationResults) {
			result.Status = RunStatusCompensated
		} else if strings.Contains(strings.ToLower(result.Error), "canceled") {
			result.Status = RunStatusCancelled
		} else {
			result.Status = RunStatusFailed
		}
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
		defer func() {
			finishedAt := time.Now().UTC()
			status := result.Status
			if result.Success {
				status = RunStatusSucceeded
			} else if compensationSucceeded(result.CompensationResults) {
				status = RunStatusCompensated
			} else if strings.Contains(strings.ToLower(result.Error), "canceled") {
				status = RunStatusCancelled
			} else {
				status = RunStatusFailed
			}
			_ = e.runStore.Finish(context.Background(), WorkflowRun{
				ExecutionID:         executionID,
				WorkflowID:          req.WorkflowID,
				Status:              status,
				Input:               req.Input,
				Output:              result.Output,
				Error:               result.Error,
				Context:             req.Context,
				Steps:               result.Steps,
				CompensationResults: result.CompensationResults,
				StartedAt:           startedAt,
				FinishedAt:          &finishedAt,
				UpdatedAt:           finishedAt,
			})
		}()
	}

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
			result.Duration = time.Since(start)
			return result, nil
		default:
		}

		if ctrl := e.getExecutionControl(executionID); ctrl != nil {
			select {
			case <-ctrl.pauseRequested:
				result.Success = false
				result.Error = "paused"
				result.Duration = time.Since(start)
				go e.finalisePaused(context.Background(), executionID, currentGeneration)
				return result, nil
			default:
			}
		}

		type nodeExecResult struct {
			nodeID string
			step   StepResult
			input  json.RawMessage
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
							nodeID: nid,
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

				input := req.Input
				if len(node.Step.Input) > 0 {
					input = node.Step.Input
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
					shouldExecute, _ := evaluateWhen(*node.Step.When)
					if !shouldExecute {
						resultChan <- nodeExecResult{
							nodeID: nid,
							step: StepResult{
								NodeID: nid,
								Status: "skipped",
							},
						}
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

				stepResult := e.executeStep(withExecutionContext(execCtx, req.Context), handler, node, input, wf.Limits)
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

		for ner := range resultChan {
			result.Steps = append(result.Steps, ner.step)
			if e.runStore != nil {
				finishedAt := time.Now().UTC()
				_ = e.runStore.SaveStep(execCtx, StepRun{ExecutionID: executionID, WorkflowID: req.WorkflowID, NodeID: ner.nodeID, Status: ner.step.Status, Input: ner.input, Output: ner.step.Output, Error: ner.step.Error, Attempt: ner.step.Attempt, StartedAt: finishedAt.Add(-ner.step.Duration), FinishedAt: &finishedAt})
			}

			if ner.step.Status == "succeeded" {
				outputsMu.Lock()
				outputs[ner.nodeID] = ner.step.Output
				outputsMu.Unlock()

				if e.checkpoint != nil {
					_ = e.checkpoint.Save(execCtx, Checkpoint{
						WorkflowID:  req.WorkflowID,
						ExecutionID: executionID,
						NodeID:      ner.nodeID,
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
				node := nodeMap[ner.nodeID]
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
				node := nodeMap[ner.nodeID]
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
	}

	if failed {
		result.Success = false
		result.Error = failError

		if e.compensation != nil {
			result.CompensationResults = e.compensation.Compensate(ctx, result.Steps)
		}

		result.Duration = time.Since(start)
		return result, nil
	}

	var lastOutput json.RawMessage = req.Input
	if len(result.Steps) > 0 {
		for i := len(result.Steps) - 1; i >= 0; i-- {
			if result.Steps[i].Status == "succeeded" && len(result.Steps[i].Output) > 0 {
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

type executionContextKey struct{}

func withExecutionContext(ctx context.Context, execution ExecutionContext) context.Context {
	return context.WithValue(ctx, executionContextKey{}, execution)
}

func ExecutionContextFromContext(ctx context.Context) (ExecutionContext, bool) {
	execution, ok := ctx.Value(executionContextKey{}).(ExecutionContext)
	return execution, ok
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

func (e *WorkflowExecutor) executeStep(ctx context.Context, handler StepHandler, node WorkflowNode, input json.RawMessage, limits WorkflowLimits) StepResult {
	start := time.Now()

	maxAttempts := 1
	if node.Step.OnError.Mode == "retry" {
		maxAttempts = e.retryMax + 1
		if maxAttempts < 1 {
			maxAttempts = 1
		}
	}

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return StepResult{
					Status:   "cancelled",
					Error:    ctx.Err().Error(),
					Duration: time.Since(start),
					Attempt:  attempt - 1,
				}
			}
			return StepResult{
				Status:   "failed",
				Error:    ErrExecutionTimeout.Error(),
				Duration: time.Since(start),
				Attempt:  attempt - 1,
			}
		default:
		}

		stepCtx := ctx
		var cancel context.CancelFunc
		if limits.MaxStepDurationMS > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, time.Duration(limits.MaxStepDurationMS)*time.Millisecond)
		}

		output, err := handler.Execute(stepCtx, node, input)
		if cancel != nil {
			cancel()
		}

		if err == nil {
			return StepResult{
				Status:   "succeeded",
				Output:   output,
				Duration: time.Since(start),
				Attempt:  attempt,
			}
		}

		lastErr = err

		if stepCtx.Err() == context.DeadlineExceeded {
			lastErr = ErrStepTimeout
		}

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				if ctx.Err() == context.Canceled {
					return StepResult{
						Status:   "cancelled",
						Error:    ctx.Err().Error(),
						Duration: time.Since(start),
						Attempt:  attempt,
					}
				}
				return StepResult{
					Status:   "failed",
					Error:    ErrExecutionTimeout.Error(),
					Duration: time.Since(start),
					Attempt:  attempt,
				}
			case <-time.After(time.Duration(attempt) * 100 * time.Millisecond):
			}
		}
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}

	return StepResult{
		Status:   "failed",
		Error:    errMsg,
		Duration: time.Since(start),
		Attempt:  maxAttempts,
	}
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

func (e *WorkflowExecutor) ExecuteCompiled(ctx context.Context, req CompiledExecuteRequest) (*ExecuteResult, error) {
	start := time.Now()

	if req.DAG == nil {
		return nil, fmt.Errorf("workflow: missing compiled DAG")
	}
	if req.DAG.WorkflowID == "" {
		return nil, fmt.Errorf("workflow: missing compiled workflow id")
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
	}
	execCtx, runCancel := context.WithCancel(execCtx)
	e.activeMu.Lock()
	e.active[executionID] = runCancel
	e.activeMu.Unlock()
	defer func() {
		runCancel()
		e.activeMu.Lock()
		delete(e.active, executionID)
		e.activeMu.Unlock()
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
					ID:        cnode.ID,
					Type:      cnode.Type,
					DependsOn: cnode.DependsOn,
					TargetID:  cnode.TargetID,
					Scope:     cnode.Scope,
					Step: WorkflowStepInput{
						Input: json.RawMessage{},
					},
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

				var inputJSON json.RawMessage = req.Input
				if len(cnode.DataRefs) > 0 {
					resolved := e.resolveDataRefs(cnode.DataRefs, req.Input, outputs, &outputsMu)
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

				stepResult := e.executeStepCompiled(execCtx, handler, node, inputJSON, limits, normalizedRetry, req.Journal)
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
			return result, nil
		}
	}
}

func (e *WorkflowExecutor) resolveDataRefs(refs []*WorkflowValueRef, input json.RawMessage, outputs map[string]json.RawMessage, outputsMu *sync.RWMutex) map[string]any {
	resolved := make(map[string]any)
	var inputMap map[string]any
	_ = json.Unmarshal(input, &inputMap)
	if inputMap == nil {
		inputMap = make(map[string]any)
	}

	for _, ref := range refs {
		switch ref.Source {
		case RefSourceInput:
			if v := traversePath(inputMap, ref.Path); v != nil {
				key := strings.Join(ref.Path, ".")
				resolved[key] = v
			}
		case RefSourceNodeOutput:
			outputsMu.RLock()
			raw, ok := outputs[ref.NodeID]
			outputsMu.RUnlock()
			if ok {
				var outMap map[string]any
				if err := json.Unmarshal(raw, &outMap); err == nil {
					if v := traversePath(outMap, ref.Path); v != nil {
						key := ref.NodeID + "." + strings.Join(ref.Path, ".")
						resolved[key] = v
					}
				}
			}
		}
	}
	return resolved
}

func (e *WorkflowExecutor) executeStepCompiled(ctx context.Context, handler StepHandler, node WorkflowNode, input json.RawMessage, limits WorkflowLimits, retry *WorkflowRetryPolicy, journal *SideEffectJournal) StepResult {
	start := time.Now()

	maxAttempts := retry.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			if ctx.Err() == context.Canceled {
				return StepResult{Status: "cancelled", Error: ctx.Err().Error(), Duration: time.Since(start), Attempt: attempt}
			}
			return StepResult{Status: "failed", Error: ErrExecutionTimeout.Error(), Duration: time.Since(start), Attempt: attempt}
		default:
		}

		stepCtx := ctx
		var cancel context.CancelFunc
		if limits.MaxStepDurationMS > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, time.Duration(limits.MaxStepDurationMS)*time.Millisecond)
		}

		output, err := handler.Execute(stepCtx, node, input)
		if cancel != nil {
			cancel()
		}

		if err == nil {
			if journal != nil {
				journal.Record(node.ID, SideEffectToolCall, node.TargetID, input, output, "", time.Since(start))
			}
			return StepResult{Status: "succeeded", Output: output, Duration: time.Since(start), Attempt: attempt}
		}

		lastErr = err
		if stepCtx.Err() == context.DeadlineExceeded {
			lastErr = ErrStepTimeout
		}

		if attempt < maxAttempts && retry.IsRetryable("") {
			backoff := retry.ComputeBackoff(attempt)
			select {
			case <-ctx.Done():
				return StepResult{Status: "cancelled", Error: ctx.Err().Error(), Duration: time.Since(start), Attempt: attempt}
			case <-time.After(backoff):
			}
		}
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	return StepResult{Status: "failed", Error: errMsg, Duration: time.Since(start), Attempt: maxAttempts}
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

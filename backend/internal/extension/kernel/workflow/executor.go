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
	UserID           string
	RootID           string
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
	DefinitionHash   string
	Depth            int
	Recovery         bool
}

type ExecuteResult struct {
	ExecutionID         string               `json:"executionId"`
	WorkflowID          string               `json:"workflowId"`
	Status              RunStatus            `json:"status"`
	Accepted            bool                 `json:"accepted"`
	Output              json.RawMessage      `json:"output,omitempty"`
	Steps               []StepResult         `json:"steps,omitempty"`
	Success             bool                 `json:"success"`
	Error               string               `json:"error,omitempty"`
	Duration            time.Duration        `json:"duration"`
	CompensationResults []CompensationResult `json:"compensationResults,omitempty"`
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

// CancelRun cancels an active execution and also supports cancelling a durable
// paused/orphaned run that no longer has an in-memory cancel function.
func (e *WorkflowExecutor) CancelRun(ctx context.Context, executionID string) (bool, error) {
	if e.Cancel(executionID) {
		return true, nil
	}
	if e.runStore == nil {
		return false, nil
	}
	run, err := e.runStore.Get(ctx, executionID)
	if err != nil {
		return false, err
	}
	if run.Status.IsTerminal() {
		return false, nil
	}
	now := time.Now().UTC()
	updated := *run
	updated.Status = RunStatusCancelled
	updated.Error = "cancelled"
	updated.FinishedAt = &now
	updated.UpdatedAt = now
	ok, err := e.runStore.UpdateStateCAS(ctx, updated, run.Status)
	if err != nil || !ok {
		return ok, err
	}
	if err := e.runStore.Finish(ctx, updated); err != nil {
		return false, err
	}
	return true, nil
}

func (e *WorkflowExecutor) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResult, error) {
	start := time.Now()

	wf, ok := e.registry.Get(req.WorkflowID)
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	definitionHash := ComputeDefinitionHash(wf)
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
	if req.Context.OperationID == "" {
		req.Context.OperationID = "wf-op-" + executionID
	}
	if req.Context.TraceID == "" {
		req.Context.TraceID = "wf-trace-" + executionID
	}
	if req.Context.IdempotencyKey == "" {
		req.Context.IdempotencyKey = executionID
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
		if result.Status == RunStatusPaused || result.Status == RunStatusPausing {
			return
		}
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
			if result.Status == RunStatusPaused || result.Status == RunStatusPausing {
				return
			}
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

				stepResult := e.executeStep(withExecutionContext(execCtx, req.Context), handler, node, input, wf.Limits, req.WorkflowID)
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
			node := nodeMap[ner.nodeID]
			if ner.step.Status == "failed" && node.Step.OnError.Mode == "use_default" && len(node.Step.OnError.Default) > 0 {
				ner.step.Status = "defaulted"
				ner.step.Output = node.Step.OnError.Default
				ner.step.Error = ""
			}
			result.Steps = append(result.Steps, ner.step)
			if e.runStore != nil && !ner.restored {
				finishedAt := time.Now().UTC()
				_ = e.runStore.SaveStep(execCtx, StepRun{ExecutionID: executionID, WorkflowID: req.WorkflowID, NodeID: ner.nodeID, Status: ner.step.Status, Input: ner.input, Output: ner.step.Output, Error: ner.step.Error, Attempt: ner.step.Attempt, StartedAt: finishedAt.Add(-ner.step.Duration), FinishedAt: &finishedAt})
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
	_ = store.SaveAttempt(persistCtx, StepAttemptRun{
		ExecutionID:   execution.InvocationID,
		WorkflowID:    workflowID,
		NodeID:        node.ID,
		Attempt:       attempt,
		Generation:    execution.Generation,
		Status:        status,
		Input:         input,
		Output:        output,
		Error:         errorMessage,
		NextBackoffMS: nextBackoff.Milliseconds(),
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
	})
}

func (e *WorkflowExecutor) executeStep(ctx context.Context, handler StepHandler, node WorkflowNode, input json.RawMessage, limits WorkflowLimits, workflowID string) StepResult {
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
		output, err := handler.Execute(stepCtx, node, input)
		attemptFinished := time.Now().UTC()
		stepErr := stepCtx.Err()
		if cancel != nil {
			cancel()
		}

		if stepErr == nil && err == nil {
			e.saveAttempt(ctx, workflowID, node, input, attempt, "succeeded", output, nil, 0, attemptStarted, attemptFinished)
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
			e.saveAttempt(ctx, workflowID, node, input, attempt, attemptStatus, nil, lastErr, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "cancelled", Error: lastErr.Error(), Duration: time.Since(start), Attempt: attempt}
		}

		nextBackoff := time.Duration(0)
		if attempt < maxAttempts && retryPolicy.IsRetryable("") {
			nextBackoff = retryPolicy.ComputeBackoff(attempt)
		}
		e.saveAttempt(ctx, workflowID, node, input, attempt, attemptStatus, output, lastErr, nextBackoff, attemptStarted, attemptFinished)

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
		req.Context.InvocationID = executionID
	}
	if req.Context.RootID == "" {
		req.Context.RootID = executionID
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
		} else if limits.MaxStepDurationMS > 0 {
			timeout = time.Duration(limits.MaxStepDurationMS) * time.Millisecond
		}
		if timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, timeout)
		}

		attemptStarted := time.Now().UTC()
		output, err := handler.Execute(stepCtx, node, input)
		attemptFinished := time.Now().UTC()
		stepErr := stepCtx.Err()
		if cancel != nil {
			cancel()
		}

		if stepErr == nil && err == nil {
			e.saveAttempt(ctx, workflowID, node, input, attempt, "succeeded", output, nil, 0, attemptStarted, attemptFinished)
			if journal != nil {
				journal.Record(node.ID, SideEffectToolCall, node.TargetID, input, output, "", time.Since(start))
			}
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
			e.saveAttempt(ctx, workflowID, node, input, attempt, attemptStatus, nil, lastErr, 0, attemptStarted, attemptFinished)
			return StepResult{Status: "cancelled", Error: lastErr.Error(), Duration: time.Since(start), Attempt: attempt}
		}

		nextBackoff := time.Duration(0)
		if attempt < maxAttempts && retry.IsRetryable("") {
			nextBackoff = retry.ComputeBackoff(attempt)
		}
		e.saveAttempt(ctx, workflowID, node, input, attempt, attemptStatus, output, lastErr, nextBackoff, attemptStarted, attemptFinished)
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

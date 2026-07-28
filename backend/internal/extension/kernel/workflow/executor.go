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
	registry     *WorkflowRegistry
	handlers     map[string]StepHandler
	checkpoint   CheckpointStore
	compensation *CompensationManager
	retryMax     int
	runStore     RunStore
	guard        StepGuard
	activeMu     sync.Mutex
	active       map[string]context.CancelFunc
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
		registry: registry,
		handlers: make(map[string]StepHandler),
		retryMax: 3,
		active:   make(map[string]context.CancelFunc),
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

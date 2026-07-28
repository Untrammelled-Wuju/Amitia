package workflow

import (
	"context"
	"encoding/json"
	"fmt"
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
}

type ExecuteResult struct {
	WorkflowID           string
	Output               json.RawMessage
	Steps                []StepResult
	Success              bool
	Error                string
	Duration             time.Duration
	CompensationResults   []CompensationResult
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

	result := &ExecuteResult{
		WorkflowID: req.WorkflowID,
		Steps:      make([]StepResult, 0, totalNodes),
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
		}

		resultChan := make(chan nodeExecResult, len(level))
		var wg sync.WaitGroup

		for _, nodeID := range level {
			wg.Add(1)
			go func(nid string) {
				defer wg.Done()

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

				stepResult := e.executeStep(execCtx, handler, node, input, wf.Limits)
				stepResult.NodeID = nid
				resultChan <- nodeExecResult{
					nodeID: nid,
					step:   stepResult,
				}
			}(nodeID)
		}

		wg.Wait()
		close(resultChan)

		for ner := range resultChan {
			result.Steps = append(result.Steps, ner.step)

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
	result.Duration = time.Since(start)
	return result, nil
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

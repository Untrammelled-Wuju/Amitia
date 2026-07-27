package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type StepHandler interface {
	Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error)
}

type WorkflowExecutor struct {
	registry   *WorkflowRegistry
	handlers   map[string]StepHandler
	checkpoint CheckpointStore
	retryMax   int
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
	WorkflowID string
	Output     json.RawMessage
	Steps      []StepResult
	Success    bool
	Error      string
	Duration   time.Duration
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

	order, err := TopologicalSort(wf.Nodes)
	if err != nil {
		return nil, err
	}

	if wf.Limits.MaxSteps > 0 && len(order) > wf.Limits.MaxSteps {
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
		Steps:      make([]StepResult, 0, len(order)),
	}

	outputs := make(map[string]json.RawMessage)
	nodeMap := make(map[string]WorkflowNode, len(wf.Nodes))
	for _, node := range wf.Nodes {
		nodeMap[node.ID] = node
	}

	var lastOutput json.RawMessage = req.Input

	for _, nodeID := range order {
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

		node := nodeMap[nodeID]

		if e.checkpoint != nil {
			cp, cpErr := e.checkpoint.Load(execCtx, executionID, nodeID)
			if cpErr == nil && cp != nil {
				result.Steps = append(result.Steps, StepResult{
					NodeID:  nodeID,
					Status:  "succeeded",
					Output:  cp.Output,
					Attempt: 0,
				})
				outputs[nodeID] = cp.Output
				lastOutput = cp.Output
				continue
			}
		}

		input := lastOutput
		if len(node.Step.Input) > 0 {
			input = node.Step.Input
		}

		if node.Step.When != nil && len(*node.Step.When) > 0 {
			shouldExecute, _ := evaluateWhen(*node.Step.When)
			if !shouldExecute {
				result.Steps = append(result.Steps, StepResult{
					NodeID: nodeID,
					Status: "skipped",
				})
				if len(node.Step.OnError.Default) > 0 {
					outputs[nodeID] = node.Step.OnError.Default
					lastOutput = node.Step.OnError.Default
				}
				continue
			}
		}

		handler, ok := e.handlers[node.Type]
		if !ok {
			result.Steps = append(result.Steps, StepResult{
				NodeID: nodeID,
				Status: "failed",
				Error:  ErrHandlerNotFound.Error(),
			})
			result.Success = false
			result.Error = fmt.Sprintf("handler not found for node type: %s", node.Type)
			result.Duration = time.Since(start)
			return result, nil
		}

		stepResult := e.executeStep(execCtx, handler, node, input, wf.Limits)
		stepResult.NodeID = nodeID
		result.Steps = append(result.Steps, stepResult)

		if stepResult.Status == "succeeded" {
			outputs[nodeID] = stepResult.Output
			lastOutput = stepResult.Output

			if e.checkpoint != nil {
				_ = e.checkpoint.Save(execCtx, Checkpoint{
					WorkflowID:  req.WorkflowID,
					ExecutionID: executionID,
					NodeID:      nodeID,
					Input:       input,
					Output:      stepResult.Output,
					CompletedAt: time.Now(),
				})
			}

			if wf.Limits.MaxOutputBytes > 0 && int64(len(stepResult.Output)) > wf.Limits.MaxOutputBytes {
				result.Success = false
				result.Error = ErrOutputLimitExceeded.Error()
				result.Duration = time.Since(start)
				return result, nil
			}
		} else if stepResult.Status == "cancelled" {
			result.Success = false
			result.Error = stepResult.Error
			result.Duration = time.Since(start)
			return result, nil
		} else {
			switch node.Step.OnError.Mode {
			case "continue":
				if len(node.Step.OnError.Default) > 0 {
					outputs[nodeID] = node.Step.OnError.Default
					lastOutput = node.Step.OnError.Default
				}
			case "retry":
				if len(node.Step.OnError.Default) > 0 {
					outputs[nodeID] = node.Step.OnError.Default
					lastOutput = node.Step.OnError.Default
				} else {
					result.Success = false
					result.Error = stepResult.Error
					result.Duration = time.Since(start)
					return result, nil
				}
			default:
				result.Success = false
				result.Error = stepResult.Error
				result.Duration = time.Since(start)
				return result, nil
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

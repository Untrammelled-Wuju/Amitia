package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PassthroughHandler struct{}

func (PassthroughHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return input, nil
}

type TransformHandler struct{}

func (TransformHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	field, _ := node.Runtime.Metadata["field"].(string)
	if field == "" {
		return input, nil
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(input, &value); err != nil {
		return nil, fmt.Errorf("transform input: %w", err)
	}
	selected, ok := value[field]
	if !ok {
		return nil, fmt.Errorf("transform field %s not found", field)
	}
	return selected, nil
}

type WaitHandler struct{}

func (WaitHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	durationMS := int64(0)
	if raw, ok := node.Runtime.Metadata["durationMs"].(float64); ok {
		durationMS = int64(raw)
	}
	if durationMS == 0 {
		var payload struct {
			DurationMS int64 `json:"durationMs"`
		}
		_ = json.Unmarshal(input, &payload)
		durationMS = payload.DurationMS
	}
	if durationMS < 0 {
		return nil, fmt.Errorf("wait duration must not be negative")
	}
	timer := time.NewTimer(time.Duration(durationMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return input, nil
	}
}

type NestedWorkflowHandler struct {
	Executor *WorkflowExecutor
}

func (h NestedWorkflowHandler) Execute(ctx context.Context, node WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	if h.Executor == nil {
		return nil, fmt.Errorf("nested workflow executor not configured")
	}
	execution, ok := ExecutionContextFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("nested workflow context missing")
	}
	targetID := strings.TrimSpace(node.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(node.Runtime.RuntimeID)
	}
	if targetID == "" {
		return nil, fmt.Errorf("nested workflow target missing")
	}

	target := node.ExecutionTarget.Normalized(WorkflowExecutionLocal)
	if node.ExecutionTarget.Placement == WorkflowExecutionDevice {
		runner := h.Executor.RemoteWorkflowRunner()
		if runner == nil {
			err := fmt.Errorf("remote workflow runner not configured")
			if target.OfflinePolicy == WorkflowOfflineWait {
				return nil, &WorkflowDeviceUnavailableError{DeviceID: target.DeviceID, Cause: err}
			}
			return nil, err
		}
		// Execute has already appended the currently running workflow frame to
		// CallStack. Forward that complete distributed stack unchanged so the
		// target executor can append its own frame and reject Cloud -> Device ->
		// Cloud (or cross-device) recursion consistently.
		execution.Depth++
		execution.InvocationID = fmt.Sprintf("%s/%s", execution.InvocationID, node.ID)
		execution.IdempotencyKey = fmt.Sprintf("%s/%s", execution.IdempotencyKey, node.ID)
		output, err := runner.RunRemoteWorkflow(ctx, RemoteWorkflowRequest{
			WorkflowID: targetID,
			Input:      input,
			Target:     target,
			Context:    execution,
		})
		if err != nil && target.OfflinePolicy == WorkflowOfflineWait {
			var unavailable *WorkflowDeviceUnavailableError
			if errors.As(err, &unavailable) {
				return nil, unavailable
			}
		}
		return output, err
	}
	if node.ExecutionTarget.Placement == WorkflowExecutionAuto {
		return nil, fmt.Errorf("nested workflow auto placement requires an explicit device workflow selection")
	}

	definition, exists := h.Executor.registry.Get(targetID)
	if !exists {
		return nil, ErrWorkflowNotFound
	}
	if definition.Source == "user" {
		owner := ""
		if definition.Metadata != nil {
			if value, exists := definition.Metadata["ownerUserId"]; exists && value != nil {
				owner = strings.TrimSpace(fmt.Sprint(value))
			}
		}
		if execution.UserID == "" || owner == "" || owner != execution.UserID {
			return nil, fmt.Errorf("%w: nested user workflow owner mismatch", ErrScopeDenied)
		}
	}
	execution.Depth++
	execution.InvocationID = fmt.Sprintf("%s/%s", execution.InvocationID, node.ID)
	execution.IdempotencyKey = fmt.Sprintf("%s/%s", execution.IdempotencyKey, node.ID)
	result, err := h.Executor.Execute(ctx, ExecuteRequest{WorkflowID: targetID, Input: input, Context: execution})
	if err != nil {
		return nil, err
	}
	if !result.Success {
		return nil, fmt.Errorf("nested workflow failed: %s", result.Error)
	}
	return result.Output, nil
}

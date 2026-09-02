package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const CompensationNodeSuffix = "::compensation"

type CompensationAction struct {
	NodeID  string
	Handler func(ctx context.Context, output json.RawMessage) error
}

type CompensationResult struct {
	NodeID     string    `json:"nodeId"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	ExecutedAt time.Time `json:"executedAt"`
}

// CompensationRecord is the durable state of one declared Saga reverse step.
// NodeID is the original forward node; the synthetic compensation execution
// node is derived deterministically with CompensationNodeSuffix.
type CompensationRecord struct {
	ExecutionID    string          `json:"executionId"`
	WorkflowID     string          `json:"workflowId"`
	NodeID         string          `json:"nodeId"`
	Generation     int64           `json:"generation"`
	Status         string          `json:"status"`
	Attempt        int             `json:"attempt"`
	IdempotencyKey string          `json:"idempotencyKey"`
	Input          json.RawMessage `json:"input,omitempty"`
	Output         json.RawMessage `json:"output,omitempty"`
	Error          string          `json:"error,omitempty"`
	StartedAt      time.Time       `json:"startedAt"`
	CompletedAt    *time.Time      `json:"completedAt,omitempty"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

type CompensationStore interface {
	SaveCompensation(ctx context.Context, record CompensationRecord) error
	GetCompensation(ctx context.Context, executionID, nodeID string) (*CompensationRecord, error)
	ListCompensations(ctx context.Context, executionID string) ([]CompensationRecord, error)
}

type CompensationStepRunStore interface {
	ListStepRuns(ctx context.Context, executionID string) ([]StepRun, error)
}

func BuildCompensationNode(node WorkflowNode) (WorkflowNode, error) {
	if node.Compensation == nil {
		return WorkflowNode{}, fmt.Errorf("compensation is not declared")
	}
	comp := node.Compensation
	nodeType := strings.TrimSpace(comp.Type)
	targetID := strings.TrimSpace(comp.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(comp.Action)
	}
	if nodeType == "" {
		nodeType = "tool"
	}
	if targetID == "" && strings.TrimSpace(comp.Runtime.RuntimeID) == "" {
		return WorkflowNode{}, fmt.Errorf("compensation action/targetId/runtimeId is required")
	}
	if err := comp.ExecutionTarget.Validate(); err != nil {
		return WorkflowNode{}, err
	}
	return WorkflowNode{
		ID:              node.ID + CompensationNodeSuffix,
		Type:            nodeType,
		TargetID:        targetID,
		Runtime:         comp.Runtime,
		ExecutionTarget: comp.ExecutionTarget,
		Permissions:     append([]string(nil), comp.Permissions...),
		Scope:           comp.Scope,
		TimeoutMS:       comp.TimeoutMS,
		Retry:           comp.Retry,
		Step: WorkflowStepInput{
			Input:   append(json.RawMessage(nil), comp.Input...),
			OnError: WorkflowOnError{Mode: "fail"},
		},
	}, nil
}

func CompensationOriginalNodeID(nodeID string) string {
	return strings.TrimSuffix(strings.TrimSpace(nodeID), CompensationNodeSuffix)
}

func BuildCompensationIdempotencyKey(workflowID, executionID, nodeID string) string {
	return strings.Join([]string{
		strings.TrimSpace(workflowID),
		strings.TrimSpace(executionID),
		CompensationOriginalNodeID(nodeID),
		"compensation",
	}, "/")
}

// CompensationManager remains for legacy programmatic registrations. User
// workflow Saga compensation is definition-driven and persisted by the
// WorkflowExecutor; this manager is only a compatibility fallback.
type CompensationManager struct {
	actions map[string]CompensationAction
}

func NewCompensationManager() *CompensationManager {
	return &CompensationManager{
		actions: make(map[string]CompensationAction),
	}
}

func (cm *CompensationManager) Register(nodeID string, action CompensationAction) {
	cm.actions[nodeID] = action
}

func (cm *CompensationManager) Compensate(ctx context.Context, completedSteps []StepResult) []CompensationResult {
	var results []CompensationResult
	for i := len(completedSteps) - 1; i >= 0; i-- {
		step := completedSteps[i]
		if step.Status != "succeeded" {
			continue
		}
		action, ok := cm.actions[step.NodeID]
		if !ok {
			continue
		}
		if err := action.Handler(ctx, step.Output); err != nil {
			results = append(results, CompensationResult{
				NodeID:     step.NodeID,
				Status:     "failed",
				Error:      err.Error(),
				ExecutedAt: time.Now().UTC(),
			})
			continue
		}
		results = append(results, CompensationResult{NodeID: step.NodeID, Status: "succeeded", ExecutedAt: time.Now().UTC()})
	}
	return results
}

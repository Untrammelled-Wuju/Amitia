package workflow

import (
	"context"
	"encoding/json"
	"time"
)

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

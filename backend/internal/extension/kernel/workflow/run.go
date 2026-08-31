package workflow

import (
	"context"
	"encoding/json"
	"time"
)

type RunStatus string

const (
	RunStatusRunning      RunStatus = "running"
	RunStatusPausing      RunStatus = "pausing"
	RunStatusPaused       RunStatus = "paused"
	RunStatusResuming     RunStatus = "resuming"
	RunStatusSucceeded    RunStatus = "succeeded"
	RunStatusFailed       RunStatus = "failed"
	RunStatusCancelled    RunStatus = "cancelled"
	RunStatusCompensating RunStatus = "compensating"
	RunStatusCompensated  RunStatus = "compensated"
)

func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusSucceeded, RunStatusFailed, RunStatusCancelled, RunStatusCompensated:
		return true
	default:
		return false
	}
}

func (s RunStatus) IsActive() bool {
	switch s {
	case RunStatusRunning, RunStatusResuming, RunStatusCompensating:
		return true
	default:
		return false
	}
}

type WorkflowRun struct {
	ExecutionID         string               `json:"executionId"`
	WorkflowID          string               `json:"workflowId"`
	Status              RunStatus            `json:"status"`
	Input               json.RawMessage      `json:"input,omitempty"`
	Output              json.RawMessage      `json:"output,omitempty"`
	Error               string               `json:"error,omitempty"`
	Context             ExecutionContext     `json:"context"`
	Steps               []StepResult         `json:"steps,omitempty"`
	CompensationResults []CompensationResult `json:"compensationResults,omitempty"`
	Attempt             int                  `json:"attempt"`
	Generation          int64                `json:"generation"`
	PauseReason         string               `json:"pauseReason,omitempty"`
	PauseRequestedAt    *time.Time           `json:"pauseRequestedAt,omitempty"`
	PausedAt            *time.Time           `json:"pausedAt,omitempty"`
	StartedAt           time.Time            `json:"startedAt"`
	FinishedAt          *time.Time           `json:"finishedAt,omitempty"`
	UpdatedAt           time.Time            `json:"updatedAt"`
}

type StepRun struct {
	ExecutionID string          `json:"executionId"`
	WorkflowID  string          `json:"workflowId"`
	NodeID      string          `json:"nodeId"`
	Status      string          `json:"status"`
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	Attempt     int             `json:"attempt"`
	StartedAt   time.Time       `json:"startedAt"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
}

type RunStore interface {
	Start(ctx context.Context, run WorkflowRun) (*WorkflowRun, bool, error)
	SaveStep(ctx context.Context, step StepRun) error
	Finish(ctx context.Context, run WorkflowRun) error
	Get(ctx context.Context, executionID string) (*WorkflowRun, error)
	ListRecoverable(ctx context.Context, limit int) ([]WorkflowRun, error)
	UpdateStateCAS(ctx context.Context, run WorkflowRun, expectedStatus RunStatus) (bool, error)
}

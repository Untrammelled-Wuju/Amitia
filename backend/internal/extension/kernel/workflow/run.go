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
	ExecutionID         string
	WorkflowID          string
	Status              RunStatus
	Input               json.RawMessage
	Output              json.RawMessage
	Error               string
	Context             ExecutionContext
	Steps               []StepResult
	CompensationResults []CompensationResult
	Attempt             int
	Generation          int64
	PauseReason         string
	PauseRequestedAt    *time.Time
	PausedAt            *time.Time
	StartedAt           time.Time
	FinishedAt          *time.Time
	UpdatedAt           time.Time
}

type StepRun struct {
	ExecutionID string
	WorkflowID  string
	NodeID      string
	Status      string
	Input       json.RawMessage
	Output      json.RawMessage
	Error       string
	Attempt     int
	StartedAt   time.Time
	FinishedAt  *time.Time
}

type RunStore interface {
	Start(ctx context.Context, run WorkflowRun) (*WorkflowRun, bool, error)
	SaveStep(ctx context.Context, step StepRun) error
	Finish(ctx context.Context, run WorkflowRun) error
	Get(ctx context.Context, executionID string) (*WorkflowRun, error)
	ListRecoverable(ctx context.Context, limit int) ([]WorkflowRun, error)
	UpdateStateCAS(ctx context.Context, run WorkflowRun, expectedStatus RunStatus) (bool, error)
}

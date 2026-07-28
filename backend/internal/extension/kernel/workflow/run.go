package workflow

import (
	"context"
	"encoding/json"
	"time"
)

type RunStatus string

const (
	RunStatusRunning      RunStatus = "running"
	RunStatusSucceeded    RunStatus = "succeeded"
	RunStatusFailed       RunStatus = "failed"
	RunStatusCancelled    RunStatus = "cancelled"
	RunStatusCompensating RunStatus = "compensating"
	RunStatusCompensated  RunStatus = "compensated"
)

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
}

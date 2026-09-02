package workflow

import (
	"context"
	"encoding/json"
	"time"
)

type RunStatus string

const (
	RunStatusQueued                     RunStatus = "queued"
	RunStatusRunning                    RunStatus = "running"
	RunStatusPausing                    RunStatus = "pausing"
	RunStatusPaused                     RunStatus = "paused"
	RunStatusResuming                   RunStatus = "resuming"
	RunStatusWaitingDevice              RunStatus = "waiting_device"
	RunStatusWaitingConfirmation        RunStatus = "waiting_confirmation"
	RunStatusCancelRequested            RunStatus = "cancel_requested"
	RunStatusCancelling                 RunStatus = "cancelling"
	RunStatusCancelTimeout              RunStatus = "cancel_timeout"
	RunStatusCancelFailed               RunStatus = "cancel_failed"
	RunStatusDropped                    RunStatus = "dropped"
	RunStatusSucceeded                  RunStatus = "succeeded"
	RunStatusFailed                     RunStatus = "failed"
	RunStatusCancelled                  RunStatus = "cancelled"
	RunStatusCompensating               RunStatus = "compensating"
	RunStatusCompensated                RunStatus = "compensated"
	RunStatusCompensationFailed         RunStatus = "compensation_failed"
	RunStatusManualInterventionRequired RunStatus = "manual_intervention_required"
)

func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusSucceeded, RunStatusFailed, RunStatusCancelled, RunStatusCancelTimeout, RunStatusCancelFailed, RunStatusDropped, RunStatusCompensated, RunStatusCompensationFailed, RunStatusManualInterventionRequired:
		return true
	default:
		return false
	}
}

func (s RunStatus) IsActive() bool {
	switch s {
	case RunStatusRunning, RunStatusResuming, RunStatusCompensating, RunStatusCancelRequested, RunStatusCancelling:
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
	ExecutionID    string          `json:"executionId"`
	WorkflowID     string          `json:"workflowId"`
	NodeID         string          `json:"nodeId"`
	TraceID        string          `json:"traceId,omitempty"`
	AttemptID      string          `json:"attemptId,omitempty"`
	DeviceID       string          `json:"deviceId,omitempty"`
	RuntimeID      string          `json:"runtimeId,omitempty"`
	ToolCallID     string          `json:"toolCallId,omitempty"`
	FencingToken   int64           `json:"fencingToken,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	Status         string          `json:"status"`
	Input          json.RawMessage `json:"input,omitempty"`
	Output         json.RawMessage `json:"output,omitempty"`
	Error          string          `json:"error,omitempty"`
	Attempt        int             `json:"attempt"`
	StartedAt      time.Time       `json:"startedAt"`
	FinishedAt     *time.Time      `json:"finishedAt,omitempty"`
}

// StepAttemptRun records one physical execution attempt of a workflow node.
// Unlike StepRun, which stores the final node outcome, attempts are append-only
// so retry/backoff/timeout behaviour remains observable after the run finishes.
type StepAttemptRun struct {
	ExecutionID    string          `json:"executionId"`
	WorkflowID     string          `json:"workflowId"`
	NodeID         string          `json:"nodeId"`
	TraceID        string          `json:"traceId,omitempty"`
	AttemptID      string          `json:"attemptId,omitempty"`
	DeviceID       string          `json:"deviceId,omitempty"`
	RuntimeID      string          `json:"runtimeId,omitempty"`
	ToolCallID     string          `json:"toolCallId,omitempty"`
	FencingToken   int64           `json:"fencingToken,omitempty"`
	IdempotencyKey string          `json:"idempotencyKey,omitempty"`
	Attempt        int             `json:"attempt"`
	Generation     int64           `json:"generation"`
	Status         string          `json:"status"`
	Input          json.RawMessage `json:"input,omitempty"`
	Output         json.RawMessage `json:"output,omitempty"`
	Error          string          `json:"error,omitempty"`
	NextBackoffMS  int64           `json:"nextBackoffMs,omitempty"`
	StartedAt      time.Time       `json:"startedAt"`
	FinishedAt     time.Time       `json:"finishedAt"`
}

// StepAttemptStore is optional. WorkflowExecutor detects it on the configured
// RunStore so existing in-memory/test stores do not need to implement attempt
// persistence.
type StepAttemptStore interface {
	SaveAttempt(ctx context.Context, attempt StepAttemptRun) error
}

// StepProgressStore is an optional durable extension used by pausable built-in
// nodes. It exposes the latest node state without widening the base RunStore
// contract used by lightweight tests and embedders.
type StepProgressStore interface {
	GetStep(ctx context.Context, executionID, nodeID string) (*StepRun, error)
}

type WorkflowExecutionStats struct {
	RunCount       int64               `json:"runCount"`
	Succeeded      int64               `json:"succeeded"`
	Failed         int64               `json:"failed"`
	Cancelled      int64               `json:"cancelled"`
	Compensated    int64               `json:"compensated"`
	SuccessRate    float64             `json:"successRate"`
	AverageRunMS   float64             `json:"averageRunMs"`
	LastRunAt      *time.Time          `json:"lastRunAt,omitempty"`
	LastError      string              `json:"lastError,omitempty"`
	NodeStatistics []NodeExecutionStat `json:"nodeStatistics"`
}

type NodeExecutionStat struct {
	NodeID          string  `json:"nodeId"`
	RunCount        int64   `json:"runCount"`
	Succeeded       int64   `json:"succeeded"`
	Failed          int64   `json:"failed"`
	TimedOut        int64   `json:"timedOut"`
	AverageStepMS   float64 `json:"averageStepMs"`
	AverageAttempts float64 `json:"averageAttempts"`
}

type RunStore interface {
	Start(ctx context.Context, run WorkflowRun) (*WorkflowRun, bool, error)
	SaveStep(ctx context.Context, step StepRun) error
	Finish(ctx context.Context, run WorkflowRun) error
	Get(ctx context.Context, executionID string) (*WorkflowRun, error)
	ListRecoverable(ctx context.Context, limit int) ([]WorkflowRun, error)
	UpdateStateCAS(ctx context.Context, run WorkflowRun, expectedStatus RunStatus) (bool, error)
}

// WaitingDeviceRunStore is an optional durable extension used only by distributed
// workflow resume. It keeps the base RunStore interface compatible with memory
// stores and older tests.
type WaitingDeviceRunStore interface {
	ListWaitingDevice(ctx context.Context, userID, deviceID string, limit int) ([]WorkflowRun, error)
}

// WorkflowRunHeartbeatStore is an optional durable extension used by the
// production reaper. Heartbeats are stored separately from run state so a
// long-running healthy execution does not need to rewrite its full run row.
type WorkflowRunHeartbeatStore interface {
	HeartbeatRun(ctx context.Context, executionID string, at time.Time) error
	ListStuckRuns(ctx context.Context, heartbeatBefore time.Time, limit int) ([]WorkflowRun, error)
}

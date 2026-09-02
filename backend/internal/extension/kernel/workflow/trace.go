package workflow

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type executionTraceMetadataKey struct{}

// ExecutionTraceMetadata carries the resolved physical route for the current
// workflow attempt. It is mutable behind a small lock because the executor
// creates it before Handler execution while routing is resolved inside the
// Handler. Persisted Attempt records read the final snapshot after execution.
type ExecutionTraceMetadata struct {
	mu         sync.RWMutex
	DeviceID   string
	RuntimeID  string
	ToolCallID string
}

func WithExecutionTraceMetadata(ctx context.Context, metadata *ExecutionTraceMetadata) context.Context {
	if metadata == nil {
		return ctx
	}
	return context.WithValue(ctx, executionTraceMetadataKey{}, metadata)
}

func UpdateExecutionTraceMetadata(ctx context.Context, deviceID, runtimeID, toolCallID string) {
	metadata, _ := ctx.Value(executionTraceMetadataKey{}).(*ExecutionTraceMetadata)
	if metadata == nil {
		return
	}
	metadata.mu.Lock()
	defer metadata.mu.Unlock()
	if value := strings.TrimSpace(deviceID); value != "" {
		metadata.DeviceID = value
	}
	if value := strings.TrimSpace(runtimeID); value != "" {
		metadata.RuntimeID = value
	}
	if value := strings.TrimSpace(toolCallID); value != "" {
		metadata.ToolCallID = value
	}
}

func ExecutionTraceMetadataSnapshot(ctx context.Context) ExecutionTraceMetadata {
	metadata, _ := ctx.Value(executionTraceMetadataKey{}).(*ExecutionTraceMetadata)
	if metadata == nil {
		return ExecutionTraceMetadata{}
	}
	metadata.mu.RLock()
	defer metadata.mu.RUnlock()
	return ExecutionTraceMetadata{DeviceID: metadata.DeviceID, RuntimeID: metadata.RuntimeID, ToolCallID: metadata.ToolCallID}
}

// DistributedTrace is the durable, transport-neutral execution tree returned by
// the Workflow control plane. It intentionally uses the same identifiers that
// are persisted for Run/Step/Attempt so Cloud, device and local execution can
// be correlated without reconstructing relationships from log text.
type DistributedTrace struct {
	TraceID    string                 `json:"traceId"`
	RunID      string                 `json:"runId"`
	WorkflowID string                 `json:"workflowId"`
	DeviceID   string                 `json:"deviceId,omitempty"`
	Status     RunStatus              `json:"status"`
	Nodes      []DistributedTraceNode `json:"nodes"`
}

type DistributedTraceNode struct {
	NodeID         string                    `json:"nodeId"`
	Status         string                    `json:"status,omitempty"`
	DeviceID       string                    `json:"deviceId,omitempty"`
	RuntimeID      string                    `json:"runtimeId,omitempty"`
	ToolCallID     string                    `json:"toolCallId,omitempty"`
	IdempotencyKey string                    `json:"idempotencyKey,omitempty"`
	Attempts       []DistributedTraceAttempt `json:"attempts"`
}

type DistributedTraceAttempt struct {
	AttemptID      string `json:"attemptId"`
	Attempt        int    `json:"attempt"`
	Generation     int64  `json:"generation"`
	Status         string `json:"status"`
	DeviceID       string `json:"deviceId,omitempty"`
	RuntimeID      string `json:"runtimeId,omitempty"`
	ToolCallID     string `json:"toolCallId,omitempty"`
	FencingToken   int64  `json:"fencingToken,omitempty"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	Error          string `json:"error,omitempty"`
}

func BuildDistributedTrace(run *WorkflowRun, steps []StepRun, attempts []StepAttemptRun) DistributedTrace {
	if run == nil {
		return DistributedTrace{}
	}
	trace := DistributedTrace{
		TraceID:    run.Context.TraceID,
		RunID:      run.ExecutionID,
		WorkflowID: run.WorkflowID,
		DeviceID:   run.Context.DeviceID,
		Status:     run.Status,
		Nodes:      make([]DistributedTraceNode, 0, len(steps)),
	}

	byNode := make(map[string]int, len(steps))
	for _, step := range steps {
		trace.Nodes = append(trace.Nodes, DistributedTraceNode{
			NodeID:         step.NodeID,
			Status:         step.Status,
			DeviceID:       step.DeviceID,
			RuntimeID:      step.RuntimeID,
			ToolCallID:     step.ToolCallID,
			IdempotencyKey: step.IdempotencyKey,
			Attempts:       make([]DistributedTraceAttempt, 0),
		})
		byNode[step.NodeID] = len(trace.Nodes) - 1
	}
	for _, attempt := range attempts {
		idx, ok := byNode[attempt.NodeID]
		if !ok {
			trace.Nodes = append(trace.Nodes, DistributedTraceNode{NodeID: attempt.NodeID, Attempts: make([]DistributedTraceAttempt, 0)})
			idx = len(trace.Nodes) - 1
			byNode[attempt.NodeID] = idx
		}
		node := &trace.Nodes[idx]
		if node.DeviceID == "" {
			node.DeviceID = attempt.DeviceID
		}
		if node.RuntimeID == "" {
			node.RuntimeID = attempt.RuntimeID
		}
		if node.ToolCallID == "" {
			node.ToolCallID = attempt.ToolCallID
		}
		if node.IdempotencyKey == "" {
			node.IdempotencyKey = attempt.IdempotencyKey
		}
		node.Attempts = append(node.Attempts, DistributedTraceAttempt{
			AttemptID:      attempt.AttemptID,
			Attempt:        attempt.Attempt,
			Generation:     attempt.Generation,
			Status:         attempt.Status,
			DeviceID:       attempt.DeviceID,
			RuntimeID:      attempt.RuntimeID,
			ToolCallID:     attempt.ToolCallID,
			FencingToken:   attempt.FencingToken,
			IdempotencyKey: attempt.IdempotencyKey,
			Error:          attempt.Error,
		})
	}

	sort.SliceStable(trace.Nodes, func(i, j int) bool { return trace.Nodes[i].NodeID < trace.Nodes[j].NodeID })
	for i := range trace.Nodes {
		sort.SliceStable(trace.Nodes[i].Attempts, func(a, b int) bool {
			left, right := trace.Nodes[i].Attempts[a], trace.Nodes[i].Attempts[b]
			if left.Generation != right.Generation {
				return left.Generation < right.Generation
			}
			return left.Attempt < right.Attempt
		})
	}
	return trace
}

// WorkflowRunLogEntry is a durable execution log synthesized from persisted
// workflow state. It complements the developer console's in-memory diagnostics
// and remains available after process restart.
type WorkflowRunLogEntry struct {
	At             time.Time `json:"at"`
	Kind           string    `json:"kind"`
	TraceID        string    `json:"traceId,omitempty"`
	RunID          string    `json:"runId"`
	NodeID         string    `json:"nodeId,omitempty"`
	AttemptID      string    `json:"attemptId,omitempty"`
	DeviceID       string    `json:"deviceId,omitempty"`
	RuntimeID      string    `json:"runtimeId,omitempty"`
	ToolCallID     string    `json:"toolCallId,omitempty"`
	FencingToken   int64     `json:"fencingToken,omitempty"`
	IdempotencyKey string    `json:"idempotencyKey,omitempty"`
	Status         string    `json:"status,omitempty"`
	Message        string    `json:"message,omitempty"`
}

func BuildWorkflowRunLogs(run *WorkflowRun, steps []StepRun, attempts []StepAttemptRun, compensations []CompensationRecord) []WorkflowRunLogEntry {
	if run == nil {
		return nil
	}
	logs := make([]WorkflowRunLogEntry, 0, len(steps)+len(attempts)+len(compensations)+1)
	logs = append(logs, WorkflowRunLogEntry{
		At: run.StartedAt, Kind: "run", TraceID: run.Context.TraceID, RunID: run.ExecutionID,
		DeviceID: run.Context.DeviceID, Status: string(run.Status), Message: "workflow run started",
	})
	for _, step := range steps {
		at := step.StartedAt
		if step.FinishedAt != nil {
			at = *step.FinishedAt
		}
		logs = append(logs, WorkflowRunLogEntry{
			At: at, Kind: "step", TraceID: step.TraceID, RunID: step.ExecutionID, NodeID: step.NodeID,
			AttemptID: step.AttemptID, DeviceID: step.DeviceID, RuntimeID: step.RuntimeID, ToolCallID: step.ToolCallID,
			FencingToken: step.FencingToken, IdempotencyKey: step.IdempotencyKey, Status: step.Status, Message: step.Error,
		})
	}
	for _, attempt := range attempts {
		logs = append(logs, WorkflowRunLogEntry{
			At: attempt.FinishedAt, Kind: "attempt", TraceID: attempt.TraceID, RunID: attempt.ExecutionID, NodeID: attempt.NodeID,
			AttemptID: attempt.AttemptID, DeviceID: attempt.DeviceID, RuntimeID: attempt.RuntimeID, ToolCallID: attempt.ToolCallID,
			FencingToken: attempt.FencingToken, IdempotencyKey: attempt.IdempotencyKey, Status: attempt.Status, Message: attempt.Error,
		})
	}
	for _, comp := range compensations {
		at := comp.UpdatedAt
		if comp.CompletedAt != nil {
			at = *comp.CompletedAt
		}
		logs = append(logs, WorkflowRunLogEntry{
			At: at, Kind: "compensation", TraceID: run.Context.TraceID, RunID: comp.ExecutionID, NodeID: comp.NodeID,
			IdempotencyKey: comp.IdempotencyKey, Status: comp.Status, Message: comp.Error,
		})
	}
	sort.SliceStable(logs, func(i, j int) bool { return logs[i].At.Before(logs[j].At) })
	return logs
}

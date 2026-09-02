package workflow

import (
	"testing"
	"time"
)

func TestBuildDistributedTraceCorrelatesAttempts(t *testing.T) {
	run := &WorkflowRun{ExecutionID: "run-1", WorkflowID: "wf-1", Status: RunStatusRunning, Context: ExecutionContext{TraceID: "trace-1", DeviceID: "device-a"}}
	steps := []StepRun{{ExecutionID: "run-1", WorkflowID: "wf-1", NodeID: "pay", Status: "succeeded", TraceID: "trace-1", DeviceID: "device-a", RuntimeID: "runtime-a", ToolCallID: "run-1/pay", IdempotencyKey: "idem-1"}}
	attempts := []StepAttemptRun{{ExecutionID: "run-1", WorkflowID: "wf-1", NodeID: "pay", Attempt: 1, Generation: 2, Status: "succeeded", TraceID: "trace-1", AttemptID: "run-1/pay/g2/a1", DeviceID: "device-a", RuntimeID: "runtime-a", ToolCallID: "run-1/pay", FencingToken: 9, IdempotencyKey: "idem-1"}}

	trace := BuildDistributedTrace(run, steps, attempts)
	if trace.TraceID != "trace-1" || trace.RunID != "run-1" || len(trace.Nodes) != 1 {
		t.Fatalf("unexpected trace root: %+v", trace)
	}
	node := trace.Nodes[0]
	if node.NodeID != "pay" || len(node.Attempts) != 1 {
		t.Fatalf("unexpected node trace: %+v", node)
	}
	attempt := node.Attempts[0]
	if attempt.AttemptID != "run-1/pay/g2/a1" || attempt.FencingToken != 9 || attempt.ToolCallID != "run-1/pay" {
		t.Fatalf("attempt correlation lost: %+v", attempt)
	}
}

func TestBuildWorkflowRunLogsSurvivesFromDurableRecords(t *testing.T) {
	started := time.Date(2026, 9, 2, 1, 0, 0, 0, time.UTC)
	finished := started.Add(time.Second)
	run := &WorkflowRun{ExecutionID: "run-1", WorkflowID: "wf-1", Status: RunStatusSucceeded, StartedAt: started, Context: ExecutionContext{TraceID: "trace-1"}}
	steps := []StepRun{{ExecutionID: "run-1", WorkflowID: "wf-1", NodeID: "n1", Status: "succeeded", TraceID: "trace-1", StartedAt: started, FinishedAt: &finished}}
	attempts := []StepAttemptRun{{ExecutionID: "run-1", WorkflowID: "wf-1", NodeID: "n1", Attempt: 1, Status: "succeeded", TraceID: "trace-1", AttemptID: "a1", StartedAt: started, FinishedAt: finished}}
	logs := BuildWorkflowRunLogs(run, steps, attempts, nil)
	if len(logs) != 3 {
		t.Fatalf("expected run+step+attempt logs, got %d", len(logs))
	}
	if logs[0].RunID != "run-1" || logs[2].AttemptID != "a1" {
		t.Fatalf("durable log correlation lost: %+v", logs)
	}
}

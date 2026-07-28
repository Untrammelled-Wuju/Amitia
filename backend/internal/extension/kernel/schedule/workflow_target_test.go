package schedule

import (
	"context"
	"encoding/json"
	"testing"
)

type workflowExecutorCapture struct {
	calls    int
	workflow string
	input    []byte
	context  WorkflowScheduleContext
}

func (e *workflowExecutorCapture) ExecuteWorkflow(ctx context.Context, workflowID string, input []byte, scheduleContext WorkflowScheduleContext) (*WorkflowExecutionResult, error) {
	e.calls++
	e.workflow = workflowID
	e.input = append([]byte(nil), input...)
	e.context = scheduleContext
	return &WorkflowExecutionResult{OperationID: scheduleContext.OperationID, InvocationID: scheduleContext.InvocationID, Status: "succeeded", ResultJSON: json.RawMessage(`{"ok":true}`)}, nil
}

func TestWorkflowTargetPassesCompleteStableScheduleContext(t *testing.T) {
	executor := &workflowExecutorCapture{}
	adapter := NewWorkflowTargetAdapter(executor)
	operationID := "operation"
	def := &ScheduleContributionDefinition{ExtensionID: "ext", ModuleID: "module", ScheduleID: "schedule", Target: ScheduleTargetDefinition{Type: TargetTypeWorkflow, TargetID: "workflow", InputTemplate: json.RawMessage(`{"value":1}`)}}
	trigger := &ScheduleTriggerRecord{TriggerID: "trigger", ScheduleID: "schedule", OperationID: &operationID, ScopeSnapshotID: "scope", PermissionSnapshotID: "permission", Generation: 8, IdempotencyKey: "idempotency"}
	first, err := adapter.Execute(context.Background(), def, trigger)
	if err != nil || !first.Success {
		t.Fatalf("workflow target failed: result=%+v err=%v", first, err)
	}
	if executor.workflow != "workflow" || executor.context.ScheduleID != "schedule" || executor.context.TriggerID != "trigger" || executor.context.ExtensionID != "ext" || executor.context.ModuleID != "module" {
		t.Fatalf("missing workflow schedule ownership context: %+v", executor.context)
	}
	if executor.context.ScopeSnapshotID != "scope" || executor.context.PermissionSnapshotID != "permission" || executor.context.Generation != 8 || executor.context.IdempotencyKey != "idempotency" {
		t.Fatalf("missing workflow security context: %+v", executor.context)
	}
	firstInvocation := executor.context.InvocationID
	if firstInvocation == "" || first.InvocationID != firstInvocation {
		t.Fatalf("missing workflow invocation: result=%+v context=%+v", first, executor.context)
	}
	second, err := adapter.Execute(context.Background(), def, trigger)
	if err != nil || !second.Success || executor.context.InvocationID != firstInvocation {
		t.Fatalf("workflow invocation was not stable across retry: result=%+v context=%+v err=%v", second, executor.context, err)
	}
}

package kernel

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type integrationWorkflowHandler struct {
	calls int
}

func (h *integrationWorkflowHandler) Execute(ctx context.Context, node workflow.WorkflowNode, input json.RawMessage) (json.RawMessage, error) {
	h.calls++
	return input, nil
}

func TestScheduleWorkflowTargetExecutesRealWorkflow(t *testing.T) {
	registry := workflow.NewWorkflowRegistry()
	if err := registry.Register(workflow.WorkflowDefinition{ID: "workflow", ExtensionID: "ext", ModuleID: "module", Name: "workflow", Enabled: true, Nodes: []workflow.WorkflowNode{{ID: "step", Type: "condition"}}}); err != nil {
		t.Fatal(err)
	}
	handler := &integrationWorkflowHandler{}
	executor := workflow.NewWorkflowExecutor(registry)
	executor.RegisterHandler("condition", handler)
	adapter := schedule.NewWorkflowTargetAdapter(NewKernelWorkflowFacadeAdapter(executor))
	definition := &schedule.ScheduleContributionDefinition{ExtensionID: "ext", ModuleID: "module", ScheduleID: "schedule", Target: schedule.ScheduleTargetDefinition{Type: schedule.TargetTypeWorkflow, TargetID: "workflow", InputTemplate: json.RawMessage(`{"source":"schedule"}`)}}
	trigger := &schedule.ScheduleTriggerRecord{TriggerID: "trigger", ScheduleID: "schedule", IdempotencyKey: "schedule-trigger", Generation: 3, ScopeSnapshotID: "scope", PermissionSnapshotID: "permission"}
	result, err := adapter.Execute(context.Background(), definition, trigger)
	if err != nil || !result.Success {
		t.Fatalf("schedule workflow execution failed: result=%+v err=%v", result, err)
	}
	if handler.calls != 1 || result.InvocationID != "sched-wf-trigger" || string(result.ResultJSON) != `{"source":"schedule"}` {
		t.Fatalf("unexpected schedule workflow result: result=%+v calls=%d", result, handler.calls)
	}
}

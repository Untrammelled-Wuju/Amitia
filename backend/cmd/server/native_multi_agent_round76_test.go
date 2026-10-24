package main

import (
	"encoding/json"
	"testing"
)

func TestRound76UnsupportedBudgetsNeverDispatchWorkers(t *testing.T) {
	for _, raw := range []string{
		`{"objectives":["Inspect the core source","Verify the core source"],"token_budget":5000}`,
		`{"objectives":["Inspect the core source","Verify the core source"],"max_tool_calls":0}`,
		`{"objectives":["Inspect the core source","Verify the core source"],"max_tool_calls":257}`,
		`{"objectives":["Inspect the core source","Verify the core source"],"max_cost_usd":2}`,
	} {
		t.Run(raw, func(t *testing.T) {
			adapter, workers, scope, tracker := testMultiAgentModelRuntime(t)
			result := adapter.Execute(t.Context(), multiAgentDelegateTool, json.RawMessage(raw), scope)
			if result.Error == nil || result.Error.Code != "INVALID_INPUT" {
				t.Fatalf("unsupported budget must be rejected: %+v", result)
			}
			if len(workers.started) != 0 {
				t.Fatalf("unsupported budget launched %d workers", len(workers.started))
			}
			parent, found, err := tracker.GetByRequestID(t.Context(), scope.SpaceID, scope.RequestID)
			if err != nil || !found || parent == nil || parent.RecoveryDescriptor != nil && parent.RecoveryDescriptor.MultiAgent != nil {
				t.Fatalf("unsupported budget created a coordination: %+v, %v", parent, err)
			}
		})
	}
}

func TestRound76ToolCallBudgetPersistsAcrossDelegationRetries(t *testing.T) {
	adapter, workers, scope, tracker := testMultiAgentModelRuntime(t)
	requested := json.RawMessage(`{"objectives":["Inspect the core source","Verify the core source"],"max_tool_calls":3}`)
	issued := adapter.Execute(t.Context(), multiAgentDelegateTool, requested, scope)
	if issued.Error != nil {
		t.Fatal(issued.Error)
	}
	parent, found, err := tracker.GetByRequestID(t.Context(), scope.SpaceID, scope.RequestID)
	if err != nil || !found || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil {
		t.Fatal("parent budget checkpoint missing", err)
	}
	ref := parent.RecoveryDescriptor.MultiAgent
	if ref.MaxToolCalls == nil || *ref.MaxToolCalls != 3 {
		t.Fatalf("wrong durable tool limit: %+v", ref.MaxToolCalls)
	}
	retried := adapter.Execute(t.Context(), multiAgentDelegateTool, requested, scope)
	if retried.Error != nil || len(workers.started) != 2 {
		t.Fatalf("replay dispatched duplicate worker: %+v", retried)
	}
	drift := adapter.Execute(t.Context(), multiAgentDelegateTool, json.RawMessage(`{"objectives":["Inspect the core source","Verify the core source"],"max_tool_calls":4}`), scope)
	if drift.Error == nil || drift.Error.Code != "MULTI_AGENT_DISPATCH_CONFLICT" {
		t.Fatalf("budget drift reused parent approval: %+v", drift)
	}
}

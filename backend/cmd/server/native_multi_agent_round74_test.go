package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRound74DelegationDeadlinePersistsAndRejectsDrift(t *testing.T) {
	adapter, workers, scope, tracker := testMultiAgentModelRuntime(t)
	startedAt := time.Now().UTC()
	request := json.RawMessage(`{"objectives":["Inspect the core source","Verify the core source"],"deadline_minutes":15}`)
	first := adapter.Execute(t.Context(), multiAgentDelegateTool, request, scope)
	if first.Error != nil {
		t.Fatal(first.Error)
	}
	parent, found, err := tracker.GetByRequestID(t.Context(), scope.SpaceID, scope.RequestID)
	if err != nil || !found || parent == nil || parent.RecoveryDescriptor == nil || parent.RecoveryDescriptor.MultiAgent == nil {
		t.Fatalf("missing persistent parent deadline evidence: %v", err)
	}
	deadline := parent.RecoveryDescriptor.MultiAgent.DeadlineAt
	if deadline == nil || deadline.Before(startedAt.Add(14*time.Minute)) || deadline.After(time.Now().Add(16*time.Minute)) {
		t.Fatalf("missing or invalid durable deadline: %v", deadline)
	}
	repeated := adapter.Execute(t.Context(), multiAgentDelegateTool, request, scope)
	if repeated.Error != nil || len(workers.started) != 2 {
		t.Fatalf("retry dispatched duplicate workers: %+v", repeated)
	}
	different := adapter.Execute(t.Context(), multiAgentDelegateTool, json.RawMessage(`{"objectives":["Inspect the core source","Verify the core source"],"deadline_minutes":16}`), scope)
	if different.Error == nil || different.Error.Code != "MULTI_AGENT_DISPATCH_CONFLICT" {
		t.Fatalf("changed deadline reused approval: %+v", different)
	}
}

func TestRound74DelegationDeadlineRejectsInvalidBounds(t *testing.T) {
	for _, raw := range []string{
		`{"objectives":["Inspect the core source","Verify the core source"],"deadline_minutes":0}`,
		`{"objectives":["Inspect the core source","Verify the core source"],"deadline_minutes":-1}`,
		`{"objectives":["Inspect the core source","Verify the core source"],"deadline_minutes":241}`,
		`{"objectives":["Inspect the core source","Verify the core source"],"deadline_minutes":1.5}`,
	} {
		t.Run(raw, func(t *testing.T) {
			adapter, workers, scope, _ := testMultiAgentModelRuntime(t)
			result := adapter.Execute(t.Context(), multiAgentDelegateTool, json.RawMessage(raw), scope)
			if result.Error == nil || (result.Error.Code != "INVALID_DEADLINE" && result.Error.Code != "INVALID_INPUT") {
				t.Fatalf("invalid deadline accepted: %+v", result)
			}
			if len(workers.started) != 0 {
				t.Fatal("invalid deadline launched a worker")
			}
		})
	}
}

func TestRound74OptionalDelegationDeadlineBoundaries(t *testing.T) {
	for _, value := range []int{1, 240} {
		if err := validateMultiAgentDeadlineMinutes(&value); err != nil {
			t.Fatal(err)
		}
	}
	if err := validateMultiAgentDeadlineMinutes(nil); err != nil {
		t.Fatal(err)
	}
}

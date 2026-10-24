package main

import (
	"encoding/json"
	"testing"
)

func TestRound77StrictTotalTokenParameterValidation(t *testing.T) {
	for _, raw := range []string{
		`{"objectives":["Inspect the implementation","Verify the implementation"],"max_total_tokens":1}`,
		`{"objectives":["Inspect the implementation","Verify the implementation"],"max_total_tokens":100000000}`,
	} {
		result, err := parseMultiAgentDelegateArguments(json.RawMessage(raw))
		if err != nil || result.MaxTotalTokens == nil {
			t.Fatalf("valid total tokens rejected: %v %v", result, err)
		}
	}
	for _, raw := range []string{
		`{"objectives":["Inspect the implementation","Verify the implementation"],"max_total_tokens":0}`,
		`{"objectives":["Inspect the implementation","Verify the implementation"],"max_total_tokens":100000001}`,
		`{"objectives":["Inspect the implementation","Verify the implementation"],"max_total_tokens":null}`,
		`{"objectives":["Inspect the implementation","Verify the implementation"],"max_total_tokens":1.5}`,
	} {
		if _, err := parseMultiAgentDelegateArguments(json.RawMessage(raw)); err == nil {
			t.Fatalf("invalid total tokens accepted: %s", raw)
		}
	}
}

func TestRound77DelegationTokenCapDurableAndFingerprintProtected(t *testing.T) {
	adapter, workers, scope, tracker := testMultiAgentModelRuntime(t)
	original := json.RawMessage(`{"objectives":["Inspect the implementation","Verify the implementation"],"max_total_tokens":4096}`)
	if response := adapter.Execute(t.Context(), multiAgentDelegateTool, original, scope); response.Error != nil {
		t.Fatal(response.Error)
	}
	parent, found, err := tracker.GetByRequestID(t.Context(), scope.SpaceID, scope.RequestID)
	if err != nil || !found || parent.RecoveryDescriptor.MultiAgent.MaxTotalTokens == nil || *parent.RecoveryDescriptor.MultiAgent.MaxTotalTokens != 4096 {
		t.Fatalf("token limit absent from durable recovery ref %+v %v", parent, err)
	}
	if response := adapter.Execute(t.Context(), multiAgentDelegateTool, original, scope); response.Error != nil || len(workers.started) != 2 {
		t.Fatalf("retry created duplicate coordination %+v", response)
	}
	changed := json.RawMessage(`{"objectives":["Inspect the implementation","Verify the implementation"],"max_total_tokens":8192}`)
	if response := adapter.Execute(t.Context(), multiAgentDelegateTool, changed, scope); response.Error == nil || response.Error.Code != "MULTI_AGENT_DISPATCH_CONFLICT" {
		t.Fatalf("token budget drift bypassed authorization %+v", response)
	}
}

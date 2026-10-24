package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRound76DelegationRejectsUnsupportedSharedBudgets(t *testing.T) {
	for _, input := range []string{
		`{"objectives":["Investigate source","Verify test results"],"token_budget":5000}`,
		`{"objectives":["Investigate source","Verify test results"],"max_cost_usd":2.0}`,
		`{"objectives":["Investigate source","Verify test results"],"budget":{"tokens":5000}}`,
	} {
		t.Run(input, func(t *testing.T) {
			_, err := parseMultiAgentDelegateArguments(json.RawMessage(input))
			if err == nil || !strings.Contains(err.Error(), "unknown field") {
				t.Fatalf("unsupported hard-budget arguments must fail closed: %v", err)
			}
		})
	}
}

func TestRound76DelegationAcceptsExistingArguments(t *testing.T) {
	input := json.RawMessage(`{"objectives":["Investigate source","Verify test results"],"strategy":"sequential","dependencies":[[],[0]],"deadline_minutes":15}`)
	request, err := parseMultiAgentDelegateArguments(input)
	if err != nil || request.Strategy != "sequential" || len(request.Dependencies) != 2 || request.DeadlineMinutes == nil || *request.DeadlineMinutes != 15 {
		t.Fatalf("valid delegation must remain supported: %+v, %v", request, err)
	}
}

func TestRound76DelegationRejectsMalformedAndTrailingInput(t *testing.T) {
	for _, input := range []string{
		`{"objectives":[], "unexpected":true}`,
		`{"objectives":[]}{"objectives":[]}`,
		`{"objectives":[],"deadline_minutes":"ten"}`,
		`{"objectives":[],`,
	} {
		if _, err := parseMultiAgentDelegateArguments(json.RawMessage(input)); err == nil {
			t.Fatalf("invalid input accepted: %s", input)
		}
	}
}

func TestRound76StrictToolCallBudgetParsing(t *testing.T) {
	for _, raw := range []string{
		`{"objectives":["Inspect the workspace","Verify the workspace"],"max_tool_calls":1}`,
		`{"objectives":["Inspect the workspace","Verify the workspace"],"max_tool_calls":256}`,
	} {
		result, err := parseMultiAgentDelegateArguments(json.RawMessage(raw))
		if err != nil || result.MaxToolCalls == nil {
			t.Fatalf("valid budget %s rejected: %+v %v", raw, result, err)
		}
	}
	for _, raw := range []string{
		`{"objectives":["Inspect the workspace","Verify the workspace"],"max_tool_calls":0}`,
		`{"objectives":["Inspect the workspace","Verify the workspace"],"max_tool_calls":257}`,
		`{"objectives":["Inspect the workspace","Verify the workspace"],"max_tool_calls":null}`,
	} {
		result, err := parseMultiAgentDelegateArguments(json.RawMessage(raw))
		if err == nil {
			t.Fatalf("invalid budget %s accepted: %+v", raw, result)
		}
	}
}

package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRound79CostBudgetDelegationInputValidation(t *testing.T) {
	valid := []string{
		`{"objectives":["Analyze the codebase", "Check the result"],"max_cost_microusd":1}`,
		`{"objectives":["Analyze the codebase", "Check the result"],"max_cost_microusd":100000000000}`,
	}
	for _, input := range valid {
		parsed, err := parseMultiAgentDelegateArguments(json.RawMessage(input))
		if err != nil || parsed.MaxCostMicroUSD == nil {
			t.Fatalf("valid limit rejected %s: %v", input, err)
		}
	}
	invalid := []string{
		`{"objectives":["Analyze the codebase", "Check the result"],"max_cost_microusd":0}`,
		`{"objectives":["Analyze the codebase", "Check the result"],"max_cost_microusd":-1}`,
		`{"objectives":["Analyze the codebase", "Check the result"],"max_cost_microusd":100000000001}`,
		`{"objectives":["Analyze the codebase", "Check the result"],"max_cost_microusd":null}`,
		`{"objectives":["Analyze the codebase", "Check the result"],"max_cost_microusd":3.5}`,
		`{"objectives":["Analyze the codebase", "Check the result"],"max_cost_microusd":"5"}`,
	}
	for _, input := range invalid {
		_, err := parseMultiAgentDelegateArguments(json.RawMessage(input))
		if err == nil || !strings.Contains(err.Error(), "max_cost_microusd") {
			t.Fatalf("invalid limit admitted %s: %v", input, err)
		}
	}
}

package main

import (
	"encoding/json"
	"testing"
)

func TestRound77StandaloneTotalTokenBudgetParser(t *testing.T) {
	for _, input := range []string{
		`{"objectives":["Inspect implementation","Verify implementation"],"max_total_tokens":1}`,
		`{"objectives":["Inspect implementation","Verify implementation"],"max_total_tokens":100000000}`,
	} {
		parsed, err := parseMultiAgentDelegateArguments(json.RawMessage(input))
		if err != nil || parsed.MaxTotalTokens == nil {
			t.Fatalf("valid token budget rejected: %s %v", input, err)
		}
	}
	for _, input := range []string{
		`{"objectives":["Inspect implementation","Verify implementation"],"max_total_tokens":0}`,
		`{"objectives":["Inspect implementation","Verify implementation"],"max_total_tokens":100000001}`,
		`{"objectives":["Inspect implementation","Verify implementation"],"max_total_tokens":null}`,
		`{"objectives":["Inspect implementation","Verify implementation"],"max_total_tokens":1.5}`,
	} {
		if _, err := parseMultiAgentDelegateArguments(json.RawMessage(input)); err == nil {
			t.Fatalf("invalid token budget accepted: %s", input)
		}
	}
}

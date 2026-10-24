package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

type multiAgentDelegateArguments struct {
	Objectives      []string `json:"objectives"`
	Strategy        string   `json:"strategy"`
	Dependencies    [][]int  `json:"dependencies,omitempty"`
	DeadlineMinutes *int     `json:"deadline_minutes,omitempty"`
	MaxToolCalls    *int     `json:"max_tool_calls,omitempty"`
	MaxTotalTokens  *int64   `json:"max_total_tokens,omitempty"`
	MaxCostMicroUSD *int64   `json:"max_cost_microusd,omitempty"`
}

func parseMultiAgentDelegateArguments(input json.RawMessage) (multiAgentDelegateArguments, error) {
	var request multiAgentDelegateArguments
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return multiAgentDelegateArguments{}, fmt.Errorf("multi_agent: invalid delegation arguments: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return multiAgentDelegateArguments{}, fmt.Errorf("multi_agent: unexpected extra delegation value")
		}
		return multiAgentDelegateArguments{}, fmt.Errorf("multi_agent: invalid trailing delegation input: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return multiAgentDelegateArguments{}, fmt.Errorf("multi_agent: invalid delegation object: %w", err)
	}
	for _, fieldName := range []string{"max_tool_calls", "max_total_tokens", "max_cost_microusd"} {
		if field, exists := fields[fieldName]; exists && bytes.Equal(bytes.TrimSpace(field), []byte("null")) {
			return multiAgentDelegateArguments{}, fmt.Errorf("multi_agent: %s must not be null", fieldName)
		}
	}
	if request.MaxToolCalls != nil && (*request.MaxToolCalls < 1 || *request.MaxToolCalls > 256) {
		return multiAgentDelegateArguments{}, fmt.Errorf("multi_agent: max_tool_calls must be 1 to 256")
	}
	if request.MaxTotalTokens != nil && (*request.MaxTotalTokens < 1 || *request.MaxTotalTokens > 100000000) {
		return multiAgentDelegateArguments{}, fmt.Errorf("multi_agent: max_total_tokens must be 1 to 100000000")
	}
	if request.MaxCostMicroUSD != nil && (*request.MaxCostMicroUSD < 1 || *request.MaxCostMicroUSD > 100000000000) {
		return multiAgentDelegateArguments{}, fmt.Errorf("multi_agent: max_cost_microusd must be 1 to 100000000000")
	}
	return request, nil
}

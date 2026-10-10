package chat

import (
	"encoding/json"
	"fmt"
	"strings"
)

func restoreAgentToolMessages(items []AssistantTurnItem, aliases map[string]string) ([]map[string]interface{}, error) {
	type savedCall struct {
		id        string
		name      string
		arguments string
		result    string
		hasResult bool
	}
	pending := make(map[string]*savedCall)
	var rounds [][]*savedCall
	var current []*savedCall
	closeRound := func() error {
		if len(current) == 0 {
			return nil
		}
		for _, call := range current {
			if !call.hasResult {
				return fmt.Errorf("agent checkpoint has unresolved tool call %q; cannot safely replay its side effects", call.id)
			}
		}
		rounds = append(rounds, current)
		current = nil
		return nil
	}

	for _, item := range items {
		switch item.ItemType {
		case assistantTurnItemToolCall:
			id := strings.TrimSpace(item.CallID)
			name := strings.TrimSpace(item.ToolName)
			if id == "" || name == "" {
				return nil, fmt.Errorf("agent checkpoint has missing tool identity")
			}
			if _, duplicate := pending[id]; duplicate {
				return nil, fmt.Errorf("agent checkpoint has duplicate call ID %q", id)
			}
			if len(current) > 0 && current[len(current)-1].hasResult {
				if err := closeRound(); err != nil {
					return nil, err
				}
			}
			advertised := ""
			for alias, original := range aliases {
				if original == name {
					advertised = alias
					break
				}
			}
			if advertised == "" {
				return nil, fmt.Errorf("agent checkpoint tool %q is not advertised in the resumed model context", name)
			}
			if !json.Valid([]byte(item.ArgumentsJSON)) {
				return nil, fmt.Errorf("agent checkpoint tool %q has invalid JSON arguments", id)
			}
			call := &savedCall{id: id, name: advertised, arguments: item.ArgumentsJSON}
			pending[id] = call
			current = append(current, call)
		case assistantTurnItemToolResult:
			id := strings.TrimSpace(item.CallID)
			call, found := pending[id]
			if !found || call.hasResult {
				return nil, fmt.Errorf("agent checkpoint has orphan or duplicate result %q", id)
			}
			content := item.ResultJSON
			var plain string
			if json.Unmarshal([]byte(content), &plain) == nil {
				content = plain
			}
			call.result = content
			call.hasResult = true
		}
	}
	if err := closeRound(); err != nil {
		return nil, err
	}
	var messages []map[string]interface{}
	for _, round := range rounds {
		calls := make([]map[string]interface{}, 0, len(round))
		for _, call := range round {
			calls = append(calls, map[string]interface{}{
				"id":   call.id,
				"type": "function",
				"function": map[string]interface{}{
					"name": call.name, "arguments": call.arguments,
				},
			})
		}
		messages = append(messages, map[string]interface{}{"role": "assistant", "content": "", "tool_calls": calls})
		for _, call := range round {
			messages = append(messages, map[string]interface{}{
				"role": "tool", "tool_call_id": call.id, "content": call.result,
			})
		}
	}
	return messages, nil
}

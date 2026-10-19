package chat

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/u-ai/backend/internal/conversationstream"
)

func ownedRecoveredToolItems(runtime *conversationstream.ActiveTurnState) ([]AssistantTurnItem, error) {
	if runtime == nil {
		return nil, nil
	}
	blocks := append([]conversationstream.BlockState(nil), runtime.Blocks...)
	sort.SliceStable(blocks, func(i, j int) bool { return blocks[i].BlockSequence < blocks[j].BlockSequence })
	var items []AssistantTurnItem
	for _, block := range blocks {
		if block.Type != "tool_call" {
			continue
		}
		name, _ := block.Payload["toolName"].(string)
		arguments, _ := block.Payload["arguments"].(string)
		if block.CallID == "" || name == "" || !json.Valid([]byte(arguments)) {
			return nil, fmt.Errorf("owned checkpoint has invalid tool identity or arguments")
		}
		if block.Status != "completed" && block.Status != "failed" {
			return nil, fmt.Errorf("owned checkpoint has unresolved tool call %q", block.CallID)
		}
		result, exists := block.Payload["result"].(string)
		if !exists {
			return nil, fmt.Errorf("owned checkpoint has unresolved result %q", block.CallID)
		}
		status := assistantTurnStatusCompleted
		errorCode, _ := block.Payload["errorCode"].(string)
		if block.Status == "failed" {
			status = assistantTurnStatusFailed
		}
		items = append(items, AssistantTurnItem{ItemType: assistantTurnItemToolCall, CallID: block.CallID, ToolName: name, ArgumentsJSON: arguments}, AssistantTurnItem{ItemType: assistantTurnItemToolResult, CallID: block.CallID, ToolName: name, Status: status, ResultJSON: result, ErrorCode: errorCode})
	}
	return items, nil
}

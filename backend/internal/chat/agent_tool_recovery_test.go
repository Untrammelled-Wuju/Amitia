package chat

import (
	"strings"
	"testing"
)

func TestAgentRecoveryKeepsParallelCallGroups(t *testing.T) {
	alias := map[string]string{"read_file": "read_file"}
	items := []AssistantTurnItem{
		{ItemType: assistantTurnItemToolCall, CallID: "a", ToolName: "read_file", ArgumentsJSON: "{}"},
		{ItemType: assistantTurnItemToolCall, CallID: "b", ToolName: "read_file", ArgumentsJSON: "{}"},
		{ItemType: assistantTurnItemToolResult, CallID: "b", ResultJSON: `"second"`},
		{ItemType: assistantTurnItemToolResult, CallID: "a", ResultJSON: `"first"`},
		{ItemType: assistantTurnItemToolCall, CallID: "c", ToolName: "read_file", ArgumentsJSON: "{}"},
		{ItemType: assistantTurnItemToolResult, CallID: "c", ResultJSON: `"third"`},
	}
	msg, err := restoreAgentToolMessages(items, alias)
	if err != nil {
		t.Fatal(err)
	}
	if len(msg) != 5 || msg[0]["role"] != "assistant" || msg[3]["role"] != "assistant" {
		t.Fatalf("parallel tool groups reconstructed incorrectly: %#v", msg)
	}
	calls, _ := msg[0]["tool_calls"].([]map[string]interface{})
	if len(calls) != 2 || msg[1]["tool_call_id"] != "a" || msg[1]["content"] != "first" || msg[2]["content"] != "second" {
		t.Fatalf("tool result order or identity damaged: %#v", msg)
	}
}

func TestAgentRecoveryRejectsOrphanResult(t *testing.T) {
	_, err := restoreAgentToolMessages([]AssistantTurnItem{{ItemType: assistantTurnItemToolResult, CallID: "orphan"}}, nil)
	if err == nil || !strings.Contains(err.Error(), "orphan") {
		t.Fatalf("expected orphan result to be rejected, got %v", err)
	}
}

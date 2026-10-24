package chat

import (
	"strings"
	"testing"
)

func TestM07CompactedToolRecoveryNeverReplaysUnconfirmedEffect(t *testing.T) {
	aliases := map[string]string{"workspace_write": "workspace.write"}
	cases := []struct {
		name    string
		items   []AssistantTurnItem
		message string
	}{
		{"missing result", []AssistantTurnItem{{ItemType: assistantTurnItemToolCall, CallID: "a", ToolName: "workspace.write", ArgumentsJSON: `{}`}}, "unresolved tool call"},
		{"missing call", []AssistantTurnItem{{ItemType: assistantTurnItemToolResult, CallID: "a", ResultJSON: `{}`}}, "orphan"},
		{"unadvertised tool", []AssistantTurnItem{{ItemType: assistantTurnItemToolCall, CallID: "a", ToolName: "os.root", ArgumentsJSON: `{}`}}, "not advertised"},
		{"invalid restored arguments", []AssistantTurnItem{{ItemType: assistantTurnItemToolCall, CallID: "a", ToolName: "workspace.write", ArgumentsJSON: `{invalid}`}}, "invalid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			messages, err := restoreAgentToolMessages(tc.items, aliases)
			if err == nil || !strings.Contains(err.Error(), tc.message) || len(messages) != 0 {
				t.Fatalf("restoration should fail closed, messages=%#v err=%v", messages, err)
			}
		})
	}
}

func TestM07CompactedToolRecoveryPreservesConfirmedEvidence(t *testing.T) {
	items := []AssistantTurnItem{
		{ItemType: assistantTurnItemToolCall, CallID: "a", ToolName: "workspace.write", ArgumentsJSON: `{"path":"note"}`},
		{ItemType: assistantTurnItemToolResult, CallID: "a", ResultJSON: `{"status":"confirmed"}`},
	}
	messages, err := restoreAgentToolMessages(items, map[string]string{"workspace_write": "workspace.write"})
	if err != nil || len(messages) != 2 || messages[0]["role"] != "assistant" || messages[1]["role"] != "tool" {
		t.Fatalf("confirmed tool evidence not reconstructed: %#v, %v", messages, err)
	}
	if messages[1]["tool_call_id"] != "a" || messages[1]["content"] != `{"status":"confirmed"}` {
		t.Fatalf("restored result lost confirmed identity: %#v", messages[1])
	}
}

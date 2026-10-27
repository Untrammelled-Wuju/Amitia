package main

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedRuntimeDocumentsRestoreCheckpointBlocks(t *testing.T) {
	scope := coordination.ExecutionScope{SpaceID: "space", ResourceOwnerID: "owner", RoleID: "role", CoreID: "core", TurnID: "turn"}
	runtime := conversationstream.ActiveTurnState{TurnID: "turn", Blocks: []conversationstream.BlockState{{BlockID: "text", BlockSequence: 1, Type: "text", Status: "running", Content: "saved text"}, {BlockID: "tool", BlockSequence: 2, Type: "tool_call", Status: "completed", CallID: "call", Payload: map[string]any{"toolName": "read", "result": "saved result"}}}}
	raw, err := json.Marshal(map[string]any{"conversationId": "conversation", "status": "completed", "scope": scope, "runtime": runtime})
	if err != nil {
		t.Fatal(err)
	}
	view := business.QueryResult{Scope: scope, ConversationID: "conversation", Snapshot: coordination.DataSnapshot{Resources: []coordination.Resource{{Kind: "checkpoint", ID: "turn/request", UpdatedAt: "2026-10-18T01:00:00Z", Body: raw}}}}
	turns, _, _ := ownedRuntimeDocuments(view, business.RuntimeKey(scope, "conversation"))
	if len(turns) != 1 {
		t.Fatalf("turn count = %d", len(turns))
	}
	items := turns[0]["items"].([]map[string]any)
	if len(items) != 2 || items[0]["content"] != "saved text" || items[0]["status"] != "completed" || items[1]["resultJson"] != "saved result" {
		t.Fatalf("restored items = %#v", items)
	}
}

func TestOwnedRuntimeDocumentsRestoreStructuredFailure(t *testing.T) {
	scope := coordination.ExecutionScope{SpaceID: "space", ResourceOwnerID: "owner", RoleID: "role", CoreID: "core", TurnID: "turn"}
	runtime := conversationstream.ActiveTurnState{TurnID: "turn", Status: "failed"}
	raw, err := json.Marshal(map[string]any{
		"conversationId": "conversation",
		"status":         "failed",
		"scope":          scope,
		"runtime":        runtime,
		"error": map[string]any{
			"errorCode":   "browser_runtime_unavailable",
			"userMessage": "浏览器运行时未启用或不可用。",
			"retryable":   true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	view := business.QueryResult{Scope: scope, ConversationID: "conversation", Snapshot: coordination.DataSnapshot{Resources: []coordination.Resource{{Kind: "checkpoint", ID: "turn/request", UpdatedAt: "2026-10-18T01:00:00Z", Body: raw}}}}
	turns, _, _ := ownedRuntimeDocuments(view, business.RuntimeKey(scope, "conversation"))
	items := turns[0]["items"].([]map[string]any)
	if len(items) != 1 || items[0]["type"] != "error" || items[0]["errorCode"] != "browser_runtime_unavailable" {
		t.Fatalf("restored failure = %#v", items)
	}
}

func TestOwnedRuntimeTurnSequenceDoesNotDependOnPage(t *testing.T) {
	resource := func(id, stamp string) coordination.Resource {
		return coordination.Resource{Kind: "checkpoint", ID: "turn/" + id, UpdatedAt: stamp, Body: json.RawMessage(`{"conversationId":"conversation","status":"completed","scope":{"turnId":"` + id + `"}}`)}
	}
	first := resource("first", "2026-10-18T01:00:00Z")
	second := resource("second", "2026-10-18T02:00:00Z")
	view := business.QueryResult{ConversationID: "conversation", Snapshot: coordination.DataSnapshot{Resources: []coordination.Resource{first, second}}}
	combined, _, _ := ownedRuntimeDocuments(view, "key")
	view.Snapshot.Resources = []coordination.Resource{second}
	page, _, _ := ownedRuntimeDocuments(view, "key")
	if combined[1]["sequence"] != page[0]["sequence"] {
		t.Fatal("same turn changed sequence between pages")
	}
}

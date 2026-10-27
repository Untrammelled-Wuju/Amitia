package business

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRuntimeToolProgressKeepsCallsSeparate(t *testing.T) {
	inference := Inference{ConversationID: t.Name(), Scope: coordination.ExecutionScope{TurnID: "turn", RequestID: "request"}}
	for _, call := range []string{"one", "two"} {
		progress, _ := json.Marshal(map[string]any{"toolCallId": call, "toolName": "read"})
		if err := publishRuntime(context.Background(), inference, Event{Type: "tool-progress", Progress: progress}); err != nil {
			t.Fatal(err)
		}
	}
	runtime := RuntimeManager().RuntimeSnapshot(RuntimeKey(inference.Scope, inference.ConversationID))
	if runtime.ActiveTurn == nil || len(runtime.ActiveTurn.Blocks) != 2 || runtime.ActiveTurn.Blocks[0].CallID == runtime.ActiveTurn.Blocks[1].CallID {
		t.Fatalf("tool progress merged calls: %#v", runtime.ActiveTurn)
	}
}

func TestRuntimeToolLifecycleRestoresArgumentsAndResult(t *testing.T) {
	inference := Inference{ConversationID: t.Name(), Scope: coordination.ExecutionScope{TurnID: "turn", RequestID: "request"}}
	for _, event := range []Event{{Type: "tool-started", Progress: json.RawMessage(`{"toolCallId":"call","toolName":"read","arguments":"{}"}`)}, {Type: "tool-completed", Progress: json.RawMessage(`{"toolCallId":"call","result":"result"}`)}} {
		if err := publishRuntime(context.Background(), inference, event); err != nil {
			t.Fatal(err)
		}
	}
	runtime := RuntimeManager().RuntimeSnapshot(RuntimeKey(inference.Scope, inference.ConversationID))
	if runtime.ActiveTurn == nil || len(runtime.ActiveTurn.Blocks) != 1 {
		t.Fatalf("missing tool block: %#v", runtime.ActiveTurn)
	}
	block := runtime.ActiveTurn.Blocks[0]
	if block.Status != "completed" || block.Payload["arguments"] != "{}" || block.Payload["result"] != "result" {
		t.Fatalf("tool lifecycle state lost: %#v", block)
	}
}

func TestRuntimeToolFailureStatusDoesNotPublishAsCompleted(t *testing.T) {
	inference := Inference{ConversationID: t.Name(), Scope: coordination.ExecutionScope{TurnID: "turn", RequestID: "request"}}
	for _, event := range []Event{
		{Type: "tool-started", Progress: json.RawMessage(`{"toolCallId":"call","toolName":"browser.agent.run","arguments":"{}"}`)},
		{Type: "tool-completed", Progress: json.RawMessage(`{"toolCallId":"call","toolName":"browser.agent.run","status":"FAILED","errorCode":"browser_agent_failed","result":"typed browser tool is unavailable: browser_session_create"}`)},
	} {
		if err := publishRuntime(context.Background(), inference, event); err != nil {
			t.Fatal(err)
		}
	}
	runtime := RuntimeManager().RuntimeSnapshot(RuntimeKey(inference.Scope, inference.ConversationID))
	if runtime.ActiveTurn == nil || len(runtime.ActiveTurn.Blocks) != 1 {
		t.Fatalf("missing tool block: %#v", runtime.ActiveTurn)
	}
	block := runtime.ActiveTurn.Blocks[0]
	if block.Status != "failed" || block.Payload["errorCode"] != "browser_agent_failed" {
		t.Fatalf("tool failure was projected as success: %#v", block)
	}
}

func TestRuntimeKeySeparatesOwnerRoleAndCore(t *testing.T) {
	base := coordination.ExecutionScope{SpaceID: "space", ResourceOwnerID: "owner", CoreID: "core", RoleID: "role"}
	key := RuntimeKey(base, "conversation")
	for _, field := range []string{"owner", "role", "core", "space"} {
		other := base
		switch field {
		case "owner":
			other.ResourceOwnerID = "other"
		case "role":
			other.RoleID = "other"
		case "core":
			other.CoreID = "other"
		case "space":
			other.SpaceID = "other"
		}
		if key == RuntimeKey(other, "conversation") {
			t.Fatalf("runtime key did not separate %s", field)
		}
	}
	if key == RuntimeKey(base, "other") {
		t.Fatal("runtime key did not separate conversation")
	}
}

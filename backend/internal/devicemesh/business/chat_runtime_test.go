package business

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/conversationstream"
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

func TestRuntimeSegmentsThinkingAndTextPerModelRound(t *testing.T) {
	inference := Inference{ConversationID: t.Name(), Scope: coordination.ExecutionScope{TurnID: "turn", RequestID: "request"}}
	for _, event := range []Event{
		{Type: "delta", Text: "first-thought", Reasoning: true, Round: 1},
		{Type: "reasoning-completed", Round: 1},
		{Type: "tool-started", Round: 1, Progress: json.RawMessage(`{"toolCallId":"call-1","toolName":"read","arguments":"{}"}`)},
		{Type: "delta", Text: "first-message", Round: 1},
		{Type: "output-completed", Round: 1},
		{Type: "delta", Text: "second-thought", Reasoning: true, Round: 2},
		{Type: "reasoning-completed", Round: 2},
		{Type: "delta", Text: "second-message", Round: 2},
		{Type: "output-completed", Round: 2},
	} {
		if err := publishRuntime(context.Background(), inference, event); err != nil {
			t.Fatal(err)
		}
	}
	runtime := RuntimeManager().RuntimeSnapshot(RuntimeKey(inference.Scope, inference.ConversationID))
	if runtime.ActiveTurn == nil {
		t.Fatal("missing active turn")
	}
	blocks := map[string]conversationstream.BlockState{}
	for _, block := range runtime.ActiveTurn.Blocks {
		blocks[block.BlockID] = block
	}
	expected := map[string]string{
		"turn/reasoning/1": "first-thought",
		"turn/text/1":      "first-message",
		"turn/reasoning/2": "second-thought",
		"turn/text/2":      "second-message",
	}
	for id, content := range expected {
		block, ok := blocks[id]
		if !ok || block.Content != content || block.Status != "completed" {
			t.Fatalf("segmented block %s = %#v", id, block)
		}
	}
	if _, exists := blocks["turn/reasoning"]; exists {
		t.Fatal("legacy shared reasoning block was created")
	}
	if _, exists := blocks["turn/text"]; exists {
		t.Fatal("legacy shared text block was created")
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

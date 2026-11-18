package main

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestOwnedChatForkCopiesMessagesIntoNewConversation(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	ctx := t.Context()
	scope := coordination.ExecutionScope{SpaceID: "space-1", CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RequestID: "seed-conversation"}
	conversation := map[string]any{"id": "conv-1", "conversationId": "conv-1", "characterId": "one", "title": "原会话", "updatedAt": "2026-01-01T00:00:00Z"}
	encodedConversation, err := json.Marshal(conversation)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.store.Apply(ctx, coordination.Commit{Scope: scope, Mutations: []coordination.Mutation{{Kind: "conversation", ID: "conv-1", RoleID: "one", Body: encodedConversation}}}); err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	for index, role := range []string{"user", "assistant"} {
		message := map[string]any{"id": fmt.Sprintf("m-%d", index), "conversationId": "conv-1", "characterId": "one", "role": role, "content": fmt.Sprintf("content-%d", index)}
		encodedMessage, err := json.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		messageScope := scope
		messageScope.RequestID = fmt.Sprintf("seed-message-%d", index)
		if _, err := p.store.Apply(ctx, coordination.Commit{Scope: messageScope, Mutations: []coordination.Mutation{{Kind: "message", ID: fmt.Sprintf("m-%d", index), RoleID: "one", Body: encodedMessage}}}); err != nil {
			t.Fatalf("seed message %d: %v", index, err)
		}
	}
	record := ownedChatManagementRecord{ID: "conv-1", Kind: "conversation", Body: conversation, Revision: 1}
	request := coordination.ChatManagementRequest{Operation: "conversation/fork", ConversationID: "conv-1", RequestID: "fork-request-1", Title: "派生会话"}
	result, err := p.managedConversationAction(ctx, scope, "one", request, []ownedChatManagementRecord{record}, []ownedChatManagementRecord{record})
	if err != nil {
		t.Fatalf("fork conversation: %v", err)
	}
	if result["forkedFrom"] != "conv-1" {
		t.Fatalf("fork origin mismatch: %+v", result)
	}
	if count, ok := result["messageCount"].(int); !ok || count != 2 {
		t.Fatalf("forked message count = %v, want 2", result["messageCount"])
	}
	forkID, _ := result["conversationId"].(string)
	if forkID == "" || forkID == "conv-1" {
		t.Fatalf("forked conversation id = %q", forkID)
	}
	rows, err := p.store.List(ctx, "message", "one", false)
	if err != nil {
		t.Fatal(err)
	}
	copied := 0
	for _, row := range rows {
		body := map[string]any{}
		if err := json.Unmarshal(row.Body, &body); err != nil {
			t.Fatal(err)
		}
		if body["conversationId"] != forkID {
			continue
		}
		copied++
		if body["forkOrigin"] == nil {
			t.Fatalf("forked message lost forkOrigin: %+v", body)
		}
	}
	if copied != 2 {
		t.Fatalf("forked messages = %d, want 2", copied)
	}
}

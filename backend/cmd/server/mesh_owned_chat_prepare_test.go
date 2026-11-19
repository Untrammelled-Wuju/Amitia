package main

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func prepareTestScope(p *meshLocalDataPort, requestID string) coordination.ExecutionScope {
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: requestID}
	scope.SpaceID, scope.InitiatorDeviceID = "core", p.ownerID
	return scope
}

func TestOwnedChatPrepareKeepsDraftOutOfCatalogUntilFirstMessage(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	ctx := t.Context()
	scope := prepareTestScope(p, "prepare-scope")
	request := coordination.ChatManagementRequest{Operation: "conversation/create", Draft: true, ConversationID: "dsh-draft-1", ConversationOwnerID: p.ownerID, RequestID: "prepare-1", Title: "草稿会话"}
	prepared, err := p.ManageChat(ctx, scope, request)
	if err != nil {
		t.Fatalf("prepare conversation: %v", err)
	}
	if prepared["conversationId"] != "dsh-draft-1" {
		t.Fatalf("prepared id = %v", prepared["conversationId"])
	}
	if prepared["draft"] != true {
		t.Fatalf("prepared conversation is not a draft: %+v", prepared)
	}
	duplicate, err := p.ManageChat(ctx, scope, request)
	if err != nil || duplicate["conversationId"] != "dsh-draft-1" {
		t.Fatalf("prepare not idempotent: %+v %v", duplicate, err)
	}
	listing := coordination.ChatManagementRequest{Operation: "conversations", ConversationOwnerID: p.ownerID, Page: 1, PageSize: 50}
	page, err := p.ManageChat(ctx, scope, listing)
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total, _ := page["total"].(int); total != 0 {
		t.Fatalf("draft conversation leaked into catalog: %+v", page["items"])
	}
	message := map[string]any{"id": "draft-message-1", "conversationId": "dsh-draft-1", "characterId": "one", "role": "user", "content": "first"}
	encoded, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	messageScope := prepareTestScope(p, "draft-message-scope")
	if _, err := p.store.Apply(ctx, coordination.Commit{Scope: messageScope, Mutations: []coordination.Mutation{{Kind: "message", ID: "draft-message-1", RoleID: "one", Body: encoded}}}); err != nil {
		t.Fatalf("append draft message: %v", err)
	}
	page, err = p.ManageChat(ctx, scope, listing)
	if err != nil {
		t.Fatalf("list conversations after first message: %v", err)
	}
	if total, _ := page["total"].(int); total != 1 {
		t.Fatalf("draft conversation still hidden after first message: %+v", page["items"])
	}
}

func TestOwnedChatCreatePromotesPreparedDraft(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	ctx := t.Context()
	scope := prepareTestScope(p, "promote-scope")
	request := coordination.ChatManagementRequest{Operation: "conversation/create", Draft: true, ConversationID: "dsh-draft-2", ConversationOwnerID: p.ownerID, RequestID: "prepare-2", Title: "草稿会话"}
	if _, err := p.ManageChat(ctx, scope, request); err != nil {
		t.Fatalf("prepare conversation: %v", err)
	}
	request.Draft = false
	promoted, err := p.ManageChat(ctx, scope, request)
	if err != nil {
		t.Fatalf("promote draft: %v", err)
	}
	if promoted["draft"] == true {
		t.Fatalf("promoted conversation still marked draft: %+v", promoted)
	}
	if publishedAt, _ := promoted["publishedAt"].(string); publishedAt == "" {
		t.Fatalf("promoted conversation missing publishedAt: %+v", promoted)
	}
	listing := coordination.ChatManagementRequest{Operation: "conversations", ConversationOwnerID: p.ownerID, Page: 1, PageSize: 50}
	page, err := p.ManageChat(ctx, scope, listing)
	if err != nil {
		t.Fatalf("list conversations: %v", err)
	}
	if total, _ := page["total"].(int); total != 1 {
		t.Fatalf("promoted conversation not in catalog: %+v", page["items"])
	}
	conflicting := request
	conflicting.Title = "另一个标题"
	if _, err := p.ManageChat(ctx, scope, conflicting); err == nil {
		t.Fatal("expected request conflict for reused requestId with different payload")
	}
}

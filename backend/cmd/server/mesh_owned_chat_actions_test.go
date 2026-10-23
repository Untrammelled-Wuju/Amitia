package main

import (
	"encoding/json"
	"testing"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/feedback"
)

func TestOwnedChatActionsCreateRenameAndDeleteWithoutLegacyWrites(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "admin-create"}
	scope.SpaceID, scope.InitiatorDeviceID = "core", p.ownerID
	request := coordination.ChatManagementRequest{Operation: "conversation/create", RequestID: "create-a", Title: "Owned conversation", Channel: "web"}
	request.ConversationOwnerID = p.ownerID
	created, err := p.ManageChat(t.Context(), scope, request)
	if err != nil {
		t.Fatal(err)
	}
	id, ok := created["id"].(string)
	if !ok || id == "" {
		t.Fatalf("missing conversation: %v", created)
	}
	duplicate, err := p.ManageChat(t.Context(), scope, request)
	if err != nil || duplicate["id"] != id {
		t.Fatalf("creation not idempotent: %v %v", duplicate, err)
	}
	var legacyCount int64
	if err := p.services.DB.Table("conversations").Where("id=?", id).Count(&legacyCount).Error; err != nil || legacyCount != 0 {
		t.Fatalf("new chat wrote legacy storage: %d %v", legacyCount, err)
	}
	request.Operation, request.ConversationID, request.RequestID, request.Title = "conversation/rename", id, "rename-a", "Renamed"
	updated, err := p.ManageChat(t.Context(), scope, request)
	if err != nil || updated["title"] != "Renamed" {
		t.Fatalf("rename failed: %v %v", updated, err)
	}
	request.Operation = "conversation/delete"
	if _, err := p.ManageChat(t.Context(), scope, request); err != nil {
		t.Fatal(err)
	}
	resource, err := p.store.Get(t.Context(), "conversation", id)
	if err != nil || resource == nil || !resource.Deleted {
		t.Fatalf("delete not applied: %+v %v", resource, err)
	}
}

func TestOwnedChatFeedbackKeepsOwnerAndRolePartitions(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "feedback"}
	seedOwnedChatManagement(t, p, "conversation", "visible", "one", `{"id":"visible","title":"Visible"}`)
	seedOwnedChatManagement(t, p, "message", "reply", "one", `{"conversationId":"visible","role":"assistant","content":"answer"}`)
	seedOwnedChatManagement(t, p, "conversation", "private", "two", `{"id":"private","title":"Private"}`)
	seedOwnedChatManagement(t, p, "message", "private-reply", "two", `{"conversationId":"private","role":"assistant","content":"private"}`)
	request := coordination.ChatManagementRequest{Operation: "feedback/create", MessageID: "reply", RequestID: "feedback-create", FeedbackType: "good", Reason: "useful"}
	request.ConversationOwnerID = p.ownerID
	created, err := p.ManageChat(t.Context(), scope, request)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(created["feedback"])
	var item feedback.MessageFeedback
	if err := json.Unmarshal(raw, &item); err != nil || item.ID < 1 {
		t.Fatalf("feedback missing: %v %v", created, err)
	}
	stats, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "feedback-stats", ConversationOwnerID: p.ownerID})
	if err != nil || stats["total"] != 1 {
		t.Fatalf("stats=%v err=%v", stats, err)
	}
	request.MessageID = "private-reply"
	if _, err := p.ManageChat(t.Context(), scope, request); err == nil {
		t.Fatal("foreign role feedback accepted")
	}
	other := scope
	other.RoleID, other.RoleRevision = "two", 1
	if _, err := p.ManageChat(t.Context(), other, coordination.ChatManagementRequest{Operation: "feedback/delete", FeedbackID: item.ID, ConversationOwnerID: p.ownerID}); err == nil {
		t.Fatal("foreign role deleted feedback")
	}
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "feedback/delete", FeedbackID: item.ID, ConversationOwnerID: p.ownerID}); err != nil {
		t.Fatal(err)
	}
}

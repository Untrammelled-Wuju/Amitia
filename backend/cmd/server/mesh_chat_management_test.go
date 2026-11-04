package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func seedOwnedChatManagement(t *testing.T, p *meshLocalDataPort, kind, id, role string, body string) {
	t.Helper()
	_, err := p.services.KernelContainer.DeviceRegistry.Database().Exec(`INSERT INTO kernel_device_owned_resources(owner_id,kind,resource_id,role_id,revision,deleted,body,updated_at) VALUES(?,?,?,?,1,0,?,?)`, p.ownerID, kind, id, role, []byte(body), time.Now().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
}

func TestOwnedChatManagementIncludesActualHistoryAndRejectsForeignOwner(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "management"}
	seedOwnedChatManagement(t, p, "conversation", "owned", "one", `{"title":"Owned","createdAt":"2026-01-01T00:00:00Z"}`)
	seedOwnedChatManagement(t, p, "message", "owned-message", "one", `{"conversationId":"owned","role":"user","content":"visible owned"}`)
	seedOwnedChatManagement(t, p, "conversation", "foreign-role", "two", `{"title":"foreign"}`)
	result, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "stats", ConversationOwnerID: p.ownerID})
	if err != nil {
		t.Fatal(err)
	}
	if result["totalConversations"] != 1 || result["totalMessages"] != 1 {
		t.Fatalf("counts=%v", result)
	}
	result, err = p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "export", ConversationOwnerID: p.ownerID, ConversationIDs: []string{"owned"}, Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	var archive struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal([]byte(result["content"].(string)), &archive); err != nil || len(archive.Messages) != 1 {
		t.Fatalf("export=%v err=%v", archive, err)
	}
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "stats", ConversationOwnerID: "other"}); err == nil {
		t.Fatal("foreign owner accepted")
	}
}

func TestOwnedChatCleanupPreservesMemoriesAndDeletesCurrentHistory(t *testing.T) {
	for _, include := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep-memory", true: "delete-memory"}[include], func(t *testing.T) {
			p := setupMeshLocalDataPort(t)
			scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "cleanup"}
			seedOwnedChatManagement(t, p, "conversation", "owned", "one", `{"title":"Owned"}`)
			seedOwnedChatManagement(t, p, "message", "owned-message", "one", `{"conversationId":"owned","role":"user","content":"hello"}`)
			seedOwnedChatManagement(t, p, "memory", "memory", "one", `{"conversationId":"owned","content":"remember me"}`)
			seedOwnedChatManagement(t, p, "memory", "owned-memory", "one", `{"conversationId":"owned","content":"owned memory","executionScope":{"roleId":"one"}}`)
			preview, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/preview", ConversationOwnerID: p.ownerID, IncludeMemories: include})
			if err != nil {
				t.Fatal(err)
			}
			request := coordination.ChatManagementRequest{Operation: "cleanup/confirm", ConversationOwnerID: p.ownerID, PreviewID: preview["previewId"].(string), ConfirmText: "确认清理"}
			if _, err := p.ManageChat(t.Context(), scope, request); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"memory", "owned-memory"} {
				resource, err := p.store.Get(t.Context(), "memory", id)
				if err != nil || resource == nil || resource.Deleted != include {
					t.Fatalf("memory=%+v err=%v include=%v", resource, err, include)
				}
			}
			messageRow, err := p.store.Get(t.Context(), "message", "owned-message")
			if err != nil || messageRow == nil || !messageRow.Deleted {
				t.Fatalf("message remains: %+v %v", messageRow, err)
			}
			if _, err := p.ManageChat(t.Context(), scope, request); err == nil {
				t.Fatal("cleanup token reused")
			}
		})
	}
}

func TestOwnedChatCleanupRejectsChangedPreviewAndRole(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "cleanup"}
	seedOwnedChatManagement(t, p, "conversation", "owned", "one", `{"title":"Owned"}`)
	preview, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/preview", ConversationOwnerID: p.ownerID})
	if err != nil {
		t.Fatal(err)
	}
	request := coordination.ChatManagementRequest{Operation: "cleanup/confirm", ConversationOwnerID: p.ownerID, PreviewID: preview["previewId"].(string), ConfirmText: "确认清理"}
	other := scope
	other.RoleID = "two"
	other.RoleRevision = 1
	if _, err := p.ManageChat(t.Context(), other, request); err == nil {
		t.Fatal("foreign role accepted cleanup token")
	}
	seedOwnedChatManagement(t, p, "message", "late", "one", `{"conversationId":"owned","role":"user","content":"late"}`)
	if _, err := p.ManageChat(t.Context(), scope, request); err == nil {
		t.Fatal("changed conversation deleted")
	}
}

func TestUnsafeOwnedFeedbackCreatesScopedIncidentWithoutLegacyEvent(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "feedback"}
	seedOwnedChatManagement(t, p, "conversation", "safety-chat", "one", `{"id":"safety-chat","title":"Safety"}`)
	seedOwnedChatManagement(t, p, "message", "assistant-message", "one", `{"id":"assistant-message","conversationId":"safety-chat","role":"assistant","content":"unsafe reply"}`)
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "feedback/create", ConversationOwnerID: p.ownerID, MessageID: "assistant-message", FeedbackType: "unsafe", Reason: "harmful", RequestID: "unsafe-feedback"}); err != nil {
		t.Fatal(err)
	}
	var incident struct {
		SpaceID        string `gorm:"column:space_id"`
		OwnerID        string `gorm:"column:owner_id"`
		CoreID         string `gorm:"column:core_id"`
		RoleID         string `gorm:"column:role_id"`
		ConversationID string `gorm:"column:conversation_id"`
		MessageID      string `gorm:"column:message_id"`
		FeedbackID     int64  `gorm:"column:feedback_id"`
		RequestID      string `gorm:"column:request_id"`
		Status         string `gorm:"column:status"`
	}
	if err := p.services.DB.Table("safety_incidents").Take(&incident).Error; err != nil {
		t.Fatal(err)
	}
	if incident.SpaceID != p.ownerID || incident.OwnerID != p.ownerID || incident.CoreID != "core" || incident.RoleID != "one" || incident.ConversationID != "safety-chat" || incident.MessageID != "assistant-message" || incident.FeedbackID < 1 || incident.RequestID != "unsafe-feedback" || incident.Status != "open" {
		t.Fatalf("incident lost authority: %+v", incident)
	}
	if p.services.DB.Migrator().HasTable("safety_events") {
		t.Fatal("legacy event table still exists")
	}
}

func TestOwnedChatFeedbackRejectsUnavailableReplies(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{CoreID: "core", RoleID: "one"}
	records := []ownedChatManagementRecord{
		{ID: "user", Kind: "message", Body: map[string]any{"role": "user"}},
	}
	for _, operation := range []string{"feedback/list", "feedback/create"} {
		for _, id := range []string{"missing", "user"} {
			_, err := p.managedChatFeedback(t.Context(), scope, coordination.ChatManagementRequest{Operation: operation, MessageID: id}, records)
			if err == nil || err.Error() != "当前回复尚未保存、已删除或不属于当前Owner与角色，暂不可反馈" {
				t.Fatalf("operation=%s message=%s error=%v", operation, id, err)
			}
		}
	}
}

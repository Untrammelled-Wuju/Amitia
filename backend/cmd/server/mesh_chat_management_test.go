package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/chat"
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
	for _, conversation := range []chat.Conversation{{ID: "legacy", SpaceID: p.legacySpaceID, Title: "Legacy"}, {ID: "foreign-space", SpaceID: "another-space"}} {
		if err := p.services.DB.Create(&conversation).Error; err != nil {
			t.Fatal(err)
		}
		message := chat.Message{ID: conversation.ID + "-message", ConversationID: conversation.ID, CharacterID: "one", Role: "user", Content: "visible history"}
		if err := p.services.DB.Create(&message).Error; err != nil {
			t.Fatal(err)
		}
	}
	result, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "stats", ConversationOwnerID: p.ownerID})
	if err != nil {
		t.Fatal(err)
	}
	if result["totalConversations"] != 2 || result["totalMessages"] != 2 {
		t.Fatalf("counts=%v", result)
	}
	result, err = p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "export", ConversationOwnerID: p.ownerID, ConversationIDs: []string{"legacy", "owned"}, Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	var archive struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal([]byte(result["content"].(string)), &archive); err != nil || len(archive.Messages) != 2 {
		t.Fatalf("export=%v err=%v", archive, err)
	}
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "stats", ConversationOwnerID: "other"}); err == nil {
		t.Fatal("foreign owner accepted")
	}
}

func TestOwnedChatCleanupPreservesMemoriesAndDeletesLegacyHistory(t *testing.T) {
	for _, include := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep-memory", true: "delete-memory"}[include], func(t *testing.T) {
			p := setupMeshLocalDataPort(t)
			scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "cleanup"}
			seedOwnedChatManagement(t, p, "conversation", "owned", "one", `{"title":"Owned"}`)
			seedOwnedChatManagement(t, p, "message", "owned-message", "one", `{"conversationId":"owned","role":"user","content":"hello"}`)
			seedOwnedChatManagement(t, p, "memory", "memory", "one", `{"conversationId":"owned","content":"remember me"}`)
			conversation := chat.Conversation{ID: "legacy", SpaceID: p.legacySpaceID, Title: "Legacy"}
			if err := p.services.DB.Create(&conversation).Error; err != nil {
				t.Fatal(err)
			}
			message := chat.Message{ID: "legacy-message", ConversationID: "legacy", CharacterID: "one", Role: "user", Content: "legacy"}
			if err := p.services.DB.Create(&message).Error; err != nil {
				t.Fatal(err)
			}
			seedOwnedChatManagement(t, p, "memory", "legacy-memory", coordination.GlobalMemoryRole, `{"conversationId":"legacy","content":"legacy memory","executionScope":{"roleId":"one"}}`)
			preview, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/preview", ConversationOwnerID: p.ownerID, IncludeMemories: include})
			if err != nil {
				t.Fatal(err)
			}
			request := coordination.ChatManagementRequest{Operation: "cleanup/confirm", ConversationOwnerID: p.ownerID, PreviewID: preview["previewId"].(string), ConfirmText: "确认清理"}
			if _, err := p.ManageChat(t.Context(), scope, request); err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"memory", "legacy-memory"} {
				resource, err := p.store.Get(t.Context(), "memory", id)
				if err != nil || resource == nil || resource.Deleted != include {
					t.Fatalf("memory=%+v err=%v include=%v", resource, err, include)
				}
			}
			var count int64
			if err := p.services.DB.Model(&chat.Message{}).Where("id='legacy-message' AND deleted_at IS NULL").Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("legacy message remains: %d %v", count, err)
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

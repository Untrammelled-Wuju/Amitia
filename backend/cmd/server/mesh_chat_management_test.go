package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/system/dataportability"
)

type chatCleanupBackupTestContributor struct {
	db      *sql.DB
	ownerID string
}

func (chatCleanupBackupTestContributor) ID() string             { return "chat-cleanup-qa" }
func (chatCleanupBackupTestContributor) Name() string           { return "Chat cleanup QA" }
func (chatCleanupBackupTestContributor) Dependencies() []string { return nil }
func installChatCleanupBackup(t *testing.T, p *meshLocalDataPort) {
	t.Helper()
	dataDir := t.TempDir()
	coordinator := dataportability.NewCoordinator(dataDir, "test", "test", "baseline-v1", dataportability.NewStagingManager(dataDir))
	if err := coordinator.RegisterContributors(chatCleanupBackupTestContributor{db: p.services.KernelContainer.DeviceRegistry.Database(), ownerID: p.ownerID}); err != nil {
		t.Fatal(err)
	}
	p.services.DataPortability = coordinator
}

func (chatCleanupBackupTestContributor) ComponentIDs() []string {
	return []string{chat.ComponentIDChatConversations, chat.ComponentIDChatMessages}
}
func (c chatCleanupBackupTestContributor) Plan(ctx context.Context, _ dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	conversations, err := c.count(ctx, "conversation")
	if err != nil {
		return nil, err
	}
	messages, err := c.count(ctx, "message")
	if err != nil {
		return nil, err
	}
	return []dataportability.BackupComponentPlan{
		{ID: chat.ComponentIDChatConversations, Kind: dataportability.KindNDJSON, LogicalName: chat.ComponentIDChatConversations, Required: true, SourceOfTruth: true, ItemCount: conversations, EstimatedSize: conversations * 1024},
		{ID: chat.ComponentIDChatMessages, Kind: dataportability.KindNDJSON, LogicalName: chat.ComponentIDChatMessages, Required: true, SourceOfTruth: true, ItemCount: messages, EstimatedSize: messages * 2048},
	}, nil
}
func (c chatCleanupBackupTestContributor) count(ctx context.Context, kind string) (int64, error) {
	var total int64
	if err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? AND deleted=0`, c.ownerID, kind).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}
func (c chatCleanupBackupTestContributor) Export(ctx context.Context, _ dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if err := c.exportKind(ctx, out, "conversation", chat.ComponentIDChatConversations); err != nil {
		return err
	}
	return c.exportKind(ctx, out, "message", chat.ComponentIDChatMessages)
}
func (c chatCleanupBackupTestContributor) exportKind(ctx context.Context, out dataportability.BackupWriter, kind, componentID string) error {
	rows, err := c.db.QueryContext(ctx, `SELECT kind,resource_id,role_id,revision,deleted,body,updated_at FROM kernel_device_owned_resources WHERE owner_id=? AND kind=? ORDER BY resource_id`, c.ownerID, kind)
	if err != nil {
		return err
	}
	defer rows.Close()
	writer, err := out.CreateComponent(componentID, componentID, dataportability.KindNDJSON)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	for rows.Next() {
		var kind, id, role, body, updated string
		var revision int64
		var deleted bool
		if err := rows.Scan(&kind, &id, &role, &revision, &deleted, &body, &updated); err != nil {
			return err
		}
		if err := encoder.Encode(map[string]any{"kind": kind, "id": id, "roleId": role, "revision": revision, "deleted": deleted, "body": json.RawMessage(body), "updatedAt": updated}); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return writer.Close()
}
func (chatCleanupBackupTestContributor) PreviewImport(context.Context, dataportability.ImportPreviewRequest, dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	return nil, nil
}
func (chatCleanupBackupTestContributor) Import(context.Context, dataportability.ImportRequest, dataportability.BackupReader) error {
	return nil
}

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
	for _, operation := range []string{"conversations", "search"} {
		for _, page := range []int{math.MaxInt, math.MaxInt/2 + 2} {
			pageResult, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{
				Operation: operation, ConversationOwnerID: p.ownerID, Keyword: "visible", Page: page, PageSize: 2,
			})
			if err != nil || pageResult["page"] != page || pageResult["total"] != 1 {
				t.Fatalf("%s page=%d result=%v err=%v", operation, page, pageResult, err)
			}
			items, ok := pageResult["items"].([]map[string]any)
			if !ok || items == nil || len(items) != 0 {
				t.Fatalf("%s page=%d items=%v", operation, page, pageResult["items"])
			}
		}
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

func TestOwnedChatManagementArchivedOnlySelection(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "archived"}
	seedOwnedChatManagement(t, p, "conversation", "archived", "one", `{"title":"Archived","archivedAt":"2026-10-01T00:00:00Z"}`)
	seedOwnedChatManagement(t, p, "conversation", "active", "one", `{"title":"Active","archivedAt":""}`)
	seedOwnedChatManagement(t, p, "message", "archived-message", "one", `{"conversationId":"archived","role":"user","content":"archived message"}`)
	seedOwnedChatManagement(t, p, "message", "active-message", "one", `{"conversationId":"active","role":"user","content":"active message"}`)
	request := coordination.ChatManagementRequest{Operation: "conversations", ConversationOwnerID: p.ownerID, ArchivedOnly: true}
	result, err := p.ManageChat(t.Context(), scope, request)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := result["items"].([]map[string]any)
	if result["total"] != 1 || len(items) != 1 || items[0]["id"] != "archived" {
		t.Fatalf("archived-only list=%v", result)
	}
	_, err = p.services.KernelContainer.DeviceRegistry.Database().Exec(`UPDATE kernel_device_owned_resources SET body=?,revision=revision+1 WHERE owner_id=? AND kind='conversation' AND resource_id='archived'`, []byte(`{"title":"Archived","archivedAt":""}`), p.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	result, err = p.ManageChat(t.Context(), scope, request)
	if err != nil {
		t.Fatal(err)
	}
	if result["total"] != 0 {
		t.Fatalf("restored conversation remained in archived list: %v", result)
	}
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "message-range", ConversationOwnerID: p.ownerID, ConversationID: "active", ArchivedOnly: true}); err == nil {
		t.Fatal("active conversation messages were exposed to archived-only management")
	}
}

func TestOwnedChatCleanupPreservesMemoriesAndDeletesCurrentHistory(t *testing.T) {
	for _, include := range []bool{false, true} {
		t.Run(map[bool]string{false: "keep-memory", true: "delete-memory"}[include], func(t *testing.T) {
			p := setupMeshLocalDataPort(t)
			installChatCleanupBackup(t, p)
			scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "cleanup"}
			seedOwnedChatManagement(t, p, "conversation", "owned", "one", `{"title":"Owned"}`)
			seedOwnedChatManagement(t, p, "message", "owned-message", "one", `{"conversationId":"owned","role":"user","content":"hello"}`)
			seedOwnedChatManagement(t, p, "memory", "memory", "one", `{"conversationId":"owned","content":"remember me"}`)
			seedOwnedChatManagement(t, p, "memory", "owned-memory", "one", `{"conversationId":"owned","content":"owned memory","executionScope":{"roleId":"one"}}`)
			preview, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/preview", ConversationOwnerID: p.ownerID, ConversationIDs: []string{"owned"}, IncludeMemories: include})
			if err != nil {
				t.Fatal(err)
			}
			if preview["estimatedSize"].(int64) <= 0 {
				t.Fatalf("cleanup preview missing data estimate: %v", preview)
			}
			request := coordination.ChatManagementRequest{Operation: "cleanup/confirm", ConversationOwnerID: p.ownerID, PreviewID: preview["previewId"].(string), ConfirmText: "确认清理"}
			result, err := p.ManageChat(t.Context(), scope, request)
			if err != nil {
				t.Fatal(err)
			}
			backupPath, ok := result["backupPath"].(string)
			if !ok || backupPath == "" {
				t.Fatalf("cleanup did not report backup: %v", result)
			}
			if _, err := os.Stat(backupPath); err != nil {
				t.Fatal(err)
			}
			if _, err := p.services.DataPortability.RestorePreview(t.Context(), backupPath); err != nil {
				t.Fatal(err)
			}
			if result["cleanedFormatted"] == "" {
				t.Fatalf("cleanup result missing logical data size: %v", result)
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
	preview, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/preview", ConversationOwnerID: p.ownerID, ConversationIDs: []string{"owned"}})
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

func TestOwnedChatCleanupRequiresConditionAndRejectsEmptySelection(t *testing.T) {
	p := setupMeshLocalDataPort(t)
	scope := coordination.ExecutionScope{TargetDeviceID: p.ownerID, CoreID: "core", ResourceOwnerID: p.ownerID, RoleOwnerID: p.ownerID, RoleID: "one", RoleRevision: 3, RequestID: "cleanup-empty"}
	seedOwnedChatManagement(t, p, "conversation", "owned", "one", `{"title":"Owned","updatedAt":"2026-11-19T00:00:00Z"}`)
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/preview", ConversationOwnerID: p.ownerID}); err == nil || !strings.Contains(err.Error(), "请至少选择一个清理条件") {
		t.Fatalf("preview without condition err=%v", err)
	}
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/preview", ConversationOwnerID: p.ownerID, OlderThanDays: 36500}); err == nil || !strings.Contains(err.Error(), "当前条件没有匹配到任何会话") {
		t.Fatalf("preview without match err=%v", err)
	}
	if _, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/confirm", ConversationOwnerID: p.ownerID, PreviewID: "missing", ConfirmText: "确认清理"}); err == nil {
		t.Fatal("cleanup confirmed without preview")
	}
	preview, err := p.ManageChat(t.Context(), scope, coordination.ChatManagementRequest{Operation: "cleanup/preview", ConversationOwnerID: p.ownerID, ConversationIDs: []string{"owned"}})
	if err != nil {
		t.Fatal(err)
	}
	if preview["conversationCount"] != 1 || preview["messageCount"] != 0 {
		t.Fatalf("unexpected selection: %v", preview)
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

package notificationruntime

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/gorm"
)

func TestResolveMetadataUsesOwnedRuntimePayload(t *testing.T) {
	runtime := NewRuntime(nil, nil, false)
	event := conversationstream.AgentUIEvent{
		ConversationID: "owned/runtime-key",
		MessageID:      "request-1/assistant",
		Type:           "turn.completed",
		Payload: map[string]any{
			"spaceId":           "space-a",
			"conversationId":    "conversation-a",
			"characterId":       "role-a",
			"characterName":     "林澈",
			"conversationTitle": "周末计划",
			"content":           "明天上午出发。",
		},
	}
	meta := runtime.resolveMetadata(context.Background(), event)
	if meta.spaceID != "space-a" ||
		meta.conversationID != "conversation-a" ||
		meta.characterID != "role-a" ||
		meta.characterName != "林澈" ||
		meta.conversationTitle != "周末计划" ||
		meta.messageContent != "明天上午出发。" {
		t.Fatalf("owned event metadata was not preserved: %#v", meta)
	}
}

func TestLegacyBrowserNotificationPreferenceMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE app_settings (key TEXT PRIMARY KEY,value TEXT NOT NULL,deleted_at DATETIME)`).Error; err != nil {
		t.Fatalf("create app settings: %v", err)
	}
	if err := db.Exec(`INSERT INTO app_settings(key,value,deleted_at) VALUES(?,?,NULL),(?,?,NULL)`,
		legacyNotificationSettingKey("space-a", "desktop-a", "enabled"), "false",
		legacyNotificationSettingKey("space-a", "desktop-a", "subscribed"), "true").Error; err != nil {
		t.Fatalf("seed legacy settings: %v", err)
	}
	runtime := NewRuntime(db, nil, false)
	enabled, found := runtime.migratedSystemNotificationEnabled(context.Background(), "space-a", "desktop-a")
	if !found || enabled {
		t.Fatalf("disabled legacy preference was not migrated: found=%v enabled=%v", found, enabled)
	}
	if _, found := runtime.migratedSystemNotificationEnabled(context.Background(), "space-a", "desktop-b"); found {
		t.Fatal("legacy preference leaked to another device")
	}
}

func TestRegisterManagerAcceptsOwnedRuntimeManager(t *testing.T) {
	runtime := NewRuntime(nil, nil, false)
	manager := conversationstream.NewManager()
	runtime.RegisterManager(manager)
	runtime.RegisterManager(manager)
	if len(runtime.managedManagers) != 1 || runtime.managedManagers[0] != manager {
		t.Fatalf("owned runtime manager registration was not idempotent: %#v", runtime.managedManagers)
	}
}

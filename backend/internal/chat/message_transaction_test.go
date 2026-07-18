package chat

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestMessageCommitRollsBackAssistantRowsWhenConversationCountUpdateFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB.Close()
	})
	if err := db.Exec(`CREATE TABLE messages (
id TEXT PRIMARY KEY,
conversation_id TEXT,
sequence INTEGER NOT NULL DEFAULT 0,
role TEXT,
content TEXT,
msg_type TEXT DEFAULT 'text',
tokens INTEGER DEFAULT 0,
source TEXT DEFAULT 'manual',
safety_level TEXT DEFAULT 'normal',
status TEXT DEFAULT 'sent',
include_in_context INTEGER DEFAULT 1,
audio_url TEXT DEFAULT '',
audio_duration REAL DEFAULT 0,
image_url TEXT DEFAULT '',
video_url TEXT DEFAULT '',
emote_id TEXT DEFAULT '',
alt_text TEXT DEFAULT '',
is_animated INTEGER DEFAULT 0,
media_width INTEGER DEFAULT 0,
media_height INTEGER DEFAULT 0,
original_asset_reference TEXT DEFAULT '',
fallback_asset_reference TEXT DEFAULT '',
response_group_id TEXT DEFAULT '',
delivery_sequence INTEGER DEFAULT 0,
emote_decision_status TEXT DEFAULT '',
request_id TEXT DEFAULT '',
reply_to_message_id TEXT,
reply_to_role TEXT,
reply_to_excerpt TEXT,
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE conversations (
id TEXT PRIMARY KEY,
updated_at TEXT DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Message{
		ID:             "user-1",
		ConversationID: "conv-1",
		Role:           "user",
		Content:        "你好",
		MsgType:        "text",
		Source:         "manual",
		Status:         "processing",
		RequestID:      "req-transaction",
	}).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC).Format("2006-01-02 15:04:05")
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&Message{
			ID:             "assistant-1",
			ConversationID: "conv-1",
			Role:           "assistant",
			Content:        "收到",
			MsgType:        "text",
			Source:         "manual",
			RequestID:      "req-transaction",
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Message{}).Where("id = ?", "user-1").Updates(map[string]interface{}{"status": "sent", "updated_at": now}).Error; err != nil {
			return err
		}
		return tx.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", now, "conv-1", "conv-1").Error
	})
	if err == nil {
		t.Fatal("expected conversation count update to fail")
	}

	var assistantRows int64
	if err := db.Model(&Message{}).Where("request_id = ? AND role = ?", "req-transaction", "assistant").Count(&assistantRows).Error; err != nil {
		t.Fatal(err)
	}
	if assistantRows != 0 {
		t.Fatalf("assistant rows committed after failed transaction: %d", assistantRows)
	}

	var status string
	if err := db.Model(&Message{}).Select("status").Where("id = ?", "user-1").Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != "processing" {
		t.Fatalf("user status committed after failed transaction: %s", status)
	}
}

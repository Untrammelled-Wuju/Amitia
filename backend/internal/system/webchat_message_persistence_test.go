package system

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/chat"
	"gorm.io/gorm"
)

func TestPersistQueuedWebChatMessageIsImmediatelyQueryable(t *testing.T) {
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
	if err := db.AutoMigrate(&chat.Conversation{}, &chat.Message{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_messages_conv_sequence_unique ON messages(conversation_id, sequence)").Error; err != nil {
		t.Fatal(err)
	}
	conv := &chat.Conversation{ID: "conv-1", CharacterID: "char-1", Title: "测试", Channel: "web", Source: "web"}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&chat.Message{ID: "existing-1", ConversationID: conv.ID, Role: "assistant", Content: "已有消息", Status: "sent"}).Error; err != nil {
		t.Fatal(err)
	}
	h := &Handler{db: db}
	body := webChatSendRequest{AudioUrl: "audio", AudioDuration: 1.5, ImageUrl: "image", VideoUrl: "video"}
	msg, err := h.persistQueuedWebChatMessage(body, conv.ID, conv.CharacterID, "web", "request-1", "新消息", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID == "" || msg.Sequence != 2 || msg.Status != "queued" {
		t.Fatalf("unexpected message: %#v", msg)
	}
	var stored chat.Message
	if err := db.Where("id = ?", msg.ID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Content != "新消息" || stored.RequestID != "request-1" || stored.Role != "user" {
		t.Fatalf("unexpected stored message: %#v", stored)
	}
	replayed, err := h.persistQueuedWebChatMessage(body, conv.ID, conv.CharacterID, "web", "request-1", "新消息", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != msg.ID {
		t.Fatalf("replayed id = %s, want %s", replayed.ID, msg.ID)
	}
	var count int64
	if err := db.Model(&chat.Message{}).Where("conversation_id = ? AND request_id = ? AND role = ?", conv.ID, "request-1", "user").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("message count = %d, want 1", count)
	}
}

func TestAssistantMessageEventFilter(t *testing.T) {
	if isAssistantMessageEvent(MessageEvent{Role: "user"}) {
		t.Fatal("user event must not be streamed")
	}
	if !isAssistantMessageEvent(MessageEvent{Role: "assistant"}) {
		t.Fatal("assistant event must be streamed")
	}
}

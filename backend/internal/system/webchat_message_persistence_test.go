package system

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/modelerror"
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

func TestPublishModelErrorsWithoutPersistence(t *testing.T) {
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
	if err := db.Create(&chat.Conversation{ID: "conv-vision", Title: "测试", Channel: "web"}).Error; err != nil {
		t.Fatal(err)
	}

	bus := GetMessageEventBus()
	subscriber := bus.Subscribe("vision-error-test", []string{"web"})
	defer bus.Unsubscribe(subscriber.ID)
	h := &Handler{db: db}
	rawError := `{"error":{"code":"InvalidParameter","message":"raw model error"}}`
	modelTypes := []string{"vision", "text", "voice", "vector"}
	for index, modelType := range modelTypes {
		userMessage := chat.Message{ID: "user-" + modelType, ConversationID: "conv-vision", Sequence: int64(index + 1), Role: "user", Content: "测试", RequestID: "request-" + modelType}
		if err := db.Create(&userMessage).Error; err != nil {
			t.Fatal(err)
		}
		h.publishModelError(modelerror.Event{ModelType: modelType, ConversationID: "conv-vision", RequestID: "request-" + modelType, Channel: "web", RawError: rawError})
	}

	var count int64
	if err := db.Model(&chat.Message{}).Where("conversation_id = ? AND role = ?", "conv-vision", "assistant").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("model error persisted %d messages", count)
	}

	for index, modelType := range modelTypes {
		select {
		case event := <-subscriber.Events:
			if event.Role != "assistant" || event.Status != "failed" || !strings.Contains(event.Content, rawError) {
				t.Fatalf("unexpected SSE event: %#v", event)
			}
			data, ok := event.Data.(map[string]interface{})
			if !ok || data["messageType"] != modelType+"_error" || data["rawError"] != rawError || data["requestId"] != "request-"+modelType || data["userMessageId"] != "user-"+modelType || data["userMessageSequence"] != int64(index+1) {
				t.Fatalf("unexpected SSE metadata: %#v", event.Data)
			}
		case <-time.After(time.Second):
			t.Fatalf("%s error SSE event not published", modelType)
		}
	}
}

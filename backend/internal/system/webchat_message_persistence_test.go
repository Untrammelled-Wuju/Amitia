package system

import (
	"path/filepath"
	"strings"
	"testing"

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
	if err := db.AutoMigrate(&chat.Conversation{}, &chat.Message{}, &chat.AssistantTurn{}, &chat.AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_messages_conv_sequence_unique ON messages(conversation_id, sequence)").Error; err != nil {
		t.Fatal(err)
	}
	conv := &chat.Conversation{ID: "conv-1", Title: "测试", Channel: "web", Source: "web"}
	if err := db.Create(conv).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&chat.Message{ID: "existing-1", ConversationID: conv.ID, Role: "assistant", Content: "已有消息", Status: "sent"}).Error; err != nil {
		t.Fatal(err)
	}
	h := &Handler{db: db}
	body := webChatSendRequest{AudioUrl: "audio", AudioDuration: 1.5, ImageUrl: "image", VideoUrl: "video"}
	msg, turn, createdTurn, err := h.persistQueuedWebChatMessage(body, conv.ID, "char-1", "web", "request-1", "新消息", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID == "" || msg.Sequence != 2 || msg.Status != "queued" {
		t.Fatalf("unexpected message: %#v", msg)
	}
	if turn == nil || turn.ID == "" || !createdTurn || turn.Status != "queued" {
		t.Fatalf("unexpected queued turn: %#v created=%v", turn, createdTurn)
	}
	var stored chat.Message
	if err := db.Where("id = ?", msg.ID).First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Content != "新消息" || stored.RequestID != "request-1" || stored.Role != "user" {
		t.Fatalf("unexpected stored message: %#v", stored)
	}
	replayed, replayedTurn, replayCreatedTurn, err := h.persistQueuedWebChatMessage(body, conv.ID, "char-1", "web", "request-1", "新消息", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != msg.ID {
		t.Fatalf("replayed id = %s, want %s", replayed.ID, msg.ID)
	}
	if replayedTurn == nil || replayedTurn.ID != turn.ID || replayCreatedTurn {
		t.Fatalf("replayed turn mismatch: %#v created=%v", replayedTurn, replayCreatedTurn)
	}
	var count int64
	if err := db.Model(&chat.Message{}).Where("conversation_id = ? AND request_id = ? AND role = ?", conv.ID, "request-1", "user").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("message count = %d, want 1", count)
	}
}

func TestPersistQueuedWebChatMessageCreatesConversationWithDefaults(t *testing.T) {
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
	if err := db.AutoMigrate(&chat.Conversation{}, &chat.Message{}, &chat.AssistantTurn{}, &chat.AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{db: db}
	msg, turn, createdTurn, err := h.persistQueuedWebChatMessage(
		webChatSendRequest{},
		"conv-new",
		"char-1",
		"mobile",
		"request-new",
		"第一条消息",
		"",
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID == "" || msg.ConversationID != "conv-new" {
		t.Fatalf("unexpected message: %#v", msg)
	}
	if turn == nil || turn.ConversationID != "conv-new" || !createdTurn {
		t.Fatalf("unexpected turn: %#v created=%v", turn, createdTurn)
	}
	var conversation chat.Conversation
	if err := db.Where("id = ?", "conv-new").First(&conversation).Error; err != nil {
		t.Fatal(err)
	}
	if conversation.PinnedAt != "" || conversation.ArchivedAt != "" {
		t.Fatalf("conversation defaults were not persisted: %#v", conversation)
	}
}

func TestFindWebChatConversationByRequestMakesDraftRetryIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&chat.Conversation{}, &chat.Message{}, &chat.AssistantTurn{}, &chat.AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	h := &Handler{db: db}
	requestID := "draft-request-1"
	message, turn, created, err := h.persistQueuedWebChatMessage(webChatSendRequest{}, "web-first", "char-1", "web", requestID, "第一条", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !created || message == nil || turn == nil {
		t.Fatalf("initial persist did not create one command: message=%#v turn=%#v created=%v", message, turn, created)
	}
	conversationID, err := h.findWebChatConversationByRequest("", requestID)
	if err != nil {
		t.Fatal(err)
	}
	if conversationID != "web-first" {
		t.Fatalf("idempotency lookup conversation=%q, want web-first", conversationID)
	}
	replayedMessage, replayedTurn, replayCreated, err := h.persistQueuedWebChatMessage(webChatSendRequest{}, conversationID, "char-1", "web", requestID, "第一条", "", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if replayCreated || replayedMessage.ID != message.ID || replayedTurn.ID != turn.ID {
		t.Fatalf("draft retry created duplicate: message=%#v turn=%#v created=%v", replayedMessage, replayedTurn, replayCreated)
	}
}

func TestPublishModelErrorsPersistTurnErrorBlocks(t *testing.T) {
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
	if err := db.AutoMigrate(&chat.Conversation{}, &chat.Message{}, &chat.AssistantTurn{}, &chat.AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&chat.Conversation{ID: "conv-vision", Title: "测试", Channel: "web"}).Error; err != nil {
		t.Fatal(err)
	}

	h := &Handler{db: db}
	rawError := `{"error":{"code":"InvalidParameter","message":"raw model error"}}`
	modelTypes := []string{"vision", "text", "voice", "vector"}
	for index, modelType := range modelTypes {
		requestID := "request-" + modelType
		userMessage := chat.Message{ID: "user-" + modelType, ConversationID: "conv-vision", Sequence: int64(index + 1), Role: "user", Content: "测试", RequestID: requestID}
		if err := db.Create(&userMessage).Error; err != nil {
			t.Fatal(err)
		}
		turn := chat.AssistantTurn{
			ID: "turn-" + modelType, ConversationID: "conv-vision", UserMessageID: userMessage.ID,
			RequestID: requestID, ExecutionID: "exec-" + modelType, Sequence: int64(index + 1), Status: "running",
		}
		if err := db.Create(&turn).Error; err != nil {
			t.Fatal(err)
		}
		h.publishModelError(modelerror.Event{ModelType: modelType, ConversationID: "conv-vision", RequestID: requestID, Channel: "web", RawError: rawError})
	}

	var messageCount int64
	if err := db.Model(&chat.Message{}).Where("conversation_id = ? AND role = ?", "conv-vision", "assistant").Count(&messageCount).Error; err != nil {
		t.Fatal(err)
	}
	if messageCount != 0 {
		t.Fatalf("model errors must not create assistant message projections: %d", messageCount)
	}

	var items []chat.AssistantTurnItem
	if err := db.Where("conversation_id = ? AND item_type = ?", "conv-vision", "error").Order("sequence ASC").Find(&items).Error; err != nil {
		t.Fatal(err)
	}
	if len(items) != len(modelTypes) {
		t.Fatalf("error blocks = %d, want %d", len(items), len(modelTypes))
	}
	for index, item := range items {
		modelType := modelTypes[index]
		if item.Status != "failed" || item.ErrorCode != modelType+"_model_error" {
			t.Fatalf("unexpected error block: %#v", item)
		}
		if !strings.Contains(item.ResultJSON, rawError) || !strings.Contains(item.ResultJSON, modelType) {
			t.Fatalf("missing error metadata: %s", item.ResultJSON)
		}
	}
}

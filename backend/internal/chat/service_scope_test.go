package chat

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupScopedChatService(t *testing.T) *service {
	t.Helper()
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
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &character.Character{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&character.Character{ID: "char-1", Name: "Amitia", Identity: "心理模拟伙伴", Status: "enabled"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&character.Character{ID: "char-2", Name: "Other", Identity: "心理模拟伙伴", Status: "enabled"}).Error; err != nil {
		t.Fatal(err)
	}
	ctx := app.NewAppContext(db, nil)
	return &service{repo: NewRepository(ctx), charRepo: character.NewRepository(ctx), db: db}
}

func TestProcessMessageRejectsConversationCharacterMismatch(t *testing.T) {
	svc := setupScopedChatService(t)
	if err := svc.db.Create(&Conversation{ID: "conv-1", CharacterID: "char-2", Title: "旧会话", Channel: "web", Source: "manual"}).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Message:        "你好",
		Channel:        "web",
		RequestID:      "req-mismatch-char",
	})
	if err == nil || !strings.Contains(err.Error(), "会话与角色或渠道不匹配") {
		t.Fatalf("expected scope mismatch error, got %v", err)
	}
	var count int64
	if err := svc.db.Model(&Message{}).Where("conversation_id = ?", "conv-1").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no messages written, got %d", count)
	}
}

func TestProcessMessageRejectsConversationChannelMismatch(t *testing.T) {
	svc := setupScopedChatService(t)
	if err := svc.db.Create(&Conversation{ID: "conv-2", CharacterID: "char-1", Title: "旧会话", Channel: "qq", Source: "manual"}).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:    "char-1",
		ConversationID: "conv-2",
		Message:        "你好",
		Channel:        "web",
		RequestID:      "req-mismatch-channel",
	})
	if err == nil || !strings.Contains(err.Error(), "会话与角色或渠道不匹配") {
		t.Fatalf("expected scope mismatch error, got %v", err)
	}
	var count int64
	if err := svc.db.Model(&Message{}).Where("conversation_id = ?", "conv-2").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no messages written, got %d", count)
	}
}

func TestChatRejectsConversationChannelMismatch(t *testing.T) {
	svc := setupScopedChatService(t)
	if err := svc.db.Create(&Conversation{ID: "conv-3", CharacterID: "char-1", Title: "旧会话", Channel: "qq", Source: "manual"}).Error; err != nil {
		t.Fatal(err)
	}

	_, err := svc.Chat(&ChatRequest{
		CharacterID:    "char-1",
		ConversationID: "conv-3",
		Message:        "你好",
		Channel:        "web",
	})
	if err == nil || !strings.Contains(err.Error(), "会话与角色或渠道不匹配") {
		t.Fatalf("expected scope mismatch error, got %v", err)
	}
}

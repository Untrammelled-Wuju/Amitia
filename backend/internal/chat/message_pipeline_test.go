package chat

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupProcessMessageTest(t *testing.T, llm llmWithToolsFunc) (*gorm.DB, *service, string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "process.db")), &gorm.Config{})
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
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &ModelConfig{}, &character.Character{}); err != nil {
		t.Fatal(err)
	}
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	charRepo := character.NewRepository(ctx)
	charID := "char-process"
	convID := "conv-process"
	if err := db.Create(&character.Character{
		ID:                charID,
		Name:              "Amitia",
		Identity:          "AI伙伴",
		Status:            "enabled",
		PersonalityConfig: "{}",
		ChatStyleConfig:   "{}",
		SceneRules:        "{}",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Conversation{ID: convID, CharacterID: charID, Title: "hello", Channel: "web", Source: "manual"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&ModelConfig{
		Name:        "test-model",
		APIType:     "openai-compatible",
		BaseURL:     "http://127.0.0.1",
		APIKey:      "test",
		ModelName:   "test",
		Temperature: 0.7,
		MaxTokens:   128,
		IsActive:    1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return db, &service{repo: repo, charRepo: charRepo, db: db, llmWithTools: llm}, convID
}

func TestProcessMessageDoesNotCommitAssistantWhenGenerationFails(t *testing.T) {
	db, svc, convID := setupProcessMessageTest(t, func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		return "", "", nil, 0, errors.New("model down")
	})
	_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:    "char-process",
		ConversationID: convID,
		Channel:        "web",
		Source:         "manual",
		Message:        "你好",
		RequestID:      "req-fail",
	})
	if err == nil || !strings.Contains(err.Error(), "AI 调用失败") {
		t.Fatalf("expected AI failure, got %v", err)
	}
	var assistantCount int64
	if err := db.Model(&Message{}).Where("conversation_id = ? AND role = ?", convID, "assistant").Count(&assistantCount).Error; err != nil {
		t.Fatal(err)
	}
	if assistantCount != 0 {
		t.Fatalf("expected no assistant message, got %d", assistantCount)
	}
	var status string
	if err := db.Model(&Message{}).Select("status").Where("conversation_id = ? AND role = ? AND request_id = ?", convID, "user", "req-fail").Row().Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("expected user message failed, got %s", status)
	}
}

func TestProcessMessageCommitsAssistantAfterGenerationSucceeds(t *testing.T) {
	db, svc, convID := setupProcessMessageTest(t, func(context.Context, *ModelConfig, []map[string]interface{}, []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		return "第一句\n第二句", "", nil, 12, nil
	})
	resp, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:    "char-process",
		ConversationID: convID,
		Channel:        "web",
		Source:         "manual",
		Message:        "你好",
		RequestID:      "req-ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Reply != "第一句\n第二句" || len(resp.MessageIDs) != 2 {
		t.Fatalf("unexpected response: %#v", resp)
	}
	var assistantCount int64
	if err := db.Model(&Message{}).Where("conversation_id = ? AND role = ? AND request_id = ?", convID, "assistant", "req-ok").Count(&assistantCount).Error; err != nil {
		t.Fatal(err)
	}
	if assistantCount != 2 {
		t.Fatalf("expected two assistant messages, got %d", assistantCount)
	}
	var status string
	if err := db.Model(&Message{}).Select("status").Where("conversation_id = ? AND role = ? AND request_id = ?", convID, "user", "req-ok").Row().Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "sent" {
		t.Fatalf("expected user message sent, got %s", status)
	}
}

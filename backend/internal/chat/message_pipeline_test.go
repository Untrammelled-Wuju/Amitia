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
	"github.com/u-ai/backend/internal/interaction"
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
	if resp.Sequence == 0 {
		t.Fatalf("expected response sequence to be set: %#v", resp)
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
	replayed, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
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
	if replayed.Sequence != resp.Sequence {
		t.Fatalf("expected idempotent response sequence %d, got %d", resp.Sequence, replayed.Sequence)
	}
}

func TestProcessMessageBuildsPromptThroughIR(t *testing.T) {
	var captured []map[string]interface{}
	_, svc, convID := setupProcessMessageTest(t, func(_ context.Context, _ *ModelConfig, messages []map[string]interface{}, _ []tool.Tool) (string, string, []map[string]interface{}, int, error) {
		captured = messages
		return "收到", "", nil, 8, nil
	})
	_, err := svc.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:    "char-process",
		ConversationID: convID,
		Channel:        "web",
		Source:         "manual",
		Message:        "你好",
		RequestID:      "req-ir",
		Runtime: &interaction.RuntimeAssembly{
			Path:        interaction.PathTypeDeep,
			Safety:      interaction.RuntimeSafetyDecision{Level: "conservative", Reasons: []string{"high_stress"}},
			Delivery:    interaction.RuntimeDeliveryIntent{Channel: "web", RequiresText: true},
			Transaction: interaction.TransactionDefinition{Name: interaction.TransactionBoundaryAll},
			Context:     interaction.ContextSnapshot{Version: "context-snapshot-v1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(captured) == 0 {
		t.Fatal("expected prompt messages")
	}
	last := captured[len(captured)-1]
	lastContent, _ := last["content"].(string)
	if last["role"] != "user" || !strings.Contains(lastContent, "你好") {
		t.Fatalf("expected current input as final user message, got %#v", last)
	}
	joined := ""
	for _, msg := range captured {
		if content, ok := msg["content"].(string); ok {
			joined += content + "\n"
		}
	}
	if !strings.Contains(joined, "runtime_plan") {
		t.Fatalf("runtime plan/expression not rendered in prompt: %s", joined)
	}
	if !strings.Contains(joined, "路径: deep") {
		t.Fatalf("runtime path missing from prompt: %s", joined)
	}
	if !strings.Contains(joined, "high_stress") {
		t.Fatalf("runtime safety reasons missing from prompt: %s", joined)
	}
}

func TestBuildProcessPromptMessagesGatewayOnlyFirstSystem(t *testing.T) {
	messages := buildProcessPromptMessages(processPromptInput{
		CharacterConfig:   "你是 Amitia",
		PersonalityConfig: "遵守当前渠道策略",
		History: []map[string]string{
			{"role": "user", "content": "上一轮"},
			{"role": "assistant", "content": "上一轮回复"},
		},
		Runtime: &interaction.RuntimeAssembly{
			Path:        interaction.PathTypeStandard,
			Safety:      interaction.RuntimeSafetyDecision{Level: "normal"},
			Delivery:    interaction.RuntimeDeliveryIntent{Channel: "web", RequiresText: true},
			Transaction: interaction.TransactionDefinition{Name: interaction.TransactionBoundaryAll},
			Context:     interaction.ContextSnapshot{Version: "context-snapshot-v1"},
		},
		StyleInstruction: "保持简洁",
		UserContent:      "当前输入",
	})
	if len(messages) == 0 {
		t.Fatal("expected prompt messages")
	}

	if messages[0]["role"] != "system" {
		t.Fatalf("first message must be system, got %#v", messages[0])
	}

	for i := 1; i < len(messages); i++ {
		if messages[i]["role"] == "system" {
			t.Fatalf("message %d must not be system", i)
		}
	}

	last := messages[len(messages)-1]
	if last["role"] != "user" {
		t.Fatalf("last message must be user, got %#v", last)
	}
	lastContent, _ := last["content"].(string)
	if !strings.Contains(lastContent, "当前输入") {
		t.Fatalf("last message missing user content: %s", lastContent)
	}
	if !strings.Contains(lastContent, "<current_user_message>") {
		t.Fatalf("last message missing current_user_message tag")
	}

	joined := ""
	for _, msg := range messages {
		if content, ok := msg["content"].(string); ok {
			joined += content + "\n"
		}
	}
	if !strings.Contains(joined, "<untrusted_data") {
		t.Fatalf("history should be in untrusted_data: %s", joined)
	}
	if !strings.Contains(joined, "runtime_plan") && !strings.Contains(joined, "character_contract") {
		t.Fatalf("missing runtime/character section: %s", joined)
	}
}

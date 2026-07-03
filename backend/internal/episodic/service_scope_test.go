package episodic

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func newEpisodicTestService(t *testing.T) (*service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&EpisodicMemory{}); err != nil {
		t.Fatalf("migrate episodic memories: %v", err)
	}
	if err := db.Exec(`CREATE TABLE conversations (id text primary key, character_id text not null default '')`).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}
	if err := db.Exec(`CREATE TABLE messages (id text primary key, conversation_id text not null, role text, content text, created_at text)`).Error; err != nil {
		t.Fatalf("create messages: %v", err)
	}
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc, ok := NewService(repo, ctx, graph.NewStubService()).(*service)
	if !ok {
		t.Fatalf("unexpected service type")
	}
	return svc, db
}

func TestSystemPromptUsesCharacterScope(t *testing.T) {
	svc, _ := newEpisodicTestService(t)

	_, err := svc.Create(&CreateEpisodicRequest{
		CharacterID:    "char-a",
		SceneType:      "milestone",
		Title:          "海边约定",
		Content:        "和A角色约定去看海",
		SentimentScore: 8,
	})
	if err != nil {
		t.Fatalf("create char-a episodic: %v", err)
	}
	_, err = svc.Create(&CreateEpisodicRequest{
		CharacterID:    "char-b",
		SceneType:      "milestone",
		Title:          "山顶约定",
		Content:        "和B角色约定去爬山",
		SentimentScore: 7,
	})
	if err != nil {
		t.Fatalf("create char-b episodic: %v", err)
	}

	prompt := svc.ToSystemPrompt("default", "char-a")
	if !strings.Contains(prompt, "海边约定") {
		t.Fatalf("prompt missing char-a memory: %s", prompt)
	}
	if strings.Contains(prompt, "山顶约定") {
		t.Fatalf("prompt leaked char-b memory: %s", prompt)
	}
}

func TestToolSaveUsesConversationCharacterScope(t *testing.T) {
	svc, db := newEpisodicTestService(t)
	if err := db.Exec(`INSERT INTO conversations (id, character_id) VALUES (?, ?)`, "conv-a", "char-a").Error; err != nil {
		t.Fatalf("insert conversation: %v", err)
	}

	item, err := svc.SaveFromTool("default", "insight", "雨夜谈心", "A角色雨夜安慰用户", 6, "conv-a", "", "")
	if err != nil {
		t.Fatalf("save from tool: %v", err)
	}
	if item.UserID != "char-a" {
		t.Fatalf("tool episodic scope = %q, want char-a", item.UserID)
	}

	prompt := svc.ToSystemPrompt("default", "char-b")
	if strings.Contains(prompt, "雨夜谈心") {
		t.Fatalf("prompt leaked char-a tool memory: %s", prompt)
	}
}

func TestRuntimePromptIgnoresUnknownScope(t *testing.T) {
	svc, _ := newEpisodicTestService(t)

	_, err := svc.SaveFromTool("default", "insight", "无作用域", "不应进入运行时提示", 5, "", "", "")
	if err == nil {
		t.Fatalf("save without scope should fail")
	}
	prompt := svc.ToSystemPrompt("default")
	if prompt != "" {
		t.Fatalf("prompt should ignore default or unknown scope: %s", prompt)
	}
}

func TestGetDetailWithMessagesLimitsMessagesToSourceConversation(t *testing.T) {
	svc, db := newEpisodicTestService(t)
	if err := db.Exec(`INSERT INTO conversations (id, character_id) VALUES (?, ?)`, "conv-a", "char-a").Error; err != nil {
		t.Fatalf("insert source conversation: %v", err)
	}
	if err := db.Exec(`INSERT INTO conversations (id, character_id) VALUES (?, ?)`, "conv-b", "char-b").Error; err != nil {
		t.Fatalf("insert other conversation: %v", err)
	}
	rows := []struct {
		id      string
		convID  string
		content string
		at      string
	}{
		{"a-1", "conv-a", "source start", "2026-01-01 10:00:00"},
		{"b-1", "conv-b", "other middle", "2026-01-01 10:30:00"},
		{"a-2", "conv-a", "source end", "2026-01-01 11:00:00"},
	}
	for _, row := range rows {
		if err := db.Exec(`INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`, row.id, row.convID, "user", row.content, row.at).Error; err != nil {
			t.Fatalf("insert message %s: %v", row.id, err)
		}
	}

	item, err := svc.Create(&CreateEpisodicRequest{
		CharacterID:    "char-a",
		SceneType:      "milestone",
		Title:          "限定会话",
		Content:        "只返回来源会话消息",
		MessageIDStart: "a-1",
		MessageIDEnd:   "a-2",
		SourceConvID:   "conv-a",
	})
	if err != nil {
		t.Fatalf("create episodic: %v", err)
	}

	_, messages, err := svc.GetDetail(item.ID)
	if err != nil {
		t.Fatalf("get detail: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, want 2: %#v", len(messages), messages)
	}
	for _, message := range messages {
		if message["conversation_id"] != "conv-a" {
			t.Fatalf("message leaked from other conversation: %#v", message)
		}
	}
}

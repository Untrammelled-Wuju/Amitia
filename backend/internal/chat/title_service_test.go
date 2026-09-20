package chat

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTitleServiceTest(t *testing.T) (*gorm.DB, *service) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "title.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.AutoMigrate(&Conversation{}, &AssistantTurn{}); err != nil {
		t.Fatalf("migrate title tables: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("open sql database: %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return db, &service{db: db}
}

func TestUpdateGeneratedConversationTitle(t *testing.T) {
	db, svc := newTitleServiceTest(t)
	var published ConversationTitleUpdatedEvent
	SetConversationTitleUpdatedPublisher(func(event ConversationTitleUpdatedEvent) {
		published = event
	})
	t.Cleanup(func() {
		SetConversationTitleUpdatedPublisher(nil)
	})
	conversation := &Conversation{
		ID:       "conv-title",
		Title:    "首条用户消息",
		Revision: 1,
		Channel:  "web",
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, err := svc.updateGeneratedConversationTitle("conv-title", "首条用户消息", "项目进度讨论")
	if err != nil {
		t.Fatalf("update generated title: %v", err)
	}
	if !updated {
		t.Fatal("expected generated title update")
	}
	var stored Conversation
	if err := db.First(&stored, "id = ?", "conv-title").Error; err != nil {
		t.Fatalf("load conversation: %v", err)
	}
	if stored.Title != "项目进度讨论" || stored.Revision != 2 {
		t.Fatalf("unexpected conversation: title=%q revision=%d", stored.Title, stored.Revision)
	}
	if published.ConversationID != "conv-title" || published.Title != "项目进度讨论" || published.Channel != "web" {
		t.Fatalf("unexpected title update event: %#v", published)
	}
}

func TestUpdateGeneratedConversationTitleDoesNotOverwriteManualRename(t *testing.T) {
	db, svc := newTitleServiceTest(t)
	conversation := &Conversation{
		ID:       "conv-manual",
		Title:    "用户手动标题",
		Revision: 3,
	}
	if err := db.Create(conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	updated, err := svc.updateGeneratedConversationTitle("conv-manual", "首条用户消息", "AI 标题")
	if err != nil {
		t.Fatalf("update generated title: %v", err)
	}
	if updated {
		t.Fatal("manual title must not be overwritten")
	}
}

func TestNormalizeGeneratedConversationTitle(t *testing.T) {
	if got := normalizeGeneratedConversationTitle("  “标题：项目进度讨论。”  "); got != "项目进度讨论" {
		t.Fatalf("unexpected normalized title: %q", got)
	}
	if got := normalizeGeneratedConversationTitle("“”"); got != "" {
		t.Fatalf("empty title should be rejected: %q", got)
	}
}

func TestNormalizeTitleGenerationReply(t *testing.T) {
	got := normalizeTitleGenerationReply("第一段[AMITIA_BR]第二段[AMITIA_OTHER]第三段")
	if got != "第一段\n第二段\n第三段" {
		t.Fatalf("unexpected title generation reply: %q", got)
	}
}

func TestConversationTitleGenerationUsesAssistantTurns(t *testing.T) {
	db, svc := newTitleServiceTest(t)
	if err := db.Create(&AssistantTurn{
		ID:             "turn-title",
		ConversationID: "conv-split",
		Status:         assistantTurnStatusCompleted,
	}).Error; err != nil {
		t.Fatalf("create assistant turn: %v", err)
	}
	if !svc.isFirstCompletedAssistantTurn("conv-split") {
		t.Fatal("first completed assistant turn should allow title generation")
	}
	if err := db.Create(&AssistantTurn{
		ID:             "turn-title-2",
		ConversationID: "conv-split",
		Status:         assistantTurnStatusCompleted,
	}).Error; err != nil {
		t.Fatalf("create second assistant turn: %v", err)
	}
	if svc.isFirstCompletedAssistantTurn("conv-split") {
		t.Fatal("later completed assistant turns must not trigger title generation")
	}
}

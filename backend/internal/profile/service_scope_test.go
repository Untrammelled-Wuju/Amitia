package profile

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func newProfileTestService(t *testing.T) (*service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&UserProfile{}); err != nil {
		t.Fatalf("migrate user profiles: %v", err)
	}
	if err := db.Exec(`CREATE TABLE conversations (id text primary key, character_id text not null default '')`).Error; err != nil {
		t.Fatalf("create conversations: %v", err)
	}
	ctx := app.NewAppContext(db, nil)
	repo := NewRepository(ctx)
	svc, ok := NewService(repo, ctx, graph.NewStubService()).(*service)
	if !ok {
		t.Fatalf("unexpected service type")
	}
	return svc, db
}

func TestCreatePreservesCharacterScope(t *testing.T) {
	svc, _ := newProfileTestService(t)

	item, err := svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		CharacterID:    "char-a",
		Category:       "preference",
		AttributeName:  "颜色",
		AttributeValue: "蓝色",
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if item.CharacterID != "char-a" {
		t.Fatalf("character scope lost: %q", item.CharacterID)
	}
}

func TestToolUpsertUsesConversationCharacterScope(t *testing.T) {
	svc, db := newProfileTestService(t)
	if err := db.Exec(`INSERT INTO conversations (id, character_id) VALUES (?, ?)`, "conv-a", "char-a").Error; err != nil {
		t.Fatalf("insert conversation: %v", err)
	}

	item, err := svc.UpsertFromTool("user-1", "preference", "饮料", "茶", 80, "conv-a")
	if err != nil {
		t.Fatalf("upsert from tool: %v", err)
	}
	if item.CharacterID != "char-a" {
		t.Fatalf("tool profile scope = %q, want char-a", item.CharacterID)
	}

	items, err := svc.repo.GetScopedByUserID("user-1", "char-b")
	if err != nil {
		t.Fatalf("query char-b profiles: %v", err)
	}
	for _, got := range items {
		if got.AttributeName == "饮料" && got.AttributeValue == "茶" {
			t.Fatalf("char-b received char-a profile: %+v", got)
		}
	}
}

func TestSystemPromptUsesRequestedCharacterScope(t *testing.T) {
	svc, _ := newProfileTestService(t)
	_, err := svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		CharacterID:    "char-a",
		Category:       "preference",
		AttributeName:  "称呼",
		AttributeValue: "姐姐",
		Confidence:     80,
	})
	if err != nil {
		t.Fatalf("create char-a profile: %v", err)
	}
	_, err = svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		CharacterID:    "char-b",
		Category:       "preference",
		AttributeName:  "称呼",
		AttributeValue: "老师",
		Confidence:     80,
	})
	if err != nil {
		t.Fatalf("create char-b profile: %v", err)
	}
	_, err = svc.Create(&CreateProfileRequest{
		UserID:         "user-1",
		Category:       "preference",
		AttributeName:  "语言",
		AttributeValue: "中文",
		Confidence:     80,
	})
	if err != nil {
		t.Fatalf("create global profile: %v", err)
	}

	prompt := svc.ToSystemPrompt("user-1", "char-a")
	if !strings.Contains(prompt, "姐姐") {
		t.Fatalf("prompt missing char-a profile: %s", prompt)
	}
	if strings.Contains(prompt, "老师") {
		t.Fatalf("prompt leaked char-b profile: %s", prompt)
	}
	if !strings.Contains(prompt, "中文") {
		t.Fatalf("prompt missing global profile fallback: %s", prompt)
	}
}

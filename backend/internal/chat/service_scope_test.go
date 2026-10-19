package chat

import (
	"errors"
	"path/filepath"
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

func TestConversationScopeRejectsChannelMismatch(t *testing.T) {
	svc := setupScopedChatService(t)
	if err := svc.db.Create(&Conversation{ID: "conv-3", SpaceID: normalizeConversationOwner(""), Title: "旧会话", Channel: "qq", Source: "manual"}).Error; err != nil {
		t.Fatal(err)
	}

	err := svc.validateConversationScope("conv-3", "web", normalizeConversationOwner(""))
	if !errors.Is(err, ErrConversationScopeMismatch) {
		t.Fatalf("expected scope mismatch error, got %v", err)
	}
}

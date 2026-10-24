package chat

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func newCitationTestService(t *testing.T) *service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&Conversation{}, &Message{}, &AssistantTurn{}, &AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	return &service{repo: NewRepository(app.NewAppContext(db, nil)), db: db}
}

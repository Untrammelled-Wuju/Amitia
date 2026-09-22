package interaction

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSQLiteInteractionTrackerPersistsThreadScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "tracker-thread.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	tracker := NewSQLiteInteractionTracker(db)
	if err := tracker.InitSchema(); err != nil {
		t.Fatal(err)
	}
	record := NewInteractionRecord(InteractionScope{SpaceID: "space-1", CharacterID: "char-1", ConversationID: "conv-1", ThreadID: "thread-1", Channel: "web", RequestID: "req-1"})
	if err := tracker.Create(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := tracker.Get(context.Background(), record.ID)
	if err != nil || !ok {
		t.Fatalf("load interaction: ok=%v err=%v", ok, err)
	}
	if loaded.Scope.ThreadID != "thread-1" {
		t.Fatalf("thread id = %q, want thread-1", loaded.Scope.ThreadID)
	}
}

package chat

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/psyche"
	"gorm.io/gorm"
)

func TestUpdatePsycheStateInitializesAndVersionsSQLiteState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "chat-psyche.db")), &gorm.Config{})
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
	store := psyche.NewSQLitePsycheStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatal(err)
	}
	svc := &service{psycheStore: store}

	if err := svc.updatePsycheState("char-chat-psyche"); err != nil {
		t.Fatalf("first update failed: %v", err)
	}
	if err := svc.updatePsycheState("char-chat-psyche"); err != nil {
		t.Fatalf("second update failed: %v", err)
	}

	state, err := store.LoadState("char-chat-psyche")
	if err != nil {
		t.Fatal(err)
	}
	if state.StateVersion != 3 {
		t.Fatalf("expected version 3 after two chat updates, got %d", state.StateVersion)
	}
	if state.Energy >= 0.7 {
		t.Fatalf("expected interaction energy delta to be persisted, got %f", state.Energy)
	}
	snapshots, err := store.LoadSnapshots("char-chat-psyche", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}
	var events int64
	if err := db.Table("psyche_events").Where("character_id = ?", "char-chat-psyche").Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	if events != 2 {
		t.Fatalf("expected 2 events, got %d", events)
	}
}

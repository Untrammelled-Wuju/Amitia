package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/conversationstream"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackfillConversationEventFiles(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "backfill.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&chat.AssistantTurn{}, &chat.AssistantTurnItem{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	turn := chat.AssistantTurn{
		ID: "turn-1", ConversationID: "conv-1", CharacterID: "char-1",
		UserMessageID: "message-1", RequestID: "request-1", ExecutionID: "exec-1",
		Sequence: 1, Status: "completed", CreatedAt: "2026-09-23 10:00:00", UpdatedAt: "2026-09-23 10:00:02",
	}
	if err := db.Create(&turn).Error; err != nil {
		t.Fatal(err)
	}
	item := chat.AssistantTurnItem{
		ID: "item-1", TurnID: turn.ID, ConversationID: turn.ConversationID, Sequence: 1,
		ItemType: "text", Status: "completed", Revision: 2, Content: "历史回复",
		CreatedAt: "2026-09-23 10:00:01", UpdatedAt: "2026-09-23 10:00:02",
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	store, err := conversationstream.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := backfillConversationEventFiles(db, store); err != nil {
		t.Fatal(err)
	}
	latest, err := store.LatestSequence(context.Background(), turn.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if latest != 3 {
		t.Fatalf("latest sequence = %d, want 3", latest)
	}
	events, err := store.ListAfter(context.Background(), turn.ConversationID, 0, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[1].Type != "text.completed" {
		t.Fatalf("unexpected backfill events: %#v", events)
	}
	if err := backfillConversationEventFiles(db, store); err != nil {
		t.Fatal(err)
	}
	latest, err = store.LatestSequence(context.Background(), turn.ConversationID)
	if err != nil {
		t.Fatal(err)
	}
	if latest != 3 {
		t.Fatalf("backfill was not idempotent: latest=%d", latest)
	}
}

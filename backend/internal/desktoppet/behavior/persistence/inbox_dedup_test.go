package persistence

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"gorm.io/gorm"
)

func TestBehaviorInboxDedupIsScopedByUserAndCharacter(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range DesktopPetBehaviorTableSQL {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, idx := range DesktopPetBehaviorIndexDefs {
		prefix := "CREATE INDEX"
		if idx.Unique {
			prefix = "CREATE UNIQUE INDEX"
		}
		statement := fmt.Sprintf("%s IF NOT EXISTS %s ON %s(%s)", prefix, idx.Name, idx.Table, joinIndexColumns(idx.Columns))
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := NewGormBehaviorStateRepository(db)
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	makeEvent := func(eventID, userID, characterID string) behavior.BehaviorEventEnvelope {
		return behavior.BehaviorEventEnvelope{
			EventID:       eventID,
			DedupKey:      "shared-business-id",
			EventType:     "chat.response.completed",
			SchemaVersion: 1,
			OccurredAt:    now,
			ReceivedAt:    now,
			UserID:        userID,
			CharacterID:   characterID,
			Origin:        behavior.OriginChat,
		}
	}

	inserted, err := repo.InsertInboxIfAbsent(context.Background(), makeEvent("event-1", "user-1", "char-1"))
	if err != nil || !inserted {
		t.Fatalf("first insert = %v, %v", inserted, err)
	}
	inserted, err = repo.InsertInboxIfAbsent(context.Background(), makeEvent("event-2", "user-2", "char-1"))
	if err != nil || !inserted {
		t.Fatalf("same dedup key for another user must be accepted: inserted=%v err=%v", inserted, err)
	}
	inserted, err = repo.InsertInboxIfAbsent(context.Background(), makeEvent("event-3", "user-1", "char-2"))
	if err != nil || !inserted {
		t.Fatalf("same dedup key for another character must be accepted: inserted=%v err=%v", inserted, err)
	}
	inserted, err = repo.InsertInboxIfAbsent(context.Background(), makeEvent("event-4", "user-1", "char-1"))
	if err != nil {
		t.Fatal(err)
	}
	if inserted {
		t.Fatal("duplicate key inside the same user+character scope was inserted")
	}
}

func joinIndexColumns(columns []string) string {
	result := ""
	for i, column := range columns {
		if i > 0 {
			result += ","
		}
		result += column
	}
	return result
}

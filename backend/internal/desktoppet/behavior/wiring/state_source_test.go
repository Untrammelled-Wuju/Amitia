package wiring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInteractionStatusToBehaviorPhase(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"received", "received"},
		{"normalized", "received"},
		{"queued", "received"},
		{"processing", "context_loading"},
		{"context_ready", "response_started"},
		{"decided", "response_started"},
		{"generated", "response_ready"},
		{"committed", "response_ready"},
		{"delivery_pending", "response_ready"},
		{"delivered", "response_ready"},
		{"completed", ""},
		{"failed", ""},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := interactionStatusToBehaviorPhase(tt.status); got != tt.want {
				t.Fatalf("interactionStatusToBehaviorPhase(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func openStateSourceTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE interaction_records (
		id TEXT PRIMARY KEY,
		user_id TEXT,
		character_id TEXT,
		conversation_id TEXT,
		channel TEXT,
		request_id TEXT,
		source TEXT,
		session_id TEXT,
		status TEXT,
		status_version INTEGER,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestQueryActiveInteractionsKeepsNewestCreatedInteractionInsideLimit(t *testing.T) {
	db := openStateSourceTestDB(t, "state-source-latest-created")
	base := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)

	// Ten older interactions receive later updates. Ordering by updated_at with a
	// limit of ten would incorrectly drop the newly created foreground request.
	for i := 0; i < 10; i++ {
		created := base.Add(time.Duration(i) * time.Minute)
		updated := base.Add(2*time.Hour + time.Duration(i)*time.Minute)
		if err := db.Exec(`INSERT INTO interaction_records
			(id, user_id, character_id, conversation_id, channel, request_id, status, status_version, created_at, updated_at)
			VALUES (?, 'user-1', 'character-1', ?, 'web', ?, 'processing', 9, ?, ?)`,
			fmt.Sprintf("old-%d", i), fmt.Sprintf("old-conv-%d", i), fmt.Sprintf("old-req-%d", i), created, updated).Error; err != nil {
			t.Fatal(err)
		}
	}
	newCreated := base.Add(30 * time.Minute)
	if err := db.Exec(`INSERT INTO interaction_records
		(id, user_id, character_id, conversation_id, channel, request_id, status, status_version, created_at, updated_at)
		VALUES ('newest', 'user-1', 'character-1', 'new-conv', 'web', 'new-req', 'received', 1, ?, ?)`, newCreated, newCreated).Error; err != nil {
		t.Fatal(err)
	}

	rows, err := NewAmitiaStateSourceQuery(db).QueryActiveInteractions(context.Background(), "user-1", "character-1")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.InteractionID == "newest" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("newest-created interaction was dropped by recovery query: %+v", rows)
	}
}

func TestQueryActiveToolsRejectsAmbiguousCrossTenantCorrelation(t *testing.T) {
	db := openStateSourceTestDB(t, "state-source-tool-tenant")
	if err := db.Exec(`CREATE TABLE tool_call_intents (
		id TEXT PRIMARY KEY,
		request_id TEXT,
		conversation_id TEXT,
		character_id TEXT,
		channel TEXT,
		tool_name TEXT,
		status TEXT,
		created_at TEXT,
		updated_at TEXT
	)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	for _, userID := range []string{"user-1", "user-2"} {
		if err := db.Exec(`INSERT INTO interaction_records
			(id, user_id, character_id, conversation_id, channel, request_id, status, status_version, created_at, updated_at)
			VALUES (?, ?, 'character-1', 'shared-conv', 'web', 'shared-req', 'processing', 2, ?, ?)`,
			"interaction-"+userID, userID, now, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	stamp := now.Format("2006-01-02 15:04:05")
	if err := db.Exec(`INSERT INTO tool_call_intents
		(id, request_id, conversation_id, character_id, channel, tool_name, status, created_at, updated_at)
		VALUES ('ambiguous-tool', 'shared-req', 'shared-conv', 'character-1', 'web', 'research', 'RUNNING', ?, ?)`, stamp, stamp).Error; err != nil {
		t.Fatal(err)
	}

	tools, err := NewAmitiaStateSourceQuery(db).QueryActiveTools(context.Background(), "user-1", "character-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tools["ambiguous-tool"]; ok {
		t.Fatalf("cross-tenant ambiguous tool was attached to user-1: %+v", tools)
	}
}

func TestQueryActiveToolsRestoresUnambiguousInteractionOwnership(t *testing.T) {
	db := openStateSourceTestDB(t, "state-source-tool-owner")
	if err := db.Exec(`CREATE TABLE tool_call_intents (
		id TEXT PRIMARY KEY,
		request_id TEXT,
		conversation_id TEXT,
		character_id TEXT,
		channel TEXT,
		tool_name TEXT,
		status TEXT,
		created_at TEXT,
		updated_at TEXT
	)`).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO interaction_records
		(id, user_id, character_id, conversation_id, channel, request_id, status, status_version, created_at, updated_at)
		VALUES ('interaction-1', 'user-1', 'character-1', 'conv-1', 'web', 'req-1', 'processing', 2, ?, ?)`, now, now).Error; err != nil {
		t.Fatal(err)
	}
	stamp := now.Format("2006-01-02 15:04:05")
	if err := db.Exec(`INSERT INTO tool_call_intents
		(id, request_id, conversation_id, character_id, channel, tool_name, status, created_at, updated_at)
		VALUES ('tool-1', 'req-1', 'conv-1', 'character-1', 'web', 'research', 'RUNNING', ?, ?)`, stamp, stamp).Error; err != nil {
		t.Fatal(err)
	}

	tools, err := NewAmitiaStateSourceQuery(db).QueryActiveTools(context.Background(), "user-1", "character-1")
	if err != nil {
		t.Fatal(err)
	}
	tool, ok := tools["tool-1"]
	if !ok || tool.InteractionID != "interaction-1" {
		t.Fatalf("tool ownership was not restored: %+v", tools)
	}
}

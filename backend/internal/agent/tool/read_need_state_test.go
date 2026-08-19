package tool

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupNeedTestDB(t *testing.T) (*gorm.DB, *sql.DB, func()) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "need_test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	schema := []string{
		`CREATE TABLE need_states (
			character_id TEXT NOT NULL,
			need_key TEXT NOT NULL,
			current_value REAL NOT NULL DEFAULT 0.5,
			baseline REAL NOT NULL DEFAULT 0.5,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (character_id, need_key)
		)`,
		`CREATE TABLE tool_call_intents (id TEXT PRIMARY KEY, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, args_json TEXT, idempotency_key TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE tool_call_results (id TEXT PRIMARY KEY, intent_id TEXT, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, status TEXT, content TEXT, error_code TEXT, visible_text TEXT, side_effects_json TEXT, external_operation_id TEXT, idempotency_key TEXT, audit_json TEXT, confidence REAL, force_voice INTEGER, created_at TEXT)`,
	}
	for _, stmt := range schema {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	oldDB := toolDB
	toolDB = sqlDB
	cleanup := func() {
		sqlDB.Close()
		toolDB = oldDB
	}
	return gormDB, sqlDB, cleanup
}

func TestReadNeedStateInit(t *testing.T) {
	for _, memTool := range GetMemoryTools() {
		if memTool.Function.Name == "read_need_state" {
			if memTool.Function.Description == "" {
				t.Fatal("read_need_state description should not be empty")
			}
			if len(memTool.Function.Parameters.Properties) == 0 {
				t.Fatal("read_need_state should have parameters")
			}
			_, hasChar := memTool.Function.Parameters.Properties["character_id"]
			_, hasHist := memTool.Function.Parameters.Properties["include_history"]
			if !hasChar {
				t.Fatal("read_need_state should have character_id parameter")
			}
			if !hasHist {
				t.Fatal("read_need_state should have include_history parameter")
			}
			return
		}
	}
	t.Fatal("read_need_state tool not registered in memory tools")
}

func TestReadNeedStateMissingCharacter(t *testing.T) {
	result := readNeedState(context.Background(), ToolExecutionContext{}, map[string]interface{}{})
	if result.Status != ToolStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.ErrorCode != "missing_character_scope" {
		t.Fatalf("expected missing_character_scope, got %s", result.ErrorCode)
	}
}

func TestReadNeedStateCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := readNeedState(ctx, ToolExecutionContext{}, map[string]interface{}{
		"character_id": "char-1",
	})
	if result.Status != ToolStatusCancelled {
		t.Fatalf("expected CANCELLED, got %s", result.Status)
	}
}

func TestReadNeedStateNoDB(t *testing.T) {
	oldDB := toolDB
	toolDB = nil
	defer func() { toolDB = oldDB }()

	result := readNeedState(context.Background(), ToolExecutionContext{}, map[string]interface{}{
		"character_id": "char-1",
	})
	if result.Status != ToolStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.ErrorCode != "database_not_initialized" {
		t.Fatalf("expected database_not_initialized, got %s", result.ErrorCode)
	}
}

func TestReadNeedStateEmptyResult(t *testing.T) {
	_, _, cleanup := setupNeedTestDB(t)
	defer cleanup()

	result := readNeedState(context.Background(), ToolExecutionContext{CharacterID: "char-empty"}, map[string]interface{}{})
	if result.Status != ToolStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	if result.Content != "当前角色暂无需求状态数据" {
		t.Fatalf("expected empty message, got %s", result.Content)
	}
}

func TestReadNeedStateWithData(t *testing.T) {
	_, _, cleanup := setupNeedTestDB(t)
	defer cleanup()

	_, err := toolDB.Exec(
		"INSERT INTO need_states (character_id, need_key, current_value, baseline, updated_at) VALUES (?, ?, ?, ?, ?)",
		"char-need", "companionship", 0.75, 0.50, "2026-07-01 12:00:00",
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = toolDB.Exec(
		"INSERT INTO need_states (character_id, need_key, current_value, baseline, updated_at) VALUES (?, ?, ?, ?, ?)",
		"char-need", "rest", 0.30, 0.60, "2026-07-01 12:00:00",
	)
	if err != nil {
		t.Fatal(err)
	}

	result := readNeedState(context.Background(), ToolExecutionContext{}, map[string]interface{}{
		"character_id": "char-need",
	})
	if result.Status != ToolStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	if !strContains(result.Content, "联系需求") || !strContains(result.Content, "休息需求") {
		t.Fatalf("result should contain need labels: %s", result.Content)
	}
	if !strContains(result.Content, "0.75") || !strContains(result.Content, "0.30") {
		t.Fatalf("result should contain need values: %s", result.Content)
	}
	if !strContains(result.Content, "↑") || !strContains(result.Content, "↓") {
		t.Fatalf("result should contain deviation arrows: %s", result.Content)
	}
}

func TestReadNeedStateWithHistoryFlag(t *testing.T) {
	_, _, cleanup := setupNeedTestDB(t)
	defer cleanup()

	result := readNeedState(context.Background(), ToolExecutionContext{CharacterID: "char-hist"}, map[string]interface{}{
		"character_id":    "char-hist",
		"include_history": true,
	})
	if result.Status != ToolStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
}

func TestReadNeedStateWithCharacterFromContext(t *testing.T) {
	_, _, cleanup := setupNeedTestDB(t)
	defer cleanup()

	_, err := toolDB.Exec(
		"INSERT INTO need_states (character_id, need_key, current_value, baseline, updated_at) VALUES (?, ?, ?, ?, ?)",
		"char-ctx", "certainty", 0.40, 0.70, "2026-07-01 12:00:00",
	)
	if err != nil {
		t.Fatal(err)
	}

	result := readNeedState(context.Background(), ToolExecutionContext{CharacterID: "char-ctx"}, map[string]interface{}{})
	if result.Status != ToolStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	if !strContains(result.Content, "确定性需求") {
		t.Fatalf("result should contain certainty: %s", result.Content)
	}
	if !strContains(result.Content, "0.40") || !strContains(result.Content, "0.70") {
		t.Fatalf("result should contain values: %s", result.Content)
	}
}

func strContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

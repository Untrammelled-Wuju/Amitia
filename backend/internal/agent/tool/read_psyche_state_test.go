package tool

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestReadPsycheStateInit(t *testing.T) {
	for _, memTool := range GetMemoryTools() {
		if memTool.Function.Name == "read_psyche_state" {
			if memTool.Function.Description == "" {
				t.Fatal("read_psyche_state description should not be empty")
			}
			if len(memTool.Function.Parameters.Properties) == 0 {
				t.Fatal("read_psyche_state should have parameters")
			}
			_, hasChar := memTool.Function.Parameters.Properties["character_id"]
			_, hasBelief := memTool.Function.Parameters.Properties["include_beliefs"]
			if !hasChar {
				t.Fatal("read_psyche_state should have character_id parameter")
			}
			if !hasBelief {
				t.Fatal("read_psyche_state should have include_beliefs parameter")
			}
			return
		}
	}
	t.Fatal("read_psyche_state tool not registered in memory tools")
}

func TestReadPsycheStateMissingCharacter(t *testing.T) {
	result := readPsycheState(context.Background(), ToolExecutionContext{}, map[string]interface{}{})
	if result.Status != ToolStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.ErrorCode != "missing_character_scope" {
		t.Fatalf("expected missing_character_scope, got %s", result.ErrorCode)
	}
}

func TestReadPsycheStateCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := readPsycheState(ctx, ToolExecutionContext{}, map[string]interface{}{
		"character_id": "char-1",
	})
	if result.Status != ToolStatusCancelled {
		t.Fatalf("expected CANCELLED, got %s", result.Status)
	}
}

func TestReadPsycheStateNoDB(t *testing.T) {
	oldDB := toolDB
	toolDB = nil
	defer func() { toolDB = oldDB }()

	result := readPsycheState(context.Background(), ToolExecutionContext{}, map[string]interface{}{
		"character_id": "char-1",
	})
	if result.Status != ToolStatusFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	if result.ErrorCode != "database_not_initialized" {
		t.Fatalf("expected database_not_initialized, got %s", result.ErrorCode)
	}
}

func setupPsycheTestDB(t *testing.T) (*gorm.DB, *sql.DB, func()) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "psyche_test.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	schema := []string{
		`CREATE TABLE psyche_states (
			character_id TEXT NOT NULL PRIMARY KEY,
			version TEXT NOT NULL DEFAULT '',
			state_version INTEGER NOT NULL DEFAULT 0,
			emotion TEXT NOT NULL DEFAULT '{}',
			mood TEXT NOT NULL DEFAULT '{}',
			stress REAL NOT NULL DEFAULT 0,
			energy REAL NOT NULL DEFAULT 0.7,
			created_at TEXT NOT NULL DEFAULT (datetime('now','localtime')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now','localtime'))
		)`,
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

func TestReadPsycheStateEmptyResult(t *testing.T) {
	_, _, cleanup := setupPsycheTestDB(t)
	defer cleanup()

	result := readPsycheState(context.Background(), ToolExecutionContext{CharacterID: "char-empty"}, map[string]interface{}{})
	if result.Status != ToolStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	if result.Content == "" {
		t.Fatal("expected non-empty content for empty psyche state")
	}
}

func TestReadPsycheStateWithData(t *testing.T) {
	_, _, cleanup := setupPsycheTestDB(t)
	defer cleanup()

	_, err := toolDB.Exec(
		"INSERT INTO psyche_states (character_id, version, state_version, emotion, mood, stress, energy, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		"char-spirit", "v1", 5,
		`{"valence":0.72,"arousal":0.65,"dominance":0.58}`,
		`{"moodValence":0.68,"moodArousal":0.55}`,
		0.35, 0.62, "2026-07-05 14:00:00",
	)
	if err != nil {
		t.Fatal(err)
	}

	result := readPsycheState(context.Background(), ToolExecutionContext{CharacterID: "char-spirit"}, map[string]interface{}{})
	if result.Status != ToolStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	if !strContains(result.Content, "效价: 0.72") {
		t.Fatalf("result should contain valence 0.72: %s", result.Content)
	}
	if !strContains(result.Content, "唤醒度: 0.65") {
		t.Fatalf("result should contain arousal 0.65: %s", result.Content)
	}
	if !strContains(result.Content, "支配感: 0.58") {
		t.Fatalf("result should contain dominance 0.58: %s", result.Content)
	}
	if !strContains(result.Content, "心境效价: 0.68") {
		t.Fatalf("result should contain mood valence 0.68: %s", result.Content)
	}
	if !strContains(result.Content, "心境唤醒度: 0.55") {
		t.Fatalf("result should contain mood arousal 0.55: %s", result.Content)
	}
	if !strContains(result.Content, "压力水平: 0.35") {
		t.Fatalf("result should contain stress 0.35: %s", result.Content)
	}
	if !strContains(result.Content, "精力水平: 0.62") {
		t.Fatalf("result should contain energy 0.62: %s", result.Content)
	}
	if result.Audit == nil {
		t.Fatal("expected audit data")
	}
	raw, ok := result.Audit["raw"].(string)
	if !ok || raw == "" {
		t.Fatal("expected raw JSON in audit")
	}
	if !strContains(raw, `"valence":0.72`) {
		t.Fatalf("raw JSON should contain valence: %s", raw)
	}
	if !strContains(raw, `"moodValence":0.68`) {
		t.Fatalf("raw JSON should contain moodValence: %s", raw)
	}
}

func TestReadPsycheStateWithBeliefs(t *testing.T) {
	_, _, cleanup := setupPsycheTestDB(t)
	defer cleanup()

	_, err := toolDB.Exec(
		"INSERT INTO psyche_states (character_id, version, state_version, emotion, mood, stress, energy, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		"char-belief", "v1", 3,
		`{"valence":0.40,"arousal":0.30,"dominance":0.45}`,
		`{"moodValence":0.42,"moodArousal":0.33}`,
		0.80, 0.25, "2026-07-05 08:00:00",
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := toolDB.Exec("CREATE TABLE resolved_beliefs (character_id TEXT NOT NULL, key TEXT NOT NULL, value TEXT NOT NULL, confidence REAL NOT NULL DEFAULT 0, created_at TEXT NOT NULL, PRIMARY KEY (character_id, key))"); err != nil {
		t.Fatal(err)
	}
	if _, err := toolDB.Exec("INSERT INTO resolved_beliefs (character_id, key, value, confidence) VALUES (?, ?, ?, ?)", "char-belief", "self-worth", "觉得自己不够好", 0.85); err != nil {
		t.Fatal(err)
	}

	result := readPsycheState(context.Background(), ToolExecutionContext{CharacterID: "char-belief"}, map[string]interface{}{
		"include_beliefs": true,
	})
	if result.Status != ToolStatusSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	if !strContains(result.Content, "关键信念") {
		t.Fatalf("result should contain beliefs section: %s", result.Content)
	}
	if !strContains(result.Content, "自己不够好") {
		t.Fatalf("result should contain belief value: %s", result.Content)
	}
	if !strContains(result.Content, "self-worth") {
		t.Fatalf("result should contain belief key: %s", result.Content)
	}
}

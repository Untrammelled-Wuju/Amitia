package tool

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	memorysvc "github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func setupSummaryTestDB(t *testing.T) func() {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "summary.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec("CREATE TABLE IF NOT EXISTS memories (id TEXT PRIMARY KEY, character_id TEXT, key TEXT, value TEXT, memory_type TEXT, importance INTEGER DEFAULT 0, confidence INTEGER DEFAULT 50, scope TEXT, last_used_at TEXT, created_at TEXT, updated_at TEXT)"); err != nil {
		t.Fatal(err)
	}
	schema := []string{
		"CREATE TABLE IF NOT EXISTS memories (id TEXT PRIMARY KEY, character_id TEXT, key TEXT, value TEXT, memory_type TEXT, importance INTEGER DEFAULT 0, confidence INTEGER DEFAULT 50, scope TEXT, last_used_at TEXT, created_at TEXT, updated_at TEXT)",
		"CREATE TABLE IF NOT EXISTS tool_call_intents (id TEXT PRIMARY KEY, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, args_json TEXT, idempotency_key TEXT, status TEXT, created_at TEXT, updated_at TEXT)",
		"CREATE TABLE IF NOT EXISTS tool_call_results (id TEXT PRIMARY KEY, intent_id TEXT, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, status TEXT, content TEXT, error_code TEXT, visible_text TEXT, side_effects_json TEXT, external_operation_id TEXT, idempotency_key TEXT, audit_json TEXT, confidence REAL, force_voice INTEGER, created_at TEXT)",
	}
	for _, stmt := range schema {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	SetDB(sqlDB)
	ctx := app.NewAppContext(gormDB, nil)
	SetMemoryService(memorysvc.NewService(memorysvc.NewRepository(ctx), ctx, nil))
	seedMemories(t, sqlDB)
	cleanup := func() {
		sqlDB.Close()
		SetDB(nil)
		SetMemoryService(nil)
	}
	return cleanup
}

func seedMemories(t *testing.T, db *sql.DB) {
	t.Helper()
	entries := []struct {
		id, charID, key, value, memType, scope, lastUsed string
		importance, confidence                           int
	}{
		{"m1", "char-1", "颜色", "最喜欢的颜色是蓝色", "personal_info", "character", "2026-06-30", 9, 90},
		{"m2", "char-1", "音乐", "喜欢古典音乐和爵士", "hobby", "character", "2026-06-28", 7, 80},
		{"m3", "char-1", "食物", "喜欢吃辣", "preference", "character", "2026-06-25", 6, 70},
		{"m4", "char-2", "颜色", "最喜欢绿色", "personal_info", "character", "2026-06-29", 8, 85},
		{"m5", "char-1", "运动", "偶尔跑步但不常去", "hobby", "character", "2026-06-20", 4, 50},
		{"m6", "char-1", "工作", "是一名软件工程师", "personal_info", "character", "2026-06-15", 9, 90},
		{"m7", "char-1", "旅行计划", "计划去日本旅行", "plan", "character", "", 6, 60},
	}
	for _, e := range entries {
		_, err := db.Exec(
			"INSERT INTO memories (id, character_id, key, value, memory_type, importance, confidence, scope, last_used_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))",
			e.id, e.charID, e.key, e.value, e.memType, e.importance, e.confidence, e.scope, e.lastUsed,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestSummarizeMemoriesReturnsResults(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-1",
		CharacterID:    "char-1",
		Channel:        "web",
		RequestID:      "req-summary-1",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"颜色"}`)
	if !ok {
		t.Fatalf("summarize_memories failed: %s", result.Content)
	}
	if !strings.Contains(result.Content, "颜色") {
		t.Fatalf("expected topic in result, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "蓝色") {
		t.Fatalf("expected memory value in result, got: %s", result.Content)
	}
}

func TestSummarizeMemoriesHonorsScopeIsolation(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-2",
		CharacterID:    "char-2",
		Channel:        "wechat",
		RequestID:      "req-summary-2",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"颜色"}`)
	if !ok {
		t.Fatalf("summarize_memories failed: %s", result.Content)
	}
	if strings.Contains(result.Content, "蓝色") {
		t.Fatalf("char-2 should not see char-1 memory, got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "绿色") {
		t.Fatalf("char-2 should see own memory, got: %s", result.Content)
	}
}

func TestSummarizeMemoriesEmptyTopicReturnsError(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-3",
		CharacterID:    "char-1",
		Channel:        "web",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":""}`)
	if !ok {
		if !strings.Contains(result.Content, "required") {
			t.Fatalf("expected error about required, got: %s", result.Content)
		}
	}
}

func TestSummarizeMemoriesRespectsLimit(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-4",
		CharacterID:    "char-1",
		Channel:        "web",
		RequestID:      "req-summary-4",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"a","limit":2}`)
	if !ok {
		t.Fatalf("summarize_memories failed: %s", result.Content)
	}
	if !strings.Contains(result.Content, "找到") {
		t.Fatalf("expected summary, got: %s", result.Content)
	}
}

func TestSummarizeMemoriesNoResultsReturnsEmptyMessage(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-5",
		CharacterID:    "char-1",
		Channel:        "web",
		RequestID:      "req-summary-5",
	}

	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"不存在的主题xyz"}`)
	if !ok {
		t.Fatalf("summarize_memories should succeed: %s", result.Content)
	}
	if !strings.Contains(result.Content, "未找到") {
		t.Fatalf("expected no results message, got: %s", result.Content)
	}
}

func TestSummarizeMemoriesCancelledContext(t *testing.T) {
	cleanup := setupSummaryTestDB(t)
	t.Cleanup(cleanup)

	callCtx, cancel := context.WithCancel(context.Background())
	cancel()

	execCtx := ToolExecutionContext{
		ConversationID: "conv-summary-6",
		CharacterID:    "char-1",
		Channel:        "web",
		RequestID:      "req-summary-6",
	}

	result, ok := ExecuteMemoryWithContextAndCancel(callCtx, execCtx, "summarize_memories", `{"topic":"颜色"}`)
	if !ok {
		t.Fatal("cancelled tool should return true")
	}
	if result.Status != ToolStatusCancelled {
		t.Fatalf("expected cancelled status, got: %s", result.Status)
	}
}

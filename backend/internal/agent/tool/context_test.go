package tool

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/internal/spaceidentity"
	"gorm.io/gorm"
)

func setupToolTestDB(t *testing.T) (*gorm.DB, *gorm.DB, func()) {
	t.Helper()
	gormDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := spaceidentity.InitializeDefault(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	canonical := requestidentity.CanonicalSpaceID()
	schema := []string{
		`CREATE TABLE memories (id TEXT PRIMARY KEY, character_id TEXT, key TEXT, value TEXT)`,
		`CREATE TABLE memory_events (id TEXT PRIMARY KEY, memory_id TEXT, event_type TEXT, key TEXT, value TEXT, memory_type TEXT, importance INTEGER, confidence INTEGER DEFAULT 0, source TEXT, character_id TEXT, created_at TEXT, version INTEGER DEFAULT 0, operation_id TEXT DEFAULT '', snapshot_hash TEXT DEFAULT '', event_reason TEXT DEFAULT '')`,
		`CREATE TABLE schedules (id TEXT PRIMARY KEY, title TEXT, description TEXT, due_time TEXT, repeat_mode TEXT, channel TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE user_profiles (id TEXT PRIMARY KEY, space_id TEXT NOT NULL DEFAULT 'default', character_id TEXT NOT NULL DEFAULT '', category TEXT NOT NULL, attribute_name TEXT NOT NULL, attribute_value TEXT NOT NULL, confidence INTEGER DEFAULT 50, source TEXT NOT NULL DEFAULT '', source_conv_id TEXT DEFAULT '', verified_at TEXT DEFAULT '', created_at TEXT, updated_at TEXT, source_memory_id TEXT DEFAULT '', projection_status TEXT DEFAULT 'active')`,
		`CREATE UNIQUE INDEX idx_user_profiles_uid_cat_attr ON user_profiles(space_id, character_id, category, attribute_name)`,
		`CREATE TABLE episodic_memories (id TEXT PRIMARY KEY, space_id TEXT NOT NULL DEFAULT 'default', character_id TEXT NOT NULL DEFAULT '', scene_type TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL, context_before TEXT DEFAULT '', context_after TEXT DEFAULT '', trigger_keywords TEXT DEFAULT '', sentiment_score INTEGER DEFAULT 0, message_id_start TEXT DEFAULT '', message_id_end TEXT DEFAULT '', message_time_start TEXT DEFAULT '', message_time_end TEXT DEFAULT '', source_conv_id TEXT DEFAULT '', created_at TEXT, updated_at TEXT, retention_level INTEGER DEFAULT 4, memory_strength REAL DEFAULT 0.5, strength_updated_at TEXT, last_reinforced_at TEXT, reinforce_count INTEGER DEFAULT 0, decay_state TEXT DEFAULT 'active', archived_at TEXT)`,
		`CREATE TABLE tool_call_intents (id TEXT PRIMARY KEY, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, args_json TEXT, idempotency_key TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE tool_call_results (id TEXT PRIMARY KEY, intent_id TEXT, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, status TEXT, content TEXT, error_code TEXT, visible_text TEXT, side_effects_json TEXT, external_operation_id TEXT, idempotency_key TEXT, audit_json TEXT, confidence REAL, force_voice INTEGER, created_at TEXT)`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, space_id TEXT NOT NULL DEFAULT '', character_id TEXT NOT NULL DEFAULT '', deleted_at DATETIME)`,
	}
	for _, stmt := range schema {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	for _, conv := range []struct {
		id          string
		characterID string
	}{
		{"conv-char-a", "char-a"},
		{"conv-char-b", "char-b"},
		{"conv-profile-a", "char-profile-a"},
		{"conv-profile-b", "char-profile-b"},
		{"conv-memory", "char-memory"},
		{"conv-invalid", "char-invalid"},
		{"conv-cancel", "char-cancel"},
		{"conv-audit", "char-audit"},
		{"conv-force-web", "char-force"},
		{"conv-force-wechat", "char-force"},
		{"conv-force-qq", "char-force"},
		{"conv-summary-1", "char-1"},
		{"conv-summary-2", "char-2"},
		{"conv-summary-3", "char-1"},
		{"conv-summary-4", "char-1"},
		{"conv-summary-5", "char-1"},
		{"conv-summary-6", "char-1"},
	} {
		if _, err := sqlDB.Exec(`INSERT INTO conversations (id, space_id, character_id) VALUES (?, ?, ?)`, conv.id, canonical, conv.characterID); err != nil {
			t.Fatal(err)
		}
	}
	SetDB(sqlDB)
	newToolMemoryTestRuntime(t, canonical)
	cleanup := func() {
		sqlDB.Close()
		SetDB(nil)
	}
	return gormDB, gormDB, cleanup
}

func TestConcurrentSchedulesKeepChannelContext(t *testing.T) {
	gormDB, _, cleanup := setupToolTestDB(t)
	t.Cleanup(cleanup)
	var received sync.Map
	SetScheduleCreator(func(_ context.Context, execution ToolExecutionContext, _ map[string]interface{}) (string, error) {
		received.Store(execution.Channel, execution.CharacterID)
		return "reminder-" + execution.CharacterID, nil
	})
	t.Cleanup(func() { SetScheduleCreator(nil) })
	var wg sync.WaitGroup
	for _, item := range []struct {
		characterID string
		key         string
		value       string
		channel     string
	}{
		{"char-a", "颜色", "蓝色", "wechat"},
		{"char-b", "颜色", "绿色", "qq"},
	} {
		item := item
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := ToolExecutionContext{ConversationID: "conv-" + item.characterID, CharacterID: item.characterID, Channel: item.channel}
			if result, ok := ExecuteWithContext(ctx, "create_schedule", `{"title":"提醒","due_time":"2026-07-01 18:00"}`); !ok || !strings.HasPrefix(result.Content, "OK") {
				t.Errorf("create_schedule failed: ok=%v result=%s", ok, result.Content)
			}
		}()
	}
	wg.Wait()
	db, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	var wechatCount, qqCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM schedules WHERE channel = 'wechat'").Scan(&wechatCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM schedules WHERE channel = 'qq'").Scan(&qqCount); err != nil {
		t.Fatal(err)
	}
	wechatRole, hasWechat := received.Load("wechat")
	qqRole, hasQQ := received.Load("qq")
	if wechatCount != 0 || qqCount != 0 || !hasWechat || !hasQQ || wechatRole != "char-a" || qqRole != "char-b" {
		t.Fatalf("schedule channel context not preserved: wechat=%d qq=%d", wechatCount, qqCount)
	}
}

func TestWriteToolsRejectMissingScopeWithoutSideEffects(t *testing.T) {
	gormDB, _, cleanup := setupToolTestDB(t)
	t.Cleanup(cleanup)
	tests := []struct {
		name      string
		toolName  string
		args      string
		execCtx   ToolExecutionContext
		wantError string
		tableName string
	}{
		{
			name:      "save_memory_missing_character",
			toolName:  "save_memory",
			args:      `{"key":"颜色","value":"蓝色"}`,
			execCtx:   ToolExecutionContext{ConversationID: "conv-scope", Channel: "wechat"},
			wantError: "missing_character_scope",
			tableName: "memories",
		},
		{
			name:      "save_profile_missing_conversation",
			toolName:  "save_profile",
			args:      `{"category":"preference","attribute_name":"颜色","attribute_value":"蓝色"}`,
			execCtx:   ToolExecutionContext{CharacterID: "char-scope", Channel: "wechat"},
			wantError: "missing_conversation_scope",
			tableName: "user_profiles",
		},
		{
			name:      "save_episodic_missing_character",
			toolName:  "save_episodic_memory",
			args:      `{"scene_type":"insight","title":"一次坦白","content":"用户分享了重要想法"}`,
			execCtx:   ToolExecutionContext{ConversationID: "conv-scope", Channel: "wechat"},
			wantError: "missing_character_scope",
			tableName: "episodic_memories",
		},
		{
			name:      "create_schedule_missing_conversation",
			toolName:  "create_schedule",
			args:      `{"title":"提醒","due_time":"2026-07-01 18:00"}`,
			execCtx:   ToolExecutionContext{CharacterID: "char-scope", Channel: "wechat"},
			wantError: "missing_conversation_scope",
			tableName: "schedules",
		},
	}
	db, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ExecuteWithContextAndCancel(context.Background(), tt.execCtx, tt.toolName, tt.args)
			if !ok {
				t.Fatalf("tool not found: %s", tt.toolName)
			}
			if result.Status != ToolStatusFailed || result.ErrorCode != tt.wantError {
				t.Fatalf("unexpected result: %#v", result)
			}
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM " + tt.tableName).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("tool wrote side effect without scope: table=%s count=%d", tt.tableName, count)
			}
		})
	}
}

func TestCancelledToolDoesNotWriteSideEffect(t *testing.T) {
	gormDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		SetDB(nil)
	})
	SetDB(db)
	schema := []string{
		`CREATE TABLE schedules (id TEXT PRIMARY KEY, title TEXT, description TEXT, due_time TEXT, repeat_mode TEXT, channel TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE tool_call_intents (id TEXT PRIMARY KEY, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, args_json TEXT, idempotency_key TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE tool_call_results (id TEXT PRIMARY KEY, intent_id TEXT, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, status TEXT, content TEXT, error_code TEXT, visible_text TEXT, side_effects_json TEXT, external_operation_id TEXT, idempotency_key TEXT, audit_json TEXT, confidence REAL, force_voice INTEGER, created_at TEXT)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	callCtx, cancel := context.WithCancel(context.Background())
	cancel()
	execCtx := ToolExecutionContext{ConversationID: "conv-cancel", CharacterID: "char-cancel", Channel: "web", RequestID: "req-cancel", ToolCallID: "tool-cancel"}
	result, ok := ExecuteWithContextAndCancel(callCtx, execCtx, "create_schedule", `{"title":"提醒","due_time":"2026-07-01 18:00"}`)
	if !ok {
		t.Fatal("tool execute failed")
	}
	if result.Status != ToolStatusCancelled {
		t.Fatalf("expected cancelled, got %#v", result)
	}
	var schedules int
	if err := db.QueryRow("SELECT COUNT(*) FROM schedules").Scan(&schedules); err != nil {
		t.Fatal(err)
	}
	if schedules != 0 {
		t.Fatalf("cancelled tool still wrote side effect: %d", schedules)
	}
}

func TestExecuteWithContextInheritsCancelledContext(t *testing.T) {
	callCtx, cancel := context.WithCancel(context.Background())
	cancel()
	execCtx := ToolExecutionContext{Context: callCtx, ConversationID: "conv-cancel", CharacterID: "char-cancel", Channel: "web"}
	result, ok := ExecuteWithContext(execCtx, "create_schedule", `{"title":"提醒","due_time":"2026-07-01 18:00"}`)
	if !ok {
		t.Fatal("tool execute failed")
	}
	if result.Status != ToolStatusCancelled {
		t.Fatalf("expected cancelled, got %#v", result)
	}
}

func TestExecuteMemoryWithContextInheritsCancelledContext(t *testing.T) {
	callCtx, cancel := context.WithCancel(context.Background())
	cancel()
	execCtx := ToolExecutionContext{Context: callCtx, ConversationID: "conv-cancel", CharacterID: "char-cancel", Channel: "web"}
	result, ok := ExecuteMemoryWithContext(execCtx, "summarize_memories", `{"topic":"颜色"}`)
	if !ok {
		t.Fatal("memory tool execute failed")
	}
	if result.Status != ToolStatusCancelled {
		t.Fatalf("expected cancelled, got %#v", result)
	}
}

func TestToolResultAuditTablesAreWritten(t *testing.T) {
	SetScheduleCreator(func(_ context.Context, execution ToolExecutionContext, _ map[string]interface{}) (string, error) {
		return "reminder-" + execution.RequestID, nil
	})
	t.Cleanup(func() { SetScheduleCreator(nil) })
	gormDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		SetDB(nil)
	})
	SetDB(db)
	schema := []string{
		`CREATE TABLE schedules (id TEXT PRIMARY KEY, title TEXT, description TEXT, due_time TEXT, repeat_mode TEXT, channel TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE tool_call_intents (id TEXT PRIMARY KEY, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, args_json TEXT, idempotency_key TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE tool_call_results (id TEXT PRIMARY KEY, intent_id TEXT, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, status TEXT, content TEXT, error_code TEXT, visible_text TEXT, side_effects_json TEXT, external_operation_id TEXT, idempotency_key TEXT, audit_json TEXT, confidence REAL, force_voice INTEGER, created_at TEXT)`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	execCtx := ToolExecutionContext{ConversationID: "conv-audit", CharacterID: "char-audit", Channel: "wechat", RequestID: "req-audit", ToolCallID: "tool-audit"}
	result, ok := ExecuteWithContextAndCancel(context.Background(), execCtx, "create_schedule", `{"title":"提醒","due_time":"2026-07-01 18:00"}`)
	if !ok || result.Status != ToolStatusSuccess {
		t.Fatalf("unexpected result: ok=%v %#v", ok, result)
	}
	var intentStatus, resultStatus, externalOperationID string
	if err := db.QueryRow("SELECT status FROM tool_call_intents WHERE request_id = 'req-audit' LIMIT 1").Scan(&intentStatus); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT status, external_operation_id FROM tool_call_results WHERE request_id = 'req-audit' LIMIT 1").Scan(&resultStatus, &externalOperationID); err != nil {
		t.Fatal(err)
	}
	if intentStatus != string(ToolStatusSuccess) || resultStatus != string(ToolStatusSuccess) || externalOperationID == "" {
		t.Fatalf("unexpected audit rows: intent=%s result=%s externalOperationID=%s", intentStatus, resultStatus, externalOperationID)
	}
}

func TestConcurrentForceVoiceResultStaysInRequest(t *testing.T) {
	channels := []string{"web", "wechat", "qq"}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := ToolExecutionContext{ConversationID: "conv-force-" + channels[i%len(channels)], CharacterID: "char-force", Channel: channels[i%len(channels)]}
			voiceResult, ok := ExecuteWithContext(ctx, "force_voice_reply", `{}`)
			if !ok {
				t.Errorf("force_voice_reply not found")
				return
			}
			if !voiceResult.ForceVoice || voiceResult.Content != "OK" {
				t.Errorf("unexpected force voice result: %#v", voiceResult)
			}
			timeResult, ok := ExecuteWithContext(ctx, "get_current_time", `{}`)
			if !ok {
				t.Errorf("get_current_time not found")
				return
			}
			if timeResult.ForceVoice {
				t.Errorf("force voice leaked into time result: %#v", timeResult)
			}
		}()
	}
	wg.Wait()
}

package tool

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	memorysvc "github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/pkg/app"
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
	if err := gormDB.AutoMigrate(&memorysvc.Memory{}); err != nil {
		t.Fatal(err)
	}
	schema := []string{
		`CREATE TABLE memory_events (id TEXT PRIMARY KEY, memory_id TEXT, event_type TEXT, key TEXT, value TEXT, memory_type TEXT, importance INTEGER, source TEXT, character_id TEXT, created_at TEXT)`,
		`CREATE TABLE schedules (id TEXT PRIMARY KEY, title TEXT, description TEXT, due_time TEXT, repeat_mode TEXT, channel TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE user_profiles (id TEXT PRIMARY KEY, user_id TEXT NOT NULL DEFAULT 'default', category TEXT NOT NULL, attribute_name TEXT NOT NULL, attribute_value TEXT NOT NULL, confidence INTEGER DEFAULT 50, source_conv_id TEXT DEFAULT '', verified_at TEXT DEFAULT '', created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now')))`,
		`CREATE UNIQUE INDEX idx_user_profiles_uid_cat_attr ON user_profiles(user_id, category, attribute_name)`,
		`CREATE TABLE episodic_memories (id TEXT PRIMARY KEY, user_id TEXT NOT NULL DEFAULT 'default', scene_type TEXT NOT NULL, title TEXT NOT NULL, content TEXT NOT NULL, context_before TEXT DEFAULT '', context_after TEXT DEFAULT '', trigger_keywords TEXT DEFAULT '', sentiment_score INTEGER DEFAULT 0, message_id_start TEXT DEFAULT '', message_id_end TEXT DEFAULT '', source_conv_id TEXT DEFAULT '', created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now')))`,
		`CREATE TABLE tool_call_intents (id TEXT PRIMARY KEY, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, args_json TEXT, idempotency_key TEXT, status TEXT, created_at TEXT, updated_at TEXT)`,
		`CREATE TABLE tool_call_results (id TEXT PRIMARY KEY, intent_id TEXT, request_id TEXT, conversation_id TEXT, character_id TEXT, channel TEXT, tool_call_id TEXT, tool_name TEXT, status TEXT, content TEXT, error_code TEXT, visible_text TEXT, side_effects_json TEXT, external_operation_id TEXT, idempotency_key TEXT, audit_json TEXT, confidence REAL, force_voice INTEGER, created_at TEXT)`,
	}
	for _, stmt := range schema {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	SetDB(sqlDB)
	ctx := app.NewAppContext(gormDB, nil)
	SetMemoryService(memorysvc.NewService(memorysvc.NewRepository(ctx), ctx, nil))
	cleanup := func() {
		sqlDB.Close()
		SetDB(nil)
		SetMemoryService(nil)
	}
	return gormDB, gormDB, cleanup
}

func TestConcurrentToolExecutionKeepsRoleContext(t *testing.T) {
	gormDB, _, cleanup := setupToolTestDB(t)
	t.Cleanup(cleanup)
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
			if result, ok := ExecuteWithContext(ctx, "save_memory", `{"key":"`+item.key+`","value":"`+item.value+`"}`); !ok || !strings.HasPrefix(result.Content, "OK") {
				t.Errorf("save_memory failed: ok=%v result=%s", ok, result.Content)
			}
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
	rows, err := db.Query("SELECT character_id, value FROM memories ORDER BY character_id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var characterID, value string
		if err := rows.Scan(&characterID, &value); err != nil {
			t.Fatal(err)
		}
		got[characterID] = value
	}
	if got["char-a"] != "蓝色" || got["char-b"] != "绿色" {
		t.Fatalf("memory context crossed roles: %#v", got)
	}
	var wechatCount, qqCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM schedules WHERE channel = 'wechat'").Scan(&wechatCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM schedules WHERE channel = 'qq'").Scan(&qqCount); err != nil {
		t.Fatal(err)
	}
	if wechatCount != 1 || qqCount != 1 {
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

func TestProfileAndEpisodicToolsStayInCharacterScope(t *testing.T) {
	gormDB, _, cleanup := setupToolTestDB(t)
	t.Cleanup(cleanup)
	for _, item := range []struct {
		characterID string
		convID      string
		value       string
	}{
		{"char-profile-a", "conv-profile-a", "蓝色"},
		{"char-profile-b", "conv-profile-b", "绿色"},
	} {
		ctx := ToolExecutionContext{ConversationID: item.convID, CharacterID: item.characterID, Channel: "wechat"}
		profileResult, ok := ExecuteWithContextAndCancel(context.Background(), ctx, "save_profile", `{"category":"preference","attribute_name":"颜色","attribute_value":"`+item.value+`"}`)
		if !ok || profileResult.Status != ToolStatusSuccess {
			t.Fatalf("save_profile failed: ok=%v result=%#v", ok, profileResult)
		}
		episodicResult, ok := ExecuteWithContextAndCancel(context.Background(), ctx, "save_episodic_memory", `{"scene_type":"insight","title":"颜色偏好","content":"用户说明了颜色偏好"}`)
		if !ok || episodicResult.Status != ToolStatusSuccess {
			t.Fatalf("save_episodic_memory failed: ok=%v result=%#v", ok, episodicResult)
		}
	}
	db, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query("SELECT user_id, attribute_value, source_conv_id FROM user_profiles ORDER BY user_id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	gotProfiles := map[string]string{}
	gotProfileConvs := map[string]string{}
	for rows.Next() {
		var userID, value, convID string
		if err := rows.Scan(&userID, &value, &convID); err != nil {
			t.Fatal(err)
		}
		gotProfiles[userID] = value
		gotProfileConvs[userID] = convID
	}
	if gotProfiles["char-profile-a"] != "蓝色" || gotProfiles["char-profile-b"] != "绿色" {
		t.Fatalf("profile context crossed roles: %#v", gotProfiles)
	}
	if gotProfileConvs["char-profile-a"] != "conv-profile-a" || gotProfileConvs["char-profile-b"] != "conv-profile-b" {
		t.Fatalf("profile conversation context crossed roles: %#v", gotProfileConvs)
	}
	var episodicA, episodicB int
	if err := db.QueryRow("SELECT COUNT(*) FROM episodic_memories WHERE user_id = ? AND source_conv_id = ?", "char-profile-a", "conv-profile-a").Scan(&episodicA); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM episodic_memories WHERE user_id = ? AND source_conv_id = ?", "char-profile-b", "conv-profile-b").Scan(&episodicB); err != nil {
		t.Fatal(err)
	}
	if episodicA != 1 || episodicB != 1 {
		t.Fatalf("episodic context crossed roles: a=%d b=%d", episodicA, episodicB)
	}
}

func TestSaveMemoryPersistsExpiresAtAndEntityID(t *testing.T) {
	gormDB, _, cleanup := setupToolTestDB(t)
	t.Cleanup(cleanup)
	entityID := "550e8400-e29b-41d4-a716-446655440000"
	execCtx := ToolExecutionContext{
		ConversationID: "conv-memory",
		CharacterID:    "char-memory",
		Channel:        "wechat",
		RequestID:      "req-memory",
		ToolCallID:     "tool-memory",
	}
	result, ok := ExecuteWithContextAndCancel(context.Background(), execCtx, "save_memory", `{"key":"计划","value":"下周出差","expiresAt":"2026-12-31","entityId":"`+entityID+`"}`)
	if !ok || result.Status != ToolStatusSuccess {
		t.Fatalf("unexpected result: ok=%v %#v", ok, result)
	}
	db, err := gormDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	var expiresAt, savedEntityID, sourceConvID string
	if err := db.QueryRow("SELECT expires_at, entity_id, source_conv_id FROM memories WHERE id = ?", result.ExternalOperationID).Scan(&expiresAt, &savedEntityID, &sourceConvID); err != nil {
		t.Fatal(err)
	}
	if expiresAt != "2026-12-31" || savedEntityID != entityID || sourceConvID != "conv-memory" {
		t.Fatalf("unexpected memory row: expiresAt=%s entityId=%s sourceConvID=%s", expiresAt, savedEntityID, sourceConvID)
	}
}

func TestSaveMemoryRejectsInvalidExpiresAt(t *testing.T) {
	_, _, cleanup := setupToolTestDB(t)
	t.Cleanup(cleanup)
	execCtx := ToolExecutionContext{
		ConversationID: "conv-invalid",
		CharacterID:    "char-invalid",
		Channel:        "wechat",
		RequestID:      "req-invalid",
		ToolCallID:     "tool-invalid",
	}
	result, ok := ExecuteWithContextAndCancel(context.Background(), execCtx, "save_memory", `{"key":"计划","value":"下周出差","expiresAt":"2026/12/31"}`)
	if !ok {
		t.Fatal("tool execute failed")
	}
	if result.Status != ToolStatusFailed || result.ErrorCode != "invalid_expires_at" {
		t.Fatalf("unexpected result: %#v", result)
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

func TestToolResultAuditTablesAreWritten(t *testing.T) {
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

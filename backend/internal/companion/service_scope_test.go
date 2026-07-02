package companion

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupCompanionScopeService(t *testing.T) *service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
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
	statements := []string{
		"CREATE TABLE conversations (id TEXT PRIMARY KEY, character_id TEXT DEFAULT '', title TEXT DEFAULT '', channel TEXT DEFAULT 'web', source TEXT DEFAULT 'manual', peer_id TEXT DEFAULT '', message_count INTEGER DEFAULT 0, created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, role TEXT NOT NULL, content TEXT NOT NULL, msg_type TEXT DEFAULT 'text', source TEXT DEFAULT 'manual', safety_level TEXT DEFAULT 'normal', status TEXT DEFAULT 'sent', include_in_context INTEGER DEFAULT 1, created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE active_message_settings (id INTEGER PRIMARY KEY AUTOINCREMENT, character_id TEXT DEFAULT '', enabled INTEGER DEFAULT 1, active_level INTEGER DEFAULT 50, min_interval INTEGER DEFAULT 60, quiet_start TEXT DEFAULT '23:00', quiet_end TEXT DEFAULT '07:00', max_per_day INTEGER DEFAULT 6, max_daily_calls INTEGER DEFAULT 10, channel TEXT DEFAULT 'all', created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE active_message_task (id INTEGER PRIMARY KEY AUTOINCREMENT, character_id TEXT DEFAULT '', task_type TEXT DEFAULT '', due_time TEXT, prompt TEXT DEFAULT '', status TEXT DEFAULT 'PENDING', reason TEXT DEFAULT '', retry_count INTEGER DEFAULT 0, max_retry INTEGER DEFAULT 3, last_error TEXT DEFAULT '', sent_at TEXT, canceled_at TEXT, cancel_reason TEXT DEFAULT '', source TEXT DEFAULT 'schedule_based', lock_until TEXT, created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE proactive_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, rule_id INTEGER DEFAULT 0, conversation_id TEXT DEFAULT '', message_content TEXT DEFAULT '', channel TEXT DEFAULT '', status TEXT DEFAULT '', created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE characters (id TEXT PRIMARY KEY, name TEXT DEFAULT '', is_default INTEGER DEFAULT 0)",
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	return &service{db: db}
}

func TestRunActiveMessageTaskDoesNotUseOtherCharacterConversation(t *testing.T) {
	svc := setupCompanionScopeService(t)
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-other', 'char-2', 'web', '2026-07-01 10:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO active_message_settings (character_id, channel) VALUES ('char-1', 'web')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO active_message_task (id, character_id, task_type, due_time, prompt, status) VALUES (1, 'char-1', 'morning_share', '2026-07-01 08:00:00', '早安', 'PENDING')").Error; err != nil {
		t.Fatal(err)
	}

	result := svc.RunActiveMessageTask(1, "char-1")
	if result["status"] != "NO_CONVERSATION" {
		t.Fatalf("expected NO_CONVERSATION, got %#v", result)
	}

	var count int64
	if err := svc.db.Table("messages").Where("conversation_id = ?", "conv-other").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no message in other character conversation, got %d", count)
	}
}

func TestRunActiveMessageTaskUsesScopedConversation(t *testing.T) {
	svc := setupCompanionScopeService(t)
	fake := &fakeProactiveUnifiedEntry{}
	svc.unifiedEntry = fake
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-other', 'char-2', 'web', '2026-07-01 10:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-target', 'char-1', 'web', '2026-07-01 09:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO active_message_settings (character_id, channel) VALUES ('char-1', 'web')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO active_message_task (id, character_id, task_type, due_time, prompt, status) VALUES (1, 'char-1', 'morning_share', '2026-07-01 08:00:00', '早安', 'PENDING')").Error; err != nil {
		t.Fatal(err)
	}

	result := svc.RunActiveMessageTask(1, "char-1")
	if result["status"] != "SENT" {
		t.Fatalf("expected SENT, got %#v", result)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("expected one unified entry request, got %d", len(fake.requests))
	}
	if fake.requests[0].ConversationID != "conv-target" || fake.requests[0].CharacterID != "char-1" {
		t.Fatalf("unexpected unified entry scope: %#v", fake.requests[0])
	}

	var targetCount int64
	if err := svc.db.Table("messages").Where("conversation_id = ?", "conv-target").Count(&targetCount).Error; err != nil {
		t.Fatal(err)
	}
	if targetCount != 0 {
		t.Fatalf("expected no direct target message, got %d", targetCount)
	}
	var otherCount int64
	if err := svc.db.Table("messages").Where("conversation_id = ?", "conv-other").Count(&otherCount).Error; err != nil {
		t.Fatal(err)
	}
	if otherCount != 0 {
		t.Fatalf("expected no other character messages, got %d", otherCount)
	}
}

func TestExternalConversationLookupKeepsCharacterScope(t *testing.T) {
	svc := setupCompanionScopeService(t)
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, peer_id, updated_at) VALUES ('conv-other-wechat', 'char-2', 'wechat', 'peer-other', '2026-07-01 11:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, peer_id, updated_at) VALUES ('conv-target-wechat', 'char-1', 'wechat', 'peer-target', '2026-07-01 09:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if got := svc.getWechatConvIDForChar("char-1"); got != "conv-target-wechat" {
		t.Fatalf("expected scoped wechat conversation, got %q", got)
	}
	if got := svc.getQQConvIDForChar("char-1"); got != "" {
		t.Fatalf("expected no scoped qq conversation, got %q", got)
	}
}

func TestIdleDurationUsesCharacterMessagesOnly(t *testing.T) {
	svc := setupCompanionScopeService(t)
	oldAt := time.Now().Add(-30 * time.Hour).Format("2006-01-02 15:04:05")
	recentAt := time.Now().Add(-5 * time.Minute).Format("2006-01-02 15:04:05")
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-target', 'char-1', 'web', ?)", oldAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-other', 'char-2', 'web', ?)", recentAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES ('msg-target', 'conv-target', 'user', '旧消息', ?)", oldAt).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES ('msg-other', 'conv-other', 'user', '新消息', ?)", recentAt).Error; err != nil {
		t.Fatal(err)
	}
	if got := svc.getIdleDuration("char-1"); got < 24*time.Hour {
		t.Fatalf("expected char-1 idle duration to ignore other character recent message, got %s", got)
	}
}

func TestRandomBurstEligibilityUsesCharacterScope(t *testing.T) {
	svc := setupCompanionScopeService(t)
	now := time.Date(2026, 7, 2, 14, 0, 0, 0, time.Local)
	svc.burstScopes = map[string]burstScopeState{
		"char-2": {lastAt: now.Add(-time.Hour), todayCount: 1},
	}
	setting := map[string]interface{}{
		"enabled":     true,
		"quietStart":  "23:59",
		"quietEnd":    "00:01",
		"minInterval": 60,
		"maxPerDay":   1,
	}

	eligible, reason := svc.checkBurstEligibility("char-1", setting, "IDLE", now)
	if !eligible {
		t.Fatalf("expected char-1 eligible without char-2 budget, got %q", reason)
	}
	eligible, reason = svc.checkBurstEligibility("char-2", setting, "IDLE", now)
	if eligible || reason != "maxPerDay" {
		t.Fatalf("expected char-2 maxPerDay, eligible=%v reason=%q", eligible, reason)
	}

	count := svc.recordBurstTriggered("char-1", now)
	if count != 1 {
		t.Fatalf("expected char-1 burst count 1, got %d", count)
	}
	if got := svc.getBurstScopeState("char-2", now).todayCount; got != 1 {
		t.Fatalf("expected char-2 burst count unchanged, got %d", got)
	}
}

func TestRandomBurstScopeRestoresFromPersistedMessages(t *testing.T) {
	svc := setupCompanionScopeService(t)
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-target', 'char-1', 'web', '2026-07-02 09:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-other', 'char-2', 'web', '2026-07-02 09:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO messages (id, conversation_id, role, content, source, created_at) VALUES ('burst-1', 'conv-target', 'assistant', 'target 1', 'proactive', '2026-07-02 10:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO messages (id, conversation_id, role, content, source, created_at) VALUES ('burst-2', 'conv-target', 'assistant', 'target 2', 'proactive', '2026-07-02 13:30:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO messages (id, conversation_id, role, content, source, created_at) VALUES ('burst-other', 'conv-other', 'assistant', 'other', 'proactive', '2026-07-02 12:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO messages (id, conversation_id, role, content, source, created_at) VALUES ('manual-1', 'conv-target', 'assistant', 'manual', 'proactive', '2026-07-02 13:45:00')").Error; err != nil {
		t.Fatal(err)
	}

	restarted := &service{db: svc.db}
	now := time.Date(2026, 7, 2, 14, 0, 0, 0, time.Local)
	scope := restarted.getBurstScopeState("char-1", now)
	if scope.todayCount != 2 {
		t.Fatalf("expected restored burst count 2, got %d", scope.todayCount)
	}
	if scope.lastAt.Format("2006-01-02 15:04:05") != "2026-07-02 13:30:00" {
		t.Fatalf("expected restored last burst at 13:30, got %s", scope.lastAt.Format("2006-01-02 15:04:05"))
	}
	setting := map[string]interface{}{
		"enabled":     true,
		"quietStart":  "23:59",
		"quietEnd":    "00:01",
		"minInterval": 1,
		"maxPerDay":   2,
	}
	eligible, reason := restarted.checkBurstEligibility("char-1", setting, "IDLE", now)
	if eligible || reason != "maxPerDay" {
		t.Fatalf("expected restored budget to block char-1, eligible=%v reason=%q", eligible, reason)
	}
}

func TestCountTodayProactiveMessagesUsesCharacterConversationScope(t *testing.T) {
	svc := setupCompanionScopeService(t)
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-target', 'char-1', 'web', '2026-07-02 09:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-other', 'char-2', 'web', '2026-07-02 09:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO proactive_messages (conversation_id, message_content, channel, status, created_at) VALUES ('conv-target', 'target today', 'web', 'sent', '2026-07-02 10:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO proactive_messages (conversation_id, message_content, channel, status, created_at) VALUES ('conv-other', 'other today 1', 'web', 'sent', '2026-07-02 10:30:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO proactive_messages (conversation_id, message_content, channel, status, created_at) VALUES ('conv-other', 'other today 2', 'web', 'sent', '2026-07-02 11:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Exec("INSERT INTO proactive_messages (conversation_id, message_content, channel, status, created_at) VALUES ('conv-target', 'target yesterday', 'web', 'sent', '2026-07-01 10:00:00')").Error; err != nil {
		t.Fatal(err)
	}

	if got := svc.countTodayProactiveMessages("char-1", "2026-07-02"); got != 1 {
		t.Fatalf("expected char-1 count 1, got %d", got)
	}
	if got := svc.countTodayProactiveMessages("char-2", "2026-07-02"); got != 2 {
		t.Fatalf("expected char-2 count 2, got %d", got)
	}
}

package proactive

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupProactiveScopeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "proactive.db")), &gorm.Config{})
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
		"CREATE TABLE characters (id TEXT PRIMARY KEY, name TEXT DEFAULT '', identity TEXT DEFAULT '', is_active INTEGER DEFAULT 0, conversation_id TEXT DEFAULT '')",
		"CREATE TABLE conversations (id TEXT PRIMARY KEY, character_id TEXT DEFAULT '', title TEXT DEFAULT '', channel TEXT DEFAULT 'web', source TEXT DEFAULT 'manual', peer_id TEXT DEFAULT '', message_count INTEGER DEFAULT 0, created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL, role TEXT NOT NULL, content TEXT NOT NULL, msg_type TEXT DEFAULT 'text', source TEXT DEFAULT 'manual', safety_level TEXT DEFAULT 'normal', status TEXT DEFAULT 'sent', include_in_context INTEGER DEFAULT 1, created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE reminders (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT DEFAULT '', content TEXT DEFAULT '', channel TEXT DEFAULT 'web', conversation_id TEXT DEFAULT '', character_id TEXT DEFAULT '', remind_at TEXT DEFAULT '', repeat_rule TEXT DEFAULT 'none', enabled INTEGER DEFAULT 1, last_triggered_at TEXT DEFAULT '', created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
		"CREATE TABLE proactive_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, rule_id INTEGER DEFAULT 0, conversation_id TEXT DEFAULT '', message_content TEXT DEFAULT '', channel TEXT DEFAULT '', status TEXT DEFAULT '', created_at TEXT DEFAULT '', updated_at TEXT DEFAULT '')",
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestResolveProactiveConversationKeepsScope(t *testing.T) {
	db := setupProactiveScopeDB(t)
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-other', 'char-2', 'web', '2026-07-01 10:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if got := resolveProactiveConversation(db, "", "char-1", "web", false); got != "" {
		t.Fatalf("expected no scoped conversation, got %q", got)
	}
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-target', 'char-1', 'web', '2026-07-01 11:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if got := resolveProactiveConversation(db, "", "char-1", "web", false); got != "conv-target" {
		t.Fatalf("expected target conversation, got %q", got)
	}
	if got := resolveProactiveConversation(db, "conv-other", "char-1", "web", false); got != "" {
		t.Fatalf("expected explicit mismatched conversation to fail, got %q", got)
	}
}

func TestResolveProactiveCharacterProfileRequiresExplicitCharacter(t *testing.T) {
	db := setupProactiveScopeDB(t)
	if err := db.Exec("INSERT INTO characters (id, name, identity, is_active) VALUES ('char-1', '小艾', '测试身份', 1)").Error; err != nil {
		t.Fatal(err)
	}
	if name, identity, ok := resolveProactiveCharacterProfile(db, ""); ok || name != "" || identity != "" {
		t.Fatalf("expected blank character scope to fail, got ok=%v name=%q identity=%q", ok, name, identity)
	}
	if name, identity, ok := resolveProactiveCharacterProfile(db, "char-1"); !ok || name != "小艾" || identity != "测试身份" {
		t.Fatalf("expected scoped character profile, got ok=%v name=%q identity=%q", ok, name, identity)
	}
	if name, identity, ok := resolveProactiveCharacterProfile(db, "char-missing"); ok || name != "" || identity != "" {
		t.Fatalf("expected missing character to fail, got ok=%v name=%q identity=%q", ok, name, identity)
	}
}

func TestResolveProactiveCharacterUsesExplicitConversationOnly(t *testing.T) {
	db := setupProactiveScopeDB(t)
	if err := db.Exec("INSERT INTO characters (id, name, identity, is_active) VALUES ('char-1', '会话角色', '会话身份', 0)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO characters (id, name, identity, is_active) VALUES ('char-active', '激活角色', '错误身份', 1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-target', 'char-1', 'web', '2026-07-01 11:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	if ch, ok := resolveProactiveCharacter(db, "", ""); ok || ch.ID != "" {
		t.Fatalf("expected blank scope to fail, got ok=%v character=%+v", ok, ch)
	}
	ch, ok := resolveProactiveCharacter(db, "", "conv-target")
	if !ok || ch.ID != "char-1" || ch.Name != "会话角色" || ch.Identity != "会话身份" {
		t.Fatalf("expected conversation scoped character, got ok=%v character=%+v", ok, ch)
	}
}

func TestTriggerReminderNowDoesNotUseOtherCharacterConversation(t *testing.T) {
	db := setupProactiveScopeDB(t)
	handler := &Handler{db: db}
	rem := &Reminder{
		ID:          1,
		Title:       "提醒",
		Channel:     "web",
		CharacterID: "char-1",
	}
	if err := db.Exec("INSERT INTO reminders (id, title, channel, character_id, enabled) VALUES (1, '提醒', 'web', 'char-1', 1)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-other', 'char-2', 'web', '2026-07-01 10:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	msgID, convID := handler.triggerReminderNow(rem)
	if msgID != "" || convID != "" {
		t.Fatalf("expected reminder trigger to fail without scoped conversation, got msg=%q conv=%q", msgID, convID)
	}
	var count int64
	if err := db.Table("messages").Where("conversation_id = ?", "conv-other").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no message in other character conversation, got %d", count)
	}
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, updated_at) VALUES ('conv-target', 'char-1', 'web', '2026-07-01 11:00:00')").Error; err != nil {
		t.Fatal(err)
	}
	msgID, convID = handler.triggerReminderNow(rem)
	if msgID == "" || convID != "conv-target" {
		t.Fatalf("expected scoped reminder delivery, got msg=%q conv=%q", msgID, convID)
	}
}

func TestExternalConversationLookupKeepsCharacterScope(t *testing.T) {
	db := setupProactiveScopeDB(t)
	handler := &Handler{db: db}
	executor := NewExecutor(db)
	statements := []string{
		"INSERT INTO conversations (id, character_id, channel, peer_id, updated_at) VALUES ('conv-wechat-other', 'char-2', 'wechat', 'peer-2', '2026-07-01 12:00:00')",
		"INSERT INTO conversations (id, character_id, channel, peer_id, updated_at) VALUES ('conv-wechat-target', 'char-1', 'wechat', 'peer-1', '2026-07-01 11:00:00')",
		"INSERT INTO conversations (id, character_id, channel, peer_id, updated_at) VALUES ('conv-qq-other', 'char-2', 'qq', 'peer-4', '2026-07-01 12:00:00')",
		"INSERT INTO conversations (id, character_id, channel, peer_id, updated_at) VALUES ('conv-qq-target', 'char-1', 'qq', 'peer-3', '2026-07-01 11:00:00')",
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	if got := handler.getWechatConvIDForTrigger("char-1"); got != "conv-wechat-target" {
		t.Fatalf("handler wechat scope mismatch: %q", got)
	}
	if got := handler.getQQConvIDForTrigger("char-1"); got != "conv-qq-target" {
		t.Fatalf("handler qq scope mismatch: %q", got)
	}
	if got := executor.getWechatConvID("char-1"); got != "conv-wechat-target" {
		t.Fatalf("executor wechat scope mismatch: %q", got)
	}
	if got := executor.getQQConvID("char-1"); got != "conv-qq-target" {
		t.Fatalf("executor qq scope mismatch: %q", got)
	}
	if got := handler.getWechatConvIDForTrigger("char-missing"); got != "" {
		t.Fatalf("expected no wechat binding for missing character, got %q", got)
	}
}

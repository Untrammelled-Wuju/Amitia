package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func schemaTableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count).Error; err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return count > 0
}

func TestFreshInstallBaselineCreatesProjects(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}
	if !schemaTableExists(t, db, "projects") {
		t.Fatal("baseline.sql does not create projects for fresh installs")
	}
	if schemaTableExists(t, db, "conversation_workspace_bindings") {
		t.Fatal("legacy conversation_workspace_bindings table must not exist")
	}
}

func TestSidebarProjectsMigrationConvertsWorkspaceBindings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "app.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`CREATE TABLE conversations (
		id TEXT PRIMARY KEY,
		space_id TEXT NOT NULL DEFAULT '',
		character_id TEXT DEFAULT '',
		title TEXT DEFAULT '',
		channel TEXT DEFAULT 'web',
		source TEXT DEFAULT 'manual',
		peer_id TEXT DEFAULT '',
		message_count INTEGER DEFAULT 0,
		state_version TEXT DEFAULT '',
		created_at TEXT DEFAULT '',
		updated_at TEXT DEFAULT '',
		revision INTEGER NOT NULL DEFAULT 1,
		deleted_at DATETIME
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TEXT DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE characters (id TEXT PRIMARY KEY, conversation_id TEXT DEFAULT '')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE workspace_mounts (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		kind TEXT NOT NULL DEFAULT 'local',
		enabled INTEGER NOT NULL DEFAULT 1
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE conversation_workspace_bindings (
		conversation_id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		device_id TEXT NOT NULL DEFAULT '',
		workspace_name TEXT NOT NULL DEFAULT '',
		workspace_kind TEXT NOT NULL DEFAULT 'local',
		root_uri TEXT NOT NULL DEFAULT '',
		updated_at DATETIME NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO conversations (id, space_id, character_id, title) VALUES ('conv-1', 'space-1', 'char-1', '项目对话')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO messages (id, conversation_id, role, content) VALUES ('msg-1', 'conv-1', 'user', '你好')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO workspace_mounts (id, name, kind) VALUES ('workspace-1', '项目', 'local')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO conversation_workspace_bindings
		(conversation_id, workspace_id, workspace_name, root_uri, updated_at)
		VALUES ('conv-1', 'workspace-1', '项目', 'amitia://workspace/@workspace-1/', datetime('now'))`).Error; err != nil {
		t.Fatal(err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{SidebarProjectsMigration()}); err != nil {
		t.Fatalf("apply sidebar projects migration: %v", err)
	}
	var projectID string
	if err := db.Raw("SELECT project_id FROM conversations WHERE id = 'conv-1'").Scan(&projectID).Error; err != nil {
		t.Fatal(err)
	}
	if projectID == "" {
		t.Fatal("conversation was not assigned to a project")
	}
	var characterID string
	if err := db.Raw("SELECT character_id FROM messages WHERE id = 'msg-1'").Scan(&characterID).Error; err != nil {
		t.Fatal(err)
	}
	if characterID != "char-1" {
		t.Fatalf("message character id = %q, want char-1", characterID)
	}
	if schemaTableExists(t, db, "conversation_workspace_bindings") {
		t.Fatal("legacy workspace binding table was not removed")
	}
}

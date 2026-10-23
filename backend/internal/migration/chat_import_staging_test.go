package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestChatImportStagingMigrationCopiesLegacyWithoutTouchingSource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy-import.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, space_id TEXT, title TEXT, source TEXT, created_at TEXT, updated_at TEXT, deleted_at DATETIME)`,
		`CREATE TABLE messages (id TEXT PRIMARY KEY, conversation_id TEXT, character_id TEXT, sequence INTEGER, role TEXT, content TEXT, created_at TEXT, deleted_at DATETIME)`,
		`INSERT INTO conversations VALUES ('legacy-conversation', 'space', 'Legacy chat', 'import', '2026-10-23 10:00:00', '2026-10-23 10:01:00', NULL)`,
		`INSERT INTO messages VALUES ('legacy-message-1', 'legacy-conversation', 'role', 1, 'user', 'hello', '2026-10-23 10:00:00', NULL)`,
		`INSERT INTO messages VALUES ('legacy-message-2', 'legacy-conversation', 'role', 2, 'assistant', 'world', '2026-10-23 10:01:00', NULL)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]Migration{ChatImportStagingMigration()}); err != nil {
		t.Fatal(err)
	}
	var batchID, status, roleID string
	var messageCount int
	if err := db.Raw(`SELECT id, status, role_id, message_count FROM chat_import_batches WHERE legacy_conversation_id = 'legacy-conversation'`).Row().Scan(&batchID, &status, &roleID, &messageCount); err != nil {
		t.Fatal(err)
	}
	if batchID == "" || status != "ready_for_promotion" || roleID != "role" || messageCount != 2 {
		t.Fatalf("batch=%s status=%s role=%s count=%d", batchID, status, roleID, messageCount)
	}
	var itemCount int
	if err := db.Raw(`SELECT COUNT(*) FROM chat_import_items WHERE batch_id = ?`, batchID).Row().Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if itemCount != 2 {
		t.Fatalf("item count=%d", itemCount)
	}
	var legacyCount int
	if err := db.Raw(`SELECT COUNT(*) FROM messages WHERE conversation_id = 'legacy-conversation'`).Row().Scan(&legacyCount); err != nil {
		t.Fatal(err)
	}
	if legacyCount != 2 {
		t.Fatalf("legacy source changed: %d", legacyCount)
	}
}

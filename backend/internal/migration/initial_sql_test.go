package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openInitialSQLTestDB(t *testing.T) *gorm.DB {
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
		_ = sqlDB.Close()
	})
	return db
}

func TestApplyInitialSQLReturnsErrorAndRollsBack(t *testing.T) {
	db := openInitialSQLTestDB(t)
	err := ApplyInitialSQL(db, `
CREATE TABLE created_before_error (id TEXT PRIMARY KEY);
CREATE TABL broken_statement (id TEXT PRIMARY KEY);
`)
	if err == nil {
		t.Fatal("expected invalid sql to fail")
	}
	if !strings.Contains(err.Error(), "statement 2") {
		t.Fatalf("error = %q, want statement number", err.Error())
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'created_before_error'").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("created_before_error table count = %d, want rollback", count)
	}
}

func TestApplyInitialSQLAllowsDuplicateColumns(t *testing.T) {
	db := openInitialSQLTestDB(t)
	err := ApplyInitialSQL(db, `
CREATE TABLE profiles (id TEXT PRIMARY KEY);
ALTER TABLE profiles ADD COLUMN display_name TEXT DEFAULT '';
ALTER TABLE profiles ADD COLUMN display_name TEXT DEFAULT '';
`)
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(profiles)").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, row := range rows {
		if row.Name == "display_name" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("display_name column count = %d, want 1", found)
	}
}

func TestApplyInitialSQLRepairsRepeatedMessageSequenceBackfillConflict(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := db.Exec(`
CREATE TABLE messages (
	id TEXT PRIMARY KEY,
	conversation_id TEXT NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL DEFAULT '',
	sequence INTEGER NOT NULL DEFAULT 0
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_messages_conv_sequence_unique ON messages(conversation_id, sequence)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, content, created_at, sequence) VALUES (?, ?, ?, ?, ?)", "m1", "c1", "a", "2026-01-01 00:00:02", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO messages (id, conversation_id, content, created_at, sequence) VALUES (?, ?, ?, ?, ?)", "m2", "c1", "b", "2026-01-01 00:00:01", 0).Error; err != nil {
		t.Fatal(err)
	}
	err := ApplyInitialSQL(db, `
UPDATE messages SET sequence = (
    SELECT rn FROM (
        SELECT id, ROW_NUMBER() OVER (PARTITION BY conversation_id ORDER BY created_at, id) AS rn
        FROM messages
    ) ranked WHERE ranked.id = messages.id
) WHERE sequence IS NULL OR sequence <= 0;
`)
	if err != nil {
		t.Fatal(err)
	}
	var sequence int64
	if err := db.Raw("SELECT sequence FROM messages WHERE id = ?", "m2").Scan(&sequence).Error; err != nil {
		t.Fatal(err)
	}
	if sequence != 2 {
		t.Fatalf("sequence = %d, want 2", sequence)
	}
}

func TestApplyInitialSQLRetriesIndexesAfterColumnsAreAdded(t *testing.T) {
	db := openInitialSQLTestDB(t)
	err := ApplyInitialSQL(db, `
CREATE TABLE retrieval_logs (id TEXT PRIMARY KEY);
CREATE INDEX IF NOT EXISTS idx_retrieval_logs_request_created ON retrieval_logs(request_id, created_at);
ALTER TABLE retrieval_logs ADD COLUMN request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN created_at TEXT NOT NULL DEFAULT '';
`)
	if err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_retrieval_logs_request_created'").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("index count = %d, want 1", count)
	}
}

func TestApplyInitialSQLReturnsDeferredIndexErrors(t *testing.T) {
	db := openInitialSQLTestDB(t)
	err := ApplyInitialSQL(db, `
CREATE TABLE retrieval_logs (id TEXT PRIMARY KEY);
CREATE INDEX IF NOT EXISTS idx_retrieval_logs_missing ON retrieval_logs(missing_column);
`)
	if err == nil {
		t.Fatal("expected unresolved index to fail")
	}
	if !strings.Contains(err.Error(), "deferred initial sql statement 2") {
		t.Fatalf("error = %q, want deferred statement number", err.Error())
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'retrieval_logs'").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("retrieval_logs table count = %d, want rollback", count)
	}
}

func TestApplyInitialSQLAcceptsCurrentDataScript(t *testing.T) {
	db := openInitialSQLTestDB(t)
	data, err := os.ReadFile(filepath.Join("..", "..", "data", "sql.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyInitialSQL(db, string(data)); err != nil {
		t.Fatal(err)
	}
}

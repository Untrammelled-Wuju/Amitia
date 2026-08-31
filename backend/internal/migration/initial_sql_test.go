package migration

import (
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

func TestApplyInitialSQLSkipsExistingColumns(t *testing.T) {
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

func TestApplyInitialSQLRepeatedExecutionNoError(t *testing.T) {
	db := openInitialSQLTestDB(t)
	sql := `
CREATE TABLE IF NOT EXISTS items (id TEXT PRIMARY KEY);
ALTER TABLE items ADD COLUMN label TEXT DEFAULT '';
ALTER TABLE items ADD COLUMN priority INTEGER DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_items_label ON items(label);
`
	if err := ApplyInitialSQL(db, sql); err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if err := ApplyInitialSQL(db, sql); err != nil {
		t.Fatalf("second apply should be idempotent without error, got: %v", err)
	}
	var colCount int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('items') WHERE name = 'label'").Scan(&colCount).Error; err != nil {
		t.Fatal(err)
	}
	if colCount != 1 {
		t.Fatalf("label column count = %d, want 1 after repeated execution", colCount)
	}
}

func TestApplyInitialSQLSkipsAddColumnWhenTableMissing(t *testing.T) {
	db := openInitialSQLTestDB(t)
	err := ApplyInitialSQL(db, `
ALTER TABLE nonexistent_table ADD COLUMN extra TEXT DEFAULT '';
`)
	if err != nil {
		t.Fatalf("should skip ALTER TABLE on missing table without error, got: %v", err)
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'nonexistent_table'").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("nonexistent_table should not be created, got count = %d", count)
	}
}

func TestApplyInitialSQLRealSQLErrorBlocksExecution(t *testing.T) {
	db := openInitialSQLTestDB(t)
	err := ApplyInitialSQL(db, `
CREATE TABLE valid_table (id TEXT PRIMARY KEY);
INSERT INTO valid_table (id, nonexistent_col) VALUES ('x', 'y');
`)
	if err == nil {
		t.Fatal("expected real SQL error to block execution")
	}
	if !strings.Contains(err.Error(), "statement 2") {
		t.Fatalf("error = %q, want statement number", err.Error())
	}
}

func TestApplyInitialSQLSkipsExistingIndexWithoutIfNotExists(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := db.Exec("CREATE TABLE t1 (id TEXT PRIMARY KEY, name TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE INDEX idx_t1_name ON t1(name)").Error; err != nil {
		t.Fatal(err)
	}
	err := ApplyInitialSQL(db, `
CREATE INDEX idx_t1_name ON t1(name);
`)
	if err != nil {
		t.Fatalf("should skip existing index without IF NOT EXISTS, got: %v", err)
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

func TestApplyInitialSQLAcceptsEmbeddedBaseline(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyInitialSQL(db, baselineSQL); err != nil {
		t.Fatal(err)
	}
}

func TestApplyInitialSQLEmbeddedBaselineRepeatedNoError(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyInitialSQL(db, baselineSQL); err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	if err := ApplyInitialSQL(db, baselineSQL); err != nil {
		t.Fatalf("second apply should be idempotent without error, got: %v", err)
	}
}

func TestApplyBaselineOnEmptyDatabase(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline on empty database failed: %v", err)
	}
	var tableCount int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&tableCount).Error; err != nil {
		t.Fatal(err)
	}
	if tableCount < 10 {
		t.Fatalf("table count = %d, want >= 10 after baseline", tableCount)
	}
}

func TestApplyBaselineAfterEmbeddedBaseline(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyInitialSQL(db, baselineSQL); err != nil {
		t.Fatalf("apply embedded baseline through generic initial SQL path failed: %v", err)
	}
	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline after generic initial SQL path failed: %v", err)
	}
	var colCount int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name = 'sequence'").Scan(&colCount).Error; err != nil {
		t.Fatal(err)
	}
	if colCount != 1 {
		t.Fatalf("messages.sequence column count = %d, want 1", colCount)
	}
	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("second baseline apply should be idempotent without error, got: %v", err)
	}
}

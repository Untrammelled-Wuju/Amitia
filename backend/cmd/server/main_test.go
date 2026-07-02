package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/config"
	"gorm.io/gorm"
)

func TestInitDatabaseMarksLegacyRetrievalLogs(t *testing.T) {
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "app.db")), &gorm.Config{})
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

	sqlText := `
CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS retrieval_logs (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL DEFAULT '',
    query_text TEXT NOT NULL DEFAULT '',
    retrieved_memory_ids TEXT DEFAULT '[]',
    scoring_details TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);
ALTER TABLE retrieval_logs ADD COLUMN character_id TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN channel TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN retrieval_version TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN legacy INTEGER NOT NULL DEFAULT 0;
UPDATE retrieval_logs
SET legacy = 1
WHERE conversation_id = ''
   OR conversation_id NOT IN (SELECT id FROM conversations);
`
	if err := os.WriteFile(filepath.Join(dataDir, "sql.sql"), []byte(sqlText), 0644); err != nil {
		t.Fatal(err)
	}

	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{}
	config.AppCfg.Storage.DataDir = dataDir
	t.Cleanup(func() {
		config.AppCfg = originalCfg
	})

	if err := db.Exec("CREATE TABLE conversations (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE retrieval_logs (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL DEFAULT '', query_text TEXT NOT NULL DEFAULT '', retrieved_memory_ids TEXT DEFAULT '[]', scoring_details TEXT DEFAULT '{}', created_at TEXT DEFAULT (datetime('now')))").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id) VALUES ('conv-real')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO retrieval_logs (id, conversation_id, query_text) VALUES ('row-1', 'conv-real', 'ok'), ('row-2', 'char-as-conv', 'bad'), ('row-3', '', 'empty')").Error; err != nil {
		t.Fatal(err)
	}

	if err := initDatabase(db); err != nil {
		t.Fatal(err)
	}

	var realLegacy int
	if err := db.Raw("SELECT legacy FROM retrieval_logs WHERE id = 'row-1'").Scan(&realLegacy).Error; err != nil {
		t.Fatal(err)
	}
	if realLegacy != 0 {
		t.Fatalf("row-1 legacy = %d, want 0", realLegacy)
	}

	var badLegacy int
	if err := db.Raw("SELECT legacy FROM retrieval_logs WHERE id = 'row-2'").Scan(&badLegacy).Error; err != nil {
		t.Fatal(err)
	}
	if badLegacy != 1 {
		t.Fatalf("row-2 legacy = %d, want 1", badLegacy)
	}

	var emptyLegacy int
	if err := db.Raw("SELECT legacy FROM retrieval_logs WHERE id = 'row-3'").Scan(&emptyLegacy).Error; err != nil {
		t.Fatal(err)
	}
	if emptyLegacy != 1 {
		t.Fatalf("row-3 legacy = %d, want 1", emptyLegacy)
	}

	var requestID string
	if err := db.Raw("SELECT request_id FROM retrieval_logs WHERE id = 'row-1'").Scan(&requestID).Error; err != nil {
		t.Fatal(err)
	}
	if requestID != "" {
		t.Fatalf("request_id = %q, want empty default", requestID)
	}
}

func TestInitDatabaseAddsConversationScopeIndexes(t *testing.T) {
	dataDir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dataDir, "app.db")), &gorm.Config{})
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

	sqlText := `
CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    character_id TEXT DEFAULT '',
    title TEXT DEFAULT '',
    channel TEXT DEFAULT 'web',
    source TEXT DEFAULT 'manual',
    peer_id TEXT DEFAULT '',
    message_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_channel_peer_unique ON conversations(channel, peer_id) WHERE peer_id <> '';
CREATE INDEX IF NOT EXISTS idx_conversations_character_channel_updated ON conversations(character_id, channel, updated_at);
`
	if err := os.WriteFile(filepath.Join(dataDir, "sql.sql"), []byte(sqlText), 0644); err != nil {
		t.Fatal(err)
	}

	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{}
	config.AppCfg.Storage.DataDir = dataDir
	t.Cleanup(func() {
		config.AppCfg = originalCfg
	})

	if err := initDatabase(db); err != nil {
		t.Fatal(err)
	}

	type indexRow struct {
		Name    string `gorm:"column:name"`
		Unique  int    `gorm:"column:unique"`
		Origin  string `gorm:"column:origin"`
		Partial int    `gorm:"column:partial"`
	}
	var rows []indexRow
	if err := db.Raw("PRAGMA index_list(conversations)").Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	foundUnique := false
	foundScope := false
	for _, row := range rows {
		if row.Name == "idx_conversations_channel_peer_unique" {
			foundUnique = row.Unique == 1 && row.Partial == 1
		}
		if row.Name == "idx_conversations_character_channel_updated" {
			foundScope = true
		}
	}
	if !foundUnique {
		t.Fatal("missing unique partial index for channel + peer")
	}
	if !foundScope {
		t.Fatal("missing scope index for character + channel + updated_at")
	}

	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, peer_id) VALUES ('conv-1', 'char-1', 'qq', 'peer-1')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO conversations (id, character_id, channel, peer_id) VALUES ('conv-2', 'char-2', 'qq', 'peer-1')").Error; err == nil {
		t.Fatal("expected duplicate peer binding to fail")
	}
}

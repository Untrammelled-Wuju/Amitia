package main

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

func TestApplyDatabaseStartupMigrationsCreatesRetrievalLogsWithAllColumns(t *testing.T) {
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

	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{}
	config.AppCfg.Storage.DataDir = dataDir
	t.Cleanup(func() {
		config.AppCfg = originalCfg
	})

	if err := applyDatabaseStartupMigrations(db); err != nil {
		t.Fatal(err)
	}

	type columnRow struct {
		Name string `gorm:"column:name"`
	}
	var columns []columnRow
	if err := db.Raw("PRAGMA table_info(retrieval_logs)").Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}

	expectedColumns := map[string]bool{
		"id":                  false,
		"conversation_id":     false,
		"character_id":        false,
		"request_id":          false,
		"channel":             false,
		"retrieval_version":   false,
		"legacy":              false,
		"query_text":          false,
		"retrieved_memory_ids": false,
		"scoring_details":     false,
		"created_at":          false,
	}
	for _, col := range columns {
		if _, ok := expectedColumns[col.Name]; ok {
			expectedColumns[col.Name] = true
		}
	}
	for col, found := range expectedColumns {
		if !found {
			t.Fatalf("retrieval_logs missing column: %s", col)
		}
	}

	isNew, err := migration.IsNewDatabase(db)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Fatal("database should not be new after applying migrations")
	}

	var migrationCount int64
	if err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE status = 'applied'").Scan(&migrationCount).Error; err != nil {
		t.Fatal(err)
	}
	if migrationCount == 0 {
		t.Fatal("expected applied migrations in schema_migrations table")
	}
}

func TestApplyDatabaseStartupMigrationsCreatesConversationScopeIndexes(t *testing.T) {
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

	originalCfg := config.AppCfg
	config.AppCfg = &config.Config{}
	config.AppCfg.Storage.DataDir = dataDir
	t.Cleanup(func() {
		config.AppCfg = originalCfg
	})

	if err := applyDatabaseStartupMigrations(db); err != nil {
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

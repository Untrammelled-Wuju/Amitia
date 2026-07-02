package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/config"
	"gorm.io/gorm"
)

func TestApplyDatabaseStartupMigrationsBacksUpExistingDatabaseBeforeInitialSQL(t *testing.T) {
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

	if err := db.Exec("CREATE TABLE legacy_keep (id TEXT PRIMARY KEY, payload TEXT NOT NULL DEFAULT '')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO legacy_keep (id, payload) VALUES ('legacy-1', 'must survive in backup')").Error; err != nil {
		t.Fatal(err)
	}

	sqlText := `
DROP TABLE legacy_keep;
CREATE TABLE IF NOT EXISTS memories (id TEXT PRIMARY KEY, character_id TEXT DEFAULT '');
CREATE TABLE IF NOT EXISTS memory_candidates (id TEXT PRIMARY KEY);
CREATE TABLE IF NOT EXISTS conversations (id TEXT PRIMARY KEY, character_id TEXT DEFAULT '');
CREATE TABLE IF NOT EXISTS messages (id TEXT PRIMARY KEY, conversation_id TEXT DEFAULT '');
`
	if err := os.WriteFile(filepath.Join(dataDir, "sql.sql"), []byte(sqlText), 0644); err != nil {
		t.Fatal(err)
	}

	if err := applyDatabaseStartupMigrations(db); err != nil {
		t.Fatal(err)
	}

	var completedBackups int64
	if err := db.Raw("SELECT COUNT(*) FROM backup_records WHERE status = 'completed'").Scan(&completedBackups).Error; err != nil {
		t.Fatal(err)
	}
	if completedBackups != 1 {
		t.Fatalf("completed backups = %d, want 1", completedBackups)
	}

	var backupPath string
	if err := db.Raw("SELECT backup_path FROM backup_records WHERE status = 'completed' LIMIT 1").Scan(&backupPath).Error; err != nil {
		t.Fatal(err)
	}
	if backupPath == "" {
		t.Fatal("missing backup path")
	}

	backupDB, err := gorm.Open(sqlite.Open(backupPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	backupSQLDB, err := backupDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = backupSQLDB.Close()
	})

	var payload string
	if err := backupDB.Raw("SELECT payload FROM legacy_keep WHERE id = 'legacy-1'").Scan(&payload).Error; err != nil {
		t.Fatal(err)
	}
	if payload != "must survive in backup" {
		t.Fatalf("backup payload = %q, want preserved legacy payload", payload)
	}

	var liveLegacyCount int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'legacy_keep'").Scan(&liveLegacyCount).Error; err != nil {
		t.Fatal(err)
	}
	if liveLegacyCount != 0 {
		t.Fatalf("legacy_keep table still exists in live database")
	}

	var migrationCount int64
	if err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = '202607010102' AND status = 'applied'").Scan(&migrationCount).Error; err != nil {
		t.Fatal(err)
	}
	if migrationCount != 1 {
		t.Fatalf("memory scope migration applied count = %d, want 1", migrationCount)
	}
}

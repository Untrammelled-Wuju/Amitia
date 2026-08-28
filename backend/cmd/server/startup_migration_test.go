package main

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/migration"
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

	if err := applyDatabaseStartupMigrations(db, dataDir); err != nil {
		t.Fatal(err)
	}

	var completedBackups int64
	if err := db.Raw("SELECT COUNT(*) FROM backup_records WHERE status = 'completed'").Scan(&completedBackups).Error; err != nil {
		t.Fatal(err)
	}
	if completedBackups < 1 {
		t.Fatalf("completed backups = %d, want >= 1", completedBackups)
	}

	var backupPath string
	if err := db.Raw("SELECT backup_path FROM backup_records WHERE status = 'completed' ORDER BY finished_at DESC LIMIT 1").Scan(&backupPath).Error; err != nil {
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
	if liveLegacyCount != 1 {
		t.Fatalf("legacy_keep table should still exist (baseline uses IF NOT EXISTS), got count = %d", liveLegacyCount)
	}

	var canonicalBaselineCutover int64
	if err := db.Raw(`SELECT COUNT(*) FROM desktop_pet_migration_operations WHERE id = 'baseline-desktop-pet-v2'`).Scan(&canonicalBaselineCutover).Error; err != nil {
		t.Fatal(err)
	}
	if canonicalBaselineCutover != 0 {
		t.Fatalf("existing database must not receive an implicit desktop pet canonical cutover, got %d", canonicalBaselineCutover)
	}

	isNew, err := migration.IsNewDatabase(db)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Fatal("database should not be new after applying migrations")
	}
}

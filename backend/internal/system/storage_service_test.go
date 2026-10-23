package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

func newStorageTestService(t *testing.T) (*service, *gorm.DB) {
	t.Helper()
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
		sqlDB.Close()
	})
	return &service{db: db, dataDir: dataDir}, db
}

func TestDataPortabilityCoordinatorOwnsServiceDataDir(t *testing.T) {
	svc, _ := newStorageTestService(t)
	coordinator := dataportability.NewCoordinator(t.TempDir(), "test", "test", "schema", dataportability.NewStagingManager(t.TempDir()))
	svc.SetDataPortabilityCoordinator(coordinator)
	if svc.dataDir != coordinator.DataDir {
		t.Fatalf("service data dir = %s, coordinator = %s", svc.dataDir, coordinator.DataDir)
	}
}

func TestDataPortabilityRestorePreviewDecryptsEncryptedBackup(t *testing.T) {
	svc, db := newStorageTestService(t)
	if err := db.AutoMigrate(&AppSetting{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AppSetting{Key: "restore-preview", Value: "true"}).Error; err != nil {
		t.Fatal(err)
	}
	coordinator := dataportability.NewCoordinator(svc.dataDir, "test", "test", "migrations-v1-1", dataportability.NewStagingManager(svc.dataDir))
	if err := coordinator.RegisterContributors(NewSettingsBackupContributor(db)); err != nil {
		t.Fatal(err)
	}
	svc.SetDataPortabilityCoordinator(coordinator)
	backup, err := coordinator.CreateBackup(context.Background(), dataportability.BackupRequest{
		Scope:      dataportability.ScopeSettings,
		Profile:    dataportability.ProfileFull,
		Purpose:    dataportability.PurposeUser,
		Passphrase: "restore-preview-passphrase",
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(backup.Path)
	if _, err := svc.PreviewDataPortabilityRestore(context.Background(), name, "wrong-passphrase"); err == nil {
		t.Fatal("wrong passphrase was accepted")
	}
	preview, err := svc.PreviewDataPortabilityRestore(context.Background(), name, "restore-preview-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if preview["operationId"] == "" || preview["encrypted"] != true {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	if err := svc.CancelDataPortabilityRestore(); err != nil {
		t.Fatal(err)
	}
}

func TestGetStorageMigrationsPrefersSchemaMigrations(t *testing.T) {
	svc, db := newStorageTestService(t)
	if err := os.WriteFile(filepath.Join(svc.dataDir, ".migration_version"), []byte("old-file-version\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE schema_migrations (
version TEXT PRIMARY KEY,
name TEXT NOT NULL DEFAULT '',
checksum TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
error_message TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL DEFAULT '',
finished_at TEXT NOT NULL DEFAULT ''
)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO schema_migrations (version, name, status, started_at, finished_at) VALUES (?, ?, ?, ?, ?)`,
		"202607010102", "runtime memory columns", "applied", "2026-07-01T01:02:00+08:00", "2026-07-01T01:02:01+08:00").Error; err != nil {
		t.Fatal(err)
	}

	result := svc.GetStorageMigrations()
	if result["source"] != "schema_migrations" {
		t.Fatalf("source = %v", result["source"])
	}
	if result["currentVersion"] != "202607010102" {
		t.Fatalf("currentVersion = %v", result["currentVersion"])
	}
	if result["legacyVersion"] != "old-file-version" {
		t.Fatalf("legacyVersion = %v", result["legacyVersion"])
	}
	migrations := result["migrations"].([]interface{})
	first := migrations[0].(map[string]interface{})
	if first["version"] != "202607010102" || first["source"] != "schema_migrations" {
		t.Fatalf("migration = %#v", first)
	}

	check := svc.CheckStorageMigrations()
	if check["needsMigration"] != false {
		t.Fatalf("needsMigration = %v", check["needsMigration"])
	}
	if check["source"] != "schema_migrations" {
		t.Fatalf("check source = %v", check["source"])
	}
}

func TestGetStorageMigrationsFallsBackToLegacyFile(t *testing.T) {
	svc, _ := newStorageTestService(t)
	if err := os.WriteFile(filepath.Join(svc.dataDir, ".migration_version"), []byte("legacy-only\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result := svc.GetStorageMigrations()
	if result["source"] != "legacy_file" {
		t.Fatalf("source = %v", result["source"])
	}
	if result["currentVersion"] != "legacy-only" {
		t.Fatalf("currentVersion = %v", result["currentVersion"])
	}
	migrations := result["migrations"].([]interface{})
	first := migrations[0].(map[string]interface{})
	if first["version"] != "legacy-only" || first["applied"] != true {
		t.Fatalf("migration = %#v", first)
	}

	check := svc.CheckStorageMigrations()
	if check["needsMigration"] != false {
		t.Fatalf("needsMigration = %v", check["needsMigration"])
	}
	if check["source"] != "legacy_file" {
		t.Fatalf("check source = %v", check["source"])
	}
}

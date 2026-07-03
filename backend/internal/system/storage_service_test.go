package system

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
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

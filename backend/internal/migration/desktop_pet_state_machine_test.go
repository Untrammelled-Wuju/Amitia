// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "migration_test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlDB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestDesktopPetStateMachine_NewDatabaseFullMigration(t *testing.T) {
	db := setupMigrationTestDB(t)

	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(DefaultMigrations()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	tables := []string{
		"desktop_pet_processing_tasks",
		"desktop_pet_processing_actions",
		"desktop_pet_processing_action_attempts",
		"desktop_pet_processed_frames",
		"desktop_pet_packages",
		"desktop_pet_identities",
		"desktop_pet_package_releases",
		"desktop_pet_release_files",
		"desktop_pet_package_operations",
		"desktop_pet_installation_operations",
		"desktop_pet_active_bindings",
		"desktop_pet_installation_release_history",
		"desktop_pet_package_validation_reports",
		"desktop_pet_state_transitions",
		"schema_migrations",
	}

	for _, table := range tables {
		var count int64
		if err := db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count).Error; err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("table %s not found (count=%d)", table, count)
		}
	}

	var migrationCount int64
	if err := db.Raw("SELECT count(*) FROM schema_migrations").Scan(&migrationCount).Error; err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	totalMigrations := len(DefaultMigrations())
	if migrationCount != int64(totalMigrations) {
		t.Fatalf("migration count = %d, want %d", migrationCount, totalMigrations)
	}
}

func TestDesktopPetStateMachine_ColumnsExistAfterMigration(t *testing.T) {
	db := setupMigrationTestDB(t)

	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(DefaultMigrations()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	columnChecks := []struct {
		table   string
		columns []string
	}{
		{"desktop_pet_processing_tasks", []string{"id", "status", "current_stage", "row_version", "execution_id", "worker_id", "lease_expires_at", "last_heartbeat_at", "progress", "cancel_requested_at"}},
		{"desktop_pet_processing_actions", []string{"id", "status", "row_version", "progress", "started_at", "completed_at", "fps", "loop_type"}},
		{"desktop_pet_packages", []string{"id", "status", "package_path", "manifest_path", "package_hash", "included_actions", "action_count"}},
	}

	for _, cc := range columnChecks {
		for _, col := range cc.columns {
			var count int64
			if err := db.Raw("SELECT count(*) FROM pragma_table_info(?) WHERE name=?", cc.table, col).Scan(&count).Error; err != nil {
				t.Fatalf("check column %s.%s: %v", cc.table, col, err)
			}
			if count != 1 {
				t.Fatalf("column %s.%s not found", cc.table, col)
			}
		}
	}
}

func TestDesktopPetStateMachine_MigrationIdempotency(t *testing.T) {
	db := setupMigrationTestDB(t)

	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("first baseline: %v", err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(DefaultMigrations()); err != nil {
		t.Fatalf("first apply: %v", err)
	}

	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("second baseline (idempotent): %v", err)
	}

	if err := runner.Apply(DefaultMigrations()); err != nil {
		t.Fatalf("second apply (idempotent): %v", err)
	}

	var migrationCount int64
	if err := db.Raw("SELECT count(*) FROM schema_migrations").Scan(&migrationCount).Error; err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	totalMigrations := len(DefaultMigrations())
	if migrationCount != int64(totalMigrations) {
		t.Fatalf("migration count after re-apply = %d, want %d", migrationCount, totalMigrations)
	}
}

func TestDesktopPetStateMachine_PartialUpgradeScenario(t *testing.T) {
	db := setupMigrationTestDB(t)

	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}

	allMigrations := DefaultMigrations()
	halfIndex := len(allMigrations) / 2
	firstHalf := allMigrations[:halfIndex]

	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(firstHalf); err != nil {
		t.Fatalf("apply first half: %v", err)
	}

	var firstCount int64
	if err := db.Raw("SELECT count(*) FROM schema_migrations").Scan(&firstCount).Error; err != nil {
		t.Fatalf("count first half migrations: %v", err)
	}
	if firstCount != int64(halfIndex) {
		t.Fatalf("first half migration count = %d, want %d", firstCount, halfIndex)
	}

	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline (idempotent after first half): %v", err)
	}

	if err := runner.Apply(allMigrations); err != nil {
		t.Fatalf("apply all migrations (upgrade): %v", err)
	}

	var finalCount int64
	if err := db.Raw("SELECT count(*) FROM schema_migrations").Scan(&finalCount).Error; err != nil {
		t.Fatalf("count final migrations: %v", err)
	}
	if finalCount != int64(len(allMigrations)) {
		t.Fatalf("final migration count = %d, want %d", finalCount, len(allMigrations))
	}
}

func TestDesktopPetStateMachine_ChecksumValidation(t *testing.T) {
	db := setupMigrationTestDB(t)

	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply(DefaultMigrations()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	type schemaMigration struct {
		Version  string `gorm:"column:version"`
		Checksum string `gorm:"column:checksum"`
	}

	var migrations []schemaMigration
	if err := db.Raw("SELECT version, checksum FROM schema_migrations WHERE checksum != '' ORDER BY version").Scan(&migrations).Error; err != nil {
		t.Fatalf("query migrations with checksums: %v", err)
	}

	if len(migrations) == 0 {
		t.Fatal("expected at least some migrations with non-empty checksums")
	}

	for _, m := range migrations {
		if m.Version == "" {
			t.Fatal("migration version should not be empty")
		}
	}
}

package migration

import (
	"testing"

	"gorm.io/gorm"
)

func schemaTableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count).Error; err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return count > 0
}

func TestFreshInstallBaselineCreatesConversationWorkspaceBindings(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}
	if err := MarkAllMigrationsApplied(db, DefaultMigrations()); err != nil {
		t.Fatalf("mark migrations applied: %v", err)
	}
	if !schemaTableExists(t, db, "conversation_workspace_bindings") {
		t.Fatal("baseline.sql does not create conversation_workspace_bindings for fresh installs")
	}
}

func TestRepairMigrationCreatesConversationWorkspaceBindingsOnExistingDatabase(t *testing.T) {
	db := openInitialSQLTestDB(t)
	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}
	if err := db.Exec("DROP TABLE IF EXISTS conversation_workspace_bindings").Error; err != nil {
		t.Fatalf("drop table: %v", err)
	}
	if err := MarkAllMigrationsApplied(db, DefaultMigrations()); err != nil {
		t.Fatalf("mark migrations applied: %v", err)
	}
	if err := db.Exec("DELETE FROM schema_migrations WHERE version = ?", "20260915001").Error; err != nil {
		t.Fatalf("reset repair migration record: %v", err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply(DefaultMigrations()); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if !schemaTableExists(t, db, "conversation_workspace_bindings") {
		t.Fatal("repair migration did not create conversation_workspace_bindings on an existing database")
	}
}

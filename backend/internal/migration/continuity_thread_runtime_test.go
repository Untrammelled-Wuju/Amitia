package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestContinuityThreadRuntimeMigrationIsRegisteredAndInBaseline(t *testing.T) {
	registered := false
	for _, item := range DefaultMigrations() {
		if item.Version == "20260922001" {
			registered = item.Name == "continuity_thread_runtime"
			break
		}
	}
	if !registered {
		t.Fatal("continuity thread runtime migration is not registered")
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "continuity-migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"continuity_threads", "continuity_thread_bindings", "continuity_thread_events", "continuity_waits"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("baseline is missing %s", table)
		}
	}
	if !db.Migrator().HasColumn("interaction_records", "thread_id") {
		t.Fatal("interaction_records.thread_id is missing")
	}
}

func TestContinuityThreadRuntimeBaselineUpgradesLegacyInteractionRecords(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "continuity-legacy.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.Exec("CREATE TABLE interaction_records (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn("interaction_records", "thread_id") {
		t.Fatal("legacy interaction_records.thread_id is missing")
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_interaction_records_thread_id'").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("index count = %d, want 1", count)
	}
}

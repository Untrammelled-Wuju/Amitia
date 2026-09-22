package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestContinuityRuntimeCompletionMigrationIsRegisteredAndInBaseline(t *testing.T) {
	registered := false
	for _, item := range DefaultMigrations() {
		if item.Version == "20260922002" {
			registered = item.Name == "continuity_runtime_completion"
			break
		}
	}
	if !registered {
		t.Fatal("continuity runtime completion migration is not registered")
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "continuity-completion.db")), &gorm.Config{})
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
	for _, column := range []string{"resolved_by", "resolution_json", "auto_resume", "wake_state", "wake_request_id", "wake_attempts", "next_wake_at", "last_wake_error", "wake_delivered_at"} {
		if !db.Migrator().HasColumn("continuity_waits", column) {
			t.Fatalf("baseline is missing continuity_waits.%s", column)
		}
	}
}

func TestContinuityRuntimeCompletionMigrationKeepsLegacyUserWaitInlineOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "continuity-upgrade.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	if err := db.Exec(`CREATE TABLE continuity_waits (
		id TEXT PRIMARY KEY,
		thread_id TEXT NOT NULL,
		wait_type TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'waiting',
		description TEXT NOT NULL DEFAULT '',
		condition_json TEXT NOT NULL DEFAULT '{}',
		resume_hint TEXT NOT NULL DEFAULT '',
		due_at DATETIME,
		resolved_at DATETIME,
		source_execution_id TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO continuity_waits(id,thread_id,wait_type,status,created_at,updated_at) VALUES ('w-user','t1','user','waiting',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),('w-time','t1','time','waiting',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatal(err)
	}

	migration := ContinuityRuntimeCompletionMigration()
	step := &Step{db: db}
	if err := migration.Up(step); err != nil {
		t.Fatal(err)
	}
	for _, command := range step.commands {
		if err := db.Exec(command).Error; err != nil {
			t.Fatalf("execute %q: %v", command, err)
		}
	}
	var userResume, timeResume int
	if err := db.Raw("SELECT auto_resume FROM continuity_waits WHERE id = 'w-user'").Scan(&userResume).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("SELECT auto_resume FROM continuity_waits WHERE id = 'w-time'").Scan(&timeResume).Error; err != nil {
		t.Fatal(err)
	}
	if userResume != 0 {
		t.Fatalf("legacy user wait auto_resume=%d, want 0", userResume)
	}
	if timeResume != 1 {
		t.Fatalf("legacy non-user wait auto_resume=%d, want 1", timeResume)
	}
}

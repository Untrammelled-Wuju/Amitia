package migration

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunnerAcceptsAndNormalizesReleasedChecksum(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runner-checksum?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	migration := InteractionRecordsCreateMigration()
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}

	legacyChecksum := migration.AcceptedChecksums[0]
	if err := db.Model(&Record{}).Where("version = ?", migration.Version).Update("checksum", legacyChecksum).Error; err != nil {
		t.Fatal(err)
	}

	if err := runner.Apply([]Migration{migration, InteractionRecordsV2Migration()}); err != nil {
		t.Fatal(err)
	}

	var record Record
	if err := db.Where("version = ?", migration.Version).First(&record).Error; err != nil {
		t.Fatal(err)
	}
	currentChecksum, err := runner.computeMigrationChecksum(migration)
	if err != nil {
		t.Fatal(err)
	}
	if record.Checksum != currentChecksum {
		t.Fatalf("checksum = %s, want %s", record.Checksum, currentChecksum)
	}

	for _, column := range []string{"owner_instance_id", "heartbeat_at", "commit_token", "commit_owner", "commit_acquired_at", "result_message_ids", "delivery_intent_ids", "correlation_id", "causation_id"} {
		if !db.Migrator().HasColumn("interaction_records", column) {
			t.Fatalf("missing column %s", column)
		}
	}
}

func TestRunnerRejectsUnknownChecksum(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runner-checksum-reject?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	migration := InteractionRecordsCreateMigration()
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Record{}).Where("version = ?", migration.Version).Update("checksum", "unknown").Error; err != nil {
		t.Fatal(err)
	}

	err = runner.Apply([]Migration{migration})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

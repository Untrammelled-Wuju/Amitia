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

func TestRunnerNormalizesUnknownChecksumWhenCompatibilityEnabled(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runner-checksum-compat?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	runner := Runner{DB: db, SkipBackup: true, AllowUnknownAppliedChecksum: true}
	migration := InteractionRecordsCreateMigration()
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Record{}).Where("version = ?", migration.Version).Update("checksum", "legacy-release-checksum").Error; err != nil {
		t.Fatal(err)
	}
	if err := runner.Apply([]Migration{migration}); err != nil {
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
}

func TestRunnerChecksumUpDoesNotRepeatDirectDataMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runner-checksum-side-effect?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE side_effect_probe (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}

	migration := Migration{
		Version: "209901010001",
		Name:    "side_effect_checksum_probe",
		ChecksumUp: func(_ *Step) error {
			return nil
		},
		Up: func(s *Step) error {
			return s.DB().Exec("INSERT INTO side_effect_probe(value) VALUES ('applied')").Error
		},
	}
	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := db.Table("side_effect_probe").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("direct data migration executed %d times, want exactly once", count)
	}
}

func TestRunnerFailsWhenMigrationIgnoresStepMutationError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runner-latched-step-error?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	migration := Migration{
		Version: "209901010002",
		Name:    "ignored_step_error_probe",
		Up: func(s *Step) error {
			// This models historical migrations that ignored AddColumn/CreateIndex
			// return values. Step must latch the failure so Runner cannot mark the
			// migration applied with a partial schema.
			_ = s.AddColumn("unsafe;table", "value", "TEXT")
			return nil
		},
	}
	err = (Runner{DB: db, SkipBackup: true}).Apply([]Migration{migration})
	if err == nil || !strings.Contains(err.Error(), "unsafe table or column name") {
		t.Fatalf("expected latched step error, got %v", err)
	}

	var record Record
	if queryErr := db.Where("version = ?", migration.Version).First(&record).Error; queryErr != nil {
		t.Fatal(queryErr)
	}
	if record.Status != "failed" {
		t.Fatalf("ignored mutation error recorded migration as %q, want failed", record.Status)
	}
}

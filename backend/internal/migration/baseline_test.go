package migration

import (
	"path/filepath"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet/installation/journal"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEnsureDesktopPetInstallationOperationColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "baseline.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE desktop_pet_installation_operations (id TEXT PRIMARY KEY, operation_type TEXT, space_id TEXT, device_id TEXT, installation_id TEXT, pet_id TEXT, target_release_id TEXT, idempotency_key TEXT, request_hash TEXT, stage TEXT, status TEXT, attempt_number INTEGER, lease_owner TEXT, lease_expires_at TEXT, error_code TEXT, error_message TEXT, created_at TEXT, started_at TEXT, updated_at TEXT, completed_at TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE desktop_pet_installation_commit_journals (id TEXT PRIMARY KEY, operation_id TEXT, installation_id TEXT, space_id TEXT, device_id TEXT, target_release_id TEXT, staging_path_key TEXT, published_path_key TEXT, trash_path_key TEXT, error_code TEXT, error_message TEXT, created_at TEXT, updated_at TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE desktop_pet_installation_switch_journals (id TEXT PRIMARY KEY, operation_id TEXT, space_id TEXT, device_id TEXT, old_installation_id TEXT, new_installation_id TEXT, old_desired_revision INTEGER, new_desired_revision INTEGER, created_at TEXT, updated_at TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := ensureDesktopPetInstallationOperationColumns(db); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runtime_id", "source_release_id", "execution_id", "heartbeat_at", "desired_revision"} {
		if !db.Migrator().HasColumn("desktop_pet_installation_operations", name) {
			t.Fatalf("missing migrated column %s", name)
		}
	}
	for table, names := range map[string][]string{
		"desktop_pet_installation_commit_journals": {"runtime_id", "pet_id", "source_release_id", "stage", "status", "execution_id", "expected_old_status", "rollback_path_key"},
		"desktop_pet_installation_switch_journals": {"runtime_id", "old_binding_revision", "new_binding_revision", "old_release_id", "new_release_id", "stage", "status", "execution_id", "expected_old_stage", "error_code", "error_message"},
	} {
		for _, name := range names {
			if !db.Migrator().HasColumn(table, name) {
				t.Fatalf("missing migrated column %s.%s", table, name)
			}
		}
	}
	if err := ensureDesktopPetInstallationOperationColumns(db); err != nil {
		t.Fatalf("migration must be idempotent: %v", err)
	}
}

func TestBaselineSupportsDesktopPetInstallationPersistence(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "baseline.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyBaseline(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&operation.InstallationOperation{
		ID:              "operation",
		OperationType:   "install",
		SpaceID:         "space",
		DeviceID:        "device",
		RuntimeID:       "runtime",
		InstallationID:  "installation",
		PetID:           "pet",
		SourceReleaseID: "source",
		TargetReleaseID: "target",
		IdempotencyKey:  "operation",
		RequestHash:     "hash",
		Status:          "created",
		Stage:           "created",
		AttemptNumber:   1,
		ExecutionID:     "execution",
		LeaseOwner:      "owner",
		LeaseExpiresAt:  "2030-01-01T00:00:00Z",
		HeartbeatAt:     "2030-01-01T00:00:00Z",
		DesiredRevision: 1,
		CreatedAt:       "2030-01-01T00:00:00Z",
		UpdatedAt:       "2030-01-01T00:00:00Z",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&journal.InstallationCommitJournal{
		ID:                "commit",
		OperationID:       "operation",
		SpaceID:           "space",
		DeviceID:          "device",
		RuntimeID:         "runtime",
		InstallationID:    "installation",
		PetID:             "pet",
		SourceReleaseID:   "source",
		TargetReleaseID:   "target",
		Stage:             journal.JournalStageReleaseVerified,
		Status:            "active",
		ExecutionID:       "execution",
		ExpectedOldStatus: "installed",
		StagingPathKey:    "staging",
		RollbackPathKey:   "rollback",
		PublishedPathKey:  "published",
		TrashPathKey:      "trash",
		CreatedAt:         "2030-01-01T00:00:00Z",
		UpdatedAt:         "2030-01-01T00:00:00Z",
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&journal.InstallationSwitchJournal{
		ID:                 "switch",
		OperationID:        "operation",
		SpaceID:            "space",
		DeviceID:           "device",
		RuntimeID:          "runtime",
		OldInstallationID:  "old",
		NewInstallationID:  "new",
		OldBindingRevision: 1,
		NewBindingRevision: 2,
		OldDesiredRevision: 1,
		NewDesiredRevision: 2,
		OldReleaseID:       "source",
		NewReleaseID:       "target",
		Stage:              journal.SwitchJournalCreated,
		Status:             "active",
		ExecutionID:        "execution",
		ExpectedOldStage:   "completed",
		CreatedAt:          "2030-01-01T00:00:00Z",
		UpdatedAt:          "2030-01-01T00:00:00Z",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

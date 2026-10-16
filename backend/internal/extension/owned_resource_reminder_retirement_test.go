package extension

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOwnedResourcesRetireReminderLedger(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "owned.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.Exec(`CREATE TABLE extension_owned_resources (
id TEXT PRIMARY KEY, extension_id TEXT NOT NULL, extension_version TEXT NOT NULL DEFAULT '',
resource_type TEXT NOT NULL, resource_id TEXT NOT NULL, owner_scope_type TEXT NOT NULL DEFAULT 'global',
owner_scope_id TEXT NOT NULL DEFAULT '', source_run_id TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'active',
cleanup_attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL,
updated_at TEXT NOT NULL, cleaned_at TEXT NOT NULL DEFAULT '', UNIQUE(extension_id, resource_type, resource_id))`).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	if err := repository.RegisterOwnedSideEffects(t.Context(), ExecutionScope{ExtensionID: "plugin", CharacterID: "role"}, []SideEffectRecord{{Type: "schedule_create", TargetID: "kernel-task", Confirmed: true}, {Type: "message_create", TargetID: "message", Confirmed: true}}); err != nil {
		t.Fatal(err)
	}
	var records []ownedResourceRecord
	if err := db.Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ResourceType != "message_create" {
		t.Fatalf("task created a second ownership ledger: %#v", records)
	}
	legacy := ownedResourceRecord{ID: "legacy", ExtensionID: "plugin", OwnerScopeType: "character", OwnerScopeID: "role", ResourceType: "schedule_create", ResourceID: "old-task", Status: "cleanup_failed", CreatedAt: "now", UpdatedAt: "now"}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.CleanupOwnedResources(t.Context(), "plugin", "character", "role"); err != nil {
		t.Fatal(err)
	}
	repository.RetryOwnedResourceCleanup(t.Context())
	if err := db.Find(&records).Error; err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.ResourceType == "schedule_create" && (record.Status != "cleanup_failed" || record.CleanupAttempts != 0) {
			t.Fatalf("retired task was marked cleaned without kernel cancellation: %#v", record)
		}
		if record.ResourceType == "message_create" && (record.Status != "retained" || record.CleanupAttempts != 1) {
			t.Fatalf("unrelated owned resource behavior changed: %#v", record)
		}
	}
}

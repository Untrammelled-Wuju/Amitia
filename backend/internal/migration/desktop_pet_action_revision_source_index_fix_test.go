package migration

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDesktopPetActionRevisionSourceIndexFixAllowsDescendantVersions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:action-revision-source-index-fix?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE desktop_pet_action_revisions (
		id TEXT PRIMARY KEY,
		source_processing_revision_id TEXT NOT NULL DEFAULT '',
		source_type TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatalf("create action revisions: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX uq_dpar_source_type
		ON desktop_pet_action_revisions(source_processing_revision_id, source_type)`).Error; err != nil {
		t.Fatalf("create historical unique index: %v", err)
	}

	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]Migration{DesktopPetActionRevisionSourceIndexFixMigration()}); err != nil {
		t.Fatalf("apply source index fix: %v", err)
	}

	insert := func(id, sourceRevisionID, sourceType string) error {
		return db.Exec(`INSERT INTO desktop_pet_action_revisions(id, source_processing_revision_id, source_type)
			VALUES (?, ?, ?)`, id, sourceRevisionID, sourceType).Error
	}
	if err := insert("baseline-1", "proc-rev-1", "processing_baseline"); err != nil {
		t.Fatalf("insert processing baseline: %v", err)
	}
	if err := insert("baseline-duplicate", "proc-rev-1", "processing_baseline"); err == nil {
		t.Fatal("processing baseline uniqueness is not enforced")
	}
	if err := insert("manual-1", "proc-rev-1", "manual_edit"); err != nil {
		t.Fatalf("insert first manual edit: %v", err)
	}
	if err := insert("manual-2", "proc-rev-1", "manual_edit"); err != nil {
		t.Fatalf("second manual edit must be allowed: %v", err)
	}
	if err := insert("regen-1", "proc-rev-1", "full_action_regeneration"); err != nil {
		t.Fatalf("insert first full action regeneration: %v", err)
	}
	if err := insert("regen-2", "proc-rev-1", "full_action_regeneration"); err != nil {
		t.Fatalf("second full action regeneration must be allowed: %v", err)
	}
	if err := insert("manual-empty-1", "", "manual_edit"); err != nil {
		t.Fatalf("insert manual edit without processing lineage: %v", err)
	}
	if err := insert("manual-empty-2", "", "manual_edit"); err != nil {
		t.Fatalf("multiple manual edits without processing lineage must be allowed: %v", err)
	}
}

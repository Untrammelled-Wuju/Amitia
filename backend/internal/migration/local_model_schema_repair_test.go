package migration

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLocalModelLibrarySchemaRepairRestoresMissingTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "local-models.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]Migration{LocalModelLibrarySchemaRepairMigration()}); err != nil {
		t.Fatalf("repair schema: %v", err)
	}
	for _, table := range []string{"model_artifacts", "model_artifact_references"} {
		exists, err := hasTable(db, table)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("repair did not create %s", table)
		}
	}
	for _, index := range []string{"idx_model_artifacts_owner_sha256", "idx_model_artifact_refs_config"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", index).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("repair did not create index %s", index)
		}
	}
}

func TestBaselineIncludesLocalModelLibrary(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "baseline.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyBaseline(db); err != nil {
		t.Fatalf("apply baseline: %v", err)
	}
	for _, table := range []string{"model_artifacts", "model_artifact_references"} {
		exists, err := hasTable(db, table)
		if err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("baseline is missing %s", table)
		}
	}
}

func hasTable(db *gorm.DB, table string) (bool, error) {
	var count int64
	err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count).Error
	return count > 0, err
}

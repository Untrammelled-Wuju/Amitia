package migration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestArchiveLegacyMetadataPreservesRowsAndClearsLegacyTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE legacy_mcp_metadata (id TEXT PRIMARY KEY, name TEXT, config TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO legacy_mcp_metadata (id, name, config) VALUES ('m1', 'demo', '{"enabled":true}'), ('m2', 'other', 'x')`).Error; err != nil {
		t.Fatal(err)
	}

	migrated, err := ArchiveLegacyMetadata(context.Background(), db, "cutover-1")
	if err != nil {
		t.Fatal(err)
	}
	if migrated["legacy_mcp_metadata"] != 2 {
		t.Fatalf("migrated rows = %d, want 2", migrated["legacy_mcp_metadata"])
	}
	var remaining int64
	if err := db.Table("legacy_mcp_metadata").Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("remaining legacy rows = %d, want 0", remaining)
	}

	var archived []LegacyMetadataArchive
	if err := db.Order("id ASC").Find(&archived).Error; err != nil {
		t.Fatal(err)
	}
	if len(archived) != 2 {
		t.Fatalf("archive rows = %d, want 2", len(archived))
	}
	seen := map[string]bool{}
	for _, row := range archived {
		if row.OperationID != "cutover-1" || row.SourceTable != "legacy_mcp_metadata" {
			t.Fatalf("unexpected archive metadata: %#v", row)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(row.PayloadJSON), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		seen[payload["id"].(string)] = true
	}
	if !seen["m1"] || !seen["m2"] {
		t.Fatalf("archive missing rows: %#v", seen)
	}
}

func TestArchiveLegacyMetadataIsIdempotentAfterLegacyRowsCleared(t *testing.T) {
	// SQLite values used by the supported metadata schema are JSON encodable;
	// verify the operation itself remains idempotent once legacy tables are empty.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE legacy_skill_metadata (id TEXT PRIMARY KEY, name TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO legacy_skill_metadata (id, name) VALUES ('s1', 'skill')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ArchiveLegacyMetadata(context.Background(), db, "cutover-2"); err != nil {
		t.Fatal(err)
	}
	migrated, err := ArchiveLegacyMetadata(context.Background(), db, "cutover-2")
	if err != nil {
		t.Fatal(err)
	}
	if len(migrated) != 0 {
		t.Fatalf("second pass migrated unexpected rows: %#v", migrated)
	}
	var archived int64
	if err := db.Model(&LegacyMetadataArchive{}).Where("source_table = ?", "legacy_skill_metadata").Count(&archived).Error; err != nil {
		t.Fatal(err)
	}
	if archived != 1 {
		t.Fatalf("archive rows after retry = %d, want 1", archived)
	}
}

func TestArchiveLegacyMetadataDoesNotClearLegacyRowsWhenArchiveWriteFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&LegacyMetadataArchive{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE legacy_plugin_metadata (id TEXT PRIMARY KEY, name TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO legacy_plugin_metadata (id, name) VALUES ('p1', 'plugin')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER fail_legacy_archive BEFORE INSERT ON cutover_legacy_metadata_archive BEGIN SELECT RAISE(FAIL, 'archive blocked'); END;`).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := ArchiveLegacyMetadata(context.Background(), db, "cutover-fail"); err == nil {
		t.Fatal("expected archive failure")
	}
	var remaining int64
	if err := db.Table("legacy_plugin_metadata").Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining != 1 {
		t.Fatalf("legacy rows after failed archive = %d, want 1", remaining)
	}
	var archived int64
	if err := db.Model(&LegacyMetadataArchive{}).Count(&archived).Error; err != nil {
		t.Fatal(err)
	}
	if archived != 0 {
		t.Fatalf("archive rows after failed transaction = %d, want 0", archived)
	}
}

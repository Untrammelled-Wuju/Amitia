package migration

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTemporalRelationshipPresenceUniqueIndexMigrationDeduplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "relationship-presence.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Exec(`CREATE TABLE temporal_relationship_presence_states (
		id TEXT PRIMARY KEY,
		space_id TEXT NOT NULL DEFAULT '',
		character_id TEXT NOT NULL DEFAULT '',
		updated_at_utc TEXT NOT NULL DEFAULT '',
		state_version INTEGER NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO temporal_relationship_presence_states
		(id, space_id, character_id, updated_at_utc, state_version)
		VALUES
		('old', 'space-1', 'char-1', '2026-09-22T00:00:00Z', 1),
		('new', 'space-1', 'char-1', '2026-09-23T00:00:00Z', 2)`).Error; err != nil {
		t.Fatal(err)
	}

	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{TemporalRelationshipPresenceUniqueIndexMigration()}); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := db.Table("temporal_relationship_presence_states").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
	var id string
	if err := db.Table("temporal_relationship_presence_states").Select("id").Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	if id != "new" {
		t.Fatalf("kept row = %q, want new", id)
	}
	if err := db.Exec(`INSERT INTO temporal_relationship_presence_states
		(id, space_id, character_id, updated_at_utc, state_version)
		VALUES ('duplicate', 'space-1', 'char-1', '2026-09-24T00:00:00Z', 3)`).Error; err == nil {
		t.Fatal("expected unique index violation")
	}
}

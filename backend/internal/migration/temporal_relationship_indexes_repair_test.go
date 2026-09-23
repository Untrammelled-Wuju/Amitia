package migration

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTemporalRelationshipIndexesRepairMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "temporal-indexes.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if err := db.Exec(`CREATE TABLE temporal_cadence_samples (
		id TEXT PRIMARY KEY,
		interaction_id TEXT NOT NULL DEFAULT '',
		sample_kind TEXT NOT NULL DEFAULT '',
		created_at_utc TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE temporal_reunion_episodes (
		id TEXT PRIMARY KEY,
		idempotency_key TEXT NOT NULL DEFAULT '',
		updated_at_utc TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE temporal_interaction_receipts (
		id TEXT PRIMARY KEY,
		space_id TEXT NOT NULL DEFAULT '',
		request_id TEXT NOT NULL DEFAULT '',
		interaction_id TEXT NOT NULL DEFAULT '',
		updated_at_utc TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE temporal_effect_ledger (
		id TEXT PRIMARY KEY,
		effect_key TEXT NOT NULL DEFAULT '',
		applied_at_utc TEXT NOT NULL DEFAULT ''
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO temporal_cadence_samples(id, interaction_id, sample_kind, created_at_utc)
		VALUES ('cad-1', 'interaction-1', 'global', '2026-09-22T00:00:00Z'),
		('cad-2', 'interaction-1', 'global', '2026-09-23T00:00:00Z')`).Error; err != nil {
		t.Fatal(err)
	}

	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{TemporalRelationshipIndexesRepairMigration()}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("temporal_cadence_samples").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("cadence row count = %d, want 1", count)
	}
	for _, name := range []string{"idx_temporal_cadence_interaction", "idx_temporal_reunion_idempotency", "idx_temporal_receipt_request", "idx_temporal_receipt_interaction", "idx_temporal_effect_key"} {
		var indexCount int64
		if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", name).Scan(&indexCount).Error; err != nil {
			t.Fatal(err)
		}
		if indexCount != 1 {
			t.Fatalf("index %s was not created", name)
		}
	}
}

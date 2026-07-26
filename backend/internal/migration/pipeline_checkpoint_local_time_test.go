package migration

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPipelineCheckpointLocalTimeMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec("CREATE TABLE pipeline_checkpoints (conversation_id TEXT, pipeline_type TEXT, created_at TEXT, updated_at TEXT, lease_expires_at TEXT)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	value := "2026-07-26 10:49:36"
	if err := db.Exec("INSERT INTO pipeline_checkpoints VALUES (?, ?, ?, ?, ?)", "conv", "memory", value, value, value).Error; err != nil {
		t.Fatalf("insert row: %v", err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{PipelineCheckpointLocalTimeMigration()}); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	var updatedAt string
	if err := db.Table("pipeline_checkpoints").Select("updated_at").Row().Scan(&updatedAt); err != nil {
		t.Fatalf("read row: %v", err)
	}
	parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC)
	if err != nil {
		t.Fatalf("parse expected time: %v", err)
	}
	expected := parsed.Local().Format("2006-01-02 15:04:05")
	if updatedAt != expected {
		t.Fatalf("updated_at = %s, want %s", updatedAt, expected)
	}
}

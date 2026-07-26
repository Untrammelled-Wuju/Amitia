package migration

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestEpisodicMessageTimeMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec("CREATE TABLE episodic_memories (id TEXT PRIMARY KEY)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{EpisodicMessageTimeMigration()}); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	for _, column := range []string{"message_time_start", "message_time_end"} {
		if !db.Migrator().HasColumn("episodic_memories", column) {
			t.Fatalf("missing column %s", column)
		}
	}
}

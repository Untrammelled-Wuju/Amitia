package migration

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestEmotionStateMigrationCreatesTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "emotion-migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{EmotionStateMigration()}); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	if !db.Migrator().HasTable("emotion_states") {
		t.Fatal("emotion_states table was not created")
	}
	if !db.Migrator().HasIndex("emotion_states", "idx_emotion_states_character") {
		t.Fatal("emotion_states character index was not created")
	}
}

package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRetireLegacyMemoryDiscardsOldRecordsWithoutTouchingOtherData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "old.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, statement := range []string{`CREATE TABLE memories(id TEXT PRIMARY KEY)`, `CREATE TABLE user_profiles(id TEXT PRIMARY KEY)`, `CREATE TABLE messages(id TEXT PRIMARY KEY)`, `INSERT INTO memories VALUES('old')`, `INSERT INTO user_profiles VALUES('old-profile')`, `INSERT INTO messages VALUES('message')`} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	runner := Runner{DB: db, BackupDir: t.TempDir(), SkipBackup: true}
	if err := runner.Apply([]Migration{RetireLegacyMemoryDataMigration()}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"memories", "user_profiles"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("%s retained old data: %d %v", table, count, err)
		}
	}
	var messages int64
	_ = db.Table("messages").Count(&messages).Error
	if messages != 1 {
		t.Fatal("retirement deleted chat records")
	}
	if err := runner.Apply([]Migration{RetireLegacyMemoryDataMigration()}); err != nil {
		t.Fatal(err)
	}
}

package migration

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestRemoveGlobalMemoryStorageDropsLegacySchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "memory.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.Exec(`CREATE TABLE characters(id TEXT PRIMARY KEY,global_memory_enabled INTEGER NOT NULL DEFAULT 1); CREATE TABLE memory_scope_transfers(id TEXT PRIMARY KEY)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{RemoveGlobalMemoryStorageMigration()}); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable("memory_scope_transfers") || db.Migrator().HasColumn("characters", "global_memory_enabled") {
		t.Fatal("legacy global memory schema remains")
	}
}

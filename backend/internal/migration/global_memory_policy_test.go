package migration

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestGlobalMemoryPolicyMigrationPreservesLegacyPartitions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "memory.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err := db.Exec(`CREATE TABLE characters(id TEXT PRIMARY KEY); INSERT INTO characters(id) VALUES('old'); CREATE TABLE memories(id TEXT PRIMARY KEY,character_id TEXT,scope TEXT); INSERT INTO memories VALUES('private','old','character')`).Error; err != nil {
		t.Fatal(err)
	}
	runner := Runner{DB: db, SkipBackup: true}
	migration := GlobalMemoryPolicyMigration()
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}
	var enabled int
	if err := db.Raw("SELECT global_memory_enabled FROM characters WHERE id='old'").Scan(&enabled).Error; err != nil || enabled != 1 {
		t.Fatalf("default %d %v", enabled, err)
	}
	var scope string
	db.Raw("SELECT scope FROM memories WHERE id='private'").Scan(&scope)
	if scope != "character" {
		t.Fatal("migration moved existing memories")
	}
	if err := db.Exec("UPDATE characters SET global_memory_enabled=0").Error; err != nil {
		t.Fatal(err)
	}
	if err := runner.Apply([]Migration{migration}); err != nil {
		t.Fatal(err)
	}
	db.Raw("SELECT global_memory_enabled FROM characters WHERE id='old'").Scan(&enabled)
	if enabled != 0 {
		t.Fatal("replay reset user choice")
	}
	fresh, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "fresh.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	freshSQL, _ := fresh.DB()
	defer freshSQL.Close()
	if err := ApplyBaseline(fresh); err != nil {
		t.Fatal(err)
	}
	if !fresh.Migrator().HasTable("memory_scope_transfers") || !fresh.Migrator().HasColumn("characters", "global_memory_enabled") {
		t.Fatal("baseline missing memory scope schema")
	}
}

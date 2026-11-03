package migration

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentPlatformMigrationCreatesOwnedTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	step := &Step{db: db}
	if err := AgentPlatformMigration().Up(step); err != nil {
		t.Fatal(err)
	}
	tables, err := db.Migrator().GetTables()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("tables=%v", tables)
	for _, table := range []string{
		"agent_profiles",
		"agent_revisions",
		"agent_releases",
		"teams",
		"approval_records",
		"reconciliation_cases",
		"runtime_instances",
		"marketplace_listings",
		"a2a_agent_cards",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing table %s", table)
		}
	}
}

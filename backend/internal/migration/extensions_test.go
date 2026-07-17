package migration

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestExtensionsMigrationOnEmptyAndExistingDatabase(t *testing.T) {
	for _, existing := range []bool{false, true} {
		db, err := gorm.Open(sqlite.Open("file:"+t.Name()+map[bool]string{false: "-empty", true: "-existing"}[existing]+"?mode=memory&cache=shared"), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		if existing {
			if err := db.Exec("CREATE TABLE existing_data (id TEXT PRIMARY KEY)").Error; err != nil {
				t.Fatal(err)
			}
		}
		runner := Runner{DB: db, SkipBackup: true}
		if err := runner.Apply([]Migration{ExtensionsMigration(), PluginRuntimeMigration()}); err != nil {
			t.Fatal(err)
		}
		if err := runner.Apply([]Migration{ExtensionsMigration(), PluginRuntimeMigration()}); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"extensions", "extension_versions", "extension_capability_grants", "extension_configs", "extension_runs"} {
			if !db.Migrator().HasTable(table) {
				t.Fatalf("missing table %s", table)
			}
		}
		for _, table := range []string{"extension_states", "extension_state_revisions", "extension_events", "extension_event_deliveries", "extension_schedules", "extension_plugin_runs", "extension_audits"} {
			if !db.Migrator().HasTable(table) {
				t.Fatalf("missing plugin runtime table %s", table)
			}
		}
		for _, column := range []string{"lifecycle_status", "health_status", "last_error_code", "enabled_at", "disabled_at"} {
			if !db.Migrator().HasColumn("extensions", column) {
				t.Fatalf("missing plugin runtime column %s", column)
			}
		}
		if existing && !db.Migrator().HasTable("existing_data") {
			t.Fatal("existing table was removed")
		}
	}
}

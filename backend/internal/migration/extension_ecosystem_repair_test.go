package migration

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestExtensionEcosystemRepairMigrationsEmptyExistingAndRerun(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "empty"
		if existing {
			name = "existing"
		}
		t.Run(name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			runner := Runner{DB: db, SkipBackup: true}
			if err := runner.Apply([]Migration{ExtensionsMigration(), ExtensionWorkshopMigration(), ExtensionPackagesMigration()}); err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("CREATE TABLE schedules (id TEXT PRIMARY KEY, title TEXT, status TEXT)").Error; err != nil {
				t.Fatal(err)
			}
			if existing {
				if err := db.Exec("INSERT INTO extensions (id, extension_id, kind, name, current_version, source, enabled, manifest_json, normalized_manifest_json, created_at, updated_at, archived_at) VALUES ('1', 'dev.test.scope', 'Skill', 'scope', '1.0.0', 'workflow', 1, '{}', '{}', 'now', 'now', '')").Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Exec("INSERT INTO extensions (id, extension_id, kind, name, current_version, source, enabled, manifest_json, normalized_manifest_json, owner_user_id, scope_type, scope_id, created_at, updated_at, archived_at) VALUES ('2', 'dev.test.character', 'Skill', 'character', '1.0.0', 'workflow', 1, '{}', '{}', '1', 'character', 'char-a', 'now', 'now', '')").Error; err != nil {
					t.Fatal(err)
				}
			}
			migrations := []Migration{ExtensionScopeBindingsMigration(), ExtensionOwnedResourcesMigration(), ExtensionPackageRecoveryMigration(), ExtensionArtifactRecoveryMigration(), ExtensionScheduleSourceMigration()}
			if err := runner.Apply(migrations); err != nil {
				t.Fatal(err)
			}
			if err := runner.Apply(migrations); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"extension_scope_bindings", "extension_owned_resources"} {
				if !db.Migrator().HasTable(table) {
					t.Fatalf("missing table %s", table)
				}
			}
			for _, column := range []string{"artifact_status", "activation_status", "operation_id", "failure_code"} {
				if !db.Migrator().HasColumn("extension_versions", column) {
					t.Fatalf("missing extension_versions.%s", column)
				}
			}
			for _, column := range []string{"artifact_status", "operation_id"} {
				if !db.Migrator().HasColumn("extension_artifacts", column) {
					t.Fatalf("missing extension_artifacts.%s", column)
				}
			}
			if existing {
				var enabled int
				if err := db.Table("extension_scope_bindings").Select("enabled").Where("extension_id = ? AND scope_type = 'global' AND scope_id = ''", "dev.test.scope").Scan(&enabled).Error; err != nil || enabled != 1 {
					t.Fatalf("global binding backfill failed: %d %v", enabled, err)
				}
				if err := db.Table("extension_scope_bindings").Select("enabled").Where("extension_id = ? AND scope_type = 'character' AND scope_id = 'char-a'", "dev.test.character").Scan(&enabled).Error; err != nil || enabled != 1 {
					t.Fatalf("character binding backfill failed: %d %v", enabled, err)
				}
				for _, column := range []string{"source_extension_id", "source_extension_version", "source_run_id", "owner_scope_type", "owner_scope_id"} {
					if !db.Migrator().HasColumn("schedules", column) {
						t.Fatalf("missing schedules.%s", column)
					}
				}
			}
		})
	}
}

func TestExtensionScheduleOwnershipRepairCreatesMissingTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{DB: db, SkipBackup: true}
	if err := runner.Apply([]Migration{ExtensionScheduleOwnershipRepairMigration()}); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"source_type", "source_extension_id", "source_extension_version", "source_run_id", "owner_scope_type", "owner_scope_id"} {
		if !db.Migrator().HasColumn("schedules", column) {
			t.Fatalf("missing schedules.%s", column)
		}
	}
	if err := db.Exec("INSERT INTO schedules (id, title, due_time, created_at, updated_at) VALUES ('one', 'test', 'now', 'now', 'now')").Error; err != nil {
		t.Fatal(err)
	}
	var sourceType string
	if err := db.Table("schedules").Select("source_type").Where("id = 'one'").Scan(&sourceType).Error; err != nil || sourceType != "user" {
		t.Fatalf("unexpected source type: %q %v", sourceType, err)
	}
}

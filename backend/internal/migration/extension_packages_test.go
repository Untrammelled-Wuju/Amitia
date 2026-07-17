package migration

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestExtensionPackagesMigrationEmptyAndExistingDatabase(t *testing.T) {
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
			base := []Migration{ExtensionsMigration(), PluginRuntimeMigration(), ExtensionWorkshopMigration(), ExtensionAgentSkillsMigration(), ExtensionAgentSkillTraceMigration()}
			if err := runner.Apply(base); err != nil {
				t.Fatal(err)
			}
			if existing {
				if err := db.Exec(`INSERT INTO extensions (id, extension_id, kind, name, current_version, source, enabled, manifest_json, normalized_manifest_json, created_at, updated_at, archived_at) VALUES ('1','dev.existing','Skill','existing','1.0.0','workflow',0,'{}','{}','','','')`).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := runner.Apply([]Migration{ExtensionPackagesMigration()}); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"extension_package_import_sessions", "extension_package_installations", "extension_package_signers", "extension_version_dependencies", "extension_package_exports"} {
				if !db.Migrator().HasTable(table) {
					t.Fatalf("missing table %s", table)
				}
			}
			for _, column := range []string{"artifact_id", "package_hash", "package_blob", "signature_status"} {
				if !db.Migrator().HasColumn("extension_versions", column) {
					t.Fatalf("missing version column %s", column)
				}
			}
			if existing {
				var count int64
				if err := db.Table("extensions").Where("extension_id = ?", "dev.existing").Count(&count).Error; err != nil || count != 1 {
					t.Fatalf("existing data changed: %d %v", count, err)
				}
			}
		})
	}
}

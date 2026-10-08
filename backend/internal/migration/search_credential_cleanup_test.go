package migration

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestSearchCredentialCleanupMigrationRegisteredWithSameBaseline(t *testing.T) {
	count := 0
	for _, migration := range DefaultMigrations() {
		if migration.Version == "20261007004" {
			count++
			if migration.Name != "search_credential_cleanup_outbox" {
				t.Fatal("unexpected migration identity")
			}
		}
	}
	if count != 1 {
		t.Fatalf("cleanup migration registered %d times", count)
	}
	step := &Step{}
	if err := SearchCredentialCleanupMigration().Up(step); err != nil {
		t.Fatal(err)
	}
	if len(step.commands) != 1 || !strings.Contains(step.commands[0], "CREATE TABLE IF NOT EXISTS search_credential_cleanup") {
		t.Fatal("cleanup migration did not declare its table")
	}
	baseline := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS search_credential_cleanup\s*\(.*?\);`).FindString(baselineSQL)
	if baseline == "" {
		t.Fatal("fresh installation baseline missing cleanup outbox")
	}
	for _, source := range []struct{ name, sql string }{{"upgrade", step.commands[0]}, {"fresh", baseline}} {
		t.Run(source.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cleanup.db")), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			defer raw.Close()
			if err := db.Exec(source.sql).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(source.sql).Error; err != nil {
				t.Fatal("migration not idempotent", err)
			}
			if err := db.Exec(`INSERT INTO search_credential_cleanup(secret_ref,engine_id,created_at) VALUES('secret://fixture/1','serper',CURRENT_TIMESTAMP)`).Error; err != nil {
				t.Fatal(err)
			}
			var row struct {
				Attempts  int
				LastError string
			}
			if err := db.Table("search_credential_cleanup").Select("attempts,last_error").First(&row).Error; err != nil || row.Attempts != 0 || row.LastError != "" {
				t.Fatal("schema defaults invalid", err)
			}
		})
	}
}

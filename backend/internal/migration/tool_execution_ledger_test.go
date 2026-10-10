package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestToolExecutionLedgerFreshBaselineAndLegacyUpgrade(t *testing.T) {
	open := func(name string) *gorm.DB {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), name)), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = sqlDB.Close() })
		return db
	}
	t.Run("fresh_install", func(t *testing.T) {
		db := open("fresh.sqlite")
		if err := ApplyInitialSQL(db, baselineSQL); err != nil {
			t.Fatal(err)
		}
		for _, column := range []string{"attempt_id", "turn_id", "execution_id",
			"input_hash", "owner_instance_id", "result_ref", "error_class",
			"started_at", "finished_at"} {
			if !db.Migrator().HasColumn("tool_call_intents", column) {
				t.Fatalf("fresh baseline missing intent field %s", column)
			}
		}
		if !db.Migrator().HasColumn("tool_call_results", "attempt_id") {
			t.Fatal("fresh baseline missing result attempt ID")
		}
	})
	t.Run("existing_install", func(t *testing.T) {
		db := open("legacy.sqlite")
		for _, statement := range []string{
			`CREATE TABLE tool_call_intents (id TEXT PRIMARY KEY, status TEXT, tool_call_id TEXT, created_at TEXT)`,
			`CREATE TABLE tool_call_results (id TEXT PRIMARY KEY, status TEXT, created_at TEXT)`,
			`INSERT INTO tool_call_intents (id, status) VALUES ('legacy', 'SUCCESS')`,
		} {
			if err := db.Exec(statement).Error; err != nil {
				t.Fatal(err)
			}
		}
		if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{ToolExecutionLedgerMigration()}); err != nil {
			t.Fatal(err)
		}
		if err := (Runner{DB: db, SkipBackup: true}).Apply([]Migration{ToolExecutionLedgerMigration()}); err != nil {
			t.Fatal(err)
		}
		for _, column := range []string{"attempt_id", "turn_id", "execution_id", "input_hash",
			"owner_instance_id", "result_ref", "error_class", "started_at", "finished_at"} {
			if !db.Migrator().HasColumn("tool_call_intents", column) {
				t.Fatalf("legacy migration missing intent field %s", column)
			}
		}
		for _, column := range []string{"attempt_id", "turn_id", "execution_id", "input_hash"} {
			if !db.Migrator().HasColumn("tool_call_results", column) {
				t.Fatalf("legacy migration missing result field %s", column)
			}
		}
		var status string
		if err := db.Table("tool_call_intents").Select("status").Where("id = ?", "legacy").Scan(&status).Error; err != nil || status != "SUCCESS" {
			t.Fatalf("migration damaged legacy intent: %q err=%v", status, err)
		}
	})
}

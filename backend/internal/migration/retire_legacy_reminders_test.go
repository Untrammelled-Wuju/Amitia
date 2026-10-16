package migration

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRetireLegacyReminderDiscardsOnlyRetiredBusinessData(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reminders.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, table := range []string{"schedules", "reminders", "trigger_histories", "messages", "extension_schedule_definitions", "extension_schedule_triggers", "plugin_tasks"} {
		if err := db.Exec("CREATE TABLE " + table + "(id TEXT PRIMARY KEY)").Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO " + table + " VALUES('record')").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec(`CREATE TABLE extension_owned_resources(id TEXT PRIMARY KEY, resource_type TEXT); INSERT INTO extension_owned_resources VALUES('old','schedule_create'),('keep','other')`).Error; err != nil {
		t.Fatal(err)
	}
	runner := Runner{DB: db, BackupDir: t.TempDir(), SkipBackup: true}
	for repeat := 0; repeat < 2; repeat++ {
		if err := runner.Apply([]Migration{RetireLegacyReminderDataMigration()}); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"schedules", "reminders", "trigger_histories", "messages", "extension_schedule_definitions", "extension_schedule_triggers", "plugin_tasks", "extension_owned_resources"} {
			var count int64
			if err := db.Table(table).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			expected := int64(1)
			if table == "schedules" || table == "reminders" || table == "trigger_histories" {
				expected = 0
			}
			if count != expected {
				t.Fatalf("%s count=%d expected=%d", table, count, expected)
			}
		}
	}
}

func TestRetireLegacyReminderHandlesMissingHistoricalTables(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "empty.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	runner := Runner{DB: db, BackupDir: t.TempDir(), SkipBackup: true}
	if err := runner.Apply([]Migration{RetireLegacyReminderDataMigration()}); err != nil {
		t.Fatal(err)
	}
}

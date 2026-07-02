package migration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRunnerCreatesPreMigrationSQLiteBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "app.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	if err := db.Exec("CREATE TABLE existing_data (id TEXT PRIMARY KEY, value TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO existing_data (id, value) VALUES ('one', 'before')").Error; err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(dir, "backups")
	runner := Runner{
		DB:        db,
		BackupDir: backupDir,
		Now: func() time.Time {
			return time.Date(2026, 7, 2, 10, 0, 0, 0, time.UTC)
		},
	}
	migrations := []Migration{
		{
			Version: "test-001",
			Name:    "create_new_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE IF NOT EXISTS new_table (id TEXT PRIMARY KEY)")
				return nil
			},
		},
	}

	if err := runner.Apply(migrations); err != nil {
		t.Fatal(err)
	}

	var records []struct {
		ID         string `gorm:"column:id"`
		BackupPath string `gorm:"column:backup_path"`
		BackupSize int64  `gorm:"column:backup_size"`
		Checksum   string `gorm:"column:checksum"`
		Status     string `gorm:"column:status"`
	}
	if err := db.Raw("SELECT id, backup_path, backup_size, checksum, status FROM backup_records").Scan(&records).Error; err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one backup record, got %d", len(records))
	}
	if records[0].Status != "completed" || records[0].BackupSize <= 0 || records[0].Checksum == "" {
		t.Fatalf("backup record was not completed: %#v", records[0])
	}
	if _, err := os.Stat(records[0].BackupPath); err != nil {
		t.Fatal(err)
	}

	backupDB, err := gorm.Open(sqlite.Open(records[0].BackupPath), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	backupSQL, err := backupDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer backupSQL.Close()
	var value string
	if err := backupDB.Raw("SELECT value FROM existing_data WHERE id = 'one'").Scan(&value).Error; err != nil {
		t.Fatal(err)
	}
	if value != "before" {
		t.Fatalf("backup did not preserve existing data: %q", value)
	}

	if err := runner.Apply(migrations); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM backup_records").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected no extra backup without pending migrations, got %d", count)
	}
}

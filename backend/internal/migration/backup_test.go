package migration

import (
	"encoding/json"
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
	var newTableCount int64
	if err := backupDB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'new_table'").Scan(&newTableCount).Error; err != nil {
		t.Fatal(err)
	}
	if newTableCount != 0 {
		t.Fatalf("backup was created after business migration")
	}
	manifestBytes, err := os.ReadFile(filepath.Join(backupDir, records[0].ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest backupManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ID != records[0].ID || manifest.BackupPath != records[0].BackupPath || len(manifest.Components) != 3 {
		t.Fatalf("backup manifest is not restorable metadata: %#v", manifest)
	}
	var contentRoles []string
	if err := db.Raw("SELECT table_name FROM backup_contents ORDER BY table_name").Scan(&contentRoles).Error; err != nil {
		t.Fatal(err)
	}
	if len(contentRoles) != 4 {
		t.Fatalf("expected main, metadata, wal and shm content records, got %#v", contentRoles)
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

func TestCopySQLiteBackupFilesIncludesExistingWALAndSHM(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "app.db")
	backupPath := filepath.Join(dir, "backup.db")
	if err := os.WriteFile(sourcePath, []byte("main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath+"-wal", []byte("wal"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath+"-shm", []byte("shm"), 0644); err != nil {
		t.Fatal(err)
	}

	components, err := copySQLiteBackupFiles(sourcePath, backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(components) != 3 {
		t.Fatalf("expected three components, got %d", len(components))
	}
	for _, item := range []struct {
		path string
		want string
	}{
		{backupPath, "main"},
		{backupPath + "-wal", "wal"},
		{backupPath + "-shm", "shm"},
	} {
		got, err := os.ReadFile(item.path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != item.want {
			t.Fatalf("unexpected backup content for %s: %q", item.path, string(got))
		}
	}
	for _, component := range components {
		if !component.SourceExists || component.Size <= 0 || component.Checksum == "" {
			t.Fatalf("component metadata not restorable: %#v", component)
		}
	}
}

func TestCopySQLiteBackupFilesRecordsMissingOptionalWALAndSHM(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "app.db")
	backupPath := filepath.Join(dir, "backup.db")
	if err := os.WriteFile(sourcePath, []byte("main"), 0644); err != nil {
		t.Fatal(err)
	}

	components, err := copySQLiteBackupFiles(sourcePath, backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(components) != 3 {
		t.Fatalf("expected three components, got %d", len(components))
	}
	if _, err := os.Stat(backupPath + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("missing wal should not create backup file: %v", err)
	}
	if _, err := os.Stat(backupPath + "-shm"); !os.IsNotExist(err) {
		t.Fatalf("missing shm should not create backup file: %v", err)
	}
	for _, component := range components {
		if component.Role == "wal" || component.Role == "shm" {
			if component.SourceExists {
				t.Fatalf("missing optional component marked present: %#v", component)
			}
		}
	}
}

func TestPreMigrationBackupFailureBlocksMigration(t *testing.T) {
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
	blockedBackupDir := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockedBackupDir, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}
	runner := Runner{
		DB:        db,
		BackupDir: blockedBackupDir,
		Now: func() time.Time {
			return time.Date(2026, 7, 2, 10, 30, 0, 0, time.UTC)
		},
	}
	err = runner.Apply([]Migration{
		{
			Version: "test-failure",
			Name:    "create_blocked_table",
			Up: func(s *Step) error {
				s.CreateTable("CREATE TABLE blocked_table (id TEXT PRIMARY KEY)")
				return nil
			},
		},
	})
	if err == nil {
		t.Fatal("expected backup failure")
	}
	var migrated int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'blocked_table'").Scan(&migrated).Error; err != nil {
		t.Fatal(err)
	}
	if migrated != 0 {
		t.Fatalf("migration ran after backup failure")
	}
	var status string
	if err := db.Raw("SELECT status FROM backup_records").Scan(&status).Error; err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("expected failed backup record, got %q", status)
	}
}

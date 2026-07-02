package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Migration struct {
	Version string
	Name    string
	Up      func(*Step) error
}

type Record struct {
	Version      string `gorm:"column:version;primaryKey"`
	Name         string `gorm:"column:name"`
	Checksum     string `gorm:"column:checksum"`
	Status       string `gorm:"column:status"`
	ErrorMessage string `gorm:"column:error_message"`
	StartedAt    string `gorm:"column:started_at"`
	FinishedAt   string `gorm:"column:finished_at"`
}

type Runner struct {
	DB        *gorm.DB
	Now       func() time.Time
	BackupDir string
}

type Step struct {
	db         *gorm.DB
	commands   []string
	operations []string
}

func (Record) TableName() string {
	return "schema_migrations"
}

func (r Runner) EnsureTable() error {
	if r.DB == nil {
		return errors.New("db is required")
	}
	return r.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
version TEXT PRIMARY KEY,
name TEXT NOT NULL DEFAULT '',
checksum TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
error_message TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL DEFAULT '',
finished_at TEXT NOT NULL DEFAULT ''
)`).Error
}

func (r Runner) Apply(migrations []Migration) error {
	if err := r.EnsureTable(); err != nil {
		return err
	}
	pending, err := r.hasPendingMigrations(migrations)
	if err != nil {
		return err
	}
	if pending {
		if err := r.CreatePreMigrationBackup(); err != nil {
			return err
		}
	}
	for _, migration := range migrations {
		if err := r.applyOne(migration); err != nil {
			return err
		}
	}
	return nil
}

func (r Runner) hasPendingMigrations(migrations []Migration) (bool, error) {
	for _, migration := range migrations {
		var existing Record
		err := r.DB.Where("version = ?", migration.Version).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if existing.Status != "applied" {
			return true, nil
		}
	}
	return false, nil
}

func (r Runner) CreatePreMigrationBackup() error {
	if r.DB == nil {
		return errors.New("db is required")
	}
	if err := ensureBackupTables(r.DB); err != nil {
		return err
	}
	sourcePath, err := sqliteMainPath(r.DB)
	if err != nil {
		return err
	}
	if sourcePath == "" || sourcePath == ":memory:" {
		return nil
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	backupDir := r.BackupDir
	if backupDir == "" {
		backupDir = filepath.Join(filepath.Dir(sourcePath), "migration_backups")
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}
	id := "backup-" + now().UTC().Format("20060102T150405.000000000")
	backupPath := filepath.Join(backupDir, id+".db")
	startedAt := now().UTC().Format(time.RFC3339)
	if err := r.DB.Exec("VACUUM INTO ?", backupPath).Error; err != nil {
		_ = insertBackupRecord(r.DB, id, backupPath, 0, "", "failed", startedAt, now().UTC().Format(time.RFC3339), err.Error())
		return err
	}
	size, checksum, err := fileSizeAndChecksum(backupPath)
	if err != nil {
		_ = insertBackupRecord(r.DB, id, backupPath, 0, "", "failed", startedAt, now().UTC().Format(time.RFC3339), err.Error())
		return err
	}
	finishedAt := now().UTC().Format(time.RFC3339)
	if err := insertBackupRecord(r.DB, id, backupPath, size, checksum, "completed", startedAt, finishedAt, ""); err != nil {
		return err
	}
	return r.DB.Exec("INSERT OR REPLACE INTO backup_contents (id, backup_id, table_name, row_count, checksum) VALUES (?, ?, ?, ?, ?)", id+"-database", id, "sqlite_database", 1, checksum).Error
}

func (r Runner) applyOne(migration Migration) error {
	if migration.Version == "" {
		return errors.New("migration version is required")
	}
	if migration.Up == nil {
		return fmt.Errorf("migration %s missing up function", migration.Version)
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	var existing Record
	err := r.DB.Where("version = ?", migration.Version).First(&existing).Error
	if err == nil && existing.Status == "applied" {
		return nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	step := &Step{}
	if err := r.DB.Transaction(func(tx *gorm.DB) error {
		step.db = tx
		step.commands = nil
		step.operations = nil
		if err := migration.Up(step); err != nil {
			return err
		}
		checksum := checksumFor(step.operations)
		startedAt := now().Format(time.RFC3339)
		record := Record{
			Version:    migration.Version,
			Name:       migration.Name,
			Checksum:   checksum,
			Status:     "applied",
			StartedAt:  startedAt,
			FinishedAt: now().Format(time.RFC3339),
		}
		for _, command := range step.commands {
			if err := tx.Exec(command).Error; err != nil {
				return err
			}
		}
		return tx.Save(&record).Error
	}); err != nil {
		failed := Record{
			Version:      migration.Version,
			Name:         migration.Name,
			Checksum:     checksumFor(step.operations),
			Status:       "failed",
			ErrorMessage: err.Error(),
			StartedAt:    now().Format(time.RFC3339),
			FinishedAt:   now().Format(time.RFC3339),
		}
		_ = r.DB.Save(&failed).Error
		return err
	}
	return nil
}

func (s *Step) CreateTable(sql string) {
	s.commands = append(s.commands, sql)
	s.operations = append(s.operations, sql)
}

func (s *Step) Execute(sql string) {
	s.commands = append(s.commands, sql)
	s.operations = append(s.operations, sql)
}

func (s *Step) AddColumn(table, column, definition string) error {
	if !safeIdentifier(table) || !safeIdentifier(column) {
		return fmt.Errorf("unsafe table or column name: %s.%s", table, column)
	}
	exists, err := s.ColumnExists(table, column)
	if err != nil {
		return err
	}
	operation := "add_column:" + table + "." + column + ":" + definition
	s.operations = append(s.operations, operation)
	if exists {
		return nil
	}
	s.commands = append(s.commands, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+definition)
	return nil
}

func (s *Step) CreateIndex(name, table string, columns []string, unique bool) error {
	if !safeIdentifier(name) || !safeIdentifier(table) {
		return fmt.Errorf("unsafe index or table name: %s %s", name, table)
	}
	for _, column := range columns {
		if !safeIdentifier(column) {
			return fmt.Errorf("unsafe index column: %s", column)
		}
	}
	exists, err := s.IndexExists(name)
	if err != nil {
		return err
	}
	operation := "create_index:" + name + ":" + table + ":" + strings.Join(columns, ",")
	s.operations = append(s.operations, operation)
	if exists {
		return nil
	}
	prefix := "CREATE INDEX "
	if unique {
		prefix = "CREATE UNIQUE INDEX "
	}
	s.commands = append(s.commands, prefix+name+" ON "+table+"("+strings.Join(columns, ", ")+")")
	return nil
}

func (s *Step) ColumnExists(table, column string) (bool, error) {
	if !safeIdentifier(table) || !safeIdentifier(column) {
		return false, fmt.Errorf("unsafe table or column name: %s.%s", table, column)
	}
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	if err := s.db.Raw("PRAGMA table_info(" + table + ")").Scan(&rows).Error; err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Name == column {
			return true, nil
		}
	}
	return false, nil
}

func (s *Step) IndexExists(name string) (bool, error) {
	if !safeIdentifier(name) {
		return false, fmt.Errorf("unsafe index name: %s", name)
	}
	var count int64
	if err := s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", name).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func checksumFor(parts []string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func safeIdentifier(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

func ensureBackupTables(db *gorm.DB) error {
	if err := db.Exec("CREATE TABLE IF NOT EXISTS backup_records (id TEXT PRIMARY KEY, backup_path TEXT NOT NULL DEFAULT '', backup_size INTEGER DEFAULT 0, checksum TEXT DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', started_at TEXT DEFAULT '', finished_at TEXT DEFAULT '', error_message TEXT DEFAULT '')").Error; err != nil {
		return err
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS backup_contents (id TEXT PRIMARY KEY, backup_id TEXT NOT NULL DEFAULT '', table_name TEXT NOT NULL DEFAULT '', row_count INTEGER DEFAULT 0, checksum TEXT DEFAULT '')").Error
}

func sqliteMainPath(db *gorm.DB) (string, error) {
	var rows []struct {
		Name string `gorm:"column:name"`
		File string `gorm:"column:file"`
	}
	if err := db.Raw("PRAGMA database_list").Scan(&rows).Error; err != nil {
		return "", err
	}
	for _, row := range rows {
		if row.Name == "main" {
			return row.File, nil
		}
	}
	return "", errors.New("main sqlite database not found")
}

func fileSizeAndChecksum(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	h := sha256.New()
	size, err := io.Copy(h, file)
	if err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(h.Sum(nil)), nil
}

func insertBackupRecord(db *gorm.DB, id, backupPath string, size int64, checksum, status, startedAt, finishedAt, errorMessage string) error {
	return db.Exec("INSERT OR REPLACE INTO backup_records (id, backup_path, backup_size, checksum, status, started_at, finished_at, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", id, backupPath, size, checksum, status, startedAt, finishedAt, errorMessage).Error
}

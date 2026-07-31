package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Migration struct {
	Version           string
	Name              string
	AcceptedChecksums []string
	Up                func(*Step) error
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
	DB         *gorm.DB
	Now        func() time.Time
	BackupDir  string
	SkipBackup bool
}

type Step struct {
	db         *gorm.DB
	commands   []string
	operations []string
}

func (s *Step) DB() *gorm.DB {
	return s.db
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
	if pending && !r.SkipBackup {
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
		currentChecksum, csErr := r.computeMigrationChecksum(migration)
		if csErr != nil {
			return fmt.Errorf("migration checksum compute failed %s: %w", migration.Version, csErr)
		}
		if existing.Checksum != "" && currentChecksum != existing.Checksum {
			accepted := false
			for _, checksum := range migration.AcceptedChecksums {
				if existing.Checksum == checksum {
					accepted = true
					break
				}
			}
			if !accepted {
				return fmt.Errorf("checksum mismatch for migration %s: recorded=%s current=%s", migration.Version, existing.Checksum, currentChecksum)
			}
			if err := r.DB.Model(&Record{}).Where("version = ?", migration.Version).Update("checksum", currentChecksum).Error; err != nil {
				return fmt.Errorf("normalize migration checksum %s: %w", migration.Version, err)
			}
		}
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
	tableExists, err := s.TableExists(table)
	if err != nil {
		return err
	}
	if exists || !tableExists {
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
	if s.db == nil {
		return false, nil
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
	if s.db == nil {
		return false, nil
	}
	var count int64
	if err := s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?", name).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Step) TableExists(name string) (bool, error) {
	if !safeIdentifier(name) {
		return false, fmt.Errorf("unsafe table name: %s", name)
	}
	if s.db == nil {
		return true, nil
	}
	var count int64
	if err := s.db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count).Error; err != nil {
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

func (r Runner) computeMigrationChecksum(migration Migration) (string, error) {
	step := &Step{}
	step.db = r.DB
	if err := migration.Up(step); err != nil {
		return "", err
	}
	return checksumFor(step.operations), nil
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

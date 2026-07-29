package migration

import (
	_ "embed"
	"fmt"
	"time"

	"gorm.io/gorm"
)

//go:embed baseline.sql
var baselineSQL string

func ApplyBaseline(db *gorm.DB) error {
	return ApplyInitialSQL(db, baselineSQL)
}

func MarkAllMigrationsApplied(db *gorm.DB, migrations []Migration) error {
	runner := Runner{DB: db}
	if err := runner.EnsureTable(); err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	for _, migration := range migrations {
		checksum, err := runner.computeMigrationChecksum(migration)
		if err != nil {
			return fmt.Errorf("compute checksum for %s: %w", migration.Version, err)
		}
		record := Record{
			Version:    migration.Version,
			Name:       migration.Name,
			Checksum:   checksum,
			Status:     "applied",
			StartedAt:  now,
			FinishedAt: now,
		}
		if err := db.Where("version = ?", migration.Version).FirstOrCreate(&record).Error; err != nil {
			return fmt.Errorf("mark migration %s applied: %w", migration.Version, err)
		}
	}
	return nil
}

func IsNewDatabase(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
		return false, err
	}
	return count == 0, nil
}

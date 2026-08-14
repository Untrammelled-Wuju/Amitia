package main

import (
	"context"
	"time"

	migrationcore "github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

type domainMigrationBackupPort struct {
	db        *gorm.DB
	backupDir string
}

func newDomainMigrationBackupPort(db *gorm.DB, backupDir string) *domainMigrationBackupPort {
	return &domainMigrationBackupPort{db: db, backupDir: backupDir}
}

func (p *domainMigrationBackupPort) CreateBackup(ctx context.Context) (string, error) {
	runner := migrationcore.Runner{
		DB:         p.db,
		BackupDir:  p.backupDir,
		Now:        func() time.Time { return time.Now() },
		SkipBackup: false,
	}
	if err := runner.CreatePreMigrationBackup(); err != nil {
		return "", err
	}
	return time.Now().UTC().Format("20060102T150405"), nil
}

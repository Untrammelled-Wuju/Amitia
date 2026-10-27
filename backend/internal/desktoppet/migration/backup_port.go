// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/database/sqlitebackup"
	"gorm.io/gorm"
)

type backupPortManifest struct {
	ID         string                `json:"id"`
	CreatedAt  string                `json:"createdAt"`
	BackupPath string                `json:"backupPath"`
	Components []backupPortComponent `json:"components"`
}

type backupPortComponent struct {
	Role       string `json:"role"`
	BackupPath string `json:"backupPath"`
	Size       int64  `json:"size"`
	Checksum   string `json:"checksum"`
}

type backupRecord struct {
	ID         string `gorm:"column:id;primaryKey"`
	BackupPath string `gorm:"column:backup_path"`
	BackupSize int64  `gorm:"column:backup_size"`
	Checksum   string `gorm:"column:checksum"`
	Status     string `gorm:"column:status"`
}

func (backupRecord) TableName() string { return "backup_records" }

type domainMigrationBackupPort struct {
	db        *gorm.DB
	backupDir string
}

func NewDomainMigrationBackupPort(db *gorm.DB, backupDir string) BackupPort {
	port := &domainMigrationBackupPort{db: db, backupDir: backupDir}
	if backupDir != "" {
		deleted, err := sqlitebackup.PruneBackups(backupDir, "", sqlitebackup.DefaultRetention)
		if err != nil {
			log.Printf("migration backup retention failed: %v", err)
		}
		if len(deleted) > 0 {
			log.Printf("migration backup retention removed %d expired snapshots", len(deleted))
		}
	}
	return port
}

func (p *domainMigrationBackupPort) CreateBackup(ctx context.Context) (string, error) {
	if err := p.db.WithContext(ctx).Exec(
		"CREATE TABLE IF NOT EXISTS backup_records (id TEXT PRIMARY KEY, backup_path TEXT NOT NULL DEFAULT '', backup_size INTEGER DEFAULT 0, checksum TEXT DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', started_at TEXT DEFAULT '', finished_at TEXT DEFAULT '', error_message TEXT DEFAULT '')",
	).Error; err != nil {
		return "", fmt.Errorf("ensure backup_records table: %w", err)
	}

	sourcePath, err := p.sqliteMainPath()
	if err != nil {
		return "", err
	}
	if sourcePath == "" || sourcePath == ":memory:" {
		return "", errors.New("cannot backup in-memory database")
	}

	backupID := "backup-" + uuid.NewString()
	backupDir := p.backupDir
	if backupDir == "" {
		backupDir = filepath.Join(filepath.Dir(sourcePath), "migration_backups")
	}
	backupPath := filepath.Join(backupDir, backupID+".db")
	startedAt := time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", p.failBackup(backupID, backupPath, startedAt, err)
	}

	if err := sqlitebackup.CreateConsistentSnapshot(ctx, p.db, backupPath); err != nil {
		return "", p.failBackup(backupID, backupPath, startedAt, err)
	}
	mainSize, mainChecksum, err := sqlitebackup.Inspect(backupPath)
	if err != nil {
		return "", p.failBackup(backupID, backupPath, startedAt, err)
	}
	components := []backupPortComponent{{
		Role:       "main",
		BackupPath: backupPath,
		Size:       mainSize,
		Checksum:   mainChecksum,
	}}

	manifestPath := filepath.Join(backupDir, backupID+".json")
	manifest := backupPortManifest{
		ID:         backupID,
		CreatedAt:  startedAt,
		BackupPath: backupPath,
		Components: components,
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", p.failBackup(backupID, backupPath, startedAt, err)
	}
	if err := os.WriteFile(manifestPath, append(manifestData, '\n'), 0o600); err != nil {
		return "", p.failBackup(backupID, backupPath, startedAt, err)
	}

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	if err := p.db.WithContext(ctx).Exec(
		"INSERT OR REPLACE INTO backup_records (id, backup_path, backup_size, checksum, status, started_at, finished_at, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		backupID, backupPath, mainSize, mainChecksum, "completed", startedAt, finishedAt, "",
	).Error; err != nil {
		return "", fmt.Errorf("insert backup record: %w", err)
	}
	deleted, pruneErr := sqlitebackup.PruneBackups(backupDir, backupPath, sqlitebackup.DefaultRetention)
	if pruneErr != nil {
		log.Printf("migration backup retention failed: %v", pruneErr)
	}
	if len(deleted) > 0 {
		log.Printf("migration backup retention removed %d expired snapshots", len(deleted))
	}

	return backupID, nil
}

func (p *domainMigrationBackupPort) BackupExists(ctx context.Context, backupID string) (bool, error) {
	if backupID == "" {
		return false, nil
	}
	var record backupRecord
	if err := p.db.WithContext(ctx).Where("id = ?", backupID).Take(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("query backup record: %w", err)
	}
	if record.Status != "completed" {
		return false, nil
	}
	if record.BackupPath == "" {
		return false, nil
	}
	mainSize, mainChecksum, err := sqlitebackup.Inspect(record.BackupPath)
	if err != nil || mainSize != record.BackupSize || !strings.EqualFold(mainChecksum, record.Checksum) {
		return false, nil
	}
	backupDir := filepath.Dir(record.BackupPath)
	manifestPath := filepath.Join(backupDir, backupID+".json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, nil
	}
	var manifest backupPortManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return false, nil
	}
	if manifest.ID != backupID || filepath.Clean(manifest.BackupPath) != filepath.Clean(record.BackupPath) || len(manifest.Components) == 0 {
		return false, nil
	}
	for _, component := range manifest.Components {
		size, checksum, err := sqlitebackup.Inspect(component.BackupPath)
		if err != nil || size != component.Size || !strings.EqualFold(checksum, component.Checksum) {
			return false, nil
		}
	}
	return true, nil
}

func (p *domainMigrationBackupPort) failBackup(backupID, backupPath, startedAt string, backupErr error) error {
	finishedAt := time.Now().UTC().Format(time.RFC3339)
	persistErr := p.db.WithContext(context.Background()).Exec(
		"INSERT OR REPLACE INTO backup_records (id, backup_path, backup_size, checksum, status, started_at, finished_at, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		backupID, backupPath, 0, "", "failed", startedAt, finishedAt, backupErr.Error(),
	).Error
	if persistErr != nil {
		return errors.Join(backupErr, fmt.Errorf("persist failed backup record: %w", persistErr))
	}
	return backupErr
}

func (p *domainMigrationBackupPort) sqliteMainPath() (string, error) {
	var rows []struct {
		Name string `gorm:"column:name"`
		File string `gorm:"column:file"`
	}
	if err := p.db.Raw("PRAGMA database_list").Scan(&rows).Error; err != nil {
		return "", err
	}
	for _, row := range rows {
		if row.Name == "main" {
			return row.File, nil
		}
	}
	return "", errors.New("main sqlite database not found")
}

var _ BackupPort = (*domainMigrationBackupPort)(nil)

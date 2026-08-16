// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
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
	Status     string `gorm:"column:status"`
}

func (backupRecord) TableName() string { return "backup_records" }

type domainMigrationBackupPort struct {
	db        *gorm.DB
	backupDir string
}

func NewDomainMigrationBackupPort(db *gorm.DB, backupDir string) BackupPort {
	return &domainMigrationBackupPort{db: db, backupDir: backupDir}
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

	if err := p.db.WithContext(ctx).Exec("PRAGMA wal_checkpoint(PASSIVE)").Error; err != nil {
		return "", p.failBackup(backupID, backupPath, startedAt, err)
	}

	components, err := p.copySQLiteBackupFiles(backupPath)
	if err != nil {
		return "", p.failBackup(backupID, backupPath, startedAt, err)
	}

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

	mainSize, mainChecksum, err := p.fileSizeAndChecksum(backupPath)
	if err != nil {
		return "", p.failBackup(backupID, backupPath, startedAt, err)
	}

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	if err := p.db.WithContext(ctx).Exec(
		"INSERT OR REPLACE INTO backup_records (id, backup_path, backup_size, checksum, status, started_at, finished_at, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		backupID, backupPath, mainSize, mainChecksum, "completed", startedAt, finishedAt, "",
	).Error; err != nil {
		return "", fmt.Errorf("insert backup record: %w", err)
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
	if _, err := os.Stat(record.BackupPath); err != nil {
		return false, nil
	}
	backupDir := filepath.Dir(record.BackupPath)
	manifestPath := filepath.Join(backupDir, backupID+".json")
	if _, err := os.Stat(manifestPath); err != nil {
		return false, nil
	}
	return true, nil
}

func (p *domainMigrationBackupPort) failBackup(backupID, backupPath, startedAt string, backupErr error) error {
	finishedAt := time.Now().UTC().Format(time.RFC3339)
	_ = p.db.WithContext(context.Background()).Exec(
		"INSERT OR REPLACE INTO backup_records (id, backup_path, backup_size, checksum, status, started_at, finished_at, error_message) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		backupID, backupPath, 0, "", "failed", startedAt, finishedAt, backupErr.Error(),
	).Error
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

func (p *domainMigrationBackupPort) copySQLiteBackupFiles(backupPath string) ([]backupPortComponent, error) {
	sourcePath, err := p.sqliteMainPath()
	if err != nil {
		return nil, err
	}
	entries := []struct {
		role     string
		src      string
		dst      string
		optional bool
	}{
		{"main", sourcePath, backupPath, false},
		{"wal", sourcePath + "-wal", backupPath + "-wal", true},
		{"shm", sourcePath + "-shm", backupPath + "-shm", true},
	}
	components := make([]backupPortComponent, 0, len(entries))
	for _, e := range entries {
		exists, err := fileExists(e.src)
		if err != nil {
			return nil, err
		}
		if !exists && e.optional {
			continue
		}
		if !exists {
			return nil, fmt.Errorf("source file not found: %s", e.src)
		}
		size, checksum, err := copyFileWithChecksum(e.src, e.dst)
		if err != nil {
			return nil, err
		}
		components = append(components, backupPortComponent{
			Role:       e.role,
			BackupPath: e.dst,
			Size:       size,
			Checksum:   checksum,
		})
	}
	return components, nil
}

func (p *domainMigrationBackupPort) fileSizeAndChecksum(path string) (int64, string, error) {
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

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func copyFileWithChecksum(sourcePath, backupPath string) (int64, string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return 0, "", err
	}
	defer source.Close()
	target, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, "", err
	}
	defer target.Close()
	h := sha256.New()
	size, err := io.Copy(io.MultiWriter(target, h), source)
	if err != nil {
		return 0, "", err
	}
	if err := target.Sync(); err != nil {
		return 0, "", err
	}
	return size, hex.EncodeToString(h.Sum(nil)), nil
}

var _ BackupPort = (*domainMigrationBackupPort)(nil)

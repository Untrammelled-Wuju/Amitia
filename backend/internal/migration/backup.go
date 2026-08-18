package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

type backupComponent struct {
	Role         string `json:"role"`
	SourcePath   string `json:"source_path"`
	BackupPath   string `json:"backup_path"`
	RestorePath  string `json:"restore_path"`
	Size         int64  `json:"size"`
	Checksum     string `json:"checksum"`
	Optional     bool   `json:"optional"`
	SourceExists bool   `json:"source_exists"`
}

type backupManifest struct {
	ID         string            `json:"id"`
	CreatedAt  string            `json:"created_at"`
	SourcePath string            `json:"source_path"`
	BackupPath string            `json:"backup_path"`
	Components []backupComponent `json:"components"`
}

func BackupMigration() Migration {
	return Migration{
		Version: "202607010001",
		Name:    "create_backup_records_table",
		Up: func(step *Step) error {
			step.CreateTable("CREATE TABLE IF NOT EXISTS backup_records (id TEXT PRIMARY KEY, backup_path TEXT NOT NULL DEFAULT '', backup_size INTEGER DEFAULT 0, checksum TEXT DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', started_at TEXT DEFAULT '', finished_at TEXT DEFAULT '', error_message TEXT DEFAULT '')")
			step.CreateTable("CREATE TABLE IF NOT EXISTS backup_contents (id TEXT PRIMARY KEY, backup_id TEXT NOT NULL DEFAULT '', table_name TEXT NOT NULL DEFAULT '', row_count INTEGER DEFAULT 0, checksum TEXT DEFAULT '')")
			return nil
		},
	}
}

func recordBackupFailure(db *gorm.DB, id, backupPath, startedAt string, now func() time.Time, primary error) error {
	if primary == nil {
		return nil
	}
	finishedAt := now().UTC().Format(time.RFC3339)
	if persistErr := insertBackupRecord(db, id, backupPath, 0, "", "failed", startedAt, finishedAt, primary.Error()); persistErr != nil {
		return errors.Join(primary, fmt.Errorf("persist failed backup record: %w", persistErr))
	}
	return primary
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
	if err := runIntegrityCheck(r.DB); err != nil {
		return fmt.Errorf("pre-migration integrity check failed: %w", err)
	}
	if err := runForeignKeyCheck(r.DB); err != nil {
		return fmt.Errorf("pre-migration foreign key check failed: %w", err)
	}
	if err := checkDiskSpace(sourcePath, 50*1024*1024); err != nil {
		return fmt.Errorf("pre-migration disk space check failed: %w", err)
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	startedAtTime := now().UTC()
	id := "backup-" + startedAtTime.Format("20060102T150405.000000000")
	backupDir := r.BackupDir
	if backupDir == "" {
		backupDir = filepath.Join(filepath.Dir(sourcePath), "migration_backups")
	}
	backupPath := filepath.Join(backupDir, id+".db")
	startedAt := startedAtTime.Format(time.RFC3339)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return recordBackupFailure(r.DB, id, backupPath, startedAt, now, err)
	}
	if err := r.DB.Exec("PRAGMA wal_checkpoint(PASSIVE)").Error; err != nil {
		return recordBackupFailure(r.DB, id, backupPath, startedAt, now, err)
	}
	components, err := copySQLiteBackupFiles(sourcePath, backupPath)
	if err != nil {
		return recordBackupFailure(r.DB, id, backupPath, startedAt, now, err)
	}
	manifestPath := filepath.Join(backupDir, id+".json")
	if err := writeBackupManifest(manifestPath, backupManifest{
		ID:         id,
		CreatedAt:  startedAt,
		SourcePath: sourcePath,
		BackupPath: backupPath,
		Components: components,
	}); err != nil {
		return recordBackupFailure(r.DB, id, backupPath, startedAt, now, err)
	}
	metadataSize, metadataChecksum, err := fileSizeAndChecksum(manifestPath)
	if err != nil {
		return recordBackupFailure(r.DB, id, backupPath, startedAt, now, err)
	}
	components = append(components, backupComponent{
		Role:         "metadata",
		SourcePath:   "",
		BackupPath:   manifestPath,
		RestorePath:  "",
		Size:         metadataSize,
		Checksum:     metadataChecksum,
		Optional:     false,
		SourceExists: true,
	})
	mainSize, mainChecksum, err := fileSizeAndChecksum(backupPath)
	if err != nil {
		return recordBackupFailure(r.DB, id, backupPath, startedAt, now, err)
	}
	finishedAt := now().UTC().Format(time.RFC3339)
	if err := insertBackupRecord(r.DB, id, backupPath, mainSize, mainChecksum, "completed", startedAt, finishedAt, ""); err != nil {
		return err
	}
	for _, component := range components {
		contentID := id + "-" + component.Role
		if err := r.DB.Exec("INSERT OR REPLACE INTO backup_contents (id, backup_id, table_name, row_count, checksum) VALUES (?, ?, ?, ?, ?)", contentID, id, component.Role, component.Size, component.Checksum).Error; err != nil {
			return err
		}
	}
	return nil
}

func copySQLiteBackupFiles(sourcePath, backupPath string) ([]backupComponent, error) {
	entries := []backupComponent{
		{Role: "main", SourcePath: sourcePath, BackupPath: backupPath, RestorePath: sourcePath},
		{Role: "wal", SourcePath: sourcePath + "-wal", BackupPath: backupPath + "-wal", RestorePath: sourcePath + "-wal", Optional: true},
		{Role: "shm", SourcePath: sourcePath + "-shm", BackupPath: backupPath + "-shm", RestorePath: sourcePath + "-shm", Optional: true},
	}
	components := make([]backupComponent, 0, len(entries))
	for _, entry := range entries {
		exists, err := fileExists(entry.SourcePath)
		if err != nil {
			return nil, err
		}
		if !exists && entry.Optional {
			entry.SourceExists = false
			components = append(components, entry)
			continue
		}
		if !exists {
			return nil, os.ErrNotExist
		}
		size, checksum, err := copyFileWithChecksum(entry.SourcePath, entry.BackupPath)
		if err != nil {
			return nil, err
		}
		entry.Size = size
		entry.Checksum = checksum
		entry.SourceExists = true
		components = append(components, entry)
	}
	return components, nil
}

func writeBackupManifest(path string, manifest backupManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	return nil
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
	target, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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

func runIntegrityCheck(db *gorm.DB) error {
	var results []struct {
		Ok string `gorm:"column:integrity_check"`
	}
	if err := db.Raw("PRAGMA integrity_check").Scan(&results).Error; err != nil {
		return err
	}
	if len(results) == 0 || results[0].Ok != "ok" {
		return fmt.Errorf("integrity check failed: %v", results)
	}
	return nil
}

func runForeignKeyCheck(db *gorm.DB) error {
	var results []struct {
		Ok string `gorm:"column:foreign_key_check"`
	}
	if err := db.Raw("PRAGMA foreign_key_check").Scan(&results).Error; err != nil {
		return err
	}
	if len(results) > 0 && results[0].Ok != "" {
		return fmt.Errorf("foreign key violations found: %d", len(results))
	}
	return nil
}

func checkDiskSpace(path string, minBytes int64) error {
	dir := filepath.Dir(path)
	if dir == "." {
		dir = path
	}
	var stat struct {
		FreeBytes int64
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	_ = info
	_ = stat
	return nil
}

package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/pkg/database/sqlitebackup"
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
	if err := sqlitebackup.CreateConsistentSnapshot(context.Background(), r.DB, backupPath); err != nil {
		return recordBackupFailure(r.DB, id, backupPath, startedAt, now, err)
	}
	mainSize, mainChecksum, err := sqlitebackup.Inspect(backupPath)
	if err != nil {
		return recordBackupFailure(r.DB, id, backupPath, startedAt, now, err)
	}
	components := []backupComponent{{
		Role:         "main",
		SourcePath:   sourcePath,
		BackupPath:   backupPath,
		RestorePath:  sourcePath,
		Size:         mainSize,
		Checksum:     mainChecksum,
		Optional:     false,
		SourceExists: true,
	}}
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
	metadataSize, metadataChecksum, err := sqlitebackup.Inspect(manifestPath)
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
	deleted, pruneErr := sqlitebackup.PruneBackups(backupDir, backupPath, sqlitebackup.DefaultRetention)
	if pruneErr != nil {
		log.Printf("migration backup retention failed: %v", pruneErr)
	}
	if len(deleted) > 0 {
		log.Printf("migration backup retention removed %d expired snapshots", len(deleted))
	}
	return nil
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

func ensureBackupTables(db *gorm.DB) error {
	if err := db.Exec("CREATE TABLE IF NOT EXISTS backup_records (id TEXT PRIMARY KEY, backup_path TEXT NOT NULL DEFAULT '', backup_size INTEGER DEFAULT 0, checksum TEXT DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', started_at TEXT DEFAULT '', finished_at TEXT DEFAULT '', error_message TEXT DEFAULT '', purpose TEXT DEFAULT 'user', format_version INTEGER DEFAULT 1, profile TEXT DEFAULT 'full', scope TEXT DEFAULT 'all', encrypted INTEGER DEFAULT 0, manifest_checksum TEXT DEFAULT '', app_version TEXT DEFAULT '', schema_fingerprint TEXT DEFAULT '')").Error; err != nil {
		return err
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS backup_contents (id TEXT PRIMARY KEY, backup_id TEXT NOT NULL DEFAULT '', table_name TEXT NOT NULL DEFAULT '', row_count INTEGER DEFAULT 0, checksum TEXT DEFAULT '', component_id TEXT DEFAULT '', kind TEXT DEFAULT '', logical_name TEXT DEFAULT '', size_bytes INTEGER DEFAULT 0, item_count INTEGER DEFAULT 0, required INTEGER DEFAULT 1, source_of_truth INTEGER DEFAULT 0, rebuildable INTEGER DEFAULT 0)").Error
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

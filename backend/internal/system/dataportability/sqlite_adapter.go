package dataportability

import (
	"context"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/migration"
	"gorm.io/gorm"
)

type SQLiteAdapter struct {
	DB        *gorm.DB
	BackupDir string
	runner    *migration.Runner
}

func NewSQLiteAdapter(db *gorm.DB, backupDir string) *SQLiteAdapter {
	return &SQLiteAdapter{
		DB:        db,
		BackupDir: backupDir,
		runner: &migration.Runner{
			DB:        db,
			BackupDir: backupDir,
			SkipBackup: false,
		},
	}
}

func (a *SQLiteAdapter) CheckpointWAL() {
	_ = a.DB.Exec("PRAGMA wal_checkpoint(PASSIVE)").Error
}

func (a *SQLiteAdapter) RunIntegrityCheck() {
	var results []struct {
		Ok string `gorm:"column:integrity_check"`
	}
	_ = a.DB.Raw("PRAGMA integrity_check").Scan(&results)
}

func (a *SQLiteAdapter) RunForeignKeyCheck() {
	var results []struct {
		Ok string `gorm:"column:foreign_key_check"`
	}
	_ = a.DB.Raw("PRAGMA foreign_key_check").Scan(&results)
}

func (a *SQLiteAdapter) GetSQLiteFiles() []SQLiteFileInfo {
	var rows []struct {
		Name string `gorm:"column:name"`
		File string `gorm:"column:file"`
	}
	_ = a.DB.Raw("PRAGMA database_list").Scan(&rows)

	var files []SQLiteFileInfo
	for _, row := range rows {
		if row.Name == "main" && row.File != "" && row.File != ":memory:" {
			files = append(files, SQLiteFileInfo{
				Name: "app.db",
				Path: row.File,
			})
			if info, err := os.Stat(row.File + "-wal"); err == nil && info.Size() > 0 {
				files = append(files, SQLiteFileInfo{
					Name: "app.db-wal",
					Path: row.File + "-wal",
				})
			}
			if info, err := os.Stat(row.File + "-shm"); err == nil && info.Size() > 0 {
				files = append(files, SQLiteFileInfo{
					Name: "app.db-shm",
					Path: row.File + "-shm",
				})
			}
		}
	}
	return files
}

func (a *SQLiteAdapter) BackupTo(dest string) error {
	dbPath := ""
	for _, f := range a.GetSQLiteFiles() {
		if f.Name == "app.db" {
			dbPath = f.Path
			break
		}
	}
	if dbPath == "" {
		return ErrBackupSnapshotFailed
	}

	entries := []SQLiteFileInfo{
		{Name: "app.db", Path: dbPath},
	}
	walPath := dbPath + "-wal"
	if info, err := os.Stat(walPath); err == nil && info.Size() > 0 {
		entries = append(entries, SQLiteFileInfo{Name: "app.db-wal", Path: walPath})
	}
	shmPath := dbPath + "-shm"
	if info, err := os.Stat(shmPath); err == nil && info.Size() > 0 {
		entries = append(entries, SQLiteFileInfo{Name: "app.db-shm", Path: shmPath})
	}

	for _, entry := range entries {
		destPath := dest
		if entry.Name != "app.db" {
			destPath = dest + "-" + entry.Name[len("app.db"):]
		}
		if err := copyFile(entry.Path, destPath); err != nil && entry.Name == "app.db" {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func (a *SQLiteAdapter) Migrate() error {
	_ = context.Background()
	return nil
}

func GetSQLiteMainPath(db *gorm.DB) string {
	adapter := NewSQLiteAdapter(db, "")
	files := adapter.GetSQLiteFiles()
	for _, f := range files {
		if f.Name == "app.db" {
			return f.Path
		}
	}
	return filepath.Dir("")
}

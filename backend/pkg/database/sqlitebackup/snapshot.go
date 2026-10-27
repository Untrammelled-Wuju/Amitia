package sqlitebackup

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/glebarez/go-sqlite"
	"gorm.io/gorm"
)

func CreateConsistentSnapshot(ctx context.Context, db *gorm.DB, destination string) error {
	if db == nil {
		return errors.New("db is required")
	}
	if strings.TrimSpace(destination) == "" {
		return errors.New("snapshot destination is required")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	removeSnapshotFiles(destination)
	quoted := "'" + strings.ReplaceAll(filepath.Clean(destination), "'", "''") + "'"
	if err := db.WithContext(ctx).Exec("VACUUM INTO " + quoted).Error; err != nil {
		removeSnapshotFiles(destination)
		return fmt.Errorf("create sqlite snapshot: %w", err)
	}
	if err := os.Chmod(destination, 0o600); err != nil {
		removeSnapshotFiles(destination)
		return err
	}
	if err := VerifySnapshot(ctx, destination); err != nil {
		removeSnapshotFiles(destination)
		return err
	}
	return nil
}

func VerifySnapshot(ctx context.Context, path string) error {
	uri := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: "mode=ro&immutable=1"}).String()
	db, err := sql.Open("sqlite", uri)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("verify sqlite snapshot: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("sqlite snapshot integrity check failed: %s", result)
	}
	return nil
}

func Inspect(path string) (int64, string, error) {
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

func removeSnapshotFiles(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	_ = os.Remove(path + "-journal")
}

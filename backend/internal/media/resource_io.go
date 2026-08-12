package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ResourceIO struct {
	tempDir string
}

func NewResourceIO(tempDir string) *ResourceIO {
	return &ResourceIO{tempDir: tempDir}
}

func (r *ResourceIO) MaterializeToLocal(inputPath string) (string, error) {
	if inputPath == "" {
		return "", fmt.Errorf("input path is empty")
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return "", fmt.Errorf("cannot stat input: %w", err)
	}

	if info.IsDir() {
		return "", fmt.Errorf("input path is a directory")
	}

	return inputPath, nil
}

func (r *ResourceIO) CreateStagingPath(extension string) (string, error) {
	if r.tempDir == "" {
		r.tempDir = os.TempDir()
	}

	if err := os.MkdirAll(r.tempDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	pattern := "media_staging_*" + extension
	f, err := os.CreateTemp(r.tempDir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create staging file: %w", err)
	}
	f.Close()

	return f.Name(), nil
}

func (r *ResourceIO) AtomicCommit(stagingPath, targetPath string) error {
	if stagingPath == "" {
		return fmt.Errorf("staging path is empty")
	}
	if targetPath == "" {
		return fmt.Errorf("target path is empty")
	}

	if _, err := os.Stat(stagingPath); err != nil {
		return fmt.Errorf("staging file not found: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	if err := os.Rename(stagingPath, targetPath); err != nil {
		if linkErr, ok := err.(*os.LinkError); ok {
			if linkErr.Op == "rename" {
				return r.copyAndDelete(stagingPath, targetPath)
			}
		}
		return fmt.Errorf("atomic commit failed: %w", err)
	}

	return nil
}

func (r *ResourceIO) copyAndDelete(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	_ = os.Remove(src)

	return nil
}

func (r *ResourceIO) CleanupStaging(stagingPath string) error {
	if stagingPath == "" {
		return nil
	}
	if err := os.Remove(stagingPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup staging: %w", err)
	}
	return nil
}

func (r *ResourceIO) ComputeContentHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (r *ResourceIO) GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	return info.Size(), nil
}

func (r *ResourceIO) CheckDiskSpace(path string, requiredBytes int64) error {
	return nil
}

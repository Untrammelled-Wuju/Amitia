// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrantlayout

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type DirectoryCopier interface {
	CopyAndVerify(ctx context.Context, source, target string) error
}

type directoryCopier struct{}

func NewDirectoryCopier() DirectoryCopier {
	return &directoryCopier{}
}

func (c *directoryCopier) CopyAndVerify(ctx context.Context, source, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if source == target {
		return newInvalidLayout("source and target are identical")
	}
	if containsPath(source, target) {
		return newInvalidLayout("target is within source")
	}
	if containsPath(target, source) {
		return newInvalidLayout("source is within target")
	}

	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}
	if !sourceInfo.IsDir() {
		return newInvalidLayout("source is not a directory")
	}

	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("read source dir: %w", err)
	}

	if err := os.MkdirAll(target, 0750); err != nil {
		return fmt.Errorf("create target: %w", err)
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		sourcePath := filepath.Join(source, entry.Name())
		targetPath := filepath.Join(target, entry.Name())

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("get file info: %w", err)
		}

		if entry.IsDir() {
			if err := c.CopyAndVerify(ctx, sourcePath, targetPath); err != nil {
				return err
			}
			continue
		}

		if !info.IsDir() && (info.Mode()&os.ModeSymlink) != 0 {
			linkTarget, err := os.Readlink(sourcePath)
			if err != nil {
				return fmt.Errorf("read symlink: %w", err)
			}
			if filepath.IsAbs(linkTarget) {
				return newInvalidLayout("absolute symlink not allowed: " + sourcePath)
			}
			if containsPath(source, filepath.Join(filepath.Dir(sourcePath), linkTarget)) {
			} else {
				return newInvalidLayout("symlink escapes source: " + sourcePath)
			}
			if err := os.Symlink(linkTarget, targetPath); err != nil {
				return fmt.Errorf("create symlink: %w", err)
			}
			continue
		}

		if !info.Mode().IsRegular() {
			return newInvalidLayout("special file not allowed: " + sourcePath)
		}

		if err := c.copyFile(ctx, sourcePath, targetPath, info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func (c *directoryCopier) copyFile(ctx context.Context, source, target string, perm os.FileMode) error {
	srcFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer srcFile.Close()

	tmpPath := target + ".copying"
	tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	hash := sha256.New()
	reader := io.TeeReader(srcFile, hash)

	if _, err := io.Copy(tmpFile, reader); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("copy file content: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := os.Rename(tmpPath, target); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}

	targetFile, err := os.Open(target)
	if err != nil {
		return fmt.Errorf("open target for verify: %w", err)
	}
	defer targetFile.Close()

	verifyHash := sha256.New()
	if _, err := io.Copy(verifyHash, targetFile); err != nil {
		return fmt.Errorf("verify hash: %w", err)
	}

	sourceHash := hex.EncodeToString(hash.Sum(nil))
	targetHash := hex.EncodeToString(verifyHash.Sum(nil))

	if sourceHash != targetHash {
		os.Remove(target)
		return newMigrationVerification(source, target, fmt.Errorf("hash mismatch: source=%s target=%s", sourceHash, targetHash))
	}

	return nil
}

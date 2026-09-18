// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/release"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func validateID(component, id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid %s: must match %s", component, idPattern.String())
	}
	return nil
}

func isUnderRoot(candidate, root string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

var _ release.ReleaseStoragePort = (*FileSystemStorage)(nil)

type FileSystemStorage struct {
	root          string
	workspaceRoot string
	stagingRoot   string
	publishedRoot string
	archiveRoot   string
}

func NewFileSystemStorage(dataDir string) *FileSystemStorage {
	root := filepath.Join(dataDir, "desktop-pets", "releases")
	return &FileSystemStorage{
		root:          root,
		workspaceRoot: filepath.Join(root, "workspaces"),
		stagingRoot:   filepath.Join(root, "staging"),
		publishedRoot: filepath.Join(root, "published"),
		archiveRoot:   filepath.Join(root, "archives"),
	}
}

func (s *FileSystemStorage) Validate() error {
	dirs := []string{
		s.root,
		s.workspaceRoot,
		s.stagingRoot,
		s.publishedRoot,
		s.archiveRoot,
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create release directory %s: %w", dir, err)
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return fmt.Errorf("stat release directory %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("release path %s is not a directory", dir)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("release path %s is a symlink/junction", dir)
		}
		testFile := filepath.Join(dir, ".write_test")
		f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("release directory %s is not writable: %w", dir, err)
		}
		f.Close()
		if err := os.Remove(testFile); err != nil {
			return fmt.Errorf("cleanup write test file in %s: %w", dir, err)
		}
	}
	return nil
}

func (s *FileSystemStorage) safeDeleteUnderRoot(root, entityPath string) error {
	info, err := os.Lstat(entityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("lstat %s: %w", entityPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symlink deletion is not allowed")
	}
	if entityPath == root {
		return fmt.Errorf("refusing to remove root directory %s", root)
	}
	if !strings.HasPrefix(entityPath, root+string(filepath.Separator)) {
		return fmt.Errorf("path %s escapes root %s", entityPath, root)
	}
	return s.removeTreeContained(root, entityPath)
}

func (s *FileSystemStorage) removeTreeContained(root, target string) error {
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("symlink deletion is not allowed")
	}
	if !info.IsDir() {
		return os.Remove(target)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(target, entry.Name())
		childInfo, err := os.Lstat(child)
		if err != nil {
			return err
		}
		if childInfo.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink inside release storage is not allowed")
		}
		if childInfo.IsDir() {
			if err := s.removeTreeContained(root, child); err != nil {
				return err
			}
			continue
		}
		if err := os.Remove(child); err != nil {
			return err
		}
	}
	return os.Remove(target)
}

func ensureNoSymlinkComponents(root, target string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path %s escapes root %s", target, root)
	}
	current := rootAbs
	if rel == "." {
		return nil
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink component is not allowed: %s", current)
		}
	}
	return nil
}

func (s *FileSystemStorage) safeMove(srcRoot, src, dstRoot, dst string) error {
	if !isUnderRoot(src, srcRoot) || !isUnderRoot(dst, dstRoot) {
		return errors.New("move path escapes release storage root")
	}
	if err := ensureNoSymlinkComponents(srcRoot, src); err != nil {
		return fmt.Errorf("source path validation: %w", err)
	}
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("source lstat: %w", err)
	}
	if srcInfo.Mode()&os.ModeSymlink != 0 || !srcInfo.IsDir() {
		return fmt.Errorf("source must be a non-symlink directory: %s", src)
	}
	if err := ensureNoSymlinkComponents(dstRoot, filepath.Dir(dst)); err != nil {
		return fmt.Errorf("destination parent validation: %w", err)
	}
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("target already exists: %s", dst)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("target lstat: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	if err := ensureNoSymlinkComponents(dstRoot, filepath.Dir(dst)); err != nil {
		return fmt.Errorf("destination parent validation after create: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("move %s to %s: %w", src, dst, err)
	}
	return nil
}

func (s *FileSystemStorage) safeCheckedID(id string) (string, error) {
	clean := strings.TrimSpace(id)
	if clean == "" || filepath.IsAbs(clean) || strings.Contains(clean, `\`) {
		return "", fmt.Errorf("unsafe id: absolute or backslash")
	}
	if err := validateID("releaseID", clean); err != nil {
		return "", err
	}
	return clean, nil
}

func (s *FileSystemStorage) safeCheckedPetID(petID string) (string, error) {
	clean := strings.TrimSpace(petID)
	if clean == "" || filepath.IsAbs(clean) || strings.Contains(clean, `\`) {
		return "", fmt.Errorf("unsafe petID: absolute or backslash")
	}
	if err := validateID("petID", clean); err != nil {
		return "", err
	}
	return clean, nil
}

func (s *FileSystemStorage) StagingDir(releaseID string) (string, error) {
	clean, err := s.safeCheckedID(releaseID)
	if err != nil {
		return "", err
	}
	target := filepath.Join(s.stagingRoot, clean)
	if !isUnderRoot(target, s.stagingRoot) {
		return "", errors.New("staging path escapes root")
	}
	return target, nil
}

func (s *FileSystemStorage) WorkspaceDir(operationID string) (string, error) {
	clean, err := s.safeCheckedID(operationID)
	if err != nil {
		return "", err
	}
	target := filepath.Join(s.workspaceRoot, clean)
	if !isUnderRoot(target, s.workspaceRoot) {
		return "", errors.New("workspace path escapes root")
	}
	return target, nil
}

func (s *FileSystemStorage) EnsureWorkspaceDir(operationID string) error {
	dir, err := s.WorkspaceDir(operationID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create workspace directory: %w", err)
	}
	return nil
}

func (s *FileSystemStorage) EnsureStagingDir(releaseID string) error {
	dir, err := s.StagingDir(releaseID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create staging directory: %w", err)
	}
	return nil
}

func (s *FileSystemStorage) RemoveStagingDir(releaseID string) error {
	dir, err := s.StagingDir(releaseID)
	if err != nil {
		return err
	}
	return s.safeDeleteUnderRoot(s.stagingRoot, dir)
}

func (s *FileSystemStorage) RemoveWorkspaceDir(operationID string) error {
	dir, err := s.WorkspaceDir(operationID)
	if err != nil {
		return err
	}
	return s.safeDeleteUnderRoot(s.workspaceRoot, dir)
}

func (s *FileSystemStorage) PublishedDir(petID, releaseID string) (string, error) {
	p, err := s.safeCheckedPetID(petID)
	if err != nil {
		return "", err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return "", err
	}

	target := filepath.Join(s.publishedRoot, p, r)

	if !isUnderRoot(target, s.publishedRoot) {
		return "", errors.New("published path escapes root")
	}

	return target, nil
}

func (s *FileSystemStorage) PublishedStorageKey(petID, releaseID string) (string, error) {
	p, err := s.safeCheckedPetID(petID)
	if err != nil {
		return "", err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return "", err
	}
	return p + "/" + r, nil
}

func (s *FileSystemStorage) ArchivePath(petID, releaseID string) (string, error) {
	p, err := s.safeCheckedPetID(petID)
	if err != nil {
		return "", err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return "", err
	}
	target := filepath.Join(s.archiveRoot, p, r+".zip")
	if !isUnderRoot(target, s.archiveRoot) {
		return "", errors.New("archive path escapes root")
	}
	return target, nil
}

func (s *FileSystemStorage) ArchiveStorageKey(petID, releaseID string) (string, error) {
	p, err := s.safeCheckedPetID(petID)
	if err != nil {
		return "", err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return "", err
	}
	return p + "/" + r + ".zip", nil
}

func (s *FileSystemStorage) MoveStagingToPublished(petID, releaseID string) error {
	p, err := s.safeCheckedPetID(petID)
	if err != nil {
		return err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return err
	}
	src := filepath.Join(s.stagingRoot, r)
	dst := filepath.Join(s.publishedRoot, p, r)
	if !isUnderRoot(src, s.stagingRoot) || !isUnderRoot(dst, s.publishedRoot) {
		return fmt.Errorf("path escape detected")
	}
	return s.safeMove(s.stagingRoot, src, s.publishedRoot, dst)
}

func (s *FileSystemStorage) MoveWorkspaceToStaging(operationID, releaseID string) error {
	o, err := s.safeCheckedID(operationID)
	if err != nil {
		return err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return err
	}
	src := filepath.Join(s.workspaceRoot, o)
	dst := filepath.Join(s.stagingRoot, r)
	if !isUnderRoot(src, s.workspaceRoot) || !isUnderRoot(dst, s.stagingRoot) {
		return fmt.Errorf("path escape detected")
	}
	return s.safeMove(s.workspaceRoot, src, s.stagingRoot, dst)
}

func (s *FileSystemStorage) RemovePublishedDir(petID, releaseID string) error {
	p, err := s.safeCheckedPetID(petID)
	if err != nil {
		return err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return err
	}
	dir := filepath.Join(s.publishedRoot, p, r)
	return s.safeDeleteUnderRoot(s.publishedRoot, dir)
}

func (s *FileSystemStorage) AtomicRenameStagingToPublished(petID, releaseID string) error {
	p, err := s.safeCheckedPetID(petID)
	if err != nil {
		return err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return err
	}
	src := filepath.Join(s.stagingRoot, r)
	dst := filepath.Join(s.publishedRoot, p, r)
	if !isUnderRoot(src, s.stagingRoot) || !isUnderRoot(dst, s.publishedRoot) {
		return fmt.Errorf("path escape detected")
	}
	return s.safeMove(s.stagingRoot, src, s.publishedRoot, dst)
}

func cleanupTempFile(path string, primary error) error {
	if path == "" {
		return primary
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Join(primary, fmt.Errorf("remove temporary archive %s: %w", path, err))
	}
	return primary
}

func (s *FileSystemStorage) StoreVerifiedArchive(sourcePath, petID, releaseID, expectedHash string) (string, string, int64, error) {
	p, err := s.safeCheckedPetID(petID)
	if err != nil {
		return "", "", 0, err
	}
	r, err := s.safeCheckedID(releaseID)
	if err != nil {
		return "", "", 0, err
	}

	sourceInfo, err := os.Lstat(sourcePath)
	if err != nil {
		return "", "", 0, fmt.Errorf("lstat source archive: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() || sourceInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", 0, errors.New("source archive must be a regular non-symlink file")
	}

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return "", "", 0, fmt.Errorf("open source archive: %w", err)
	}
	defer sourceFile.Close()

	hash := sha256.New()
	tmpF, err := os.CreateTemp(s.archiveRoot, ".archive-*")
	if err != nil {
		return "", "", 0, fmt.Errorf("create temp archive: %w", err)
	}
	tmpPath := tmpF.Name()
	tmpClosed := false
	closeTemp := func() error {
		if tmpClosed {
			return nil
		}
		tmpClosed = true
		return tmpF.Close()
	}
	fail := func(primary error) (string, string, int64, error) {
		if closeErr := closeTemp(); closeErr != nil {
			primary = errors.Join(primary, fmt.Errorf("close temp archive: %w", closeErr))
		}
		return "", "", 0, cleanupTempFile(tmpPath, primary)
	}

	written, err := io.Copy(io.MultiWriter(tmpF, hash), sourceFile)
	if err != nil {
		return fail(fmt.Errorf("stream archive: %w", err))
	}
	if err := tmpF.Sync(); err != nil {
		return fail(fmt.Errorf("fsync archive: %w", err))
	}
	if err := closeTemp(); err != nil {
		return "", "", 0, cleanupTempFile(tmpPath, fmt.Errorf("close temp archive: %w", err))
	}

	actualHash := hex.EncodeToString(hash.Sum(nil))
	if expectedHash != "" && !strings.EqualFold(actualHash, expectedHash) {
		return "", "", 0, cleanupTempFile(tmpPath, fmt.Errorf("archive hash mismatch: expected %s, got %s", expectedHash, actualHash))
	}

	finalPath := filepath.Join(s.archiveRoot, p, r+".zip")
	if !isUnderRoot(finalPath, s.archiveRoot) {
		return "", "", 0, cleanupTempFile(tmpPath, errors.New("archive path escapes root"))
	}

	archiveParent := filepath.Dir(finalPath)
	if err := ensureNoSymlinkComponents(s.archiveRoot, archiveParent); err != nil {
		return "", "", 0, cleanupTempFile(tmpPath, fmt.Errorf("archive parent validation: %w", err))
	}
	if _, err := os.Lstat(finalPath); err == nil {
		return "", "", 0, cleanupTempFile(tmpPath, errors.New("archive destination already exists"))
	} else if !os.IsNotExist(err) {
		return "", "", 0, cleanupTempFile(tmpPath, fmt.Errorf("archive destination lstat: %w", err))
	}
	if err := os.MkdirAll(archiveParent, 0o700); err != nil {
		return "", "", 0, cleanupTempFile(tmpPath, fmt.Errorf("create archive dir: %w", err))
	}
	if err := ensureNoSymlinkComponents(s.archiveRoot, archiveParent); err != nil {
		return "", "", 0, cleanupTempFile(tmpPath, fmt.Errorf("archive parent validation after create: %w", err))
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", "", 0, cleanupTempFile(tmpPath, fmt.Errorf("atomic rename archive: %w", err))
	}

	storageKey := p + "/" + r + ".zip"
	return storageKey, actualHash, written, nil
}

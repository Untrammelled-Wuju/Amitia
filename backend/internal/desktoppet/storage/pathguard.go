// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrPathEscape        = errors.New("storage: path escapes root")
	ErrEmptyPath         = errors.New("storage: empty path")
	ErrAbsolutePath      = errors.New("storage: absolute path not allowed")
	ErrInvalidCharacters = errors.New("storage: invalid characters in path")
	ErrSymlinkDetected   = errors.New("storage: symlink not allowed")
	ErrJunctionDetected  = errors.New("storage: junction/reparse point not allowed")
	ErrReservedName      = errors.New("storage: reserved name not allowed")
	ErrPathTooLong       = errors.New("storage: path too long")
	ErrRootDelete        = errors.New("storage: cannot delete root directory")
	ErrInvalidID         = errors.New("storage: invalid entity ID")
)

type SafePath struct {
	Root       CanonicalRoot
	StorageKey string
	Resolved   string
}

type DeleteExpectation struct {
	ExpectedEntityType  string
	ExpectedEntityID    string
	ExpectedContentHash string
	AllowRootDelete     bool
}

type PathGuard interface {
	ResolveRead(rootKind StorageRootKind, storageKey string) (SafePath, error)
	ResolveWrite(rootKind StorageRootKind, storageKey string) (SafePath, error)
	ResolveCreate(rootKind StorageRootKind, storageKey string) (SafePath, error)
	ResolveDelete(rootKind StorageRootKind, storageKey string, expectation DeleteExpectation) (SafePath, error)
	ValidateID(id string) error
	ValidateActionKey(key string) error
}

type defaultPathGuard struct {
	registry PathRootRegistry
	maxDepth int
	maxLen   int
}

func NewPathGuard(registry PathRootRegistry) PathGuard {
	return &defaultPathGuard{
		registry: registry,
		maxDepth: 32,
		maxLen:   255,
	}
}

func (g *defaultPathGuard) ResolveRead(rootKind StorageRootKind, storageKey string) (SafePath, error) {
	root, err := g.registry.Root(rootKind)
	if err != nil {
		return SafePath{}, err
	}
	resolved, err := g.resolve(root, storageKey, false)
	if err != nil {
		return SafePath{}, err
	}
	return SafePath{Root: root, StorageKey: storageKey, Resolved: resolved}, nil
}

func (g *defaultPathGuard) ResolveWrite(rootKind StorageRootKind, storageKey string) (SafePath, error) {
	root, err := g.registry.Root(rootKind)
	if err != nil {
		return SafePath{}, err
	}
	resolved, err := g.resolve(root, storageKey, false)
	if err != nil {
		return SafePath{}, err
	}
	return SafePath{Root: root, StorageKey: storageKey, Resolved: resolved}, nil
}

func (g *defaultPathGuard) ResolveCreate(rootKind StorageRootKind, storageKey string) (SafePath, error) {
	root, err := g.registry.Root(rootKind)
	if err != nil {
		return SafePath{}, err
	}
	resolved, err := g.resolve(root, storageKey, true)
	if err != nil {
		return SafePath{}, err
	}
	return SafePath{Root: root, StorageKey: storageKey, Resolved: resolved}, nil
}

func (g *defaultPathGuard) ResolveDelete(rootKind StorageRootKind, storageKey string, expectation DeleteExpectation) (SafePath, error) {
	if !expectation.AllowRootDelete && storageKey == "" {
		return SafePath{}, ErrRootDelete
	}
	root, err := g.registry.Root(rootKind)
	if err != nil {
		return SafePath{}, err
	}
	if !expectation.AllowRootDelete && root.AbsolutePath == storageKey {
		return SafePath{}, ErrRootDelete
	}
	resolved, err := g.resolve(root, storageKey, false)
	if err != nil {
		return SafePath{}, err
	}
	return SafePath{Root: root, StorageKey: storageKey, Resolved: resolved}, nil
}

func (g *defaultPathGuard) resolve(root CanonicalRoot, storageKey string, allowMissingTail bool) (string, error) {
	if storageKey == "" {
		return "", ErrEmptyPath
	}
	if filepath.IsAbs(storageKey) || strings.HasPrefix(storageKey, "/") || strings.HasPrefix(storageKey, "\\") {
		return "", ErrAbsolutePath
	}
	if strings.Contains(storageKey, "..") {
		return "", ErrPathEscape
	}
	if strings.Contains(storageKey, "\x00") {
		return "", ErrInvalidCharacters
	}
	if runtime.GOOS == "windows" {
		if err := validateWindowsPathComponents(storageKey); err != nil {
			return "", err
		}
	}
	if len(storageKey) > g.maxLen {
		return "", ErrPathTooLong
	}
	cleanKey := filepath.ToSlash(storageKey)
	cleanKey = strings.TrimPrefix(cleanKey, "/")
	if cleanKey == "" {
		return "", ErrEmptyPath
	}
	fullPath := filepath.Join(root.AbsolutePath, cleanKey)
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("storage: failed to resolve path: %w", err)
	}
	rootAbs, err := filepath.Abs(root.AbsolutePath)
	if err != nil {
		return "", fmt.Errorf("storage: failed to resolve root: %w", err)
	}
	absPath = filepath.Clean(absPath)
	rootAbs = filepath.Clean(rootAbs)
	if absPath != rootAbs && !strings.HasPrefix(absPath, rootAbs+string(filepath.Separator)) {
		return "", ErrPathEscape
	}
	if err := validatePathDepth(absPath, rootAbs, g.maxDepth); err != nil {
		return "", err
	}
	if err := validateNoSymlinksAndReparse(rootAbs, absPath, allowMissingTail); err != nil {
		return "", err
	}
	return absPath, nil
}

func (g *defaultPathGuard) ValidateID(id string) error {
	if id == "" {
		return ErrInvalidID
	}
	if id == "." || id == ".." {
		return ErrInvalidID
	}
	if strings.ContainsAny(id, "/\\:\x00*?\"<>|") {
		return ErrInvalidID
	}
	if runtime.GOOS == "windows" {
		upper := strings.ToUpper(id)
		reserved := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
		for _, r := range reserved {
			if upper == r || strings.HasPrefix(upper, r+".") {
				return ErrInvalidID
			}
		}
	}
	return nil
}

func (g *defaultPathGuard) ValidateActionKey(key string) error {
	if key == "" {
		return ErrInvalidID
	}
	for _, c := range key {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			return ErrInvalidID
		}
	}
	return nil
}

func validatePathDepth(absPath, rootAbs string, maxDepth int) error {
	rel, err := filepath.Rel(rootAbs, absPath)
	if err != nil {
		return ErrPathEscape
	}
	depth := 0
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part != "" && part != "." {
			depth++
		}
	}
	if depth > maxDepth {
		return ErrPathTooLong
	}
	return nil
}

func validateNoSymlinksAndReparse(rootAbs, targetPath string, allowMissingTail bool) error {
	rel, err := filepath.Rel(rootAbs, targetPath)
	if err != nil {
		return ErrPathEscape
	}
	parts := strings.Split(rel, string(filepath.Separator))
	current := rootAbs
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		isLast := i == len(parts)-1
		if isLast && allowMissingTail {
			if _, err := os.Lstat(current); err != nil {
				return nil
			}
		}
		info, err := os.Lstat(current)
		if err != nil {
			if !isLast || !allowMissingTail {
				return fmt.Errorf("storage: stat failed: %w", err)
			}
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrSymlinkDetected
		}
		if isWindowsReparsePoint(current, info) {
			return ErrJunctionDetected
		}
	}
	return nil
}

func isWindowsReparsePoint(path string, info fs.FileInfo) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if info == nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0 ||
		info.Mode()&os.ModeDevice != 0 ||
		info.Mode()&os.ModeCharDevice != 0
}

func validateWindowsPathComponents(key string) error {
	parts := strings.Split(key, "/")
	for _, part := range parts {
		if part == "" {
			continue
		}
		trimmed := strings.TrimRight(part, ".")
		trimmed = strings.TrimRight(trimmed, " ")
		if trimmed == "" {
			return ErrInvalidCharacters
		}
		upper := strings.ToUpper(trimmed)
		reserved := []string{"CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9", "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9"}
		for _, r := range reserved {
			if upper == r {
				return ErrReservedName
			}
		}
	}
	return nil
}

func CopyVerifiedFile(src SafePath, dst SafePath, expectedHash string, maxBytes int64) error {
	srcInfo, err := os.Lstat(src.Resolved)
	if err != nil {
		return fmt.Errorf("storage: source stat failed: %w", err)
	}
	if !srcInfo.Mode().IsRegular() {
		return errors.New("storage: source is not a regular file")
	}
	if srcInfo.Size() > maxBytes {
		return errors.New("storage: source exceeds maximum size")
	}
	if err := ensureParentDir(dst.Resolved); err != nil {
		return err
	}
	dstDir, _ := filepath.Split(dst.Resolved)
	tmp, err := os.CreateTemp(dstDir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("storage: failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := copyAndHash(tmp, src.Resolved, expectedHash, maxBytes); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("storage: failed to sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("storage: failed to close: %w", err)
	}
	if err := os.Rename(tmpPath, dst.Resolved); err != nil {
		return fmt.Errorf("storage: failed to rename: %w", err)
	}
	syncParentDir(dstDir)
	_ = success
	return nil
}

func copyAndHash(dst *os.File, srcPath, expectedHash string, maxBytes int64) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("storage: failed to open source: %w", err)
	}
	defer src.Close()
	limited := &limitedWriter{w: dst, remaining: maxBytes}
	buf := make([]byte, 64*1024)
	written, err := copyBuffer(limited, src, buf)
	if err != nil {
		return err
	}
	_ = written
	if expectedHash != "" {
		if err := verifyHash(dst, srcPath, expectedHash); err != nil {
			return err
		}
	}
	return nil
}

func verifyHash(dstFile *os.File, srcPath, expectedHash string) error {
	if expectedHash == "" {
		return nil
	}
	if dstFile == nil {
		return nil
	}
	if _, err := dstFile.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("storage: failed to seek temp file: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, dstFile); err != nil {
		return fmt.Errorf("storage: failed to hash temp file: %w", err)
	}
	computed := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(computed, expectedHash) {
		return errors.New("storage: content hash mismatch after copy")
	}
	return nil
}

type limitedWriter struct {
	w         *os.File
	remaining int64
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > l.remaining {
		return 0, errors.New("storage: write exceeds size limit")
	}
	n, err := l.w.Write(p)
	l.remaining -= int64(n)
	return n, err
}

func copyBuffer(dst *limitedWriter, src interface {
	Read([]byte) (int, error)
}, buf []byte) (int64, error) {
	var written int64
	for {
		nr, rerr := src.Read(buf)
		if nr > 0 {
			nw, werr := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if werr != nil {
				return written, werr
			}
			if nr != nw {
				return written, errors.New("storage: short write")
			}
		}
		if rerr != nil {
			if rerr.Error() == "EOF" {
				return written, nil
			}
			return written, rerr
		}
	}
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0700)
}

func syncParentDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = f.Sync()
	_ = f.Close()
}

func SafeDeleteTree(root CanonicalRoot, targetPath string, expectation DeleteExpectation) (int, int64, error) {
	if !expectation.AllowRootDelete && root.AbsolutePath == targetPath {
		return 0, 0, ErrRootDelete
	}
	rootAbs, err := filepath.Abs(root.AbsolutePath)
	if err != nil {
		return 0, 0, err
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return 0, 0, err
	}
	targetAbs = filepath.Clean(targetAbs)
	rootAbs = filepath.Clean(rootAbs)
	if targetAbs != rootAbs && !strings.HasPrefix(targetAbs, rootAbs+string(filepath.Separator)) {
		return 0, 0, ErrPathEscape
	}
	var deletedCount int
	var deletedBytes int64
	err = filepath.Walk(targetAbs, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == targetAbs {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrSymlinkDetected
		}
		size := info.Size()
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("storage: failed to remove %s: %w", path, err)
		}
		deletedCount++
		deletedBytes += size
		if info.IsDir() {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return deletedCount, deletedBytes, err
	}
	if err := os.Remove(targetAbs); err != nil && !os.IsNotExist(err) {
		return deletedCount, deletedBytes, err
	}
	if targetAbs != rootAbs {
		deletedCount++
	}
	return deletedCount, deletedBytes, nil
}

func InitializeDesktopPetRoots(registry PathRootRegistry, baseDir string) error {
	kinds := []StorageRootKind{
		RootReferenceAssets,
		RootGenerationWork,
		RootGenerationArtifacts,
		RootProcessingWork,
		RootProcessingRevisions,
		RootReleaseWork,
		RootReleasePublished,
		RootReleaseArchives,
		RootInstallations,
		RootInstallationRollback,
		RootInstallationTrash,
		RootEditingUploads,
		RootQualityReports,
		RootImportQuarantine,
		RootMigrationBackup,
	}
	for _, kind := range kinds {
		rel := strings.ReplaceAll(string(kind), "_", "/")
		full := filepath.Join(baseDir, rel)
		if err := registry.Register(kind, full); err != nil {
			return fmt.Errorf("storage: failed to register root %q: %w", kind, err)
		}
		if err := os.MkdirAll(full, 0700); err != nil {
			return fmt.Errorf("storage: failed to create root %q: %w", kind, err)
		}
	}
	return nil
}

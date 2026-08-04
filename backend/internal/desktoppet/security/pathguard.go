// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type StorageRootKind string

const (
	RootGenerationArtifacts StorageRootKind = "generation_artifacts"
	RootProcessingRevisions StorageRootKind = "processing_revisions"
	RootEditingAssets       StorageRootKind = "editing_assets"
	RootQualityReports      StorageRootKind = "quality_reports"
	RootReleasePublished    StorageRootKind = "release_published"
	RootInstallations       StorageRootKind = "installations"
	RootImportQuarantine    StorageRootKind = "import_quarantine"
	RootStorageTrash        StorageRootKind = "storage_trash"
)

type DeleteExpectation struct {
	EntityType string
	EntityID   string
}

type PathRootRegistry struct {
	mu    sync.RWMutex
	roots map[StorageRootKind]string
}

func NewPathRootRegistry() *PathRootRegistry {
	return &PathRootRegistry{roots: make(map[StorageRootKind]string)}
}

func (r *PathRootRegistry) Register(kind StorageRootKind, root string) error {
	if kind == "" {
		return fmt.Errorf("pathguard: root kind required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) {
		resolved = abs
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.roots[kind]; exists {
		return fmt.Errorf("pathguard: root kind already registered: %s", kind)
	}
	r.roots[kind] = resolved
	return nil
}

func (r *PathRootRegistry) Root(kind StorageRootKind) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	root, ok := r.roots[kind]
	if !ok {
		return "", ErrPathEscape
	}
	return root, nil
}

func (r *PathRootRegistry) Resolve(kind StorageRootKind, storageKey string) (string, error) {
	if storageKey == "" || filepath.IsAbs(storageKey) || strings.Contains(storageKey, `\`) {
		return "", ErrUnsafePath
	}
	clean := path.Clean(storageKey)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrPathEscape
	}
	r.mu.RLock()
	root, ok := r.roots[kind]
	r.mu.RUnlock()
	if !ok {
		return "", ErrPathEscape
	}
	candidate := filepath.Join(root, filepath.FromSlash(clean))
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathEscape
	}
	return candidate, nil
}

func (r *PathRootRegistry) Contains(path string) bool {
	_, err := r.resolve(path)
	return err == nil
}

func (r *PathRootRegistry) ResolvePath(filePath string) (string, error) {
	resolved, err := r.resolve(filePath)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (r *PathRootRegistry) resolve(filePath string) (string, error) {
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return r.resolveNonExistent(abs)
		}
		return "", err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, root := range r.roots {
		if resolved == root || strings.HasPrefix(resolved, root+string(filepath.Separator)) {
			return resolved, nil
		}
	}
	return "", ErrPathEscape
}

func (r *PathRootRegistry) resolveNonExistent(path string) (string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, root := range r.roots {
		if resolvedDir == root || strings.HasPrefix(resolvedDir, root+string(filepath.Separator)) {
			return filepath.Join(resolvedDir, base), nil
		}
	}
	return "", ErrPathEscape
}

type SafeArtifactResponder struct {
	registry *PathRootRegistry
}

func NewSafeArtifactResponder(registry *PathRootRegistry) *SafeArtifactResponder {
	return &SafeArtifactResponder{registry: registry}
}

func (s *SafeArtifactResponder) SafeFileResponse(c *gin.Context, filePath string) {
	resolved, err := s.registry.ResolvePath(filePath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	info, err := os.Stat(resolved)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if info.IsDir() {
		c.Status(http.StatusForbidden)
		return
	}
	http.ServeFile(c.Writer, c.Request, resolved)
}

func (s *SafeArtifactResponder) SafeDelete(kind StorageRootKind, storageKey string, expectation DeleteExpectation) error {
	if strings.TrimSpace(expectation.EntityType) == "" || strings.TrimSpace(expectation.EntityID) == "" {
		return ErrUnsafePath
	}
	resolved, err := s.registry.Resolve(kind, storageKey)
	if err != nil {
		return err
	}
	root, err := s.registry.Root(kind)
	if err != nil {
		return err
	}
	if resolved == root {
		return ErrUnsafePath
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || !strings.Contains(filepath.ToSlash(rel), expectation.EntityID) {
		return ErrUnsafePath
	}
	return removeNoSymlinks(resolved)
}

func removeNoSymlinks(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(path)
	}
	if !info.IsDir() {
		return os.Remove(path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := removeNoSymlinks(filepath.Join(path, entry.Name())); err != nil {
			return err
		}
	}
	return os.Remove(path)
}

func (s *SafeArtifactResponder) SafeCopyTree(src, dst string) error {
	srcResolved, err := s.registry.ResolvePath(src)
	if err != nil {
		return err
	}
	dstResolved, err := s.registry.ResolvePath(dst)
	if err != nil {
		return err
	}
	return copyTreeNoSymlinks(srcResolved, dstResolved)
}

func SafeRemoveTree(root string) error {
	return removeNoSymlinks(root)
}

func copyTreeNoSymlinks(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ErrUnsafePath
	}
	if info.IsDir() {
		return copyDirNoSymlinks(src, dst)
	}
	return copyFileNoSymlinks(src, dst, info.Mode().Perm())
}

func copyDirNoSymlinks(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return ErrUnsafePath
		}
		if info.IsDir() {
			if err := copyDirNoSymlinks(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFileNoSymlinks(srcPath, dstPath, info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFileNoSymlinks(src, dst string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, perm)
}

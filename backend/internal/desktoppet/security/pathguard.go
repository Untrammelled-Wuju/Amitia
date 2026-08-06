// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/auth"
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

func (r *PathRootRegistry) CreateAndRegister(kind StorageRootKind, absolutePath string) error {
	if err := os.MkdirAll(absolutePath, 0o700); err != nil {
		return err
	}
	return r.Register(kind, absolutePath)
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

func (r *PathRootRegistry) Contains(p string) bool {
	_, err := r.resolve(p)
	return err == nil
}

func (r *PathRootRegistry) ResolvePath(filePath string) (string, error) {
	resolved, err := r.resolve(filePath)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (r *PathRootRegistry) StorageKeyFromPath(kind StorageRootKind, absolutePath string) (string, error) {
	abs, err := filepath.Abs(absolutePath)
	if err != nil {
		return "", err
	}
	root, err := r.Root(kind)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrPathEscape
	}
	return filepath.ToSlash(rel), nil
}

func (r *PathRootRegistry) MoveToTrash(resolvedPath string, entityID string) error {
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return os.Remove(resolvedPath)
	}
	trashRoot, err := r.Root(RootStorageTrash)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(trashRoot, 0o700); err != nil {
		return err
	}
	timestamp := time.Now().UTC().Format("20060102_150405")
	baseName := filepath.Base(resolvedPath)
	trashName := fmt.Sprintf("%s_%s_%s", timestamp, entityID, baseName)
	destPath := filepath.Join(trashRoot, trashName)
	if err := os.Rename(resolvedPath, destPath); err != nil {
		return err
	}
	return nil
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

func (r *PathRootRegistry) Validate() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for kind, root := range r.roots {
		info, err := os.Lstat(root)
		if err != nil {
			return fmt.Errorf("root %s: %w", kind, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("root %s: %s is not a directory", kind, root)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("root %s: %s is a symlink", kind, root)
		}
		if err := checkWritable(root); err != nil {
			return fmt.Errorf("root %s: %w", kind, err)
		}
	}
	return nil
}

func checkWritable(dir string) error {
	tmpFile := filepath.Join(dir, ".write_test")
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("directory not writable: %w", err)
	}
	f.Close()
	return os.Remove(tmpFile)
}

type ArtifactReference struct {
	ArtifactID  string
	OwnerUserID string
	RootKind    StorageRootKind
	StorageKey  string
	ContentHash string
	ByteSize    int64
	MIME        string
}

type SafeArtifactResponder struct {
	registry *PathRootRegistry
}

func NewSafeArtifactResponder(registry *PathRootRegistry) *SafeArtifactResponder {
	return &SafeArtifactResponder{registry: registry}
}

type SafeTreeDeleter interface {
	SafeDelete(kind StorageRootKind, storageKey string, expectation DeleteExpectation) error
}

func (s *SafeArtifactResponder) ServeArtifact(c *gin.Context, actor *auth.ActorContext, ref ArtifactReference) {
	if actor == nil || ref.OwnerUserID == "" || actor.UserID != ref.OwnerUserID {
		c.Status(http.StatusNotFound)
		return
	}
	resolved, err := s.registry.Resolve(ref.RootKind, ref.StorageKey)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Size() != ref.ByteSize {
		c.Status(http.StatusNotFound)
		return
	}
	actualHash, err := hashFileSHA256(resolved)
	if err != nil || !strings.EqualFold(actualHash, ref.ContentHash) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Type", ref.MIME)
	c.Header("ETag", `"`+ref.ContentHash+`"`)
	c.Header("Cache-Control", "private, max-age=0")
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
	return s.registry.MoveToTrash(resolved, expectation.EntityID)
}

func hashFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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

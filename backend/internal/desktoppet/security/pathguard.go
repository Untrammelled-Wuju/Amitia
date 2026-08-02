// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

type PathRootRegistry struct {
	mu    sync.RWMutex
	roots map[string]struct{}
}

func NewPathRootRegistry() *PathRootRegistry {
	return &PathRootRegistry{roots: make(map[string]struct{})}
}

func (r *PathRootRegistry) Register(roots ...string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			return fmt.Errorf("pathguard: failed to resolve root %q: %w", root, err)
		}
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("pathguard: failed to resolve root symlinks %q: %w", root, err)
			}
			resolved = abs
		}
		r.roots[resolved] = struct{}{}
	}
	return nil
}

func (r *PathRootRegistry) Contains(path string) bool {
	resolved, err := r.resolve(path)
	if err != nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for root := range r.roots {
		if resolved == root || strings.HasPrefix(resolved, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (r *PathRootRegistry) Resolve(path string) (string, error) {
	resolved, err := r.resolve(path)
	if err != nil {
		return "", err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for root := range r.roots {
		if resolved == root || strings.HasPrefix(resolved, root+string(filepath.Separator)) {
			return resolved, nil
		}
	}
	return "", ErrPathEscape
}

func (r *PathRootRegistry) resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
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
	return resolved, nil
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
	for root := range r.roots {
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
	resolved, err := s.registry.Resolve(filePath)
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

func (s *SafeArtifactResponder) SafeDelete(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !s.registry.Contains(abs) {
		return ErrPathEscape
	}
	return removeNoSymlinks(abs)
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
	srcResolved, err := s.registry.Resolve(src)
	if err != nil {
		return err
	}
	dstResolved, err := s.registry.Resolve(dst)
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

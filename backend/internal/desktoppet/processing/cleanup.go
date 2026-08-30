// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CleanupManager struct {
	dataDir string
}

func NewCleanupManager(dataDir string) *CleanupManager {
	return &CleanupManager{dataDir: dataDir}
}

func (c *CleanupManager) CleanupTempDir(taskID string) error {
	target, root, err := c.safeOwnedPath(taskID, "processed", ".tmp")
	if err != nil {
		return err
	}
	return removeOwnedDirWithRetry(root, target, 5)
}

// CleanupProcessingVersion intentionally removes only the mutable .tmp tree.
// Committed version-N history is immutable and must never be deleted by this
// compatibility API.
func (c *CleanupManager) CleanupProcessingVersion(taskID string, processingVersion int) error {
	if processingVersion <= 0 {
		return fmt.Errorf("processingVersion must be positive")
	}
	return c.CleanupTempDir(taskID)
}

func (c *CleanupManager) CleanupActionResources(taskID string, processingVersion int, actionKey string) error {
	if processingVersion <= 0 {
		return fmt.Errorf("processingVersion must be positive")
	}
	if err := validateStorageComponent("actionKey", actionKey); err != nil {
		return err
	}
	target, root, err := c.safeOwnedPath(taskID, "processed", fmt.Sprintf("version-%d", processingVersion), "actions", actionKey)
	if err != nil {
		return err
	}
	return removeOwnedDirWithRetry(root, target, 5)
}

func (c *CleanupManager) CleanupFailedPackage(taskID, packageID string) error {
	if err := validateStorageComponent("packageID", packageID); err != nil {
		return err
	}
	target, root, err := c.safeOwnedPath(taskID, "packages", packageID)
	if err != nil {
		return err
	}
	return removeOwnedDirWithRetry(root, target, 5)
}

func (c *CleanupManager) EnsureVersionDir(taskID string, processingVersion int) (string, error) {
	if processingVersion <= 0 {
		return "", fmt.Errorf("processingVersion must be positive")
	}
	target, root, err := c.safeOwnedPath(taskID, "processed", fmt.Sprintf("version-%d", processingVersion))
	if err != nil {
		return "", err
	}
	if err := mkdirOwnedPath(root, target, 0o755); err != nil {
		return "", fmt.Errorf("create version dir failed: %w", err)
	}
	return target, nil
}

func (c *CleanupManager) EnsureActionsDir(taskID string, processingVersion int, actionKey string) (string, error) {
	if processingVersion <= 0 {
		return "", fmt.Errorf("processingVersion must be positive")
	}
	if err := validateStorageComponent("actionKey", actionKey); err != nil {
		return "", err
	}
	target, root, err := c.safeOwnedPath(taskID, "processed", fmt.Sprintf("version-%d", processingVersion), "actions", actionKey)
	if err != nil {
		return "", err
	}
	if err := mkdirOwnedPath(root, target, 0o755); err != nil {
		return "", fmt.Errorf("create actions dir failed: %w", err)
	}
	return target, nil
}

func (c *CleanupManager) safeOwnedPath(taskID string, components ...string) (string, string, error) {
	if err := validateStorageComponent("taskID", taskID); err != nil {
		return "", "", err
	}
	for index, component := range components {
		if err := validateStorageComponent(fmt.Sprintf("path component %d", index), component); err != nil {
			return "", "", err
		}
	}
	root, err := filepath.Abs(filepath.Join(c.dataDir, "desktop-pets", "generation-tasks"))
	if err != nil {
		return "", "", fmt.Errorf("resolve generation root: %w", err)
	}
	parts := append([]string{root, taskID}, components...)
	target, err := filepath.Abs(filepath.Join(parts...))
	if err != nil {
		return "", "", fmt.Errorf("resolve cleanup target: %w", err)
	}
	if err := ensureStrictDescendant(root, target); err != nil {
		return "", "", err
	}
	return target, root, nil
}

func validateStorageComponent(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is empty", name)
	}
	if value == "." || value == ".." || filepath.IsAbs(value) || strings.ContainsAny(value, `/\\\x00`) {
		return fmt.Errorf("%s is unsafe: %q", name, value)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%s contains control character", name)
		}
	}
	return nil
}

func ensureStrictDescendant(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return fmt.Errorf("resolve owned path: %w", err)
	}
	if rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes generation root: %s", target)
	}
	return nil
}

func mkdirOwnedPath(root, target string, mode os.FileMode) error {
	if err := ensureStrictDescendant(root, target); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("generation root is not a real directory")
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		switch {
		case statErr == nil:
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
				return fmt.Errorf("unsafe directory component: %s", current)
			}
		case os.IsNotExist(statErr):
			if mkErr := os.Mkdir(current, mode); mkErr != nil && !os.IsExist(mkErr) {
				return mkErr
			}
			created, inspectErr := os.Lstat(current)
			if inspectErr != nil {
				return inspectErr
			}
			if created.Mode()&os.ModeSymlink != 0 || !created.IsDir() {
				return fmt.Errorf("created unsafe directory component: %s", current)
			}
		default:
			return statErr
		}
	}
	return nil
}

func removeOwnedDirWithRetry(root, target string, maxAttempts int) error {
	if err := ensureStrictDescendant(root, target); err != nil {
		return err
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ensureNoSymlinkComponents(root, target); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if err := removeTreeNoSymlinks(target); err != nil {
			lastErr = err
		} else if _, err := os.Lstat(target); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("directory still exists after removal: %s", target)
		}
		if attempt+1 < maxAttempts {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to remove directory: %s", target)
	}
	return lastErr
}

func ensureNoSymlinkComponents(root, target string) error {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("generation root is not a real directory")
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed in cleanup path: %s", current)
		}
	}
	return nil
}

func removeTreeNoSymlinks(target string) error {
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to remove symlink cleanup root: %s", target)
	}
	if !info.IsDir() {
		_ = os.Chmod(target, info.Mode()|0o200)
		return os.Remove(target)
	}

	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		child := filepath.Join(target, entry.Name())
		childInfo, statErr := os.Lstat(child)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return statErr
		}
		if childInfo.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(child); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if childInfo.IsDir() {
			if err := removeTreeNoSymlinks(child); err != nil {
				return err
			}
			continue
		}
		_ = os.Chmod(child, childInfo.Mode()|0o200)
		if err := os.Remove(child); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	_ = os.Chmod(target, info.Mode()|0o700)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type CleanupManager struct {
	dataDir string
}

func NewCleanupManager(dataDir string) *CleanupManager {
	return &CleanupManager{dataDir: dataDir}
}

func (c *CleanupManager) CleanupTempDir(taskID string) error {
	if taskID == "" {
		return fmt.Errorf("taskID is empty")
	}
	tmpDir := c.tempDir(taskID)
	return removeDirWithRetry(tmpDir, 5)
}

func (c *CleanupManager) CleanupProcessingVersion(taskID string, processingVersion int) error {
	if taskID == "" {
		return fmt.Errorf("taskID is empty")
	}
	if processingVersion <= 0 {
		return fmt.Errorf("processingVersion must be positive")
	}

	tmpDir := c.tempDir(taskID)
	return removeDirWithRetry(tmpDir, 5)
}

func (c *CleanupManager) CleanupFailedPackage(taskID, packageID string) error {
	if taskID == "" {
		return fmt.Errorf("taskID is empty")
	}
	if packageID == "" {
		return fmt.Errorf("packageID is empty")
	}
	pkgDir := c.packageDir(taskID, packageID)
	return removeDirWithRetry(pkgDir, 5)
}

func (c *CleanupManager) EnsureVersionDir(taskID string, processingVersion int) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskID is empty")
	}
	if processingVersion <= 0 {
		return "", fmt.Errorf("processingVersion must be positive")
	}
	dir := c.versionDir(taskID, processingVersion)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create version dir failed: %w", err)
	}
	return dir, nil
}

func (c *CleanupManager) EnsureActionsDir(taskID string, processingVersion int, actionKey string) (string, error) {
	if taskID == "" {
		return "", fmt.Errorf("taskID is empty")
	}
	if processingVersion <= 0 {
		return "", fmt.Errorf("processingVersion must be positive")
	}
	if actionKey == "" {
		return "", fmt.Errorf("actionKey is empty")
	}
	dir := c.actionsDir(taskID, processingVersion, actionKey)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create actions dir failed: %w", err)
	}
	return dir, nil
}

func (c *CleanupManager) processedDir(taskID string) string {
	return filepath.Join(c.dataDir, "desktop-pets", "generation-tasks", taskID, "processed")
}

func (c *CleanupManager) tempDir(taskID string) string {
	return filepath.Join(c.processedDir(taskID), ".tmp")
}

func (c *CleanupManager) versionDir(taskID string, processingVersion int) string {
	return filepath.Join(c.processedDir(taskID), fmt.Sprintf("version-%d", processingVersion))
}

func (c *CleanupManager) actionsDir(taskID string, processingVersion int, actionKey string) string {
	return filepath.Join(c.versionDir(taskID, processingVersion), "actions", actionKey)
}

func (c *CleanupManager) packageDir(taskID, packageID string) string {
	return filepath.Join(c.dataDir, "desktop-pets", "generation-tasks", taskID, "packages", packageID)
}

func removeDirWithRetry(dir string, maxAttempts int) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		if err := os.RemoveAll(dir); err != nil {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
			continue
		}
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return nil
		}
		if runtime.GOOS == "windows" {
			if err := powershellRemoveDir(dir); err == nil {
				time.Sleep(100 * time.Millisecond)
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					return nil
				}
			}
		}
		lastErr = fmt.Errorf("directory still exists after removal: %s", dir)
		time.Sleep(200 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to remove directory after %d attempts: %s", maxAttempts, dir)
	}
	return lastErr
}

func powershellRemoveDir(dir string) error {
	cmd := exec.Command("pwsh", "-NoProfile", "-Command", fmt.Sprintf("Remove-Item -LiteralPath '%s' -Recurse -Force", dir))
	return cmd.Run()
}

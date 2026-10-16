package gitworktree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (m *Manager) recoverFailedCreate(ctx context.Context, repoRoot, id, target string, originalErr error) error {
	recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	entries, _, err := m.list(recoveryCtx, repoRoot)
	if err != nil {
		return fmt.Errorf("%w; worktree recovery inspection failed: %w", originalErr, err)
	}
	for _, item := range entries {
		if item.ID != id || !item.Managed || filepath.Clean(item.Path) != filepath.Clean(target) {
			continue
		}
		if item.TaskBinding != nil || item.Locked || item.Prunable {
			return fmt.Errorf("%w; worktree recovery refused unsafe state", originalErr)
		}
		if _, err := m.removeWithMergeLock(recoveryCtx, repoRoot, id); err != nil {
			return fmt.Errorf("%w; worktree recovery removal failed: %w", originalErr, err)
		}
		return originalErr
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("%w; worktree recovery found unregistered directory", originalErr)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("%w; worktree recovery inspection failed: %w", originalErr, err)
	}
	return originalErr
}

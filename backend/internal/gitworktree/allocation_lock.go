package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var ErrAllocationLockUnavailable = errors.New("worktree allocation lock is busy or unavailable")

func (m *Manager) lockRepositoryAllocation(ctx context.Context, repoRoot string) (func(), error) {
	_, key, err := m.repository(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(m.managedRoot(key), ".bindings")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || metadataFilePermissionsUnsafe(info.Mode().Perm()) {
		return nil, ErrInvalidArgument
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(directory) {
		return nil, ErrInvalidArgument
	}
	timeout := 5 * time.Second
	if deadline, bounded := ctx.Deadline(); bounded && time.Until(deadline) < timeout {
		timeout = time.Until(deadline)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, context.DeadlineExceeded
	}
	unlock, err := acquireMergeFileLock(ctx, filepath.Join(directory, ".allocate.lock"), timeout)
	if errors.Is(err, ErrMergeLockUnavailable) {
		return nil, ErrAllocationLockUnavailable
	}
	return unlock, err
}

package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var ErrMergeLockUnavailable = errors.New("merge lock is busy or unavailable")

func (m *Manager) lockRepositoryMerge(ctx context.Context, repoRoot string) (func(), error) {
	_, key, err := m.repository(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(m.managedRoot(key), ".merges")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || metadataFilePermissionsUnsafe(info.Mode().Perm()) {
		return nil, ErrInvalidArgument
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(dir) {
		return nil, ErrInvalidArgument
	}
	deadline, bounded := ctx.Deadline()
	timeout := 5 * time.Second
	if bounded && time.Until(deadline) < timeout {
		timeout = time.Until(deadline)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		return nil, context.DeadlineExceeded
	}
	return acquireMergeFileLock(ctx, filepath.Join(dir, ".apply.lock"), timeout)
}

func mergeLockExpiry(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, set := ctx.Deadline(); set && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return ErrMergeLockUnavailable
}

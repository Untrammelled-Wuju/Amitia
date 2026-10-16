package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var ErrCheckpointTestRunning = errors.New("checkpoint test is running or its execution lock is busy")

func (m *Manager) lockCheckpointExecution(ctx context.Context, repoRoot, id string) (func(), error) {
	if !managedID.MatchString(id) {
		return nil, ErrInvalidArgument
	}
	_, key, err := m.repository(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(m.managedRoot(key), ".checkpoints")
	if err = os.MkdirAll(dir, 0700); err != nil {
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
	wait := 150 * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < wait {
		wait = time.Until(deadline)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if wait <= 0 {
		return nil, context.DeadlineExceeded
	}
	unlock, err := acquireMergeFileLock(ctx, filepath.Join(dir, id+".test.lock"), wait)
	if errors.Is(err, ErrMergeLockUnavailable) {
		return nil, ErrCheckpointTestRunning
	}
	return unlock, err
}

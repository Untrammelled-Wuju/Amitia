package gitworktree

import (
	"context"
	amitialog "log"
	"os"
	"syscall"
	"time"
)

func acquireMergeFileLock(ctx context.Context, path string, timeout time.Duration) (func(), error) {
	if info, err := os.Lstat(path); err == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0) {
		return nil, ErrInvalidArgument
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		if err := f.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return nil, ErrInvalidArgument
	}
	deadline := time.Now().Add(timeout)
	for {
		if err = ctx.Err(); err != nil {
			if err := f.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, err
		}
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN && err != syscall.EINTR {
			if err := f.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, err
		}
		if !time.Now().Before(deadline) {
			if err := f.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, mergeLockExpiry(ctx)
		}
		time.Sleep(12 * time.Millisecond)
	}
	return func() {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := f.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}, nil
}

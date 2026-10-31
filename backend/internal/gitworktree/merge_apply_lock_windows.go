package gitworktree

import (
	"context"
	amitialog "log"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var mergeKernel32 = syscall.NewLazyDLL("kernel32.dll")
var mergeLockFile = mergeKernel32.NewProc("LockFileEx")
var mergeUnlockFile = mergeKernel32.NewProc("UnlockFileEx")

func acquireMergeFileLock(ctx context.Context, path string, timeout time.Duration) (func(), error) {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return nil, ErrInvalidArgument
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		if err := f.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return nil, ErrInvalidArgument
	}
	ol := &syscall.Overlapped{}
	deadline := time.Now().Add(timeout)
	for {
		if err = ctx.Err(); err != nil {
			if err := f.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, err
		}
		result, _, callErr := mergeLockFile.Call(f.Fd(), 0x3, 0, 1, 0, uintptr(unsafe.Pointer(ol)))
		runtime.KeepAlive(ol)
		if result != 0 {
			break
		}
		if callErr != syscall.Errno(33) && callErr != syscall.Errno(158) && callErr != syscall.Errno(167) {
			if err := f.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, callErr
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
		_, _, _ = mergeUnlockFile.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(ol)))
		runtime.KeepAlive(ol)
		if err := f.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}, nil
}

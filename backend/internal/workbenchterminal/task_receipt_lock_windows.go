package workbenchterminal

import (
	amitialog "log"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var receiptKernel32 = syscall.NewLazyDLL("kernel32.dll")
var receiptLockFile = receiptKernel32.NewProc("LockFileEx")
var receiptUnlockFile = receiptKernel32.NewProc("UnlockFileEx")

func lockReceiptJournal(path string, timeout time.Duration) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		if err := file.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return nil, ErrInvalid
	}
	overlapped := &syscall.Overlapped{}
	deadline := time.Now().Add(timeout)
	for {
		code, _, lockErr := receiptLockFile.Call(file.Fd(), 0x3, 0, 1, 0, uintptr(unsafe.Pointer(overlapped)))
		runtime.KeepAlive(overlapped)
		if code != 0 {
			break
		}
		if lockErr != syscall.Errno(33) && lockErr != syscall.Errno(158) && lockErr != syscall.Errno(167) {
			if err := file.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, lockErr
		}
		if time.Now().After(deadline) {
			if err := file.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, ErrReceiptUnavailable
		}
		time.Sleep(15 * time.Millisecond)
	}
	return func() {
		_, _, _ = receiptUnlockFile.Call(file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(overlapped)))
		runtime.KeepAlive(overlapped)
		if err := file.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}, nil
}

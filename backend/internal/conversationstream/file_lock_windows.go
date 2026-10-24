package conversationstream

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

var (
	lockFileKernel      = syscall.NewLazyDLL("kernel32.dll")
	lockFileProcedure   = lockFileKernel.NewProc("LockFileEx")
	unlockFileProcedure = lockFileKernel.NewProc("UnlockFileEx")
)

func tryFileLock(file *os.File) (bool, error) {
	var overlapped syscall.Overlapped
	result, _, err := lockFileProcedure.Call(file.Fd(), uintptr(3), uintptr(0), uintptr(1), uintptr(0), uintptr(unsafe.Pointer(&overlapped)))
	if result != 0 {
		return true, nil
	}
	if errors.Is(err, syscall.Errno(33)) {
		return false, nil
	}
	return false, err
}

func unlockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	result, _, err := unlockFileProcedure.Call(file.Fd(), uintptr(0), uintptr(1), uintptr(0), uintptr(unsafe.Pointer(&overlapped)))
	if result == 0 {
		return err
	}
	return nil
}

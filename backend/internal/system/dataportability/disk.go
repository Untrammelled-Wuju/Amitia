//go:build !android

package dataportability

import (
	"syscall"
	"unsafe"
)

func GetDiskFreeSpace(path string) (uint64, error) {
	kernel32, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return 0, err
	}
	proc, err := kernel32.FindProc("GetDiskFreeSpaceExW")
	if err != nil {
		return 0, err
	}

	var freeBytes uint64
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}

	ret, _, _ := proc.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytes)),
		0,
		0,
	)
	if ret == 0 {
		return 0, syscall.GetLastError()
	}
	return freeBytes, nil
}

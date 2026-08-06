//go:build windows

package kernel

import (
	"syscall"
	"testing"
	"unsafe"
)

func r64ProcessHandleCount(t *testing.T) uint32 {
	t.Helper()
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	var count uint32
	handle, _, err := proc.Call(uintptr(^uintptr(0)), uintptr(unsafe.Pointer(&count)))
	if handle == 0 {
		t.Skipf("GetProcessHandleCount unavailable: %v", err)
	}
	return count
}

//go:build !windows

package kernel

import (
	"os"
	"testing"
)

func r64ProcessHandleCount(t *testing.T) uint32 {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skipf("cannot count open fds: %v", err)
	}
	return uint32(len(entries))
}

//go:build !windows

package kernel

import "golang.org/x/sys/unix"

func syscallNofollow() int {
	return unix.O_NOFOLLOW
}

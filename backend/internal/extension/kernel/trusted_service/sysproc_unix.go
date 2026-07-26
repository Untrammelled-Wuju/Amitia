//go:build !windows

package trusted_service

import "syscall"

func newPlatformSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

//go:build !windows

package kernel

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func syscallNofollow() int {
	return unix.O_NOFOLLOW
}

func validatePlatformPathComponent(component string) error {
	if component == "." || component == ".." {
		return fmt.Errorf("kernel: path component %q is forbidden", component)
	}
	for _, r := range component {
		if r == 0 {
			return fmt.Errorf("kernel: path component contains null byte")
		}
	}
	return nil
}

func safeCreateFilePlatform(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_WRONLY|unix.O_NOFOLLOW|unix.O_EXCL, 0600)
	if err != nil {
		return nil, fmt.Errorf("kernel: safe create file %s: %w", path, err)
	}
	return os.NewFile(uintptr(fd), path), nil
}

func validateReparsePoint(path string) error {
	var stat unix.Stat_t
	err := unix.Lstat(path, &stat)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("kernel: lstat %s: %w", path, err)
	}
	if stat.Mode&syscall.S_IFMT == syscall.S_IFLNK {
		return fmt.Errorf("kernel: path %s is a symlink (reparse point), forbidden", path)
	}
	return nil
}

//go:build windows

package kernel

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func syscallNofollow() int {
	return 0
}

func validatePlatformPathComponent(component string) error {
	if component == "." || component == ".." {
		return fmt.Errorf("kernel: path component %q is forbidden", component)
	}
	for _, r := range component {
		if r == 0 {
			return fmt.Errorf("kernel: path component contains null byte")
		}
		if r == '/' || r == '\\' {
			return fmt.Errorf("kernel: path component %q contains separator character", component)
		}
	}
	return nil
}

func safeCreateFilePlatform(path string) (*os.File, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("kernel: utf16 path %s: %w", path, err)
	}
	h, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_WRITE,
		0,
		nil,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("kernel: safe create file %s: %w", path, err)
	}
	return os.NewFile(uintptr(h), path), nil
}

func validateReparsePoint(path string) error {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("kernel: utf16 path %s: %w", path, err)
	}
	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("kernel: GetFileAttributes %s: %w", path, err)
	}
	if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return fmt.Errorf("kernel: path %s is a reparse point, forbidden", path)
	}
	return nil
}

func createHardLinkWindows(newName, oldName string) error {
	newPtr, err := windows.UTF16PtrFromString(newName)
	if err != nil {
		return fmt.Errorf("kernel: utf16 path %s: %w", newName, err)
	}
	oldPtr, err := windows.UTF16PtrFromString(oldName)
	if err != nil {
		return fmt.Errorf("kernel: utf16 path %s: %w", oldName, err)
	}
	return windows.CreateHardLink(newPtr, oldPtr, 0)
}

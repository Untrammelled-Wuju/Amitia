//go:build windows

package platform

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

func processImagePath(pid int) (string, error) {
	if pid <= 0 {
		return "", errors.New("invalid pid")
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buffer[:size]), nil
}

func terminateProcess(pid int) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	if err := windows.TerminateProcess(handle, 1); err != nil {
		return err
	}
	event, err := windows.WaitForSingleObject(handle, uint32((5 * time.Second).Milliseconds()))
	if err != nil {
		return err
	}
	if event != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("process %d did not exit", pid)
	}
	return nil
}

func sameProcessImage(currentImage, targetImage string) bool {
	clean := func(value string) string {
		value = strings.TrimSuffix(strings.TrimSpace(value), " (deleted)")
		resolved, err := filepath.EvalSymlinks(value)
		if err == nil {
			value = resolved
		}
		return filepath.Clean(value)
	}
	return strings.EqualFold(clean(currentImage), clean(targetImage))
}

//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func processImagePath(pid int) (string, error) {
	if pid <= 0 {
		return "", errors.New("invalid pid")
	}
	value, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return "", err
	}
	return value, nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func terminateProcess(pid int) error {
	if pid <= 0 {
		return errors.New("invalid pid")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	_ = proc.Signal(syscall.SIGTERM)
	deadline := time.Now().Add(5 * time.Second)
	for processAlive(pid) {
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !processAlive(pid) {
		return nil
	}
	if err := proc.Kill(); err != nil {
		return err
	}
	deadline = time.Now().Add(5 * time.Second)
	for processAlive(pid) {
		if time.Now().After(deadline) {
			return fmt.Errorf("process %d did not exit", pid)
		}
		time.Sleep(100 * time.Millisecond)
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
	return clean(currentImage) == clean(targetImage)
}

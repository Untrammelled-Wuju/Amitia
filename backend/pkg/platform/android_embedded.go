// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build android

package platform

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type androidPlatform struct{}

func Detect() RuntimePlatform {
	return androidPlatform{}
}

func (androidPlatform) Name() string {
	return "android-embedded"
}

func (androidPlatform) ExecutableSuffix() string {
	return ""
}

func (androidPlatform) BinarySuffix() string {
	return ""
}

func (androidPlatform) RootFSDir() string {
	if v := os.Getenv("AMITIA_ROOTFS_DIR"); v != "" {
		return v
	}
	if absDir := os.Getenv("AMITIA_DATA_DIR"); absDir != "" {
		return absDir
	}
	return ""
}

func (androidPlatform) DefaultDataDir() string {
	return "data"
}

func (androidPlatform) IsWindows() bool {
	return false
}

func (androidPlatform) IsLinux() bool {
	return true
}

func (androidPlatform) IsAndroid() bool {
	return true
}

func (androidPlatform) IsAndroidEmbedded() bool {
	return true
}

func (p androidPlatform) KillExistingServer(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return nil
	}
	conn.Close()

	dataDir := p.DefaultDataDir()
	if absDir := os.Getenv("AMITIA_DATA_DIR"); absDir != "" {
		dataDir = absDir
	}

	pid, pidErr := p.ReadPidFile(dataDir)
	if pidErr != nil {
		return nil
	}
	if pid <= 0 {
		return nil
	}

	proc, findErr := os.FindProcess(pid)
	if findErr != nil {
		return nil
	}
	_ = proc.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = proc.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = proc.Kill()
	}
	time.Sleep(1 * time.Second)
	return nil
}

func (androidPlatform) WritePidFile(dataDir string) error {
	if dataDir == "" {
		dataDir = "data"
	}
	if absDir := os.Getenv("AMITIA_DATA_DIR"); absDir != "" {
		dataDir = absDir
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func (androidPlatform) ReadPidFile(dataDir string) (int, error) {
	if dataDir == "" {
		dataDir = "data"
	}
	if absDir := os.Getenv("AMITIA_DATA_DIR"); absDir != "" {
		dataDir = absDir
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if parseErr != nil {
		return 0, parseErr
	}
	if pid <= 0 {
		return 0, errors.New("invalid pid")
	}
	return pid, nil
}

func (androidPlatform) RemovePidFile(dataDir string) error {
	if dataDir == "" {
		dataDir = "data"
	}
	if absDir := os.Getenv("AMITIA_DATA_DIR"); absDir != "" {
		dataDir = absDir
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	return os.Remove(pidPath)
}

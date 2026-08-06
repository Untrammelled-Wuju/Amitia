// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux && !android

package platform

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type linuxPlatform struct{}

func Detect() RuntimePlatform {
	mode := os.Getenv(RuntimeModeEnv)
	if IsAndroidPRootMode(mode) {
		return androidPootPlatform{}
	}
	return linuxPlatform{}
}

func (linuxPlatform) Name() string {
	return "desktop-linux"
}

func (linuxPlatform) Descriptor() RuntimeDescriptor {
	return newRuntimeDescriptor(HostPlatformLinux, RuntimeKindNativeProcess, GuestPlatformLinux)
}

func (linuxPlatform) ExecutableSuffix() string {
	return ""
}

func (linuxPlatform) BinarySuffix() string {
	return ""
}

func (linuxPlatform) RootFSDir() string {
	if v := os.Getenv("AMITIA_ROOTFS_DIR"); v != "" {
		return v
	}
	return ""
}

func (linuxPlatform) DefaultDataDir() string {
	if v := os.Getenv("AMITIA_DATA_DIR"); v != "" {
		return v
	}
	return "data"
}

func (linuxPlatform) IsWindows() bool {
	return false
}

func (linuxPlatform) IsLinux() bool {
	return true
}

func (linuxPlatform) IsAndroid() bool {
	return false
}

func (linuxPlatform) IsAndroidEmbedded() bool {
	return false
}

func (p linuxPlatform) KillExistingServer(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return nil
	}
	conn.Close()

	if _, _, splitErr := net.SplitHostPort(addr); splitErr != nil {
		return fmt.Errorf("parse addr failed: %w", splitErr)
	}

	dataDir := p.DefaultDataDir()

	if pid, pidErr := p.ReadPidFile(dataDir); pidErr == nil && pid > 0 {
		if pid == os.Getpid() {
			return fmt.Errorf("port occupied by current process pid=%d", pid)
		}
		if killErr := killPid(pid); killErr == nil {
			time.Sleep(2 * time.Second)
			return nil
		}
		_ = p.RemovePidFile(dataDir)
		return fmt.Errorf("port occupied by pid=%d (process not responsive)", pid)
	}

	return fmt.Errorf("port occupied by unknown process (no valid pid file found)")
}

func (linuxPlatform) WritePidFile(dataDir string) error {
	if dataDir == "" {
		dataDir = linuxPlatform{}.DefaultDataDir()
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func (linuxPlatform) ReadPidFile(dataDir string) (int, error) {
	if dataDir == "" {
		dataDir = linuxPlatform{}.DefaultDataDir()
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (linuxPlatform) RemovePidFile(dataDir string) error {
	if dataDir == "" {
		dataDir = linuxPlatform{}.DefaultDataDir()
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	return os.Remove(pidPath)
}

var _ RuntimePlatform = linuxPlatform{}

func killPid(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		_, _ = proc.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		_ = proc.Kill()
		return nil
	}
}

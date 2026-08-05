// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux && !android

package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
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

	_, port, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return fmt.Errorf("parse addr failed: %w", splitErr)
	}

	dataDir := p.DefaultDataDir()

	if pid, pidErr := p.ReadPidFile(dataDir); pidErr == nil && pid > 0 {
		if killErr := killPid(pid); killErr == nil {
			time.Sleep(2 * time.Second)
			return nil
		}
	}

	if path, lookupErr := exec.LookPath("fuser"); lookupErr == nil {
		_ = exec.Command(path, "-k", port+"/tcp").Run()
		time.Sleep(2 * time.Second)
		return nil
	}

	if path, lookupErr := exec.LookPath("lsof"); lookupErr == nil {
		out, _ := exec.Command(path, "-ti", ":"+port).Output()
		for _, raw := range strings.Fields(string(out)) {
			if pid, err2 := strconv.Atoi(raw); err2 == nil {
				_ = killPid(pid)
			}
		}
		time.Sleep(2 * time.Second)
	}
	return nil
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

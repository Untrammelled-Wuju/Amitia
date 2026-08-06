// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package platform

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type windowsPlatform struct{}

func Detect() RuntimePlatform {
	return windowsPlatform{}
}

func (windowsPlatform) Name() string {
	return "desktop-windows"
}

func (windowsPlatform) Descriptor() RuntimeDescriptor {
	return newRuntimeDescriptor(HostPlatformWindows, RuntimeKindNativeProcess, GuestPlatformWindows)
}

var _ RuntimePlatform = windowsPlatform{}

func (windowsPlatform) ExecutableSuffix() string {
	return ".exe"
}

func (windowsPlatform) BinarySuffix() string {
	return ".exe"
}

func (windowsPlatform) RootFSDir() string {
	return ""
}

func (windowsPlatform) DefaultDataDir() string {
	return "data"
}

func (windowsPlatform) IsWindows() bool {
	return true
}

func (windowsPlatform) IsLinux() bool {
	return false
}

func (windowsPlatform) IsAndroid() bool {
	return false
}

func (windowsPlatform) IsAndroidEmbedded() bool {
	return false
}

func (p windowsPlatform) KillExistingServer(addr string) error {
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
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			_ = proc.Signal(os.Interrupt)
			done := make(chan struct{})
			go func() {
				_, _ = proc.Wait()
				close(done)
			}()
			select {
			case <-done:
				time.Sleep(1 * time.Second)
				return nil
			case <-time.After(2 * time.Second):
				_ = proc.Kill()
				time.Sleep(1 * time.Second)
				return nil
			}
		}
		_ = p.RemovePidFile(dataDir)
		return fmt.Errorf("port occupied by pid=%d (process not responsive)", pid)
	}

	return fmt.Errorf("port occupied by unknown process (no valid pid file found)")
}

func (windowsPlatform) WritePidFile(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	return os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func (windowsPlatform) ReadPidFile(dataDir string) (int, error) {
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func (windowsPlatform) RemovePidFile(dataDir string) error {
	pidPath := filepath.Join(dataDir, ".amitia-backend.pid")
	return os.Remove(pidPath)
}

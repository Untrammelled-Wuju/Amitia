// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
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

func (windowsPlatform) KillExistingServer(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return nil
	}
	conn.Close()

	_, port, splitErr := net.SplitHostPort(addr)
	if splitErr != nil {
		return fmt.Errorf("parse addr failed: %w", splitErr)
	}

	out, _ := exec.Command("cmd", "/c", "netstat -ano | findstr :"+port+" | findstr LISTENING").Output()
	fields := strings.Fields(string(out))
	selfPid := os.Getpid()
	for _, f := range fields {
		pid, err2 := strconv.Atoi(f)
		if err2 != nil {
			continue
		}
		if pid == selfPid {
			continue
		}
		_ = exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
	}
	time.Sleep(2 * time.Second)
	return nil
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

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func (p windowsPlatform) KillExistingServer(addr, dataDir string) error {
	if dataDir == "" {
		dataDir = p.DefaultDataDir()
	}
	return killExistingServer(addr, dataDir, p.ReadPidFile, p.RemovePidFile)
}

func (windowsPlatform) WritePidFile(dataDir string) error {
	return writePidFile(dataDir)
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

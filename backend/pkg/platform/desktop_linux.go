// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux && !android

package platform

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func (p linuxPlatform) KillExistingServer(addr, dataDir string) error {
	if dataDir == "" {
		dataDir = p.DefaultDataDir()
	}
	return killExistingServer(addr, dataDir, p.ReadPidFile, p.RemovePidFile)
}

func (linuxPlatform) WritePidFile(dataDir string) error {
	if dataDir == "" {
		dataDir = linuxPlatform{}.DefaultDataDir()
	}
	return writePidFile(dataDir)
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

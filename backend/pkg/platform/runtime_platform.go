// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package platform

import "strings"

const RuntimeModeEnv = "AMITIA_RUNTIME_MODE"

const AndroidPRootMode = "android-proot"

func NormalizeRuntimeMode(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func IsAndroidPRootMode(v string) bool {
	return NormalizeRuntimeMode(v) == AndroidPRootMode
}

type RuntimePlatform interface {
	Name() string
	KillExistingServer(addr string) error
	ExecutableSuffix() string
	BinarySuffix() string
	RootFSDir() string
	DefaultDataDir() string
	IsWindows() bool
	IsLinux() bool
	IsAndroid() bool
	IsAndroidEmbedded() bool
	WritePidFile(dataDir string) error
	ReadPidFile(dataDir string) (int, error)
	RemovePidFile(dataDir string) error
}

var current RuntimePlatform

func Set(p RuntimePlatform) {
	current = p
}

func Get() RuntimePlatform {
	if current == nil {
		current = Detect()
	}
	return current
}

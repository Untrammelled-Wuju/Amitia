// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package platform

type RuntimePlatform interface {
	Name() string
	KillExistingServer(addr string) error
	ExecutableSuffix() string
	DefaultDataDir() string
	IsWindows() bool
	IsLinux() bool
	IsAndroidEmbedded() bool
	WritePidFile(dataDir string) error
	ReadPidFile(dataDir string) (int, error)
	RemovePidFile(dataDir string) error
}

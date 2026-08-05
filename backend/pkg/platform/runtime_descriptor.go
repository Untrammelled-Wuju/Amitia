// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package platform

import "runtime"

type HostPlatform string

const (
	HostPlatformUnknown HostPlatform = "unknown"
	HostPlatformAndroid HostPlatform = "android"
	HostPlatformIOS     HostPlatform = "ios"
	HostPlatformWindows HostPlatform = "windows"
	HostPlatformMacOS   HostPlatform = "macos"
	HostPlatformLinux   HostPlatform = "linux"
)

type RuntimeKind string

const (
	RuntimeKindUnknown       RuntimeKind = "unknown"
	RuntimeKindNativeProcess RuntimeKind = "native-process"
	RuntimeKindProot         RuntimeKind = "proot"
	RuntimeKindEmbedded      RuntimeKind = "embedded"
	RuntimeKindSandbox       RuntimeKind = "sandbox"
	RuntimeKindRemote        RuntimeKind = "remote"
)

type GuestPlatform string

const (
	GuestPlatformUnknown GuestPlatform = "unknown"
	GuestPlatformNone    GuestPlatform = "none"
	GuestPlatformAndroid GuestPlatform = "android"
	GuestPlatformIOS     GuestPlatform = "ios"
	GuestPlatformWindows GuestPlatform = "windows"
	GuestPlatformMacOS   GuestPlatform = "macos"
	GuestPlatformLinux   GuestPlatform = "linux"
)

type RuntimeDescriptor struct {
	Host         HostPlatform
	Kind         RuntimeKind
	Guest        GuestPlatform
	Architecture string
}

func newRuntimeDescriptor(host HostPlatform, kind RuntimeKind, guest GuestPlatform) RuntimeDescriptor {
	return RuntimeDescriptor{
		Host:         host,
		Kind:         kind,
		Guest:        guest,
		Architecture: runtime.GOARCH,
	}
}

func hostPlatformFromGOOS(goos string) HostPlatform {
	switch goos {
	case "windows":
		return HostPlatformWindows
	case "linux":
		return HostPlatformLinux
	case "android":
		return HostPlatformAndroid
	case "darwin":
		return HostPlatformMacOS
	case "ios":
		return HostPlatformIOS
	default:
		return HostPlatformUnknown
	}
}

func guestPlatformFromGOOS(goos string) GuestPlatform {
	switch goos {
	case "windows":
		return GuestPlatformWindows
	case "linux":
		return GuestPlatformLinux
	case "android":
		return GuestPlatformAndroid
	case "darwin":
		return GuestPlatformMacOS
	case "ios":
		return GuestPlatformIOS
	default:
		return GuestPlatformUnknown
	}
}

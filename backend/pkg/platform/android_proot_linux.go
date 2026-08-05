// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build linux && !android

package platform

type androidPootPlatform struct {
	linuxPlatform
}

var _ RuntimePlatform = androidPootPlatform{}

func (androidPootPlatform) Name() string {
	return "android-proot"
}

func (androidPootPlatform) IsAndroid() bool {
	return true
}

func (androidPootPlatform) IsAndroidEmbedded() bool {
	return true
}

func (androidPootPlatform) Descriptor() RuntimeDescriptor {
	return newRuntimeDescriptor(HostPlatformAndroid, RuntimeKindProot, GuestPlatformLinux)
}

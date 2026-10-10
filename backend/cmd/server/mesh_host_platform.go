package main

import (
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/platform"
)

func devicePlatformFromDescriptor(descriptor platform.RuntimeDescriptor, guestOS string) runtimeidentity.Platform {
	switch descriptor.Host {
	case platform.HostPlatformAndroid:
		return runtimeidentity.PlatformAndroid
	case platform.HostPlatformIOS:
		return runtimeidentity.PlatformIOS
	case platform.HostPlatformWindows:
		return runtimeidentity.PlatformWindows
	case platform.HostPlatformMacOS:
		return runtimeidentity.PlatformDarwin
	case platform.HostPlatformLinux:
		return runtimeidentity.PlatformLinux
	default:
		return platformFromGOOS(guestOS)
	}
}

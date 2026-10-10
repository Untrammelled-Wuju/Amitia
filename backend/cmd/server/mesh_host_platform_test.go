package main

import (
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/pkg/platform"
	"testing"
)

func TestDevicePlatformUsesActualHostInsteadOfLinuxGuest(t *testing.T) {
	for _, test := range []struct {
		host     platform.HostPlatform
		guest    string
		expected runtimeidentity.Platform
	}{
		{platform.HostPlatformIOS, "linux", runtimeidentity.PlatformIOS},
		{platform.HostPlatformAndroid, "linux", runtimeidentity.PlatformAndroid},
		{platform.HostPlatformMacOS, "darwin", runtimeidentity.PlatformDarwin},
		{platform.HostPlatformWindows, "windows", runtimeidentity.PlatformWindows},
		{platform.HostPlatformLinux, "linux", runtimeidentity.PlatformLinux},
		{platform.HostPlatformUnknown, "android", runtimeidentity.PlatformAndroid},
	} {
		if actual := devicePlatformFromDescriptor(platform.RuntimeDescriptor{Host: test.host, Guest: platform.GuestPlatformLinux}, test.guest); actual != test.expected {
			t.Fatalf("host=%s guest=%s actual=%s expected=%s", test.host, test.guest, actual, test.expected)
		}
	}
}

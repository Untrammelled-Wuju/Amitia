package runtimehost

import (
	"github.com/u-ai/backend/pkg/platform"
	"testing"
)

func TestIOSGuestHostCanRunServicesWithoutClaimingTaskIsolation(t *testing.T) {
	host, err := NewRuntimeHost(HostBuildContext{Descriptor: platform.RuntimeDescriptor{Host: platform.HostPlatformIOS, Guest: platform.GuestPlatformLinux, Kind: platform.RuntimeKindEmulated, Architecture: "arm64"}})
	if err != nil {
		t.Fatal(err)
	}
	if !host.Capabilities().Supports(CapProcessSpawn) || !host.Capabilities().Supports(CapRuntimeIOSNative) {
		t.Fatal("guest service or native transport unavailable")
	}
	if host.Capabilities().Support(CapRuntimeSandboxedExec) != SupportUnsupported {
		t.Fatal("emulation incorrectly claims task isolation")
	}
}

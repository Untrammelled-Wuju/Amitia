package main

import (
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/runtimeorchestrator/builtin"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/pkg/platform"
	"testing"
)

func TestIOSGuestRuntimeRegistersActualNativeProviderAndRelay(t *testing.T) {
	descriptor := platform.RuntimeDescriptor{Host: platform.HostPlatformIOS, Kind: platform.RuntimeKindEmulated, Guest: platform.GuestPlatformLinux, Architecture: "arm64"}
	host, err := runtimehost.NewRuntimeHost(runtimehost.HostBuildContext{Descriptor: descriptor})
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := &runtimeBootstrap{host: host, providerRegistry: runtimeorchestrator.NewProviderRegistry(), orchestrator: runtimeorchestrator.NewWithProfile(descriptor, runtimeprofile.ProfileLocal)}
	if bridge := bootstrap.buildIOSNativeBridge(); bridge == nil {
		t.Fatal("Linux guest native bridge missing")
	}
	if err := bootstrap.registerProviderFactoriesIOS(); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.buildPlatformProvidersIOS(); err != nil {
		t.Fatal(err)
	}
	if bootstrap.iosNativeProvider == nil {
		t.Fatal("iOS native provider not registered")
	}
	state := bootstrap.iosNativeProvider.Capability().(builtin.IOSNativeProviderCapability)
	if state.Healthy || state.BridgeReady {
		t.Fatal("unattached native host claimed ready")
	}
	relay := newNativeBridgeRelay()
	tryRegisterIOSBridge(relay, bootstrap)
	bridge, found := relay.Handler().GetBridge("ios")
	if !found || bridge != bootstrap.IOSNativeBridge().(*nativebridge.IOSBridge) {
		t.Fatal("local relay did not register original iOS host bridge")
	}
}

func TestNonIOSRuntimeDoesNotRegisterIOSNativeProvider(t *testing.T) {
	descriptor := platform.RuntimeDescriptor{Host: platform.HostPlatformLinux, Kind: platform.RuntimeKindNativeProcess, Guest: platform.GuestPlatformLinux}
	host, err := runtimehost.NewRuntimeHost(runtimehost.HostBuildContext{Descriptor: descriptor})
	if err != nil {
		t.Fatal(err)
	}
	bootstrap := &runtimeBootstrap{host: host, providerRegistry: runtimeorchestrator.NewProviderRegistry()}
	if bootstrap.buildIOSNativeBridge() != nil {
		t.Fatal("foreign host received iOS bridge")
	}
	if err := bootstrap.registerProviderFactoriesIOS(); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.buildPlatformProvidersIOS(); err != nil || bootstrap.iosNativeProvider != nil {
		t.Fatal("foreign host received iOS provider")
	}
	relay := newNativeBridgeRelay()
	tryRegisterIOSBridge(relay, bootstrap)
	if _, found := relay.Handler().GetBridge("ios"); found {
		t.Fatal("foreign runtime accepted iOS native relay")
	}
}

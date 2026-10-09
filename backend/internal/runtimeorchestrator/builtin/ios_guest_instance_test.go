package builtin

import (
	"context"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
	"testing"
)

type iosGuestIdleTransport struct{}

func (iosGuestIdleTransport) Send([]byte) error { return nil }

func TestIOSGuestNativeProviderTracksActualHostGeneration(t *testing.T) {
	host, err := runtimehost.NewRuntimeHost(runtimehost.HostBuildContext{Descriptor: platform.RuntimeDescriptor{Host: platform.HostPlatformIOS, Guest: platform.GuestPlatformLinux, Kind: platform.RuntimeKindEmulated}})
	if err != nil {
		t.Fatal(err)
	}
	bridge := nativebridge.NewIOSBridge()
	factory := NewIOSNativeProviderFactory(IOSNativeProviderConfig{Bridge: bridge})
	instance, err := factory.Build(runtimeorchestrator.ProviderBuildContext{Host: host})
	if err != nil {
		t.Fatal(err)
	}
	provider := instance.(*iosNativeProviderInstance)
	if err := provider.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := provider.Capability().(IOSNativeProviderCapability)
	if before.BridgeReady || before.Healthy {
		t.Fatal("unattached host claimed ready")
	}
	first := bridge.AttachRelaySession(iosGuestIdleTransport{})
	after := provider.Capability().(IOSNativeProviderCapability)
	if !after.BridgeReady || !after.Healthy || uint64(after.Generation) != first {
		t.Fatal("actual host generation was not exposed")
	}
	second := bridge.AttachRelaySession(iosGuestIdleTransport{})
	bridge.DetachRelaySession(first)
	if uint64(provider.Capability().(IOSNativeProviderCapability).Generation) != second || provider.Health(context.Background()) != capability.HealthReady {
		t.Fatal("old host detach removed current native readiness")
	}
	provider.Stop(context.Background())
	if provider.Capability().(IOSNativeProviderCapability).BridgeReady || provider.Health(context.Background()) == capability.HealthReady {
		t.Fatal("stopped native provider remained ready")
	}
	response := provider.Execute(context.Background(), capability.IOSBridgeRequest{ProtocolVersion: 1, RequestID: "stopped-request", Operation: "calendar.list"})
	if response.Status != "error" || response.Error == nil {
		t.Fatal("stopped provider executed")
	}
	bridge.DetachRelaySession(second)
}

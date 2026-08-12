//go:build ios
// +build ios

package builtin

import (
	"context"
	"testing"

	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

func newTestIOSHost() runtimehost.RuntimeHost {
	paths := &util.RuntimePaths{}
	host, _ := runtimehost.NewRuntimeHost(runtimehost.HostBuildContext{
		Descriptor: platform.RuntimeDescriptor{
			Host: platform.HostPlatformIOS,
		},
		Paths: *paths,
	})
	if host == nil {
		panic("failed to create test host")
	}
	return host
}

func TestIOSNativeProviderInstance_Descriptor(t *testing.T) {
	bridge := &mockNativeBridge{}
	host := newTestIOSHost()

	instance := newIOSNativeProviderInstance(bridge, host)

	desc := instance.Descriptor()
	if desc.ID != ComponentIDIOSNative {
		t.Fatalf("expected component id provider.ios-native, got %s", desc.ID)
	}

	found := false
	for _, cap := range desc.Capabilities {
		if cap == "platform/ios/native" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected platform/ios/native capability")
	}
}

func TestIOSNativeProviderInstance_Start(t *testing.T) {
	bridge := &mockNativeBridge{
		healthFunc: func(ctx context.Context) nativebridge.Health {
			return nativebridge.HealthReady
		},
	}
	host := newTestIOSHost()

	instance := newIOSNativeProviderInstance(bridge, host)

	err := instance.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIOSNativeProviderInstance_Ready_NilBridge(t *testing.T) {
	host := newTestIOSHost()

	instance := newIOSNativeProviderInstance(nil, host)

	err := instance.Ready(context.Background())
	if err == nil {
		t.Fatal("expected error for nil bridge")
	}
}

func TestIOSNativeProviderInstance_Ready_Healthy(t *testing.T) {
	bridge := &mockNativeBridge{
		healthFunc: func(ctx context.Context) nativebridge.Health {
			return nativebridge.HealthReady
		},
	}
	host := newTestIOSHost()

	instance := newIOSNativeProviderInstance(bridge, host)

	err := instance.Ready(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIOSNativeProviderInstance_Stop(t *testing.T) {
	bridge := &mockNativeBridge{}
	host := newTestIOSHost()

	instance := newIOSNativeProviderInstance(bridge, host)

	err := instance.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIOSNativeProviderInstance_Capability(t *testing.T) {
	bridge := &mockNativeBridge{}
	host := newTestIOSHost()

	instance := newIOSNativeProviderInstance(bridge, host)

	cap := instance.Capability()
	capStruct, ok := cap.(IOSNativeProviderCapability)
	if !ok {
		t.Fatal("expected IOSNativeProviderCapability type")
	}

	if capStruct.ProviderID != "ios-native" {
		t.Fatalf("expected provider id ios-native, got %s", capStruct.ProviderID)
	}

	if capStruct.BridgeReady != true {
		t.Fatal("expected bridge ready to be true")
	}
}

func TestIOSNativeProviderInstance_Slot(t *testing.T) {
	bridge := &mockNativeBridge{}
	host := newTestIOSHost()

	instance := newIOSNativeProviderInstance(bridge, host)

	if instance.Slot() != runtimeorchestrator.ProviderSlotIOSNative {
		t.Fatalf("expected slot ios-native, got %s", instance.Slot())
	}
}

func TestIOSNativeProviderInstance_ProviderID(t *testing.T) {
	bridge := &mockNativeBridge{}
	host := newTestIOSHost()

	instance := newIOSNativeProviderInstance(bridge, host)

	if instance.ProviderID() != "ios-native" {
		t.Fatalf("expected provider id ios-native, got %s", instance.ProviderID())
	}
}

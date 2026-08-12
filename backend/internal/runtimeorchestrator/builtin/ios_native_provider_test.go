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

type mockNativeBridge struct {
	executeFunc func(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error)
	healthFunc  func(ctx context.Context) nativebridge.Health
}

func (m *mockNativeBridge) Execute(ctx context.Context, req nativebridge.Request) (nativebridge.Response, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, req)
	}
	return nativebridge.Response{
		ProtocolVersion: req.ProtocolVersion,
		RequestID:       req.RequestID,
		Status:          "success",
	}, nil
}

func (m *mockNativeBridge) Health(ctx context.Context) nativebridge.Health {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nativebridge.HealthReady
}

func newTestHost() runtimehost.RuntimeHost {
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

func TestIOSNativeProviderFactory_ProviderID(t *testing.T) {
	factory := NewIOSNativeProviderFactory(IOSNativeProviderConfig{})
	if factory.ProviderID() != "ios-native" {
		t.Fatalf("expected provider id ios-native, got %s", factory.ProviderID())
	}
}

func TestIOSNativeProviderFactory_Slot(t *testing.T) {
	factory := NewIOSNativeProviderFactory(IOSNativeProviderConfig{})
	if factory.Slot() != runtimeorchestrator.ProviderSlotIOSNative {
		t.Fatalf("expected slot ios-native, got %s", factory.Slot())
	}
}

func TestIOSNativeProviderFactory_Build_NilHost(t *testing.T) {
	factory := NewIOSNativeProviderFactory(IOSNativeProviderConfig{
		Bridge: &mockNativeBridge{},
	})

	_, err := factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: nil,
	})

	if err == nil {
		t.Fatal("expected error for nil host")
	}
}

func TestIOSNativeProviderFactory_Build_UnsupportedHost(t *testing.T) {
	factory := NewIOSNativeProviderFactory(IOSNativeProviderConfig{
		Bridge: &mockNativeBridge{},
	})

	descriptor := platform.RuntimeDescriptor{
		Host: platform.HostPlatformWindows,
	}
	paths := &util.RuntimePaths{}
	host, err := runtimehost.NewRuntimeHost(runtimehost.HostBuildContext{
		Descriptor: descriptor,
		Paths:      *paths,
	})
	if err != nil {
		t.Fatalf("failed to create host: %v", err)
	}

	_, err = factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: host,
	})

	if err == nil {
		t.Fatal("expected error for unsupported host platform")
	}
}

func TestIOSNativeProviderFactory_Build_NilBridge(t *testing.T) {
	factory := NewIOSNativeProviderFactory(IOSNativeProviderConfig{
		Bridge: nil,
	})

	host := newTestHost()

	_, err := factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: host,
	})

	if err == nil {
		t.Fatal("expected error for nil bridge")
	}
}

func TestIOSNativeProviderFactory_Build_Success(t *testing.T) {
	factory := NewIOSNativeProviderFactory(IOSNativeProviderConfig{
		Bridge: &mockNativeBridge{},
	})

	host := newTestHost()

	instance, err := factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: host,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if instance.ProviderID() != "ios-native" {
		t.Fatalf("expected provider id ios-native, got %s", instance.ProviderID())
	}

	if instance.Slot() != runtimeorchestrator.ProviderSlotIOSNative {
		t.Fatalf("expected slot ios-native, got %s", instance.Slot())
	}
}

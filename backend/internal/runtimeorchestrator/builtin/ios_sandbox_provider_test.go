//go:build ios
// +build ios

package builtin

import (
	"context"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
)

type fakeBackend struct {
	sandbox.SandboxBackend
	startCalled int
	stopCalled  int
}

func (b *fakeBackend) Availability(_ context.Context) sandbox.BackendAvailability {
	return sandbox.BackendAvailable
}

func (b *fakeBackend) Start(_ context.Context, _ sandbox.SandboxConfig) error {
	b.startCalled++
	return nil
}

func (b *fakeBackend) Stop(_ context.Context) error {
	b.stopCalled++
	return nil
}

func (b *fakeBackend) Execute(_ context.Context, _ sandbox.SandboxExecuteRequest) (sandbox.SandboxExecuteResult, error) {
	return sandbox.SandboxExecuteResult{}, fmt.Errorf("iSH native execution not available in this build")
}

func (b *fakeBackend) Cancel(_ context.Context, _ string) error {
	return fmt.Errorf("iSH native execution not available in this build")
}

func (b *fakeBackend) Health(_ context.Context) sandbox.SandboxHealth {
	return sandbox.SandboxHealth{
		Healthy: false,
		Message: "fake backend for test",
	}
}

type fakeHost struct {
	runtimehost.RuntimeHost
	descriptor platform.RuntimeDescriptor
}

func (h *fakeHost) Descriptor() platform.RuntimeDescriptor {
	return h.descriptor
}

func (h *fakeHost) RuntimeInstanceID() string {
	return "test-runtime-id"
}

func TestIOSSandboxProviderFactory_Build_NonIOSHost(t *testing.T) {
	factory := NewIOSSandboxProviderFactory(IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	})

	_, err := factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: &fakeHost{
			descriptor: platform.RuntimeDescriptor{
				Host: platform.HostPlatformWindows,
			},
		},
	})

	if err == nil {
		t.Fatal("expected error for non-iOS host, got nil")
	}

	sErr, ok := err.(*SandboxError)
	if !ok {
		t.Fatalf("expected SandboxError, got %T: %v", err, err)
	}

	if sErr.Code != SandboxErrUnsupportedHost {
		t.Errorf("expected code %s, got %s", SandboxErrUnsupportedHost, sErr.Code)
	}
}

func TestIOSSandboxProviderFactory_Build_HostNil(t *testing.T) {
	factory := NewIOSSandboxProviderFactory(IOSSandboxProviderConfig{
		Enabled: true,
	})

	_, err := factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: nil,
	})

	if err == nil {
		t.Fatal("expected error for nil host, got nil")
	}

	sErr, ok := err.(*SandboxError)
	if !ok {
		t.Fatalf("expected SandboxError, got %T: %v", err, err)
	}

	if sErr.Code != SandboxErrHostRequired {
		t.Errorf("expected code %s, got %s", SandboxErrHostRequired, sErr.Code)
	}
}

func TestIOSSandboxProviderFactory_Build_MissingRootfsURI(t *testing.T) {
	factory := NewIOSSandboxProviderFactory(IOSSandboxProviderConfig{
		Enabled: true,
	})

	backendCalled := false
	factory.newBackend = func() (sandbox.SandboxBackend, error) {
		backendCalled = true
		return &fakeBackend{}, nil
	}

	_, err := factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: &fakeHost{
			descriptor: platform.RuntimeDescriptor{
				Host: platform.HostPlatformIOS,
			},
		},
	})

	if err == nil {
		t.Fatal("expected error for missing rootfsUri, got nil")
	}

	if backendCalled {
		t.Error("backend should not have been created when rootfsUri missing")
	}

	sErr, ok := err.(*SandboxError)
	if !ok {
		t.Fatalf("expected SandboxError, got %T: %v", err, err)
	}

	if sErr.Code != SandboxErrRootfsNotConfigured {
		t.Errorf("expected code %s, got %s", SandboxErrRootfsNotConfigured, sErr.Code)
	}
}

func TestIOSSandboxProviderFactory_Build_Disabled(t *testing.T) {
	factory := NewIOSSandboxProviderFactory(IOSSandboxProviderConfig{
		Enabled: false,
	})

	instance, err := factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: &fakeHost{
			descriptor: platform.RuntimeDescriptor{
				Host: platform.HostPlatformIOS,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error for disabled provider: %v", err)
	}

	if instance == nil {
		t.Fatal("expected instance, got nil")
	}

	if instance.Descriptor().Enabled {
		t.Error("expected descriptor.Enabled=false for disabled provider")
	}
}

func TestIOSSandboxProviderFactory_Build_RootfsPresent(t *testing.T) {
	factory := NewIOSSandboxProviderFactory(IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	})

	backend := &fakeBackend{}
	factory.newBackend = func() (sandbox.SandboxBackend, error) {
		return backend, nil
	}

	instance, err := factory.Build(runtimeorchestrator.ProviderBuildContext{
		Host: &fakeHost{
			descriptor: platform.RuntimeDescriptor{
				Host: platform.HostPlatformIOS,
			},
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if instance == nil {
		t.Fatal("instance is nil")
	}

	if instance.ProviderID() != sandbox.ProviderIDIOSSandbox {
		t.Errorf("providerID mismatch: %s", instance.ProviderID())
	}

	if instance.Descriptor().Enabled != true {
		t.Error("expected descriptor.Enabled=true")
	}
}

func TestIOSSandboxProviderFactory_DescriptorPresent(t *testing.T) {
	factory := NewIOSSandboxProviderFactory(IOSSandboxProviderConfig{})

	if factory.ProviderID() != sandbox.ProviderIDIOSSandbox {
		t.Errorf("providerID mismatch: %s", factory.ProviderID())
	}

	if factory.Slot() != runtimeorchestrator.ProviderSlotIOSSandbox {
		t.Errorf("slot mismatch: %s", factory.Slot())
	}

	reqs := factory.Requirements()
	if len(reqs) != 2 {
		t.Errorf("expected 2 capability requirements, got %d", len(reqs))
	}
}

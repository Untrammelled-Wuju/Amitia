//go:build ios
// +build ios

package builtin

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
)

type controllableBackend struct {
	startCalled int
	stopCalled  int
	startFn     func() error
	stopFn      func() error
	healthFn    func() sandbox.SandboxHealth
	availFn     func() sandbox.BackendAvailability
	mu          sync.Mutex
}

func newControllableBackend() *controllableBackend {
	return &controllableBackend{
		healthFn: func() sandbox.SandboxHealth {
			return sandbox.SandboxHealth{
				Healthy: false,
				Message: "test backend not running",
			}
		},
		availFn: func() sandbox.BackendAvailability {
			return sandbox.BackendUnavailable
		},
		startFn: func() error { return nil },
		stopFn:  func() error { return nil },
	}
}

func (b *controllableBackend) Availability(_ context.Context) sandbox.BackendAvailability {
	b.mu.Lock()
	fn := b.availFn
	b.mu.Unlock()
	return fn()
}

func (b *controllableBackend) Start(_ context.Context, _ sandbox.SandboxConfig) error {
	b.mu.Lock()
	b.startCalled++
	fn := b.startFn
	b.mu.Unlock()
	return fn()
}

func (b *controllableBackend) Stop(_ context.Context, _ sandbox.SandboxStopReason) error {
	b.mu.Lock()
	b.stopCalled++
	fn := b.stopFn
	b.mu.Unlock()
	return fn()
}

func (b *controllableBackend) Execute(_ context.Context, _ sandbox.SandboxExecuteRequest) (sandbox.SandboxExecuteResult, error) {
	return sandbox.SandboxExecuteResult{}, fmt.Errorf("iSH native execution not available in this build")
}

func (b *controllableBackend) Cancel(_ context.Context, _ string) error {
	return fmt.Errorf("iSH native execution not available in this build")
}

func (b *controllableBackend) Health(_ context.Context) sandbox.SandboxHealth {
	b.mu.Lock()
	fn := b.healthFn
	b.mu.Unlock()
	return fn()
}

func (b *controllableBackend) Quiesce(_ context.Context) error {
	return fmt.Errorf("iSH native execution not available in this build")
}

func (b *controllableBackend) Resume(_ context.Context) error {
	return fmt.Errorf("iSH native execution not available in this build")
}

func (b *controllableBackend) Restart(_ context.Context, _ sandbox.SandboxRestartReason) error {
	return fmt.Errorf("iSH native execution not available in this build")
}

func (b *controllableBackend) Recover(_ context.Context) error {
	return fmt.Errorf("iSH native execution not available in this build")
}

func (b *controllableBackend) LifecycleState(_ context.Context) sandbox.SandboxLifecycleState {
	return sandbox.SandboxStateIdle
}

func (b *controllableBackend) RecoverySnapshot(_ context.Context) sandbox.SandboxRecoverySnapshot {
	return sandbox.SandboxRecoverySnapshot{}
}

func (b *controllableBackend) setStartError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.startFn = func() error { return err }
}

func (b *controllableBackend) setHealthy(healthy bool, msg string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.healthFn = func() sandbox.SandboxHealth {
		return sandbox.SandboxHealth{
			Healthy: healthy,
			Message: msg,
		}
	}
}

type testHost struct {
	runtimehost.RuntimeHost
	descriptor   platform.RuntimeDescriptor
	instanceID   string
}

func (h *testHost) Descriptor() platform.RuntimeDescriptor {
	return h.descriptor
}

func (h *testHost) RuntimeInstanceID() string {
	return h.instanceID
}

func newTestHost() *testHost {
	return &testHost{
		descriptor: platform.RuntimeDescriptor{
			Host: platform.HostPlatformIOS,
		},
		instanceID: "test-runtime-001",
	}
}

func TestIOSSandboxInstance_Disabled(t *testing.T) {
	backend := newControllableBackend()
	host := newTestHost()
	config := IOSSandboxProviderConfig{
		Enabled: false,
	}

	instance := newIOSSandboxProviderInstance(backend, host, config)

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start on disabled instance should return nil, got: %v", err)
	}

	if backend.startCalled != 0 {
		t.Error("disabled provider should not call backend.Start")
	}

	if err := instance.Stop(context.Background()); err != nil {
		t.Fatalf("Stop on disabled instance should return nil, got: %v", err)
	}

	if backend.stopCalled != 0 {
		t.Error("disabled provider should not call backend.Stop")
	}

	cap := instance.Capability()
	iosCap, ok := cap.(IOSSandboxProviderCapability)
	if !ok {
		t.Fatalf("expected IOSSandboxProviderCapability, got %T", cap)
	}

	if iosCap.Availability != "unavailable" {
		t.Errorf("expected availability=unavailable, got %s", iosCap.Availability)
	}

	if iosCap.Healthy {
		t.Error("disabled provider should report healthy=false")
	}
}

func TestIOSSandboxInstance_StartFailClosed(t *testing.T) {
	backend := newControllableBackend()
	backend.setStartError(fmt.Errorf("iSH container init failed"))
	host := newTestHost()
	config := IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	}

	instance := newIOSSandboxProviderInstance(backend, host, config)

	err := instance.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when backend.Start fails")
	}

	if backend.startCalled != 1 {
		t.Errorf("expected backend startCalled=1, got %d", backend.startCalled)
	}

	cap := instance.Capability()
	iosCap, ok := cap.(IOSSandboxProviderCapability)
	if !ok {
		t.Fatalf("expected IOSSandboxProviderCapability, got %T", cap)
	}

	if iosCap.Healthy {
		t.Error("failed start should result in healthy=false")
	}

	if err := instance.Ready(context.Background()); err == nil {
		t.Fatal("Ready should fail after failed Start")
	}
}

func TestIOSSandboxInstance_IdempotentStartStop(t *testing.T) {
	backend := newControllableBackend()
	host := newTestHost()
	config := IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	}

	instance := newIOSSandboxProviderInstance(backend, host, config)

	ctx := context.Background()

	if err := instance.Start(ctx); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	if err := instance.Start(ctx); err != nil {
		t.Fatalf("second Start failed: %v", err)
	}

	if backend.startCalled != 1 {
		t.Errorf("expected backend startCalled=1, got %d", backend.startCalled)
	}

	if err := instance.Stop(ctx); err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}

	if err := instance.Stop(ctx); err != nil {
		t.Fatalf("second Stop failed: %v", err)
	}

	if backend.stopCalled != 1 {
		t.Errorf("expected backend stopCalled=1, got %d", backend.stopCalled)
	}
}

func TestIOSSandboxInstance_ReadyRealHealth(t *testing.T) {
	backend := newControllableBackend()
	backend.setHealthy(false, "iSH native execution not available")
	host := newTestHost()
	config := IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	}

	instance := newIOSSandboxProviderInstance(backend, host, config)

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := instance.Ready(context.Background()); err == nil {
		t.Error("Ready should fail when Health.Healthy=false")
	}
}

func TestIOSSandboxInstance_ReadySucceedsAfterHealthy(t *testing.T) {
	backend := newControllableBackend()
	backend.setHealthy(true, "running")
	host := newTestHost()
	config := IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	}

	instance := newIOSSandboxProviderInstance(backend, host, config)

	if err := instance.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := instance.Ready(context.Background()); err != nil {
		t.Errorf("Ready should succeed when healthy=true, got: %v", err)
	}
}

func TestIOSSandboxInstance_CapabilityStableDTO(t *testing.T) {
	backend := newControllableBackend()
	backend.setHealthy(true, "running")
	host := newTestHost()
	config := IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	}

	instance := newIOSSandboxProviderInstance(backend, host, config)

	raw := instance.Capability()
	iosCap, ok := raw.(IOSSandboxProviderCapability)
	if !ok {
		t.Fatalf("expected IOSSandboxProviderCapability, got %T", raw)
	}

	if iosCap.ProviderID != sandbox.ProviderIDIOSSandbox {
		t.Errorf("ProviderID mismatch: %s", iosCap.ProviderID)
	}

	if iosCap.Slot != string(runtimeorchestrator.ProviderSlotIOSSandbox) {
		t.Errorf("Slot mismatch: %s", iosCap.Slot)
	}

	if iosCap.RuntimeID != host.RuntimeInstanceID() {
		t.Errorf("RuntimeID mismatch: %s", iosCap.RuntimeID)
	}

	if iosCap.HostPlatform != string(platform.HostPlatformIOS) {
		t.Errorf("HostPlatform mismatch: %s", iosCap.HostPlatform)
	}

	if iosCap.RootfsInstalled {
		t.Error("B16 should not fake rootfsInstalled=true")
	}

	if iosCap.ISHInitialized {
		t.Error("B16 should not fake ishInitialized=true")
	}
}

func TestIOSSandboxInstance_RuntimeIDPropagation(t *testing.T) {
	backend := newControllableBackend()
	host := newTestHost()
	host.instanceID = "specific-runtime-uuid-123"
	config := IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	}

	instance := newIOSSandboxProviderInstance(backend, host, config)

	raw := instance.Capability()
	cap := raw.(IOSSandboxProviderCapability)

	if cap.RuntimeID != "specific-runtime-uuid-123" {
		t.Errorf("RuntimeID mismatch: expected specific-runtime-uuid-123, got %s", cap.RuntimeID)
	}
}

func TestIOSSandboxInstance_UnregisteredHost(t *testing.T) {
	backend := newControllableBackend()
	host := newTestHost()
	config := IOSSandboxProviderConfig{
		Enabled:   true,
		RootfsURI: "amitia://runtime/ios/rootfs",
	}

	instance := newIOSSandboxProviderInstance(backend, host, config)

	raw := instance.Capability()
	cap := raw.(IOSSandboxProviderCapability)

	if cap.ProviderID != sandbox.ProviderIDIOSSandbox {
		t.Errorf("ProviderID should always be present, got: %s", cap.ProviderID)
	}

	if cap.Slot != string(runtimeorchestrator.ProviderSlotIOSSandbox) {
		t.Errorf("Slot should always be present, got: %s", cap.Slot)
	}
}

package builtin

import (
	"context"
	"fmt"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

// Verify IOSSandboxProviderFactory satisfies ProviderFactory interface
var _ runtimeorchestrator.ProviderFactory = (*IOSSandboxProviderFactory)(nil)

type fakeIOSSandboxBackend struct {
	config sandbox.SandboxConfig
	state  sandbox.BackendAvailability
}

func newFakeIOSSandboxBackend() *fakeIOSSandboxBackend {
	return &fakeIOSSandboxBackend{state: sandbox.BackendUnavailable}
}

func (b *fakeIOSSandboxBackend) Availability(_ context.Context) sandbox.BackendAvailability {
	return b.state
}

func (b *fakeIOSSandboxBackend) Start(_ context.Context, config sandbox.SandboxConfig) error {
	b.config = config
	b.state = sandbox.BackendStarting
	return nil
}

func (b *fakeIOSSandboxBackend) Stop(_ context.Context) error {
	b.state = sandbox.BackendUnavailable
	return nil
}

func (b *fakeIOSSandboxBackend) Execute(_ context.Context, cmd sandbox.SandboxCommand) (sandbox.SandboxResult, error) {
	return sandbox.SandboxResult{
		Error: fmt.Sprintf("fake backend: not connected: %v", cmd.Command),
	}, fmt.Errorf("fake backend: not connected")
}

func (b *fakeIOSSandboxBackend) Health(_ context.Context) sandbox.SandboxHealth {
	return sandbox.SandboxHealth{
		Healthy:         b.state == sandbox.BackendRunning,
		Message:         b.state.String(),
		ISHInitialized:  b.state == sandbox.BackendRunning,
		RootfsInstalled: false,
	}
}

type fakeRuntimeHost struct {
	descriptor platform.RuntimeDescriptor
	caps       *runtimehost.HostCapabilities
	instanceID string
	paths      util.RuntimePaths
	processes  *noopProcessSupervisor
}

func newIOSTestHost() *fakeRuntimeHost {
	f := &fakeRuntimeHost{
		descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformIOS,
			Kind:  platform.RuntimeKindEmbedded,
			Guest: platform.GuestPlatformIOS,
		},
		instanceID: "ios-test-instance-001",
		paths:      util.RuntimePaths{},
	}
	f.caps = runtimehost.NewTestCapabilitiesForTest(map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
		runtimehost.CapRuntimeSandboxedExec: runtimehost.SupportSupported,
		runtimehost.CapRuntimeNativeOffload: runtimehost.SupportLimited,
	})
	f.processes = &noopProcessSupervisor{}
	return f
}

func newLinuxTestHost() *fakeRuntimeHost {
	f := &fakeRuntimeHost{
		descriptor: platform.RuntimeDescriptor{
			Host:  platform.HostPlatformLinux,
			Kind:  platform.RuntimeKindNativeProcess,
			Guest: platform.GuestPlatformLinux,
		},
		instanceID: "linux-test-instance-001",
		paths:      util.RuntimePaths{},
	}
	f.caps = runtimehost.NewTestCapabilitiesForTest(map[runtimehost.HostCapabilityID]runtimehost.CapabilitySupport{
		runtimehost.CapProcessSpawn:         runtimehost.SupportSupported,
		runtimehost.CapRuntimeSandboxedExec: runtimehost.SupportUnsupported,
		runtimehost.CapRuntimeNativeOffload: runtimehost.SupportUnsupported,
	})
	f.processes = &noopProcessSupervisor{}
	return f
}

func (f *fakeRuntimeHost) Descriptor() platform.RuntimeDescriptor      { return f.descriptor }
func (f *fakeRuntimeHost) Capabilities() *runtimehost.HostCapabilities { return f.caps }
func (f *fakeRuntimeHost) Paths() util.RuntimePaths                    { return f.paths }
func (f *fakeRuntimeHost) Processes() runtimehost.ProcessSupervisor    { return f.processes }
func (f *fakeRuntimeHost) RuntimeInstanceID() string                   { return f.instanceID }

type noopProcessSupervisor struct{}

func (s *noopProcessSupervisor) Register(_ runtimehost.ProcessSpec) error {
	return runtimehost.ErrHostProcessUnsupported
}
func (s *noopProcessSupervisor) Unregister(_ runtimehost.ProcessID) error { return nil }
func (s *noopProcessSupervisor) Start(_ context.Context, _ runtimehost.ProcessID) error {
	return runtimehost.ErrHostProcessUnsupported
}
func (s *noopProcessSupervisor) WaitReady(_ context.Context, _ runtimehost.ProcessID) error {
	return runtimehost.ErrHostProcessUnsupported
}
func (s *noopProcessSupervisor) Restart(_ context.Context, _ runtimehost.ProcessID) error {
	return runtimehost.ErrHostProcessUnsupported
}
func (s *noopProcessSupervisor) Stop(_ context.Context, _ runtimehost.ProcessID) error { return nil }
func (s *noopProcessSupervisor) StopAll(_ context.Context) error                       { return nil }
func (s *noopProcessSupervisor) Snapshot(_ runtimehost.ProcessID) (runtimehost.ProcessSnapshot, bool) {
	return runtimehost.ProcessSnapshot{}, false
}
func (s *noopProcessSupervisor) List() []runtimehost.ProcessSnapshot { return nil }
func (s *noopProcessSupervisor) Subscribe(_ func(runtimehost.ProcessEvent)) func() {
	return func() {}
}

func TestIOSSandboxProviderFactorySlot(t *testing.T) {
	factory := NewIOSSandboxProviderFactory(IOSSandboxProviderConfig{})
	if factory.Slot() != runtimeorchestrator.ProviderSlotIOSSandbox {
		t.Fatalf("slot=%s, want ios-sandbox", factory.Slot())
	}
}

func TestIOSSandboxProviderFactoryProviderID(t *testing.T) {
	factory := NewIOSSandboxProviderFactory(IOSSandboxProviderConfig{})
	if factory.ProviderID() != sandbox.ProviderIDIOSSandbox {
		t.Fatalf("providerID=%s, want ios.ish-sandbox", factory.ProviderID())
	}
}

func TestIOSSandboxProviderFactoryRejectsNonIOSHost(t *testing.T) {
	factory := &IOSSandboxProviderFactory{
		config:     IOSSandboxProviderConfig{Enabled: true},
		newBackend: func() (sandbox.SandboxBackend, error) { return newFakeIOSSandboxBackend(), nil },
	}
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: newLinuxTestHost(),
	}
	_, err := factory.Build(bc)
	if err == nil {
		t.Fatal("expected error for non-iOS host")
	}
}

func TestIOSSandboxProviderFactoryRejectsNilHost(t *testing.T) {
	factory := &IOSSandboxProviderFactory{
		config:     IOSSandboxProviderConfig{Enabled: true},
		newBackend: func() (sandbox.SandboxBackend, error) { return newFakeIOSSandboxBackend(), nil },
	}
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: nil,
	}
	_, err := factory.Build(bc)
	if err == nil {
		t.Fatal("expected error for nil host")
	}
}

func TestIOSSandboxProviderFactoryBackendReturnsNil(t *testing.T) {
	factory := &IOSSandboxProviderFactory{
		config:     IOSSandboxProviderConfig{Enabled: true},
		newBackend: func() (sandbox.SandboxBackend, error) { return nil, nil },
	}
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: newIOSTestHost(),
	}
	_, err := factory.Build(bc)
	if err == nil {
		t.Fatal("expected error when backend factory returns nil")
	}
}

func TestIOSSandboxProviderFactoryBackendReturnsError(t *testing.T) {
	expectedErr := fmt.Errorf("backend creation failed")
	factory := &IOSSandboxProviderFactory{
		config:     IOSSandboxProviderConfig{Enabled: true},
		newBackend: func() (sandbox.SandboxBackend, error) { return nil, expectedErr },
	}
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: newIOSTestHost(),
	}
	_, err := factory.Build(bc)
	if err == nil {
		t.Fatal("expected error when backend factory returns error")
	}
}

func TestIOSSandboxProviderRuntimeInstanceIDPropagation(t *testing.T) {
	fakeBackend := newFakeIOSSandboxBackend()
	factory := &IOSSandboxProviderFactory{
		config:     IOSSandboxProviderConfig{Enabled: true},
		newBackend: func() (sandbox.SandboxBackend, error) { return fakeBackend, nil },
	}
	host := newIOSTestHost()
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: host,
	}
	inst, err := factory.Build(bc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := inst.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if fakeBackend.config.RuntimeID != host.RuntimeInstanceID() {
		t.Fatalf("runtimeID=%s, want %s", fakeBackend.config.RuntimeID, host.RuntimeInstanceID())
	}
}

func TestIOSSandboxProviderConfigPropagation(t *testing.T) {
	fakeBackend := newFakeIOSSandboxBackend()
	factory := &IOSSandboxProviderFactory{
		config: IOSSandboxProviderConfig{
			Enabled:      true,
			WorkspaceURI: "amitia://workspace/ios",
			RootfsURI:    "amitia://runtime/ios/rootfs",
			Environment: map[string]string{
				"HOME": "/root",
			},
		},
		newBackend: func() (sandbox.SandboxBackend, error) { return fakeBackend, nil },
	}
	host := newIOSTestHost()
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: host,
	}
	inst, err := factory.Build(bc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := inst.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if fakeBackend.config.WorkspaceURI != "amitia://workspace/ios" {
		t.Fatalf("workspaceURI=%s", fakeBackend.config.WorkspaceURI)
	}
	if fakeBackend.config.RootfsURI != "amitia://runtime/ios/rootfs" {
		t.Fatalf("rootfsURI=%s", fakeBackend.config.RootfsURI)
	}
	if fakeBackend.config.Environment["HOME"] != "/root" {
		t.Fatalf("HOME=%s", fakeBackend.config.Environment["HOME"])
	}
}

func TestIOSSandboxProviderEnvironmentDefensiveCopy(t *testing.T) {
	fakeBackend := newFakeIOSSandboxBackend()
	factory := &IOSSandboxProviderFactory{
		config: IOSSandboxProviderConfig{
			Enabled: true,
			Environment: map[string]string{
				"HOME": "/root",
			},
		},
		newBackend: func() (sandbox.SandboxBackend, error) { return fakeBackend, nil },
	}
	host := newIOSTestHost()
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: host,
	}
	inst, err := factory.Build(bc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := inst.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	fakeBackend.config.Environment["HOME"] = "changed"
	if factory.config.Environment["HOME"] != "/root" {
		t.Fatalf("config Environment was mutated: HOME=%s", factory.config.Environment["HOME"])
	}
}

func TestIOSSandboxProviderDescriptorNotRequired(t *testing.T) {
	fakeBackend := newFakeIOSSandboxBackend()
	factory := &IOSSandboxProviderFactory{
		config:     IOSSandboxProviderConfig{Enabled: true},
		newBackend: func() (sandbox.SandboxBackend, error) { return fakeBackend, nil },
	}
	host := newIOSTestHost()
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: host,
	}
	inst, err := factory.Build(bc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	desc := inst.Descriptor()
	if desc.Required {
		t.Fatal("iOS sandbox provider must not be required")
	}
}

func TestIOSSandboxProviderDisabledDoesNotStart(t *testing.T) {
	fakeBackend := newFakeIOSSandboxBackend()
	factory := &IOSSandboxProviderFactory{
		config:     IOSSandboxProviderConfig{Enabled: false},
		newBackend: func() (sandbox.SandboxBackend, error) { return fakeBackend, nil },
	}
	host := newIOSTestHost()
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: host,
	}
	inst, err := factory.Build(bc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := inst.Start(context.Background()); err != nil {
		t.Fatalf("Start with disabled config should return nil: %v", err)
	}
	if fakeBackend.state == sandbox.BackendStarting {
		t.Fatal("disabled provider should not start backend")
	}
}

func TestIOSSandboxProviderCapabilityIncludesHostInfo(t *testing.T) {
	fakeBackend := newFakeIOSSandboxBackend()
	factory := &IOSSandboxProviderFactory{
		config:     IOSSandboxProviderConfig{Enabled: true},
		newBackend: func() (sandbox.SandboxBackend, error) { return fakeBackend, nil },
	}
	host := newIOSTestHost()
	bc := runtimeorchestrator.ProviderBuildContext{
		Host: host,
	}
	inst, err := factory.Build(bc)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	cap := inst.Capability()
	m, ok := cap.(map[string]any)
	if !ok {
		t.Fatalf("capability is not map[string]any: %T", cap)
	}
	if m["runtimeId"] != host.RuntimeInstanceID() {
		t.Fatalf("runtimeId=%v", m["runtimeId"])
	}
	if m["hostPlatform"] != string(platform.HostPlatformIOS) {
		t.Fatalf("hostPlatform=%v", m["hostPlatform"])
	}
	if m["slot"] != string(runtimeorchestrator.ProviderSlotIOSSandbox) {
		t.Fatalf("slot=%v", m["slot"])
	}
}

var _ runtimehost.ProcessSupervisor = (*noopProcessSupervisor)(nil)

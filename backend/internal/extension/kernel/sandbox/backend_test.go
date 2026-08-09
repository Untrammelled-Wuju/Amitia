package sandbox

import (
	"context"
	"strings"
	"testing"
)

type fakeNativeBridge struct {
	availability BackendAvailability
	started      bool
	stopped      bool
	lastConfig   SandboxConfig
	lastCmd      SandboxCommand
	health       SandboxHealth
}

func newFakeNativeBridge() *fakeNativeBridge {
	return &fakeNativeBridge{
		availability: BackendUnavailable,
		health: SandboxHealth{
			Healthy: false,
			Message: "fake",
		},
	}
}

func (b *fakeNativeBridge) Availability(_ context.Context) BackendAvailability {
	return b.availability
}

func (b *fakeNativeBridge) Start(_ context.Context, cfg SandboxConfig) error {
	b.started = true
	b.lastConfig = cfg
	b.availability = BackendRunning
	return nil
}

func (b *fakeNativeBridge) Stop(_ context.Context) error {
	b.stopped = true
	b.availability = BackendUnavailable
	return nil
}

func (b *fakeNativeBridge) Execute(_ context.Context, cmd SandboxCommand) (SandboxResult, error) {
	b.lastCmd = cmd
	return SandboxResult{
		Stdout:   "fake-stdout",
		Stderr:   "",
		ExitCode: 0,
		Error:    "",
	}, nil
}

func (b *fakeNativeBridge) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b *fakeNativeBridge) Health(_ context.Context) SandboxHealth {
	return b.health
}

func TestISHBackendUsesNativeBridge(t *testing.T) {
	bridge := newFakeNativeBridge()
	backend := &ishBackend{bridge: bridge}

	if err := backend.Start(context.Background(), SandboxConfig{
		RuntimeID:    "test-runtime",
		RootfsURI:    "amitia://rootfs",
		WorkspaceURI: "amitia://workspace",
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !bridge.started {
		t.Fatal("expected native bridge Start to be called")
	}
	if bridge.lastConfig.RuntimeID != "test-runtime" {
		t.Fatalf("runtimeID=%s", bridge.lastConfig.RuntimeID)
	}
	if bridge.lastConfig.RootfsURI != "amitia://rootfs" {
		t.Fatalf("rootfsURI=%s", bridge.lastConfig.RootfsURI)
	}
}

func TestISHBackendDelegatesExecute(t *testing.T) {
	bridge := newFakeNativeBridge()
	bridge.availability = BackendRunning
	backend := &ishBackend{bridge: bridge}

	res, err := backend.Execute(context.Background(), SandboxCommand{
		Command: []string{"echo", "hello"},
		Stdin:   "input",
		Timeout: 30,
		WorkDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Stdout != "fake-stdout" {
		t.Fatalf("stdout=%s", res.Stdout)
	}
	if len(bridge.lastCmd.Command) != 2 || bridge.lastCmd.Command[0] != "echo" {
		t.Fatalf("command not delegated: %v", bridge.lastCmd.Command)
	}
	if bridge.lastCmd.WorkDir != "/tmp" {
		t.Fatalf("workdir not delegated: %s", bridge.lastCmd.WorkDir)
	}
}

func TestISHBackendDelegatesStop(t *testing.T) {
	bridge := newFakeNativeBridge()
	backend := &ishBackend{bridge: bridge}

	if err := backend.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !bridge.stopped {
		t.Fatal("expected native bridge Stop to be called")
	}
	if backend.Availability(context.Background()) != BackendUnavailable {
		t.Fatal("expected backend to report unavailable after stop")
	}
}

func TestISHBackendAvailabilityReflectsBridge(t *testing.T) {
	bridge := newFakeNativeBridge()
	backend := &ishBackend{bridge: bridge}

	if backend.Availability(context.Background()) != BackendUnavailable {
		t.Fatal("expected unavailable")
	}

	bridge.availability = BackendRunning
	if backend.Availability(context.Background()) != BackendRunning {
		t.Fatal("expected running")
	}
}

func TestISHBackendHealthDelegates(t *testing.T) {
	bridge := newFakeNativeBackendHealthy()
	backend := &ishBackend{bridge: bridge}

	h := backend.Health(context.Background())
	if !h.Healthy {
		t.Fatal("expected healthy")
	}
	if !h.ISHInitialized {
		t.Fatal("expected ishInitialized")
	}
}

func TestISHBackendHealthIncludesLifecycleFields(t *testing.T) {
	bridge := newFakeNativeBridge()
	backend := &ishBackend{bridge: bridge}

	h := backend.Health(context.Background())
	if h.LifecycleState != "" {
		t.Fatalf("expected legacy empty lifecycle_state on old fake bridge, got: %s", h.LifecycleState)
	}
	if h.Generation != 0 {
		t.Fatal("expected generation=0 on initial fake bridge")
	}
	if h.DesiredRunning {
		t.Fatal("expected desiredRunning=false initially")
	}
	if h.RestartRequired {
		t.Fatal("expected restartRequired=false initially")
	}
	if h.RecoveryPending {
		t.Fatal("expected recoveryPending=false initially")
	}
	if h.ActiveExecutionID != "" {
		t.Fatal("expected empty activeExecutionID")
	}
	if h.RunningRootfsVersion != "" {
		t.Fatal("expected empty runningRootfsVersion")
	}
	if h.RunningRootfsDigest != "" {
		t.Fatal("expected empty runningRootfsDigest")
	}
}

func newFakeNativeBackendHealthy() *fakeNativeBridge {
	bridge := newFakeNativeBridge()
	bridge.health = SandboxHealth{
		Healthy:        true,
		Message:        "running",
		ISHInitialized: true,
	}
	return bridge
}

func TestUnavailableBackendOnNonIOS(t *testing.T) {
	_ = context.Background()
	backend, err := NewIOSSandboxBackend()
	if err != nil {
		t.Fatalf("NewIOSSandboxBackend: %v", err)
	}

	if backend.Availability(context.Background()) != BackendUnavailable {
		t.Fatal("expected BackendUnavailable on non-iOS platform")
	}

	err = backend.Start(context.Background(), SandboxConfig{})
	if err == nil {
		t.Fatal("expected Start to fail on non-iOS")
	}

	_, err = backend.Execute(context.Background(), SandboxCommand{Command: []string{"echo"}})
	if err == nil {
		t.Fatal("expected Execute to fail on non-iOS")
	}
}

func TestFakeNativeBridgeImplementsInterface(t *testing.T) {
	var _ NativeBridge = (*fakeNativeBridge)(nil)
}

func TestISHBackendStateTransitions(t *testing.T) {
	bridge := newFakeNativeBridge()
	backend := &ishBackend{bridge: bridge}

	if backend.Availability(context.Background()) != BackendUnavailable {
		t.Fatal("expected initial state unavailable")
	}

	if err := backend.Start(context.Background(), SandboxConfig{RootfsURI: "amitia://test"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if backend.Availability(context.Background()) != BackendRunning {
		t.Fatal("expected running after start")
	}

	if err := backend.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if backend.Availability(context.Background()) != BackendUnavailable {
		t.Fatal("expected unavailable after stop")
	}
}

func TestNoFakeSuccessPath(t *testing.T) {
	backend, err := NewIOSSandboxBackend()
	if err != nil {
		t.Fatalf("NewIOSSandboxBackend: %v", err)
	}

	_, err = backend.Execute(context.Background(), SandboxCommand{Command: []string{"echo", "should-fail"}})
	if err == nil {
		t.Fatal("Execute must not succeed on non-iOS platform without native bridge")
	}

	if !strings.Contains(err.Error(), "not") && !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected descriptive error about platform, got: %v", err)
	}
}

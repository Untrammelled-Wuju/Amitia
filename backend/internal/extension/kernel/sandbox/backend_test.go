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
	lastReq      SandboxExecuteRequest
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

func (b *fakeNativeBridge) Execute(_ context.Context, req SandboxExecuteRequest) (SandboxExecuteResult, error) {
	b.lastReq = req
	return SandboxExecuteResult{
		ExecutionID: req.ExecutionID,
		ExitCode:    0,
		Stdout:      []byte("fake-stdout"),
		Stderr:      []byte{},
	}, nil
}

func (b *fakeNativeBridge) Cancel(_ context.Context, _ string) error {
	return nil
}

func (b *fakeNativeBridge) Health(_ context.Context) SandboxHealth {
	return b.health
}

func (b *fakeNativeBridge) RootfsStatus(_ context.Context) (RootfsStatus, error) {
	return RootfsStatus{}, nil
}

func (b *fakeNativeBridge) EnsureRootfs(_ context.Context, spec RootfsInstallSpec) (RootfsInstallResult, error) {
	return RootfsInstallResult{Version: spec.RootfsVersion}, nil
}

func (b *fakeNativeBridge) ActivateRootfs(_ context.Context, _ string) error {
	return nil
}

func (b *fakeNativeBridge) RemoveRootfs(_ context.Context, _ string) error {
	return nil
}

func TestISHBackendUsesNativeBridge(t *testing.T) {
	bridge := newFakeNativeBridge()
	backend := &ishBackend{bridge: bridge}

	if err := backend.Start(context.Background(), SandboxConfig{
		RuntimeID:    "test-runtime",
		RootfsURI:    "amitia://runtime/rootfs",
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
	if bridge.lastConfig.RootfsURI != "amitia://runtime/rootfs" {
		t.Fatalf("rootfsURI=%s", bridge.lastConfig.RootfsURI)
	}
}

func TestISHBackendDelegatesExecute(t *testing.T) {
	bridge := newFakeNativeBridge()
	bridge.availability = BackendRunning
	backend := &ishBackend{bridge: bridge}

	res, err := backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID:         "exec-001",
		Argv:                []string{"echo", "hello"},
		Stdin:               []byte("input"),
		WorkingDirectoryURI: "/tmp",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if string(res.Stdout) != "fake-stdout" {
		t.Fatalf("stdout=%s", res.Stdout)
	}
	if len(bridge.lastReq.Argv) != 2 || bridge.lastReq.Argv[0] != "echo" {
		t.Fatalf("command not delegated: %v", bridge.lastReq.Argv)
	}
	if bridge.lastReq.WorkingDirectoryURI != "/tmp" {
		t.Fatalf("workdir not delegated: %s", bridge.lastReq.WorkingDirectoryURI)
	}
	if res.ExecutionID != "exec-001" {
		t.Fatalf("executionID not propagated: %s", res.ExecutionID)
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

	_, err = backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID: "exec-test",
		Argv:        []string{"echo"},
	})
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

	if err := backend.Start(context.Background(), SandboxConfig{RootfsURI: "amitia://runtime/test"}); err != nil {
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

func TestExecuteRequestValidationRejectsEmptyExecID(t *testing.T) {
	backend := &ishBackend{bridge: newFakeNativeBridge()}
	backend.bridge.(*fakeNativeBridge).availability = BackendRunning

	_, err := backend.Execute(context.Background(), SandboxExecuteRequest{
		Argv: []string{"echo"},
	})
	if err == nil {
		t.Fatal("expected error for empty execution ID")
	}
}

func TestExecuteRequestValidationRejectsEmptyArgv(t *testing.T) {
	backend := &ishBackend{bridge: newFakeNativeBridge()}
	backend.bridge.(*fakeNativeBridge).availability = BackendRunning

	_, err := backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID: "e1",
	})
	if err == nil {
		t.Fatal("expected error for empty argv")
	}
}

func TestExecuteRequestValidationRejectsNulInArgv(t *testing.T) {
	backend := &ishBackend{bridge: newFakeNativeBridge()}
	backend.bridge.(*fakeNativeBridge).availability = BackendRunning

	_, err := backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID: "e1",
		Argv:        []string{"echo", "hello\x00world"},
	})
	if err == nil {
		t.Fatal("expected error for NUL in argv")
	}
}

func TestExecuteRequestValidationRejectsInvalidEnvKey(t *testing.T) {
	backend := &ishBackend{bridge: newFakeNativeBridge()}
	backend.bridge.(*fakeNativeBridge).availability = BackendRunning

	_, err := backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID: "e1",
		Argv:        []string{"echo"},
		Environment: map[string]string{"123INVALID": "v"},
	})
	if err == nil {
		t.Fatal("expected error for invalid env key")
	}
}

func TestExecuteRequestValidationRejectsStdinTooLarge(t *testing.T) {
	backend := &ishBackend{bridge: newFakeNativeBridge()}
	backend.bridge.(*fakeNativeBridge).availability = BackendRunning

	bigStdin := make([]byte, MaxStdinSize+1)
	_, err := backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID: "e1",
		Argv:        []string{"echo"},
		Stdin:       bigStdin,
	})
	if err == nil {
		t.Fatal("expected error for stdin too large")
	}
}

func TestExecuteRequestAcceptsValidRequest(t *testing.T) {
	backend := &ishBackend{bridge: newFakeNativeBridge()}
	backend.bridge.(*fakeNativeBridge).availability = BackendRunning

	_, err := backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID: "e1",
		Argv:        []string{"echo", "hello"},
		Stdin:       []byte("input"),
		Environment: map[string]string{"HOME": "/root"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteFailsWhenBackendNotRunning(t *testing.T) {
	backend := &ishBackend{bridge: newFakeNativeBridge()}
	backend.bridge.(*fakeNativeBridge).availability = BackendStarting

	_, err := backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID: "e1",
		Argv:        []string{"echo"},
	})
	if err == nil {
		t.Fatal("expected error when backend not running")
	}
	var bridgeErr *NativeBridgeError
	if err.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
	_ = bridgeErr
}

func TestCancelDelegatesToBridge(t *testing.T) {
	bridge := newFakeNativeBridge()
	bridge.availability = BackendRunning
	backend := &ishBackend{bridge: bridge}

	if err := backend.Cancel(context.Background(), "exec-123"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
}

func TestCancelRejectsEmptyExecutionID(t *testing.T) {
	backend := &ishBackend{bridge: newFakeNativeBridge()}
	backend.bridge.(*fakeNativeBridge).availability = BackendRunning

	err := backend.Cancel(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty execution ID")
	}
}

func TestSandboxConfigValidateHostConfig_RequiresRuntimeID(t *testing.T) {
	config := SandboxConfig{
		RootfsURI: "amitia://runtime/test",
	}

	err := config.ValidateHostConfig()
	if err == nil {
		t.Fatal("expected error when RuntimeID is empty")
	}
}

func TestSandboxConfigValidateHostConfig_ValidWorkspaceURI(t *testing.T) {
	config := SandboxConfig{
		RuntimeID:    "test-runtime",
		WorkspaceURI: "amitia://workspace/test",
	}

	if err := config.ValidateHostConfig(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSandboxConfigValidateHostConfig_InvalidWorkspaceURI(t *testing.T) {
	config := SandboxConfig{
		RuntimeID:    "test-runtime",
		WorkspaceURI: "://invalid",
	}

	err := config.ValidateHostConfig()
	if err == nil {
		t.Fatal("expected error for invalid workspaceUri")
	}
}

func TestSandboxConfigValidateHostConfig_ValidRootfsURI(t *testing.T) {
	config := SandboxConfig{
		RuntimeID: "test-runtime",
		RootfsURI: "amitia://runtime/rootfs",
	}

	if err := config.ValidateHostConfig(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSandboxConfigValidateHostConfig_InvalidRootfsURI(t *testing.T) {
	config := SandboxConfig{
		RuntimeID: "test-runtime",
		RootfsURI: "://invalid",
	}

	err := config.ValidateHostConfig()
	if err == nil {
		t.Fatal("expected error for invalid rootfsUri")
	}
}

func TestSandboxConfigValidateHostConfig_EmptyURIsValid(t *testing.T) {
	config := SandboxConfig{
		RuntimeID: "test-runtime",
	}

	if err := config.ValidateHostConfig(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSandboxConfigClone_DefensiveCopy(t *testing.T) {
	original := map[string]string{
		"FOO": "bar",
	}
	config := SandboxConfig{
		RuntimeID:   "test",
		Environment: original,
	}

	cloned := config.Clone()

	cloned.Environment["NEW"] = "value"

	if _, ok := original["NEW"]; ok {
		t.Error("Clone should not share Environment map with original")
	}
}

func TestSandboxConfigCopy_NilEnvironment(t *testing.T) {
	config := SandboxConfig{
		RuntimeID: "test",
	}

	cloned := config.Clone()

	if cloned.Environment != nil {
		t.Error("Clone with nil environment should remain nil")
	}
}

func TestSandboxConfigClone_Isolation(t *testing.T) {
	original := map[string]string{"KEY": "original"}
	config := SandboxConfig{
		RuntimeID:   "test",
		Environment: original,
	}

	cloned := config.Clone()

	cloned.RuntimeID = "modified"
	cloned.RootfsURI = "amitia://runtime/modified"
	cloned.Environment["KEY"] = "modified"

	if config.RuntimeID != "test" {
		t.Error("Clone modification should not affect original RuntimeID")
	}
	if config.RootfsURI != "" {
		t.Error("Clone modification should not affect original RootfsURI")
	}
	if original["KEY"] != "original" {
		t.Error("Clone Environment modification should not affect original map")
	}
}

func TestNoFakeSuccessPath(t *testing.T) {
	backend, err := NewIOSSandboxBackend()
	if err != nil {
		t.Fatalf("NewIOSSandboxBackend: %v", err)
	}

	_, err = backend.Execute(context.Background(), SandboxExecuteRequest{
		ExecutionID: "exec-test",
		Argv:        []string{"echo", "should-fail"},
	})
	if err == nil {
		t.Fatal("Execute must not succeed on non-iOS platform without native bridge")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "iOS") && !strings.Contains(errStr, "ios") &&
		!strings.Contains(errStr, "native") && !strings.Contains(errStr, "unavailable") {
		t.Fatalf("expected descriptive error about platform, got: %v", err)
	}
}

func TestNativeBridgeErrorFormat(t *testing.T) {
	err := &NativeBridgeError{Code: "ISH_TEST", Message: "test message"}
	if err.Error() != "ISH_TEST: test message" {
		t.Fatalf("unexpected error format: %s", err.Error())
	}
}

func TestNativeBridgeErrorNoCode(t *testing.T) {
	err := &NativeBridgeError{Message: "only message"}
	if err.Error() != "only message" {
		t.Fatalf("unexpected error format: %s", err.Error())
	}
}

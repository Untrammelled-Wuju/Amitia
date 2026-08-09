package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type fakeProcessSupervisor struct {
	instances    map[string]*fakeInstance
	startErr     error
	stopErr      error
	definitions  map[string]*trusted_service.ServiceRuntimeDefinition
}

type fakeInstance struct {
	instanceID string
	serviceID  string
	state      trusted_service.ServiceState
	pid        int
}

func newFakeSupervisor() *fakeProcessSupervisor {
	return &fakeProcessSupervisor{
		instances:   make(map[string]*fakeInstance),
		definitions: make(map[string]*trusted_service.ServiceRuntimeDefinition),
	}
}

func (f *fakeProcessSupervisor) Start(ctx context.Context, req trusted_service.StartRequest) (*trusted_service.StartResult, error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	if _, exists := f.instances[req.ServiceID]; exists {
		return nil, trusted_service.ErrAlreadyRunning
	}
	inst := &fakeInstance{
		instanceID: req.InstanceID,
		serviceID:  req.ServiceID,
		state:      trusted_service.ServiceStateReady,
		pid:        12345,
	}
	f.instances[req.ServiceID] = inst
	return &trusted_service.StartResult{
		InstanceID: req.InstanceID,
		PID:        12345,
		State:      trusted_service.ServiceStateReady,
	}, nil
}

func (f *fakeProcessSupervisor) Stop(ctx context.Context, req trusted_service.StopRequest) (*trusted_service.StopResult, error) {
	if f.stopErr != nil {
		return nil, f.stopErr
	}
	inst, exists := f.instances[req.ServiceID]
	if !exists {
		return nil, trusted_service.ErrServiceNotFound
	}
	inst.state = trusted_service.ServiceStateStopped
	return &trusted_service.StopResult{
		ServiceID: req.ServiceID,
		State:     trusted_service.ServiceStateStopped,
	}, nil
}

func (f *fakeProcessSupervisor) Get(serviceID string) (*fakeInstance, error) {
	inst, exists := f.instances[serviceID]
	if !exists {
		return nil, trusted_service.ErrServiceNotFound
	}
	return inst, nil
}

func (f *fakeProcessSupervisor) Register(def *trusted_service.ServiceRuntimeDefinition) error {
	f.definitions[def.ServiceID] = def
	return nil
}

func TestBuildProcessInstanceID(t *testing.T) {
	id := BuildProcessInstanceID("runtime-1", "bridge-service")
	expected := "runtime-1/bridge-service"
	if id != expected {
		t.Errorf("expected %s, got %s", expected, id)
	}
}

func TestParseProcessInstanceID(t *testing.T) {
	runtimeID, serviceID, err := ParseProcessInstanceID("runtime-1/bridge-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtimeID != "runtime-1" {
		t.Errorf("expected runtime-1, got %s", runtimeID)
	}
	if serviceID != "bridge-service" {
		t.Errorf("expected bridge-service, got %s", serviceID)
	}
}

func TestParseProcessInstanceID_Invalid(t *testing.T) {
	_, _, err := ParseProcessInstanceID("invalid-no-slash")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestProcessInstanceID_RoundTrip(t *testing.T) {
	originalRuntimeID := domain.RuntimeInstanceID("rt-abc-123")
	originalServiceID := domain.ServiceID("my-service-xyz")
	id := BuildProcessInstanceID(originalRuntimeID, originalServiceID)
	runtimeID, serviceID, err := ParseProcessInstanceID(id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if runtimeID != originalRuntimeID {
		t.Errorf("runtimeID mismatch: expected %s, got %s", originalRuntimeID, runtimeID)
	}
	if serviceID != originalServiceID {
		t.Errorf("serviceID mismatch: expected %s, got %s", originalServiceID, serviceID)
	}
}

func TestMultipleRuntimesSameDefinition(t *testing.T) {
	idA := BuildProcessInstanceID("runtime-a", "bridge")
	idB := BuildProcessInstanceID("runtime-b", "bridge")
	if idA == idB {
		t.Errorf("expected different IDs for different runtimes, got same: %s", idA)
	}
}

func TestServiceExecutionContext_DirectoryIdentifier(t *testing.T) {
	ctx := ServiceExecutionContext{
		RuntimeID: "rt-1",
		ServiceID: "bridge",
	}
	id := ctx.DirectoryIdentifier()
	expected := "rt-1/bridge"
	if id != expected {
		t.Errorf("expected %s, got %s", expected, id)
	}
}

type fakeAdapter struct {
	startResult *trusted_service.StartResult
	startHandle *ServiceExecutionHandle
	startErr    error
	stopErr     error
	runningKeys map[string]bool
}

func newFakeAdapter() *fakeAdapter {
	return &fakeAdapter{
		runningKeys: make(map[string]bool),
	}
}

func (f *fakeAdapter) StartProcess(ctx context.Context, def *trusted_service.ServiceRuntimeDefinition, execCtx ServiceExecutionContext) (*trusted_service.StartResult, *ServiceExecutionHandle, error) {
	if f.startErr != nil {
		return nil, nil, f.startErr
	}
	instanceID := execCtx.DirectoryIdentifier()
	f.runningKeys[instanceID] = true
	handle := &ServiceExecutionHandle{
		RuntimeID:  string(execCtx.RuntimeID),
		ServiceID:  string(execCtx.ServiceID),
		InstanceID: instanceID,
		PID:        12345,
	}
	return &trusted_service.StartResult{
		InstanceID: instanceID,
		PID:        12345,
		State:      trusted_service.ServiceStateReady,
	}, handle, nil
}

func (f *fakeAdapter) StopProcess(ctx context.Context, handle ServiceExecutionHandle, force bool) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	delete(f.runningKeys, handle.InstanceID)
	return nil
}

func (f *fakeAdapter) IsRunning(supervisorKey string) bool {
	return f.runningKeys[supervisorKey]
}

func TestProcessAdapter_NilSupervisor(t *testing.T) {
	_, err := NewProcessSupervisorAdapter(nil)
	if err == nil {
		t.Fatal("expected error for nil supervisor")
	}
}

func TestServiceExecutionHandle_NilDef(t *testing.T) {
	adapter := newFakeAdapter()
	adapter.startErr = errors.New("nil definition")
	execCtx := ServiceExecutionContext{
		RuntimeID: "rt-1",
		ServiceID: "bridge",
	}
	_, _, err := adapter.StartProcess(nil, nil, execCtx)
	if err == nil {
		t.Fatal("expected error for nil definition")
	}
}

func TestStageExecutionResult(t *testing.T) {
	result := NewStageExecutionResult(0)
	if result.HasFailures() {
		t.Error("new result should not have failures")
	}
	result.AddStarted("svc-1")
	result.AddStarted("svc-2")
	if len(result.Started) != 2 {
		t.Errorf("expected 2 started, got %d", len(result.Started))
	}
	result.AddFailure("svc-3", errors.New("failed"))
	if !result.HasFailures() {
		t.Error("expected failures")
	}
	if result.FailedCount() != 1 {
		t.Errorf("expected 1 failed, got %d", result.FailedCount())
	}
}

func TestRollbackResult(t *testing.T) {
	result := NewRollbackResult("rt-1")
	result.AddStopped("svc-1")
	result.AddStopped("svc-2")
	if result.StoppedCount != 2 {
		t.Errorf("expected 2 stopped, got %d", result.StoppedCount)
	}
	result.AddError(errors.New("stop failed"))
	if !result.HasErrors() {
		t.Error("expected errors")
	}
}

func TestShutdownResult(t *testing.T) {
	result := NewShutdownResult("rt-1")
	result.AddStopped("svc-1")
	result.AddStopFailure("svc-2", errors.New("timeout"))
	result.AddCleanupError(errors.New("cleanup failed"))
	if !result.HasErrors() {
		t.Error("expected errors")
	}
	if len(result.Stopped) != 1 {
		t.Errorf("expected 1 stopped, got %d", len(result.Stopped))
	}
}

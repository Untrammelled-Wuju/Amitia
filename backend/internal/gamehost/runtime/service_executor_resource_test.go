package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type resourceTestTopology struct {
	svc           *ServiceInstance
	failOnRunning bool
}

func (t *resourceTestTopology) GetService(serviceID domain.ServiceID) (*ServiceInstance, error) {
	if t.svc == nil || t.svc.ServiceID != serviceID {
		return nil, errors.New("service not found")
	}
	return t.svc, nil
}

func (t *resourceTestTopology) UpdateServiceState(serviceID domain.ServiceID, next ServiceRuntimeState, now time.Time) error {
	if t.svc == nil || t.svc.ServiceID != serviceID {
		return errors.New("service not found")
	}
	if next == ServiceStateRunning && t.failOnRunning {
		return errors.New("injected running-state persistence failure")
	}
	return t.svc.Transition(next, now)
}

func (t *resourceTestTopology) Snapshot() RuntimeTopologySnapshot {
	if t.svc == nil {
		return RuntimeTopologySnapshot{}
	}
	return RuntimeTopologySnapshot{
		RuntimeID: t.svc.RuntimeID,
		PluginID:  t.svc.PluginID,
		Services:  []ServiceInstanceSnapshot{t.svc.Snapshot()},
	}
}

func (t *resourceTestTopology) ListServices() []ServiceInstanceSnapshot {
	if t.svc == nil {
		return nil
	}
	return []ServiceInstanceSnapshot{t.svc.Snapshot()}
}

type resourceTestTopologyStore struct {
	topology TopologyAccessor
}

func (s resourceTestTopologyStore) GetTopologySnapshot(runtimeID domain.RuntimeInstanceID) (RuntimeTopologySnapshot, error) {
	return s.topology.Snapshot(), nil
}

func (s resourceTestTopologyStore) GetDependencyGraphSnapshot(runtimeID domain.RuntimeInstanceID) (DependencyGraphSnapshot, error) {
	return DependencyGraphSnapshot{}, nil
}

func (s resourceTestTopologyStore) GetTopology(runtimeID domain.RuntimeInstanceID) (TopologyAccessor, error) {
	return s.topology, nil
}

type resourceTestDefinitionResolver struct{}

func (resourceTestDefinitionResolver) ResolveDefinitionID(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (string, error) {
	return "def-1", nil
}

type allowServiceStart struct{}

func (allowServiceStart) AuthorizeServiceStart(context.Context, ServiceExecutionContext, *trusted_service.ServiceRuntimeDefinition) error {
	return nil
}

type recordingServiceResourceAdmission struct {
	prepared    int
	finished    []bool
	released    int
	releasedRun domain.RuntimeInstanceID
	releasedSvc domain.ServiceID
	prepareErr  error
}

func (a *recordingServiceResourceAdmission) PrepareServiceStart(context.Context, ServiceExecutionContext, *trusted_service.ServiceRuntimeDefinition) (func(bool), error) {
	a.prepared++
	if a.prepareErr != nil {
		return nil, a.prepareErr
	}
	return func(started bool) {
		a.finished = append(a.finished, started)
	}, nil
}

func (a *recordingServiceResourceAdmission) ReleaseService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	a.released++
	a.releasedRun = runtimeID
	a.releasedSvc = serviceID
}

type recordingLeaseLifecycle struct {
	prepared int
	revoked  int
	reason   string
}

func (l *recordingLeaseLifecycle) PrepareServiceStart(context.Context, ServiceExecutionContext) (*contracts.RuntimeSecretLeaseSession, error) {
	l.prepared++
	return &contracts.RuntimeSecretLeaseSession{SessionID: "lease-session-1"}, nil
}

func (l *recordingLeaseLifecycle) RevokeServiceLeases(_ domain.RuntimeInstanceID, _ domain.ServiceID, _ int64, reason string) {
	l.revoked++
	l.reason = reason
}

func newResourceTestService(t *testing.T, state ServiceRuntimeState) *ServiceInstance {
	t.Helper()
	now := time.Now()
	svc, err := NewServiceInstance(
		BuildServiceInstanceID("rt-1", "svc-1"),
		"rt-1",
		"plugin-1",
		"svc-1",
		true,
		domain.ServiceKindProcess,
		nil,
		now,
	)
	if err != nil {
		t.Fatalf("NewServiceInstance: %v", err)
	}
	switch state {
	case ServiceStateCreated:
	case ServiceStateRunning:
		if err := svc.Transition(ServiceStateStarting, now.Add(time.Millisecond)); err != nil {
			t.Fatalf("transition starting: %v", err)
		}
		if err := svc.Transition(ServiceStateRunning, now.Add(2*time.Millisecond)); err != nil {
			t.Fatalf("transition running: %v", err)
		}
	default:
		t.Fatalf("unsupported test state %s", state)
	}
	return svc
}

func TestNewServiceExecutorRequiresResourceAdmission(t *testing.T) {
	_, err := NewServiceExecutor(
		newFakeAdapter(),
		NewUnavailableExternalServiceAdapter(),
		resourceTestTopologyStore{topology: &resourceTestTopology{svc: newResourceTestService(t, ServiceStateCreated)}},
		resourceTestDefinitionResolver{},
		allowServiceStart{},
		nil,
	)
	if err == nil {
		t.Fatal("expected nil resource admission to be rejected")
	}
}

func TestServiceExecutorRollsBackSpawnWhenRunningStatePersistenceFails(t *testing.T) {
	svc := newResourceTestService(t, ServiceStateCreated)
	topology := &resourceTestTopology{svc: svc, failOnRunning: true}
	processAdapter := newFakeAdapter()
	admission := &recordingServiceResourceAdmission{}
	executor, err := NewServiceExecutor(
		processAdapter,
		NewUnavailableExternalServiceAdapter(),
		resourceTestTopologyStore{topology: topology},
		resourceTestDefinitionResolver{},
		allowServiceStart{},
		admission,
	)
	if err != nil {
		t.Fatalf("NewServiceExecutor: %v", err)
	}

	entry := ServicePlanEntry{ServiceInstanceID: svc.ID, ServiceID: svc.ServiceID, Required: true}
	ctx := WithExecutionGeneration(context.Background(), 1)
	_, err = executor.Start(ctx, entry, func(string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return &trusted_service.ServiceRuntimeDefinition{ServiceID: "svc-1"}, nil
	})
	if err == nil {
		t.Fatal("expected running-state persistence failure")
	}
	if processAdapter.IsRunning(string(svc.ID)) {
		t.Fatal("process must be force-stopped after failed running-state persistence")
	}
	if admission.prepared != 1 {
		t.Fatalf("resource admission prepare count = %d, want 1", admission.prepared)
	}
	if len(admission.finished) != 1 || admission.finished[0] {
		t.Fatalf("resource admission must roll back failed start, got %+v", admission.finished)
	}
	if svc.State != ServiceStateFailed {
		t.Fatalf("service state = %s, want failed", svc.State)
	}
}

func TestServiceExecutorReleasesResourceStateAfterStop(t *testing.T) {
	svc := newResourceTestService(t, ServiceStateRunning)
	topology := &resourceTestTopology{svc: svc}
	processAdapter := newFakeAdapter()
	processAdapter.runningKeys[string(svc.ID)] = true
	admission := &recordingServiceResourceAdmission{}
	executor, err := NewServiceExecutor(
		processAdapter,
		NewUnavailableExternalServiceAdapter(),
		resourceTestTopologyStore{topology: topology},
		resourceTestDefinitionResolver{},
		allowServiceStart{},
		admission,
	)
	if err != nil {
		t.Fatalf("NewServiceExecutor: %v", err)
	}

	handle := ServiceExecutionHandle{RuntimeID: "rt-1", ServiceID: "svc-1", InstanceID: string(svc.ID), PID: 12345}
	if err := executor.Stop(WithExecutionGeneration(context.Background(), 1), handle, true); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if admission.released != 1 || admission.releasedRun != "rt-1" || admission.releasedSvc != "svc-1" {
		t.Fatalf("resource release mismatch: count=%d runtime=%s service=%s", admission.released, admission.releasedRun, admission.releasedSvc)
	}
	if svc.State != ServiceStateStopped {
		t.Fatalf("service state = %s, want stopped", svc.State)
	}
}

func TestServiceExecutorRevokesLeaseWhenProcessStartFails(t *testing.T) {
	svc := newResourceTestService(t, ServiceStateCreated)
	topology := &resourceTestTopology{svc: svc}
	processAdapter := newFakeAdapter()
	processAdapter.startErr = errors.New("injected process start failure")
	admission := &recordingServiceResourceAdmission{}
	executor, err := NewServiceExecutor(
		processAdapter,
		NewUnavailableExternalServiceAdapter(),
		resourceTestTopologyStore{topology: topology},
		resourceTestDefinitionResolver{},
		allowServiceStart{},
		admission,
	)
	if err != nil {
		t.Fatalf("NewServiceExecutor: %v", err)
	}
	lease := &recordingLeaseLifecycle{}
	if aware, ok := executor.(SecretLeaseAwareServiceExecutor); ok {
		aware.SetServiceLeaseLifecycle(lease)
	} else {
		t.Fatal("service executor must support lease lifecycle")
	}

	entry := ServicePlanEntry{ServiceInstanceID: svc.ID, ServiceID: svc.ServiceID, Required: true}
	_, err = executor.Start(WithExecutionGeneration(context.Background(), 1), entry, func(string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return &trusted_service.ServiceRuntimeDefinition{ServiceID: "svc-1"}, nil
	})
	if err == nil {
		t.Fatal("expected process start failure")
	}
	if lease.prepared != 1 || lease.revoked != 1 {
		t.Fatalf("lease lifecycle prepared=%d revoked=%d, want 1/1", lease.prepared, lease.revoked)
	}
	if lease.reason != "service startup failed" {
		t.Fatalf("revoke reason = %q", lease.reason)
	}
	if len(admission.finished) != 1 || admission.finished[0] {
		t.Fatalf("resource admission must roll back failed start, got %+v", admission.finished)
	}
	if svc.State != ServiceStateFailed {
		t.Fatalf("service state = %s, want failed", svc.State)
	}
}

type nilHandleProcessAdapter struct {
	result      *trusted_service.StartResult
	stopErr     error
	stopCalled  int
	stoppedWith ServiceExecutionHandle
}

func (a *nilHandleProcessAdapter) StartProcess(context.Context, *trusted_service.ServiceRuntimeDefinition, ServiceExecutionContext) (*trusted_service.StartResult, *ServiceExecutionHandle, error) {
	return a.result, nil, nil
}

func (a *nilHandleProcessAdapter) StopProcess(_ context.Context, handle ServiceExecutionHandle, _ bool) error {
	a.stopCalled++
	a.stoppedWith = handle
	return a.stopErr
}

func (a *nilHandleProcessAdapter) IsRunning(string) bool { return false }

func TestServiceExecutorForceStopsSuccessfulStartWithoutHandle(t *testing.T) {
	svc := newResourceTestService(t, ServiceStateCreated)
	topology := &resourceTestTopology{svc: svc}
	adapter := &nilHandleProcessAdapter{result: &trusted_service.StartResult{
		InstanceID: "rt-1/svc-1",
		PID:        4242,
		State:      trusted_service.ServiceStateReady,
	}}
	admission := &recordingServiceResourceAdmission{}
	executor, err := NewServiceExecutor(
		adapter,
		NewUnavailableExternalServiceAdapter(),
		resourceTestTopologyStore{topology: topology},
		resourceTestDefinitionResolver{},
		allowServiceStart{},
		admission,
	)
	if err != nil {
		t.Fatalf("NewServiceExecutor: %v", err)
	}

	entry := ServicePlanEntry{ServiceInstanceID: svc.ID, ServiceID: svc.ServiceID, Required: true}
	_, err = executor.Start(WithExecutionGeneration(context.Background(), 1), entry, func(string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return &trusted_service.ServiceRuntimeDefinition{ServiceID: "svc-1"}, nil
	})
	if err == nil {
		t.Fatal("expected nil-handle contract violation")
	}
	if adapter.stopCalled != 1 {
		t.Fatalf("force-stop calls = %d, want 1", adapter.stopCalled)
	}
	if adapter.stoppedWith.InstanceID != "rt-1/svc-1" || adapter.stoppedWith.PID != 4242 {
		t.Fatalf("cleanup handle = %+v", adapter.stoppedWith)
	}
	if len(admission.finished) != 1 || admission.finished[0] {
		t.Fatalf("resource admission must roll back failed start, got %+v", admission.finished)
	}
	if svc.State != ServiceStateFailed {
		t.Fatalf("service state = %s, want failed", svc.State)
	}
}

func TestServiceExecutorSurfacesRollbackFailureAfterRunningStatePersistenceFailure(t *testing.T) {
	svc := newResourceTestService(t, ServiceStateCreated)
	topology := &resourceTestTopology{svc: svc, failOnRunning: true}
	processAdapter := newFakeAdapter()
	processAdapter.stopErr = errors.New("injected cleanup failure")
	admission := &recordingServiceResourceAdmission{}
	executor, err := NewServiceExecutor(
		processAdapter,
		NewUnavailableExternalServiceAdapter(),
		resourceTestTopologyStore{topology: topology},
		resourceTestDefinitionResolver{},
		allowServiceStart{},
		admission,
	)
	if err != nil {
		t.Fatalf("NewServiceExecutor: %v", err)
	}

	entry := ServicePlanEntry{ServiceInstanceID: svc.ID, ServiceID: svc.ServiceID, Required: true}
	_, err = executor.Start(WithExecutionGeneration(context.Background(), 1), entry, func(string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return &trusted_service.ServiceRuntimeDefinition{ServiceID: "svc-1"}, nil
	})
	if err == nil {
		t.Fatal("expected start rollback failure")
	}
	if got := err.Error(); !strings.Contains(got, "force-stop failed") || !strings.Contains(got, "injected cleanup failure") {
		t.Fatalf("error must surface rollback failure, got %q", got)
	}
	if len(admission.finished) != 1 || admission.finished[0] {
		t.Fatalf("resource admission must roll back failed start, got %+v", admission.finished)
	}
	if svc.State != ServiceStateFailed {
		t.Fatalf("service state = %s, want failed", svc.State)
	}
}

package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type fakeRuntimeManager struct {
	runtimes map[domain.RuntimeInstanceID]*RuntimeInstanceRef
}

func newFakeRuntimeManager() *fakeRuntimeManager {
	return &fakeRuntimeManager{
		runtimes: make(map[domain.RuntimeInstanceID]*RuntimeInstanceRef),
	}
}

func (m *fakeRuntimeManager) AddRuntime(runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, state domain.RuntimeState) {
	m.runtimes[runtimeID] = &RuntimeInstanceRef{
		ID:       runtimeID,
		PluginID: pluginID,
		State:    state,
	}
}

func (m *fakeRuntimeManager) GetRuntime(runtimeID domain.RuntimeInstanceID) (*RuntimeInstanceRef, error) {
	rt, ok := m.runtimes[runtimeID]
	if !ok {
		return nil, &ExecutionError{Code: ErrRuntimeUnavailable, RuntimeID: string(runtimeID), Message: "runtime not found"}
	}
	return rt, nil
}

func (m *fakeRuntimeManager) UpdateRuntimeState(runtimeID domain.RuntimeInstanceID, next domain.RuntimeState, reason string, nowTime time.Time) error {
	rt, ok := m.runtimes[runtimeID]
	if !ok {
		return &ExecutionError{Code: ErrRuntimeUnavailable, RuntimeID: string(runtimeID), Message: "runtime not found"}
	}
	rt.State = next
	return nil
}

func (m *fakeRuntimeManager) ListRuntimes() []*RuntimeInstanceRef {
	result := make([]*RuntimeInstanceRef, 0, len(m.runtimes))
	for _, rt := range m.runtimes {
		result = append(result, rt)
	}
	return result
}

type fakeTopologyStore struct {
	topology RuntimeTopologySnapshot
	graph    DependencyGraphSnapshot
	accessor TopologyAccessor
}

func newFakeTopologyStore() *fakeTopologyStore {
	return &fakeTopologyStore{}
}

func (s *fakeTopologyStore) SetTopology(t RuntimeTopologySnapshot) {
	s.topology = t
	s.accessor = &fakeTopologyAccessor{topology: t}
}

func (s *fakeTopologyStore) SetGraph(g DependencyGraphSnapshot) {
	s.graph = g
}

func (s *fakeTopologyStore) GetTopologySnapshot(runtimeID domain.RuntimeInstanceID) (RuntimeTopologySnapshot, error) {
	return s.topology, nil
}

func (s *fakeTopologyStore) GetDependencyGraphSnapshot(runtimeID domain.RuntimeInstanceID) (DependencyGraphSnapshot, error) {
	return s.graph, nil
}

func (s *fakeTopologyStore) GetTopology(runtimeID domain.RuntimeInstanceID) (TopologyAccessor, error) {
	return s.accessor, nil
}

func (s *fakeTopologyStore) UpdateServiceState(serviceID domain.ServiceID, next ServiceRuntimeState, now time.Time) error {
	return s.accessor.UpdateServiceState(serviceID, next, now)
}

type fakeServiceExecutorWithDefs struct {
	startFn      func(ctx context.Context, entry ServicePlanEntry, resolveFn DefinitionResolverFunc) (*ServiceExecutionHandle, error)
	stopFn       func(ctx context.Context, handle ServiceExecutionHandle, force bool) error
	startedSvcs  []domain.ServiceID
	stoppedSvcs  []domain.ServiceID
}

func (f *fakeServiceExecutorWithDefs) Start(ctx context.Context, entry ServicePlanEntry, resolveFn DefinitionResolverFunc) (*ServiceExecutionHandle, error) {
	f.startedSvcs = append(f.startedSvcs, entry.ServiceID)
	if f.startFn != nil {
		return f.startFn(ctx, entry, resolveFn)
	}
	return &ServiceExecutionHandle{
		RuntimeID:  "rt-1",
		ServiceID:  string(entry.ServiceID),
		InstanceID: "rt-1/" + string(entry.ServiceID),
		PID:        12345,
	}, nil
}

func (f *fakeServiceExecutorWithDefs) Stop(ctx context.Context, handle ServiceExecutionHandle, force bool) error {
	f.stoppedSvcs = append(f.stoppedSvcs, domain.ServiceID(handle.ServiceID))
	if f.stopFn != nil {
		return f.stopFn(ctx, handle, force)
	}
	return nil
}

func TestNewRuntimeExecutor_NilArgs(t *testing.T) {
	_, err := NewRuntimeExecutor(nil, newFakeRuntimeManager(), &fakeServiceExecutorWithDefs{}, nil)
	if err == nil {
		t.Fatal("expected error for nil topology store")
	}

	topoStore := newFakeTopologyStore()
	_, err = NewRuntimeExecutor(topoStore, nil, &fakeServiceExecutorWithDefs{}, nil)
	if err == nil {
		t.Fatal("expected error for nil runtime manager")
	}

	_, err = NewRuntimeExecutor(topoStore, newFakeRuntimeManager(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil service executor")
	}
}

func TestRuntimeExecutor_StartRuntime_Success(t *testing.T) {
	rtManager := newFakeRuntimeManager()
	rtManager.AddRuntime("rt-1", "plugin-1", domain.RuntimeStateCreated)

	topoStore := newFakeTopologyStore()
	topoStore.SetTopology(RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		PluginID:  "plugin-1",
		Services: []ServiceInstanceSnapshot{
			{
				ID:        "rt-1/bridge",
				RuntimeID: "rt-1",
				PluginID:  "plugin-1",
				ServiceID: "bridge",
				State:     ServiceStateCreated,
				Required:  true,
			},
			{
				ID:           "rt-1/agent",
				RuntimeID:    "rt-1",
				PluginID:     "plugin-1",
				ServiceID:    "agent",
				State:        ServiceStateCreated,
				Required:     true,
				Dependencies: []domain.ServiceID{"bridge"},
			},
		},
	})
	topoStore.SetGraph(DependencyGraphSnapshot{
		RuntimeID: "rt-1",
		Nodes: []DependencyNodeSnapshot{
			{ServiceID: "bridge", Dependents: []domain.ServiceID{"agent"}},
			{ServiceID: "agent", Dependencies: []domain.ServiceID{"bridge"}},
		},
	})

	svcExecutor := &fakeServiceExecutorWithDefs{}
	exec, err := NewRuntimeExecutor(topoStore, rtManager, svcExecutor, NewLifecyclePlanner())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolveFn := func(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return &trusted_service.ServiceRuntimeDefinition{
			ServiceID:  definitionID,
			ExtensionID: "ext-1",
			ModuleID:   "mod-1",
		}, nil
	}
	exec.SetResolveDefinition(resolveFn)

	ctx := context.Background()
	err = exec.StartRuntime(ctx, "rt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svcExecutor.startedSvcs) != 2 {
		t.Errorf("expected 2 services started, got %d: %v", len(svcExecutor.startedSvcs), svcExecutor.startedSvcs)
	}

	startOrder := make(map[domain.ServiceID]int)
	for i, svc := range svcExecutor.startedSvcs {
		startOrder[svc] = i
	}
	if startOrder["bridge"] >= startOrder["agent"] {
		t.Error("expected bridge to start before agent")
	}
}

func TestRuntimeExecutor_StartRuntime_InvalidState(t *testing.T) {
	rtManager := newFakeRuntimeManager()
	rtManager.AddRuntime("rt-1", "plugin-1", domain.RuntimeStateRunning)

	topoStore := newFakeTopologyStore()
	svcExecutor := &fakeServiceExecutorWithDefs{}
	exec, err := NewRuntimeExecutor(topoStore, rtManager, svcExecutor, NewLifecyclePlanner())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	err = exec.StartRuntime(ctx, "rt-1")
	if err == nil {
		t.Fatal("expected error for invalid runtime state")
	}
	if !IsExecutionError(err, ErrInvalidState) {
		t.Errorf("expected invalid_state error, got %v", err)
	}
}

func TestRuntimeExecutor_StopRuntime_Success(t *testing.T) {
	rtManager := newFakeRuntimeManager()
	rtManager.AddRuntime("rt-1", "plugin-1", domain.RuntimeStateCreated)

	topoStore := newFakeTopologyStore()
	topoStore.SetTopology(RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		PluginID:  "plugin-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", State: ServiceStateCreated, Required: true},
			{ServiceID: "agent", State: ServiceStateCreated, Required: true, Dependencies: []domain.ServiceID{"bridge"}},
		},
	})
	topoStore.SetGraph(DependencyGraphSnapshot{
		RuntimeID: "rt-1",
		Nodes: []DependencyNodeSnapshot{
			{ServiceID: "bridge", Dependents: []domain.ServiceID{"agent"}},
			{ServiceID: "agent", Dependencies: []domain.ServiceID{"bridge"}},
		},
	})

	svcExecutor := &fakeServiceExecutorWithDefs{}
	exec, err := NewRuntimeExecutor(topoStore, rtManager, svcExecutor, NewLifecyclePlanner())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exec.SetResolveDefinition(func(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return &trusted_service.ServiceRuntimeDefinition{ServiceID: definitionID}, nil
	})

	ctx := context.Background()
	err = exec.StartRuntime(ctx, "rt-1")
	if err != nil {
		t.Fatalf("unexpected start error: %v", err)
	}

	rtManager.runtimes["rt-1"].State = domain.RuntimeStateRunning
	err = exec.StopRuntime(ctx, "rt-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(svcExecutor.stoppedSvcs) != 2 {
		t.Errorf("expected 2 services stopped, got %d: %v", len(svcExecutor.stoppedSvcs), svcExecutor.stoppedSvcs)
	}
}

func TestRuntimeExecutor_RuntimeStart_RollbackOnRequiredFailure(t *testing.T) {
	rtManager := newFakeRuntimeManager()
	rtManager.AddRuntime("rt-1", "plugin-1", domain.RuntimeStateCreated)

	topoStore := newFakeTopologyStore()
	topoStore.SetTopology(RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		PluginID:  "plugin-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", State: ServiceStateCreated, Required: true},
			{ServiceID: "agent", State: ServiceStateCreated, Required: true, Dependencies: []domain.ServiceID{"bridge"}},
		},
	})
	topoStore.SetGraph(DependencyGraphSnapshot{
		RuntimeID: "rt-1",
		Nodes: []DependencyNodeSnapshot{
			{ServiceID: "bridge", Dependents: []domain.ServiceID{"agent"}},
			{ServiceID: "agent", Dependencies: []domain.ServiceID{"bridge"}},
		},
	})

	svcExecutor := &fakeServiceExecutorWithDefs{
		startFn: func(ctx context.Context, entry ServicePlanEntry, resolveFn DefinitionResolverFunc) (*ServiceExecutionHandle, error) {
			if string(entry.ServiceID) == "agent" {
				return nil, errors.New("agent start failed")
			}
			return &ServiceExecutionHandle{
				RuntimeID:  "rt-1",
				ServiceID:  string(entry.ServiceID),
				InstanceID: "rt-1/" + string(entry.ServiceID),
			}, nil
		},
	}

	exec, err := NewRuntimeExecutor(topoStore, rtManager, svcExecutor, NewLifecyclePlanner())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolveFn := func(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return &trusted_service.ServiceRuntimeDefinition{ServiceID: definitionID}, nil
	}
	exec.SetResolveDefinition(resolveFn)

	ctx := context.Background()
	startErr := exec.StartRuntime(ctx, "rt-1")

	if startErr == nil {
		t.Fatal("expected start to fail due to required service failure")
	}

	rte, ok := startErr.(*RuntimeStartError)
	if !ok {
		t.Fatalf("expected RuntimeStartError, got %T", startErr)
	}
	if rte.Cause == nil {
		t.Error("expected cause to be set in RuntimeStartError")
	}
}

func TestRuntimeExecutor_ConcurrentRuntimeLocks(t *testing.T) {
	exec1 := &runtimeExecutor{
		runtimeLocks: make(map[domain.RuntimeInstanceID]*sync.Mutex),
	}
	exec2 := &runtimeExecutor{
		runtimeLocks: make(map[domain.RuntimeInstanceID]*sync.Mutex),
	}

	lockA1 := exec1.getRuntimeLock("rt-a")
	lockA2 := exec1.getRuntimeLock("rt-a")
	if lockA1 != lockA2 {
		t.Error("expected same lock for same runtime")
	}

	lockB := exec1.getRuntimeLock("rt-b")
	if lockA1 == lockB {
		t.Error("expected different locks for different runtimes")
	}

	lockC := exec2.getRuntimeLock("rt-c")
	if lockC == nil {
		t.Error("expected non-nil lock")
	}
}

func TestRuntimeStartError(t *testing.T) {
	cause := errors.New("root cause")
	rbErrs := []error{errors.New("rb1"), errors.New("rb2")}
	err := &RuntimeStartError{
		Cause:          cause,
		RuntimeID:      "rt-1",
		RollbackErrors: rbErrs,
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}

	if err.Unwrap() != cause {
		t.Error("expected Unwrap to return cause")
	}
}

func TestRuntimeStopError(t *testing.T) {
	err := &RuntimeStopError{
		RuntimeID:  "rt-1",
		StopErrors: []error{errors.New("stop1")},
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestExecutionError(t *testing.T) {
	err := &ExecutionError{
		Code:         ErrServiceLaunchFailed,
		RuntimeID:    "rt-1",
		PluginID:     "p-1",
		ServiceID:    "svc-1",
		DefinitionID: "def-1",
		Message:      "launch failed",
	}
	if !IsExecutionError(err, ErrServiceLaunchFailed) {
		t.Error("expected error to match code")
	}
	if IsExecutionError(err, ErrInvalidState) {
		t.Error("expected error to NOT match different code")
	}

	wrapped := NewExecutionErrorWithCause(ErrServiceLaunchFailed, "rt", "p", "s", "d", "msg", err)
	if wrapped.Unwrap() != err {
		t.Error("expected Unwrap to return wrapped cause")
	}
}

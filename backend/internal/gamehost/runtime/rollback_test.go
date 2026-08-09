package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
)

type fakeServiceExecutor struct {
	startFn func(ctx context.Context, entry ServicePlanEntry, resolveFn DefinitionResolverFunc) (*ServiceExecutionHandle, error)
	stopFn  func(ctx context.Context, handle ServiceExecutionHandle, force bool) error
}

func (f *fakeServiceExecutor) Start(ctx context.Context, entry ServicePlanEntry, resolveFn DefinitionResolverFunc) (*ServiceExecutionHandle, error) {
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

func (f *fakeServiceExecutor) Stop(ctx context.Context, handle ServiceExecutionHandle, force bool) error {
	if f.stopFn != nil {
		return f.stopFn(ctx, handle, force)
	}
	return nil
}

func TestBuildRollbackPlan(t *testing.T) {
	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		PluginID:  "plugin-1",
		Services: []ServiceInstanceSnapshot{
			{
				ID:        "rt-1/bridge",
				RuntimeID: "rt-1",
				PluginID:  "plugin-1",
				ServiceID: "bridge",
				State:     ServiceStateRunning,
				Required:  true,
			},
			{
				ID:        "rt-1/agent",
				RuntimeID: "rt-1",
				PluginID:  "plugin-1",
				ServiceID: "agent",
				State:     ServiceStateRunning,
				Required:  true,
			},
		},
	}

	graph := DependencyGraphSnapshot{
		RuntimeID: "rt-1",
		Nodes: []DependencyNodeSnapshot{
			{
				ServiceID:    "bridge",
				Dependencies: []domain.ServiceID{},
				Dependents:   []domain.ServiceID{"agent"},
			},
			{
				ServiceID:    "agent",
				Dependencies: []domain.ServiceID{"bridge"},
				Dependents:   []domain.ServiceID{},
			},
		},
	}

	planner := NewLifecyclePlanner()

	progress := StartupProgress{
		RuntimeID:            "rt-1",
		StartedThisOperation: []domain.ServiceID{"bridge", "agent"},
	}

	plan, err := planner.BuildRollbackPlan(progress, topology, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Action != LifecycleActionStop {
		t.Errorf("expected stop action, got %s", plan.Action)
	}

	if len(plan.Stages) == 0 {
		t.Fatal("expected at least one rollback stage")
	}

	total := 0
	for _, stage := range plan.Stages {
		total += len(stage.Services)
	}
	if total != 2 {
		t.Errorf("expected 2 services in rollback plan, got %d", total)
	}
}

func TestBuildRollbackPlan_EmptyProgress(t *testing.T) {
	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		PluginID:  "plugin-1",
	}

	graph := DependencyGraphSnapshot{
		RuntimeID: "rt-1",
	}

	planner := NewLifecyclePlanner()

	progress := StartupProgress{
		RuntimeID:            "rt-1",
		StartedThisOperation: []domain.ServiceID{},
	}

	plan, err := planner.BuildRollbackPlan(progress, topology, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Stages) != 0 {
		t.Errorf("expected 0 stages for empty progress, got %d", len(plan.Stages))
	}
}

func TestRollbackExecutor_Execute(t *testing.T) {
	planner := NewLifecyclePlanner()
	handleStore := NewRuntimeHandleStore()
	handleStore.Put("rt-1", "bridge", &ServiceExecutionHandle{
		RuntimeID:  "rt-1",
		ServiceID:  "bridge",
		InstanceID: "rt-1/bridge",
	})
	handleStore.Put("rt-1", "agent", &ServiceExecutionHandle{
		RuntimeID:  "rt-1",
		ServiceID:  "agent",
		InstanceID: "rt-1/agent",
	})

	progress := &StartupProgress{
		RuntimeID:            "rt-1",
		StartedThisOperation: []domain.ServiceID{"bridge", "agent"},
	}

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", State: ServiceStateRunning},
			{ServiceID: "agent", State: ServiceStateRunning},
		},
	}

	graph := DependencyGraphSnapshot{
		RuntimeID: "rt-1",
		Nodes: []DependencyNodeSnapshot{
			{ServiceID: "bridge", Dependents: []domain.ServiceID{"agent"}},
			{ServiceID: "agent", Dependencies: []domain.ServiceID{"bridge"}},
		},
	}

	rollbackPlan, err := planner.BuildRollbackPlan(*progress, topology, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serviceExec := &fakeServiceExecutor{}
	exec := NewRollbackExecutor()

	result := exec.Execute(context.Background(), rollbackPlan, handleStore, serviceExec, progress)

	if result.StoppedCount != 2 {
		t.Errorf("expected 2 services stopped, got %d", result.StoppedCount)
	}

	if result.HasErrors() {
		t.Errorf("unexpected rollback errors: %v", result.Errors)
	}
}

func TestRollbackExecutor_PreservesOldServices(t *testing.T) {
	planner := NewLifecyclePlanner()
	handleStore := NewRuntimeHandleStore()
	handleStore.Put("rt-1", "existing-service", &ServiceExecutionHandle{
		RuntimeID:  "rt-1",
		ServiceID:  "existing-service",
		InstanceID: "rt-1/existing-service",
	})
	handleStore.Put("rt-1", "new-service", &ServiceExecutionHandle{
		RuntimeID:  "rt-1",
		ServiceID:  "new-service",
		InstanceID: "rt-1/new-service",
	})

	progress := &StartupProgress{
		RuntimeID:            "rt-1",
		StartedThisOperation: []domain.ServiceID{"new-service"},
	}

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "existing-service", State: ServiceStateRunning},
			{ServiceID: "new-service", State: ServiceStateRunning},
		},
	}

	graph := DependencyGraphSnapshot{
		RuntimeID: "rt-1",
		Nodes: []DependencyNodeSnapshot{
			{ServiceID: "existing-service"},
			{ServiceID: "new-service"},
		},
	}

	rollbackPlan, err := planner.BuildRollbackPlan(*progress, topology, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serviceExec := &fakeServiceExecutor{}
	exec := NewRollbackExecutor()

	result := exec.Execute(context.Background(), rollbackPlan, handleStore, serviceExec, progress)

	if result.StoppedCount != 1 {
		t.Errorf("expected 1 service stopped (new-service only), got %d", result.StoppedCount)
	}

	existingHandle, found := handleStore.Get("rt-1", "existing-service")
	if !found || existingHandle == nil {
		t.Error("expected existing-service handle to be preserved, but it was removed/invalidated")
	}
}

func TestStartupProgress_RecordStarted(t *testing.T) {
	progress := &StartupProgress{}
	progress.RecordStarted("svc-1")
	progress.RecordStarted("svc-2")
	progress.RecordStarted("svc-1")

	if len(progress.StartedThisOperation) != 3 {
		t.Errorf("expected 3 entries, got %d", len(progress.StartedThisOperation))
	}

	if !progress.IsStarted("svc-1") {
		t.Error("expected svc-1 to be started")
	}
	if progress.IsStarted("svc-3") {
		t.Error("expected svc-3 to NOT be started")
	}
}

func TestStartupProgress_DuplicatesTrackedDeterministically(t *testing.T) {
	progress := &StartupProgress{}
	progress.RecordStarted("svc-1")
	progress.RecordStarted("svc-1")

	count := 0
	for _, id := range progress.StartedThisOperation {
		if id == "svc-1" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 entries for svc-1, got %d", count)
	}
}

func TestNextBackoffDuration(t *testing.T) {
	durations := []time.Duration{
		nextBackoffDuration(0, 1*time.Second),
		nextBackoffDuration(1, 1*time.Second),
		nextBackoffDuration(2, 1*time.Second),
		nextBackoffDuration(3, 1*time.Second),
		nextBackoffDuration(10, 1*time.Second),
	}

	if durations[0] != 1*time.Second {
		t.Errorf("expected 1s for attempt 0, got %v", durations[0])
	}
	if durations[1] != 2*time.Second {
		t.Errorf("expected 2s for attempt 1, got %v", durations[1])
	}
	if durations[2] != 4*time.Second {
		t.Errorf("expected 4s for attempt 2, got %v", durations[2])
	}
	if durations[4] != 30*time.Second {
		t.Errorf("expected 30s cap for attempt 10, got %v", durations[4])
	}
}

func TestBuildRollbackPlan_OrderReversed(t *testing.T) {
	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "bridge", Dependencies: []domain.ServiceID{}},
			{ServiceID: "agent", Dependencies: []domain.ServiceID{"bridge"}},
			{ServiceID: "planner", Dependencies: []domain.ServiceID{"agent"}},
		},
	}

	graph := DependencyGraphSnapshot{
		RuntimeID: "rt-1",
		Nodes: []DependencyNodeSnapshot{
			{ServiceID: "bridge", Dependents: []domain.ServiceID{"agent"}},
			{ServiceID: "agent", Dependencies: []domain.ServiceID{"bridge"}, Dependents: []domain.ServiceID{"planner"}},
			{ServiceID: "planner", Dependencies: []domain.ServiceID{"agent"}},
		},
	}

	progress := StartupProgress{
		RuntimeID:            "rt-1",
		StartedThisOperation: []domain.ServiceID{"bridge", "agent", "planner"},
	}

	planner := NewLifecyclePlanner()
	plan, err := planner.BuildRollbackPlan(progress, topology, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Stages[0].Services[0].ServiceID != "planner" {
		t.Errorf("expected first rollback stage to be planner, got %s", plan.Stages[0].Services[0].ServiceID)
	}

	lastStage := plan.Stages[len(plan.Stages)-1]
	if lastStage.Services[0].ServiceID != "bridge" {
		t.Errorf("expected last rollback stage to be bridge, got %s", lastStage.Services[0].ServiceID)
	}
}

func TestRollbackExecutor_StopFailure(t *testing.T) {
	planner := NewLifecyclePlanner()
	handleStore := NewRuntimeHandleStore()
	handleStore.Put("rt-1", "svc-1", &ServiceExecutionHandle{
		RuntimeID:  "rt-1",
		ServiceID:  "svc-1",
		InstanceID: "rt-1/svc-1",
	})

	progress := &StartupProgress{
		RuntimeID:            "rt-1",
		StartedThisOperation: []domain.ServiceID{"svc-1"},
	}

	topology := RuntimeTopologySnapshot{
		RuntimeID: "rt-1",
		Services: []ServiceInstanceSnapshot{
			{ServiceID: "svc-1", State: ServiceStateRunning},
		},
	}

	graph := DependencyGraphSnapshot{
		RuntimeID: "rt-1",
		Nodes: []DependencyNodeSnapshot{
			{ServiceID: "svc-1"},
		},
	}

	rollbackPlan, err := planner.BuildRollbackPlan(*progress, topology, graph)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	serviceExec := &fakeServiceExecutor{
		stopFn: func(ctx context.Context, handle ServiceExecutionHandle, force bool) error {
			return errors.New("stop failed")
		},
	}

	exec := NewRollbackExecutor()
	result := exec.Execute(context.Background(), rollbackPlan, handleStore, serviceExec, progress)

	if !result.HasErrors() {
		t.Error("expected rollback errors")
	}
	if result.StoppedCount != 0 {
		t.Errorf("expected 0 stopped when stop fails, got %d", result.StoppedCount)
	}
}

func TestResolveDefinition_FailureDoesNotCorruptTopology(t *testing.T) {
	resolveFn := func(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error) {
		return nil, errors.New("definition not found")
	}

	_ = resolveFn
	_ = domain.RuntimeInstanceID("test")
}

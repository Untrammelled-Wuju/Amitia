package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type fakeTopologyAccessor struct {
	topology RuntimeTopologySnapshot
}

func (f *fakeTopologyAccessor) GetService(serviceID domain.ServiceID) (*ServiceInstance, error) {
	for i := range f.topology.Services {
		if f.topology.Services[i].ServiceID == serviceID {
			svc := f.topology.Services[i]
			inst, err := NewServiceInstance(svc.ID, svc.RuntimeID, svc.PluginID, svc.ServiceID, svc.Required, svc.ServiceKind, svc.Dependencies, svc.CreatedAt)
			if err != nil {
				return nil, err
			}
			inst.State = svc.State
			return inst, nil
		}
	}
	return nil, &TopologyError{Code: ErrNotFound, Message: "service not found"}
}

func (f *fakeTopologyAccessor) UpdateServiceState(serviceID domain.ServiceID, next ServiceRuntimeState, now time.Time) error {
	return nil
}

func (f *fakeTopologyAccessor) Snapshot() RuntimeTopologySnapshot {
	return f.topology
}

func TestShutdownCoordinator_NilExecutor(t *testing.T) {
	_, err := NewShutdownCoordinator(nil, 4, 30*time.Second)
	if err == nil {
		t.Fatal("expected error for nil executor")
	}
}

func TestShutdownCoordinator_Defaults(t *testing.T) {
	exec := &fakeRuntimeExecutor{}
	coord, err := NewShutdownCoordinator(exec, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if coord == nil {
		t.Fatal("expected coordinator to be created with defaults")
	}
}

func TestShutdownCoordinator_ShutdownEmpty(t *testing.T) {
	exec := &fakeRuntimeExecutor{}
	coord, err := NewShutdownCoordinator(exec, 4, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = coord.ShutdownAll(context.Background(), []domain.RuntimeInstanceID{})
	if err != nil {
		t.Errorf("expected no error for empty runtime list, got %v", err)
	}
}

func TestShutdownCoordinator_ShutdownMultiple(t *testing.T) {
	exec := &fakeRuntimeExecutor{}
	coord, err := NewShutdownCoordinator(exec, 4, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	runtimeIDs := []domain.RuntimeInstanceID{"rt-1", "rt-2", "rt-3"}
	err = coord.ShutdownAll(context.Background(), runtimeIDs)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(exec.stoppedRuntimes) != 3 {
		t.Errorf("expected 3 runtimes stopped, got %d", len(exec.stoppedRuntimes))
	}
}

func TestShutdownCoordinator_StopError(t *testing.T) {
	exec := &fakeRuntimeExecutor{
		stopErr: errors.New("stop failed"),
	}
	coord, err := NewShutdownCoordinator(exec, 4, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	runtimeIDs := []domain.RuntimeInstanceID{"rt-1"}
	err = coord.ShutdownAll(context.Background(), runtimeIDs)
	if err == nil {
		t.Error("expected error when stop fails")
	}
}

func TestShutdownCoordinator_ForceCleanup(t *testing.T) {
	exec := &fakeRuntimeExecutor{}
	coord, err := NewShutdownCoordinator(exec, 4, 30*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	runtimeIDs := []domain.RuntimeInstanceID{"rt-1", "rt-2"}
	err = coord.ForceCleanupAll(context.Background(), runtimeIDs)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(exec.cleanedRuntimes) != 2 {
		t.Errorf("expected 2 runtimes cleaned, got %d", len(exec.cleanedRuntimes))
	}
}

type fakeRuntimeExecutor struct {
	mu               sync.Mutex
	stoppedRuntimes  []domain.RuntimeInstanceID
	cleanedRuntimes  []domain.RuntimeInstanceID
	stopErr          error
	cleanupErr       error
}

func (f *fakeRuntimeExecutor) StartRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	return nil
}

func (f *fakeRuntimeExecutor) StopRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	f.mu.Lock()
	f.stoppedRuntimes = append(f.stoppedRuntimes, runtimeID)
	f.mu.Unlock()
	return nil
}

func (f *fakeRuntimeExecutor) StartServices(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceIDs []domain.ServiceID) error {
	return nil
}

func (f *fakeRuntimeExecutor) StopServices(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceIDs []domain.ServiceID) error {
	return nil
}

func (f *fakeRuntimeExecutor) CleanupRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if f.cleanupErr != nil {
		return f.cleanupErr
	}
	f.mu.Lock()
	f.cleanedRuntimes = append(f.cleanedRuntimes, runtimeID)
	f.mu.Unlock()
	return nil
}

func (f *fakeRuntimeExecutor) SetResolveDefinition(fn DefinitionResolverFunc) {
}

func TestRuntimeHandleStore(t *testing.T) {
	store := NewRuntimeHandleStore()
	store.Put("rt-1", "svc-1", &ServiceExecutionHandle{
		RuntimeID: "rt-1",
		ServiceID: "svc-1",
	})
	store.Put("rt-1", "svc-2", &ServiceExecutionHandle{
		RuntimeID: "rt-1",
		ServiceID: "svc-2",
	})
	store.Put("rt-2", "svc-1", &ServiceExecutionHandle{
		RuntimeID: "rt-2",
		ServiceID: "svc-1",
	})

	if store.Count() != 3 {
		t.Errorf("expected 3 handles, got %d", store.Count())
	}

	handle, found := store.Get("rt-1", "svc-1")
	if !found {
		t.Fatal("expected to find handle for rt-1/svc-1")
	}
	if handle.ServiceID != "svc-1" {
		t.Errorf("expected svc-1, got %s", handle.ServiceID)
	}

	store.Remove("rt-1", "svc-1")
	if store.Count() != 2 {
		t.Errorf("expected 2 handles after remove, got %d", store.Count())
	}

	handles := store.ListByRuntime("rt-1")
	if len(handles) != 1 {
		t.Errorf("expected 1 handle for rt-1, got %d", len(handles))
	}

	all := store.ListAll()
	if len(all) != 2 {
		t.Errorf("expected 2 runtimes in list, got %d", len(all))
	}
}

func TestRuntimeHandleStore_RemoveRuntime(t *testing.T) {
	store := NewRuntimeHandleStore()
	store.Put("rt-1", "svc-1", &ServiceExecutionHandle{RuntimeID: "rt-1", ServiceID: "svc-1"})
	store.Put("rt-1", "svc-2", &ServiceExecutionHandle{RuntimeID: "rt-1", ServiceID: "svc-2"})
	store.Put("rt-2", "svc-1", &ServiceExecutionHandle{RuntimeID: "rt-2", ServiceID: "svc-1"})

	handles := store.RemoveRuntime("rt-1")
	if len(handles) != 2 {
		t.Errorf("expected 2 handles removed, got %d", len(handles))
	}

	if store.Count() != 1 {
		t.Errorf("expected 1 handle remaining, got %d", store.Count())
	}
}

func TestRuntimeHandleStore_Clear(t *testing.T) {
	store := NewRuntimeHandleStore()
	store.Put("rt-1", "svc-1", &ServiceExecutionHandle{RuntimeID: "rt-1", ServiceID: "svc-1"})
	store.Put("rt-2", "svc-1", &ServiceExecutionHandle{RuntimeID: "rt-2", ServiceID: "svc-1"})

	store.Clear()
	if store.Count() != 0 {
		t.Errorf("expected 0 handles after clear, got %d", store.Count())
	}
}

func TestRuntimeExecutionResult(t *testing.T) {
	result := NewRuntimeExecutionResult("rt-1")
	result.AddServiceResult(&ServiceExecutionResult{
		ServiceID: "svc-1",
		Success:   true,
	})
	result.AddServiceResult(&ServiceExecutionResult{
		ServiceID: "svc-2",
		Success:   false,
		Error:     errors.New("failed"),
	})
	result.AddStageResult(NewStageExecutionResult(0))

	failed := result.FailedServices()
	if len(failed) != 1 || failed[0] != "svc-2" {
		t.Errorf("expected [svc-2] as failed, got %v", failed)
	}
}

func TestCanStartService(t *testing.T) {
	if !canStartService(ServiceStateCreated) {
		t.Error("expected created to be startable")
	}
	if !canStartService(ServiceStateStopped) {
		t.Error("expected stopped to be startable")
	}
	if canStartService(ServiceStateRunning) {
		t.Error("expected running to NOT be startable")
	}
	if canStartService(ServiceStateStarting) {
		t.Error("expected starting to NOT be startable")
	}
	if canStartService(ServiceStateStopping) {
		t.Error("expected stopping to NOT be startable")
	}
}

func TestCanStopService(t *testing.T) {
	if !canStopService(ServiceStateRunning) {
		t.Error("expected running to be stoppable")
	}
	if canStopService(ServiceStateCreated) {
		t.Error("expected created to NOT be stoppable")
	}
	if canStopService(ServiceStateStopped) {
		t.Error("expected stopped to NOT be stoppable")
	}
	if canStopService(ServiceStateStarting) {
		t.Error("expected starting to NOT be stoppable")
	}
}

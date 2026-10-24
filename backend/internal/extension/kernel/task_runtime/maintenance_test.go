package task_runtime

import (
	"context"
	"errors"
	"testing"
)

type maintenanceStore struct {
	TaskStore
	active int
	runs   []*TaskRun
	err    error
}

func (s *maintenanceStore) CountActive(context.Context) (int, error) { return s.active, s.err }
func (s *maintenanceStore) CountActiveByExtension(context.Context, string) (int, error) {
	return 0, s.err
}
func (s *maintenanceStore) CountActiveByDefinition(context.Context, string) (int, error) {
	return 0, s.err
}
func (s *maintenanceStore) ListTaskRuns(context.Context, ListTasksFilter) ([]*TaskRun, error) {
	return s.runs, s.err
}

func TestTaskMaintenanceDynamicLimitControlsQueueAndPhysicalProcesses(t *testing.T) {
	store := &maintenanceStore{active: 1}
	service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	service.ApplyMaxConcurrent(1)
	if ok, _, err := service.limiter.CanStart(t.Context(), &TaskRun{}); err != nil || ok {
		t.Fatal("queue ignored new global limit")
	}
	first, err := service.reserveTaskProcess(t.Context(), &TaskDefinition{TaskID: "one", ExtensionID: "one"})
	if err != nil {
		t.Fatal(err)
	}
	defer first()
	if _, err := service.reserveTaskProcess(t.Context(), &TaskDefinition{TaskID: "two", ExtensionID: "two"}); !errors.Is(err, ErrTaskProcessCapacity) {
		t.Fatal("physical process ignored limit")
	}
	service.ApplyMaxConcurrent(2)
	if ok, _, err := service.limiter.CanStart(t.Context(), &TaskRun{}); err != nil || !ok {
		t.Fatal("increased queue limit was not applied")
	}
	second, err := service.reserveTaskProcess(t.Context(), &TaskDefinition{TaskID: "two", ExtensionID: "two"})
	if err != nil {
		t.Fatal(err)
	}
	defer second()
	service.ApplyMaxConcurrent(1)
	if service.processBudget.total != 2 {
		t.Fatal("lowering limit killed active reservations")
	}
}

func TestTaskMaintenanceStatusUsesRuntimeAndPropagatesErrors(t *testing.T) {
	store := &maintenanceStore{active: 1, runs: []*TaskRun{{TaskRunID: "real", Status: RunStatusRunning}}}
	service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	status, err := service.MaintenanceStatus(t.Context())
	if err != nil || !status.Running || status.ActiveCount != 1 || len(status.Tasks) != 1 || status.Tasks[0].ID != "real" {
		t.Fatalf("wrong runtime status: %+v %v", status, err)
	}
	store.active = 0
	store.runs[0].Status = RunStatusSucceeded
	status, err = service.MaintenanceStatus(t.Context())
	if err != nil || status.Running {
		t.Fatal("completed task reported running")
	}
	store.err = errors.New("storage unavailable")
	if _, err = service.MaintenanceStatus(t.Context()); err == nil {
		t.Fatal("storage failure reported idle")
	}
}

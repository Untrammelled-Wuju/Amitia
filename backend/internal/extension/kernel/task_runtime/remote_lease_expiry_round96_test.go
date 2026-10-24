package task_runtime

import (
	"testing"
	"time"
)

func TestRound96ExpiredLeaseCannotBeRenewedOrSubmitProgress(t *testing.T) {
	for _, tc := range []struct {
		name      string
		isMissing bool
	}{
		{name: "expired"},
		{name: "missing", isMissing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			past := time.Now().UTC().Add(-time.Minute)
			run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, ExecutionAttemptID: "attempt", Generation: 2, Revision: 3, Status: RunStatusRunning, LeaseID: "lease", LeaseExpiresAt: &past}
			if tc.isMissing {
				run.LeaseExpiresAt = nil
			}
			store := &pauseStore{run: run, def: &TaskDefinition{TaskID: "task"}}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			if err := service.HeartbeatRemoteTask(t.Context(), "run", "attempt", "lease", time.Minute); err == nil {
				t.Fatal("expired remote worker revived via heartbeat")
			}
			if err := service.UpdateLease(t.Context(), "run", "lease", time.Minute); err == nil {
				t.Fatal("expired remote worker revived via generic lease update")
			}
			if err := service.HandleProgress(t.Context(), "run", "attempt", "lease", 1, nil, nil, nil, "stage", "late"); err == nil {
				t.Fatal("expired remote worker progress was accepted")
			}
			if err := service.HandleCheckpoint(t.Context(), "run", "attempt", "lease", "checkpoint", 1, nil, ""); err == nil {
				t.Fatal("expired remote worker checkpoint was accepted")
			}
			if store.run.Revision != 3 || store.run.Status != RunStatusRunning {
				t.Fatalf("rejected remote lease unexpectedly changed task: %+v", store.run)
			}
		})
	}
}

func TestRound96EmptyLeaseCannotCreateRemoteAuthority(t *testing.T) {
	future := time.Now().UTC().Add(time.Minute)
	run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, ExecutionAttemptID: "attempt", Generation: 2, Revision: 3, Status: RunStatusRunning, LeaseExpiresAt: &future}
	store := &pauseStore{run: run, def: &TaskDefinition{TaskID: "task"}}
	service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	if err := service.HeartbeatRemoteTask(t.Context(), "run", "attempt", "", time.Minute); err == nil {
		t.Fatal("empty lease ID allowed remote heartbeat")
	}
	if err := service.UpdateLease(t.Context(), "run", "", time.Minute); err == nil {
		t.Fatal("empty lease ID allowed generic update")
	}
	if err := service.HandleProgress(t.Context(), "run", "attempt", "", 1, nil, nil, nil, "stage", "late"); err == nil {
		t.Fatal("empty lease ID allowed progress")
	}
}

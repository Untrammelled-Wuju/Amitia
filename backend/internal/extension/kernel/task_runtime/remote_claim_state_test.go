package task_runtime

import (
	"testing"
	"time"
)

func TestRemoteClaimCannotReactivatePausedOrUnknownExecution(t *testing.T) {
	for _, status := range []TaskRunStatus{RunStatusStarting, RunStatusRunning, RunStatusPaused, RunStatusPausing, RunStatusRecoveryRequired, RunStatusCancelling, RunStatusQueued} {
		t.Run(string(status), func(t *testing.T) {
			expires := time.Now().UTC().Add(time.Minute)
			run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, ExecutionAttemptID: "attempt", Generation: 2, Revision: 3, Status: status, LeaseID: "lease", LeaseExpiresAt: &expires}
			store := &pauseStore{run: run, def: &TaskDefinition{TaskID: "task"}}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			err := service.HandleRemoteClaim(t.Context(), "run", "attempt", "lease", expires)
			allowed := status == RunStatusStarting || status == RunStatusRunning
			if allowed && (err != nil || store.run.Status != RunStatusRunning) || !allowed && (err == nil || store.run.Status != status) {
				t.Fatalf("claim changed an invalid execution state: %v %+v", err, store.run)
			}
		})
	}
}

func TestRemoteHeartbeatKeepsPauseCheckpointLeaseWithoutUnpausing(t *testing.T) {
	for _, status := range []TaskRunStatus{RunStatusRunning, RunStatusCheckpointing, RunStatusPausing, RunStatusPaused, RunStatusRecoveryRequired} {
		t.Run(string(status), func(t *testing.T) {
			expires := time.Now().UTC().Add(time.Minute)
			run := &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, ExecutionAttemptID: "attempt", Generation: 2, Revision: 3, Status: status, LeaseID: "lease", LeaseExpiresAt: &expires}
			store := &pauseStore{run: run, def: &TaskDefinition{TaskID: "task"}}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			err := service.HeartbeatRemoteTask(t.Context(), "run", "attempt", "lease", 2*time.Minute)
			allowed := status == RunStatusRunning || status == RunStatusCheckpointing || status == RunStatusPausing
			if allowed && (err != nil || store.run.Status != status || !store.run.LeaseExpiresAt.After(expires)) || !allowed && (err == nil || store.run.Status != status) {
				t.Fatalf("heartbeat changed the pause state or missed its lease: %v %+v", err, store.run)
			}
		})
	}
}

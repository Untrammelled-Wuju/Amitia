package task_runtime

import (
	"context"
	"testing"
	"time"
)

type delayedRemoteLeaseStore struct {
	*pauseStore
	delay time.Duration
}

func (s *delayedRemoteLeaseStore) RenewRemoteLeaseCAS(ctx context.Context, next *TaskRun, status TaskRunStatus, generation, revision int64) (bool, error) {
	time.Sleep(s.delay)
	return s.pauseStore.RenewRemoteLeaseCAS(ctx, next, status, generation, revision)
}

func TestRound97LeaseExpiryBetweenReadAndWriteRejectsBothRenewalPaths(t *testing.T) {
	for _, name := range []string{"heartbeat", "generic"} {
		t.Run(name, func(t *testing.T) {
			expires := time.Now().UTC().Add(100 * time.Millisecond)
			run := &TaskRun{
				TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice,
				ExecutionAttemptID: "attempt", Generation: 2, Revision: 3, Status: RunStatusRunning,
				LeaseID: "lease", LeaseExpiresAt: &expires,
			}
			store := &delayedRemoteLeaseStore{
				pauseStore: &pauseStore{run: run, def: &TaskDefinition{TaskID: "task"}},
				delay:      175 * time.Millisecond,
			}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			var err error
			if name == "heartbeat" {
				err = service.HeartbeatRemoteTask(t.Context(), "run", "attempt", "lease", time.Minute)
			} else {
				err = service.UpdateLease(t.Context(), "run", "lease", time.Minute)
			}
			if err == nil {
				t.Fatal("a lease that expired after the initial read was renewed")
			}
			actual, _ := store.GetTaskRun(t.Context(), "run")
			if actual.Revision != 3 || actual.LeaseExpiresAt == nil || !actual.LeaseExpiresAt.Equal(expires) {
				t.Fatalf("rejected renewal changed the persisted task: %+v", actual)
			}
		})
	}
}

package task_runtime

import (
	"context"
	"testing"
	"time"
)

type round107StoppedVerifier struct {
	UnavailableRemoteTaskExecutor
	store    *pauseStore
	stopped  bool
	mutation string
	checks   int
}

func (v *round107StoppedVerifier) ConfirmPauseStopped(_ context.Context, _ *TaskRun) bool {
	v.checks++
	if v.mutation != "" {
		v.store.mu.Lock()
		switch v.mutation {
		case "revision":
			v.store.run.Revision++
		case "attempt":
			v.store.run.ExecutionAttemptID = "replacement-attempt"
		case "lease":
			v.store.run.LeaseID = "replacement-lease"
		case "cancel":
			v.store.run.Status = RunStatusCancelling
		}
		v.store.mu.Unlock()
	}
	return v.stopped
}

func TestRound107PausedRemoteIdempotencyNeedsStopReceipt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stopped  bool
		mutation string
		code     TaskErrorCode
	}{
		{name: "stopped", stopped: true},
		{name: "worker-still-running", code: ErrTaskPauseInProgress},
		{name: "revision-changed", stopped: true, mutation: "revision", code: ErrTaskResumeStaleGeneration},
		{name: "attempt-replaced", stopped: true, mutation: "attempt", code: ErrTaskResumeStaleGeneration},
		{name: "lease-replaced", stopped: true, mutation: "lease", code: ErrTaskResumeStaleGeneration},
		{name: "cancel-concurrent", stopped: true, mutation: "cancel", code: ErrTaskResumeStaleGeneration},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			store := &pauseStore{run: &TaskRun{
				TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice,
				Status: RunStatusPaused, PausedAt: &now, Generation: 4, Revision: 7,
				ExecutionAttemptID: "attempt", LeaseID: "lease",
			}, def: &TaskDefinition{TaskID: "task", Checkpoint: true}}
			svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			verifier := &round107StoppedVerifier{store: store, stopped: tc.stopped, mutation: tc.mutation}
			svc.SetRemoteExecutor(verifier)
			err := svc.PauseTask(context.Background(), PauseTaskRequest{TaskRunID: "run", Generation: 4})
			if tc.code == "" && err != nil || tc.code != "" && !IsTaskErrorCode(err, tc.code) {
				t.Fatalf("expected %q, got %v", tc.code, err)
			}
			if verifier.checks != 1 {
				t.Fatalf("expected one stop verification, got %d", verifier.checks)
			}
		})
	}
}

func TestRound107RemotePausedNoVerifierIsNotConfirmed(t *testing.T) {
	now := time.Now().UTC()
	store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, Status: RunStatusPaused, PausedAt: &now, Generation: 2}, def: &TaskDefinition{TaskID: "task", Checkpoint: true}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	if err := svc.PauseTask(context.Background(), PauseTaskRequest{TaskRunID: "run", Generation: 2}); !IsTaskErrorCode(err, ErrTaskPauseInProgress) {
		t.Fatalf("missing stop verifier accepted: %v", err)
	}
}

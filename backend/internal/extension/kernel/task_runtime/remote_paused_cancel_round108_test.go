package task_runtime

import (
	"context"
	"testing"
	"time"
)

func TestRound108RemotePausedCancellationRequiresOriginalWorkerStop(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stopped  bool
		mutation string
		pauseAt  bool
		wantCode TaskErrorCode
	}{
		{name: "confirmed-stopped", stopped: true, pauseAt: true},
		{name: "worker-active", stopped: false, pauseAt: true, wantCode: ErrTaskPauseInProgress},
		{name: "missing-paused-proof", stopped: true, wantCode: ErrTaskPauseInProgress},
		{name: "revision-race", stopped: true, pauseAt: true, mutation: "revision", wantCode: ErrTaskResumeStaleGeneration},
		{name: "attempt-replaced", stopped: true, pauseAt: true, mutation: "attempt", wantCode: ErrTaskResumeStaleGeneration},
		{name: "lease-replaced", stopped: true, pauseAt: true, mutation: "lease", wantCode: ErrTaskResumeStaleGeneration},
		{name: "concurrent-cancel", stopped: true, pauseAt: true, mutation: "cancel", wantCode: ErrTaskResumeStaleGeneration},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			run := &TaskRun{
				TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice,
				Status: RunStatusPaused, Generation: 3, Revision: 7,
				ExecutionAttemptID: "original-attempt", LeaseID: "original-lease",
			}
			if tc.pauseAt {
				run.PausedAt = &now
			}
			store := &pauseStore{run: run}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			verifier := &round107StoppedVerifier{store: store, stopped: tc.stopped, mutation: tc.mutation}
			service.SetRemoteExecutor(verifier)
			err := service.Cancel(context.Background(), "run", "user")
			if tc.wantCode != "" {
				if !IsTaskErrorCode(err, tc.wantCode) {
					t.Fatalf("expected %s, got %v", tc.wantCode, err)
				}
				if store.run.CancelRequestedAt != nil || store.run.FinishedAt != nil || store.run.Status == RunStatusCancelled && tc.mutation != "cancel" {
					t.Fatalf("unconfirmed cancellation changed run: %+v", store.run)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if verifier.checks != 1 || store.run.Status != RunStatusCancelled || store.run.CancelRequestedAt == nil || store.run.FinishedAt == nil || store.run.Generation != 3 || store.run.ExecutionAttemptID != "original-attempt" {
				t.Fatalf("paused cancellation did not confirm original stop: checks=%d run=%+v", verifier.checks, store.run)
			}
		})
	}
}

func TestRound108RemotePausedCancelAndWaitHonorsStopReceipt(t *testing.T) {
	now := time.Now().UTC()
	store := &pauseStore{run: &TaskRun{
		TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice,
		Status: RunStatusPaused, PausedAt: &now, Generation: 4, Revision: 9,
		ExecutionAttemptID: "attempt", LeaseID: "lease",
	}}
	service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	verifier := &round107StoppedVerifier{store: store, stopped: false}
	service.SetRemoteExecutor(verifier)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.CancelAndWait(ctx, "run", "user"); !IsTaskErrorCode(err, ErrTaskPauseInProgress) {
		t.Fatalf("unconfirmed stop was reported cancelled: %v", err)
	}
	if store.run.Status != RunStatusPaused {
		t.Fatalf("cancellation without worker receipt altered status: %s", store.run.Status)
	}
	verifier.stopped = true
	if err := service.CancelAndWait(ctx, "run", "user"); err != nil {
		t.Fatalf("verified paused cancellation failed: %v", err)
	}
	if store.run.Status != RunStatusCancelled {
		t.Fatalf("verified paused cancellation not persisted: %s", store.run.Status)
	}
}

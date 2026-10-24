package task_runtime

import (
	"context"
	"testing"
	"time"
)

type round106CancelExecutor struct {
	UnavailableRemoteTaskExecutor
	store *pauseStore
	mode  string
	calls int
}

func (e *round106CancelExecutor) CancelAndWait(_ context.Context, run *TaskRun, _ time.Duration) error {
	e.calls++
	e.store.mu.Lock()
	defer e.store.mu.Unlock()
	if e.mode == "no-ack" {
		return nil
	}
	e.store.run.Status = RunStatusCancelled
	e.store.run.Revision++
	if e.mode == "new-attempt" {
		e.store.run.ExecutionAttemptID = "replacement-attempt"
	}
	if e.mode == "new-generation" {
		e.store.run.Generation++
	}
	return nil
}

func TestRound106RemoteCancellationOfInFlightStatuses(t *testing.T) {
	for _, status := range []TaskRunStatus{RunStatusRunning, RunStatusPausing, RunStatusCheckpointing} {
		t.Run(string(status), func(t *testing.T) {
			store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, Status: status, ExecutionAttemptID: "attempt", Generation: 3, Revision: 5}}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			executor := &round106CancelExecutor{store: store}
			service.SetRemoteExecutor(executor)
			if err := service.Cancel(t.Context(), "run", "user"); err != nil {
				t.Fatalf("in-flight cancellation rejected: %v", err)
			}
			if executor.calls != 1 || store.run.Status != RunStatusCancelled || store.run.CancelRequestedAt == nil {
				t.Fatalf("cancellation was not confirmed: calls=%d run=%+v", executor.calls, store.run)
			}
		})
	}
}

func TestRound106RemoteCancellationRequiresExactStoppedAttempt(t *testing.T) {
	for _, mode := range []string{"new-attempt", "new-generation", "no-ack"} {
		t.Run(mode, func(t *testing.T) {
			store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, Status: RunStatusPausing, ExecutionAttemptID: "attempt", Generation: 3, Revision: 5}}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			service.SetRemoteExecutor(&round106CancelExecutor{store: store, mode: mode})
			if err := service.Cancel(t.Context(), "run", "user"); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
				t.Fatalf("unconfirmed original attempt was accepted: %v", err)
			}
		})
	}
}

func TestRound106RemoteCancellationDoesNotReanimateUnknownState(t *testing.T) {
	for _, status := range []TaskRunStatus{RunStatusPaused, RunStatusRecoveryRequired, RunStatusSucceeded, RunStatusFailed} {
		t.Run(string(status), func(t *testing.T) {
			store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", ExecutionPlacement: TaskExecutionPlacementDevice, Status: status, ExecutionAttemptID: "attempt", Generation: 3, Revision: 5}}
			service := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			executor := &round106CancelExecutor{store: store}
			service.SetRemoteExecutor(executor)
			if err := service.Cancel(t.Context(), "run", "user"); err == nil || executor.calls != 0 || store.run.Status != status {
				t.Fatalf("invalid state was cancelled: %v %+v", err, store.run)
			}
		})
	}
}

func TestRound106ControlStateTransitionsMatchExecutionPaths(t *testing.T) {
	for _, transition := range [][2]TaskRunStatus{
		{RunStatusRunning, RunStatusPausing},
		{RunStatusCheckpointing, RunStatusPausing},
		{RunStatusPausing, RunStatusPaused},
		{RunStatusPausing, RunStatusCancelling},
		{RunStatusCheckpointing, RunStatusCancelling},
		{RunStatusPaused, RunStatusQueued},
		{RunStatusPaused, RunStatusCancelled},
		{RunStatusStarting, RunStatusCancelling},
	} {
		if !IsValidTransition(transition[0], transition[1]) {
			t.Fatalf("active control transition refused: %s -> %s", transition[0], transition[1])
		}
	}
	for _, transition := range [][2]TaskRunStatus{
		{RunStatusPaused, RunStatusSucceeded},
		{RunStatusCancelled, RunStatusRunning},
		{RunStatusPausing, RunStatusSucceeded},
	} {
		if IsValidTransition(transition[0], transition[1]) {
			t.Fatalf("unsupported control transition admitted: %s -> %s", transition[0], transition[1])
		}
	}
}

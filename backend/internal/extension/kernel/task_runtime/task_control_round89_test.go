package task_runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func TestRound89CancelAndWaitNeverConfirmsAnotherTerminalOutcome(t *testing.T) {
	for _, terminal := range []TaskRunStatus{RunStatusSucceeded, RunStatusFailed, RunStatusTimedOut, RunStatusManualIntervention} {
		t.Run(string(terminal), func(t *testing.T) {
			store := &pauseStore{run: &TaskRun{TaskRunID: "run", Status: terminal, Generation: 2, Revision: 1}}
			svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
			defer cancel()
			if err := svc.CancelAndWait(ctx, "run", "request"); !IsTaskErrorCode(err, ErrTaskNotCancelable) {
				t.Fatalf("non-cancelled terminal state was confirmed: %v", err)
			}
		})
	}
}

func TestRound89CancelAndWaitConcurrentFailureIsNotCancellation(t *testing.T) {
	for _, terminal := range []TaskRunStatus{RunStatusSucceeded, RunStatusFailed} {
		t.Run(string(terminal), func(t *testing.T) {
			store := &pauseStore{run: &TaskRun{TaskRunID: "run", Status: RunStatusRunning, Generation: 2, Revision: 1, ExecutionAttemptID: "attempt"}}
			svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			go func() {
				for {
					store.mu.Lock()
					if store.run.Status == RunStatusCancelling {
						store.run.Status = terminal
						store.mu.Unlock()
						return
					}
					store.mu.Unlock()
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Millisecond):
					}
				}
			}()
			if err := svc.CancelAndWait(ctx, "run", "request"); !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) {
				t.Fatalf("non-cancelled concurrent outcome was reported as stop: %v", err)
			}
		})
	}
}

func TestRound89CancelAndWaitConfirmedQueuedCancellation(t *testing.T) {
	store := &pauseStore{run: &TaskRun{TaskRunID: "run", Status: RunStatusQueued, Generation: 1, Revision: 1}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := svc.CancelAndWait(ctx, "run", "request"); err != nil {
		t.Fatalf("queued task cancellation not confirmed: %v", err)
	}
	if store.run.Status != RunStatusCancelled {
		t.Fatalf("unexpected status: %s", store.run.Status)
	}
}

func TestRound89CancelRejectsHistoricalReadAuthority(t *testing.T) {
	svc, _, authority, run, _ := taskAuthorityFixture(t)
	run.Status, run.Generation, run.Revision = RunStatusQueued, 1, 1
	run.DefinitionFingerprint = strings.Repeat("a", 64)
	store := &pauseStore{run: CloneTaskRun(run)}
	svc.store = store
	ctx, err := coordination.WithTaskReadAuthority(coordination.WithScope(t.Context(), authority), authority, coordination.TaskReadProof{Scope: authority, TaskRunID: run.TaskRunID, DefinitionFingerprint: run.DefinitionFingerprint})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Cancel(ctx, run.TaskRunID, "request"); !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("historical read authority cancelled a task: %v", err)
	}
	if store.run.Status != RunStatusQueued {
		t.Fatalf("historical read changed task: %s", store.run.Status)
	}
}

func TestRound89RetryRejectsExpiredDeadlineBeforeQueue(t *testing.T) {
	old := time.Now().UTC().Add(-time.Minute)
	store := &pauseStore{run: &TaskRun{TaskRunID: "run", TaskDefinitionID: "task", Status: RunStatusFailed, Attempt: 1, MaxAttempts: 3, DeadlineAt: &old, Generation: 1, Revision: 1}, def: &TaskDefinition{TaskID: "task", Idempotent: true}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	if next, err := svc.Retry(context.Background(), "run"); next != nil || !IsTaskErrorCode(err, ErrTaskNotRetryable) {
		t.Fatalf("expired retry created new task: %+v %v", next, err)
	}
	if store.queued != 0 || store.run.Status != RunStatusFailed {
		t.Fatalf("expired retry mutated queue or prior run: %+v queued=%d", store.run, store.queued)
	}
}

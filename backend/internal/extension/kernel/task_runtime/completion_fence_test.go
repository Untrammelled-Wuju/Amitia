package task_runtime

import (
	"context"
	"encoding/json"
	"testing"
)

type completionFenceStore struct {
	TaskStore
	run      *TaskRun
	reject   bool
	casCalls int
	results  int
}

func (s *completionFenceStore) GetTaskRun(context.Context, string) (*TaskRun, error) {
	return CloneTaskRun(s.run), nil
}
func (s *completionFenceStore) WithinTaskTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (s *completionFenceStore) UpdateTaskRunCAS(_ context.Context, next *TaskRun, status TaskRunStatus, generation, revision int64) (bool, error) {
	s.casCalls++
	if s.reject || s.run.Status != status || s.run.Generation != generation || s.run.Revision != revision {
		return false, nil
	}
	s.run = CloneTaskRun(next)
	return true, nil
}
func (s *completionFenceStore) PutResult(context.Context, *TaskRunResult) error {
	s.results++
	return nil
}
func (s *completionFenceStore) RemoveFromQueue(context.Context, string) error { return nil }

func TestTaskCompletionCASRejectsConcurrentCancellationBeforeResultWrite(t *testing.T) {
	svc, _, _, run, _ := taskAuthorityFixture(t)
	run.Status = RunStatusRunning
	store := &completionFenceStore{run: CloneTaskRun(run), reject: true}
	svc.store = store
	svc.handleFinished(t.Context(), run, "succeeded", json.RawMessage(`{"private":"result"}`), "", "", "")
	if store.casCalls != 1 || store.results != 0 || store.run.Status != RunStatusRunning {
		t.Fatalf("unconfirmed result stored: %+v", store)
	}
}

func TestTaskCompletionAfterCancelCannotReportSuccessOrScheduleCrashRecovery(t *testing.T) {
	svc, _, _, run, def := taskAuthorityFixture(t)
	run.Status = RunStatusRunning
	current := CloneTaskRun(run)
	current.Status = RunStatusCancelling
	store := &completionFenceStore{run: current}
	svc.store = store
	svc.handleCrash(t.Context(), run, def, 1)
	if store.run.Status != RunStatusCancelled || store.results != 0 {
		t.Fatalf("cancelled task entered recovery: %s", store.run.Status)
	}
	svc.handleFinished(t.Context(), run, "succeeded", json.RawMessage(`{"private":"late"}`), "", "", "")
	if store.run.Status != RunStatusCancelled || store.results != 0 || store.casCalls != 1 {
		t.Fatal("late success replaced cancellation")
	}
}

func TestTaskCompletionRejectsPausedOrReplacedExecution(t *testing.T) {
	for _, scenario := range []string{"paused", "replaced", "terminal"} {
		t.Run(scenario, func(t *testing.T) {
			svc, _, _, run, _ := taskAuthorityFixture(t)
			run.Status = RunStatusRunning
			current := CloneTaskRun(run)
			switch scenario {
			case "paused":
				current.Status = RunStatusPaused
			case "replaced":
				current.Generation++
			case "terminal":
				current.Status = RunStatusSucceeded
			}
			store := &completionFenceStore{run: current}
			svc.store = store
			svc.handleFinished(t.Context(), run, "succeeded", json.RawMessage(`{"private":"late"}`), "", "", "")
			if store.casCalls != 0 || store.results != 0 {
				t.Fatal("old execution wrote a result")
			}
		})
	}
}

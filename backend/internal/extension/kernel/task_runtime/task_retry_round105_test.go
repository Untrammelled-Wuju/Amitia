package task_runtime

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type round105RetryStore struct {
	TaskStore
	mu       sync.Mutex
	tx       sync.Mutex
	runs     map[string]*TaskRun
	queued   int
	creation int
}

func (s *round105RetryStore) GetTaskRun(_ context.Context, id string) (*TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if run := s.runs[id]; run != nil {
		return CloneTaskRun(run), nil
	}
	return nil, NewTaskError(ErrTaskNotFound, "missing")
}

func (s *round105RetryStore) GetTaskDefinition(context.Context, string) (*TaskDefinition, error) {
	return &TaskDefinition{TaskID: "definition", ExtensionID: "extension", ModuleID: "module", Idempotent: true}, nil
}

func (s *round105RetryStore) WithinTaskTx(ctx context.Context, operation func(context.Context) error) error {
	s.tx.Lock()
	defer s.tx.Unlock()
	return operation(ctx)
}

func (s *round105RetryStore) CreateRetryTaskRun(_ context.Context, run *TaskRun) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runs[run.TaskRunID] != nil {
		return false, nil
	}
	s.runs[run.TaskRunID] = CloneTaskRun(run)
	s.creation++
	return true, nil
}

func (s *round105RetryStore) EnqueueTask(context.Context, *TaskQueueEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queued++
	return nil
}

func TestRound105RetryIsIdempotentAcrossConcurrentControls(t *testing.T) {
	origin := &TaskRun{TaskRunID: "tr-original", OperationID: "op", TaskDefinitionID: "definition", ExtensionID: "extension", ModuleID: "module", Status: RunStatusFailed, Generation: 3, Revision: 2, Attempt: 1, MaxAttempts: 3, InputHash: "hash"}
	store := &round105RetryStore{runs: map[string]*TaskRun{origin.TaskRunID: CloneTaskRun(origin)}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	atomic.StoreInt32(&svc.dispatching, 1)
	const workers = 32
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan *TaskRun, workers)
	errors := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			next, err := svc.Retry(ctx, origin.TaskRunID)
			if err != nil {
				errors <- err
			} else {
				results <- next
			}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		t.Errorf("concurrent retry failed: %v", err)
	}
	seen := 0
	for next := range results {
		seen++
		if next.TaskRunID != retryTaskRunID(origin.TaskRunID) || next.Attempt != 2 {
			t.Fatalf("split retry identity: %+v", next)
		}
	}
	if seen != workers || store.creation != 1 || store.queued != 1 || len(store.runs) != 2 {
		t.Fatalf("duplicate retry execution: completed=%d created=%d queued=%d runs=%d", seen, store.creation, store.queued, len(store.runs))
	}
	store.mu.Lock()
	completed := store.runs[retryTaskRunID(origin.TaskRunID)]
	completed.Status, completed.Revision, completed.Generation = RunStatusSucceeded, 3, 4
	store.mu.Unlock()
	if replay, err := svc.Retry(context.Background(), origin.TaskRunID); err != nil || replay.TaskRunID != completed.TaskRunID || replay.Status != RunStatusSucceeded {
		t.Fatalf("retry of finished successor created another execution: %+v %v", replay, err)
	}
	if store.creation != 1 || store.queued != 1 {
		t.Fatal("finished successor was requeued")
	}
}

func TestRound105RetryRejectsCollisionWithForeignScope(t *testing.T) {
	origin := &TaskRun{TaskRunID: "tr-original", TaskDefinitionID: "definition", ExtensionID: "extension", ModuleID: "module", Status: RunStatusFailed, Attempt: 1, MaxAttempts: 3, Generation: 1}
	forged := CloneTaskRun(origin)
	forged.TaskRunID = retryTaskRunID(origin.TaskRunID)
	forged.Generation, forged.Attempt, forged.ScopeSnapshotID = 2, 2, "other-owner"
	store := &round105RetryStore{runs: map[string]*TaskRun{origin.TaskRunID: origin, forged.TaskRunID: forged}}
	svc := NewTaskRuntimeService(store, DefaultTaskRuntimeConfig())
	if next, err := svc.Retry(context.Background(), origin.TaskRunID); next != nil || !IsTaskErrorCode(err, ErrTaskScopeDenied) {
		t.Fatalf("foreign successor reused: %+v %v", next, err)
	}
	if store.queued != 0 || store.creation != 0 {
		t.Fatal("forged successor was dispatched")
	}
}

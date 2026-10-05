package task_runtime

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

type remoteLifetimeStore struct {
	TaskStore
	mu   sync.Mutex
	run  *TaskRun
	read chan struct{}
}

func (s *remoteLifetimeStore) GetTaskRun(context.Context, string) (*TaskRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	select {
	case s.read <- struct{}{}:
	default:
	}
	return CloneTaskRun(s.run), nil
}

type remoteLifetimeExecutor struct {
	UnavailableRemoteTaskExecutor
	cancelled chan struct{}
}

func (e remoteLifetimeExecutor) Cancel(context.Context, *TaskRun) error {
	select {
	case e.cancelled <- struct{}{}:
	default:
	}
	return nil
}

func TestOwnedRemoteAuthorityRemainsUntilConfirmedTerminalState(t *testing.T) {
	svc, _, authority, run, _ := taskAuthorityFixture(t)
	run.Status = RunStatusRunning
	store := &remoteLifetimeStore{run: CloneTaskRun(run), read: make(chan struct{}, 1)}
	svc.store = store
	executor := remoteLifetimeExecutor{cancelled: make(chan struct{}, 1)}
	ctx, cancel := context.WithTimeout(coordination.WithScope(t.Context(), authority), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svc.waitOwnedRemoteExecution(ctx, run, executor) }()
	select {
	case <-store.read:
	case <-ctx.Done():
		t.Fatal("remote task not observed")
	}
	select {
	case err := <-done:
		t.Fatalf("running task released authority: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	store.mu.Lock()
	store.run.Status = RunStatusSucceeded
	store.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("confirmed task did not release authority")
	}
	select {
	case <-executor.cancelled:
		t.Fatal("confirmed task was cancelled")
	default:
	}
}

func TestOwnedRemoteAuthorityCancellationStopsRemoteBeforeRelease(t *testing.T) {
	svc, _, authority, run, _ := taskAuthorityFixture(t)
	run.Status = RunStatusRunning
	store := &remoteLifetimeStore{run: CloneTaskRun(run), read: make(chan struct{}, 1)}
	svc.store = store
	executor := remoteLifetimeExecutor{cancelled: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancelCause(coordination.WithScope(t.Context(), authority))
	defer cancel(nil)
	done := make(chan error, 1)
	go func() { done <- svc.waitOwnedRemoteExecution(ctx, run, executor) }()
	select {
	case <-store.read:
	case <-time.After(time.Second):
		t.Fatal("remote task not observed")
	}
	cancel(coordination.ErrScopeExpired)
	select {
	case err := <-done:
		if !errors.Is(err, coordination.ErrScopeExpired) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("revoked authority remained active")
	}
	select {
	case <-executor.cancelled:
	default:
		t.Fatal("remote execution was not cancelled before release")
	}
}

func TestOwnedRemoteAuthorityRejectsReplacedExecutionGeneration(t *testing.T) {
	svc, _, authority, run, _ := taskAuthorityFixture(t)
	run.Status = RunStatusRunning
	other := CloneTaskRun(run)
	other.Generation++
	svc.store = &remoteLifetimeStore{run: other, read: make(chan struct{}, 1)}
	executor := remoteLifetimeExecutor{cancelled: make(chan struct{}, 1)}
	if err := svc.waitOwnedRemoteExecution(coordination.WithScope(t.Context(), authority), run, executor); err == nil {
		t.Fatal("replacement execution accepted")
	}
	select {
	case <-executor.cancelled:
	default:
		t.Fatal("old execution was not cancelled")
	}
}

func TestOwnedRemoteAuthorityDeadlineStopsUnfinishedExecution(t *testing.T) {
	svc, _, authority, run, _ := taskAuthorityFixture(t)
	run.Status = RunStatusRunning
	deadline := time.Now().Add(40 * time.Millisecond)
	run.DeadlineAt = &deadline
	svc.store = &remoteLifetimeStore{run: CloneTaskRun(run), read: make(chan struct{}, 1)}
	executor := remoteLifetimeExecutor{cancelled: make(chan struct{}, 1)}
	if err := svc.waitOwnedRemoteExecution(coordination.WithScope(t.Context(), authority), run, executor); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unfinished deadline accepted: %v", err)
	}
	select {
	case <-executor.cancelled:
	default:
		t.Fatal("expired execution was not cancelled")
	}
}

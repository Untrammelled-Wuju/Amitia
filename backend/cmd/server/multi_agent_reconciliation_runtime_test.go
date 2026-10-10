package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestMultiAgentReconciliationRunsUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	called := make(chan struct{}, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runMultiAgentReconciliation(ctx, 3*time.Millisecond, func(context.Context) error {
			calls.Add(1)
			select {
			case called <- struct{}{}:
			default:
			}
			return nil
		})
	}()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for i := 0; i < 3; i++ {
		select {
		case <-called:
		case <-timeout.C:
			t.Fatal("multi-agent reconciliation did not repeat")
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("multi-agent reconciliation goroutine did not stop on shutdown")
	}
	atStop := calls.Load()
	time.Sleep(15 * time.Millisecond)
	if calls.Load() != atStop {
		t.Fatalf("reconciliation continued after shutdown: %d to %d", atStop, calls.Load())
	}
}

func TestMultiAgentReconciliationRespectsOrchestratorReadiness(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var ready atomic.Bool
	var runs atomic.Int32
	executed := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runMultiAgentReconciliation(ctx, 3*time.Millisecond, func(context.Context) error {
			if !ready.Load() {
				return nil
			}
			runs.Add(1)
			select {
			case executed <- struct{}{}:
			default:
			}
			return nil
		})
	}()
	time.Sleep(18 * time.Millisecond)
	if runs.Load() != 0 {
		t.Fatal("worker reconciliation ran before the orchestrator was ready")
	}
	ready.Store(true)
	select {
	case <-executed:
	case <-time.After(time.Second):
		t.Fatal("pending workers were not reconciled after orchestrator became ready")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reconciliation ignored shutdown")
	}
}

func TestParentRecoveryContinuesWhenLegacyMultiAgentCheckpointFails(t *testing.T) {
	legacyError := errors.New("legacy checkpoint requires manual reconciliation")
	parentError := errors.New("parent turn scan cannot read a record")
	workerCalls := 0
	parentCalls := 0
	err := reconcileIndependentRecovery(context.Background(),
		func(context.Context) error { workerCalls++; return legacyError },
		func(context.Context) error { parentCalls++; return parentError })
	if workerCalls != 1 || parentCalls != 1 {
		t.Fatalf("one failed coordination starved independent parent recovery: workers=%d parents=%d", workerCalls, parentCalls)
	}
	if !errors.Is(err, legacyError) || !errors.Is(err, parentError) {
		t.Fatalf("independent reconciliation errors were lost: %v", err)
	}
}

func TestIndependentRecoveryDoesNotRunParentAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	parents := 0
	err := reconcileIndependentRecovery(ctx,
		func(context.Context) error { cancel(); return nil },
		func(context.Context) error { parents++; return nil })
	if err != nil || parents != 0 {
		t.Fatalf("recovery executed after cancellation: parents=%d err=%v", parents, err)
	}
}

func TestParentRecoveryRunsWithoutMultiAgentCoordinator(t *testing.T) {
	parentCalls := 0
	err := reconcileIndependentRecovery(context.Background(), nil, func(context.Context) error {
		parentCalls++
		return nil
	})
	if err != nil || parentCalls != 1 {
		t.Fatalf("ordinary parent recovery depends on MultiAgent: calls=%d err=%v", parentCalls, err)
	}
}

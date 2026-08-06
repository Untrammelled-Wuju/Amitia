// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

type mockGuard struct {
	started atomic.Bool
	exited  atomic.Bool
	pid     atomic.Int32
}

func (g *mockGuard) IsStarted() bool { return g.started.Load() }
func (g *mockGuard) IsExited() bool  { return g.exited.Load() }
func (g *mockGuard) PID() int        { return int(g.pid.Load()) }

func TestNewReadyWaiter(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()
	guard := &mockGuard{}

	waiter := NewReadyWaiter(target, policy, guard)
	if waiter == nil {
		t.Fatal("NewReadyWaiter returned nil")
	}
}

func TestReadyWaiterStop(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()
	guard := &mockGuard{}

	waiter := NewReadyWaiter(target, policy, guard)
	if waiter.Stopped() {
		t.Error("should not be stopped initially")
	}
	waiter.Stop()
	if !waiter.Stopped() {
		t.Error("should be stopped after Stop()")
	}
}

func TestReadyWaiterWaitInvalidTarget(t *testing.T) {
	policy := NewPolicy()
	guard := &mockGuard{}
	waiter := NewReadyWaiter(Target{}, policy, guard)

	_, err := waiter.Wait(context.Background())
	if err != ErrTargetAddressRequired {
		t.Errorf("expected ErrTargetAddressRequired, got %v", err)
	}
}

func TestReadyWaiterWaitNoGuard(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()
	waiter := NewReadyWaiter(target, policy, nil)

	_, err := waiter.Wait(context.Background())
	if err != ErrGuardRequired {
		t.Errorf("expected ErrGuardRequired, got %v", err)
	}
}

func TestReadyWaiterWaitProcessNotStarted(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()
	guard := &mockGuard{}

	waiter := NewReadyWaiter(target, policy, guard)
	_, err := waiter.Wait(context.Background())
	if err != ErrProcessNotStarted {
		t.Errorf("expected ErrProcessNotStarted, got %v", err)
	}
}

func TestReadyWaiterWaitProcessExited(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()
	guard := &mockGuard{}
	guard.started.Store(true)
	guard.exited.Store(true)

	waiter := NewReadyWaiter(target, policy, guard)
	_, err := waiter.Wait(context.Background())
	if err != ErrProcessExited {
		t.Errorf("expected ErrProcessExited, got %v", err)
	}
}

func TestReadyWaiterSuccess(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"title":"qdrant","version":"1.15.0"}`))
		case "/readyz":
			w.WriteHeader(http.StatusOK)
		case "/livez":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	policy := NewPolicy().WithStartupTimeout(10 * time.Second)
	guard := &mockGuard{}
	guard.started.Store(true)

	waiter := NewReadyWaiter(target, policy, guard)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	snapshot, err := waiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if snapshot.State != StateReady {
		t.Errorf("State = %v, want ready", snapshot.State)
	}
	if !snapshot.Identity.Confirmed {
		t.Error("identity should be confirmed")
	}
}

func TestReadyWaiterTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	policy := NewPolicy().WithStartupTimeout(1 * time.Second)
	guard := &mockGuard{}
	guard.started.Store(true)

	waiter := NewReadyWaiter(target, policy, guard)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := waiter.Wait(ctx)
	if err != ErrStartupTimeout {
		t.Errorf("expected ErrStartupTimeout, got %v", err)
	}
}

func TestReadyWaiterContextCancel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	policy := NewPolicy().WithStartupTimeout(30 * time.Second)
	guard := &mockGuard{}
	guard.started.Store(true)

	waiter := NewReadyWaiter(target, policy, guard)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := waiter.Wait(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestReadyWaiterOnReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"title":"qdrant","version":"1.15.0"}`))
		case "/readyz", "/livez":
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	policy := NewPolicy().WithStartupTimeout(5 * time.Second)
	guard := &mockGuard{}
	guard.started.Store(true)

	var readyCalled bool
	waiter := NewReadyWaiter(target, policy, guard)
	waiter.SetOnReady(func(s Snapshot) {
		readyCalled = true
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	waiter.Wait(ctx)
	if !readyCalled {
		t.Error("OnReady callback should have been called")
	}
}

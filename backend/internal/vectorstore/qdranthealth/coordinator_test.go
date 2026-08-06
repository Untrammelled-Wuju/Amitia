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

type mockController struct {
	stopCalled        atomic.Bool
	leaseReleaseCalled atomic.Bool
}

func (c *mockController) Stop(_ context.Context) error {
	c.stopCalled.Store(true)
	return nil
}

func (c *mockController) ReleaseLease(_ context.Context) error {
	c.leaseReleaseCalled.Store(true)
	return nil
}

func TestNewCoordinator(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()
	guard := &mockGuard{}

	c := NewCoordinator(target, policy, guard, nil)
	if c == nil {
		t.Fatal("NewCoordinator returned nil")
	}
	if c.Target().BaseURL != target.BaseURL {
		t.Errorf("Target = %v", c.Target())
	}
}

func TestCoordinatorWaitReady(t *testing.T) {
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
	policy := NewPolicy().WithStartupTimeout(10 * time.Second)
	guard := &mockGuard{}
	guard.started.Store(true)
	ctrl := &mockController{}

	c := NewCoordinator(target, policy, guard, ctrl)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	snapshot, err := c.WaitReady(ctx)
	if err != nil {
		t.Fatalf("WaitReady error: %v", err)
	}
	if snapshot.State != StateReady {
		t.Errorf("State = %v", snapshot.State)
	}
	if !c.IsReady() {
		t.Error("should be ready")
	}
}

func TestCoordinatorWaitReadyFailure(t *testing.T) {
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
	ctrl := &mockController{}

	c := NewCoordinator(target, policy, guard, ctrl)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.WaitReady(ctx)
	if err == nil {
		t.Error("expected timeout error")
	}

	if !ctrl.stopCalled.Load() {
		t.Error("controller Stop should have been called")
	}
	if !ctrl.leaseReleaseCalled.Load() {
		t.Error("controller ReleaseLease should have been called")
	}
}

func TestCoordinatorHealthCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	policy := NewPolicy()
	guard := &mockGuard{}
	guard.started.Store(true)

	c := NewCoordinator(target, policy, guard, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	snapshot := c.HealthCheck(ctx)
	if snapshot.State != StateReady {
		t.Errorf("State = %v", snapshot.State)
	}
}

func TestCoordinatorStop(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()
	guard := &mockGuard{}

	c := NewCoordinator(target, policy, guard, nil)
	c.Stop()
}

func TestCoordinatorReset(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()
	guard := &mockGuard{}

	c := NewCoordinator(target, policy, guard, nil)
	c.snapshot.State = StateReady
	c.started = true

	c.Reset()
	if c.State() != StateProcessStarted {
		t.Errorf("State = %v", c.State())
	}
}

func TestWaitWithTimeout(t *testing.T) {
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
	policy := NewPolicy()
	guard := &mockGuard{}
	guard.started.Store(true)

	snapshot, err := WaitWithTimeout(target, policy, guard, 10*time.Second)
	if err != nil {
		t.Fatalf("WaitWithTimeout error: %v", err)
	}
	if snapshot.State != StateReady {
		t.Errorf("State = %v", snapshot.State)
	}
}

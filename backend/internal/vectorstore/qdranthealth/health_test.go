// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHealthCheck(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	policy := NewPolicy()

	hc := NewHealthCheck(target, policy)
	if hc == nil {
		t.Fatal("NewHealthCheck returned nil")
	}
	if hc.State() != StateProcessStarted {
		t.Errorf("initial state = %v", hc.State())
	}
}

func TestHealthCheckCheckReady(t *testing.T) {
	readyCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/readyz":
			readyCalled = true
			w.WriteHeader(http.StatusOK)
		case "/livez":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	hc := NewHealthCheck(target, NewPolicy())

	snapshot := hc.Check(context.Background())
	if !readyCalled {
		t.Error("ready endpoint should have been called")
	}
	if snapshot.State != StateReady {
		t.Errorf("State = %v, want ready", snapshot.State)
	}
}

func TestHealthCheckCheckLive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	hc := NewHealthCheck(target, NewPolicy())

	snapshot := hc.CheckLive(context.Background())
	if snapshot.State != StateLive {
		t.Errorf("State = %v, want live", snapshot.State)
	}
	if !snapshot.LastResult.IsOK() {
		t.Error("expected OK result")
	}
}

func TestHealthCheckCheckReadyEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	hc := NewHealthCheck(target, NewPolicy())

	snapshot := hc.CheckReady(context.Background())
	if !hc.IsReady() {
		t.Error("should be ready")
	}
	if snapshot.State != StateReady {
		t.Errorf("State = %v", snapshot.State)
	}
}

func TestHealthCheckIsReady(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	hc := NewHealthCheck(target, NewPolicy())

	if hc.IsReady() {
		t.Error("initial state should not be ready")
	}

	hc.snapshot.State = StateReady
	if !hc.IsReady() {
		t.Error("should be ready after setting state")
	}
}

func TestHealthCheckIsLive(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	hc := NewHealthCheck(target, NewPolicy())

	if hc.IsLive() {
		t.Error("initial state should not be live")
	}

	hc.snapshot.State = StateLive
	if !hc.IsLive() {
		t.Error("should be live after setting state")
	}

	hc.snapshot.State = StateReady
	if !hc.IsLive() {
		t.Error("ready implies live")
	}
}

func TestHealthCheckReset(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	hc := NewHealthCheck(target, NewPolicy())

	hc.snapshot.State = StateReady
	hc.Reset()
	if hc.State() != StateProcessStarted {
		t.Errorf("State = %v", hc.State())
	}
}

func TestHealthCheckListener(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	hc := NewHealthCheck(target, NewPolicy())

	var received Snapshot
	hc.AddListener(func(s Snapshot) {
		received = s
	})

	hc.CheckReady(context.Background())
	if received.State != StateReady {
		t.Errorf("listener received state = %v", received.State)
	}
}

func TestHealthCheckSnapshotIsolation(t *testing.T) {
	target := NewTarget("127.0.0.1", 6333)
	hc := NewHealthCheck(target, NewPolicy())

	s1 := hc.Snapshot()
	s2 := hc.Snapshot()

	s1.State = StateReady
	if s2.State == StateReady {
		t.Error("snapshots should be independent clones")
	}
}

func TestHealthCheckCheckIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"title":"qdrant","version":"1.15.0"}`))
		}
	}))
	defer server.Close()

	target := NewTargetFromURL(server.URL)
	hc := NewHealthCheck(target, NewPolicy())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := hc.CheckIdentity(ctx)
	if err != nil {
		t.Fatalf("CheckIdentity error: %v", err)
	}
	if !id.Confirmed {
		t.Error("identity should be confirmed")
	}
	if id.Title != "qdrant" {
		t.Errorf("Title = %q", id.Title)
	}
	if id.Version != "1.15.0" {
		t.Errorf("Version = %q", id.Version)
	}
}

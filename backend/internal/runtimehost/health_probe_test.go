// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPCheckSuccess(t *testing.T) {
	// Create test server returning 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	}))
	defer server.Close()

	probe := NewHTTPHealthProbe(server.URL, 5*time.Second)
	ctx := context.Background()
	err := probe.Check(ctx)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestHTTPCheckFailureStatus(t *testing.T) {
	// Create test server returning 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	probe := NewHTTPHealthProbe(server.URL, 5*time.Second)
	ctx := context.Background()
	err := probe.Check(ctx)
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestHTTPCheckTimeout(t *testing.T) {
	// Create slow server that takes longer than timeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	probe := NewHTTPHealthProbe(server.URL, 100*time.Millisecond)
	ctx := context.Background()
	err := probe.Check(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestTCPHealthCheckSuccess(t *testing.T) {
	// Create TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer ln.Close()

	// Accept connections in background
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	probe := TCPHealthProbe{
		Address: ln.Addr().String(),
		Timeout: 5 * time.Second,
	}
	ctx := context.Background()
	err = probe.Check(ctx)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestTCPHealthCheckFailure(t *testing.T) {
	// Find a port that's not listening
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	addr := ln.Addr().String()
	// Close immediately so the port is not accepting connections
	ln.Close()

	probe := TCPHealthProbe{
		Address: addr,
		Timeout: 5 * time.Second,
	}
	ctx := context.Background()
	err = probe.Check(ctx)
	if err == nil {
		t.Fatal("expected error connecting to closed port")
	}
}

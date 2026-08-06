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

func TestHTTPProberSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"title":"qdrant","version":"1.15.0"}`))
	}))
	defer server.Close()

	prober := NewHTTPProber()
	target := NewProbeTarget(server.URL, EndpointRoot, 5*time.Second)
	result := prober.Probe(context.Background(), target)

	if !result.IsOK() {
		t.Errorf("expected OK, got status=%s err=%v", result.Status, result.Err)
	}
	if result.HTTPStatus != 200 {
		t.Errorf("HTTPStatus = %d, want 200", result.HTTPStatus)
	}
	if result.Endpoint != EndpointRoot {
		t.Errorf("Endpoint = %v", result.Endpoint)
	}
}

func TestHTTPProberUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	prober := NewHTTPProber()
	target := NewProbeTarget(server.URL, EndpointReadyz, 5*time.Second)
	result := prober.Probe(context.Background(), target)

	if result.IsOK() {
		t.Error("expected non-OK result for 503")
	}
	if result.Status != StatusUnexpectedStatus {
		t.Errorf("Status = %s, want unexpected_status", result.Status)
	}
}

func TestHTTPProberUnreachable(t *testing.T) {
	prober := NewHTTPProber()
	target := NewProbeTarget("http://127.0.0.1:1", EndpointRoot, 1*time.Second)
	result := prober.Probe(context.Background(), target)

	if result.IsOK() {
		t.Error("expected non-OK result for unreachable")
	}
	if result.Status != StatusUnreachable {
		t.Errorf("Status = %s, want unreachable", result.Status)
	}
}

func TestHTTPProberInvalidTarget(t *testing.T) {
	prober := NewHTTPProber()
	target := ProbeTarget{}
	result := prober.Probe(context.Background(), target)

	if result.Status != StatusProbeError {
		t.Errorf("Status = %s, want probe_error", result.Status)
	}
	if result.Err == nil {
		t.Error("expected error for invalid target")
	}
}

func TestHTTPProberBodyCapture(t *testing.T) {
	expectedBody := `{"title":"qdrant","version":"1.15.0"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	prober := NewHTTPProber()
	target := NewProbeTarget(server.URL, EndpointRoot, 5*time.Second)
	result := prober.Probe(context.Background(), target)

	if !result.IsOK() {
		t.Fatalf("unexpected status %s, err: %v", result.Status, result.Err)
	}
	if string(result.Body) != expectedBody {
		t.Errorf("Body = %q, want %q", string(result.Body), expectedBody)
	}
}

func TestEndpointFromPath(t *testing.T) {
	tests := []struct {
		path string
		want Endpoint
	}{
		{"/", EndpointRoot},
		{"/healthz", EndpointHealthz},
		{"/livez", EndpointLivez},
		{"/readyz", EndpointReadyz},
		{"/unknown", EndpointRoot},
	}
	for _, tt := range tests {
		if got := endpointFromPath(tt.path); got != tt.want {
			t.Errorf("endpointFromPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestNewProber(t *testing.T) {
	p := NewProber()
	if p == nil {
		t.Error("NewProber returned nil")
	}
	_, ok := p.(*HTTPProber)
	if !ok {
		t.Error("NewProber should return *HTTPProber")
	}
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"testing"
	"time"
)

func TestEndpointPath(t *testing.T) {
	tests := []struct {
		e    Endpoint
		want string
	}{
		{EndpointRoot, "/"},
		{EndpointHealthz, "/healthz"},
		{EndpointLivez, "/livez"},
		{EndpointReadyz, "/readyz"},
	}
	for _, tt := range tests {
		if got := tt.e.Path(); got != tt.want {
			t.Errorf("Endpoint(%d).Path() = %q, want %q", tt.e, got, tt.want)
		}
	}
}

func TestEndpointString(t *testing.T) {
	if EndpointRoot.String() != "/" {
		t.Errorf("EndpointRoot.String() = %q", EndpointRoot.String())
	}
	if EndpointReadyz.String() != "/readyz" {
		t.Errorf("EndpointReadyz.String() = %q", EndpointReadyz.String())
	}
}

func TestEndpointMethodString(t *testing.T) {
	if MethodGet.String() != "GET" {
		t.Errorf("MethodGet.String() = %q, want GET", MethodGet.String())
	}
}

func TestNewProbeTarget(t *testing.T) {
	target := NewProbeTarget("http://127.0.0.1:6333", EndpointReadyz, 5*time.Second)
	if target.BaseURL != "http://127.0.0.1:6333" {
		t.Errorf("BaseURL = %q", target.BaseURL)
	}
	if target.Path != "/readyz" {
		t.Errorf("Path = %q", target.Path)
	}
	if target.Method != "GET" {
		t.Errorf("Method = %q", target.Method)
	}
	if target.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v", target.Timeout)
	}
}

func TestProbeTargetURL(t *testing.T) {
	target := NewProbeTarget("http://127.0.0.1:6333", EndpointHealthz, 0)
	want := "http://127.0.0.1:6333/healthz"
	if got := target.URL(); got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}
}

func TestProbeTargetDefaultTimeout(t *testing.T) {
	target := NewProbeTarget("http://127.0.0.1:6333", EndpointRoot, 0)
	if target.Timeout != DefaultProbeTimeout {
		t.Errorf("default timeout = %v, want %v", target.Timeout, DefaultProbeTimeout)
	}
}

func TestProbeTargetWithTimeout(t *testing.T) {
	target := NewProbeTarget("http://127.0.0.1:6333", EndpointRoot, 0)
	target = target.WithTimeout(10 * time.Second)
	if target.Timeout != 10*time.Second {
		t.Errorf("timeout = %v, want 10s", target.Timeout)
	}
}

func TestValidateTarget(t *testing.T) {
	valid := NewProbeTarget("http://127.0.0.1:6333", EndpointRoot, 0)
	if err := ValidateTarget(valid); err != nil {
		t.Errorf("ValidateTarget returned error: %v", err)
	}

	invalid := ProbeTarget{}
	if err := ValidateTarget(invalid); err != ErrTargetAddressRequired {
		t.Errorf("ValidateTarget should return ErrTargetAddressRequired, got %v", err)
	}

	noPath := ProbeTarget{BaseURL: "http://127.0.0.1:6333"}
	if err := ValidateTarget(noPath); err != ErrTargetRequired {
		t.Errorf("ValidateTarget should return ErrTargetRequired, got %v", err)
	}
}

func TestBuildBaseURL(t *testing.T) {
	got := BuildBaseURL("127.0.0.1", 6333)
	want := "http://127.0.0.1:6333"
	if got != want {
		t.Errorf("BuildBaseURL = %q, want %q", got, want)
	}
}

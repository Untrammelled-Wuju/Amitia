package health

import (
	"testing"
	"time"
)

func TestMCPHealthSnapshot_IsReady(t *testing.T) {
	tests := []struct {
		state MCPHealthState
		want  bool
	}{
		{MCPHealthReady, true},
		{MCPHealthDegraded, false},
		{MCPHealthUnreachable, false},
		{MCPHealthUnknown, false},
		{MCPHealthAuthorizationRequired, false},
	}
	for _, tt := range tests {
		s := MCPHealthSnapshot{State: tt.state}
		if got := s.IsReady(); got != tt.want {
			t.Fatalf("IsReady(%s) = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestMCPHealthSnapshot_IsDegraded(t *testing.T) {
	s := MCPHealthSnapshot{State: MCPHealthDegraded}
	if !s.IsDegraded() {
		t.Fatal("expected IsDegraded() to return true")
	}
	s.State = MCPHealthReady
	if s.IsDegraded() {
		t.Fatal("expected IsDegraded() to return false for ready state")
	}
}

func TestMCPHealthSnapshot_IsUnavailable(t *testing.T) {
	tests := []struct {
		state MCPHealthState
		want  bool
	}{
		{MCPHealthUnreachable, true},
		{MCPHealthIncompatible, true},
		{MCPHealthFailed, true},
		{MCPHealthReady, false},
		{MCPHealthDegraded, false},
		{MCPHealthUnknown, false},
	}
	for _, tt := range tests {
		s := MCPHealthSnapshot{State: tt.state}
		if got := s.IsUnavailable(); got != tt.want {
			t.Fatalf("IsUnavailable(%s) = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestMCPHealthStateValues(t *testing.T) {
	states := []MCPHealthState{
		MCPHealthUnknown, MCPHealthDisabled, MCPHealthInstalling,
		MCPHealthAuthorizationRequired, MCPHealthStarting, MCPHealthReady,
		MCPHealthDegraded, MCPHealthUnreachable, MCPHealthIncompatible,
		MCPHealthFailed, MCPHealthStopped,
	}
	seen := make(map[MCPHealthState]bool)
	for _, s := range states {
		if s == "" {
			t.Fatal("health state should not be empty")
		}
		if seen[s] {
			t.Fatalf("duplicate health state: %s", s)
		}
		seen[s] = true
	}
}

func TestProtocolEraValues(t *testing.T) {
	eras := []ProtocolEra{MCPProtocolEraUnknown, MCPProtocolEraModern, MCPProtocolEraLegacy}
	seen := make(map[ProtocolEra]bool)
	for _, e := range eras {
		if e == "" {
			t.Fatal("protocol era should not be empty")
		}
		if seen[e] {
			t.Fatalf("duplicate protocol era: %s", e)
		}
		seen[e] = true
	}
}

func TestMCPHealthSnapshot_RetryAt(t *testing.T) {
	now := time.Now().UTC()
	s := MCPHealthSnapshot{
		ServerID:            "test",
		State:               MCPHealthUnreachable,
		ConsecutiveFailures: 3,
		RetryAt:             &now,
	}
	if s.RetryAt == nil {
		t.Fatal("RetryAt should not be nil")
	}
	if !s.RetryAt.Equal(now) {
		t.Fatalf("RetryAt mismatch: got %v, want %v", *s.RetryAt, now)
	}
}

func TestMCPHealthSnapshot_NoRetryOnSuccess(t *testing.T) {
	s := MCPHealthSnapshot{
		ServerID:            "test",
		State:               MCPHealthReady,
		ConsecutiveFailures: 0,
	}
	if s.RetryAt != nil {
		t.Fatal("RetryAt should be nil when no failures")
	}
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"testing"
)

func TestStateString(t *testing.T) {
	tests := []struct {
		s    State
		want string
	}{
		{StateProcessNotStarted, "process_not_started"},
		{StateProcessStarted, "process_started"},
		{StateIdentityConfirmed, "identity_confirmed"},
		{StateLive, "live"},
		{StateReady, "ready"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestStateComparisons(t *testing.T) {
	if StateProcessStarted.IsStarted() != true {
		expectedStates := []State{StateProcessStarted, StateIdentityConfirmed, StateLive, StateReady}
		for _, s := range expectedStates {
			if !s.IsStarted() {
				t.Errorf("%s should be started", s)
			}
		}
		unstarted := []State{StateProcessNotStarted}
		for _, s := range unstarted {
			if s.IsStarted() {
				t.Errorf("%s should not be started", s)
			}
		}
	}

	liveStates := []State{StateLive, StateReady}
	for _, s := range liveStates {
		if !s.IsLive() {
			t.Errorf("%s should be live", s)
		}
	}
	for _, s := range []State{StateProcessNotStarted, StateProcessStarted, StateIdentityConfirmed} {
		if s.IsLive() {
			t.Errorf("%s should not be live", s)
		}
	}

	if !StateReady.IsReady() {
		t.Error("StateReady should be ready")
	}
	for _, s := range []State{StateProcessNotStarted, StateProcessStarted, StateIdentityConfirmed, StateLive} {
		if s.IsReady() {
			t.Errorf("%s should not be ready", s)
		}
	}
}

func TestStateAtLeast(t *testing.T) {
	if !StateLive.AtLeast(StateProcessStarted) {
		t.Error("StateLive should be >= StateProcessStarted")
	}
	if !StateReady.AtLeast(StateLive) {
		t.Error("StateReady should be >= StateLive")
	}
	if StateProcessStarted.AtLeast(StateReady) {
		t.Error("StateProcessStarted should not be >= StateReady")
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		s    string
		want State
	}{
		{"process_started", StateProcessStarted},
		{"identity_confirmed", StateIdentityConfirmed},
		{"live", StateLive},
		{"ready", StateReady},
		{"unknown", StateProcessNotStarted},
	}

	for _, tt := range tests {
		if got := ParseState(tt.s); got != tt.want {
			t.Errorf("ParseState(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestStateFromStrings(t *testing.T) {
	input := []string{"ready", "live", "process_started"}
	want := []State{StateReady, StateLive, StateProcessStarted}
	got := StateFromStrings(input...)
	if len(got) != len(want) {
		t.Fatalf("got %d states, want %d", len(got), len(want))
	}
	for i, s := range got {
		if s != want[i] {
			t.Errorf("state[%d] = %v, want %v", i, s, want[i])
		}
	}
}

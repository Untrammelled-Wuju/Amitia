package workflow

import "testing"

func TestNodeStateTerminal(t *testing.T) {
	terminalStates := []NodeState{
		NodeStateSucceeded, NodeStateFailed, NodeStateDefaulted,
		NodeStateSkipped, NodeStateCancelled,
	}
	for _, s := range terminalStates {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}

	nonTerminalStates := []NodeState{
		NodeStatePending, NodeStateBlocked, NodeStateReady,
		NodeStateRunning, NodeStateRetryWait,
	}
	for _, s := range nonTerminalStates {
		if s.IsTerminal() {
			t.Errorf("%s should not be terminal", s)
		}
	}
}

func TestNodeStateActive(t *testing.T) {
	activeStates := []NodeState{NodeStateRunning, NodeStateRetryWait}
	for _, s := range activeStates {
		if !s.IsActive() {
			t.Errorf("%s should be active", s)
		}
	}

	inactiveStates := []NodeState{
		NodeStatePending, NodeStateBlocked, NodeStateReady,
		NodeStateSucceeded, NodeStateFailed, NodeStateDefaulted,
		NodeStateSkipped, NodeStateCancelled,
	}
	for _, s := range inactiveStates {
		if s.IsActive() {
			t.Errorf("%s should not be active", s)
		}
	}
}

func TestNodeStateValidity(t *testing.T) {
	validStates := []NodeState{
		NodeStatePending, NodeStateBlocked, NodeStateReady, NodeStateRunning,
		NodeStateSucceeded, NodeStateFailed, NodeStateDefaulted, NodeStateSkipped,
		NodeStateCancelled, NodeStateRetryWait,
	}
	for _, s := range validStates {
		if !s.IsValid() {
			t.Errorf("%s should be valid", s)
		}
	}

	invalidStates := []NodeState{"", "unknown", "invalid"}
	for _, s := range invalidStates {
		if s.IsValid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestNodeStateTransitions(t *testing.T) {
	tests := []struct {
		from    NodeState
		to      NodeState
		wantErr bool
	}{
		{NodeStatePending, NodeStateReady, false},
		{NodeStatePending, NodeStateBlocked, false},
		{NodeStatePending, NodeStateCancelled, false},
		{NodeStateBlocked, NodeStateReady, false},
		{NodeStateBlocked, NodeStateCancelled, false},
		{NodeStateReady, NodeStateRunning, false},
		{NodeStateReady, NodeStateCancelled, false},
		{NodeStateRunning, NodeStateSucceeded, false},
		{NodeStateRunning, NodeStateFailed, false},
		{NodeStateRunning, NodeStateDefaulted, false},
		{NodeStateRunning, NodeStateSkipped, false},
		{NodeStateRunning, NodeStateRetryWait, false},
		{NodeStateRunning, NodeStateCancelled, false},
		{NodeStateRetryWait, NodeStateRunning, false},
		{NodeStateRetryWait, NodeStateFailed, false},
		{NodeStateSucceeded, NodeStateSucceeded, false},
		{NodeStateSucceeded, NodeStateFailed, true},
		{NodeStateFailed, NodeStateRunning, true},
		{NodeStateCancelled, NodeStateRunning, true},
		{NodeStateReady, NodeStateSucceeded, true},
		{NodeStatePending, NodeStateRunning, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.from)+"_"+string(tt.to), func(t *testing.T) {
			err := ValidateNodeTransition(tt.from, tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNodeTransition(%s, %s) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
		})
	}
}

func TestDAGState(t *testing.T) {
	if !DAGStateSucceeded.IsTerminal() {
		t.Error("Succeeded should be terminal")
	}
	if !DAGStateFailed.IsTerminal() {
		t.Error("Failed should be terminal")
	}
	if !DAGStateCancelled.IsTerminal() {
		t.Error("Cancelled should be terminal")
	}
	if DAGStateRunning.IsTerminal() {
		t.Error("Running should not be terminal")
	}
	if DAGStateWaiting.IsTerminal() {
		t.Error("Waiting should not be terminal")
	}
	if DAGStateBlocked.IsTerminal() {
		t.Error("Blocked should not be terminal")
	}

	if !DAGStateRunning.IsActive() {
		t.Error("Running should be active")
	}
	if !DAGStateWaiting.IsActive() {
		t.Error("Waiting should be active")
	}
	if DAGStateSucceeded.IsActive() {
		t.Error("Succeeded should not be active")
	}
}

func TestDAGStateToRunStatus(t *testing.T) {
	tests := []struct {
		dagState DAGState
		expected RunStatus
	}{
		{DAGStateRunning, RunStatusRunning},
		{DAGStateWaiting, RunStatusRunning},
		{DAGStateSucceeded, RunStatusSucceeded},
		{DAGStateFailed, RunStatusFailed},
		{DAGStateCancelled, RunStatusCancelled},
		{DAGStateBlocked, RunStatusPaused},
	}

	for _, tt := range tests {
		got := tt.dagState.ToRunStatus()
		if got != tt.expected {
			t.Errorf("DAGState(%s).ToRunStatus() = %s, want %s", tt.dagState, got, tt.expected)
		}
	}
}

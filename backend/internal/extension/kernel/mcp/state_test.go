// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "testing"

func TestCanTransitionInstall_ValidTransitions(t *testing.T) {
	tests := []struct {
		from MCPInstallState
		to   MCPInstallState
	}{
		{MCPInstallAbsent, MCPInstallPlanning},
		{MCPInstallAbsent, MCPInstallInstalling},
		{MCPInstallPlanning, MCPInstallAwaitingApproval},
		{MCPInstallPlanning, MCPInstallInstalling},
		{MCPInstallPlanning, MCPInstallFailed},
		{MCPInstallAwaitingApproval, MCPInstallInstalling},
		{MCPInstallAwaitingApproval, MCPInstallFailed},
		{MCPInstallInstalling, MCPInstallInstalled},
		{MCPInstallInstalling, MCPInstallFailed},
		{MCPInstallInstalled, MCPInstallUpgrading},
		{MCPInstallInstalled, MCPInstallUninstalling},
		{MCPInstallInstalled, MCPInstallRollbackPending},
		{MCPInstallInstalled, MCPInstallRollingBack},
		{MCPInstallUpgrading, MCPInstallInstalled},
		{MCPInstallUpgrading, MCPInstallFailed},
		{MCPInstallUpgrading, MCPInstallRollbackPending},
		{MCPInstallRollbackPending, MCPInstallRollingBack},
		{MCPInstallRollingBack, MCPInstallInstalled},
		{MCPInstallRollingBack, MCPInstallFailed},
		{MCPInstallUninstalling, MCPInstallRemoved},
		{MCPInstallUninstalling, MCPInstallFailed},
		{MCPInstallFailed, MCPInstallPlanning},
		{MCPInstallFailed, MCPInstallInstalling},
		{MCPInstallFailed, MCPInstallUninstalling},
	}

	for _, tt := range tests {
		if !CanTransitionInstall(tt.from, tt.to) {
			t.Errorf("expected %s -> %s to be valid", tt.from, tt.to)
		}
	}
}

func TestCanTransitionInstall_InvalidTransitions(t *testing.T) {
	tests := []struct {
		from MCPInstallState
		to   MCPInstallState
	}{
		{MCPInstallAbsent, MCPInstallInstalled},
		{MCPInstallAbsent, MCPInstallUpgrading},
		{MCPInstallInstalled, MCPInstallInstalling},
		{MCPInstallInstalled, MCPInstallAbsent},
		{MCPInstallInstalling, MCPInstallUpgrading},
		{MCPInstallUpgrading, MCPInstallUninstalling},
		{MCPInstallRemoved, MCPInstallInstalled},
		{MCPInstallRemoved, MCPInstallAbsent},
		{MCPInstallRollingBack, MCPInstallUninstalling},
		{MCPInstallFailed, MCPInstallInstalled},
		{MCPInstallFailed, MCPInstallRemoved},
	}

	for _, tt := range tests {
		if CanTransitionInstall(tt.from, tt.to) {
			t.Errorf("expected %s -> %s to be invalid", tt.from, tt.to)
		}
	}
}

func TestCanTransitionRuntime_ValidTransitions(t *testing.T) {
	tests := []struct {
		from MCPRuntimeState
		to   MCPRuntimeState
	}{
		{MCPRuntimeDisabled, MCPRuntimeStopped},
		{MCPRuntimeStopped, MCPRuntimeStarting},
		{MCPRuntimeStopped, MCPRuntimeDisabled},
		{MCPRuntimeStarting, MCPRuntimeReady},
		{MCPRuntimeStarting, MCPRuntimeFailed},
		{MCPRuntimeReady, MCPRuntimeStopping},
		{MCPRuntimeReady, MCPRuntimeDegraded},
		{MCPRuntimeReady, MCPRuntimeFailed},
		{MCPRuntimeStopping, MCPRuntimeStopped},
		{MCPRuntimeStopping, MCPRuntimeFailed},
		{MCPRuntimeDegraded, MCPRuntimeStarting},
		{MCPRuntimeDegraded, MCPRuntimeFailed},
		{MCPRuntimeDegraded, MCPRuntimeStopping},
		{MCPRuntimeFailed, MCPRuntimeStarting},
		{MCPRuntimeFailed, MCPRuntimeStopped},
	}

	for _, tt := range tests {
		if !CanTransitionRuntime(tt.from, tt.to) {
			t.Errorf("expected %s -> %s to be valid", tt.from, tt.to)
		}
	}
}

func TestCanTransitionRuntime_InvalidTransitions(t *testing.T) {
	tests := []struct {
		from MCPRuntimeState
		to   MCPRuntimeState
	}{
		{MCPRuntimeDisabled, MCPRuntimeReady},
		{MCPRuntimeDisabled, MCPRuntimeStarting},
		{MCPRuntimeStopped, MCPRuntimeReady},
		{MCPRuntimeStopped, MCPRuntimeDegraded},
		{MCPRuntimeStarting, MCPRuntimeStopped},
		{MCPRuntimeStarting, MCPRuntimeDisabled},
		{MCPRuntimeReady, MCPRuntimeStarting},
		{MCPRuntimeReady, MCPRuntimeDisabled},
		{MCPRuntimeStopping, MCPRuntimeReady},
		{MCPRuntimeStopping, MCPRuntimeDisabled},
		{MCPRuntimeDegraded, MCPRuntimeReady},
		{MCPRuntimeDegraded, MCPRuntimeDisabled},
		{MCPRuntimeFailed, MCPRuntimeReady},
		{MCPRuntimeFailed, MCPRuntimeDegraded},
	}

	for _, tt := range tests {
		if CanTransitionRuntime(tt.from, tt.to) {
			t.Errorf("expected %s -> %s to be invalid", tt.from, tt.to)
		}
	}
}

func TestRequireApprovalForInstall(t *testing.T) {
	tests := []struct {
		state MCPInstallState
		want  bool
	}{
		{MCPInstallPlanning, true},
		{MCPInstallAwaitingApproval, true},
		{MCPInstallAbsent, false},
		{MCPInstallInstalling, false},
		{MCPInstallInstalled, false},
		{MCPInstallFailed, false},
		{MCPInstallRemoved, false},
		{MCPInstallUpgrading, false},
		{MCPInstallRollingBack, false},
		{MCPInstallUninstalling, false},
		{MCPInstallRollbackPending, false},
	}

	for _, tt := range tests {
		if got := RequireApprovalForInstall(tt.state); got != tt.want {
			t.Errorf("RequireApprovalForInstall(%s)=%v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestInvalidInstallTransitionError(t *testing.T) {
	err := &InvalidInstallTransitionError{From: MCPInstallAbsent, To: MCPInstallInstalled}
	want := "MCP_INSTALL_STATE_INVALID: invalid install transition from absent to installed"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestInvalidRuntimeTransitionError(t *testing.T) {
	err := &InvalidRuntimeTransitionError{From: MCPRuntimeStopped, To: MCPRuntimeReady}
	want := "MCP_RUNTIME_STATE_INVALID: invalid runtime transition from stopped to ready"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestInstallState_String(t *testing.T) {
	tests := []struct {
		state MCPInstallState
		want  string
	}{
		{MCPInstallAbsent, "absent"},
		{MCPInstallPlanning, "planning"},
		{MCPInstallInstalled, "installed"},
		{MCPInstallFailed, "install_failed"},
		{MCPInstallRemoved, "removed"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

func TestRuntimeState_String(t *testing.T) {
	tests := []struct {
		state MCPRuntimeState
		want  string
	}{
		{MCPRuntimeDisabled, "disabled"},
		{MCPRuntimeStopped, "stopped"},
		{MCPRuntimeReady, "ready"},
		{MCPRuntimeFailed, "runtime_failed"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}

// migration-only: temporary compatibility adapter
// remove at step 65 cutover
package mcp

import "fmt"

type MCPInstallState string

const (
	MCPInstallAbsent           MCPInstallState = "absent"
	MCPInstallPlanning         MCPInstallState = "planning"
	MCPInstallAwaitingApproval MCPInstallState = "awaiting_approval"
	MCPInstallInstalling       MCPInstallState = "installing"
	MCPInstallInstalled        MCPInstallState = "installed"
	MCPInstallUpgrading        MCPInstallState = "upgrading"
	MCPInstallRollbackPending  MCPInstallState = "rollback_pending"
	MCPInstallRollingBack      MCPInstallState = "rolling_back"
	MCPInstallUninstalling     MCPInstallState = "uninstalling"
	MCPInstallFailed           MCPInstallState = "install_failed"
	MCPInstallRemoved          MCPInstallState = "removed"
)

type MCPRuntimeState string

const (
	MCPRuntimeDisabled MCPRuntimeState = "disabled"
	MCPRuntimeStopped  MCPRuntimeState = "stopped"
	MCPRuntimeStarting MCPRuntimeState = "starting"
	MCPRuntimeReady    MCPRuntimeState = "ready"
	MCPRuntimeStopping MCPRuntimeState = "stopping"
	MCPRuntimeDegraded MCPRuntimeState = "degraded"
	MCPRuntimeFailed   MCPRuntimeState = "runtime_failed"
)

func (s MCPInstallState) String() string { return string(s) }
func (s MCPRuntimeState) String() string { return string(s) }

var validInstallTransitions = map[MCPInstallState][]MCPInstallState{
	MCPInstallAbsent:           {MCPInstallPlanning, MCPInstallInstalling},
	MCPInstallPlanning:         {MCPInstallAwaitingApproval, MCPInstallInstalling, MCPInstallFailed},
	MCPInstallAwaitingApproval: {MCPInstallInstalling, MCPInstallFailed},
	MCPInstallInstalling:       {MCPInstallInstalled, MCPInstallFailed},
	MCPInstallInstalled:        {MCPInstallUpgrading, MCPInstallUninstalling, MCPInstallRollbackPending, MCPInstallRollingBack},
	MCPInstallUpgrading:        {MCPInstallInstalled, MCPInstallFailed, MCPInstallRollbackPending},
	MCPInstallRollbackPending:  {MCPInstallRollingBack},
	MCPInstallRollingBack:      {MCPInstallInstalled, MCPInstallFailed},
	MCPInstallUninstalling:     {MCPInstallRemoved, MCPInstallFailed},
	MCPInstallFailed:           {MCPInstallPlanning, MCPInstallInstalling, MCPInstallUninstalling},
}

var validRuntimeTransitions = map[MCPRuntimeState][]MCPRuntimeState{
	MCPRuntimeDisabled: {MCPRuntimeStopped},
	MCPRuntimeStopped:  {MCPRuntimeStarting, MCPRuntimeDisabled},
	MCPRuntimeStarting: {MCPRuntimeReady, MCPRuntimeFailed},
	MCPRuntimeReady:    {MCPRuntimeStopping, MCPRuntimeDegraded, MCPRuntimeFailed},
	MCPRuntimeStopping: {MCPRuntimeStopped, MCPRuntimeFailed},
	MCPRuntimeDegraded: {MCPRuntimeStarting, MCPRuntimeFailed, MCPRuntimeStopping},
	MCPRuntimeFailed:   {MCPRuntimeStarting, MCPRuntimeStopped},
}

func CanTransitionInstall(from, to MCPInstallState) bool {
	allowed, ok := validInstallTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func CanTransitionRuntime(from, to MCPRuntimeState) bool {
	allowed, ok := validRuntimeTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func RequireApprovalForInstall(current MCPInstallState) bool {
	switch current {
	case MCPInstallPlanning, MCPInstallAwaitingApproval:
		return true
	}
	return false
}

type InvalidInstallTransitionError struct {
	From MCPInstallState
	To   MCPInstallState
}

func (e *InvalidInstallTransitionError) Error() string {
	return fmt.Errorf("MCP_INSTALL_STATE_INVALID: invalid install transition from %s to %s", e.From, e.To).Error()
}

type InvalidRuntimeTransitionError struct {
	From MCPRuntimeState
	To   MCPRuntimeState
}

func (e *InvalidRuntimeTransitionError) Error() string {
	return fmt.Errorf("MCP_RUNTIME_STATE_INVALID: invalid runtime transition from %s to %s", e.From, e.To).Error()
}

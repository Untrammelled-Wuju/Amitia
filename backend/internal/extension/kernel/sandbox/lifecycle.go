// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sandbox

import (
	"fmt"
	"time"
)

type SandboxLifecycleState string

const (
	SandboxStateIdle            SandboxLifecycleState = "idle"
	SandboxStateStarting        SandboxLifecycleState = "starting"
	SandboxStateRunning         SandboxLifecycleState = "running"
	SandboxStateQuiescing       SandboxLifecycleState = "quiescing"
	SandboxStateQuiesced        SandboxLifecycleState = "quiesced"
	SandboxStateStopping        SandboxLifecycleState = "stopping"
	SandboxStateFailed          SandboxLifecycleState = "failed"
	SandboxStateRecoveryPending SandboxLifecycleState = "recovery_pending"
	SandboxStateRecovering      SandboxLifecycleState = "recovering"
)

const (
	SandboxErrInvalidState    = "SANDBOX_INVALID_STATE"
	SandboxErrAlreadyRunning  = "SANDBOX_ALREADY_RUNNING"
	SandboxErrNotRunning      = "SANDBOX_NOT_RUNNING"
	SandboxErrQuiescing       = "SANDBOX_QUIESCING"
	SandboxErrQuiesced        = "SANDBOX_QUIESCED"
	SandboxErrStopping        = "SANDBOX_STOPPING"
	SandboxErrRecoveryPending = "SANDBOX_RECOVERY_PENDING"
	SandboxErrRecoveryFailed  = "SANDBOX_RECOVERY_FAILED"
	SandboxErrRestartFailed   = "SANDBOX_RESTART_FAILED"
	SandboxErrStartCancelled  = "SANDBOX_START_CANCELLED"
	SandboxErrStopFailed      = "SANDBOX_STOP_FAILED"
	SandboxErrDrainTimeout    = "SANDBOX_DRAIN_TIMEOUT"
	SandboxErrStaleGeneration = "SANDBOX_STALE_GENERATION"
	SandboxErrRootfsInvalid   = "SANDBOX_ROOTFS_INVALID"
	SandboxErrNativeCrash     = "SANDBOX_NATIVE_CRASH"
)

const (
	CommandDrainTimeout  = 30 * time.Second
	CommandCancelTimeout = 10 * time.Second
)

type SandboxLifecycleError struct {
	Code  string
	State SandboxLifecycleState
	Cause error
}

func (e *SandboxLifecycleError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s (state=%s): %v", e.Code, e.State, e.Cause)
	}
	return fmt.Sprintf("%s (state=%s)", e.Code, e.State)
}

func (e *SandboxLifecycleError) Unwrap() error {
	return e.Cause
}

type SandboxStopReason string

const (
	StopReasonUser                SandboxStopReason = "user"
	StopReasonApplicationShutdown SandboxStopReason = "application_shutdown"
	StopReasonRestart             SandboxStopReason = "restart"
	StopReasonRecovery            SandboxStopReason = "recovery"
	StopReasonDisable             SandboxStopReason = "disable"
)

type SandboxRestartReason string

const (
	RestartReasonRootfsChanged        SandboxRestartReason = "rootfs_changed"
	RestartReasonConfigurationChanged SandboxRestartReason = "configuration_changed"
	RestartReasonHealthFailure        SandboxRestartReason = "health_failure"
	RestartReasonManual               SandboxRestartReason = "manual"
	RestartReasonRecovery             SandboxRestartReason = "recovery"
)

type SandboxExecutionHandle struct {
	ExecutionID string
	RuntimeID   string
	Generation  uint64
}

var sandboxStateTransitions = map[SandboxLifecycleState]map[SandboxLifecycleState]bool{
	SandboxStateIdle: {
		SandboxStateStarting: true,
	},
	SandboxStateStarting: {
		SandboxStateRunning:  true,
		SandboxStateFailed:   true,
		SandboxStateStopping: true,
	},
	SandboxStateRunning: {
		SandboxStateQuiescing: true,
		SandboxStateStopping:  true,
		SandboxStateFailed:    true,
	},
	SandboxStateQuiescing: {
		SandboxStateQuiesced: true,
		SandboxStateStopping: true,
		SandboxStateFailed:   true,
	},
	SandboxStateQuiesced: {
		SandboxStateRunning:  true,
		SandboxStateStopping: true,
		SandboxStateFailed:   true,
	},
	SandboxStateFailed: {
		SandboxStateRecoveryPending: true,
		SandboxStateStopping:        true,
	},
	SandboxStateRecoveryPending: {
		SandboxStateRecovering: true,
		SandboxStateStopping:   true,
	},
	SandboxStateRecovering: {
		SandboxStateRunning:  true,
		SandboxStateFailed:   true,
		SandboxStateStopping: true,
	},
	SandboxStateStopping: {
		SandboxStateIdle:   true,
		SandboxStateFailed: true,
	},
}

func CanTransitionSandboxState(from, to SandboxLifecycleState) bool {
	if from == to {
		return false
	}
	targets, ok := sandboxStateTransitions[from]
	if !ok {
		return false
	}
	return targets[to]
}

func IsValidSandboxState(s SandboxLifecycleState) bool {
	switch s {
	case SandboxStateIdle, SandboxStateStarting, SandboxStateRunning,
		SandboxStateQuiescing, SandboxStateQuiesced, SandboxStateStopping,
		SandboxStateFailed, SandboxStateRecoveryPending, SandboxStateRecovering:
		return true
	default:
		return false
	}
}

func IsSandboxLifecycleTerminal(s SandboxLifecycleState) bool {
	return s == SandboxStateIdle
}

func CanExecuteInState(s SandboxLifecycleState) bool {
	return s == SandboxStateRunning
}

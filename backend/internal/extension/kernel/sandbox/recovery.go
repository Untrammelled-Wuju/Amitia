// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sandbox

import (
	"context"
	"time"
)

type SandboxRecoveryClass string

const (
	SandboxRecoveryNone                    SandboxRecoveryClass = "none"
	SandboxRecoveryRestartRuntime          SandboxRecoveryClass = "restart_runtime"
	SandboxRecoveryRestartWithActiveRootfs SandboxRecoveryClass = "restart_with_active_rootfs"
	SandboxRecoveryManualRepair            SandboxRecoveryClass = "manual_repair"
	SandboxRecoveryDisabled                SandboxRecoveryClass = "disabled"
)

type SandboxRecoveryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

var DefaultRecoveryPolicy = SandboxRecoveryPolicy{
	MaxAttempts:    3,
	InitialBackoff: 500 * time.Millisecond,
	MaxBackoff:     30 * time.Second,
}

type SandboxRecoverySnapshot struct {
	RuntimeID            string
	LifecycleState       SandboxLifecycleState
	Generation           uint64
	DesiredRunning       bool
	RecoveryPending      bool
	RestartRequired      bool
	ActiveExecutionID    string
	ActiveRootfsVersion  string
	ActiveRootfsDigest   string
	RunningRootfsVersion string
	RunningRootfsDigest  string
	RootfsInstalled      bool
	RootfsCorrupted      bool
	LastErrorCode        string
}

type SandboxRecoveryDescriptor struct {
	SchemaVersion           int       `json:"schemaVersion"`
	RuntimeID               string    `json:"runtimeId"`
	DesiredRunning          bool      `json:"desiredRunning"`
	Generation              uint64    `json:"generation"`
	ActiveRootfsVersion     string    `json:"activeRootfsVersion"`
	ActiveRootfsDigest      string    `json:"activeRootfsDigest"`
	LastKnownLifecycleState string    `json:"lastKnownLifecycleState"`
	CleanShutdown           bool      `json:"cleanShutdown"`
	Timestamp               time.Time `json:"timestamp"`
}

type SandboxRecoveryInspector interface {
	RecoverySnapshot(ctx context.Context) SandboxRecoverySnapshot
}

func (s SandboxRecoverySnapshot) Classify(policy SandboxRecoveryPolicy, providerEnabled bool) SandboxRecoveryClass {
	if !providerEnabled {
		return SandboxRecoveryDisabled
	}

	if s.RootfsCorrupted {
		return SandboxRecoveryManualRepair
	}

	if s.DesiredRunning && s.RuntimeID != "" {
		if s.RestartRequired && s.ActiveRootfsVersion != s.RunningRootfsVersion {
			return SandboxRecoveryRestartWithActiveRootfs
		}
		return SandboxRecoveryRestartRuntime
	}

	return SandboxRecoveryNone
}

func BuildRecoveryDescriptor(providerEnabled bool, snap SandboxRecoverySnapshot, cleanShutdown bool) *SandboxRecoveryDescriptor {
	return &SandboxRecoveryDescriptor{
		SchemaVersion:           1,
		RuntimeID:               snap.RuntimeID,
		DesiredRunning:          providerEnabled && snap.DesiredRunning,
		Generation:              snap.Generation,
		ActiveRootfsVersion:     snap.ActiveRootfsVersion,
		ActiveRootfsDigest:      snap.ActiveRootfsDigest,
		LastKnownLifecycleState: string(snap.LifecycleState),
		CleanShutdown:           cleanShutdown,
		Timestamp:               time.Now().UTC(),
	}
}

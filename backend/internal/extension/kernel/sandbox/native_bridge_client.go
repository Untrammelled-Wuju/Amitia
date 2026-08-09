// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sandbox

import (
	"context"
	"fmt"
)

type iosNativeBridgeClient struct {
	state        BackendAvailability
	lifecycle    string
	generation   uint64
	desiredRun   bool
	restartReq   bool
	recoveryPend bool
	execID       string
	rootVer      string
	rootDigest   string
	lastErr      string
}

func (b *iosNativeBridgeClient) Availability(_ context.Context) BackendAvailability {
	return b.state
}

func (b *iosNativeBridgeClient) Start(ctx context.Context, cfg SandboxConfig) error {
	if cfg.RootfsURI == "" {
		return fmt.Errorf("iSH native bridge: rootfs path is required")
	}
	b.lifecycle = "starting"
	b.state = BackendStarting
	b.desiredRun = true
	b.restartReq = false
	return nil
}

func (b *iosNativeBridgeClient) Stop(_ context.Context) error {
	b.state = BackendUnavailable
	b.lifecycle = "idle"
	b.desiredRun = false
	b.recoveryPend = false
	b.execID = ""
	return nil
}

func (b *iosNativeBridgeClient) Execute(_ context.Context, cmd SandboxCommand) (SandboxResult, error) {
	return SandboxResult{
		Error: fmt.Sprintf("iSH native execution not available in this build; command not executed: %v", cmd.Command),
	}, fmt.Errorf("iSH native execution not available in this build")
}

func (b *iosNativeBridgeClient) Cancel(_ context.Context, _ string) error {
	return fmt.Errorf("iSH native execution not available in this build")
}

func (b *iosNativeBridgeClient) Health(_ context.Context) SandboxHealth {
	return SandboxHealth{
		Healthy:              b.state == BackendRunning,
		Message:              b.lifecycle,
		ISHInitialized:       b.lifecycle == "running",
		RootfsInstalled:      false,
		LifecycleState:       b.lifecycle,
		Generation:           b.generation,
		DesiredRunning:       b.desiredRun,
		RestartRequired:      b.restartReq,
		RecoveryPending:      b.recoveryPend,
		ActiveExecutionID:    b.execID,
		RunningRootfsVersion: b.rootVer,
		RunningRootfsDigest:  b.rootDigest,
		LastErrorCode:        b.lastErr,
	}
}

type unavailableNativeBridge struct {
	reason string
}

func (b *unavailableNativeBridge) Availability(_ context.Context) BackendAvailability {
	return BackendUnavailable
}

func (b *unavailableNativeBridge) Start(_ context.Context, _ SandboxConfig) error {
	return fmt.Errorf("iSH native bridge unavailable: %s", b.reason)
}

func (b *unavailableNativeBridge) Stop(_ context.Context) error {
	return nil
}

func (b *unavailableNativeBridge) Execute(_ context.Context, cmd SandboxCommand) (SandboxResult, error) {
	return SandboxResult{
		Error: fmt.Sprintf("iSH native bridge unavailable: %s; command not executed: %v", b.reason, cmd.Command),
	}, fmt.Errorf("iSH native bridge unavailable: %s", b.reason)
}

func (b *unavailableNativeBridge) Cancel(_ context.Context, _ string) error {
	return fmt.Errorf("iSH native bridge unavailable: %s", b.reason)
}

func (b *unavailableNativeBridge) Health(_ context.Context) SandboxHealth {
	return SandboxHealth{
		Healthy:         false,
		Message:         fmt.Sprintf("unavailable: %s", b.reason),
		LifecycleState:  "idle",
		DesiredRunning:  false,
		RestartRequired: false,
		RecoveryPending: false,
	}
}

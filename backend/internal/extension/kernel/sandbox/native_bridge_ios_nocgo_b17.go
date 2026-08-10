// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
//go:build ios && !cgo
// +build ios,!cgo

package sandbox

import (
	"context"
)

func newIOSNativeBridgeClient() *iosNativeBridgeClient {
	return &iosNativeBridgeClient{state: BackendUnavailable}
}

type iosNativeBridgeClient struct {
	state BackendAvailability
}

func (b *iosNativeBridgeClient) Availability(_ context.Context) BackendAvailability {
	return BackendUnavailable
}

func (b *iosNativeBridgeClient) Start(_ context.Context, _ SandboxConfig) error {
	return &NativeBridgeError{Code: NativeErrRuntimeNotStarted, Message: "iSH native bridge requires CGO_ENABLED=1"}
}

func (b *iosNativeBridgeClient) Stop(_ context.Context) error {
	return nil
}

func (b *iosNativeBridgeClient) Execute(_ context.Context, _ SandboxExecuteRequest) (SandboxExecuteResult, error) {
	return SandboxExecuteResult{}, &NativeBridgeError{
		Code:    NativeErrRuntimeNotStarted,
		Message: "iSH native bridge requires CGO_ENABLED=1",
	}
}

func (b *iosNativeBridgeClient) Cancel(_ context.Context, _ string) error {
	return &NativeBridgeError{
		Code:    NativeErrRuntimeNotStarted,
		Message: "iSH native bridge requires CGO_ENABLED=1",
	}
}

func (b *iosNativeBridgeClient) Health(_ context.Context) SandboxHealth {
	return SandboxHealth{
		Healthy: false,
		Message: "iSH native bridge requires CGO_ENABLED=1",
	}
}

func (b *iosNativeBridgeClient) RootfsStatus(_ context.Context) (RootfsStatus, error) {
	return RootfsStatus{}, &RootfsError{
		Code:    RootfsErrNotConfigured,
		Message: "iSH native bridge requires CGO_ENABLED=1",
	}
}

func (b *iosNativeBridgeClient) EnsureRootfs(_ context.Context, _ RootfsInstallSpec) (RootfsInstallResult, error) {
	return RootfsInstallResult{}, &RootfsError{
		Code:    RootfsErrNotConfigured,
		Message: "iSH native bridge requires CGO_ENABLED=1",
	}
}

func (b *iosNativeBridgeClient) ActivateRootfs(_ context.Context, _ string) error {
	return &RootfsError{
		Code:    RootfsErrNotConfigured,
		Message: "iSH native bridge requires CGO_ENABLED=1",
	}
}

func (b *iosNativeBridgeClient) RemoveRootfs(_ context.Context, _ string) error {
	return &RootfsError{
		Code:    RootfsErrNotConfigured,
		Message: "iSH native bridge requires CGO_ENABLED=1",
	}
}

package sandbox

import (
	"context"
	"fmt"
	"runtime"
)

type NativeBridge interface {
	Availability(ctx context.Context) BackendAvailability
	Start(ctx context.Context, config SandboxConfig) error
	Stop(ctx context.Context) error
	Execute(ctx context.Context, req SandboxExecuteRequest) (SandboxExecuteResult, error)
	Cancel(ctx context.Context, executionID string) error
	Health(ctx context.Context) SandboxHealth
	RootfsStatus(ctx context.Context) (RootfsStatus, error)
	EnsureRootfs(ctx context.Context, spec RootfsInstallSpec) (RootfsInstallResult, error)
	ActivateRootfs(ctx context.Context, installationID string) error
	RemoveRootfs(ctx context.Context, installationID string) error
}

func NewNativeBridge() NativeBridge {
	if runtime.GOOS == "ios" {
		return newIOSNativeBridgeClient()
	}
	return &unavailableNativeBridge{reason: fmt.Sprintf("iSH native bridge not supported on %s", runtime.GOOS)}
}

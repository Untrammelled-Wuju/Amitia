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
	Execute(ctx context.Context, cmd SandboxCommand) (SandboxResult, error)
	Cancel(ctx context.Context, executionID string) error
	Health(ctx context.Context) SandboxHealth
}

func NewNativeBridge() NativeBridge {
	if runtime.GOOS == "darwin" || runtime.GOOS == "ios" {
		return &iosNativeBridgeClient{state: BackendUnavailable}
	}
	return &unavailableNativeBridge{reason: fmt.Sprintf("iSH native bridge not supported on %s", runtime.GOOS)}
}

package sandbox

import (
	"context"
	"fmt"
)

type iosNativeBridgeClient struct {
	state BackendAvailability
}

func (b *iosNativeBridgeClient) Availability(_ context.Context) BackendAvailability {
	return b.state
}

func (b *iosNativeBridgeClient) Start(ctx context.Context, cfg SandboxConfig) error {
	if cfg.RootfsURI == "" {
		return fmt.Errorf("iSH native bridge: rootfs path is required")
	}
	b.state = BackendStarting
	return nil
}

func (b *iosNativeBridgeClient) Stop(_ context.Context) error {
	b.state = BackendUnavailable
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
		Healthy:         b.state == BackendRunning,
		Message:         b.state.String(),
		ISHInitialized:  false,
		RootfsInstalled: false,
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
		Healthy: false,
		Message: fmt.Sprintf("unavailable: %s", b.reason),
	}
}

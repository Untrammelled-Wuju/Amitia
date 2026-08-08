package builtin

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

const ComponentIDIOSSandbox runtimeorchestrator.ComponentID = "provider.ios-sandbox"

type iosSandboxProviderInstance struct {
	backend sandbox.SandboxBackend
	host    runtimehost.RuntimeHost
	config  IOSSandboxProviderConfig
}

func newIOSSandboxProviderInstance(
	backend sandbox.SandboxBackend,
	host runtimehost.RuntimeHost,
	config IOSSandboxProviderConfig,
) *iosSandboxProviderInstance {
	return &iosSandboxProviderInstance{
		backend: backend,
		host:    host,
		config:  config,
	}
}

func (p *iosSandboxProviderInstance) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:       ComponentIDIOSSandbox,
		Phase:    runtimeorchestrator.PhaseApplication,
		Enabled:  p.config.Enabled,
		Required: false,
		Capabilities: []string{
			"platform/ios/sandbox",
			"platform/ios/ish",
			"sandbox/execute",
		},
	}
}

func (p *iosSandboxProviderInstance) Start(
	ctx context.Context,
) error {
	if !p.config.Enabled {
		return nil
	}

	if p.host == nil {
		return fmt.Errorf(
			"ios sandbox: runtime host missing",
		)
	}

	return p.backend.Start(
		ctx,
		sandbox.SandboxConfig{
			RuntimeID:    p.host.RuntimeInstanceID(),
			WorkspaceURI: p.config.WorkspaceURI,
			RootfsURI:    p.config.RootfsURI,
			Environment:  cloneStringMap(p.config.Environment),
		},
	)
}

func (p *iosSandboxProviderInstance) Ready(
	ctx context.Context,
) error {
	if !p.config.Enabled {
		return nil
	}

	health := p.backend.Health(ctx)

	if !health.Healthy {
		return fmt.Errorf(
			"ios sandbox provider not ready: %s",
			health.Message,
		)
	}

	return nil
}

func (p *iosSandboxProviderInstance) Stop(
	ctx context.Context,
) error {
	if !p.config.Enabled {
		return nil
	}

	return p.backend.Stop(ctx)
}

func (p *iosSandboxProviderInstance) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotIOSSandbox
}

func (p *iosSandboxProviderInstance) ProviderID() string {
	return sandbox.ProviderIDIOSSandbox
}

func (p *iosSandboxProviderInstance) Capability() any {
	health := p.backend.Health(context.Background())

	runtimeID := ""
	hostPlatform := ""

	if p.host != nil {
		runtimeID = p.host.RuntimeInstanceID()
		hostPlatform = string(
			p.host.Descriptor().Host,
		)
	}

	return map[string]any{
		"providerId":      sandbox.ProviderIDIOSSandbox,
		"slot":            string(runtimeorchestrator.ProviderSlotIOSSandbox),
		"runtimeId":       runtimeID,
		"hostPlatform":    hostPlatform,
		"availability":    p.backend.Availability(context.Background()).String(),
		"healthy":         health.Healthy,
		"ishInitialized":  health.ISHInitialized,
		"rootfsInstalled": health.RootfsInstalled,
	}
}

func cloneStringMap(
	src map[string]string,
) map[string]string {
	if src == nil {
		return nil
	}

	dst := make(map[string]string, len(src))

	for k, v := range src {
		dst[k] = v
	}

	return dst
}

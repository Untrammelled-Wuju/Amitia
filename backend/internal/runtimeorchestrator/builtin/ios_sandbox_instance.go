package builtin

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

const ComponentIDIOSSandbox runtimeorchestrator.ComponentID = "provider.ios-sandbox"

type providerInstance struct {
	backend    sandbox.SandboxBackend
	runtimeID  string
	slot       runtimeorchestrator.ProviderSlot
	providerID string
}

func newProviderInstance(backend sandbox.SandboxBackend, bc runtimeorchestrator.ProviderBuildContext) *providerInstance {
	return &providerInstance{
		backend:    backend,
		runtimeID:  "",
		slot:       runtimeorchestrator.ProviderSlot(sandbox.SlotIOSSandbox),
		providerID: sandbox.ProviderIDIOSSandbox,
	}
}

func (p *providerInstance) Descriptor() runtimeorchestrator.ComponentDescriptor {
	return runtimeorchestrator.ComponentDescriptor{
		ID:       ComponentIDIOSSandbox,
		Phase:    runtimeorchestrator.PhaseApplication,
		Enabled:  true,
		Required: false,
		Capabilities: []string{
			"platform/ios/ish",
			"sandbox/execute",
		},
	}
}

func (p *providerInstance) Start(ctx context.Context) error {
	return p.backend.Start(ctx, sandbox.SandboxConfig{
		RuntimeID:  p.runtimeID,
		RootfsURI:  "",
	})
}

func (p *providerInstance) Ready(ctx context.Context) error {
	health := p.backend.Health(ctx)
	if !health.Healthy {
		return fmt.Errorf("ios sandbox provider not ready: %s", health.Message)
	}
	return nil
}

func (p *providerInstance) Stop(ctx context.Context) error {
	return p.backend.Stop(ctx)
}

func (p *providerInstance) Slot() runtimeorchestrator.ProviderSlot {
	return p.slot
}

func (p *providerInstance) ProviderID() string {
	return p.providerID
}

func (p *providerInstance) Capability() any {
	return map[string]any{
		"available": p.backend.Availability(context.Background()),
		"slot":      string(p.slot),
	}
}

package builtin

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

type IOSSandboxProviderFactory struct {
	config ProviderConfig
}

type ProviderConfig struct {
	EnableISH	bool
	RootfsURI	string
}

func NewIOSSandboxProviderFactory(config ProviderConfig) *IOSSandboxProviderFactory {
	return &IOSSandboxProviderFactory{config: config}
}

func (f *IOSSandboxProviderFactory) ProviderID() string {
	return sandbox.ProviderIDIOSSandbox
}

func (f *IOSSandboxProviderFactory) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlot(sandbox.SlotIOSSandbox)
}

func (f *IOSSandboxProviderFactory) Requirements() []runtimehost.CapabilityRequirement {
	return nil
}

func (f *IOSSandboxProviderFactory) Build(ctx context.Context, bc runtimeorchestrator.ProviderBuildContext) (runtimeorchestrator.ProviderInstance, error) {
	backend, err := sandbox.NewIOSSandboxBackend()
	if err != nil {
		return nil, fmt.Errorf("ios sandbox: failed to create backend: %w", err)
	}
	instance := newProviderInstance(backend, bc)
	return instance, nil
}

package builtin

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
)

type IOSSandboxProviderConfig struct {
	Enabled      bool
	WorkspaceURI string
	RootfsURI    string
	Environment  map[string]string
}

type IOSSandboxProviderFactory struct {
	config IOSSandboxProviderConfig

	newBackend func() (sandbox.SandboxBackend, error)
}

func NewIOSSandboxProviderFactory(
	config IOSSandboxProviderConfig,
) *IOSSandboxProviderFactory {
	return &IOSSandboxProviderFactory{
		config:     config,
		newBackend: sandbox.NewIOSSandboxBackend,
	}
}

func (f *IOSSandboxProviderFactory) ProviderID() string {
	return sandbox.ProviderIDIOSSandbox
}

func (f *IOSSandboxProviderFactory) Slot() runtimeorchestrator.ProviderSlot {
	return runtimeorchestrator.ProviderSlotIOSSandbox
}

func (f *IOSSandboxProviderFactory) Requirements() []runtimehost.CapabilityRequirement {
	return []runtimehost.CapabilityRequirement{
		{
			ID:      runtimehost.CapRuntimeSandboxedExec,
			Minimum: runtimehost.SupportSupported,
		},
		{
			ID:      runtimehost.CapRuntimeNativeOffload,
			Minimum: runtimehost.SupportLimited,
		},
	}
}

func (f *IOSSandboxProviderFactory) Build(
	bc runtimeorchestrator.ProviderBuildContext,
) (runtimeorchestrator.ProviderInstance, error) {
	if bc.Host == nil {
		return nil, fmt.Errorf(
			"ios sandbox: runtime host is required",
		)
	}

	descriptor := bc.Host.Descriptor()

	if descriptor.Host != platform.HostPlatformIOS {
		return nil, fmt.Errorf(
			"ios sandbox: unsupported host platform: %s",
			descriptor.Host,
		)
	}

	if f.newBackend == nil {
		return nil, fmt.Errorf(
			"ios sandbox: backend factory is not configured",
		)
	}

	backend, err := f.newBackend()
	if err != nil {
		return nil, fmt.Errorf(
			"ios sandbox: create backend: %w", err,
		)
	}

	if backend == nil {
		return nil, fmt.Errorf(
			"ios sandbox: backend factory returned nil",
		)
	}

	return newIOSSandboxProviderInstance(
		backend,
		bc.Host,
		f.config,
	), nil
}

var _ runtimeorchestrator.ProviderFactory = (*IOSSandboxProviderFactory)(nil)

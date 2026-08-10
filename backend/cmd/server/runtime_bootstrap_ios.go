//go:build ios

package main

import (
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
)

func (b *runtimeBootstrap) buildPlatformProvidersIOS() error {
	if b == nil {
		return nil
	}
	if b.host.Descriptor().Host != platform.HostPlatformIOS {
		return nil
	}

	iosCfg := config.AppCfg.Runtime.IOSandbox
	if err := iosCfg.Validate(); err != nil {
		return fmt.Errorf("ios sandbox config validation failed: %w", err)
	}

	if !iosCfg.Enabled {
		return nil
	}

	buildCtx := runtimeorchestrator.ProviderBuildContext{
		Config: config.AppCfg,
		Host:   b.host,
	}

	instance, err := b.providerRegistry.Build(
		runtimeorchestrator.ProviderSlotIOSSandbox,
		sandbox.ProviderIDIOSSandbox,
		buildCtx,
	)
	if err != nil {
		return fmt.Errorf("build ios sandbox provider: %w", err)
	}

	if err := b.orchestrator.Register(instance); err != nil {
		return fmt.Errorf("register ios sandbox provider to orchestrator: %w", err)
	}

	b.iosSandboxProvider = instance
	return nil
}

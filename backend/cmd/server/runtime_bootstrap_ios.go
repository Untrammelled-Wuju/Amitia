//go:build ios
// +build ios

package main

import (
	"fmt"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/extension/kernel/sandbox"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/runtimeorchestrator/builtin"
)

func (b *runtimeBootstrap) registerProviderFactoriesIOS() error {
	if b == nil || b.providerRegistry == nil {
		return nil
	}

	sandboxFactory := builtin.NewIOSSandboxProviderFactory(
		builtin.IOSSandboxProviderConfig{
			Enabled: true,
		},
	)

	if err := b.providerRegistry.Register(sandboxFactory); err != nil {
		return fmt.Errorf(
			"register ios sandbox provider factory: %w", err,
		)
	}

	nativeFactory := builtin.NewIOSNativeProviderFactory(
		builtin.IOSNativeProviderConfig{
			Bridge: b.iosNativeBridge,
		},
	)

	if err := b.providerRegistry.Register(nativeFactory); err != nil {
		return fmt.Errorf(
			"register ios native provider factory: %w", err,
		)
	}

	return nil
}

func (b *runtimeBootstrap) buildIOSNativeBridge() nativebridge.Bridge {
	bridge := nativebridge.NewIOSBridge()
	b.SetIOSNativeBridge(bridge)
	return bridge
}

func (b *runtimeBootstrap) IOSNativeBridge() nativebridge.Bridge {
	if b == nil {
		return nil
	}
	return b.iosNativeBridge
}

func (b *runtimeBootstrap) SetIOSNativeBridge(bridge nativebridge.Bridge) error {
	if b == nil {
		return nil
	}
	b.iosNativeBridge = bridge
	if b.iosNativeProvider != nil {
		prov, ok := b.iosNativeProvider.(interface {
			SetBridge(nativebridge.Bridge) error
		})
		if ok {
			return prov.SetBridge(bridge)
		}
	}
	return nil
}

func (b *runtimeBootstrap) buildPlatformProvidersIOS() error {
	if b == nil || b.providerRegistry == nil {
		return nil
	}

	sandboxInstance, err := b.providerRegistry.Build(
		runtimeorchestrator.ProviderSlotIOSSandbox,
		sandbox.ProviderIDIOSSandbox,
		runtimeorchestrator.ProviderBuildContext{
			Config: config.AppCfg,
			Host:   b.host,
		},
	)
	if err != nil {
		return fmt.Errorf("build ios sandbox provider: %w", err)
	}

	b.iosSandboxProvider = sandboxInstance
	b.orchestrator.Register(sandboxInstance)

	nativeInstance, err := b.providerRegistry.Build(
		runtimeorchestrator.ProviderSlotIOSNative,
		"ios-native",
		runtimeorchestrator.ProviderBuildContext{
			Config: config.AppCfg,
			Host:   b.host,
		},
	)
	if err != nil {
		return fmt.Errorf("build ios native provider: %w", err)
	}

	b.iosNativeProvider = nativeInstance
	b.orchestrator.Register(nativeInstance)

	return nil
}

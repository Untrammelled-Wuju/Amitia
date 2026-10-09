//go:build !ios

package main

import (
	"fmt"
	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/pkg/platform"
)

func (b *runtimeBootstrap) buildIOSNativeBridge() nativebridge.Bridge {
	if b == nil || b.host == nil || b.host.Descriptor().Host != platform.HostPlatformIOS {
		return nil
	}
	b.iosNativeBridge = nativebridge.NewIOSBridge()
	return b.iosNativeBridge
}

func (b *runtimeBootstrap) IOSNativeBridge() nativebridge.Bridge {
	if b == nil {
		return nil
	}
	return b.iosNativeBridge
}

func (b *runtimeBootstrap) buildPlatformProvidersIOS() error {
	if b == nil || b.host == nil || b.providerRegistry == nil || b.host.Descriptor().Host != platform.HostPlatformIOS {
		return nil
	}
	instance, err := b.providerRegistry.Build(runtimeorchestrator.ProviderSlotIOSNative, "ios-native", runtimeorchestrator.ProviderBuildContext{Config: config.AppCfg, Host: b.host})
	if err != nil {
		return fmt.Errorf("build iOS guest native provider: %w", err)
	}
	b.iosNativeProvider = instance
	b.orchestrator.Register(instance)
	return nil
}

//go:build !ios
// +build !ios

package main

import (
	"github.com/u-ai/backend/internal/runtimeorchestrator/builtin"
	"github.com/u-ai/backend/pkg/platform"
)

func (b *runtimeBootstrap) registerProviderFactoriesIOS() error {
	if b == nil || b.providerRegistry == nil {
		return nil
	}

	if b.host == nil || b.host.Descriptor().Host != platform.HostPlatformIOS {
		return nil
	}
	return b.providerRegistry.Register(builtin.NewIOSNativeProviderFactory(builtin.IOSNativeProviderConfig{Bridge: b.iosNativeBridge}))
}

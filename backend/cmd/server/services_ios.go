//go:build ios
// +build ios

package main

import (
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

func applyIOSNativeProvider(builder *kernel.ContainerBuilder, provider runtimeorchestrator.ProviderInstance) *kernel.ContainerBuilder {
	if builder == nil || provider == nil {
		return builder
	}
	iosProvider, ok := provider.(capability.IOSProvider)
	if !ok {
		return builder
	}
	return builder.WithIOSNativeProvider(iosProvider)
}

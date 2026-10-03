//go:build !ios
// +build !ios

package main

import (
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

func applyIOSNativeProvider(builder *kernel.ContainerBuilder, provider runtimeorchestrator.ProviderInstance) *kernel.ContainerBuilder {
	return builder
}

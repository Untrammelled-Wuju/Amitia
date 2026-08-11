//go:build ios
// +build ios

package main

import (
	"fmt"

	"github.com/u-ai/backend/internal/runtimeorchestrator/builtin"
)

func (b *runtimeBootstrap) registerProviderFactoriesIOS() error {
	if b == nil || b.providerRegistry == nil {
		return nil
	}

	factory := builtin.NewIOSSandboxProviderFactory(
		builtin.IOSSandboxProviderConfig{
			Enabled: true,
		},
	)

	if err := b.providerRegistry.Register(factory); err != nil {
		return fmt.Errorf(
			"register ios sandbox provider factory: %w", err,
		)
	}

	return nil
}

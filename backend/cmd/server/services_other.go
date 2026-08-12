//go:build !linux || android

package main

import (
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/runtimehost"
)

func applyAndroidLinuxProvider(builder *kernel.ContainerBuilder, host runtimehost.RuntimeHost) *kernel.ContainerBuilder {
	return builder
}

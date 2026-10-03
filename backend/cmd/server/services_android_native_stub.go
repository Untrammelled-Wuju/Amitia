//go:build !linux

package main

import (
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/imageintelligence"
	"github.com/u-ai/backend/pkg/resourceuri"
)

func applyAndroidNativeProvider(
	builder *kernel.ContainerBuilder,
	bootstrap *runtimeBootstrap,
	imageIntelligence imageintelligence.ImageIntelligence,
	resourceResolver *resourceuri.PhysicalResolver,
	dataDir string,
) (*kernel.ContainerBuilder, error) {
	return builder, nil
}

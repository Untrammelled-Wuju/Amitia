//go:build linux && !android

package main

import (
	"log"

	"github.com/u-ai/backend/internal/androidlinux/terminal"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/runtimehost"
)

func applyAndroidLinuxProvider(builder *kernel.ContainerBuilder, host runtimehost.RuntimeHost) *kernel.ContainerBuilder {
	if host == nil {
		return builder
	}
	if !terminal.IsAndroidLinuxRuntime(host) {
		return builder
	}
	workspaceRoot := ""
	if host != nil {
		workspaceRoot = host.Paths().WorkspaceDir
	}
	provider, err := terminal.NewProvider(
		host,
		workspaceRoot,
		terminal.DefaultPolicy(),
	)
	if err != nil {
		log.Warn("failed to create android linux terminal provider:", err)
		return builder
	}
	return builder.WithAndroidLinuxProvider(provider)
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"fmt"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type RuntimeDependencies struct {
	Host         runtimehost.RuntimeHost
	NodeResolver nodeenv.Resolver
	Config       *config.Config
}

func NewRuntimeDependencies() (*RuntimeDependencies, error) {
	runtimeRoot := util.RuntimeRoot()
	configDir := util.RuntimeConfigDir(runtimeRoot)
	config.InitConfig(configDir)

	paths := util.DetectRuntimePaths(config.AppCfg.Storage.DataDir)

	descriptor := platform.Get().Descriptor()
	host, err := runtimehost.NewRuntimeHost(runtimehost.HostBuildContext{
		Descriptor: descriptor,
		Paths:      paths,
	})
	if err != nil {
		return nil, fmt.Errorf("create runtime host: %w", err)
	}

	nodeResolver, err := nodeenv.NewResolver(nodeenv.ResolveContext{
		Config: config.AppCfg,
		Host:   host,
	})
	if err != nil {
		return nil, fmt.Errorf("create node environment resolver: %w", err)
	}

	return &RuntimeDependencies{
		Host:         host,
		NodeResolver: nodeResolver,
		Config:       config.AppCfg,
	}, nil
}

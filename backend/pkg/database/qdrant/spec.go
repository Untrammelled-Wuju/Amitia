// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrant

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	qdrantenv "github.com/u-ai/backend/internal/vectorstore/qdrantenv"
	"github.com/u-ai/backend/internal/vectorstore/qdrantlayout"
	"github.com/u-ai/backend/internal/vectorstore/qdrantconfig"
	"github.com/u-ai/backend/pkg/util"
)

// BuildQdrantProcessSpec prepares the Qdrant ProcessSpec using the runtimehost to resolve binary paths.
// It uses qdrantlayout to determine all directory paths and qdrantconfig to generate the Qdrant config.
func BuildQdrantProcessSpec(host runtimehost.RuntimeHost) (runtimehost.ProcessSpec, error) {
	if host == nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("BuildQdrantProcessSpec: host is nil")
	}
	if config.AppCfg == nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("BuildQdrantProcessSpec: config.AppCfg is nil")
	}

	instanceID := host.RuntimeInstanceID()
	if instanceID == "" {
		instanceID = "qdrant-unsigned"
	}

	vectorCfg := &config.AppCfg.Providers.VectorStore

	envResolver, err := qdrantenv.NewResolver(qdrantenv.ResolveContext{
		Config: config.AppCfg,
		Host:   host,
	})
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create qdrantenv resolver: %w", err)
	}

	env, err := envResolver.Resolve(context.Background())
	if err != nil {
		if errors.Is(err, qdrantenv.ErrQdrantBinaryNotInstalled) && !env.Explicit {
			installer := qdrantenv.NewInstaller(nil, util.UnzipFile)
			if instErr := installer.EnsureInstalled(qdrantenv.InstallRequest{Target: env}); instErr != nil {
				return runtimehost.ProcessSpec{}, fmt.Errorf("ensure qdrant installed: %w", instErr)
			}
			envResolver.Invalidate()
			env, err = envResolver.Resolve(context.Background())
		}
		if err != nil {
			return runtimehost.ProcessSpec{}, fmt.Errorf("resolve qdrant environment: %w", err)
		}
	}

	if env.BinaryPath == "" {
		return runtimehost.ProcessSpec{}, fmt.Errorf("BuildQdrantProcessSpec: resolved binary path is empty")
	}
	if env.DistributionRoot == "" {
		return runtimehost.ProcessSpec{}, fmt.Errorf("BuildQdrantProcessSpec: resolved distribution root is empty")
	}

	layoutResolver, err := qdrantlayout.NewResolver(qdrantlayout.ResolveContext{
		Config: config.AppCfg,
		Host:   host,
	})
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create layout resolver: %w", err)
	}

	layout, err := layoutResolver.Resolve(context.Background(), env)
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("resolve layout: %w", err)
	}

	dm := qdrantlayout.NewDirectoryManager(nil)
	if err := dm.Ensure(context.Background(), layout); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("ensure directories: %w", err)
	}

	doc := qdrantconfig.Document{
		HTTPPort:     vectorCfg.Qdrant.Port,
		GRPCPort:     vectorCfg.Qdrant.Port + 1,
		StoragePath:  layout.StorageDir,
		SnapshotPath: layout.SnapshotsDir,
	}

	renderer := qdrantconfig.NewRenderer()
	configBytes, err := renderer.Render(doc)
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("render config: %w", err)
	}

	writer := qdrantconfig.NewWriter(nil)
	if err := writer.Write(context.Background(), layout.ConfigPath, configBytes); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("write config: %w", err)
	}

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/readyz", vectorCfg.Qdrant.Port)

	return runtimehost.ProcessSpec{
		ID:         runtimehost.ProcessIDQdrant,
		Executable: env.BinaryPath,
		Args:       []string{"--config-path", layout.ConfigPath},
		WorkingDir: env.DistributionRoot,
		Environment: runtimehost.EnvironmentSpec{
			Policy: runtimehost.EnvPolicyMinimal,
			Values: map[string]string{
				"AMITIA_PROCESS_ID":          "amitia.qdrant",
				"AMITIA_RUNTIME_INSTANCE_ID": instanceID,
			},
		},
		Ports: []runtimehost.LoopbackPortClaim{
			{Host: "127.0.0.1", Port: vectorCfg.Qdrant.Port, Protocol: "tcp"},
		},
		StartupTimeout:  runtimehost.DefaultStartupTimeout,
		StopGracePeriod: runtimehost.DefaultStopGracePeriod,
		HealthProbe:     runtimehost.NewHTTPHealthProbe(healthURL, 5*time.Second),
		HealthInterval:  runtimehost.DefaultHealthInterval,
		RestartPolicy: runtimehost.RestartPolicy{
			Mode:        runtimehost.RestartOnFailure,
			MaxRestarts: 5,
			BaseDelay:   2 * time.Second,
			MaxDelay:    30 * time.Second,
			ResetAfter:  5 * time.Minute,
		},
	}, nil
}

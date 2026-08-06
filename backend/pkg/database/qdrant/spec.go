// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrant

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	qdrantenv "github.com/u-ai/backend/internal/vectorstore/qdrantenv"
	"github.com/u-ai/backend/pkg/util"
)

// BuildQdrantProcessSpec prepares the Qdrant ProcessSpec using the runtimehost to resolve binary paths via qdrantenv.
// It does NOT start Qdrant; the caller (typically a ProcessSupervisor-based adapter) handles execution and lifecycle.
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

	resolver, err := qdrantenv.NewResolver(qdrantenv.ResolveContext{
		Config: config.AppCfg,
		Host:   host,
	})
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create qdrantenv resolver: %w", err)
	}

	env, err := resolver.Resolve(context.Background())
	if err != nil {
		if errors.Is(err, qdrantenv.ErrQdrantBinaryNotInstalled) && !env.Explicit {
			installer := qdrantenv.NewInstaller(nil, util.UnzipFile)
			if instErr := installer.EnsureInstalled(qdrantenv.InstallRequest{Target: env}); instErr != nil {
				return runtimehost.ProcessSpec{}, fmt.Errorf("ensure qdrant installed: %w", instErr)
			}
			resolver.Invalidate()
			env, err = resolver.Resolve(context.Background())
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

	qdrantDir := env.DistributionRoot
	configDir := filepath.Join(qdrantDir, "config")
	configPath := filepath.Join(configDir, "config.yaml")

	if mkErr := os.MkdirAll(configDir, 0755); mkErr != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create qdrant config dir: %w", mkErr)
	}

	configContent := fmt.Sprintf("service:\n  http_port: %d\n  grpc_port: %d\nstorage:\n  storage_path: %s\n",
		vectorCfg.Qdrant.Port, vectorCfg.Qdrant.Port+1, resolveQdrantDataDir(qdrantDir))
	if wErr := os.WriteFile(configPath, []byte(configContent), 0644); wErr != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("write qdrant config: %w", wErr)
	}

	dataDir := resolveQdrantDataDir(qdrantDir)
	if mkErr := os.MkdirAll(dataDir, 0755); mkErr != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create qdrant data dir: %w", mkErr)
	}

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/readyz", vectorCfg.Qdrant.Port)

	return runtimehost.ProcessSpec{
		ID:         runtimehost.ProcessIDQdrant,
		Executable: env.BinaryPath,
		Args:       []string{"--config-path", configPath},
		WorkingDir: qdrantDir,
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

func resolveQdrantDataDir(qdrantDir string) string {
	if cfgDir := config.AppCfg.Providers.VectorStore.Qdrant.DataDir; cfgDir != "" {
		if filepath.IsAbs(cfgDir) {
			return cfgDir
		}
		return filepath.Join(qdrantDir, cfgDir)
	}
	return filepath.Join(qdrantDir, "data")
}

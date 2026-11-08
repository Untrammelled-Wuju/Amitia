// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrant

import (
	"context"
	"errors"
	"fmt"
	amitialog "log"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/vectorstore/qdrantconfig"
	qdrantenv "github.com/u-ai/backend/internal/vectorstore/qdrantenv"
	"github.com/u-ai/backend/internal/vectorstore/qdrantlayout"
	"github.com/u-ai/backend/internal/vectorstore/qdrantprofile"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type qdrantDescriptorProvider struct {
	host runtimehost.RuntimeHost
}

func (p qdrantDescriptorProvider) Descriptor() platform.RuntimeDescriptor {
	return p.host.Descriptor()
}

func (p qdrantDescriptorProvider) Capabilities() *runtimehost.HostCapabilities {
	return p.host.Capabilities()
}

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
		Config:    config.AppCfg,
		Host:      host,
		Inspector: nil,
	})
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create qdrantlayout resolver: %w", err)
	}

	layout, err := layoutResolver.Resolve(context.Background(), env)
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("resolve qdrant layout: %w", err)
	}

	if err := qdrantlayout.NewDirectoryManager(nil).Ensure(context.Background(), layout); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("ensure qdrant directories: %w", err)
	}

	profileResolver, err := qdrantprofile.NewResolver(qdrantprofile.ResolveContext{
		DescriptorProvider: qdrantDescriptorProvider{host: host},
	})
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create qdrant profile resolver: %w", err)
	}
	resolvedProfile, err := profileResolver.Resolve(context.Background(), vectorCfg.Qdrant.ResourceProfile)
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("resolve qdrant profile: %w", err)
	}

	doc := qdrantconfig.Document{
		HTTPPort:     vectorCfg.Qdrant.Port,
		GRPCPort:     vectorCfg.Qdrant.Port + 1,
		StoragePath:  layout.StorageDir,
		SnapshotPath: layout.SnapshotsDir,
	}
	if resolvedProfile.Mobile {
		doc.ResourceProfile = resolvedProfile.Settings
	}

	configBytes, err := qdrantconfig.NewRenderer().Render(doc)
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("render qdrant config: %w", err)
	}

	if err := qdrantconfig.NewWriter(nil).Write(context.Background(), layout.ConfigPath, configBytes); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("write qdrant config: %w", err)
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
		OnStdout: func(line string) {
			amitialog.Printf("[qdrant-stdout] %s", line)
		},
		OnStderr: func(line string) {
			amitialog.Printf("[qdrant-stderr] %s", line)
		},
		RestartPolicy: runtimehost.RestartPolicy{
			Mode:        runtimehost.RestartOnFailure,
			MaxRestarts: 5,
			BaseDelay:   2 * time.Second,
			MaxDelay:    30 * time.Second,
			ResetAfter:  5 * time.Minute,
		},
	}, nil
}

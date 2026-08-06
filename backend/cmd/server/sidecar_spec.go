// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/scriptruntime/nodeenv"
	"github.com/u-ai/backend/internal/scriptruntime/sidecar"
	"github.com/u-ai/backend/pkg/platform"
)

func buildWeChatSidecarSpec(
	instanceID string,
	nodeResolver nodeenv.Resolver,
	artifactResolver sidecar.ArtifactResolver,
) (runtimehost.ProcessSpec, error) {
	return buildSidecarSpec(instanceID, nodeResolver, artifactResolver, sidecar.KindWeChat, runtimehost.ProcessIDSidecarWeChat, config.AppCfg.Components.Sidecars.Wechat.Port, config.AppCfg.Components.Sidecars.Wechat.HealthURL, 19876)
}

func buildQQSidecarSpec(
	instanceID string,
	nodeResolver nodeenv.Resolver,
	artifactResolver sidecar.ArtifactResolver,
) (runtimehost.ProcessSpec, error) {
	return buildSidecarSpec(instanceID, nodeResolver, artifactResolver, sidecar.KindQQ, runtimehost.ProcessIDSidecarQQ, config.AppCfg.Components.Sidecars.QQ.Port, config.AppCfg.Components.Sidecars.QQ.HealthURL, 19877)
}

func buildSidecarSpec(
	instanceID string,
	nodeResolver nodeenv.Resolver,
	artifactResolver sidecar.ArtifactResolver,
	kind sidecar.Kind,
	processID runtimehost.ProcessID,
	port int,
	healthURL string,
	defaultPort int,
) (runtimehost.ProcessSpec, error) {
	ctx := context.Background()

	nodeEnv, err := nodeResolver.Resolve(ctx)
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("resolve node environment: %w", err)
	}

	artifact, err := artifactResolver.Resolve(ctx, kind)
	if err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("resolve sidecar artifact: %w", err)
	}

	if nodeEnv.NodeBinary == "" {
		return runtimehost.ProcessSpec{}, fmt.Errorf("managed node binary not available")
	}
	if artifact.EntryPath == "" {
		return runtimehost.ProcessSpec{}, fmt.Errorf("sidecar entry path not resolved")
	}
	if artifact.WorkingDir == "" {
		return runtimehost.ProcessSpec{}, fmt.Errorf("sidecar working directory not resolved")
	}

	if port <= 0 {
		port = defaultPort
	}
	if healthURL == "" {
		healthURL = fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	}

	args := make([]string, 0, len(artifact.ArgsPrefix)+1)
	args = append(args, artifact.ArgsPrefix...)
	args = append(args, artifact.EntryPath)

	return runtimehost.ProcessSpec{
		ID:         processID,
		Executable: nodeEnv.NodeBinary,
		Args:       args,
		WorkingDir: artifact.WorkingDir,
		Environment: runtimehost.EnvironmentSpec{
			Policy: runtimehost.EnvPolicyMinimal,
			Values: map[string]string{
				"AMITIA_HOST_PLATFORM": string(platform.Get().Descriptor().Host),
			},
		},
		Ports: []runtimehost.LoopbackPortClaim{
			{Host: "127.0.0.1", Port: port, Protocol: "tcp"},
		},
		StartupTimeout:  runtimehost.DefaultStartupTimeout,
		StopGracePeriod: runtimehost.DefaultStopGracePeriod,
		HealthProbe:     runtimehost.NewHTTPHealthProbe(healthURL, 5*time.Second),
		HealthInterval:  runtimehost.DefaultHealthInterval,
		RestartPolicy: runtimehost.RestartPolicy{
			Mode:        runtimehost.RestartOnFailure,
			MaxRestarts: 10,
			BaseDelay:   2 * time.Second,
			MaxDelay:    30 * time.Second,
			ResetAfter:  60 * time.Second,
		},
	}, nil
}

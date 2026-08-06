// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdrant

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

// BuildQdrantProcessSpec prepares the Qdrant ProcessSpec but does NOT start Qdrant.
// The caller (typically a runtimehost.ProcessSupervisor-based provider) handles actual execution.
func BuildQdrantProcessSpec(instanceID string) (runtimehost.ProcessSpec, error) {
	if instanceID == "" {
		instanceID = "qdrant-unsigned"
	}
	cfg := config.AppCfg.Providers.VectorStore.Qdrant
	workDir := util.RuntimeRoot()
	qdrantDir := filepath.Join(workDir, "qdrant")
	configDir := filepath.Join(qdrantDir, "config")
	configPath := filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create qdrant config dir: %w", err)
	}

	configContent := fmt.Sprintf("service:\n  http_port: %d\n  grpc_port: %d\nstorage:\n  storage_path: %s\n",
		cfg.Port, cfg.Port+1, resolveQdrantDataDir(qdrantDir))
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("write qdrant config: %w", err)
	}

	qdrantPath := resolveQdrantBinaryPath(qdrantDir)
	if _, err := os.Stat(qdrantPath); os.IsNotExist(err) {
		if err := ensureQdrantBinary(qdrantPath, qdrantDir); err != nil {
			return runtimehost.ProcessSpec{}, err
		}
	}

	dataDir := resolveQdrantDataDir(qdrantDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create qdrant data dir: %w", err)
	}

	workingDir := resolveQdrantWorkDir(qdrantDir)
	if !filepath.IsAbs(workingDir) {
		return runtimehost.ProcessSpec{}, fmt.Errorf("working_dir must be absolute: %s", workingDir)
	}
	if !filepath.IsAbs(qdrantPath) {
		return runtimehost.ProcessSpec{}, fmt.Errorf("executable must be absolute: %s", qdrantPath)
	}

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/readyz", cfg.Port)

	return runtimehost.ProcessSpec{
		ID:         runtimehost.ProcessIDQdrant,
		Executable: qdrantPath,
		Args:       []string{"--config-path", configPath},
		WorkingDir: workingDir,
		Environment: runtimehost.EnvironmentSpec{
			Policy: runtimehost.EnvPolicyMinimal,
			Values: map[string]string{
				"AMITIA_PROCESS_ID":          "amitia.qdrant",
				"AMITIA_RUNTIME_INSTANCE_ID": instanceID,
				"AMITIA_HOST_PLATFORM":       string(platform.Get().Descriptor().Host),
			},
		},
		Ports: []runtimehost.LoopbackPortClaim{
			{Host: "127.0.0.1", Port: cfg.Port, Protocol: "tcp"},
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
		OnStdout: func(line string) {
			if line != "" {
				log.Info("[Qdrant]", line)
			}
		},
		OnStderr: func(line string) {
			if line != "" {
				log.Error("[Qdrant]", line)
			}
		},
	}, nil
}

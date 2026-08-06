// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package surrealdb

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

// BuildSurrealProcessSpec prepares the SurrealDB ProcessSpec but does NOT start SurrealDB.
// The caller (typically a runtimehost.ProcessSupervisor-based provider) handles actual execution.
func BuildSurrealProcessSpec(instanceID string) (runtimehost.ProcessSpec, error) {
	if instanceID == "" {
		instanceID = "surrealdb-unsigned"
	}
	cfg := config.AppCfg.Providers.GraphStore.SurrealDB
	workDir := util.RuntimeRoot()
	surrealDir := filepath.Join(workDir, "surrealdb")

	p := platform.Get()
	binaryName := "surreal" + p.ExecutableSuffix()
	surrealPath := filepath.Join(surrealDir, binaryName)

	if _, err := os.Stat(surrealPath); os.IsNotExist(err) {
		zipCandidates := []string{"surreal.zip", "surreal.exe.zip"}
		for _, zip := range zipCandidates {
			zipPath := filepath.Join(surrealDir, zip)
			if _, err := os.Stat(zipPath); err == nil {
				if err := util.UnzipFile(zipPath, surrealDir); err == nil {
					break
				}
			}
		}
	}

	// Ensure data directory exists for file-based storage
	dataPath := filepath.Join(surrealDir, "data")
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return runtimehost.ProcessSpec{}, fmt.Errorf("create surrealdb data dir: %w", err)
	}

	if !filepath.IsAbs(surrealDir) {
		return runtimehost.ProcessSpec{}, fmt.Errorf("working_dir must be absolute: %s", surrealDir)
	}
	if !filepath.IsAbs(surrealPath) {
		return runtimehost.ProcessSpec{}, fmt.Errorf("executable must be absolute: %s", surrealPath)
	}

	healthURL := fmt.Sprintf("http://127.0.0.1:%d/health", cfg.Port)

	args := []string{
		"start",
		"surrealkv:" + dataPath,
		"--bind", fmt.Sprintf("0.0.0.0:%d", cfg.Port),
		"--user", cfg.Username,
		"--pass", cfg.Password,
	}

	// Password is at len(args)-1
	sensitiveIdx := len(args) - 1

	return runtimehost.ProcessSpec{
		ID:         runtimehost.ProcessIDSurrealDB,
		Executable: surrealPath,
		Args:       args,
		WorkingDir: surrealDir,
		Environment: runtimehost.EnvironmentSpec{
			Policy: runtimehost.EnvPolicyMinimal,
			Values: map[string]string{
				"AMITIA_PROCESS_ID":          "amitia.surrealdb",
				"AMITIA_RUNTIME_INSTANCE_ID": instanceID,
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
		SensitiveArgIndexes: []int{sensitiveIdx},
	}, nil
}

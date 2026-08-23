// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package surrealdb

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	bindHost, err := normalizeLoopbackHost(cfg.Host)
	if err != nil {
		return runtimehost.ProcessSpec{}, err
	}
	if err := validateSurrealCredentials(cfg.Username, cfg.Password); err != nil {
		return runtimehost.ProcessSpec{}, err
	}
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

	bindAddress := net.JoinHostPort(bindHost, fmt.Sprintf("%d", cfg.Port))
	healthURL := "http://" + bindAddress + "/health"

	args := []string{
		"start",
		"surrealkv:" + dataPath,
		"--bind", bindAddress,
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
			{Host: bindHost, Port: cfg.Port, Protocol: "tcp"},
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

func normalizeLoopbackHost(raw string) (string, error) {
	host := strings.TrimSpace(strings.Trim(raw, "[]"))
	if host == "" || strings.EqualFold(host, "localhost") {
		return "127.0.0.1", nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("surrealdb host must be loopback, got %q", raw)
	}
	return ip.String(), nil
}

func validateSurrealCredentials(username, password string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("surrealdb username must not be empty")
	}
	trimmed := strings.TrimSpace(password)
	lower := strings.ToLower(trimmed)
	if len(trimmed) < 24 || lower == "root" || lower == "admin" || lower == "password" {
		return fmt.Errorf("surrealdb password is missing or insecure; configure a per-installation random password of at least 24 characters")
	}
	return nil
}

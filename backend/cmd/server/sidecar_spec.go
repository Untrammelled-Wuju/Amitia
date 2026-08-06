// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

func buildWeChatSidecarSpec(instanceID string) (runtimehost.ProcessSpec, error) {
	runtimeRoot := util.RuntimeRoot()
	bundledWX := filepath.Join(runtimeRoot, "sidecar", "bundle.mjs")
	sourceWX := filepath.Join(runtimeRoot, "sidecar", "src", "index.ts")
	_, wxOk := os.Stat(bundledWX)
	_, wxSourceOk := os.Stat(sourceWX)
	inBundle := wxOk == nil && wxSourceOk == nil

	if !inBundle {
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			exeBundledWX := filepath.Join(exeDir, "sidecar", "bundle.mjs")
			if _, sErr := os.Stat(exeBundledWX); sErr == nil {
				inBundle = true
			}
		}
	}

	var (
		workingDir string
		executable string
		args       []string
	)
	if inBundle {
		bundledRoot := runtimeRoot
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			if _, sErr := os.Stat(filepath.Join(exeDir, "sidecar", "bundle.mjs")); sErr == nil {
				bundledRoot = exeDir
			}
		}
		workingDir = filepath.Join(bundledRoot, "sidecar")
		executable = bundledNodePath(bundledRoot)
		args = []string{"launcher.mjs"}
	} else {
		workspace := util.RuntimeWorkspaceDir(runtimeRoot)
		workingDir = filepath.Join(workspace, "backend", "sidecar")
		executable = filepath.Join(workspace, "backend", "node", "node.exe")
		if runtime.GOOS != "windows" {
			executable = filepath.Join(workspace, "backend", "node", "node")
		}
		args = []string{"node_modules/tsx/dist/cli.mjs", "src/index.ts"}
	}

	port := config.AppCfg.Components.Sidecars.Wechat.Port
	if port <= 0 {
		port = 19876
	}
	healthURL := config.AppCfg.Components.Sidecars.Wechat.HealthURL
	if healthURL == "" {
		healthURL = fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	}

	return runtimehost.ProcessSpec{
		ID:         runtimehost.ProcessIDSidecarWeChat,
		Executable: executable,
		Args:       args,
		WorkingDir: workingDir,
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

func buildQQSidecarSpec(instanceID string) (runtimehost.ProcessSpec, error) {
	runtimeRoot := util.RuntimeRoot()
	bundledQQ := filepath.Join(runtimeRoot, "qq-sidecar", "bundle.mjs")
	sourceQQ := filepath.Join(runtimeRoot, "qq-sidecar", "src", "index.ts")
	_, qqOk := os.Stat(bundledQQ)
	_, qqSourceOk := os.Stat(sourceQQ)
	inBundle := qqOk == nil && qqSourceOk == nil

	if !inBundle {
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			exeBundledQQ := filepath.Join(exeDir, "qq-sidecar", "bundle.mjs")
			if _, sErr := os.Stat(exeBundledQQ); sErr == nil {
				inBundle = true
			}
		}
	}

	var (
		workingDir string
		executable string
		args       []string
	)
	if inBundle {
		bundledRoot := runtimeRoot
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			if _, sErr := os.Stat(filepath.Join(exeDir, "qq-sidecar", "bundle.mjs")); sErr == nil {
				bundledRoot = exeDir
			}
		}
		workingDir = filepath.Join(bundledRoot, "qq-sidecar")
		executable = bundledNodePath(bundledRoot)
		args = []string{"launcher.mjs"}
	} else {
		workspace := util.RuntimeWorkspaceDir(runtimeRoot)
		workingDir = filepath.Join(workspace, "backend", "qq-sidecar")
		executable = filepath.Join(workspace, "backend", "node", "node.exe")
		if runtime.GOOS != "windows" {
			executable = filepath.Join(workspace, "backend", "node", "node")
		}
		args = []string{"node_modules/tsx/dist/cli.mjs", "src/index.ts"}
	}

	port := config.AppCfg.Components.Sidecars.QQ.Port
	if port <= 0 {
		port = 19877
	}
	healthURL := config.AppCfg.Components.Sidecars.QQ.HealthURL
	if healthURL == "" {
		healthURL = fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	}

	return runtimehost.ProcessSpec{
		ID:         runtimehost.ProcessIDSidecarQQ,
		Executable: executable,
		Args:       args,
		WorkingDir: workingDir,
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

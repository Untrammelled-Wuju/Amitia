// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package surrealdb

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

// SetSurrealShuttingDown is a no-op. Process shutdown is managed by ProcessSupervisor.
func SetSurrealShuttingDown() {}

// IsSurrealShuttingDown always returns false. Process lifecycle is managed by ProcessSupervisor.
func IsSurrealShuttingDown() bool { return false }

// SetSurrealRestartCallback is a no-op. Restart callbacks are managed by ProcessSupervisor.
func SetSurrealRestartCallback(_ func()) {}

// StartSurreal is deprecated. Use BuildSurrealProcessSpec and the runtimehost.ProcessSupervisor.
func StartSurreal() error {
	return errors.New("surrealdb: use BuildSurrealProcessSpec instead")
}

// StopSurreal is deprecated. Use runtimehost.ProcessSupervisor.Stop.
func StopSurreal() {}

// StartSurrealMonitor is a no-op. Health monitoring is handled by ProcessSupervisor.
func StartSurrealMonitor() {}

// StopSurrealMonitor is a no-op. Health monitoring is handled by ProcessSupervisor.
func StopSurrealMonitor() {}

type surrealWriter struct{}

func (w *surrealWriter) Write(p []byte) (int, error) {
	lines := string(p)
	for len(lines) > 0 && (lines[len(lines)-1] == '\n' || lines[len(lines)-1] == '\r') {
		lines = lines[:len(lines)-1]
	}
	if lines != "" {
		log.Info("[SurrealDB]", lines)
	}
	return len(p), nil
}

func resolveSurrealBinaryPath(surrealDir string) string {
	if cfgPath := config.AppCfg.Providers.GraphStore.SurrealDB.BinaryPath; cfgPath != "" {
		if filepath.IsAbs(cfgPath) {
			return cfgPath
		}
		return filepath.Join(surrealDir, cfgPath)
	}

	p := platform.Get()
	if rootfs := p.RootFSDir(); rootfs != "" && !p.IsWindows() {
		binName := "surreal" + p.BinarySuffix()
		rootfsPath := filepath.Join(rootfs, "bin", binName)
		if _, err := os.Stat(rootfsPath); err == nil {
			return rootfsPath
		}
		if IsLinuxARM64() {
			candidates := []string{
				filepath.Join(rootfs, "bin", "surreal_linux_aarch64"),
				filepath.Join(rootfs, "bin", "surreal"),
				filepath.Join(rootfs, "surreal"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					return c
				}
			}
		}
		if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
			candidates := []string{
				filepath.Join(rootfs, "bin", "surreal_linux_x86"),
				filepath.Join(rootfs, "bin", "surreal"),
				filepath.Join(rootfs, "surreal"),
			}
			for _, c := range candidates {
				if _, err := os.Stat(c); err == nil {
					return c
				}
			}
		}
	}

	suffix := p.ExecutableSuffix()
	defaultPath := filepath.Join(surrealDir, "surreal"+suffix)
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath
	}

	if IsLinuxARM64() {
		candidates := []string{
			filepath.Join(surrealDir, "surreal_linux_aarch64"),
			filepath.Join(surrealDir, "surreal"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		candidates := []string{
			filepath.Join(surrealDir, "surreal_linux_x86"),
			filepath.Join(surrealDir, "surreal"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	return defaultPath
}

func resolveSurrealWorkDir(surrealDir string) string {
	p := platform.Get()
	if rootfs := p.RootFSDir(); rootfs != "" && !p.IsWindows() {
		binDir := filepath.Join(rootfs, "bin")
		if info, err := os.Stat(binDir); err == nil && info.IsDir() {
			return binDir
		}
	}
	return surrealDir
}

// IsLinuxARM64 reports whether the current platform is Linux ARM64.
func IsLinuxARM64() bool {
	return runtime.GOOS == "linux" && runtime.GOARCH == "arm64"
}

func ensureSurrealBinary(surrealPath, surrealDir string) error {
	if _, err := os.Stat(surrealPath); err == nil {
		return nil
	}

	candidates := []string{"surreal.exe.zip", "surreal.zip"}
	if IsLinuxARM64() {
		candidates = []string{"surreal_linux_aarch64.zip", "surreal-arm64.zip", "surreal.zip"}
	} else if runtime.GOOS == "linux" {
		candidates = []string{"surreal_linux_x86.zip", "surreal-x86_64.zip", "surreal.zip"}
	}

	for _, name := range candidates {
		zipPath := filepath.Join(surrealDir, name)
		if _, err := os.Stat(zipPath); err == nil {
			log.Info("正在解压SurrealDB程序", "zip", zipPath)
			if err := util.UnzipFile(zipPath, surrealDir); err != nil {
				return fmt.Errorf("解压SurrealDB程序失败: %w", err)
			}
			if _, err := os.Stat(surrealPath); err == nil {
				return nil
			}
			return fmt.Errorf("SurrealDB压缩包中未找到程序: %s", surrealPath)
		}
	}

	return fmt.Errorf("SurrealDB程序不存在: %s", surrealPath)
}

// WaitForSurreal polls the SurrealDB health endpoint until it responds OK.
func WaitForSurreal(port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := http.Client{Timeout: 500 * time.Millisecond}
	for i := 0; i < 60; i++ {
		time.Sleep(500 * time.Millisecond)
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			log.Info("SurrealDB端口就绪", "port", port)
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return fmt.Errorf("等待SurrealDB启动超时(30s)")
}

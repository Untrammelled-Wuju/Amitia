// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package surrealdb

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

var surrealCmd *exec.Cmd
var surrealMu sync.Mutex
var surrealMonitorStop chan struct{}
var surrealRestartFn func()

func SetSurrealRestartCallback(fn func()) {
	surrealRestartFn = fn
}

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

func killExistingSurreal(port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return
	}
	conn.Close()

	log.Warn("检测到旧SurrealDB进程，正在终止...")
	if runtime.GOOS == "windows" {
		exec.Command("taskkill", "/F", "/IM", "surreal.exe").Run()
	} else {
		exec.Command("pkill", "-9", "surreal").Run()
	}
	time.Sleep(2 * time.Second)

	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err != nil {
			log.Info("旧SurrealDB已释放端口", port)
			return
		}
		conn.Close()
		time.Sleep(1 * time.Second)
	}
	log.Warn("旧SurrealDB未能在10秒内释放端口，继续启动...")
}

func resolveSurrealBinaryPath(surrealDir string) string {
	if cfgPath := config.AppCfg.Surreal.BinaryPath; cfgPath != "" {
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

func StartSurreal() error {
	surrealMu.Lock()
	defer surrealMu.Unlock()

	return startSurrealInternal()
}

func startSurrealInternal() error {
	cfg := config.AppCfg.Surreal
	workDir := util.RuntimeRoot()

	killExistingSurreal(cfg.Port)

	surrealDir := filepath.Join(workDir, "surrealdb")

	surrealPath := resolveSurrealBinaryPath(surrealDir)

	if _, err := os.Stat(surrealPath); os.IsNotExist(err) {
		if err := ensureSurrealBinary(surrealPath, surrealDir); err != nil {
			return err
		}
	}

	bindAddr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	storagePath := cfg.DataPath
	if storagePath != "" && storagePath != "memory" {
		absPath := util.ResolveRuntimePath(workDir, storagePath)
		os.MkdirAll(absPath, 0755)
		storagePath = "surrealkv:" + absPath
	}

	cmd := exec.Command(surrealPath, "start",
		"--log", "info",
		"--user", cfg.Username,
		"--pass", cfg.Password,
		"--bind", bindAddr,
		storagePath,
	)
	cmd.Dir = resolveSurrealWorkDir(surrealDir)
	cmd.Stdout = &surrealWriter{}
	cmd.Stderr = &surrealWriter{}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动SurrealDB失败: %w", err)
	}

	surrealCmd = cmd
	log.Info("SurrealDB已启动", "port", cfg.Port, "pid", cmd.Process.Pid)
	return nil
}

func isSurrealAlive(port int) bool {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func StartSurrealMonitor() {
	if surrealMonitorStop != nil {
		return
	}
	surrealMonitorStop = make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Warn("SurrealDB监控协程异常恢复:", r)
			}
		}()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				surrealMu.Lock()
				needRestart := false
				if surrealCmd == nil || surrealCmd.Process == nil {
					needRestart = true
				} else if !isSurrealAlive(config.AppCfg.Surreal.Port) {
					needRestart = true
				}
				if needRestart {
					log.Warn("检测到SurrealDB进程异常，尝试重启...")
					if err := startSurrealInternal(); err != nil {
						log.Error("SurrealDB重启失败:", err)
						surrealMu.Unlock()
						continue
					}
					if err := WaitForSurreal(config.AppCfg.Surreal.Port); err != nil {
						log.Error("等待SurrealDB就绪超时:", err)
						surrealMu.Unlock()
						continue
					}
					log.Info("SurrealDB已自动恢复")
					surrealMu.Unlock()
					if surrealRestartFn != nil {
						surrealRestartFn()
					}
				} else {
					surrealMu.Unlock()
				}
			case <-surrealMonitorStop:
				return
			}
		}
	}()
	log.Info("SurrealDB进程监控已启动")
}

func StopSurrealMonitor() {
	if surrealMonitorStop != nil {
		close(surrealMonitorStop)
		surrealMonitorStop = nil
	}
}

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

func StopSurreal() {
	StopSurrealMonitor()
	surrealMu.Lock()
	defer surrealMu.Unlock()

	if surrealCmd == nil || surrealCmd.Process == nil {
		return
	}
	_ = surrealCmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = surrealCmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = surrealCmd.Process.Kill()
		log.Warn("强制终止SurrealDB进程(超时)")
	}
	log.Info("SurrealDB已停止")
}

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

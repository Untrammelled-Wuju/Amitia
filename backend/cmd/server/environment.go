// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/u-ai/backend/pkg/util"
)

var envShuttingDown atomic.Bool

func SetEnvShuttingDown() {
	envShuttingDown.Store(true)
}

func IsEnvShuttingDown() bool {
	return envShuttingDown.Load()
}

type Service struct {
	Name          string
	Dir           string
	Cmd           string
	Args          []string
	Env           []string
	Port          int
	cmd           *exec.Cmd
	cancel        context.CancelFunc
	ctx           context.Context
	HealthURL     string
	stopped       bool
	restartCount  int
	maxRestarts   int
	lastRestartAt time.Time
}

type Environment struct {
	services   []*Service
	workspace  string
	wg         sync.WaitGroup
	onShutdown func()
}

func NewEnvironment(workspace string) *Environment {
	return &Environment{workspace: workspace}
}

func (e *Environment) SetOnShutdown(fn func()) {
	e.onShutdown = fn
}

func (e *Environment) AddService(name, dir, cmd string, args []string, port int, env []string) {
	e.services = append(e.services, &Service{
		Name:        name,
		Dir:         filepath.Join(e.workspace, dir),
		Cmd:         cmd,
		Args:        args,
		Env:         env,
		Port:        port,
		HealthURL:   fmt.Sprintf("http://127.0.0.1:%d/api/health", port),
		maxRestarts: 10,
	})
}

func (e *Environment) StartAll() {
	for _, svc := range e.services {
		go func(s *Service) {
			if err := e.startService(s); err != nil {
				log.Printf("[Env] %s 启动失败: %v", s.Name, err)
			}
		}(svc)
	}
}

func (e *Environment) startService(svc *Service) error {
	if svc.Port > 0 {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", svc.Port), 2*time.Second); err == nil {
			conn.Close()
			log.Printf("[Env] %s 端口 %d 被占用，正在终止旧进程...", svc.Name, svc.Port)
			killByPort(svc.Port)
			time.Sleep(1 * time.Second)
		}
	}

	if _, err := os.Stat(svc.Dir); os.IsNotExist(err) {
		log.Printf("[Env] %s 目录不存在，跳过: %s", svc.Name, svc.Dir)
		return nil
	}

	log.Printf("[Env] 正在启动 %s (端口 %d)...", svc.Name, svc.Port)

	ctx, cancel := context.WithCancel(context.Background())
	svc.cancel = cancel
	svc.ctx = ctx
	svc.cmd = exec.CommandContext(ctx, svc.Cmd, svc.Args...)
	svc.cmd.Dir = svc.Dir
	svc.cmd.Stdout = &serviceWriter{prefix: svc.Name}
	svc.cmd.Stderr = &serviceWriter{prefix: svc.Name}
	svc.cmd.Env = os.Environ()
	if svc.Env != nil {
		svc.cmd.Env = append(svc.cmd.Env, svc.Env...)
	}

	if err := svc.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("无法启动进程: %w", err)
	}

	if svc.Port > 0 {
		if err := e.waitForHealthy(svc); err != nil {
			cancel()
			return fmt.Errorf("健康检查失败: %w", err)
		}
	}

	log.Printf("[Env] %s 已就绪 (pid=%d)", svc.Name, svc.cmd.Process.Pid)

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		for {
			svc.cmd.Wait()
			log.Printf("[Env] %s 已退出", svc.Name)
			if svc.stopped {
				return
			}
			if IsEnvShuttingDown() {
				log.Printf("[Env] %s 检测到全局关闭标志，停止保活", svc.Name)
				return
			}
			svc.restartCount++
			if svc.restartCount > svc.maxRestarts {
				log.Printf("[Env] %s 已达最大重启次数 %d，停止保活", svc.Name, svc.maxRestarts)
				return
			}
			backoff := time.Duration(svc.restartCount) * 2 * time.Second
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			svc.lastRestartAt = time.Now()
			log.Printf("[Env] %s 第 %d 次自动重启，等待 %v 后拉起...", svc.Name, svc.restartCount, backoff)
			select {
			case <-time.After(backoff):
			case <-svc.ctx.Done():
				return
			}
			if svc.stopped {
				return
			}
			if IsEnvShuttingDown() {
				log.Printf("[Env] %s 检测到全局关闭标志，取消重启", svc.Name)
				return
			}
			if err := e.restartService(svc); err != nil {
				log.Printf("[Env] %s 重启失败: %v", svc.Name, err)
				continue
			}
			log.Printf("[Env] %s 重启成功 (第 %d 次)", svc.Name, svc.restartCount)
		}
	}()

	return nil
}

func killByPort(port int) {
	out, _ := exec.Command("cmd", "/c", "netstat -ano | findstr :"+strconv.Itoa(port)+" | findstr LISTENING").Output()
	fields := strings.Fields(string(out))
	for _, f := range fields {
		if pid, err := strconv.Atoi(f); err == nil {
			if pid != os.Getpid() {
				log.Printf("[Env] 终止旧进程 PID=%d", pid)
				exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
			}
		}
	}
}

func (e *Environment) restartService(svc *Service) error {
	if svc.Port > 0 {
		if conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", svc.Port), 2*time.Second); err == nil {
			conn.Close()
			killByPort(svc.Port)
			time.Sleep(1 * time.Second)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	svc.cancel = cancel
	svc.ctx = ctx
	svc.cmd = exec.CommandContext(ctx, svc.Cmd, svc.Args...)
	svc.cmd.Dir = svc.Dir
	svc.cmd.Stdout = &serviceWriter{prefix: svc.Name}
	svc.cmd.Stderr = &serviceWriter{prefix: svc.Name}
	svc.cmd.Env = os.Environ()
	if svc.Env != nil {
		svc.cmd.Env = append(svc.cmd.Env, svc.Env...)
	}

	if err := svc.cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("无法启动进程: %w", err)
	}

	if svc.Port > 0 {
		if err := e.waitForHealthy(svc); err != nil {
			cancel()
			return fmt.Errorf("健康检查失败: %w", err)
		}
	}

	return nil
}

func (e *Environment) waitForHealthy(svc *Service) error {
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)
		resp, err := client.Get(svc.HealthURL)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return fmt.Errorf("%s 在 60s 内未就绪", svc.Name)
}

func (e *Environment) StopAll() {
	log.Println("[Env] 正在停止所有附属服务...")
	for _, svc := range e.services {
		svc.stopped = true
		if svc.cancel != nil {
			svc.cancel()
		}
	}

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("[Env] 所有附属服务已停止")
	case <-time.After(10 * time.Second):
		log.Println("[Env] 超时，强制终止...")
		for _, svc := range e.services {
			if svc.cmd != nil && svc.cmd.Process != nil {
				svc.cmd.Process.Kill()
			}
		}
	}
}

func (e *Environment) SetupSignalHandler() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("[Env] 收到信号 %v，设置关闭标志...", sig)
		SetEnvShuttingDown()
	}()
}

type serviceWriter struct{ prefix string }

func (w *serviceWriter) Write(p []byte) (int, error) {
	lines := string(p)
	for len(lines) > 0 && (lines[len(lines)-1] == '\n' || lines[len(lines)-1] == '\r') {
		lines = lines[:len(lines)-1]
	}
	if lines != "" {
		log.Printf("[%s] %s", w.prefix, lines)
	}
	return len(p), nil
}

func startEnvironment() *Environment {
	runtimeRoot := util.RuntimeRoot()

	bundledQQ := filepath.Join(runtimeRoot, "qq-sidecar", "bundle.mjs")
	bundledWX := filepath.Join(runtimeRoot, "sidecar", "bundle.mjs")
	sourceQQ := filepath.Join(runtimeRoot, "qq-sidecar", "src", "index.ts")
	sourceWX := filepath.Join(runtimeRoot, "sidecar", "src", "index.ts")
	_, qqOk := os.Stat(bundledQQ)
	_, wxOk := os.Stat(bundledWX)
	_, qqSourceOk := os.Stat(sourceQQ)
	_, wxSourceOk := os.Stat(sourceWX)
	useBundled := qqOk == nil && wxOk == nil && (qqSourceOk != nil || wxSourceOk != nil)

	bundledRoot := runtimeRoot
	if !useBundled {
		if exePath, err := os.Executable(); err == nil {
			exeDir := filepath.Dir(exePath)
			exeBundledQQ := filepath.Join(exeDir, "qq-sidecar", "bundle.mjs")
			exeBundledWX := filepath.Join(exeDir, "sidecar", "bundle.mjs")
			_, exeQQOk := os.Stat(exeBundledQQ)
			_, exeWXOk := os.Stat(exeBundledWX)
			if exeQQOk == nil && exeWXOk == nil {
				useBundled = true
				bundledRoot = exeDir
				log.Printf("[Env] 在可执行文件目录找到捆绑侧车: %s", exeDir)
			}
		}
	}

	var env *Environment
	if useBundled {
		env = NewEnvironment(bundledRoot)
		log.Printf("[Env] 根目录: %s", runtimeRoot)
		log.Printf("[Env] 使用打包版附属服务")
	} else {
		workspace := findWorkspace()
		env = NewEnvironment(workspace)
		log.Printf("[Env] 根目录: %s", workspace)
	}

	if useBundled {
		if err := ensureBundledNode(bundledRoot); err != nil {
			log.Printf("[Env] Node运行时解压失败: %v", err)
		}
	}

	sidecarCmd := filepath.Join(env.workspace, "backend", "node", "node.exe")
	sidecarArgs := []string{"node_modules/tsx/dist/cli.mjs", "src/index.ts"}
	sidecarDir := "backend/sidecar"
	if useBundled {
		sidecarCmd = bundledNodePath(bundledRoot)
		sidecarArgs = []string{"launcher.mjs"}
		sidecarDir = "sidecar"
	}
	env.AddService("backend/sidecar", sidecarDir, sidecarCmd, sidecarArgs, 19876, nil)

	qqSidecarCmd := filepath.Join(env.workspace, "backend", "node", "node.exe")
	qqSidecarArgs := []string{"node_modules/tsx/dist/cli.mjs", "src/index.ts"}
	qqSidecarDir := "backend/qq-sidecar"
	if useBundled {
		qqSidecarCmd = bundledNodePath(bundledRoot)
		qqSidecarArgs = []string{"launcher.mjs"}
		qqSidecarDir = "qq-sidecar"
	}
	env.AddService("qq-sidecar", qqSidecarDir, qqSidecarCmd, qqSidecarArgs, 19877, nil)

	env.SetupSignalHandler()
	env.StartAll()
	log.Println("[Env] 附属服务启动中...")

	return env
}

func findWorkspace() string {
	cwd, _ := os.Getwd()
	for i := 0; i < 4; i++ {
		if info, err := os.Stat(filepath.Join(cwd, "backend")); err == nil && info.IsDir() {
			return cwd
		}
		cwd = filepath.Dir(cwd)
	}

	exe, _ := os.Executable()
	dir := filepath.Dir(exe)

	for i := 0; i < 6; i++ {
		for _, check := range []string{"backend", "ai-companion/apps", "apps"} {
			if info, err := os.Stat(filepath.Join(dir, check)); err == nil && info.IsDir() {
				return dir
			}
		}
		if info, err := os.Stat(filepath.Join(dir, "ai-companion")); err == nil && info.IsDir() {
			return dir
		}
		dir = filepath.Dir(dir)
	}

	cwd, _ = os.Getwd()
	for i := 0; i < 3; i++ {
		if info, err := os.Stat(filepath.Join(cwd, "backend")); err == nil && info.IsDir() {
			return cwd
		}
		cwd = filepath.Dir(cwd)
	}

	cwd, _ = os.Getwd()
	return cwd
}

func bundledNodePath(root string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(root, "node", "node.exe")
	}
	return filepath.Join(root, "node", "node")
}

func ensureBundledNode(root string) error {
	nodeExe := "node"
	zipName := "node.zip"
	if runtime.GOOS == "windows" {
		nodeExe = "node.exe"
		zipName = "node.exe.zip"
	}
	nodeDir := filepath.Join(root, "node")
	nodePath := filepath.Join(nodeDir, nodeExe)
	if _, err := os.Stat(nodePath); err == nil {
		return nil
	}
	zipPath := filepath.Join(nodeDir, zipName)
	if _, err := os.Stat(zipPath); err != nil {
		return fmt.Errorf("node压缩包不存在: %s", zipPath)
	}
	log.Printf("[Env] 正在解压Node运行时: %s", zipPath)
	if err := util.UnzipFile(zipPath, nodeDir); err != nil {
		return fmt.Errorf("解压Node失败: %w", err)
	}
	if _, err := os.Stat(nodePath); err != nil {
		return fmt.Errorf("解压后未找到Node: %s", nodePath)
	}
	return nil
}

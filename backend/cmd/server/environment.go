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
	"syscall"
	"time"

	"github.com/u-ai/backend/pkg/util"
)

type Service struct {
	Name      string
	Dir       string
	Cmd       string
	Args      []string
	Env       []string
	Port      int
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	HealthURL string
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
		Name:      name,
		Dir:       filepath.Join(e.workspace, dir),
		Cmd:       cmd,
		Args:      args,
		Env:       env,
		Port:      port,
		HealthURL: fmt.Sprintf("http://127.0.0.1:%d/api/health", port),
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
		svc.cmd.Wait()
		log.Printf("[Env] %s 已退出", svc.Name)
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

func (e *Environment) waitForHealthy(svc *Service) error {
	client := &http.Client{Timeout: 2 * time.Second}

	for i := 0; i < 30; i++ {
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
	return fmt.Errorf("%s 在 30s 内未就绪", svc.Name)
}

func (e *Environment) StopAll() {
	log.Println("[Env] 正在停止所有附属服务...")
	for _, svc := range e.services {
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
		log.Printf("[Env] 收到信号 %v，正在关闭...", sig)
		e.StopAll()
		if e.onShutdown != nil {
			e.onShutdown()
		}
		os.Exit(0)
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
	_, qqOk := os.Stat(bundledQQ)
	_, wxOk := os.Stat(bundledWX)
	useBundled := qqOk == nil && wxOk == nil

	var env *Environment
	if useBundled {
		env = NewEnvironment(runtimeRoot)
		log.Printf("[Env] 根目录: %s", runtimeRoot)
		log.Printf("[Env] 使用打包版附属服务")
	} else {
		workspace := findWorkspace()
		env = NewEnvironment(workspace)
		log.Printf("[Env] 根目录: %s", workspace)
	}

	sidecarCmd := "npx"
	sidecarArgs := []string{"tsx", "src/index.ts"}
	sidecarDir := "backend/sidecar"
	if runtime.GOOS == "windows" {
		sidecarCmd = "npx.cmd"
	}
	if useBundled {
		sidecarCmd = "node"
		sidecarArgs = []string{"bundle.mjs"}
		sidecarDir = "sidecar"
	}
	env.AddService("backend/sidecar", sidecarDir, sidecarCmd, sidecarArgs, 9876, nil)

	qqSidecarCmd := "npx.cmd"
	qqSidecarArgs := []string{"tsx", "src/index.ts"}
	qqSidecarDir := "backend/qq-sidecar"
	if useBundled {
		qqSidecarCmd = "node"
		qqSidecarArgs = []string{"bundle.mjs"}
		qqSidecarDir = "qq-sidecar"
	}
	env.AddService("qq-sidecar", qqSidecarDir, qqSidecarCmd, qqSidecarArgs, 9877, nil)

	env.SetupSignalHandler()
	env.StartAll()
	log.Println("[Env] 附属服务启动中...")

	return env
}

func findWorkspace() string {
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

	cwd, _ := os.Getwd()
	for i := 0; i < 3; i++ {
		if info, err := os.Stat(filepath.Join(cwd, "backend")); err == nil && info.IsDir() {
			return cwd
		}
		cwd = filepath.Dir(cwd)
	}

	cwd, _ = os.Getwd()
	return cwd
}

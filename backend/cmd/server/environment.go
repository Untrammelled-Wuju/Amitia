// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"

	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/pkg/util"
)

var envShuttingDown atomic.Bool

func SetEnvShuttingDown() {
	envShuttingDown.Store(true)
}

func IsEnvShuttingDown() bool {
	return envShuttingDown.Load()
}

type ProcessEntry struct {
	runtimehost.ProcessID
	runtimehost.ProcessSpec
}

type Environment struct {
	host       runtimehost.RuntimeHost
	supervisor runtimehost.ProcessSupervisor
	entries    []ProcessEntry
}

func NewEnvironment(host runtimehost.RuntimeHost) *Environment {
	return &Environment{
		host:       host,
		supervisor: host.Processes(),
	}
}

func (e *Environment) AddProcess(id runtimehost.ProcessID, spec runtimehost.ProcessSpec) {
	e.entries = append(e.entries, ProcessEntry{ProcessID: id, ProcessSpec: spec})
}

func (e *Environment) SetOnShutdown(fn func()) {
	_ = fn
}

func (e *Environment) StartAll() error {
	ctx := context.Background()
	for _, entry := range e.entries {
		if err := e.supervisor.Register(entry.ProcessSpec); err != nil {
			log.Printf("[Env] %s 注册失败: %v", entry.ID, err)
			_ = e.stopStarted(ctx)
			return fmt.Errorf("register %s: %w", entry.ID, err)
		}
		if err := e.supervisor.Start(ctx, entry.ID); err != nil {
			log.Printf("[Env] %s 启动失败: %v", entry.ID, err)
			_ = e.stopStarted(ctx)
			return fmt.Errorf("start %s: %w", entry.ID, err)
		}
	}
	return nil
}

func (e *Environment) stopStarted(ctx context.Context) error {
	var lastErr error
	_ = ctx
	for _, entry := range e.entries {
		_ = e.supervisor.Stop(ctx, entry.ID)
	}
	return lastErr
}

func (e *Environment) WaitReady(ctx context.Context) error {
	var firstErr error
	for _, entry := range e.entries {
		if err := e.supervisor.WaitReady(ctx, entry.ID); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (e *Environment) Ready() error {
	for _, entry := range e.entries {
		snap, ok := e.supervisor.Snapshot(entry.ID)
		if !ok || snap.State != runtimehost.StateReady {
			return fmt.Errorf("%s not ready (state=%s)", entry.ID, snap.State)
		}
	}
	return nil
}

func (e *Environment) StopAll(ctx context.Context) error {
	log.Println("[Env] 正在停止所有附属服务...")
	for i := len(e.entries) - 1; i >= 0; i-- {
		if err := e.supervisor.Stop(ctx, e.entries[i].ID); err != nil {
			log.Printf("[Env] %s 停止失败: %v", e.entries[i].ID, err)
		}
	}
	log.Println("[Env] 所有附属服务已停止")
	return nil
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
		env = NewEnvironment(nil)
		log.Printf("[Env] 根目录: %s", runtimeRoot)
		log.Printf("[Env] 使用打包版附属服务")
	} else {
		workspace := util.RuntimeWorkspaceDir(runtimeRoot)
		_ = workspace
		env = NewEnvironment(nil)
		log.Printf("[Env] 根目录: %s", workspace)
	}

	if useBundled {
		if err := ensureBundledNode(bundledRoot); err != nil {
			log.Printf("[Env] Node运行时解压失败: %v", err)
		}
	}
	_ = env
	return env
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

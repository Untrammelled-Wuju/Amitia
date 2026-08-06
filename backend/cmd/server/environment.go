// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/u-ai/backend/internal/runtimehost"
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

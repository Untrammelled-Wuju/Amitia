// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/u-ai/backend/internal/adminrelease"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	cfg := adminrelease.LoadConfigFromEnv()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("管理服务配置无效: %v", err)
	}
	server, err := adminrelease.NewServer(cfg)
	if err != nil {
		log.Fatalf("管理服务初始化失败: %v", err)
	}
	defer server.Close()
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Minute,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Printf("Amitia 更新管理服务监听 %s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("管理服务启动失败: %v", err)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("管理服务关闭失败: %v", err)
	}
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/proactive"
	"github.com/u-ai/backend/internal/qq"
	"gorm.io/gorm"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/pkg/database/mysql"
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"
	surrealdbDB "github.com/u-ai/backend/pkg/database/surrealdb"
	"github.com/u-ai/backend/pkg/util"

	agenttool "github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/migration"
)

func killExistingServer(addr string) {
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		return
	}
	conn.Close()
	log.Warn("检测到服务端口已被占用，正在终止旧进程...")
	out, _ := exec.Command("cmd", "/c", "netstat -ano | findstr :8899 | findstr LISTENING").Output()
	fields := strings.Fields(string(out))
	for _, f := range fields {
		if pid, err2 := strconv.Atoi(f); err2 == nil {
			if pid != os.Getpid() {
				exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
			}
		}
	}
	time.Sleep(2 * time.Second)
}

func main() {
	runtimeRoot := util.RuntimeRoot()

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = filepath.Join(runtimeRoot, "config")
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(runtimeRoot, configPath)
	}
	config.InitConfig(configPath)

	config.AppCfg.Storage.DataDir = util.ResolveRuntimePath(runtimeRoot, config.AppCfg.Storage.DataDir)
	config.AppCfg.Surreal.DataPath = util.ResolveRuntimePath(runtimeRoot, config.AppCfg.Surreal.DataPath)

	log.InitLogger(filepath.Join(runtimeRoot, "logs"))

	rootCtx, stopRoot := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopRoot()

	db := mysql.NewSQLite(config.AppCfg.Storage.DataDir)

	sqlDB, _ := db.DB()
	agenttool.SetDB(sqlDB)
	if err := applyDatabaseStartupMigrations(db); err != nil {
		log.Error("数据库启动迁移失败:", err)
		os.Exit(1)
	}
	ctx := app.NewAppContext(db, nil)

	env := startEnvironment()
	env.SetOnShutdown(func() {
		qdrantDB.StopQdrant()
		surrealdbDB.StopSurreal()
	})

	cleanup := func() {
		if env != nil {
			env.StopAll()
		}
		qdrantDB.StopQdrant()
		surrealdbDB.StopSurreal()
	}
	defer cleanup()

	startQdrant()
	startSurreal()

	graphSvc := initGraph()
	services := NewAppServices(ctx, graphSvc)
	agenttool.SetMemoryService(services.Memory)

	serverAddr := config.AppCfg.Server.Addr()
	fmt.Printf("\n  ========================================\n")
	fmt.Printf("    %s Backend Server\n", config.AppCfg.App.Name)
	fmt.Printf("    Version:     %s\n", config.AppCfg.App.Version)
	fmt.Printf("    Listen:      http://%s\n", serverAddr)
	fmt.Printf("    Deploy Mode: %s\n", config.AppCfg.App.DeployMode)
	fmt.Printf("    Database:    %s/app.db\n", config.AppCfg.Storage.DataDir)
	fmt.Printf("    Qdrant:      %s:%d\n", config.AppCfg.Qdrant.Host, config.AppCfg.Qdrant.Port)
	fmt.Printf("    SurrealDB:   %s:%d\n", config.AppCfg.Surreal.Host, config.AppCfg.Surreal.Port)
	fmt.Printf("  ========================================\n\n")

	qqMgr := qq.NewManager("http://127.0.0.1:9877")
	qq.SetManager(qqMgr)

	agenttool.SetOnMemorySaved(func(id, key, value, memoryType, characterID string) {
		services.Memory.SyncEmbedding(id, key, value, characterID, memoryType)
		services.Memory.SyncGraphMemory(id)
	})
	agenttool.SetOnProfileSaved(func(id string) {
		services.Profile.SyncGraphProfile(id)
	})
	agenttool.SetOnEpisodicSaved(func(id string) {
		services.Episodic.SyncGraphEpisodic(id)
	})
	chat.InitBuffer(config.AppCfg.Chat.MergeWindowMs)
	go func() {
		time.Sleep(3 * time.Second)
		services.Chat.EnsureChannelConversation("wechat")
		services.Chat.EnsureChannelConversation("qq")
		log.Info("频道对话已确保创建")
	}()
	count, err := services.Chat.RecalculateMessageCounts()
	if err != nil {
		log.Error("重算消息计数失败:", err)
	} else {
		log.Info("消息计数已修复，影响", count, "条对话")
	}
	var charIDs []string
	db.Table("characters").Pluck("id", &charIDs)
	for _, cid := range charIDs {
		services.Companion.ScheduleBasedGenerator(time.Now().Format("2006-01-02"), cid)
	}
	log.Info("今日主动消息任务已生成")

	killExistingServer(serverAddr)

	r := setupRouter(ctx, services)
	if result, err := services.UnifiedEntry.RecoverStaleInteractions(context.Background(), time.Now()); err != nil {
		log.Error("交互启动恢复失败:", err)
	} else if result.Recovered > 0 || result.Failed > 0 {
		log.Info("交互启动恢复完成 scanned=", result.Scanned, " recovered=", result.Recovered, " skipped=", result.Skipped, " failed=", result.Failed)
	}
	services.UnifiedEntry.SetOrchestratorReady(true)
	defer services.UnifiedEntry.SetOrchestratorReady(false)
	services.OutboxRuntime.Start(rootCtx)
	defer services.OutboxRuntime.Stop()
	services.OutboxWorker.Start(rootCtx)
	defer services.OutboxWorker.Stop()
	cron := NewProactiveCron(db, services.Companion, services.RuntimeQueue)
	cron.Start()
	proactive.SchedulerRunning = true
	defer func() {
		proactive.SchedulerRunning = false
		cron.Stop()
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				services.DataLifecycle.ExecuteOutboxCleanup()
			case <-rootCtx.Done():
				return
			}
		}
	}()

	go services.Reconciliation.RunWorker(rootCtx, 10*time.Minute, mindruntime.DefaultReconciliationWorkerTargets())

	srv := &http.Server{
		Addr:    serverAddr,
		Handler: r,
	}
	log.Info("所有服务已就绪，开始监听 ", serverAddr)
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Error("服务启动失败:", err)
			cleanup()
			os.Exit(1)
		}
	case <-rootCtx.Done():
		log.Info("收到关闭信号，开始排水...")
		services.UnifiedEntry.SetOrchestratorReady(false)
		log.Info("已停止接收新请求，等待现有请求完成...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("服务关闭失败:", err)
		}
		if err := <-serverErr; err != nil && err != http.ErrServerClosed {
			log.Error("服务退出异常:", err)
		}
	}
	if err := rootCtx.Err(); err != nil && err != context.Canceled {
		log.Error("服务启动失败:", err)
		cleanup()
		os.Exit(1)
	}
}

func applyDatabaseStartupMigrations(db *gorm.DB) error {
	existingDatabase, err := hasExistingDatabaseTables(db)
	if err != nil {
		return fmt.Errorf("check existing database: %w", err)
	}
	migRunner := migration.Runner{DB: db, SkipBackup: existingDatabase}
	if existingDatabase {
		if err := migRunner.CreatePreMigrationBackup(); err != nil {
			return fmt.Errorf("预迁移备份失败: %w", err)
		}
	}
	if err := initDatabase(db); err != nil {
		return fmt.Errorf("apply initial sql: %w", err)
	}
	if err := migRunner.Apply(migration.DefaultMigrations()); err != nil {
		return fmt.Errorf("apply versioned migrations: %w", err)
	}
	return nil
}

func hasExistingDatabaseTables(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func initDatabase(db *gorm.DB) error {
	sqlPath := filepath.Join(config.AppCfg.Storage.DataDir, "sql.sql")
	if _, err := os.Stat(sqlPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		log.Warn("sql.sql未找到，跳过建表:", err)
		return nil
	}
	if err := migration.ApplyInitialSQLFile(db, sqlPath); err != nil {
		return err
	}
	log.Info("sql.sql建表完成")
	return nil
}

func startQdrant() {
	qcfg := config.AppCfg.Qdrant
	log.Info("正在启动Qdrant...")
	if err := qdrantDB.StartQdrant(); err != nil {
		log.Error("Qdrant启动失败:", err)
		log.Warn("向量检索功能不可用，将回退到关键词搜索")
		return
	}
	if err := qdrantDB.WaitForQdrant(qcfg.Port); err != nil {
		log.Error("等待Qdrant就绪超时:", err)
		qdrantDB.StopQdrant()
		log.Warn("向量检索功能不可用，将回退到关键词搜索")
		return
	}
	if err := qdrantDB.InitClient(); err != nil {
		log.Error("Qdrant客户端初始化失败:", err)
		qdrantDB.StopQdrant()
		log.Warn("向量检索功能不可用，将回退到关键词搜索")
		return
	}
	if err := qdrantDB.EnsureCollections(); err != nil {
		log.Error("Qdrant集合创建失败:", err)
		qdrantDB.StopQdrant()
		log.Warn("向量检索功能不可用，将回退到关键词搜索")
		return
	}
	log.Info("Qdrant就绪，向量检索功能已启用")
}

func startSurreal() {
	cfg := config.AppCfg.Surreal
	log.Info("正在启动SurrealDB...")
	if err := surrealdbDB.StartSurreal(); err != nil {
		log.Error("SurrealDB启动失败:", err)
		log.Warn("图谱功能不可用")
		return
	}
	if err := surrealdbDB.WaitForSurreal(cfg.Port); err != nil {
		log.Error("等待SurrealDB就绪超时:", err)
		surrealdbDB.StopSurreal()
		log.Warn("图谱功能不可用")
		return
	}
	log.Info("SurrealDB就绪，图谱功能已启用")
}

func initGraph() graph.Service {
	cfg := config.AppCfg.Surreal
	var lastErr error
	for i := 0; i < 30; i++ {
		client, err := graph.NewClient(cfg)
		if err == nil {
			return graph.NewService(client)
		}
		lastErr = err
		time.Sleep(time.Second)
	}
	log.Warn("SurrealDB连接失败，图谱功能不可用:", lastErr)
	return nil
}

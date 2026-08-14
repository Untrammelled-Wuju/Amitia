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
	"github.com/u-ai/backend/internal/temporal"
	"gorm.io/gorm"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/security"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/pkg/database/mysql"
	surrealdbDB "github.com/u-ai/backend/pkg/database/surrealdb"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"

	agenttool "github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
)

func killExistingServer(addr string) {
	log.Warn("检测到服务端口已被占用，正在终止旧进程...")
	if err := platform.Get().KillExistingServer(addr); err != nil {
		log.Warn("终止旧进程失败:", err)
	}
}

var triggerShutdown context.CancelFunc

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		os.Exit(handleVersion())
	}
	if len(os.Args) > 2 && os.Args[1] == "--version" {
		fmt.Fprintf(os.Stderr, "error: --version cannot be combined with other arguments\n")
		os.Exit(1)
	}
	runtimeRoot := util.RuntimeRoot()
	configDir := util.RuntimeConfigDir(runtimeRoot)
	config.InitConfig(configDir)

	resolvedToken := config.AppCfg.Security.LocalToken
	if resolvedToken == "" && config.AppCfg.Security.LocalTokenFile != "" {
		if data, rerr := os.ReadFile(config.AppCfg.Security.LocalTokenFile); rerr == nil {
			resolvedToken = strings.TrimSpace(string(data))
		}
	}

	secCfg := &security.SecurityConfig{
		Mode:              security.SecurityMode(config.AppCfg.Security.Mode),
		AllowRemoteAccess: config.AppCfg.Security.AllowRemoteAccess,
		ListenAddress:     config.AppCfg.Server.Addr(),
		JWTSecret:         config.AppCfg.JWT.Secret,
		LocalToken:        resolvedToken,
		AllowedOrigins:    config.AppCfg.Security.AllowedOrigins,
	}
	if err := secCfg.Validate(); err != nil {
		log.Error("安全配置验证失败:", err)
		fmt.Fprintf(os.Stderr, "安全配置验证失败: %v\n", err)
		os.Exit(1)
	}

	if secCfg.Mode == security.SecurityModeNetwork {
		if secCfg.JWTSecret == "" || secCfg.JWTSecret == "u-ai-secret-key-change-me" || len(secCfg.JWTSecret) < 32 {
			log.Error("网络模式要求有效的JWT Secret，长度至少32字节")
			fmt.Fprintln(os.Stderr, "网络模式要求有效的JWT Secret，长度至少32字节")
			os.Exit(1)
		}
	}

	config.AppCfg.Storage.DataDir = util.RuntimeDataDir(runtimeRoot, config.AppCfg.Storage.DataDir)
	config.AppCfg.Providers.GraphStore.SurrealDB.DataPath = util.ResolveRuntimePath(runtimeRoot, config.AppCfg.Providers.GraphStore.SurrealDB.DataPath)

	logDir := util.RuntimeLogDir(runtimeRoot)
	log.InitLogger(logDir)

	paths := util.DetectRuntimePaths(config.AppCfg.Storage.DataDir)
	log.Info("Runtime Root: ", paths.Root)
	log.Info("Config Dir: ", paths.ConfigDir)
	log.Info("Data Dir: ", paths.DataDir)
	log.Info("Log Dir: ", paths.LogDir)
	log.Info("Workspace Dir: ", paths.WorkspaceDir)
	log.Info("Cache Dir: ", paths.CacheDir)
	log.Info("Temp Dir: ", paths.TempDir)

	rootCtx, stopRoot := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopRoot()

	appCtx, appCancel := context.WithCancel(rootCtx)
	triggerShutdown = appCancel

	db := mysql.NewSQLite(config.AppCfg.Storage.DataDir)

	sqlDB, _ := db.DB()
	agenttool.SetDB(sqlDB)
	if err := applyDatabaseStartupMigrations(db, paths.DataDir); err != nil {
		log.Error("数据库启动迁移失败:", err)
		os.Exit(1)
	}
	ctx := app.NewAppContext(db, nil)

	bootstrap, bootErr := newRuntimeBootstrap(&paths)
	if bootErr != nil {
		log.Error("创建运行时宿主失败:", bootErr)
		os.Exit(1)
	}
	graphSvc := initGraph()
	if err := bootstrap.RegisterInfrastructure(sqlDB, graphSvc); err != nil {
		log.Error("基础设施注册失败:", err)
		os.Exit(1)
	}
	if err := bootstrap.StartPhase(appCtx, runtimeorchestrator.PhaseInfrastructure); err != nil {
		log.Error("基础设施启动失败:", err)
		os.Exit(1)
	}
	bootstrap.SetGraphService(graphSvc)
	services, err := NewAppServices(ctx, graphSvc, bootstrap)
	if err != nil {
		log.Error("应用服务初始化失败:", err)
		_ = bootstrap.StopAll(context.Background())
		os.Exit(1)
	}
	if err := bootstrap.RegisterApplication(services); err != nil {
		log.Error("应用组件注册失败:", err)
		_ = bootstrap.StopAll(context.Background())
		os.Exit(1)
	}
	if err := bootstrap.StartPhase(appCtx, runtimeorchestrator.PhaseApplication); err != nil {
		log.Error("应用层启动失败:", err)
		_ = bootstrap.StopAll(context.Background())
		os.Exit(1)
	}
	services.RuntimeOrchestrator = bootstrap

	cleanup := func() {
		_ = bootstrap.StopAll(context.Background())
	}
	defer cleanup()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = services.Extension.Close(shutdownCtx)
	}()
	surrealdbDB.SetSurrealRestartCallback(func() {
		newGraphSvc := initGraph()
		if newGraphSvc != nil {
			services.Graph = newGraphSvc
			bootstrap.SetGraphService(newGraphSvc)
			log.Info("SurrealDB恢复后图谱服务已重新连接")
		}
	})
	agenttool.SetMemoryService(services.Memory)
	agenttool.SetTemporalService(services.Temporal)
	temporalScheduler := temporal.NewScheduler(services.Temporal)
	_ = temporalScheduler
	defer temporalScheduler.Stop()

	serverAddr := config.AppCfg.Server.Addr()
	fmt.Printf("\n  ========================================\n")
	fmt.Printf("    %s Backend Server\n", config.AppCfg.App.Name)
	fmt.Printf("    Version:     %s\n", config.AppCfg.App.Version)
	fmt.Printf("    Listen:      http://%s\n", serverAddr)
	fmt.Printf("    Deploy Mode: %s\n", config.AppCfg.App.DeployMode)
	fmt.Printf("    Database:    %s/app.db\n", config.AppCfg.Storage.DataDir)
	fmt.Printf("    Qdrant:      %s:%d\n", config.AppCfg.Providers.VectorStore.Qdrant.Host, config.AppCfg.Providers.VectorStore.Qdrant.Port)
	fmt.Printf("    SurrealDB:   %s:%d\n", config.AppCfg.Providers.GraphStore.SurrealDB.Host, config.AppCfg.Providers.GraphStore.SurrealDB.Port)
	fmt.Printf("  ========================================\n\n")

	qqMgr := qq.NewManager("http://127.0.0.1:19877")
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
	backfilled, backfillErr := services.Chat.BackfillMissingConversations()
	if backfillErr != nil {
		log.Error("回填缺失对话记录失败:", backfillErr)
	} else if backfilled > 0 {
		log.Info("已回填缺失对话记录，影响", backfilled, "条")
	}

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

	r, errSetup := setupRouter(ctx, services)
	if errSetup != nil {
		log.Error("路由和安全服务初始化失败:", errSetup)
		cleanup()
		os.Exit(1)
	}
	if result, err := services.UnifiedEntry.RecoverStaleInteractions(context.Background(), time.Now()); err != nil {
		log.Error("交互启动恢复失败:", err)
	} else if result.Recovered > 0 || result.Failed > 0 {
		log.Info("交互启动恢复完成 scanned=", result.Scanned, " recovered=", result.Recovered, " skipped=", result.Skipped, " failed=", result.Failed)
	}
	services.UnifiedEntry.SetOrchestratorReady(true)
	defer services.UnifiedEntry.SetOrchestratorReady(false)
	services.OutboxWorker.Start(appCtx)
	defer services.OutboxWorker.Stop()
	services.DeliveryWorker.Start(appCtx)
	defer services.DeliveryWorker.Stop()

	selfHeal := startSelfHealMonitor(appCtx, db)
	defer selfHeal.Stop()
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
				services.DataLifecycle.ExecuteRecalculationTasks()
			case <-appCtx.Done():
				return
			}
		}
	}()

	go services.Reconciliation.RunWorker(appCtx, 10*time.Minute, mindruntime.DefaultReconciliationWorkerTargets())

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
	case <-appCtx.Done():
		log.Info("收到关闭信号，开始排水...")
		services.UnifiedEntry.SetOrchestratorReady(false)
		pluginShutdownCtx, pluginCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := services.Extension.Close(pluginShutdownCtx); err != nil {
			log.Error("Plugin Runtime 关闭失败:", err)
		}
		pluginCancel()
		bootstrap.StopAll(context.Background())
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
	if err := appCtx.Err(); err != nil && err != context.Canceled {
		log.Error("服务启动失败:", err)
		cleanup()
		os.Exit(1)
	}
}

func applyDatabaseStartupMigrations(db *gorm.DB, dataDir string) error {
	isNew, err := migration.IsNewDatabase(db)
	if err != nil {
		return fmt.Errorf("check existing database: %w", err)
	}
	migrations := migration.DefaultMigrations()
	lockDir := filepath.Join(dataDir, "locks")
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return fmt.Errorf("create lock directory: %w", err)
	}
	persistentLock := migration.NewPersistentLock(db, lockDir)
	migRunner := migration.Runner{
		DB:         db,
		Locker:     persistentLock,
		LockName:   "schema_migrations",
		LockTTL:    5 * time.Minute,
		SkipBackup: !isNew,
	}
	if isNew {
		log.Info("检测到新数据库，执行基线快通道...")
		if err := migration.ApplyBaseline(db); err != nil {
			return fmt.Errorf("apply baseline: %w", err)
		}
		log.Info("基线建表完成，标记所有迁移为已应用")
		if err := migration.MarkAllMigrationsApplied(db, migrations); err != nil {
			return fmt.Errorf("mark all migrations applied: %w", err)
		}
		log.Info("所有迁移已标记为已应用，跳过增量迁移执行")
		return nil
	}
	if err := migRunner.CreatePreMigrationBackup(); err != nil {
		return fmt.Errorf("预迁移备份失败: %w", err)
	}
	if err := migration.ApplyBaseline(db); err != nil {
		return fmt.Errorf("apply baseline: %w", err)
	}
	log.Info("基线建表完成")
	if err := migRunner.Apply(migrations); err != nil {
		return fmt.Errorf("apply versioned migrations: %w", err)
	}
	return nil
}

func initGraph() graph.Service {
	cfg := config.AppCfg.Providers.GraphStore.SurrealDB
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

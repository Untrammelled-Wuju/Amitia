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
	"github.com/u-ai/backend/internal/runtimeprofile"
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

	profileResolution, err := runtimeprofile.Resolve(runtimeprofile.ResolveInput{
		Args:             os.Args[1:],
		Env:              nil,
		LegacyDeployMode: config.AppCfg.App.DeployMode,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if !profileResolution.Profile.IsValid() {
		fmt.Fprintf(os.Stderr, "error: invalid runtime profile: %q\n", string(profileResolution.Profile))
		os.Exit(1)
	}
	profile := profileResolution.Profile
	policy := runtimeprofile.PolicyFor(profile)

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

	if err := validateProfileSecurity(profile, secCfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
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

	bootstrap, bootErr := newRuntimeBootstrap(&paths, profile)
	if bootErr != nil {
		log.Error("创建运行时宿主失败:", bootErr)
		os.Exit(1)
	}
	var graphSvc graph.Service
	if policy.GraphStore {
		graphSvc = initGraph()
	}
	if err := bootstrap.RegisterInfrastructure(sqlDB, graphSvc); err != nil {
		log.Error("基础设施注册失败:", err)
		os.Exit(1)
	}
	if err := bootstrap.StartPhase(appCtx, runtimeorchestrator.PhaseInfrastructure); err != nil {
		log.Error("基础设施启动失败:", err)
		os.Exit(1)
	}
	bootstrap.SetGraphService(graphSvc)
	services, err := NewAppServices(ctx, graphSvc, bootstrap, profile, policy)
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
	registerNativeBridgeTransports(services.NativeBridgeRelay, bootstrap)

	if err := runCanonicalRuntimeAssertions(services); err != nil {
		log.Error("Canonical runtime startup assertions failed:", err)
		_ = bootstrap.StopAll(context.Background())
		os.Exit(1)
	}

	if policy.CoreBusinessServices {
		productionCutover := initCutoverGate(db, services)
		closureGate := NewStage2ClosureGateAdapterFromCanonical(services, productionCutover.runtimeGate, productionCutover.closureGate)
		services.ProductionCutover = productionCutover
		services.ClosureGate = closureGate

		archReady := closureGate.ArchitectureReady()
		if !archReady.Ready {
			log.Info("Closure gate not ready:", archReady.Reasons)
		}

		checkResult, err := productionCutover.cutoverPlan.ComputeCutoverCheck(appCtx)
		if err != nil {
			log.Error("检查生产切换状态失败:", err)
			_ = bootstrap.StopAll(context.Background())
			os.Exit(1)
		}

		if checkResult.Failed {
			log.Error("检测到生产切换失败状态，进入recovery模式")
			_ = bootstrap.StopAll(context.Background())
			os.Exit(1)
		}

		if !checkResult.Committed {
			canRun, reasons := closureGate.CanRunCutover()
			if !canRun {
				log.Error(closureGate.FailureMessage(reasons))
				_ = bootstrap.StopAll(context.Background())
				os.Exit(1)
			}
			if checkResult.Incomplete {
				log.Info("检测到未完成的生产切换，正在恢复执行...")
			} else if checkResult.NeverRun {
				log.Info("数据库无cutover记录，G0已通过，开始首次生产切换...")
			} else {
				log.Info("执行生产切换...")
			}
			if err := productionCutover.RunCutover(appCtx); err != nil {
				log.Error("生产切换执行失败:", err)
				_ = bootstrap.StopAll(context.Background())
				os.Exit(1)
			}
			log.Info("生产切换完成")
		}

		if err := runCanonicalRuntimeAssertions(services); err != nil {
			log.Error("Post-cutover canonical assertions failed:", err)
			_ = bootstrap.StopAll(context.Background())
			os.Exit(1)
		}
	}

	cleanup := func() {
		_ = bootstrap.StopAll(context.Background())
	}
	defer cleanup()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = services.Extension.Close(shutdownCtx)
		if services.DeviceMesh != nil {
			_ = services.DeviceMesh.Stop()
		}
	}()
	if policy.GraphStore {
		surrealdbDB.SetSurrealRestartCallback(func() {
			newGraphSvc := initGraph()
			if newGraphSvc != nil {
				services.Graph = newGraphSvc
				bootstrap.SetGraphService(newGraphSvc)
				log.Info("SurrealDB恢复后图谱服务已重新连接")
			}
		})
	}
	if policy.CoreBusinessServices {
		agenttool.SetMemoryService(services.Memory)
		agenttool.SetTemporalService(services.Temporal)
		temporalScheduler := temporal.NewScheduler(services.Temporal)
		_ = temporalScheduler
		defer temporalScheduler.Stop()
	}

	serverAddr := config.AppCfg.Server.Addr()
	fmt.Printf("\n  ========================================\n")
	fmt.Printf("    %s Backend Server\n", config.AppCfg.App.Name)
	fmt.Printf("    Version:        %s\n", config.AppCfg.App.Version)
	fmt.Printf("    Listen:         http://%s\n", serverAddr)
	fmt.Printf("    Runtime Profile: %s\n", profile.String())
	fmt.Printf("    Profile Source: %s\n", profileResolution.Source)
	fmt.Printf("    Deploy Mode:    %s\n", config.AppCfg.App.DeployMode)
	fmt.Printf("    Database:       %s/app.db\n", config.AppCfg.Storage.DataDir)
	fmt.Printf("    Qdrant:         %s:%d\n", config.AppCfg.Providers.VectorStore.Qdrant.Host, config.AppCfg.Providers.VectorStore.Qdrant.Port)
	fmt.Printf("    SurrealDB:      %s:%d\n", config.AppCfg.Providers.GraphStore.SurrealDB.Host, config.AppCfg.Providers.GraphStore.SurrealDB.Port)
	fmt.Printf("  ========================================\n\n")

	if policy.FullHTTPAPI {
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
		if services.DB != nil && services.Companion != nil {
			var charIDs []string
			services.DB.Table("characters").Pluck("id", &charIDs)
			for _, cid := range charIDs {
				services.Companion.ScheduleBasedGenerator(time.Now().Format("2006-01-02"), cid)
			}
		}
		log.Info("今日主动消息任务已生成")
	}

	killExistingServer(serverAddr)

	if !policy.VectorStore {
		log.Info("Qdrant: disabled by runtime profile")
	}
	if !policy.GraphStore {
		log.Info("SurrealDB: disabled by runtime profile")
	}

	srv := &http.Server{
		Addr:    serverAddr,
		Handler: nil,
	}
	serverErr := make(chan error, 1)

	if policy.FullHTTPAPI {
		r, errSetup := setupRouter(ctx, services, bootstrap)
		if errSetup != nil {
			log.Error("路由和安全服务初始化失败:", errSetup)
			cleanup()
			os.Exit(1)
		}
		srv.Handler = r
		if err := startCoreWorkers(appCtx, services, srv); err != nil {
			log.Error("核心Worker启动失败:", err)
			cleanup()
			os.Exit(1)
		}
		if services.DeviceMesh != nil {
			if err := services.DeviceMesh.Start(); err != nil {
				log.Warn("device-mesh runtime start failed: ", err)
			}
		}
		runCoreServer(appCtx, srv, serverErr, services, srv)
		return
	}
	if services.DeviceMesh != nil && services.DeviceMesh.LocalHandler != nil {
		setLocalMeshHandler(services.DeviceMesh.LocalHandler)
	}
	r, errSetup := setupDeviceAgentRouter(ctx, services)
	if errSetup != nil {
		log.Error("Device Agent路由初始化失败:", errSetup)
		cleanup()
		os.Exit(1)
	}
	srv.Handler = r
	runDeviceAgentServer(appCtx, srv, serverErr, services, srv)
}

func startCoreWorkers(appCtx context.Context, services *AppServices, r *http.Server) error {
	ginEngine := r.Handler
	_ = ginEngine
	if result, err := services.UnifiedEntry.RecoverStaleInteractions(context.Background(), time.Now()); err != nil {
		log.Error("交互启动恢复失败:", err)
	} else if result.Recovered > 0 || result.Failed > 0 {
		log.Info("交互启动恢复完成 scanned=", result.Scanned, " recovered=", result.Recovered, " skipped=", result.Skipped, " failed=", result.Failed)
	}
	services.UnifiedEntry.SetOrchestratorReady(true)
	services.OutboxWorker.Start(appCtx)
	services.DeliveryWorker.Start(appCtx)

	selfHeal := startSelfHealMonitor(appCtx, services.DB)
	defer selfHeal.Stop()
	cron := NewProactiveCron(services.DB, services.Companion, services.RuntimeQueue)
	cron.Start()
	proactive.SchedulerRunning = true

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
	if services.InstallationProjectionBridge != nil {
		services.InstallationProjectionBridge.Start(appCtx)
	}
	if services.InstallationDesiredOutbox != nil {
		go func() {
			if err := services.InstallationDesiredOutbox.Run(appCtx); err != nil && appCtx.Err() == nil {
				log.Error("installation desired outbox worker stopped: ", err)
			}
		}()
	}
	if services.InstallationRecoveryWorker != nil {
		go func() {
			if err := services.InstallationRecoveryWorker.Run(appCtx); err != nil && appCtx.Err() == nil {
				log.Error("installation recovery worker stopped: ", err)
			}
		}()
	}
	return nil
}

func runCoreServer(appCtx context.Context, srv *http.Server, serverErr chan error, services *AppServices, r *http.Server) {
	log.Info("所有服务已就绪，开始监听 ", srv.Addr)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Error("服务启动失败:", err)
		}
	case <-appCtx.Done():
		log.Info("收到关闭信号，开始排水...")
		services.UnifiedEntry.SetOrchestratorReady(false)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("服务关闭失败:", err)
		}
		if err := <-serverErr; err != nil && err != http.ErrServerClosed {
			log.Error("服务退出异常:", err)
		}
	}
}

func runDeviceAgentServer(appCtx context.Context, srv *http.Server, serverErr chan error, services *AppServices, r *http.Server) {
	log.Info("Device Agent已就绪，开始监听 ", srv.Addr)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Error("Device Agent启动失败:", err)
		}
	case <-appCtx.Done():
		log.Info("收到关闭信号，Device Agent停止...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Error("Device Agent关闭失败:", err)
		}
		if err := <-serverErr; err != nil && err != http.ErrServerClosed {
			log.Error("Device Agent退出异常:", err)
		}
	}
}

func validateProfileSecurity(profile runtimeprofile.Profile, secCfg *security.SecurityConfig) error {
	switch profile {
	case runtimeprofile.ProfileCloudCore:
		if secCfg.Mode != security.SecurityModeNetwork {
			return &runtimeprofile.ProfileSecurityConflictError{
				Profile: profile,
				Detail:  "cloud-core requires network security mode",
			}
		}
		if !secCfg.AllowRemoteAccess {
			return &runtimeprofile.ProfileSecurityConflictError{
				Profile: profile,
				Detail:  "cloud-core requires allowRemoteAccess=true",
			}
		}
	case runtimeprofile.ProfileDeviceAgent:
		if secCfg.Mode == security.SecurityModeNetwork && secCfg.AllowRemoteAccess {
			return &runtimeprofile.ProfileSecurityConflictError{
				Profile: profile,
				Detail:  "device-agent must not expose public inbound by default",
			}
		}
	case runtimeprofile.ProfileLocal:
		return nil
	}
	return nil
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

func isNewDatabase(db *gorm.DB) bool {
	if db == nil {
		return false
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Count(&count).Error; err != nil {
		return false
	}
	return count == 0
}

func initializeCommittedCutoverState(db *gorm.DB, services *AppServices) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	existing := &migration.CutoverState{}
	if err := db.Order("started_at DESC").First(existing).Error; err == nil && existing.OperationID != "" {
		return nil
	}

	var canonicalGeneration int64 = 1
	if services != nil && services.KernelContainer != nil {
		container := services.KernelContainer
		if container.ToolFacade != nil || container.PermissionBroker != nil ||
			container.EventService != nil || container.ScheduleService != nil ||
			container.TaskRuntimeService != nil || container.HookService != nil {
			canonicalGeneration = 2
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	return db.Exec(`INSERT INTO production_cutover_state
	(operation_id, phase, status, phase_status, snapshot_id, error_message, started_at, updated_at, completed_at, canonical_generation, plan_version)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"baseline-"+time.Now().Format("20060102"),
		"commit",
		"committed",
		"completed",
		"",
		"",
		now,
		now,
		now,
		canonicalGeneration,
		1,
	).Error
}

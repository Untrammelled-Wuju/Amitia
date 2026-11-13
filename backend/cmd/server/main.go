// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"github.com/u-ai/backend/internal/conversationstream"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/graph"
	"github.com/u-ai/backend/internal/mindruntime"
	"github.com/u-ai/backend/internal/temporal"
	"gorm.io/gorm"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimeendpoint"
	"github.com/u-ai/backend/internal/runtimeport"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/security"
	"github.com/u-ai/backend/internal/spaceidentity"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/pkg/database/mysql"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"

	agenttool "github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	amitialog "log"
)

func ensureSingleInstance(addr, dataDir string) {
	if isDynamicListenAddr(addr) {
		addr = ""
	}
	if err := platform.Get().KillExistingServer(addr, dataDir); err != nil {
		log.Error("检查已有服务实例失败:", err)
		fmt.Fprintf(os.Stderr, "检查已有服务实例失败: %v\n", err)
		os.Exit(1)
	}
	if err := platform.Get().WritePidFile(dataDir); err != nil {
		log.Error("写入服务进程标识失败:", err)
		fmt.Fprintf(os.Stderr, "写入服务进程标识失败: %v\n", err)
		os.Exit(1)
	}
}

func isDynamicListenAddr(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	return err == nil && port == "0"
}

func normalizeRuntimeListenPort(port int) int {
	if port == 1 {
		return 0
	}
	return port
}

func applyDynamicProviderPorts(policy runtimeprofile.Policy) error {
	if policy.VectorStore && config.AppCfg.Providers.VectorStore.Enabled && config.AppCfg.Providers.VectorStore.Qdrant.Port == 0 {
		port, err := runtimeport.AllocateLoopbackPair()
		if err != nil {
			return fmt.Errorf("分配 Qdrant 动态端口失败: %w", err)
		}
		config.AppCfg.Providers.VectorStore.Qdrant.Port = port
	}
	if policy.GraphStore && config.AppCfg.Providers.GraphStore.Enabled && config.AppCfg.Providers.GraphStore.SurrealDB.Port == 0 {
		port, err := runtimeport.AllocateLoopback()
		if err != nil {
			return fmt.Errorf("分配 SurrealDB 动态端口失败: %w", err)
		}
		config.AppCfg.Providers.GraphStore.SurrealDB.Port = port
	}
	return nil
}

func runtimeServiceEndpoints() map[string]runtimeendpoint.ServiceEndpoint {
	services := make(map[string]runtimeendpoint.ServiceEndpoint)
	qdrant := config.AppCfg.Providers.VectorStore.Qdrant
	if config.AppCfg.Providers.VectorStore.Enabled && qdrant.Port > 0 {
		services["qdrant"] = runtimeendpoint.ServiceEndpoint{
			HTTPBaseURL: fmt.Sprintf("http://127.0.0.1:%d", qdrant.Port),
			GRPCBaseURL: fmt.Sprintf("http://127.0.0.1:%d", qdrant.Port+1),
			Host:        "127.0.0.1",
			Port:        qdrant.Port,
			GRPCPort:    qdrant.Port + 1,
		}
	}
	surreal := config.AppCfg.Providers.GraphStore.SurrealDB
	if config.AppCfg.Providers.GraphStore.Enabled && surreal.Port > 0 {
		services["surrealdb"] = runtimeendpoint.ServiceEndpoint{
			HTTPBaseURL: fmt.Sprintf("http://127.0.0.1:%d", surreal.Port),
			Host:        "127.0.0.1",
			Port:        surreal.Port,
		}
	}
	return services
}

func writeRuntimeEndpoint(paths util.RuntimePaths, listenAddr string) (runtimeendpoint.Endpoint, error) {
	host, rawPort, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return runtimeendpoint.Endpoint{}, err
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return runtimeendpoint.Endpoint{}, err
	}
	endpoint := runtimeendpoint.New(config.AppCfg.App.Version, spaceidentity.DefaultSpaceID(), host, port)
	endpoint.Services = runtimeServiceEndpoints()
	if err := runtimeendpoint.Write(paths.RunDir, endpoint); err != nil {
		return runtimeendpoint.Endpoint{}, err
	}
	return endpoint, nil
}

var triggerShutdown context.CancelFunc

func main() {
	if len(os.Args) > 1 && os.Args[1] == process.ParentBoundChildFlag {
		if err := process.RunParentBoundChild(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		os.Exit(handleVersion())
	}
	if len(os.Args) > 2 && os.Args[1] == "--version" {
		fmt.Fprintf(os.Stderr, "error: --version cannot be combined with other arguments\n")
		os.Exit(1)
	}
	if hasSupervisorFlag(os.Args[1:]) {
		if err := runSupervisor(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "supervisor error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	runtimeRoot := util.RuntimeRoot()
	configDir := util.RuntimeConfigDir(runtimeRoot)
	config.InitConfig(configDir)
	conversationstream.DefaultManager().SetRingSize(config.AppCfg.Chat.EventReplayRingSize)
	conversationstream.DefaultManager().SetMaxConcurrentExecutions(config.AppCfg.Chat.AgentMaxParallelTurns)

	profileResolution, err := runtimeprofile.Resolve(runtimeprofile.ResolveInput{
		Args: os.Args[1:],
		Env:  nil,
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
	config.AppCfg.Server.Port = normalizeRuntimeListenPort(config.AppCfg.Server.Port)
	if err := applyDynamicProviderPorts(policy); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	config.AppCfg.Storage.DataDir = util.RuntimeDataDir(runtimeRoot, config.AppCfg.Storage.DataDir)
	if _, err := spaceidentity.InitializeDefault(config.AppCfg.Storage.DataDir); err != nil {
		fmt.Fprintf(os.Stderr, "初始化 Space 身份失败: %v\n", err)
		os.Exit(1)
	}
	config.AppCfg.Providers.GraphStore.SurrealDB.DataPath = util.ResolveRuntimePath(runtimeRoot, config.AppCfg.Providers.GraphStore.SurrealDB.DataPath)

	resolvedToken, securityBootstrapErr := prepareSecurityMaterial(config.AppCfg.Storage.DataDir)
	if securityBootstrapErr != nil {
		log.Error("本地安全材料初始化失败:", securityBootstrapErr)
		fmt.Fprintf(os.Stderr, "本地安全材料初始化失败: %v\n", securityBootstrapErr)
		os.Exit(1)
	}

	secCfg := &security.SecurityConfig{
		Mode:              security.SecurityMode(config.AppCfg.Security.Mode),
		AllowRemoteAccess: config.AppCfg.Security.AllowRemoteAccess,
		ListenAddress:     config.AppCfg.Server.Addr(),
		LocalToken:        resolvedToken,
		AllowedOrigins:    config.AppCfg.Security.AllowedOrigins,
	}
	if err := secCfg.Validate(); err != nil {
		log.Error("安全配置验证失败:", err)
		fmt.Fprintf(os.Stderr, "安全配置验证失败: %v\n", err)
		os.Exit(1)
	}

	if err := validateProfileSecurity(profile, secCfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	logDir := util.RuntimeLogDir(runtimeRoot)
	if err := log.InitLogger(logDir); err != nil {
		fmt.Fprintf(os.Stderr, "initialize log: %v\n", err)
		os.Exit(1)
	}

	paths := util.DetectRuntimePaths(config.AppCfg.Storage.DataDir)
	log.Info("Runtime Root: ", paths.Root)
	log.Info("Config Dir: ", paths.ConfigDir)
	log.Info("Data Dir: ", paths.DataDir)
	log.Info("Log Dir: ", paths.LogDir)
	log.Info("Workspace Dir: ", paths.WorkspaceDir)
	log.Info("Cache Dir: ", paths.CacheDir)
	log.Info("Temp Dir: ", paths.TempDir)
	serverAddr := config.AppCfg.Server.Addr()
	ensureSingleInstance(serverAddr, config.AppCfg.Storage.DataDir)
	defer platform.Get().RemovePidFile(config.AppCfg.Storage.DataDir)
	listener, errListen := net.Listen("tcp", serverAddr)
	if errListen != nil {
		log.Error("服务监听失败:", errListen)
		fmt.Fprintf(os.Stderr, "服务监听失败: %v\n", errListen)
		os.Exit(1)
	}
	actualAddr := listener.Addr().String()
	actualHost, actualPortText, _ := net.SplitHostPort(actualAddr)
	actualPort, _ := strconv.Atoi(actualPortText)
	config.AppCfg.Server.Host = actualHost
	config.AppCfg.Server.Port = actualPort
	serverAddr = actualAddr
	endpoint, errEndpoint := writeRuntimeEndpoint(paths, actualAddr)
	if errEndpoint != nil {
		if err := listener.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		log.Error("写运行端点失败:", errEndpoint)
		fmt.Fprintf(os.Stderr, "写运行端点失败: %v\n", errEndpoint)
		os.Exit(1)
	}
	defer runtimeendpoint.RemoveIfOwner(paths.RunDir, os.Getpid())
	log.Info("运行端点: ", serverAddr)

	rootCtx, stopRoot := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopRoot()

	appCtx, appCancel := context.WithCancel(rootCtx)
	triggerShutdown = appCancel

	conversationEventStore, err := conversationstream.NewFileStore(filepath.Join(paths.DataDir, "conversations"))
	if err != nil {
		log.Error("会话事件存储初始化失败:", err)
		os.Exit(1)
	}
	conversationEventFileStore = conversationEventStore
	conversationstream.DefaultManager().SetDurableStore(conversationEventStore)

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
	if err := bootstrap.RegisterInfrastructure(sqlDB); err != nil {
		log.Error("基础设施注册失败:", err)
		os.Exit(1)
	}
	if err := bootstrap.StartPhase(appCtx, runtimeorchestrator.PhaseInfrastructure); err != nil {
		log.Error("基础设施启动失败:", err)
		os.Exit(1)
	}
	graphSvc := bootstrap.GraphService()
	services, err := NewAppServices(ctx, graphSvc, bootstrap, profile, policy)
	if err != nil {
		log.Error("应用服务初始化失败:", err)
		if err := bootstrap.StopAll(context.Background()); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		os.Exit(1)
	}
	if err := bootstrap.RegisterApplication(services); err != nil {
		log.Error("应用组件注册失败:", err)
		if err := bootstrap.StopAll(context.Background()); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		os.Exit(1)
	}
	if err := bootstrap.StartPhase(appCtx, runtimeorchestrator.PhaseApplication); err != nil {
		log.Error("应用层启动失败:", err)
		if err := bootstrap.StopAll(context.Background()); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		os.Exit(1)
	}
	services.RuntimeOrchestrator = bootstrap
	registerNativeBridgeTransports(services.NativeBridgeRelay, bootstrap)

	if err := runCanonicalRuntimeAssertions(services); err != nil {
		log.Error("Canonical runtime startup assertions failed:", err)
		if err := bootstrap.StopAll(context.Background()); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		os.Exit(1)
	}

	cleanup := func() {
		if err := bootstrap.StopAll(context.Background()); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	defer cleanup()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := services.Extension.Close(shutdownCtx); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if services.DeviceMesh != nil {
			if err := services.DeviceMesh.Stop(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	}()
	if policy.CoreBusinessServices {
		agenttool.SetOwnedMemoryRuntime(services.MemoryConsumers)
		agenttool.SetTemporalService(services.Temporal)
		temporalScheduler := temporal.NewScheduler(services.Temporal)
		temporalScheduler.Start(appCtx)
		defer temporalScheduler.Stop()
	}

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
	}

	if !policy.VectorStore {
		log.Info("Qdrant: disabled by runtime profile")
	} else if !config.AppCfg.Providers.VectorStore.Enabled {
		log.Info("Qdrant: disabled by provider configuration")
	}
	if !policy.GraphStore {
		log.Info("SurrealDB: disabled by runtime profile")
	} else if !config.AppCfg.Providers.GraphStore.Enabled {
		log.Info("SurrealDB: disabled by provider configuration")
	}

	srv := newHTTPServer(serverAddr, appCtx)
	serverErr := make(chan error, 1)

	if policy.FullHTTPAPI {
		r, errSetup := setupRouter(ctx, services, bootstrap)
		if errSetup != nil {
			if err := listener.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			log.Error("路由和安全服务初始化失败:", errSetup)
			cleanup()
			os.Exit(1)
		}
		srv.Handler = r
		if err := startMeshLAN(appCtx, services, r, config.AppCfg.Server.LANPort); err != nil {
			if err := listener.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			log.Error("局域网加密服务启动失败: ", err)
			fmt.Fprintf(os.Stderr, "局域网加密服务启动失败: %v\n", err)
			cleanup()
			os.Exit(1)
		}
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
		runCoreServer(appCtx, srv, listener, serverErr, services, endpoint, paths.RunDir)
		return
	}
	if services.DeviceMesh != nil && services.DeviceMesh.LocalHandler != nil {
		setLocalMeshHandler(services.DeviceMesh.LocalHandler)
	}
	r, errSetup := setupDeviceAgentRouter(ctx, services)
	if errSetup != nil {
		if err := listener.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		log.Error("Device Agent路由初始化失败:", errSetup)
		cleanup()
		os.Exit(1)
	}
	srv.Handler = r
	runDeviceAgentServer(appCtx, srv, listener, serverErr, services, endpoint, paths.RunDir)
}

func startCoreWorkers(appCtx context.Context, services *AppServices, r *http.Server) error {
	ginEngine := r.Handler
	_ = ginEngine
	if result, err := services.UnifiedEntry.RecoverStaleInteractions(context.Background(), time.Now()); err != nil {
		log.Error("交互启动恢复失败:", err)
	} else if result.Recovered > 0 || result.Failed > 0 {
		log.Info("交互启动恢复完成 scanned=", result.Scanned, " recovered=", result.Recovered, " skipped=", result.Skipped, " failed=", result.Failed)
	}
	services.UnifiedEntry.SetReady(true)
	services.OutboxWorker.Start(appCtx)
	services.DeliveryWorker.Start(appCtx)
	startAttentionNotificationWorker(appCtx, services)
	go runRolePurgeWorker(appCtx, services)
	if services.OwnedBusiness != nil {
		go services.OwnedBusiness.RunMemoryWorker(appCtx)
		go services.OwnedBusiness.RunContinuityWorker(appCtx)
		if services.LocalSpaceID != "" && services.LocalDeviceID != "" {
			go services.OwnedBusiness.RunMemoryConsolidationWorker(appCtx, business.Request{SpaceID: services.LocalSpaceID, DeviceID: services.LocalDeviceID, TargetDeviceID: services.LocalDeviceID, CoreID: services.LocalSpaceID})
		}
	}
	if services.DeviceMesh != nil {
		for _, source := range []coordination.DataPort{services.DeviceMesh.CoreDataPort, services.DeviceMesh.LocalDeviceDataPort} {
			if port, ok := source.(*meshLocalDataPort); ok {
				go runOwnedProjections(appCtx, port)
				go runOwnedPrivacyDeletions(appCtx, port)
			}
		}
	}
	selfHeal := startSelfHealMonitor(appCtx, services.DB, r.Addr)
	defer selfHeal.Stop()

	go services.Reconciliation.RunWorker(appCtx, 10*time.Minute, mindruntime.DefaultReconciliationWorkerTargets())
	return nil
}

func runCoreServer(appCtx context.Context, srv *http.Server, listener net.Listener, serverErr chan error, services *AppServices, endpoint runtimeendpoint.Endpoint, runDir string) {
	log.Info("所有服务已就绪，开始监听 ", srv.Addr)
	go func() {
		serverErr <- srv.Serve(listener)
	}()

	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Error("服务启动失败:", err)
		}
	case <-appCtx.Done():
		log.Info("收到关闭信号，开始排水...")
		services.UnifiedEntry.SetReady(false)
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

func newHTTPServer(addr string, appCtx context.Context) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: nil,
		BaseContext: func(net.Listener) context.Context {
			return appCtx
		},
	}
}

func runDeviceAgentServer(appCtx context.Context, srv *http.Server, listener net.Listener, serverErr chan error, services *AppServices, endpoint runtimeendpoint.Endpoint, runDir string) {
	log.Info("Device Agent已就绪，开始监听 ", srv.Addr)
	go func() {
		serverErr <- srv.Serve(listener)
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

func applyDatabaseStartupMigrations(db *gorm.DB, _ string) error {
	if err := migration.ApplyBaseline(db); err != nil {
		return fmt.Errorf("apply baseline: %w", err)
	}
	log.Info("当前版本基线建表完成")
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

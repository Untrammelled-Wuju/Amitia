package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/canary"
	"github.com/u-ai/backend/internal/extension/kernel/chat_ui_extension"
	"github.com/u-ai/backend/internal/extension/kernel/contribution"
	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/desktop"
	"github.com/u-ai/backend/internal/extension/kernel/desktop_update"
	"github.com/u-ai/backend/internal/extension/kernel/developer_console"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/extension_slots"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/javascript_main"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/sandbox_webui"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_ordering"
	"github.com/u-ai/backend/internal/extension/kernel/update"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
)

type ContainerBuilder struct {
	dbPath  string
	extRoot string
	db      *sql.DB
}

func NewContainerBuilder() *ContainerBuilder {
	return &ContainerBuilder{}
}

func (b *ContainerBuilder) WithDBPath(path string) *ContainerBuilder {
	b.dbPath = path
	return b
}

func (b *ContainerBuilder) WithExtensionRoot(root string) *ContainerBuilder {
	b.extRoot = root
	return b
}

func (b *ContainerBuilder) WithDB(db *sql.DB) *ContainerBuilder {
	b.db = db
	return b
}

func (b *ContainerBuilder) Build(ctx context.Context) (*Container, error) {
	store, err := b.buildStore(ctx)
	if err != nil {
		return nil, err
	}

	db := store.DB()

	tm := sqlite.NewTransactionManager(db)

	defRepo := sqlite.NewDefinitionRepository(db)
	instRepo := sqlite.NewInstallationRepository(db)
	runtimeRepo := sqlite.NewRuntimeRepository(db)
	moduleRepo := sqlite.NewModuleRepository(db)
	contribRepo := sqlite.NewContributionRepository(db)
	opRepo := sqlite.NewOperationRepository(db)
	scopeRepo := sqlite.NewScopeRepository(db)
	permRepo := sqlite.NewPermissionRepository(db)
	resourceRepo := sqlite.NewResourceRepository(db)
	stateStore := sqlite.NewStateStore(db)

	enablementResolver := enablement.NewDefaultResolver(stateStore)
	enablementService := enablement.NewEnablementService(stateStore, enablementResolver)

	permDefRegistry := permission.NewPermissionDefinitionRegistry()
	permStorage := permission.NewSQLitePermissionStorage(db)
	permBroker := permission.NewDefaultPermissionBroker(permDefRegistry, permStorage)

	scopeStore := scope.NewSQLiteScopeStore(db)
	scopeEvaluator := scope.NewScopeEvaluator(scopeStore, nil)
	scopeManager := scope.NewScopeManager(scopeStore, scopeEvaluator)

	candidateProvider := newContainerCandidateProvider(instRepo, defRepo)
	dependencyResolver := dependency.NewDefaultResolver(candidateProvider)

	supervisor := runtime_supervisor.NewDefaultSupervisor()

	wasmModuleMgr := wasm_runtime.NewModuleManager(b.extRoot)
	wasmFactory := wasm_runtime.NewWASMRuntimeFactory(nil, wasmModuleMgr)
	wasmHostGateway := wasm_runtime.NewHostGateway(nil)
	wasmFactory.SetHostGateway(wasmHostGateway)
	wasmDefRepo := sqlite.NewWASMDefinitionRepository(db)
	_ = supervisor.RegisterFactory(wasmFactory)

	trustedSvcRoot := filepath.Join(b.extRoot, "trusted-services")
	trustedSupervisor := trusted_service.NewProcessSupervisor(trustedSvcRoot)
	defProvider := newMemoryDefinitionProvider()
	trustedFactory := trusted_service.NewTrustedServiceFactory(trustedSupervisor, defProvider, b.extRoot)
	_ = supervisor.RegisterFactory(trustedFactory)

	contribRegistry := contribution.NewContributionRegistry()

	agentSkillCatalog := agent_skill.NewAgentSkillCatalog()

	workflowRegistry := workflow.NewWorkflowRegistry()

	auditWriter := package_security.NewMemoryAuditWriter()
	packageSec := package_security.NewPackageSecurityService(package_security.DefaultArchivePolicy(), auditWriter)

	stateLoader := newContainerStateLoader(instRepo, defRepo, moduleRepo, contribRepo, runtimeRepo, stateStore)
	preflightChecker := newContainerPreflightChecker(dependencyResolver)
	planExecutor := newContainerPlanExecutor(instRepo, defRepo, moduleRepo, contribRepo, stateStore)
	lcAuditWriter := newContainerAuditWriter(opRepo)
	lifecycleMgr := lifecycle_manager.NewManager(stateLoader, preflightChecker, planExecutor, lcAuditWriter)

	adapterRegistry := capability.NewRuntimeAdapterRegistry()
	toolRegistry := capability.NewToolRegistry()
	executionKernel := &execution.ExecutionPipeline{
		InvocationValidator: execution.NewInvocationValidator(),
		InputValidator:      execution.NewInputValidator(),
		AvailabilityGate:    execution.NewAvailabilityGate(nil),
		ScopeGate:           execution.NewScopeGate(),
		PermissionGate:      execution.NewPermissionGate(),
		ApprovalGate:        execution.NewApprovalGate(),
		ConcurrencyCtrl:     execution.NewConcurrencyController(),
		RateLimiter:         execution.NewRateLimiter(),
		IdempotencyGuard:    execution.NewIdempotencyGuard(),
		RetryCtrl:           execution.NewRetryController(),
		TimeoutCtrl:         execution.NewTimeoutController(),
		CancellationCtrl:    execution.NewCancellationController(),
		DepthGuard:          execution.NewDepthGuard(),
		Dispatcher:          execution.NewRuntimeDispatcher(adapterRegistry),
		ResultValidator:     execution.NewResultValidator(),
		Sanitizer:           execution.NewSanitizer(),
		SideEffectRec:       execution.NewSideEffectRecorder(),
		AuditRec:            execution.NewAuditRecorder(),
		MetricsRec:          execution.NewMetricsRecorder(),
		CircuitBreaker:      execution.NewCircuitBreakerCoordinator(),
	}
	executionKernel.ScopeGate.ScopeManager = scopeManager
	executionKernel.PermissionGate.Broker = permBroker
	executionKernel.ToolResolver = func(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
		def, ok := toolRegistry.Get(ctx, toolID)
		if !ok {
			if alt, altOK := toolRegistry.GetByModelName(ctx, toolID); altOK {
				return alt, nil
			}
			return capability.ToolDefinition{}, fmt.Errorf("tool %s not registered in kernel tool registry", toolID)
		}
		return def, nil
	}

	taskRepo := sqlite.NewTaskRepository(db)
	taskCfg := task_runtime.DefaultTaskRuntimeConfig()
	taskCfg.WorkspaceRoot = b.extRoot
	taskCfg.NodePath = b.resolveNodePath()
	taskCfg.TaskHostPath = b.resolveTaskHostPath()
	taskRuntimeService := task_runtime.NewTaskRuntimeService(taskRepo, taskCfg)
	taskHandler := task_runtime.NewTaskHandler(taskRuntimeService)

	taskSupervisorFactory := task_runtime.NewTaskSupervisorFactory(taskRuntimeService)
	_ = supervisor.RegisterFactory(taskSupervisorFactory)

	scheduleRepo := sqlite.NewScheduleRepository(db)
	scheduleSvc, err := schedule.NewScheduleService(schedule.ScheduleDeps{
		Store:             scheduleRepo,
		PermissionChecker: schedule.NewBrokerPermissionChecker(permBroker, scheduleRepo),
		ScopeChecker:      schedule.NewManagerScopeChecker(scopeManager, scheduleRepo, scopeStore),
		DependencyChecker: schedule.NewResolverDependencyChecker(dependencyResolver),
		WorkflowExecutor:  NewKernelWorkflowFacadeAdapter(),
		TaskEnqueueFn:     BuildScheduleTaskEnqueueFunc(taskRuntimeService),
		RuntimeHandlerFn:  BuildScheduleRuntimeHandlerFn(supervisor),
	})
	if err != nil {
		return nil, fmt.Errorf("kernel: create schedule service: %w", err)
	}

	hookService, err := hook.NewService(hook.ServiceConfig{
		DB:                 db,
		Supervisor:         supervisor,
		PermissionBroker:   permBroker,
		ScopeManager:       scopeManager,
		DependencyResolver: dependencyResolver,
		UseSQLite:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("kernel: create hook service: %w", err)
	}

	eventSvc, err := event.NewService(event.DefaultServiceConfig().WithDB(db))
	if err != nil {
		return nil, fmt.Errorf("kernel: create event service: %w", err)
	}
	if err := eventSvc.RegisterDefaultEventTypes(ctx); err != nil {
		return nil, fmt.Errorf("kernel: register default event types: %w", err)
	}
	eventResolver := BuildEventEffectiveResolver(permBroker, scopeManager, dependencyResolver, supervisor, eventSvc.GetDispatcher())
	eventSvc.SetEffectiveResolver(eventResolver)
	eventBridge := event.NewRuntimeBridge(eventSvc)
	eventBridge.Attach()
	hostEmitter := event.NewHostEventEmitter(eventSvc)
	lifecycleEmitter := event.NewLifecycleEventEmitter(hostEmitter)

	jsFactory := javascript_main.NewRuntimeFactory()
	eventDeliveryAdapter := javascript_main.NewEventDeliveryAdapter(jsFactory)
	eventBridge.SetDeliveryCallback(eventDeliveryAdapter.HandleDelivery)
	taskRuntimeService.SetEventEmitter(hostEmitter)

	hostAPIGateway := host_api.NewDefaultGateway()
	host_api.RegisterPermissionDefinitions(permDefRegistry)
	hostAPIGateway.SetPermissionChecker(host_api.NewBrokerPermissionChecker(permBroker))
	hostAPIGateway.SetScopeChecker(host_api.NewManagerScopeChecker(scopeManager, host_api.NewSnapshotStoreAdapter(scopeStore)))
	hostAPIGateway.SetAuditWriter(newHostAPIAuditWriter())
	if err := setupDefaultHostAPIRoutes(hostAPIGateway, eventSvc, scheduleSvc); err != nil {
		return nil, fmt.Errorf("kernel: setup host api routes: %w", err)
	}
	scheduleSvc.GetExecutor().RegisterTargetAdapter(schedule.NewToolTargetAdapter(NewHostAPIToolExecutorAdapter(hostAPIGateway)))
	jsFactory.SetHostAPI(hostAPIGateway)

	jsSupervisorFactory := javascript_main.NewSupervisorFactory(jsFactory, b.resolveNodePath(), b.resolvePluginHostPath())
	_ = supervisor.RegisterFactory(jsSupervisorFactory)

	uiHost := ui_contribution.NewUIHost()
	uiContribRepo := sqlite.NewSQLiteUIContributionRepository(store.DB())
	savedContribs, _ := uiContribRepo.ListAll(ctx)
	for _, def := range savedContribs {
		_ = uiHost.RegisterContribution(def)
	}
	slotRegistry := extension_slots.DefaultSlotRegistry()
	pageRegistry := extension_page_host.NewPageRegistry()
	pageSessionMgr := extension_page_host.NewSessionManager()
	uiPermValidator := permission.NewUIPermissionValidator()
	pageHost := extension_page_host.NewPageHostWithValidator(pageRegistry, pageSessionMgr, uiPermValidator)
	schemaValidator := schema_ui.NewValidator()
	schemaCache := schema_ui.NewCompilerCache()
	sandboxHost := sandbox_webui.NewHost()
	sandboxDispatcher := sandbox_webui.NewBridgeActionDispatcher(func(ctx context.Context, sessionID, actionID string, input json.RawMessage) (json.RawMessage, error) {
		session, err := sandboxHost.GetSession(sessionID)
		if err != nil {
			return nil, fmt.Errorf("sandbox: session not found: %w", err)
		}
		if !session.IsActionAllowed(actionID) {
			return nil, fmt.Errorf("sandbox: action %s not allowed for session %s", actionID, sessionID)
		}
		if hostAPIGateway != nil {
			identity := runtime_supervisor.RuntimeIdentity{
				InstanceID:  sessionID,
				ExtensionID: domain.ExtensionID(session.ExtensionID),
				ModuleID:    domain.ModuleID(session.ModuleID),
			}
			callReq := host_api.CallRequest{
				CallID:          fmt.Sprintf("sandbox-%s-%s", sessionID, actionID),
				RuntimeIdentity: identity,
				Method:          host_api.MethodToolExecute,
				Version:         1,
				Input:           input,
			}
			result := hostAPIGateway.Call(ctx, callReq)
			if result.Error != nil {
				return nil, fmt.Errorf("sandbox: action %s failed: %s", actionID, result.Error.Message)
			}
			return result.Output, nil
		}
		return json.Marshal(map[string]any{
			"ok":       true,
			"actionId": actionID,
			"sessionId": sessionID,
		})
	})
	sandboxDataProvider := sandbox_webui.NewBridgeDataSourceProvider()
	sandboxNavigator := sandbox_webui.NewBridgeNavigator(func(ctx context.Context, sessionID, target string) error {
		session, err := sandboxHost.GetSession(sessionID)
		if err != nil {
			return fmt.Errorf("sandbox: session not found: %w", err)
		}
		if !sandbox_webui.IsNavigationTargetAllowed(target, session) {
			return fmt.Errorf("sandbox: navigation to %s denied", target)
		}
		return nil
	})
	sandboxBridge := sandbox_webui.NewBridge(sandboxHost, sandboxDispatcher, sandboxDataProvider, sandboxNavigator)
	sandboxHost.SetBridge(sandboxBridge)
	chatExtRegistry := chat_ui_extension.NewChatExtensionRegistry()
	orderingEngine := ui_ordering.NewOrderingEngine()

	desktopHost := desktop.NewDesktopHost()
	updateBaseDir := filepath.Join(b.extRoot, "update-downloads")
	if b.extRoot == "" {
		updateBaseDir = filepath.Join(os.TempDir(), "amitia-update-downloads")
	}
	updateManager := desktop_update.NewUpdateManager(updateBaseDir, "26.1.8")
	updateAdapter := NewUpdateManagerAdapter(updateManager)
	desktopActionBridge := NewDesktopActionBridge(permBroker, scopeManager, executionKernel)
	desktopHost.SetPermissionChecker(desktopActionBridge)
	desktopHost.SetScopeChecker(desktopActionBridge)
	desktopHost.SetActionExecutor(desktopActionBridge)
	desktopAPI := desktop.NewDesktopAPI(desktopHost)
	updateAPI := desktop.NewUpdateAPI(updateAdapter)

	amitiaxInstaller := amitiax.NewInstaller()

	devModeStore := dev_mode.NewSQLiteWorkspaceStore(db)
	devModeRegistry := dev_mode.NewWorkspaceRegistry()
	devModePipeline := dev_mode.NewRebuildPipeline(b.resolveNodePath())
	devModePreserver := dev_mode.NewStatePreserver()
	devModeReloader := dev_mode.NewRuntimeReloader(devModeRegistry, devModePipeline, devModePreserver)
	devModeSessions := dev_mode.NewSessionManager(8 * time.Hour)

	devConsoleRepo := developer_console.NewDiagnosticRepository(2000)
	aggregators := &developer_console.ConsoleAggregators{
		ExtensionProvider:  newContainerExtensionSummaryProvider(instRepo, moduleRepo),
		InvocationProvider: newContainerInvocationSummaryProvider(devConsoleRepo),
		EventProvider:      newContainerEventSummaryProvider(devConsoleRepo),
		StorageProvider:    newContainerStorageSummaryProvider(db),
		PermissionProvider: newContainerPermissionSummaryProvider(db),
		LifecycleProvider:  newContainerLifecycleSummaryProvider(devConsoleRepo),
	}
	devConsoleSvc := developer_console.NewConsoleService(aggregators)
	devConsoleHandler := developer_console.NewHTTPHandler(devConsoleSvc, devConsoleRepo)

	rollbackRepo := update.NewRollbackRepository(db)
	journalMgr := update.NewJournalManager(rollbackRepo)
	rollbackPointStore := update.NewRollbackPointStore()
	generationMgr := update.NewGenerationManager()
	sideEffectAssessor := update.NewSideEffectAssessor()
	updateMigrationExecutor := update.NewMigrationExecutor()
	rollbackPlanner := update.NewRollbackPlanner(rollbackPointStore, sideEffectAssessor, generationMgr)
	rollbackExecutorV2 := update.NewRollbackExecutorV2(
		rollbackPointStore, updateMigrationExecutor, generationMgr,
		journalMgr, rollbackRepo, rollbackPlanner,
	)
	recoveryMgr := update.NewRecoveryManager(journalMgr, rollbackExecutorV2, rollbackPlanner, rollbackRepo)

	migrationRepo := migration.NewMigrationRepository(db)
	migrationGraphResolver := migration.NewMigrationGraphResolver()
	migrationPlanner := migration.NewMigrationPlanner(migrationGraphResolver)
	snapshotBaseDir := filepath.Join(b.extRoot, "migration-snapshots")
	if b.extRoot == "" {
		snapshotBaseDir = filepath.Join(os.TempDir(), "amitia-migration-snapshots")
	}
	migrationSnapshotMgr := migration.NewSnapshotManager(snapshotBaseDir, db)
	migrationCheckpointMgr := migration.NewCheckpointManager(db)
	migrationValidator := migration.NewPreconditionValidator()
	migrationExecutor := migration.NewMigrationExecutor(
		migrationPlanner, migrationSnapshotMgr, migrationCheckpointMgr,
		migrationValidator, migrationRepo, migrationGraphResolver,
	)

	canaryRepo := canary.NewCanaryRepository(db)
	canaryStageMgr := canary.NewStageManager(canaryRepo)
	canaryRouter := canary.NewGenerationRouter(canaryRepo)
	canaryHealthCollector := canary.NewHealthMetricsCollector()
	canaryHealthEvaluator := canary.NewHealthEvaluator()
	canaryCohortResolver := canary.NewCohortResolver()
	canaryDualWriteMgr := canary.NewDualWriteManager()
	canaryShadowMgr := canary.NewShadowManager()
	canaryOwnershipResolver := canary.NewBackgroundOwnershipResolver()

	container := &Container{
		Store:                  store,
		TransactionManager:     tm,
		DefinitionRepository:   defRepo,
		InstallationRepository: instRepo,
		ModuleRepository:       moduleRepo,
		ContributionRepository: contribRepo,
		EnablementStore:        stateStore,
		RuntimeRepository:      runtimeRepo,
		OperationRepository:    opRepo,
		ScopeRepository:        scopeRepo,
		PermissionRepository:   permRepo,
		ResourceRepository:     resourceRepo,

		PackageSecurity:      packageSec,
		LifecycleManager:     lifecycleMgr,
		ContributionRegistry: contribRegistry,
		DependencyResolver:   dependencyResolver,
		RuntimeSupervisor:    supervisor,
		ExecutionKernel:      executionKernel,
		HostAPIGateway:       hostAPIGateway,
		PermissionBroker:     permBroker,
		ScopeManager:         scopeManager,
		AgentSkillCatalog:    agentSkillCatalog,
		WorkflowRegistry:     workflowRegistry,
		EnablementService:    enablementService,
		EnablementResolver:   enablementResolver,

		ToolRegistry:    toolRegistry,
		AdapterRegistry: adapterRegistry,

		TaskRepository:     taskRepo,
		TaskRuntimeService: taskRuntimeService,
		TaskHandler:        taskHandler,

		ScheduleService:    scheduleSvc,
		ScheduleRepository: scheduleRepo,

		WASMRuntimeFactory: wasmFactory,
		WASMModuleManager:  wasmModuleMgr,
		WASMHostGateway:    wasmHostGateway,
		WASMDefinitionRepo: wasmDefRepo,

		TrustedServiceSupervisor: trustedSupervisor,
		TrustedServiceFactory:    trustedFactory,
		HookService:              hookService,
		EventService:             eventSvc,
		EventRuntimeBridge:       eventBridge,
		HostEventEmitter:         hostEmitter,
		LifecycleEventEmitter:    lifecycleEmitter,
		JSRuntimeFactory:         jsFactory,
		EventDeliveryAdapter:     eventDeliveryAdapter,

		UIHost:              uiHost,
		UIContributionRepo:  uiContribRepo,
		SlotRegistry:        slotRegistry,
		PageHost:            pageHost,
		SchemaValidator:     schemaValidator,
		SchemaCompilerCache: schemaCache,
		SandboxHost:         sandboxHost,
		ChatExtensionRegistry: chatExtRegistry,
		OrderingEngine:      orderingEngine,
		ExtRoot:             b.extRoot,

		DesktopHost:     desktopHost,
		UpdateManager:   updateManager,
		UpdateAdapter:   updateAdapter,
		DesktopAPI:      desktopAPI,
		UpdateAPI:       updateAPI,
		DesktopActionBridge: desktopActionBridge,

		DevConsoleService:   devConsoleSvc,
		DevConsoleRepo:      devConsoleRepo,
		DevConsoleHandler:   devConsoleHandler,
		TrustService:        trust.NewTrustService(trust.TrustServiceConfig{}),
		AmitiaxInstaller:    amitiaxInstaller,
		DevModeRegistry:     devModeRegistry,
		DevModePipeline:     devModePipeline,
		DevModeReloader:     devModeReloader,
		DevModeSessions:     devModeSessions,
		DevModeStore:        devModeStore,

		UpdateRecoveryManager: recoveryMgr,

		MigrationRepository:    migrationRepo,
		MigrationExecutor:      migrationExecutor,
		MigrationGraphResolver: migrationGraphResolver,
		MigrationPlanner:       migrationPlanner,
		MigrationSnapshotMgr:   migrationSnapshotMgr,
		MigrationCheckpointMgr: migrationCheckpointMgr,
		MigrationValidator:     migrationValidator,

		RollbackRepository:   rollbackRepo,
		JournalManager:       journalMgr,
		RollbackExecutorV2:   rollbackExecutorV2,
		RollbackPlanner:      rollbackPlanner,
		RollbackPointStore:   rollbackPointStore,
		UpdateGenerationMgr:  generationMgr,
		UpdateMigrationExec:  updateMigrationExecutor,

		CanaryRepository:        canaryRepo,
		CanaryStageManager:      canaryStageMgr,
		CanaryGenerationRouter:  canaryRouter,
		CanaryHealthCollector:   canaryHealthCollector,
		CanaryHealthEvaluator:   canaryHealthEvaluator,
		CanaryCohortResolver:    canaryCohortResolver,
		CanaryDualWriteManager:  canaryDualWriteMgr,
		CanaryShadowManager:     canaryShadowMgr,
		CanaryOwnershipResolver: canaryOwnershipResolver,
	}

	return container, nil
}

func (b *ContainerBuilder) buildStore(ctx context.Context) (*sqlite.Store, error) {
	if b.db != nil {
		return sqlite.NewStoreFromDB(b.db), nil
	}
	if b.dbPath == "" {
		return nil, fmt.Errorf("kernel: db path or *sql.DB is required")
	}
	store, err := sqlite.NewStore(b.dbPath)
	if err != nil {
		return nil, fmt.Errorf("kernel: create store: %w", err)
	}
	if err := store.Migrate(ctx); err != nil {
		store.Close()
		return nil, fmt.Errorf("kernel: migrate store: %w", err)
	}
	return store, nil
}

func (b *ContainerBuilder) resolveNodePath() string {
	candidates := []string{
		"node.exe",
		filepath.Join("backend", "node.exe"),
	}
	for _, c := range candidates {
		absPath, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			return absPath
		}
	}
	return "node"
}

func (b *ContainerBuilder) resolveTaskHostPath() string {
	candidates := []string{
		filepath.Join("..", "runtime", "task-host", "dist", "index.js"),
		filepath.Join("runtime", "task-host", "dist", "index.js"),
	}
	for _, c := range candidates {
		absPath, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			return absPath
		}
	}
	absPath, _ := filepath.Abs(filepath.Join("..", "runtime", "task-host", "dist", "index.js"))
	return absPath
}

func (b *ContainerBuilder) resolvePluginHostPath() string {
	candidates := []string{
		filepath.Join("..", "runtime", "plugin-host", "dist", "index.js"),
		filepath.Join("runtime", "plugin-host", "dist", "index.js"),
	}
	for _, c := range candidates {
		absPath, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && !info.IsDir() {
			return absPath
		}
	}
	absPath, _ := filepath.Abs(filepath.Join("..", "runtime", "plugin-host", "dist", "index.js"))
	return absPath
}

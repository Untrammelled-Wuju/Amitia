package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/canary"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/chat_ui_extension"
	"github.com/u-ai/backend/internal/extension/kernel/contribution"
	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/desktop"
	"github.com/u-ai/backend/internal/extension/kernel/desktop_update"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/developer_console"
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
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_ordering"
	"github.com/u-ai/backend/internal/extension/kernel/update"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/pkg/sse"
)

type ContainerBuilder struct {
	dbPath                  string
	extRoot                 string
	db                      *sql.DB
	characterReader         CharacterReader
	conversationReader      ConversationReader
	memoryQueryService      MemoryQueryService
	nodeEnvironmentResolver script_host.NodeEnvironmentResolver
	hostArtifactResolver    script_host.ArtifactResolver
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

func (b *ContainerBuilder) WithCharacterReader(r CharacterReader) *ContainerBuilder {
	b.characterReader = r
	return b
}

func (b *ContainerBuilder) WithConversationReader(r ConversationReader) *ContainerBuilder {
	b.conversationReader = r
	return b
}

func (b *ContainerBuilder) WithMemoryQueryService(s MemoryQueryService) *ContainerBuilder {
	b.memoryQueryService = s
	return b
}

func (b *ContainerBuilder) WithNodeEnvironmentResolver(
	resolver script_host.NodeEnvironmentResolver,
) *ContainerBuilder {
	b.nodeEnvironmentResolver = resolver
	return b
}

func (b *ContainerBuilder) WithHostArtifactResolver(
	resolver script_host.ArtifactResolver,
) *ContainerBuilder {
	b.hostArtifactResolver = resolver
	return b
}

func (b *ContainerBuilder) Build(ctx context.Context) (*Container, error) {
	nodeResolver := b.nodeEnvironmentResolver
	if nodeResolver == nil {
		nodeResolver = script_host.UnavailableNodeResolver()
	}
	artifactResolver := b.hostArtifactResolver
	if artifactResolver == nil {
		artifactResolver = script_host.UnavailableArtifactResolver()
	}

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
	permSnapshotStore := permission.NewSQLitePermissionSnapshotStore(db)
	permBroker := permission.NewDefaultPermissionBroker(permDefRegistry, permStorage)

	scopeStore := scope.NewSQLiteScopeStore(db)
	relationChecker := newRepositoryScopeRelationChecker(db, resourceRepo, opRepo)
	scopeEvaluator := scope.NewScopeEvaluator(scopeStore, relationChecker)
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
	workflowDefRepo := sqlite.NewWorkflowDefinitionRepository(db)
	workflowRegistry.SetDefinitionStore(workflowDefRepo)
	_ = workflowRegistry.LoadFromStore(ctx)
	workflowExecutor := workflow.NewWorkflowExecutor(workflowRegistry)
	workflowExecutor.SetCheckpointStore(sqlite.NewSQLiteCheckpointStore(db))
	workflowExecutor.SetCompensationManager(workflow.NewCompensationManager())
	workflowExecRepo := sqlite.NewWorkflowExecutionRepository(db)
	workflowExecutor.SetRunStore(workflowExecRepo)
	workflowTriggerManager := workflow.NewTriggerManager(workflowExecutor)
	workflowTriggerManager.SetStore(workflowDefRepo)
	workflowExecutor.SetStepGuard(&workflow.SecurityGuard{
		PermissionCheck: func(ctx context.Context, extensionID, moduleID string, permissionsRequired []string, background bool) error {
			subject := permission.SubjectForExtension(extensionID)
			if moduleID != "" {
				subject = permission.PermissionSubject{Type: permission.SubjectModule, ID: moduleID, ExtensionID: extensionID, ModuleID: moduleID}
			}
			requirements := make([]permission.PermissionRequirement, 0, len(permissionsRequired))
			for _, permissionID := range permissionsRequired {
				requirements = append(requirements, permission.PermissionRequirement{PermissionID: permissionID, Scope: permission.ScopeForExtension(extensionID)})
			}
			decision := permBroker.Evaluate(ctx, permission.PermissionEvaluationRequest{Subject: subject, Requirements: requirements, IsBackground: background})
			if decision.Decision != permission.DecisionAllow && decision.Decision != permission.DecisionAllowPersistent && decision.Decision != permission.DecisionAllowOnce && decision.Decision != permission.DecisionAllowSession {
				return fmt.Errorf("permission decision %s", decision.Decision)
			}
			return nil
		},
		ScopeCheck: func(ctx context.Context, extensionID, moduleID, scopeName string, executionContext workflow.ExecutionContext) error {
			subjectType := scope.SubjectExtension
			subjectID := extensionID
			if moduleID != "" {
				subjectType = scope.SubjectModule
				subjectID = moduleID
			}
			decision := scopeManager.Evaluate(ctx, scope.ScopeEvaluationRequest{SubjectType: subjectType, SubjectID: subjectID, CharacterID: executionContext.CharacterID, ConversationID: executionContext.ConversationID, ExtensionID: extensionID, ModuleID: moduleID, InvocationID: executionContext.InvocationID, Generation: executionContext.Generation})
			if !decision.Allowed {
				return fmt.Errorf("scope %s denied", scopeName)
			}
			return nil
		},
		GenerationCheck: func(ctx context.Context, extensionID string, generation int64) error {
			installation, err := instRepo.GetInstallation(ctx, domain.ExtensionID(extensionID))
			if err != nil {
				return err
			}
			if installation.Generation != generation {
				return fmt.Errorf("expected generation %d, got %d", installation.Generation, generation)
			}
			return nil
		},
	})

	packageSec := package_security.NewPackageSecurityServiceAtRoot(package_security.DefaultArchivePolicy(), package_security.NewSQLiteAuditWriter(db), b.extRoot)
	packageRepo := NewPackageRepository(db)
	packageArtifactStore := NewPackageArtifactStore(b.extRoot)
	packageGenerationStore := NewPackageGenerationStore(b.extRoot)
	packageTrustRepo := NewPackageTrustRepository(db)
	userDataSnapshotStore := NewUserDataSnapshotStore(db)
	resourceSnapshotStore := NewResourceSnapshotStore(db, b.extRoot)
	if err := resourceSnapshotStore.EnsureSchema(ctx); err != nil {
		return nil, fmt.Errorf("kernel: ensure resource snapshot schema: %w", err)
	}
	packageSnapshotRepo := NewPackageSnapshotRepository(db)
	trustService := trust.NewTrustService(trust.TrustServiceConfig{})
	if err := packageTrustRepo.Restore(ctx, trustService); err != nil {
		return nil, fmt.Errorf("kernel: restore package trust: %w", err)
	}

	stateLoader := newContainerStateLoader(instRepo, defRepo, moduleRepo, contribRepo, runtimeRepo, stateStore)
	preflightChecker := newContainerPreflightChecker(dependencyResolver)
	typedInstaller := NewTypedContributionInstaller(nil)
	planExecutor := newContainerPlanExecutor(instRepo, defRepo, moduleRepo, contribRepo, stateStore, typedInstaller)
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
	taskCfg.NodeEnvironmentResolver = nodeResolver
	taskCfg.HostArtifactResolver = artifactResolver
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
		WorkflowExecutor: NewKernelWorkflowFacadeAdapter(workflowExecutor, func(ctx context.Context, extensionID string) (int64, error) {
			installation, err := instRepo.GetInstallation(ctx, domain.ExtensionID(extensionID))
			if err != nil {
				return 0, err
			}
			return installation.Generation, nil
		}),
		TaskEnqueueFn:    BuildScheduleTaskEnqueueFunc(taskRuntimeService),
		RuntimeHandlerFn: BuildScheduleRuntimeHandlerFn(supervisor),
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
	eventResolver := BuildEventEffectiveResolver(permBroker, scopeManager, dependencyResolver, supervisor, eventSvc.GetDispatcher(), enablementResolver, instRepo)
	if err := eventSvc.SetEffectiveResolver(eventResolver); err != nil {
		return nil, fmt.Errorf("kernel: set event effective resolver: %w", err)
	}
	genResolver := NewEventGenerationResolverAdapter(instRepo)
	eventSvc.SetGenerationResolver(genResolver)
	eventSvc.GetDispatcher().SetGenerationResolver(genResolver)
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
	permIDValidator := permission.NewPermissionIDValidator(permDefRegistry)
	if permSnapshotStore != nil {
		_, _ = permSnapshotStore.RevokeInvalidSnapshots(ctx, permIDValidator)
	}
	hostAPIGateway.SetPermissionChecker(host_api.NewBrokerPermissionChecker(permBroker))
	hostAPIGateway.SetScopeChecker(host_api.NewManagerScopeChecker(scopeManager, host_api.NewSnapshotStoreAdapter(scopeStore)))
	hostAPIGateway.SetPermissionSnapshotChecker(host_api.NewBrokerPermissionSnapshotChecker(host_api.NewPermissionSnapshotStoreAdapter(permSnapshotStore)).WithValidator(permIDValidator))
	hostAPIGateway.SetAuditWriter(newHostAPIAuditWriter(sqlite.NewHostAPIAuditRepository(db)))
	extensionStateStore := sqlite.NewExtensionStateStore(db)
	charReader := b.characterReader
	if charReader == nil {
		charReader = NewDefaultCharacterReader()
	}
	convReader := b.conversationReader
	if convReader == nil {
		convReader = NewDefaultConversationReader()
	}
	memQuerySvc := b.memoryQueryService
	if memQuerySvc == nil {
		memQuerySvc = NewDefaultMemoryQueryService()
	}
	uiHostNotifier := NewSSEUIHostNotifier(sse.Global)
	bridgeClipboardHost := NewBridgeClipboardHost(sse.Global)
	if err := setupDefaultHostAPIRoutes(hostAPIGateway, HostAPIRouteDeps{
		StateStore:          extensionStateStore,
		CharacterReader:     charReader,
		ConversationReader:  convReader,
		MemoryQueryService:  memQuerySvc,
		UIHostNotifier:      uiHostNotifier,
		ClipboardHost:       bridgeClipboardHost,
		RuntimeSupervisor:   supervisor,
		EventService:        eventSvc,
		ScheduleService:     scheduleSvc,
		ExecutionKernel:     executionKernel,
		ToolRegistry:        toolRegistry,
		OperationRepository: opRepo,
		ExtensionRoot:       b.extRoot,
		ScopeSnapshotStore:  host_api.NewSnapshotStoreAdapter(scopeStore),
	}); err != nil {
		return nil, fmt.Errorf("kernel: setup host api routes: %w", err)
	}
	if err := setupMigrationSandboxRoutes(hostAPIGateway, MigrationSandboxDeps{
		DB: store.DB(),
	}); err != nil {
		return nil, fmt.Errorf("kernel: setup migration sandbox routes: %w", err)
	}
	scheduleSvc.GetExecutor().RegisterTargetAdapter(schedule.NewToolTargetAdapter(NewKernelToolExecutorAdapter(executionKernel)))
	jsFactory.SetHostAPI(hostAPIGateway)

	jsSupervisorFactory := javascript_main.NewSupervisorFactory(jsFactory, nodeResolver, artifactResolver)
	_ = supervisor.RegisterFactory(jsSupervisorFactory)

	builtinDispatcher := func(ctx context.Context, handlerName string, input json.RawMessage, invocation capability.ToolInvocationContext) (json.RawMessage, error) {
		execCtx := tool.ToolExecutionContext{
			Context:        ctx,
			ConversationID: invocation.ConversationID,
			CharacterID:    invocation.CharacterID,
			User:           invocation.UserID,
			Path:           "kernel.builtin",
			ToolCallID:     invocation.InvocationID,
			IdempotencyKey: invocation.IdempotencyKey,
		}
		result, ok := tool.ExecuteWithContextAndCancel(ctx, execCtx, handlerName, string(input))
		if !ok {
			memResult, memOk := tool.ExecuteMemoryWithContextAndCancel(ctx, execCtx, handlerName, string(input))
			if !memOk {
				return nil, fmt.Errorf("builtin handler %s not found", handlerName)
			}
			return json.Marshal(memResult)
		}
		return json.Marshal(result)
	}

	if err := RegisterProductionAdapters(adapterRegistry, AdapterRegistrationDeps{
		JSGlobalFactory:   jsFactory,
		WASMFactory:       wasmFactory,
		WASMModuleMgr:     wasmModuleMgr,
		Supervisor:        supervisor,
		TaskService:       taskRuntimeService,
		WorkflowCaller:    makeWorkflowCallFunc(workflowExecutor),
		WorkflowCancel:    makeWorkflowCancelFunc(workflowExecutor),
		BuiltinDispatcher: builtinDispatcher,
	}); err != nil {
		return nil, fmt.Errorf("kernel: register production adapters: %w", err)
	}
	registerWorkflowStepHandlers(workflowExecutor, executionKernel, adapterRegistry)

	toolFacade := NewToolFacade(toolRegistry, executionKernel, DefaultToolFacadeConfig())
	toolFacade.SetAgentSkillCatalog(agentSkillCatalog)

	uiHost := ui_contribution.NewUIHost()
	uiHost.Bridge().SetScopeSnapshotFactory(func(extensionID, moduleID string, generation int64, characterID, conversationID string) (string, error) {
		invocationID := fmt.Sprintf("ui-sess-%d", time.Now().UnixNano())
		scopes := []scope.ScopeRef{
			scope.NewExtensionScope(extensionID),
			scope.NewModuleScope(extensionID, moduleID),
			scope.NewSessionScope(invocationID),
		}
		if characterID != "" {
			scopes = append(scopes, scope.NewCharacterScope(characterID))
		}
		if conversationID != "" {
			scopes = append(scopes, scope.NewConversationScope(conversationID))
		}
		snapshot := scope.CreateSnapshot(invocationID, scopes, characterID, conversationID, extensionID, moduleID, generation)
		if err := scopeStore.SaveSnapshot(context.Background(), snapshot); err != nil {
			return "", err
		}
		return snapshot.SnapshotID, nil
	})
	uiHost.Bridge().SetPermissionSnapshotFactory(func(sessionID, extensionID, moduleID string, generation int64, characterID, conversationID string, grantedPerms []string, expiresAt time.Time) (string, error) {
		snap := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{
			SessionID:      sessionID,
			ExtensionID:    extensionID,
			ModuleID:       moduleID,
			Generation:     generation,
			CharacterID:    characterID,
			ConversationID: conversationID,
			GrantedPerms:   grantedPerms,
			Lifetime:       time.Until(expiresAt),
		})
		if err := snap.ValidateGrantedPerms(permIDValidator); err != nil {
			return "", err
		}
		if err := permSnapshotStore.SaveSnapshot(context.Background(), snap); err != nil {
			return "", err
		}
		return snap.SnapshotID, nil
	})
	uiHost.Bridge().SetSnapshotReleaser(func(scopeSnapshotID, permissionSnapshotID string) error {
		ctx := context.Background()
		if scopeSnapshotID != "" {
			_ = scopeStore.DeleteSnapshot(ctx, scopeSnapshotID)
		}
		if permissionSnapshotID != "" {
			_ = permSnapshotStore.DeleteSnapshot(ctx, permissionSnapshotID)
		}
		return nil
	})
	uiContribRepo := sqlite.NewSQLiteUIContributionRepository(store.DB())
	savedContribs, _ := uiContribRepo.ListAll(ctx)
	slotRegistry := extension_slots.DefaultSlotRegistry()
	pageRegistry := extension_page_host.NewPageRegistry()
	pageSessionMgr := extension_page_host.NewSessionManager()
	relationChecker.sessions = pageSessionMgr
	uiPermValidator := permission.NewUIPermissionValidator()
	pageHost := extension_page_host.NewPageHostWithValidator(pageRegistry, pageSessionMgr, uiPermValidator)
	schemaValidator := schema_ui.NewValidator()
	schemaCache := schema_ui.NewCompilerCache()
	schemaRegistry := schema_ui.NewSchemaRegistry(schemaValidator, schemaCache)
	for _, def := range savedContribs {
		if def.Entry.SchemaPath != "" || def.Sandbox.Type == ui_contribution.SandboxSchemaRenderer {
			basePath := resolveExtensionBundlePath(b.extRoot, string(def.ExtensionID))
			if basePath == "" || schemaRegistry.LoadFromPathWithContext(string(def.ExtensionID), string(def.ContributionID), def.Integrity.Generation, "", "", basePath, def.Entry.SchemaPath) != nil {
				continue
			}
		}
		_ = uiHost.RegisterContribution(def)
	}
	sandboxHost := sandbox_webui.NewHost()
	sandboxHost.SetScopeSnapshotFactory(func(extensionID, moduleID string, generation int64, characterID, conversationID string) (string, error) {
		invocationID := fmt.Sprintf("sandbox-sess-%d", time.Now().UnixNano())
		scopes := []scope.ScopeRef{
			scope.NewExtensionScope(extensionID),
			scope.NewModuleScope(extensionID, moduleID),
			scope.NewSessionScope(invocationID),
		}
		if characterID != "" {
			scopes = append(scopes, scope.NewCharacterScope(characterID))
		}
		if conversationID != "" {
			scopes = append(scopes, scope.NewConversationScope(conversationID))
		}
		snapshot := scope.CreateSnapshot(invocationID, scopes, characterID, conversationID, extensionID, moduleID, generation)
		if err := scopeStore.SaveSnapshot(context.Background(), snapshot); err != nil {
			return "", err
		}
		return snapshot.SnapshotID, nil
	})
	sandboxHost.SetPermissionSnapshotFactory(func(sessionID, extensionID, moduleID string, generation int64, characterID, conversationID string, grantedPerms []string, expiresAt time.Time) (string, error) {
		snap := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{
			SessionID:      sessionID,
			ExtensionID:    extensionID,
			ModuleID:       moduleID,
			Generation:     generation,
			CharacterID:    characterID,
			ConversationID: conversationID,
			GrantedPerms:   grantedPerms,
			Lifetime:       time.Until(expiresAt),
		})
		if err := snap.ValidateGrantedPerms(permIDValidator); err != nil {
			return "", err
		}
		if err := permSnapshotStore.SaveSnapshot(context.Background(), snap); err != nil {
			return "", err
		}
		return snap.SnapshotID, nil
	})
	sandboxHost.SetSnapshotReleaser(func(scopeSnapshotID, permissionSnapshotID string) error {
		ctx := context.Background()
		if scopeSnapshotID != "" {
			_ = scopeStore.DeleteSnapshot(ctx, scopeSnapshotID)
		}
		if permissionSnapshotID != "" {
			_ = permSnapshotStore.DeleteSnapshot(ctx, permissionSnapshotID)
		}
		return nil
	})
	hostCmdRegistry := NewHostCommandRegistry()
	if err := SetupDefaultHostCommands(hostCmdRegistry, hostAPIGateway); err != nil {
		return nil, fmt.Errorf("kernel: setup host commands: %w", err)
	}
	actionExecutor := NewUIActionExecutor(hostAPIGateway, workflowExecutor, workflowExecRepo, hostCmdRegistry, opRepo)
	sandboxDispatcher := buildSandboxActionDispatcher(sandboxActionDispatcherDeps{
		getSession: sandboxHost.GetSession,
		getContribution: func(contributionID string) (*ui_contribution.UIContributionDefinition, error) {
			return uiHost.GetContribution(ui_contribution.ContributionID(contributionID))
		},
		executeAction: actionExecutor.Execute,
	})
	dataSourceProvider := NewUIDataSourceProvider(hostAPIGateway)
	sandboxDataProvider := sandbox_webui.NewBridgeDataSourceProviderWithHandler(func(ctx context.Context, sessionID, sourceID string, params json.RawMessage) (json.RawMessage, error) {
		session, err := sandboxHost.GetSession(sessionID)
		if err != nil {
			return nil, fmt.Errorf("sandbox: session not found: %w", err)
		}
		if !session.IsDataSourceAllowed(sourceID) {
			return nil, fmt.Errorf("sandbox: data source %s not allowed for session %s", sourceID, sessionID)
		}
		return dataSourceProvider.Query(ctx, sessionID, session.ExtensionID, session.ModuleID, sourceID, session.ScopeSnapshotID, session.PermissionSnapshotID, params)
	})
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
	uiHost.Bridge().SetHandlers(
		func(ctx context.Context, session *ui_contribution.BridgeSession, action *ui_contribution.UIActionDefinition, input json.RawMessage) (json.RawMessage, error) {
			return actionExecutor.Execute(ctx, UIActionExecContext{
				SessionID:            session.SessionID,
				ContributionID:       session.ContributionID,
				ExtensionID:          session.ExtensionID,
				ModuleID:             session.ModuleID,
				Generation:           session.Generation,
				ScopeSnapshotID:      session.ScopeSnapshotID,
				PermissionSnapshotID: session.PermissionSnapshotID,
				CharacterID:          session.CharacterID,
				ConversationID:       session.ConversationID,
			}, action, input)
		},
		func(ctx context.Context, session *ui_contribution.BridgeSession, sourceID string, params json.RawMessage) (json.RawMessage, error) {
			return dataSourceProvider.Query(ctx, session.SessionID, session.ExtensionID, session.ModuleID, sourceID, session.ScopeSnapshotID, session.PermissionSnapshotID, params)
		},
	)
	chatExtRegistry := chat_ui_extension.NewChatExtensionRegistry()
	orderingEngine := ui_ordering.NewOrderingEngine()

	desktopHost := desktop.NewDesktopHost()
	updateBaseDir := filepath.Join(b.extRoot, "update-downloads")
	if b.extRoot == "" {
		updateBaseDir = filepath.Join(os.TempDir(), "amitia-update-downloads")
	}
	updateManager := desktop_update.NewUpdateManager(updateBaseDir, "26.1.8")
	updateAdapter := NewUpdateManagerAdapter(updateManager, desktopHost)
	desktopActionBridge := NewDesktopActionBridge(permBroker, scopeManager, executionKernel)
	desktopHost.SetPermissionChecker(desktopActionBridge)
	desktopHost.SetScopeChecker(desktopActionBridge)
	desktopHost.SetActionExecutor(desktopActionBridge)
	desktopAPI := desktop.NewDesktopAPI(desktopHost)
	updateAPI := desktop.NewUpdateAPI(updateAdapter)

	amitiaxInstaller := amitiax.NewInstaller()

	devModeStore := dev_mode.NewSQLiteWorkspaceStore(db)
	devModeRegistry := dev_mode.NewWorkspaceRegistry()
	devModePipeline := dev_mode.NewRebuildPipeline(nodeResolver)
	devModePreserver := dev_mode.NewStatePreserver()
	devModeReloader := dev_mode.NewRuntimeReloader(devModeRegistry, devModePipeline, devModePreserver)
	devModeSessions := dev_mode.NewSessionManager(8 * time.Hour)
	if err := restorePackageTrustMutations(ctx, packageTrustRepo, trustService, packageRepo, devModeSessions); err != nil {
		return nil, fmt.Errorf("kernel: replay package trust mutations: %w", err)
	}

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
	devConsoleHandler.SetHostAPIAuditQuery(newContainerHostAPIAuditQuery(sqlite.NewHostAPIAuditRepository(db)))

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

		PackageSecurity:        packageSec,
		PackageRepository:      packageRepo,
		PackageArtifactStore:   packageArtifactStore,
		PackageGenerationStore: packageGenerationStore,
		PackageTrustRepository: packageTrustRepo,
		UserDataSnapshotStore:  userDataSnapshotStore,
		ResourceSnapshotStore:  resourceSnapshotStore,
		PackageSnapshotRepo:    packageSnapshotRepo,
		LifecycleManager:       lifecycleMgr,
		ContributionRegistry:   contribRegistry,
		ContributionInstaller:  typedInstaller,
		DependencyResolver:     dependencyResolver,
		RuntimeSupervisor:      supervisor,
		ExecutionKernel:        executionKernel,
		HostAPIGateway:         hostAPIGateway,
		PermissionBroker:       permBroker,
		ScopeManager:           scopeManager,
		ScopeSnapshotCreator: func(extensionID, moduleID string, generation int64, characterID, conversationID string) (string, error) {
			invocationID := fmt.Sprintf("ui-sess-%d", time.Now().UnixNano())
			scopes := []scope.ScopeRef{
				scope.NewExtensionScope(extensionID),
				scope.NewModuleScope(extensionID, moduleID),
				scope.NewSessionScope(invocationID),
			}
			if characterID != "" {
				scopes = append(scopes, scope.NewCharacterScope(characterID))
			}
			if conversationID != "" {
				scopes = append(scopes, scope.NewConversationScope(conversationID))
			}
			snapshot := scope.CreateSnapshot(invocationID, scopes, characterID, conversationID, extensionID, moduleID, generation)
			if err := scopeStore.SaveSnapshot(context.Background(), snapshot); err != nil {
				return "", err
			}
			return snapshot.SnapshotID, nil
		},
		AgentSkillCatalog:      agentSkillCatalog,
		WorkflowRegistry:       workflowRegistry,
		WorkflowExecutor:       workflowExecutor,
		WorkflowTriggerManager: workflowTriggerManager,
		WorkflowDefRepo:        workflowDefRepo,
		WorkflowExecRepo:       workflowExecRepo,
		EnablementService:      enablementService,
		EnablementResolver:     enablementResolver,

		ToolRegistry:    toolRegistry,
		AdapterRegistry: adapterRegistry,
		ToolFacade:      toolFacade,

		HostCommandRegistry: hostCmdRegistry,

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

		UIHost:                uiHost,
		UIHostNotifier:        uiHostNotifier,
		ClipboardHostBridge:   bridgeClipboardHost,
		UIContributionRepo:    uiContribRepo,
		SlotRegistry:          slotRegistry,
		PageHost:              pageHost,
		SchemaValidator:       schemaValidator,
		SchemaCompilerCache:   schemaCache,
		SchemaRegistry:        schemaRegistry,
		SandboxHost:           sandboxHost,
		ChatExtensionRegistry: chatExtRegistry,
		OrderingEngine:        orderingEngine,
		ExtRoot:               b.extRoot,

		DesktopHost:         desktopHost,
		UpdateManager:       updateManager,
		UpdateAdapter:       updateAdapter,
		DesktopAPI:          desktopAPI,
		UpdateAPI:           updateAPI,
		DesktopActionBridge: desktopActionBridge,

		DevConsoleService: devConsoleSvc,
		DevConsoleRepo:    devConsoleRepo,
		DevConsoleHandler: devConsoleHandler,
		TrustService:      trustService,
		AmitiaxInstaller:  amitiaxInstaller,
		DevModeRegistry:   devModeRegistry,
		DevModePipeline:   devModePipeline,
		DevModeReloader:   devModeReloader,
		DevModeSessions:   devModeSessions,
		DevModeStore:      devModeStore,

		UpdateRecoveryManager: recoveryMgr,

		MigrationRepository:    migrationRepo,
		MigrationExecutor:      migrationExecutor,
		MigrationGraphResolver: migrationGraphResolver,
		MigrationPlanner:       migrationPlanner,
		MigrationSnapshotMgr:   migrationSnapshotMgr,
		MigrationCheckpointMgr: migrationCheckpointMgr,
		MigrationValidator:     migrationValidator,

		RollbackRepository:  rollbackRepo,
		JournalManager:      journalMgr,
		RollbackExecutorV2:  rollbackExecutorV2,
		RollbackPlanner:     rollbackPlanner,
		RollbackPointStore:  rollbackPointStore,
		UpdateGenerationMgr: generationMgr,
		UpdateMigrationExec: updateMigrationExecutor,

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

	typedInstaller.SetContainer(container)

	candidateNS := NewCandidateNamespace()
	typedInstaller.SetCandidateNamespace(candidateNS)

	candidateRepo := NewCandidateRepository(store.DB())
	candidateMgr := NewCandidateContributionManager(typedInstaller, generationMgr, supervisor, candidateRepo, candidateNS)
	container.CandidateMgr = candidateMgr
	container.CandidateNS = candidateNS
	container.CandidateRepository = candidateRepo
	container.PageSessionRepository = sqlite.NewSQLitePageSessionRepository(store.DB())

	candidateRunner := NewProductionCandidateRunner(
		supervisor,
		typedInstaller,
		generationMgr,
		candidateMgr,
		b.extRoot,
	).WithCleanupRepo(NewRuntimeCleanupRepository(db))
	devModeReloader.SetCandidateRunner(candidateRunner)
	devModeReloader.SetSessionManager(devModeSessions)
	devModeReloader.SetCleanupFailureStore(NewSQLiteCleanupFailureStore(db))

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

package kernel

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/browser"
	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/desktoppet/integration"
	"github.com/u-ai/backend/internal/desktoppet/plugin_boundary"
	"github.com/u-ai/backend/internal/devicemesh/server"
	"github.com/u-ai/backend/internal/deviceruntime"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/authority"
	"github.com/u-ai/backend/internal/extension/kernel/builtin"
	"github.com/u-ai/backend/internal/extension/kernel/canary"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
	"github.com/u-ai/backend/internal/extension/kernel/contribution"
	"github.com/u-ai/backend/internal/extension/kernel/dependency"
	"github.com/u-ai/backend/internal/extension/kernel/desktop"
	"github.com/u-ai/backend/internal/extension/kernel/desktop_update"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
	"github.com/u-ai/backend/internal/extension/kernel/developer_console"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/enablement"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/eventbridge"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/extension_center"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/extension_slots"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/javascript_main"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	kernelmcp "github.com/u-ai/backend/internal/extension/kernel/mcp"
	kernelmcpinstaller "github.com/u-ai/backend/internal/extension/kernel/mcp/installer"
	"github.com/u-ai/backend/internal/extension/kernel/migration"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"github.com/u-ai/backend/internal/extension/kernel/package_security"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
	"github.com/u-ai/backend/internal/extension/kernel/runtime_supervisor"
	"github.com/u-ai/backend/internal/extension/kernel/sandbox_webui"
	"github.com/u-ai/backend/internal/extension/kernel/schedule"
	"github.com/u-ai/backend/internal/extension/kernel/schema_ui"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/script_host"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_ordering"
	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
	"github.com/u-ai/backend/internal/extension/kernel/update"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/gamehost"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/upgrade"
	"github.com/u-ai/backend/internal/imagegen"
	"github.com/u-ai/backend/internal/imageintelligence"
	"github.com/u-ai/backend/internal/imageprovider"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval/local"
	"github.com/u-ai/backend/internal/mcp"
	"github.com/u-ai/backend/internal/media"
	"github.com/u-ai/backend/internal/nativebridge"
	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/internal/runtimehost"
	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/internal/runtimeorchestrator"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/uiagent"
	"github.com/u-ai/backend/internal/uiagent/preview"
	"github.com/u-ai/backend/internal/uiagent/preview/adapters"
	"github.com/u-ai/backend/internal/uiagent/schema"
	"github.com/u-ai/backend/internal/uiagent/source"
	"github.com/u-ai/backend/internal/vision"
	"github.com/u-ai/backend/internal/workspace"
	"github.com/u-ai/backend/pkg/resourceuri"
	"github.com/u-ai/backend/pkg/sse"
)

type ContainerBuilder struct {
	dbPath                       string
	extRoot                      string
	db                           *sql.DB
	characterReader              CharacterReader
	conversationReader           ConversationReader
	memoryQueryService           MemoryQueryService
	nodeEnvironmentResolver      script_host.NodeEnvironmentResolver
	hostArtifactResolver         script_host.ArtifactResolver
	androidLinuxProvider         interface{}
	androidNativeProvider        capability.AndroidProvider
	iosNativeProvider            capability.IOSProvider
	host                         runtimehost.RuntimeHost
	searchConfig                 search.Config
	deepSearchTaskEntry          string
	visionSvc                    vision.Service
	imagegenSvc                  imagegen.Service
	imageProviderRegistry        *imageprovider.Registry
	resourceResolver             *resourceuri.PhysicalResolver
	gameHostArchiveUpdater       GameHostArchiveUpdater
	mediaService                 *media.Service
	workspaceService             *workspace.Service
	browserProvider              browser.BrowserProvider
	desktopPetPluginCapabilities *integration.DesktopPetPluginCapabilities
	runtimeProfile               runtimeprofile.Profile
	runtimePolicy                runtimeprofile.Policy

	mcpRepository         *mcp.Repository
	mcpRuntimeConnectPort acquisition.MCPRuntimeConnectPort

	meshHub                  *server.ConnectionHub
	pendingInvocationManager *capability.PendingInvocationManager
	pendingTaskManager       *task_runtime.PendingTaskManager

	nativeBridgeRelay *nativebridge.RelayHandler

	backgroundBootstrapFunc func() (backgroundremoval.Registry, error)

	workshopModelGenerator WorkshopModelGenerator

	channelStore capability.ChannelStore
}

type WorkshopModelGenerator interface {
	GenerateWorkshopJSON(ctx context.Context, systemPrompt string, userPrompt string) (string, string, string, error)
}

func NewContainerBuilder() *ContainerBuilder {
	authority.MustValidate()
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

func (b *ContainerBuilder) WithDesktopPetPluginCapabilities(caps integration.DesktopPetPluginCapabilities) *ContainerBuilder {
	b.desktopPetPluginCapabilities = &caps
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

func (b *ContainerBuilder) WithMeshHub(hub *server.ConnectionHub) *ContainerBuilder {
	b.meshHub = hub
	return b
}

func (b *ContainerBuilder) WithPendingInvocationManager(mgr *capability.PendingInvocationManager) *ContainerBuilder {
	b.pendingInvocationManager = mgr
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

func (b *ContainerBuilder) WithNativeBridgeRelay(relay *nativebridge.RelayHandler) *ContainerBuilder {
	b.nativeBridgeRelay = relay
	return b
}

func (b *ContainerBuilder) WithRuntimeHost(
	host runtimehost.RuntimeHost,
) *ContainerBuilder {
	b.host = host
	return b
}

func (b *ContainerBuilder) WithWorkshopModelGenerator(model WorkshopModelGenerator) *ContainerBuilder {
	b.workshopModelGenerator = model
	return b
}

func (b *ContainerBuilder) WithSearchConfig(cfg search.Config) *ContainerBuilder {
	b.searchConfig = cfg
	return b
}

func (b *ContainerBuilder) WithDeepSearchTaskEntry(entry string) *ContainerBuilder {
	b.deepSearchTaskEntry = entry
	return b
}

func (b *ContainerBuilder) WithVisionService(svc vision.Service) *ContainerBuilder {
	b.visionSvc = svc
	return b
}

func (b *ContainerBuilder) WithImageGenService(svc imagegen.Service) *ContainerBuilder {
	b.imagegenSvc = svc
	return b
}

func (b *ContainerBuilder) WithImageProviderRegistry(reg *imageprovider.Registry) *ContainerBuilder {
	b.imageProviderRegistry = reg
	return b
}

func (b *ContainerBuilder) WithResourceResolver(resolver *resourceuri.PhysicalResolver) *ContainerBuilder {
	b.resourceResolver = resolver
	return b
}

func (b *ContainerBuilder) WithAndroidNativeProvider(provider capability.AndroidProvider) *ContainerBuilder {
	b.androidNativeProvider = provider
	return b
}

func (b *ContainerBuilder) WithIOSNativeProvider(provider capability.IOSProvider) *ContainerBuilder {
	b.iosNativeProvider = provider
	return b
}

type GameHostArchiveUpdater interface {
	UpdateArchive(ctx context.Context, extensionID string, archivePath string) (*gamehostKernelUpdateResult, error)
	GetPreviousArchivePath(ctx context.Context, extensionID string) (string, error)
}

type gamehostKernelUpdateResult = upgrade.KernelUpdateResult

func (b *ContainerBuilder) WithGameHostArchiveUpdater(updater GameHostArchiveUpdater) *ContainerBuilder {
	b.gameHostArchiveUpdater = updater
	return b
}

func (b *ContainerBuilder) WithMediaService(svc *media.Service) *ContainerBuilder {
	b.mediaService = svc
	return b
}

func (b *ContainerBuilder) WithWorkspaceService(svc *workspace.Service) *ContainerBuilder {
	b.workspaceService = svc
	return b
}

func (b *ContainerBuilder) WithBrowserProvider(provider browser.BrowserProvider) *ContainerBuilder {
	b.browserProvider = provider
	return b
}

func (b *ContainerBuilder) WithRuntimeProfile(profile runtimeprofile.Profile) *ContainerBuilder {
	b.runtimeProfile = profile
	b.runtimePolicy = runtimeprofile.PolicyFor(profile)
	return b
}

func (b *ContainerBuilder) WithBackgroundBootstrapFunc(fn func() (backgroundremoval.Registry, error)) *ContainerBuilder {
	b.backgroundBootstrapFunc = fn
	return b
}

func (b *ContainerBuilder) WithChannelStore(store capability.ChannelStore) *ContainerBuilder {
	b.channelStore = store
	return b
}

func (b *ContainerBuilder) WithMCPRepository(repo *mcp.Repository) *ContainerBuilder {
	b.mcpRepository = repo
	return b
}

func (b *ContainerBuilder) WithMCPRuntimeConnectPort(port acquisition.MCPRuntimeConnectPort) *ContainerBuilder {
	b.mcpRuntimeConnectPort = port
	return b
}

func (b *ContainerBuilder) WithPendingTaskManager(mgr *task_runtime.PendingTaskManager) *ContainerBuilder {
	b.pendingTaskManager = mgr
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
	permBroker.SetSnapshotStore(permSnapshotStore)
	permBroker.SetTrustLevelChecker(newRepositoryPermissionTrustChecker(instRepo, defRepo))

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
	trustedSupervisor := trusted_service.NewProcessSupervisorWithVerifier(
		trustedSvcRoot,
		trusted_service.NewBinaryVerifierWithManagedNode(newManagedNodeChecker(nodeResolver)),
	)
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
	packageArtifactStoreAdapter := &kernelArtifactStoreAdapter{store: packageArtifactStore}
	packageRepoAdapter := &kernelArtifactRegistryAdapter{repo: packageRepo}
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

	adapterRegistry := capability.NewRuntimeAdapterRegistry()
	toolRegistry := capability.NewToolRegistry()

	kernelSecretBroker, err := buildKernelSecretBroker(b.extRoot)
	if err != nil {
		return nil, fmt.Errorf("kernel: build secret broker: %w", err)
	}
	observabilityStore := observability.NewSQLiteStorage(db)
	observabilityWriter := observability.NewRecordWriter(observabilityStore, observability.DefaultWriterConfig())
	observabilitySanitizer := observability.NewRecordSanitizer()
	observabilitySanitizer.SetRedactor(kernelSecretBroker.Redactor())
	executionAuditHook := observability.NewExecutionHook(observabilityWriter, observabilitySanitizer)
	concurrencyCtrl, err := execution.NewConcurrencyController(execution.ConcurrencyPolicy{
		GlobalLimit:          100,
		PerToolLimit:         0,
		PerExtensionLimit:    0,
		PerCharacterLimit:    0,
		PerConversationLimit: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("kernel: create concurrency controller: %w", err)
	}
	execution.SetConcurrencyObservabilityHooks(func(permits []execution.ConcurrencyPermit) {
		dims := make([]string, 0, len(permits))
		for _, p := range permits {
			dims = append(dims, string(p.Key.Dimension))
		}
		_ = executionAuditHook.OnConcurrencyAcquired(context.Background(), dims)
	}, func(permits []execution.ConcurrencyPermit, waitDuration time.Duration) {
		dims := make([]string, 0, len(permits))
		for _, p := range permits {
			dims = append(dims, string(p.Key.Dimension))
		}
		_ = executionAuditHook.OnConcurrencyReleased(context.Background(), dims, waitDuration.Milliseconds())
	}, func(permits []execution.ConcurrencyPermit, waitDuration time.Duration) {
		dims := make([]string, 0, len(permits))
		for _, p := range permits {
			dims = append(dims, string(p.Key.Dimension))
		}
		_ = executionAuditHook.OnConcurrencyWait(context.Background(), dims, waitDuration.Milliseconds())
	})
	rateLimitPolicy := execution.RateLimitPolicy{Enabled: false}
	rateLimiter, err := execution.NewRateLimiter(rateLimitPolicy)
	if err != nil {
		return nil, fmt.Errorf("kernel: create rate limiter: %w", err)
	}
	execution.SetRateLimitObservabilityHooks(func(dimensions []string) {
		_ = executionAuditHook.OnRateLimitAdmitted(context.Background(), dimensions)
	}, func(dimensions []string, reason string, retryAfterMs int64) {
		_ = executionAuditHook.OnRateLimitRejected(context.Background(), dimensions, reason, retryAfterMs)
	}, func(dimensions []string, reason string, retryAfterMs int64) {
		_ = executionAuditHook.OnBackpressureRejected(context.Background(), dimensions, reason, retryAfterMs)
	}, func(dimensions []string, waitMs int64) {
		_ = executionAuditHook.OnRateLimitWait(context.Background(), dimensions, waitMs)
	})

	capabilityProviderRegistry := capability.NewProviderRegistry()
	providerExecutionResolver := capability.NewProviderRuntimeExecutionResolver(&capability.ProviderRegistryExecutionLookup{Registry: capabilityProviderRegistry})

	executionKernel := &execution.ExecutionPipeline{
		InvocationValidator: execution.NewInvocationValidator(),
		InputValidator:      execution.NewInputValidator(),
		AvailabilityGate:    execution.NewAvailabilityGate(nil),
		ScopeGate:           execution.NewScopeGate(),
		PermissionGate:      execution.NewPermissionGate(),
		ApprovalGate:        execution.NewApprovalGate(),
		ConcurrencyCtrl:     concurrencyCtrl,
		RateLimiter:         rateLimiter,
		IdempotencyGuard:    execution.NewIdempotencyGuard(execution.NewExecutionIdempotencyStorage(db)),
		RetryCtrl:           execution.NewRetryController(),
		TimeoutCtrl:         execution.NewTimeoutController(30 * time.Second),
		CancellationCtrl:    execution.NewCancellationController(),
		DepthGuard:          execution.NewDepthGuard(),
		Dispatcher:          execution.NewRuntimeDispatcher(adapterRegistry, providerExecutionResolver),
		ResultValidator:     execution.NewResultValidator(),
		Sanitizer:           execution.NewSanitizer(),
		SideEffectRec:       execution.NewSideEffectRecorder(),
		AuditSink:           executionAuditHook,
		MetricsRec:          execution.NewMetricsRecorder(),
		CircuitBreaker:      execution.NewCircuitBreakerCoordinator(),
		SecretBroker:        kernelSecretBroker,
	}

	executionKernel.CircuitBreaker.SetEventHook(func(snapshot execution.CircuitSnapshot, from, to execution.CircuitState, reason string) {
		_ = executionAuditHook.OnCircuitStateChange(context.Background(), snapshot.Key, string(from), string(to), reason, snapshot.ConsecutiveFailures, string(snapshot.State))
	})
	executionKernel.ScopeGate.ScopeManager = scopeManager
	executionKernel.PermissionGate.Broker = permBroker
	executionKernel.ScopeStore = scopeStore
	executionKernel.PermissionSnapshotStore = permSnapshotStore

	platformProcessMgr := process.NewDefaultProcessManager()
	runtimeCapAdapter := execution.NewRuntimeCapabilityAdapter(execution.DefaultRuntimeCapabilities)
	platformIsoAdapter := execution.NewPlatformIsolationAdapter(platformProcessMgr.IsolationReport)
	resourceLimitResolver := execution.NewDefaultResourceLimitResolver(runtimeCapAdapter, platformIsoAdapter)
	executionKernel.ResourceQuotaCtrl = execution.NewResourceQuotaController(resourceLimitResolver)

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

	if err := validateExecutionWiring(executionKernel, adapterRegistry, toolRegistry); err != nil {
		return nil, fmt.Errorf("kernel: validate execution wiring: %w", err)
	}

	taskRepo := sqlite.NewTaskRepository(db)
	var taskRuntimeService *task_runtime.TaskRuntimeService
	var taskHandler *task_runtime.TaskRuntimeHandler
	if b.runtimePolicy.TaskRuntime {
		taskCfg := task_runtime.DefaultTaskRuntimeConfig()
		taskCfg.WorkspaceRoot = b.extRoot
		taskCfg.NodeEnvironmentResolver = nodeResolver
		taskCfg.HostArtifactResolver = artifactResolver
		taskRuntimeService = task_runtime.NewTaskRuntimeService(taskRepo, taskCfg)
		taskHandler = task_runtime.NewTaskRuntimeHandler(taskRuntimeService)
	}

	taskSupervisorAdapter := task_runtime.NewSupervisorAdapter(taskRuntimeService)
	_ = taskSupervisorAdapter

	scheduleRepo := sqlite.NewScheduleRepository(db)
	var scheduleSvc *schedule.ScheduleService
	if b.runtimePolicy.TaskRuntime {
		scheduleSvc, err = schedule.NewScheduleService(schedule.ScheduleDeps{
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

	var eventSvc *event.Service
	var eventBridge *event.RuntimeBridge
	var hostEmitter event.HostEventEmitter
	var lifecycleEmitter *event.LifecycleEventEmitter
	var eventBridgePublisher *eventbridge.Publisher

	if b.runtimePolicy.DurableEvents {
		eventSvc, err = event.NewService(event.DefaultServiceConfig().WithDB(db))
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

		eventBridge = event.NewRuntimeBridge(eventSvc)
		eventBridge.Attach()
		hostEmitter = event.NewHostEventEmitter(eventSvc)
		lifecycleEmitter = event.NewLifecycleEventEmitter(hostEmitter)
	}

	var petPluginSource plugin_boundary.KernelContributionSource
	if contribRepo != nil && instRepo != nil {
		petPluginSource = plugin_boundary.NewContainerSource(contribRepo, instRepo)
	}
	if b.desktopPetPluginCapabilities == nil {
		caps := integration.NewFixtureCapabilities()
		b.desktopPetPluginCapabilities = &caps
	}
	var petPluginBoundary *plugin_boundary.DesktopPetPluginBoundary
	if petPluginSource != nil && b.desktopPetPluginCapabilities != nil {
		boundary, err := plugin_boundary.NewProductionBoundary(petPluginSource, *b.desktopPetPluginCapabilities)
		if err != nil {
			return nil, fmt.Errorf("kernel: create production pet plugin boundary: %w", err)
		}
		petPluginBoundary = boundary
	}

	jsFactory := javascript_main.NewRuntimeFactory()
	eventDeliveryAdapter := javascript_main.NewEventDeliveryAdapter(jsFactory)
	if eventBridge != nil {
		eventBridge.SetDeliveryCallback(eventDeliveryAdapter.HandleDelivery)
	}

	if b.runtimePolicy.DurableEvents {
		eventBridgePublisher, err = eventbridge.NewPublisher(eventSvc)
		if err != nil {
			return nil, fmt.Errorf("kernel: create eventbridge publisher: %w", err)
		}
	}

	if b.runtimePolicy.DurableEvents && taskRuntimeService != nil {
		taskEventPublisher := eventbridge.NewTaskEventPublisher(eventBridgePublisher)
		taskRuntimeService.SetEventSink(taskEventPublisher)
	}

	hostAPIGateway := host_api.NewDefaultGateway()
	host_api.RegisterPermissionDefinitions(permDefRegistry)
	permIDValidator := permission.NewPermissionIDValidator(permDefRegistry)
	if permSnapshotStore != nil {
		_, _ = permSnapshotStore.RevokeInvalidSnapshots(ctx, permIDValidator, func(extensionID, moduleID string, generation int64) bool {
			installation, err := instRepo.GetInstallation(ctx, domain.ExtensionID(extensionID))
			if err != nil {
				return false
			}
			return installation.Generation == generation
		})
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

	if b.runtimePolicy.DurableEvents {
		if err := eventbridge.RegisterCloudFoundationEventTypes(ctx, eventSvc.EventTypeRegistry()); err != nil {
			return nil, fmt.Errorf("kernel: register cloud foundation event types: %w", err)
		}
	}

	deviceRegistry := host_registry.NewRegistry(db)
	if err := deviceRegistry.LoadFromStore(ctx); err != nil {
		return nil, fmt.Errorf("kernel: load device registry: %w", err)
	}

	uiHostNotifier := NewSSEUIHostNotifierWithRegistry(sse.Global, deviceRegistry)
	uiHostNotifier.SetClientRuntimeDatabase(db)
	tool.SetExecutionEventSink(func(eventCtx context.Context, execCtx tool.ToolExecutionContext, name string, result tool.ToolCallResult) {
		payload, err := json.Marshal(map[string]interface{}{
			"type":           "tool.invocation_completed",
			"conversationId": execCtx.ConversationID,
			"characterId":    execCtx.CharacterID,
			"channel":        execCtx.Channel,
			"requestId":      execCtx.RequestID,
			"toolCallId":     execCtx.ToolCallID,
			"toolName":       name,
			"status":         string(result.Status),
			"errorCode":      result.ErrorCode,
		})
		if err != nil {
			return
		}
		_, _, _ = eventSvc.PublishConversationUIEvent(eventCtx, execCtx.ConversationID, payload, execCtx.RequestID)
	})
	stateLoader := newContainerStateLoader(instRepo, defRepo, moduleRepo, contribRepo, runtimeRepo, stateStore)
	preflightChecker := newContainerPreflightChecker(dependencyResolver)
	typedInstaller := NewTypedContributionInstaller(nil)
	planExecutor := newContainerPlanExecutor(instRepo, defRepo, moduleRepo, contribRepo, stateStore, typedInstaller, packageRepo, packageArtifactStore, packageGenerationStore, packageSec, uiHostNotifier)
	lcAuditWriter := newContainerAuditWriter(opRepo)
	lifecycleMgr := lifecycle_manager.NewManager(stateLoader, preflightChecker, planExecutor, lcAuditWriter)

	var providerEventSink capability.ProviderEventSink
	if b.runtimePolicy.DurableEvents {
		providerEventSink = eventbridge.NewProviderEventEmitter(eventBridgePublisher)
	}
	providerLifecycle := capability.NewProviderLifecycleService(capabilityProviderRegistry, providerEventSink)

	deviceRuntimePresence := host_registry.NewDeviceRuntimePresenceAdapterWithCallback(deviceRegistry, func(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) {
		allInstances := capabilityProviderRegistry.SnapshotInstances()
		for _, inst := range allInstances {
			if inst == nil {
				continue
			}
			if inst.Availability != capability.ProviderAvailabilityAvailable {
				continue
			}
			if userID != "" && inst.UserID != userID {
				continue
			}
			if deviceID != "" && inst.DeviceID != deviceID {
				continue
			}
			if runtimeID != "" && inst.RuntimeID != runtimeID {
				continue
			}
			_ = providerLifecycle.UpdateInstanceAvailability(inst.ID, capability.ProviderAvailabilityUnavailable)
		}
	})
	providerExecutionResolver.SetSessionResolver(capability.NewHostRegistryDeviceSessionResolver(deviceRegistry))

	var sessionStore *deviceruntime.SQLiteSessionStore
	var sessionService *deviceruntime.Service
	if b.runtimeProfile == runtimeprofile.ProfileLocal || b.runtimeProfile == runtimeprofile.ProfileCloudCore || b.runtimeProfile == runtimeprofile.ProfileDeviceAgent {
		sessionStore = deviceruntime.NewSQLiteSessionStore(db)
		if err := sessionStore.EnsureSchema(ctx); err != nil {
			return nil, fmt.Errorf("kernel: ensure session store schema: %w", err)
		}
		sessionEvents := eventbridge.NewRuntimeSessionEventPublisher(eventBridgePublisher)
		sessionService, err = deviceruntime.NewService(sessionStore, deviceruntime.ServiceOptions{
			PresencePort: deviceRuntimePresence,
			Events:       sessionEvents,
		})
		if err != nil {
			return nil, fmt.Errorf("kernel: create device runtime session service: %w", err)
		}
	}

	if b.meshHub != nil && taskRuntimeService != nil {
		executor := task_runtime.NewMeshRemoteTaskExecutor(b.meshHub, sessionService)
		executor.SetProgressHandler(taskRuntimeService)
		executor.SetCheckpointHandler(taskRuntimeService)
		executor.SetCompletionHandler(taskRuntimeService)
		taskRuntimeService.SetRemoteExecutor(executor)
	}
	extensionProviderReconciler := capability.NewExtensionProviderReconciler(providerLifecycle, capabilityProviderRegistry)
	providerInstanceReconciler := capability.NewProviderInstanceReconcilerWithAdapters(providerLifecycle, capabilityProviderRegistry, runtimeidentity.Identity{}, adapterRegistry)
	builtinProviderReconciler := capability.NewBuiltinProviderReconciler(capabilityProviderRegistry, providerLifecycle, adapterRegistry)
	capabilityResolver := capability.NewResolver(capability.NewProviderCatalogAdapter(capabilityProviderRegistry))
	capabilityResolver.SetRuntimeCatalog(capability.NewRuntimeAdapterCatalogAdapter(adapterRegistry))

	var runtimeStateService *runtimeorchestrator.RuntimeStateService
	if sessionService != nil {
		runtimeStateAdapter := NewRuntimeStateServiceBridge(sessionService, deviceRegistry)
		instanceAdapter := NewProviderInstanceBridge(capabilityProviderRegistry)
		runtimeStateService = runtimeorchestrator.NewRuntimeStateService(runtimeStateAdapter, instanceAdapter)
	} else {
		runtimeStateService = runtimeorchestrator.NewRuntimeStateService(nil, nil)
	}
	runtimeAvailabilityAdapter := capability.NewRuntimeAvailabilityAdapter(runtimeStateService)
	capabilityResolver.SetAvailability(runtimeAvailabilityAdapter)

	capabilityService := capability.NewCapabilityService(capabilityProviderRegistry)

	builtinCatalog := builtin.NewCatalog()
	builtinHandlerRegistry := builtin.NewHandlerRegistry()
	builtinBootstrapper := builtin.NewBootstrapper(builtinCatalog, defRepo, instRepo)
	builtinBootstrapper.SetModuleRepository(moduleRepo)
	builtinBootstrapper.SetContributionRepository(contribRepo)
	builtinBootstrapper.SetProviderReconciler(extensionProviderReconciler)
	builtinBootstrapper.SetEnableFunc(func(ctx context.Context, extID domain.ExtensionID) error {
		inst, err := instRepo.GetInstallation(ctx, extID)
		if err != nil {
			return fmt.Errorf("enable builtin %s: get installation: %w", extID, err)
		}
		inst.EnablementState = domain.EnablementEnabled
		inst.UpdatedAt = time.Now().UTC()
		if err := instRepo.PutInstallation(ctx, inst); err != nil {
			return fmt.Errorf("enable builtin %s: persist installation: %w", extID, err)
		}
		return nil
	})

	if err := builtin.ApplyBuiltinRegistrations(builtinCatalog); err != nil {
		return nil, fmt.Errorf("register builtin extensions: %w", err)
	}

	initCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := builtinBootstrapper.Reconcile(initCtx); err != nil {
		cancel()
		return nil, fmt.Errorf("builtin reconcile failed: %w", err)
	}

	var bgRegistry backgroundremoval.Registry
	if b.backgroundBootstrapFunc != nil {
		bgRegistry, err = b.backgroundBootstrapFunc()
		if err != nil {
			cancel()
			return nil, fmt.Errorf("background removal bootstrap failed: %w", err)
		}
	}
	if bgRegistry == nil {
		bgRegistry = backgroundremoval.NewRegistry()
	}
	if err := bgRegistry.Register(local.NewLocalProvider(), local.LocalCapabilities()); err != nil {
		cancel()
		return nil, fmt.Errorf("background removal register local provider failed: %w", err)
	}
	cancel()

	providerInvocationService := capability.NewProviderInvocationService(capabilityService, adapterRegistry)
	kernelProviderInvoker := NewKernelProviderInvoker(providerInvocationService)

	// CapabilityChannelResolver 是正式的 Channel 发现机制：
	// channel.deliver.* → CapabilityService → ProviderInvocation
	// 原 BuildChannelResolverFromConfig 只作为 Builtin Channel Provider 内部实现（fallback）
	capabilityChannelInvoker := delivery.NewProviderInvocationCapabilityInvoker(providerInvocationService, "")
	builtinChannelResolver := delivery.BuildChannelResolverFromConfig()
	capabilityChannelResolver := delivery.NewCapabilityChannelResolver(capabilityChannelInvoker, builtinChannelResolver)
	if b.channelStore == nil {
		b.channelStore = delivery.NewResolverChannelStore(capabilityChannelResolver)
	}

	acquisitionSourceRegistry := acquisition.NewSourceRegistry()

	var builtContainer *Container
	canonicalPackageInstaller := newAcquisitionPackageInstaller(func() *Container { return builtContainer })

	acquisitionSourceRegistry.Register(acquisition.NewInstalledSource(capabilityService, capabilityProviderRegistry))
	acquisitionSourceRegistry.Register(acquisition.NewAgentSkillSource(agentSkillCatalog))
	acquisitionSourceRegistry.Register(acquisition.NewGeneratedSkillSource(true))
	remoteSkillCatalog := acquisition.NewRemoteSkillCatalog("https://amitia.untrammelled.top/api/skills")
	acquisitionSourceRegistry.Register(acquisition.NewNewSkillSource(remoteSkillCatalog))

	extensionCenterService := extension_center.NewCenterService(extension_center.NewKernelCardProvider(defRepo, instRepo))
	acquisitionSourceRegistry.Register(acquisition.NewExtensionCatalogSource(extensionCenterService))

	if b.mcpRepository != nil {
		mcpAdapter := acquisition.NewMCPRepositoryAdapter(b.mcpRepository)
		acquisitionSourceRegistry.Register(acquisition.NewMCPPackageSource(mcpAdapter))
	}
	acquisitionSourceRegistry.Register(acquisition.NewRemoteCatalogSource("https://amitia.untrammelled.top/api/catalog"))
	acquisitionSourceRegistry.Register(acquisition.NewRemoteMCPCatalogSource("https://amitia.untrammelled.top/api/mcp"))

	mcpProvisioner := kernelmcpinstaller.NewDefaultProvisioner()
	mcpInstaller := kernelmcpinstaller.NewDefaultInstaller()
	mcpLifecycle := kernelmcp.NewMCPLifecycle(mcpProvisioner, mcpInstaller)
	workshopPort := acquisition.NewDefaultWorkshop()

	var mcpInstallPort acquisition.MCPInstallPort
	if b.mcpRepository != nil {
		mcpInstallPort = acquisition.NewMCPRepositoryBridge(b.mcpRepository)
	} else {
		mcpInstallPort = acquisition.NewMCPPortBridge(mcpLifecycle)
	}
	mcpToolSync := NewAcquisitionMCPToolSync(toolRegistry)
	if b.mcpRuntimeConnectPort != nil {
		mcpInstallPort = acquisition.NewMCPPortBridgeWithRuntime(mcpLifecycle, b.mcpRuntimeConnectPort, mcpToolSync)
	}

	acquisitionInstallerRegistry, err := acquisition.NewInstallerRegistry(&acquisition.InstallerRegistryOpts{
		EnableExistingPort: acquisition.NewEnableExistingPortBridgeWithDeps(acquisition.EnableExistingDeps{
			EnablementSvc:      enablementService,
			InstallRepo:        instRepo,
			DefinitionRepo:     defRepo,
			InstanceReconciler: providerInstanceReconciler,
			MCPRepository:      b.mcpRepository,
			MCPLifecycle:       mcpLifecycle,
			AgentSkillCatalog:  agentSkillCatalog,
			ProviderRegistry:   capabilityProviderRegistry,
		}),
		PackageInstallPort: acquisition.NewPackagePortBridgeWithCanonicalResolver(lifecycleMgr, packageArtifactStoreAdapter, packageRepoAdapter, canonicalPackageInstaller),
		MCPInstallPort:     mcpInstallPort,
		SkillInstallPort:   acquisition.NewSkillPortBridge(acquisition.NewSkillCatalogBridge(agentSkillCatalog)),
		WorkshopPort:       workshopPort,
	})
	if err != nil {
		return nil, fmt.Errorf("kernel: create installer registry: %w", err)
	}
	acquisitionService, err := acquisition.NewAcquisitionService(acquisition.AcquisitionDependencies{
		CapabilityService: capabilityService,
		ProviderRegistry:  capabilityProviderRegistry,
		SourceRegistry:    acquisitionSourceRegistry,
		InstallerRegistry: acquisitionInstallerRegistry,
		PolicyEngine:      acquisition.NewPolicyEngine(),
		DeploymentPlanner: acquisition.NewDeploymentPlanner(),
		ProviderLifecycle: providerLifecycle,
	})
	if err != nil {
		return nil, fmt.Errorf("kernel: create acquisition service: %w", err)
	}
	acquisitionBridge := acquisition.NewAgentCapabilityBridge(acquisitionService)
	if err := acquisition.RegisterAcquisitionTools(initCtx, toolRegistry); err != nil {
		return nil, fmt.Errorf("kernel: register acquisition tools: %w", err)
	}

	startupAt := time.Now().UTC()
	if sessionService != nil {
		if err := sessionService.RecoverStartup(ctx, startupAt); err != nil {
			return nil, fmt.Errorf("kernel: session startup recovery: %w", err)
		}
	}
	if err := deviceRegistry.MarkRuntimeEntriesDisconnectedOnStartup(ctx, startupAt); err != nil {
		return nil, fmt.Errorf("kernel: registry startup recovery: %w", err)
	}

	bridgeClipboardHost := NewBridgeClipboardHostWithRegistry(sse.Global, deviceRegistry)
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
		SecretStore:         nil,
		ProviderInvoker:     kernelProviderInvoker,
	}); err != nil {
		return nil, fmt.Errorf("kernel: setup host api routes: %w", err)
	}
	if err := setupMigrationSandboxRoutes(hostAPIGateway, MigrationSandboxDeps{
		DB: store.DB(),
	}); err != nil {
		return nil, fmt.Errorf("kernel: setup migration sandbox routes: %w", err)
	}
	if scheduleSvc != nil {
		scheduleSvc.GetExecutor().RegisterTargetAdapter(schedule.NewToolTargetAdapter(NewKernelToolExecutorAdapter(executionKernel)))
	}
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

	var imageIntelligenceHandler *imageintelligence.ToolHandler
	if b.visionSvc != nil || b.imagegenSvc != nil {
		imgIntFactory := imageintelligence.NewImageIntelligenceFactory(b.visionSvc, b.imagegenSvc, b.imageProviderRegistry, b.resourceResolver)
		imgIntFacade := imgIntFactory.Build()
		imageIntelligenceHandler = imageintelligence.NewToolHandler(imgIntFacade)
	}

	internalDispatcher := func(ctx context.Context, handlerName string, input json.RawMessage) (json.RawMessage, error) {
		if imageIntelligenceHandler == nil {
			return nil, fmt.Errorf("image intelligence not configured")
		}
		return imageIntelligenceHandler.Dispatch(ctx, handlerName, input)
	}

	mediaCaller := makeMediaCallFunc(b.mediaService)
	mediaHealth := makeMediaHealthFunc(b.mediaService)
	workspaceCaller := makeWorkspaceCallFunc(b.workspaceService)
	workspaceHealth := makeWorkspaceHealthFunc(b.workspaceService)

	var deviceRuntimePort capability.DeviceRuntimeInvocationPort
	if b.meshHub != nil && sessionService != nil {
		meshPorts := &capability.MeshRuntimePorts{
			Hub:                b.meshHub,
			SessionLookup:      sessionService,
			PendingInvocations: b.pendingInvocationManager,
		}
		deviceRuntimePort = capability.NewMeshDeviceRuntimeInvocationPort(meshPorts)
	}

	if err := RegisterProductionAdapters(adapterRegistry, AdapterRegistrationDeps{
		JSGlobalFactory:        jsFactory,
		WASMFactory:            wasmFactory,
		WASMModuleMgr:          wasmModuleMgr,
		Supervisor:             supervisor,
		TaskService:            taskRuntimeService,
		WorkflowCaller:         makeWorkflowCallFunc(workflowExecutor),
		WorkflowCancel:         makeWorkflowCancelFunc(workflowExecutor),
		BuiltinDispatcher:      builtinDispatcher,
		BuiltinHandlerVerifier: tool.HasHandler,
		AndroidLinuxProvider:   b.androidLinuxProvider,
		AndroidNativeProvider:  b.androidNativeProvider,
		SearchCaller:           makeSearchCallFunc(b.searchConfig, kernelSecretBroker),
		SearchHealth:           makeSearchHealthFunc(b.searchConfig, kernelSecretBroker),
		InternalDispatcher:     internalDispatcher,
		MediaCaller:            mediaCaller,
		MediaHealth:            mediaHealth,
		WorkspaceCaller:        workspaceCaller,
		WorkspaceHealth:        workspaceHealth,
		BrowserCaller:          makeBrowserCallFunc(b.browserProvider),
		BrowserHealth:          makeBrowserHealthFunc(b.browserProvider),
		DeviceRuntimePort:      deviceRuntimePort,
		BackgroundRemoval:      bgRegistry,
		ChannelStore:           b.channelStore,
	}); err != nil {
		return nil, fmt.Errorf("kernel: register production adapters: %w", err)
	}

	if err := builtinProviderReconciler.Reconcile(); err != nil {
		return nil, fmt.Errorf("builtin provider reconcile failed: %w", err)
	}

	if err := registerIOSToolsIfPresent(toolRegistry, b.iosNativeProvider); err != nil {
		return nil, fmt.Errorf("kernel: register ios native tools: %w", err)
	}
	if err := registerAndroidNativeToolsIfPresent(ctx, toolRegistry, b.androidNativeProvider); err != nil {
		return nil, fmt.Errorf("kernel: register android native tools: %w", err)
	}
	if err := registerDeepSearchSystemTask(ctx, taskRuntimeService, b.deepSearchTaskEntry); err != nil {
		return nil, fmt.Errorf("kernel: register deep search system task: %w", err)
	}
	registerWorkflowStepHandlers(workflowExecutor, executionKernel, adapterRegistry)

	if b.host != nil && b.androidLinuxProvider != nil {
		if err := registerTerminalTools(b.host, b.androidLinuxProvider, toolRegistry); err != nil {
			return nil, err
		}
	}

	toolFacade := NewToolFacade(toolRegistry, executionKernel, DefaultToolFacadeConfig())
	toolFacade.SetAgentSkillCatalog(agentSkillCatalog)
	toolFacade.SetCapabilityResolver(capabilityResolver)
	toolFacade.SetCapabilityService(capabilityService)
	toolFacade.SetAcquisitionBridge(acquisitionBridge)
	capabilityService.SetToolRegistry(toolRegistry)
	if hookService != nil {
		toolFacade.SetHookService(hookService)
	}

	recoveryService := acquisition.NewRecoveryService(acquisitionService)
	toolFacade.SetRecoveryService(recoveryService)

	builtin.SetUIAgentInspector(source.NewSourceInspectorWithWorkspace(b.workspaceService))
	preciseEditingSvc := workspace.NewDefaultPreciseEditingService(b.workspaceService)
	builtin.SetUIAgentPreciseService(preciseEditingSvc)
	var llmSchemaCall schema.LLMSchemaCallFunc
	if b.workshopModelGenerator != nil {
		llmSchemaCall = func(_ interface{}, promptJSON []byte) ([]byte, error) {
			raw, _, _, err := b.workshopModelGenerator.GenerateWorkshopJSON(context.Background(), "You are a SchemaUI generator. Return valid SchemaUIDocument JSON.", string(promptJSON))
			if err != nil {
				return nil, err
			}
			return []byte(raw), nil
		}
	}
	schemaGenerator := schema.NewAISchemaGenerator(schema.DefaultCatalog, llmSchemaCall)
	builtin.SetUIAgentAISchemaGenerator(schemaGenerator)
	previewSessionMgr := preview.NewSessionManager()
	previewValidator := schema.NewSchemaValidator(schema.DefaultCatalog)

	flutterAnalyzer := adapters.NewFlutterAnalyzer(b.workspaceService)
	webAnalyzer := adapters.NewWebAnalyzer(b.workspaceService)
	realDiagnosticRunner := adapters.NewRealDiagnosticRunner(flutterAnalyzer, webAnalyzer)

	previewObserver := preview.NewObserver(previewSessionMgr, previewValidator, preview.WithDiagnosticRunner(realDiagnosticRunner))
	patchGenerator := preview.NewDefaultPatchGeneratorWithCatalog(previewValidator, schema.DefaultCatalog)
	patchApplier := preview.NewDefaultApplierWithCatalog(previewSessionMgr, schema.DefaultCatalog)
	previewRefiner := preview.NewAutoRefinerWithPatch(previewSessionMgr, previewObserver, patchGenerator, patchApplier)
	builtin.SetUIAgentPreviewManager(previewSessionMgr)
	builtin.SetUIAgentObserver(previewObserver)
	builtin.SetUIAgentRefiner(previewRefiner)
	builtin.SetUIAgentDiagnosticRunner(realDiagnosticRunner)
	uiAgentExecutor := uiagent.NewUIExecutor(
		uiagent.WithPolicy(uiagent.DefaultPolicy()),
		uiagent.WithSchemaGenerator(schemaGenerator),
		uiagent.WithSourceEditor(source.NewSourceEditor(preciseEditingSvc)),
		uiagent.WithPreviewManager(previewSessionMgr),
		uiagent.WithObserver(previewObserver),
		uiagent.WithAutoRefiner(previewRefiner),
	)
	builtin.SetUIAgentExecutor(uiAgentExecutor)

	resumeRepo := coreexec.NewSQLiteResumeRepository(db)
	if err := resumeRepo.InitTable(ctx); err != nil {
		return nil, fmt.Errorf("kernel: init execution_resumes table: %w", err)
	}
	execService := coreexec.NewExecutionServiceWithRepository(resumeRepo)
	execService.RegisterResumeHandler(NewUISourceResumeHandler())
	execService.RegisterResumeHandler(NewUISchemaResumeHandler())
	execService.RegisterResumeHandler(NewApprovalResumeHandler())
	execService.RegisterResumeHandler(NewCapabilityAcquisitionResumeHandler(acquisitionService))
	if err := execService.LoadPendingResumes(ctx); err != nil {
		return nil, fmt.Errorf("kernel: load pending resumes: %w", err)
	}

	acquisitionService.SetExecution(acquisition.NewExecutionPort(execService))
	acquisitionService.SetResumeRepo(resumeRepo)

	runtimeState := NewRuntimeState()

	uiHost := ui_contribution.NewUIHost()
	uiProviderRegistry := ui_provider.NewRegistryWithBuiltins()
	uiProviderProfileStore := ui_provider.NewSQLiteProfileStore(db)
	if err := uiProviderProfileStore.Init(ctx); err != nil {
		return nil, fmt.Errorf("kernel: init UI provider profile store: %w", err)
	}
	if err := uiProviderRegistry.AttachStore(ctx, uiProviderProfileStore); err != nil {
		return nil, fmt.Errorf("kernel: load UI provider profile: %w", err)
	}
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
	rollbackExecutorV2.SetCompletionCallback(func(extensionID string, success bool, generation int64) {
		if uiHostNotifier != nil {
			if success {
				extra := map[string]interface{}{"status": "completed"}
				if generation > 0 {
					extra["generation"] = generation
				}
				uiHostNotifier.BroadcastExtensionChange("extension_rolled_back", extensionID, extra)
				uiHostNotifier.BroadcastExtensionChange("extension_generation_changed", extensionID, map[string]interface{}{"generation": generation})
			} else {
				uiHostNotifier.BroadcastExtensionChange("extension_rollback_failed", extensionID, map[string]interface{}{"status": "failed"})
			}
		}
	})
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
		PermissionDefinitions:  permDefRegistry,
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

		ExecutionService: execService,
		RuntimeState:     runtimeState,

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

		NativeBridgeRelay: b.nativeBridgeRelay,

		UIHost:              uiHost,
		UIHostNotifier:      uiHostNotifier,
		ClipboardHostBridge: bridgeClipboardHost,
		UIContributionRepo:  uiContribRepo,
		SlotRegistry:        slotRegistry,
		PageHost:            pageHost,
		SchemaValidator:     schemaValidator,
		SchemaCompilerCache: schemaCache,
		SchemaRegistry:      schemaRegistry,
		SandboxHost:         sandboxHost,
		OrderingEngine:      orderingEngine,
		UIProviderRegistry:  uiProviderRegistry,
		ExtRoot:             b.extRoot,

		DesktopHost:              desktopHost,
		UpdateManager:            updateManager,
		UpdateAdapter:            updateAdapter,
		DesktopAPI:               desktopAPI,
		UpdateAPI:                updateAPI,
		DesktopActionBridge:      desktopActionBridge,
		DesktopPetPluginBoundary: petPluginBoundary,

		ObservabilityStore: observabilityStore,

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

		RuntimeProfile:              b.runtimeProfile,
		RuntimePolicy:               b.runtimePolicy,
		DeviceRegistry:              deviceRegistry,
		DeviceRuntimePresence:       deviceRuntimePresence,
		DeviceRuntimeSessions:       sessionService,
		CapabilityProviders:         capabilityProviderRegistry,
		ProviderLifecycle:           providerLifecycle,
		ProviderExecutionResolver:   providerExecutionResolver,
		ExtensionProviderReconciler: extensionProviderReconciler,
		ProviderInstanceReconciler:  providerInstanceReconciler,
		CapabilityService:           capabilityService,

		BuiltinCatalog:         builtinCatalog,
		BuiltinBootstrapper:    builtinBootstrapper,
		BuiltinHandlerRegistry: builtinHandlerRegistry,

		ProviderInvocationService: providerInvocationService,
		ProviderInvoker:           kernelProviderInvoker,

		AcquisitionService: acquisitionService,

		BackgroundRemovalRegistry: bgRegistry,

		ChannelResolver: capabilityChannelResolver,

		EventBridgePublisher: eventBridgePublisher,
	}

	builtin.SetUIAgentPublisher(newUIAgentPublisher(b.extRoot, schemaRegistry, uiContribRepo, uiHost))
	builtin.SetUIAgentClientRuntime(uiHostNotifier)
	builtContainer = container

	if b.iosNativeProvider != nil {
		container.WireIOSPlatformAdapter(b.iosNativeProvider)
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

	// GameHost is a device execution plane. Cloud Core keeps the extension
	// metadata and package state, but must never compose or start a local game
	// plugin runtime. Enforce the runtime-profile policy at the composition
	// boundary so downstream services cannot accidentally expose a half-wired
	// GameHost on cloud deployments.
	var gameHost *gamehost.GameHostContainer
	var productionArchiveUpdater *ProductionArchiveUpdater
	if b.runtimePolicy.DevicePluginRuntime {
		kernelSource := gamehost.NewKernelContributionSource(instRepo, defRepo, contribRepo)

		var archiveUpdater upgrade.KernelArchiveUpdater
		if b.gameHostArchiveUpdater != nil {
			archiveUpdater = upgrade.NewKernelArchiveUpdaterAdapterWithArchivePath(b.gameHostArchiveUpdater.GetPreviousArchivePath, b.gameHostArchiveUpdater.UpdateArchive)
		} else {
			productionArchiveUpdater = NewProductionArchiveUpdater()
			archiveUpdater = upgrade.NewKernelArchiveUpdaterAdapterWithArchivePath(productionArchiveUpdater.GetPreviousArchivePath, productionArchiveUpdater.UpdateArchive)
		}

		gameHost, err = gamehost.ComposeGameHost(gamehost.GameHostComposeOptions{
			DataRoot:           b.extRoot,
			KernelSource:       kernelSource,
			TrustedSupervisor:  trustedSupervisor,
			NodeResolver:       nodeResolver,
			GenerationResolver: newGameHostInstalledGenerationResolver(packageGenerationStore),
			EventService:       eventSvc,
			HostAPIGateway:     hostAPIGateway,
			SecretBroker:       kernelSecretBroker,
			PermissionBroker:   permBroker,
			ArchiveUpdater:     archiveUpdater,
			StrictProduction:   true,

			KernelPermissionBroker:        permBroker,
			KernelPermissionSnapshotStore: permSnapshotStore,
			KernelScopeManager:            scopeManager,

			DefinitionReconcile: newGameHostDefinitionReconcile(instRepo, defRepo, contribRepo, typedInstaller),
			KernelLifecycle:     newGameHostKernelLifecycle(lifecycleMgr),
		})
		if err != nil {
			return nil, fmt.Errorf("kernel: compose gamehost: %w", err)
		}
		container.GameHost = gameHost
		if gameHost.AgentBridge != nil {
			if err := adapterRegistry.RegisterAdapter(capability.RuntimeTypeGameHost, gameHost.AgentBridge); err != nil {
				return nil, fmt.Errorf("kernel: register game host runtime adapter: %w", err)
			}
		}
	}

	if productionArchiveUpdater != nil {
		productionArchiveUpdater.SetContainer(container)
	}

	permBroker.OnPermissionRevoked = func(extensionID, runtimeID string) {
		if gameHost != nil && gameHost.SecretSubscriptions != nil {
			gameHost.SecretSubscriptions.OnPermissionRevoked(extensionID, runtimeID)
		}
	}

	return container, nil
}

type gameHostDefinitionReconcile struct {
	instRepo    domain.InstallationRepository
	defRepo     domain.DefinitionRepository
	contribRepo sqlite.ContributionRepository
	installer   *TypedContributionInstaller
}

func newGameHostDefinitionReconcile(instRepo domain.InstallationRepository, defRepo domain.DefinitionRepository, contribRepo sqlite.ContributionRepository, installer *TypedContributionInstaller) upgrade.DefinitionReconciler {
	return &gameHostDefinitionReconcile{instRepo: instRepo, defRepo: defRepo, contribRepo: contribRepo, installer: installer}
}

func (r *gameHostDefinitionReconcile) ReconcileExtension(extensionID string) *service_definition.ReconcileReport {
	report := &service_definition.ReconcileReport{ExtensionID: extensionID, DefinitionErrors: make(map[string]error)}
	ctx := context.Background()
	extID := domain.ExtensionID(extensionID)
	if r.instRepo == nil || r.defRepo == nil || r.contribRepo == nil || r.installer == nil {
		report.Errors = append(report.Errors, fmt.Errorf("gamehost definition reconcile: required repository or installer is unavailable"))
		return report
	}
	inst, err := r.instRepo.GetInstallation(ctx, extID)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Errorf("get installation: %w", err))
		return report
	}
	def, err := r.defRepo.GetExtension(ctx, extID, inst.InstalledVersion)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Errorf("get installed extension definition: %w", err))
		return report
	}
	desired := def.AllContributions()
	current, err := r.contribRepo.ListContributions(ctx, extID)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Errorf("list persisted contributions: %w", err))
		return report
	}

	currentByID := make(map[domain.ContributionID]domain.ContributionDefinition, len(current))
	for _, item := range current {
		currentByID[item.ID] = item
	}
	desiredByID := make(map[domain.ContributionID]domain.ContributionDefinition, len(desired))
	for _, item := range desired {
		item.ExtensionID = extID
		desiredByID[item.ID] = item
		if before, ok := currentByID[item.ID]; !ok {
			report.Added++
		} else {
			beforeJSON, _ := json.Marshal(before)
			afterJSON, _ := json.Marshal(item)
			if string(beforeJSON) == string(afterJSON) {
				report.Skipped++
			} else {
				report.Updated++
			}
		}
	}
	for id := range currentByID {
		if _, ok := desiredByID[id]; !ok {
			report.Removed++
		}
	}
	if report.Added == 0 && report.Updated == 0 && report.Removed == 0 {
		return report
	}

	// Unregister the old runtime contributions first, then atomically rebuild the
	// persisted contribution view. If any stage fails, restore the previous view
	// and registrations so upgrade does not report a false successful reconcile.
	if err := r.installer.UninstallContributions(ctx, extID); err != nil {
		report.Errors = append(report.Errors, fmt.Errorf("uninstall previous contributions: %w", err))
		return report
	}
	restore := func(cause error) {
		if err := r.installer.UninstallContributions(ctx, extID); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("rollback uninstall partial contributions: %w", err))
		}
		_ = r.contribRepo.DeleteContributions(ctx, extID)
		for _, item := range current {
			_ = r.contribRepo.PutContribution(ctx, item)
		}
		if err := r.installer.InstallContributions(ctx, current, inst.Generation); err != nil {
			report.Errors = append(report.Errors, fmt.Errorf("rollback contribution registrations: %w", err))
		} else if inst.EnablementState == domain.EnablementEnabled {
			if err := r.installer.ActivateContributions(ctx, extID); err != nil {
				report.Errors = append(report.Errors, fmt.Errorf("rollback contribution activation: %w", err))
			}
		}
		report.Errors = append(report.Errors, cause)
	}
	if err := r.contribRepo.DeleteContributions(ctx, extID); err != nil {
		restore(fmt.Errorf("replace contribution records: %w", err))
		return report
	}
	for _, item := range desired {
		item.ExtensionID = extID
		if err := r.contribRepo.PutContribution(ctx, item); err != nil {
			restore(fmt.Errorf("persist contribution %s: %w", item.ID, err))
			return report
		}
	}
	if err := r.installer.InstallContributions(ctx, desired, inst.Generation); err != nil {
		restore(fmt.Errorf("install reconciled contributions: %w", err))
		return report
	}
	if inst.EnablementState == domain.EnablementEnabled {
		if err := r.installer.ActivateContributions(ctx, extID); err != nil {
			restore(fmt.Errorf("activate reconciled contributions: %w", err))
			return report
		}
	}
	return report
}

type gameHostKernelLifecycle struct {
	manager *lifecycle_manager.Manager
}

func newGameHostKernelLifecycle(manager *lifecycle_manager.Manager) upgrade.KernelExtensionLifecycle {
	return &gameHostKernelLifecycle{manager: manager}
}

func (k *gameHostKernelLifecycle) ExecuteUpdate(ctx context.Context, extensionID string, targetVersion string, operationID upgrade.UpgradeOperationID) (*upgrade.KernelUpdateResult, error) {
	if k.manager == nil {
		return nil, fmt.Errorf("kernel lifecycle manager not available")
	}
	version, err := domain.ParseVersion(strings.TrimSpace(targetVersion))
	if err != nil {
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, fmt.Errorf("parse target version %q: %w", targetVersion, err)
	}
	result, err := k.manager.Execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind: lifecycle_manager.CmdUpdate, ExtensionID: domain.ExtensionID(extensionID), TargetVersion: version, RequestID: string(operationID),
	})
	if err != nil {
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, err
	}
	if result.Status != "completed" {
		err := fmt.Errorf("kernel update did not complete: status=%s reason=%s", result.Status, result.Error)
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, err
	}
	if result.FinalState.Installation == nil {
		err := fmt.Errorf("kernel update completed without installation state")
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, err
	}
	newVersion := strings.TrimSpace(result.FinalState.Installation.InstalledVersion.String())
	if newVersion == "" || newVersion != version.String() {
		err := fmt.Errorf("kernel update version mismatch: expected %s, got %q", version.String(), newVersion)
		return &upgrade.KernelUpdateResult{Success: false, NewVersion: newVersion, Reason: err.Error()}, err
	}
	return &upgrade.KernelUpdateResult{Success: true, NewVersion: newVersion}, nil
}

func (b *ContainerBuilder) buildStore(ctx context.Context) (*sqlite.Store, error) {
	if b.db != nil {
		store := sqlite.NewStoreFromDB(b.db)
		if err := host_registry.MigrateSessionTokens(ctx, store.DB()); err != nil {
			return nil, fmt.Errorf("kernel: migrate session tokens: %w", err)
		}
		return store, nil
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
	if err := host_registry.MigrateSessionTokens(ctx, store.DB()); err != nil {
		store.Close()
		return nil, fmt.Errorf("kernel: migrate session tokens: %w", err)
	}
	return store, nil
}

func buildKernelSecretBroker(extRoot string) (*secret.Broker, error) {
	secretsPath := filepath.Join(extRoot, "kernel-secrets.json")
	keyPath := filepath.Join(extRoot, "kernel-secrets.key")
	store, err := secret.NewEncryptedFileStore(secretsPath, keyPath)
	if err != nil {
		return nil, fmt.Errorf("create encrypted file store: %w", err)
	}
	broker, err := secret.NewBroker(secret.BrokerConfig{Store: store})
	if err != nil {
		return nil, fmt.Errorf("create secret broker: %w", err)
	}
	return broker, nil
}

func validateExecutionWiring(kernel *execution.ExecutionPipeline, adapters *capability.RuntimeAdapterRegistry, tools *capability.ToolRegistry) error {
	if kernel == nil {
		return fmt.Errorf("execution pipeline is nil")
	}
	if kernel.InvocationValidator == nil || kernel.InputValidator == nil ||
		kernel.AvailabilityGate == nil ||
		kernel.ScopeGate == nil || kernel.PermissionGate == nil ||
		kernel.ApprovalGate == nil ||
		kernel.ConcurrencyCtrl == nil || kernel.RateLimiter == nil ||
		kernel.IdempotencyGuard == nil || kernel.RetryCtrl == nil ||
		kernel.TimeoutCtrl == nil || kernel.CancellationCtrl == nil ||
		kernel.DepthGuard == nil || kernel.Dispatcher == nil ||
		kernel.ResultValidator == nil || kernel.Sanitizer == nil ||
		kernel.CircuitBreaker == nil || kernel.ResourceQuotaCtrl == nil ||
		kernel.AuditSink == nil || kernel.ToolResolver == nil ||
		kernel.ScopeStore == nil || kernel.PermissionSnapshotStore == nil {
		return fmt.Errorf("one or more critical execution controllers is nil")
	}
	if adapters == nil {
		return fmt.Errorf("runtime adapter registry is nil")
	}
	if tools == nil {
		return fmt.Errorf("tool registry is nil")
	}
	return nil
}

func makeSearchCallFunc(cfg search.Config, broker *secret.Broker) capability.SearchCallFunc {
	svc := buildSearchService(cfg, broker)
	if svc == nil {
		return nil
	}
	return buildSearchCallFunc(svc)
}

func makeSearchHealthFunc(cfg search.Config, broker *secret.Broker) capability.SearchHealthFunc {
	svc := buildSearchService(cfg, broker)
	if svc == nil {
		return nil
	}
	return buildSearchHealthFunc(svc)
}

func registerDeepSearchSystemTask(ctx context.Context, svc *task_runtime.TaskRuntimeService, entry string) error {
	return RegisterDeepSearchSystemTask(ctx, svc, entry)
}

// kernelArtifactStoreAdapter adapts PackageArtifactStore to the acquisition RemoteArtifactStorer interface.
type kernelArtifactStoreAdapter struct {
	store *PackageArtifactStore
}

func (a *kernelArtifactStoreAdapter) PutArchiveFromURI(ctx context.Context, uri string, metadata acquisition.ArtifactStoreMetadata) (acquisition.StoredArtifact, error) {
	kernelMeta := ArtifactMetadata{
		ExtensionID:  metadata.ExtensionID,
		Version:      metadata.Version,
		SourceURI:    metadata.SourceURI,
		ExpectedHash: metadata.ExpectedHash,
	}
	result, err := a.store.PutArchiveFromURI(ctx, uri, kernelMeta)
	if err != nil {
		return acquisition.StoredArtifact{}, err
	}
	return acquisition.StoredArtifact{
		ArtifactID:   result.ArtifactID,
		ArchiveHash:  result.ArchiveHash,
		ArchivePath:  result.ArchivePath,
		ManifestHash: result.ManifestHash,
	}, nil
}

func (a *kernelArtifactStoreAdapter) HasArtifactAtHash(expectedHash string) (string, error) {
	return a.store.HasArtifactAtHash(expectedHash)
}

func (a *kernelArtifactStoreAdapter) ArtifactIDFromHash(hash string) string {
	return a.store.ArtifactIDFromHash(hash)
}

// kernelArtifactRegistryAdapter adapts PackageRepository to the acquisition RemoteArtifactRegistry interface.
type kernelArtifactRegistryAdapter struct {
	repo *PackageRepository
}

func (a *kernelArtifactRegistryAdapter) PutArtifact(ctx context.Context, artifact acquisition.ArtifactRecord) error {
	return a.repo.PutArtifact(ctx, PackageArtifact{
		ArtifactID:   artifact.ArtifactID,
		ExtensionID:  artifact.ExtensionID,
		Version:      artifact.Version,
		ArchiveHash:  artifact.ArchiveHash,
		ArchivePath:  artifact.ArchivePath,
		ManifestHash: artifact.ManifestHash,
	})
}

func (a *kernelArtifactRegistryAdapter) GetArtifactByArchivePath(ctx context.Context, archivePath string) (*acquisition.ArtifactRecord, error) {
	result, err := a.repo.GetArtifactByArchivePath(ctx, archivePath)
	if err != nil {
		return nil, err
	}
	return &acquisition.ArtifactRecord{
		ArtifactID:   result.ArtifactID,
		ExtensionID:  result.ExtensionID,
		Version:      result.Version,
		ArchiveHash:  result.ArchiveHash,
		ArchivePath:  result.ArchivePath,
		ManifestHash: result.ManifestHash,
	}, nil
}

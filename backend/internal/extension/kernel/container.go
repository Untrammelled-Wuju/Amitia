package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/delivery"
	"github.com/u-ai/backend/internal/desktoppet/plugin_boundary"
	"github.com/u-ai/backend/internal/deviceruntime"
	coreexec "github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/builtin"
	"github.com/u-ai/backend/internal/extension/kernel/canary"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/capability/acquisition"
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
	"github.com/u-ai/backend/internal/extension/kernel/eventbridge"
	"github.com/u-ai/backend/internal/extension/kernel/execution"
	"github.com/u-ai/backend/internal/extension/kernel/extension_page_host"
	"github.com/u-ai/backend/internal/extension/kernel/extension_slots"
	"github.com/u-ai/backend/internal/extension/kernel/hook"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/javascript_main"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
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
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_ordering"
	"github.com/u-ai/backend/internal/extension/kernel/update"
	"github.com/u-ai/backend/internal/extension/kernel/wasm_runtime"
	"github.com/u-ai/backend/internal/extension/kernel/workflow"
	"github.com/u-ai/backend/internal/gamehost"
	"github.com/u-ai/backend/internal/imageprovider/backgroundremoval"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

type MCPDuplicateDetail struct {
	ToolID     string `json:"toolId"`
	ServerID   string `json:"serverId"`
	Owner      string `json:"owner"`
	Generation int64  `json:"generation"`
	DetectedAt string `json:"detectedAt"`
}

type MCPDuplicateMetricProvider interface {
	CountUnresolved(ctx context.Context) (int64, error)
	ListUnresolved(ctx context.Context) ([]MCPDuplicateDetail, error)
}

type Container struct {
	Store                  *sqlite.Store
	TransactionManager     *sqlite.TransactionManager
	DefinitionRepository   domain.DefinitionRepository
	InstallationRepository domain.InstallationRepository
	ModuleRepository       sqlite.ModuleRepository
	ContributionRepository sqlite.ContributionRepository
	EnablementStore        enablement.StateStore
	RuntimeRepository      domain.RuntimeRepository
	OperationRepository    sqlite.OperationRepository
	ScopeRepository        sqlite.ScopeRepository
	PermissionRepository   sqlite.PermissionRepository
	ResourceRepository     sqlite.ResourceRepository

	PackageSecurity        *package_security.PackageSecurityService
	PackageRepository      *PackageRepository
	PackageArtifactStore   *PackageArtifactStore
	PackageGenerationStore *PackageGenerationStore
	ArtifactMaintenance    *PackageArtifactMaintenance
	PackageTrustRepository *PackageTrustRepository
	UserDataSnapshotStore  *UserDataSnapshotStore
	ResourceSnapshotStore  *ResourceSnapshotStore
	PackageSnapshotRepo    *PackageSnapshotRepository
	LifecycleManager       *lifecycle_manager.Manager
	ContributionRegistry   *contribution.ContributionRegistry
	ContributionInstaller  *TypedContributionInstaller
	DependencyResolver     dependency.Resolver
	RuntimeSupervisor      runtime_supervisor.Supervisor
	ExecutionKernel        *execution.ExecutionPipeline
	HostAPIGateway         *host_api.DefaultGateway
	PermissionBroker       permission.PermissionBroker
	PermissionDefinitions  *permission.PermissionDefinitionRegistry
	ScopeManager           scope.ScopeManager
	ScopeSnapshotCreator   func(extensionID, moduleID string, generation int64, characterID, conversationID string) (string, error)
	AgentSkillCatalog      *agent_skill.AgentSkillCatalog
	WorkflowRegistry       *workflow.WorkflowRegistry
	WorkflowExecutor       *workflow.WorkflowExecutor
	WorkflowTriggerManager *workflow.TriggerManager
	WorkflowDefRepo        *sqlite.WorkflowDefinitionRepository
	WorkflowExecRepo       *sqlite.WorkflowExecutionRepository
	EnablementService      *enablement.EnablementService
	EnablementResolver     enablement.EffectiveStateResolver

	ToolRegistry    *capability.ToolRegistry
	AdapterRegistry *capability.RuntimeAdapterRegistry
	ToolFacade      *ToolFacade

	ExecutionService *coreexec.ExecutionService
	RuntimeState     *RuntimeState

	HostCommandRegistry *HostCommandRegistry

	TaskRepository     *sqlite.TaskRepository
	TaskRuntimeService *task_runtime.TaskRuntimeService
	TaskHandler        *task_runtime.TaskRuntimeHandler

	ScheduleService    *schedule.ScheduleService
	ScheduleRepository *sqlite.ScheduleRepository

	WASMRuntimeFactory *wasm_runtime.WASMRuntimeFactory
	WASMModuleManager  *wasm_runtime.ModuleManager
	WASMHostGateway    *wasm_runtime.HostGateway
	WASMDefinitionRepo *sqlite.WASMDefinitionRepository

	TrustedServiceSupervisor *trusted_service.ProcessSupervisor
	TrustedServiceFactory    *trusted_service.TrustedServiceFactory
	HookService              *hook.Service
	EventService             *event.Service
	EventRuntimeBridge       *event.RuntimeBridge
	HostEventEmitter         event.HostEventEmitter
	LifecycleEventEmitter    *event.LifecycleEventEmitter
	JSRuntimeFactory         *javascript_main.RuntimeFactory
	EventDeliveryAdapter     *javascript_main.EventDeliveryAdapter

	UIHost                *ui_contribution.UIHost
	UIHostNotifier        *SSEUIHostNotifier
	ClipboardHostBridge   *BridgeClipboardHost
	UIContributionRepo    *sqlite.SQLiteUIContributionRepository
	SlotRegistry          *extension_slots.SlotRegistry
	PageHost              *extension_page_host.PageHost
	SchemaValidator       *schema_ui.Validator
	SchemaCompilerCache   *schema_ui.CompilerCache
	SchemaRegistry        *schema_ui.SchemaRegistry
	SandboxHost           *sandbox_webui.Host
	ChatExtensionRegistry *chat_ui_extension.ChatExtensionRegistry
	OrderingEngine        *ui_ordering.OrderingEngine
	ExtRoot               string

	DesktopHost              *desktop.DesktopHost
	UpdateManager            *desktop_update.UpdateManager
	UpdateAdapter            *UpdateManagerAdapter
	DesktopAPI               *desktop.DesktopAPI
	UpdateAPI                *desktop.UpdateAPI
	DesktopActionBridge      *DesktopActionBridge
	DesktopPetPluginBoundary *plugin_boundary.DesktopPetPluginBoundary

	ObservabilityStore observability.StorageBackend

	DevConsoleService *developer_console.ConsoleService
	DevConsoleRepo    *developer_console.DiagnosticRepository
	DevConsoleHandler *developer_console.HTTPHandler
	TrustService      *trust.TrustService
	AmitiaxInstaller  *amitiax.Installer
	DevModeRegistry   *dev_mode.WorkspaceRegistry
	DevModePipeline   *dev_mode.RebuildPipeline
	DevModeReloader   *dev_mode.RuntimeReloader
	DevModeSessions   *dev_mode.SessionManager
	DevModeStore      *dev_mode.SQLiteWorkspaceStore

	UpdateRecoveryManager *update.RecoveryManager

	MigrationRepository    *migration.MigrationRepository
	MigrationExecutor      *migration.MigrationExecutor
	MigrationGraphResolver *migration.MigrationGraphResolver
	MigrationPlanner       *migration.MigrationPlanner
	MigrationSnapshotMgr   *migration.SnapshotManager
	MigrationCheckpointMgr *migration.CheckpointManager
	MigrationValidator     *migration.PreconditionValidator

	RollbackRepository  *update.RollbackRepository
	JournalManager      *update.JournalManager
	RollbackExecutorV2  *update.RollbackExecutorV2
	RollbackPlanner     *update.RollbackPlanner
	RollbackPointStore  *update.RollbackPointStore
	UpdateGenerationMgr *update.GenerationManager
	UpdateMigrationExec *update.MigrationExecutor

	CanaryRepository        *canary.CanaryRepository
	CanaryStageManager      *canary.StageManager
	CanaryGenerationRouter  *canary.GenerationRouter
	CanaryHealthCollector   *canary.HealthMetricsCollector
	CanaryHealthEvaluator   *canary.HealthEvaluator
	CanaryCohortResolver    *canary.CohortResolver
	CanaryDualWriteManager  *canary.DualWriteManager
	CanaryShadowManager     *canary.ShadowManager
	CanaryOwnershipResolver *canary.BackgroundOwnershipResolver

	CandidateMgr *CandidateContributionManager
	CandidateNS  *CandidateNamespace

	MCPDuplicateProvider MCPDuplicateMetricProvider

	PageSessionRepository *sqlite.SQLitePageSessionRepository
	CandidateRepository   *CandidateRepository

	GameHost *gamehost.GameHostContainer

	RuntimeProfile              runtimeprofile.Profile
	RuntimePolicy               runtimeprofile.Policy
	DeviceRegistry              *host_registry.Registry
	DeviceRuntimePresence       *host_registry.DeviceRuntimePresenceAdapter
	DeviceRuntimeSessions       *deviceruntime.Service
	CapabilityProviders         *capability.ProviderRegistry
	ProviderLifecycle           *capability.ProviderLifecycleService
	ProviderExecutionResolver   *capability.ProviderRuntimeExecutionResolver
	ExtensionProviderReconciler *capability.ExtensionProviderReconciler
	ProviderInstanceReconciler  capability.ProviderInstanceReconciler
	CapabilityService           *capability.CapabilityService

	BuiltinCatalog         *builtin.Catalog
	BuiltinBootstrapper    *builtin.Bootstrapper
	BuiltinHandlerRegistry *builtin.HandlerRegistry

	ProviderInvocationService *capability.ProviderInvocationService
	ProviderInvoker           *KernelProviderInvoker

	EventBridgePublisher *eventbridge.Publisher

	NativeBridgeRelay interface{}

	AcquisitionService *acquisition.AcquisitionService

	BackgroundRemovalRegistry backgroundremoval.Registry
	ChannelResolver           delivery.ChannelResolver
}

func (c *Container) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	if c.ArtifactMaintenance != nil {
		c.ArtifactMaintenance.Stop()
	}

	if c.ScheduleService != nil {
		c.ScheduleService.Shutdown(context.Background())
	}

	if c.EventService != nil {
		c.EventService.Stop()
	}

	if c.HookService != nil {
		if err := c.HookService.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if c.PermissionBroker != nil {
		if broker, ok := c.PermissionBroker.(*permission.DefaultPermissionBroker); ok {
			if err := broker.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	if c.Store != nil {
		if err := c.Store.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.GameHost != nil {
		if err := c.GameHost.Shutdown(context.Background()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Container) Recover(ctx context.Context) error {
	if c.CandidateMgr != nil {
		if _, err := c.CandidateMgr.RecoverOrphanCandidates(ctx); err != nil {
			fmt.Printf("kernel: recover orphan candidates warning: %v\n", err)
		}
	}

	if c.DevModeReloader != nil {
		if cleaned := c.DevModeReloader.RecoverStaleInstances(ctx); cleaned > 0 {
			fmt.Printf("kernel: recovered %d stale runtime instances from cleanup failures\n", cleaned)
		}
	}

	insts, err := c.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		return fmt.Errorf("kernel: recover installations: %w", err)
	}
	var recoverErrs []error
	for _, inst := range insts {
		if inst.InstallationState != domain.InstallationStateInstalled {
			continue
		}
		contribs, err := c.ContributionRepository.ListContributions(ctx, inst.ExtensionID)
		if err != nil {
			recoverErrs = append(recoverErrs, fmt.Errorf("kernel: list contributions for %s: %w", inst.ExtensionID, err))
			c.markRequiresRecovery(ctx, inst)
			continue
		}
		for _, cd := range contribs {
			bc := &contribution.BaseContribution{
				ID:        string(cd.ID),
				Type:      contribution.ContributionType(cd.Kind),
				Extension: string(cd.ExtensionID),
				Module:    string(cd.ModuleID),
				Enabled:   true,
			}
			_ = c.ContributionRegistry.Register(bc)
		}
		extSubject := enablement.StateSubject{
			Kind: enablement.SubjectExtension,
			ID:   string(inst.ExtensionID),
		}
		if _, err := c.EnablementStore.Get(ctx, extSubject); err != nil {
			if inst.EnablementState == domain.EnablementEnabled {
				_ = c.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementEnabled)
			} else {
				_ = c.EnablementStore.SetEnablement(ctx, extSubject, enablement.EnablementDisabled)
			}
		}
		if inst.EnablementState == domain.EnablementEnabled {
			modules, err := c.ModuleRepository.ListModules(ctx, inst.ExtensionID)
			if err != nil {
				recoverErrs = append(recoverErrs, fmt.Errorf("kernel: list modules for %s: %w", inst.ExtensionID, err))
				c.markRequiresRecovery(ctx, inst)
				continue
			}
			for _, mod := range modules {
				modSubject := enablement.StateSubject{
					Kind:     enablement.SubjectModule,
					ID:       string(mod.ID),
					ParentID: string(inst.ExtensionID),
				}
				_ = c.EnablementStore.SetEnablement(ctx, modSubject, enablement.EnablementEnabled)
			}

			runtimeFailed := false
			if c.RuntimeSupervisor != nil {
				for _, mod := range modules {
					if mod.Runtime != nil && mod.Runtime.Type != "" && mod.Runtime.Type != domain.RuntimeTypeBuiltin {
						defID := runtime_supervisor.BuildRuntimeDefinitionID(string(inst.ExtensionID), string(mod.ID), mod.Runtime.Type)
						spec := runtime_supervisor.InstanceSpec{
							DefinitionID: defID,
							ExtensionID:  inst.ExtensionID,
							ModuleID:     mod.ID,
							RuntimeType:  mod.Runtime.Type,
							Generation:   inst.Generation,
						}
						result := c.RuntimeSupervisor.Reconcile(ctx, runtime_supervisor.ReconcileRequest{
							DefinitionID: defID,
							Desired:      runtime_supervisor.DesiredRunning,
							Spec:         spec,
						})
						if result.Error != nil || result.Actual != runtime_supervisor.ActualReady {
							c.recordReconcileFailure(ctx, inst.ExtensionID, result)
							recoverErrs = append(recoverErrs, fmt.Errorf("kernel: runtime reconcile failed for %s module %s: actual=%s", inst.ExtensionID, mod.ID, result.Actual))
							runtimeFailed = true
						}
					}
				}
			}

			if runtimeFailed {
				c.markRequiresRecovery(ctx, inst)
				continue
			}

			if c.ContributionInstaller != nil {
				if err := c.ContributionInstaller.RecoverContributions(ctx, inst.ExtensionID); err != nil {
					recoverErrs = append(recoverErrs, fmt.Errorf("kernel: recover typed contributions for %s: %w", inst.ExtensionID, err))
					c.markRequiresRecovery(ctx, inst)
				}
			}
		}
	}
	if c.WorkflowExecutor != nil {
		if err := c.WorkflowExecutor.Recover(ctx, 100); err != nil {
			return fmt.Errorf("kernel: recover workflow executions: %w", err)
		}
	}
	if len(recoverErrs) > 0 {
		return fmt.Errorf("kernel: recover encountered %d error(s): %v", len(recoverErrs), recoverErrs)
	}
	c.assertNoDuplicateOwnerSource(ctx)
	return nil
}

func (c *Container) assertNoDuplicateOwnerSource(ctx context.Context) {
	if c.MCPDuplicateProvider != nil {
		if count, err := c.MCPDuplicateProvider.CountUnresolved(ctx); err == nil && count > 0 {
			if details, listErr := c.MCPDuplicateProvider.ListUnresolved(ctx); listErr == nil {
				for _, d := range details {
					fmt.Printf("[kernel-startup-assert] duplicate_mcp_tool_registration: toolID=%s serverID=%s owner=%s generation=%d detectedAt=%s\n", d.ToolID, d.ServerID, d.Owner, d.Generation, d.DetectedAt)
				}
			}
			fmt.Printf("[kernel-startup-assert] WARNING: %d unresolved duplicate MCP tool registration(s) detected\n", count)
		}
	}
	if GlobalLegacyCallCounter() != nil {
		if dupContrib := GlobalLegacyCallCounter().DuplicateContributionRegistrations(); dupContrib > 0 {
			fmt.Printf("[kernel-startup-assert] WARNING: %d duplicate contribution registration(s) detected\n", dupContrib)
		}
	}
}

func (c *Container) markRequiresRecovery(ctx context.Context, inst domain.ExtensionInstallation) {
	if c == nil || c.InstallationRepository == nil {
		return
	}
	inst.EnablementState = domain.EnablementRequiresRecovery
	inst.UpdatedAt = time.Now().UTC()
	_ = c.InstallationRepository.PutInstallation(ctx, inst)
}

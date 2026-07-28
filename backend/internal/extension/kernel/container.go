package kernel

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/agent_skill"
	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/canary"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
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

	PackageSecurity      *package_security.PackageSecurityService
	LifecycleManager     *lifecycle_manager.Manager
	ContributionRegistry *contribution.ContributionRegistry
	DependencyResolver   dependency.Resolver
	RuntimeSupervisor    runtime_supervisor.Supervisor
	ExecutionKernel      *execution.ExecutionPipeline
	HostAPIGateway       *host_api.DefaultGateway
	PermissionBroker     permission.PermissionBroker
	ScopeManager         scope.ScopeManager
	AgentSkillCatalog    *agent_skill.AgentSkillCatalog
	WorkflowRegistry     *workflow.WorkflowRegistry
	EnablementService    *enablement.EnablementService
	EnablementResolver   enablement.EffectiveStateResolver

	ToolRegistry      *capability.ToolRegistry
	AdapterRegistry   *capability.RuntimeAdapterRegistry
	ToolFacade        *ToolFacade

	TaskRepository    *sqlite.TaskRepository
	TaskRuntimeService *task_runtime.TaskRuntimeService
	TaskHandler       *task_runtime.TaskHandler

	ScheduleService     *schedule.ScheduleService
	ScheduleRepository  *sqlite.ScheduleRepository

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

	UIHost             *ui_contribution.UIHost
	UIContributionRepo *sqlite.SQLiteUIContributionRepository
	SlotRegistry       *extension_slots.SlotRegistry
	PageHost           *extension_page_host.PageHost
	SchemaValidator    *schema_ui.Validator
	SchemaCompilerCache *schema_ui.CompilerCache
	SandboxHost        *sandbox_webui.Host
	ChatExtensionRegistry *chat_ui_extension.ChatExtensionRegistry
	OrderingEngine     *ui_ordering.OrderingEngine
	ExtRoot            string

	DesktopHost     *desktop.DesktopHost
	UpdateManager   *desktop_update.UpdateManager
	UpdateAdapter   *UpdateManagerAdapter
	DesktopAPI      *desktop.DesktopAPI
	UpdateAPI       *desktop.UpdateAPI
	DesktopActionBridge *DesktopActionBridge

	DevConsoleService   *developer_console.ConsoleService
	DevConsoleRepo      *developer_console.DiagnosticRepository
	DevConsoleHandler   *developer_console.HTTPHandler
	TrustService        *trust.TrustService
	AmitiaxInstaller    *amitiax.Installer
	DevModeRegistry     *dev_mode.WorkspaceRegistry
	DevModePipeline     *dev_mode.RebuildPipeline
	DevModeReloader     *dev_mode.RuntimeReloader
	DevModeSessions     *dev_mode.SessionManager
	DevModeStore        *dev_mode.SQLiteWorkspaceStore

	UpdateRecoveryManager *update.RecoveryManager

	MigrationRepository    *migration.MigrationRepository
	MigrationExecutor      *migration.MigrationExecutor
	MigrationGraphResolver *migration.MigrationGraphResolver
	MigrationPlanner       *migration.MigrationPlanner
	MigrationSnapshotMgr   *migration.SnapshotManager
	MigrationCheckpointMgr *migration.CheckpointManager
	MigrationValidator     *migration.PreconditionValidator

	RollbackRepository   *update.RollbackRepository
	JournalManager       *update.JournalManager
	RollbackExecutorV2   *update.RollbackExecutorV2
	RollbackPlanner      *update.RollbackPlanner
	RollbackPointStore   *update.RollbackPointStore
	UpdateGenerationMgr  *update.GenerationManager
	UpdateMigrationExec  *update.MigrationExecutor

	CanaryRepository         *canary.CanaryRepository
	CanaryStageManager       *canary.StageManager
	CanaryGenerationRouter   *canary.GenerationRouter
	CanaryHealthCollector    *canary.HealthMetricsCollector
	CanaryHealthEvaluator    *canary.HealthEvaluator
	CanaryCohortResolver     *canary.CohortResolver
	CanaryDualWriteManager   *canary.DualWriteManager
	CanaryShadowManager      *canary.ShadowManager
	CanaryOwnershipResolver  *canary.BackgroundOwnershipResolver
}

func (c *Container) Close() error {
	if c == nil {
		return nil
	}
	var firstErr error
	if c.Store != nil {
		if err := c.Store.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Container) Recover(ctx context.Context) error {
	insts, err := c.InstallationRepository.ListInstallations(ctx)
	if err != nil {
		return fmt.Errorf("kernel: recover installations: %w", err)
	}
	for _, inst := range insts {
		if inst.InstallationState != domain.InstallationStateInstalled {
			continue
		}
		contribs, err := c.ContributionRepository.ListContributions(ctx, inst.ExtensionID)
		if err != nil {
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
		}
	}
	return nil
}

package gamehost

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/config"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/hostapi"
	"github.com/u-ai/backend/internal/gamehost/integration"
	"github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/notification"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/resource"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/runtime/checkpoint"
	"github.com/u-ai/backend/internal/gamehost/state"
	"github.com/u-ai/backend/internal/gamehost/startup"
	"github.com/u-ai/backend/internal/gamehost/storage"
	"github.com/u-ai/backend/internal/gamehost/stream"
	"github.com/u-ai/backend/internal/gamehost/stream/binary"
	"github.com/u-ai/backend/internal/gamehost/recovery"
	"github.com/u-ai/backend/internal/gamehost/upgrade"
	ghTrustedService "github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type GameHostComposeOptions struct {
	DataRoot          string
	KernelSource      integration.KernelContributionSource
	TrustedSupervisor *ghTrustedService.ProcessSupervisor
	EventService      *event.Service
	HostAPIGateway    host_api.Gateway
	ArchiveUpdater    upgrade.KernelArchiveUpdater
}

func NewKernelContributionSource(
	instRepo installationLister,
	defRepo definitionLister,
	contribRepo contributionLister,
) integration.KernelContributionSource {
	return newKernelContributionSource(instRepo, defRepo, contribRepo)
}

type kernelArchiveUpdaterFunc func(ctx context.Context, extensionID string, archivePath string) (*upgrade.KernelUpdateResult, error)

type kernelArchiveUpdaterAdapter struct {
	fn kernelArchiveUpdaterFunc
}

func (a *kernelArchiveUpdaterAdapter) UpdateArchive(ctx context.Context, extensionID string, archivePath string) (*upgrade.KernelUpdateResult, error) {
	if a.fn != nil {
		return a.fn(ctx, extensionID, archivePath)
	}
	return nil, fmt.Errorf("archive updater not wired")
}

func ComposeUpgradeCoordinator(
	pluginReg *registry.Registry,
	contributionSync *integration.GamePluginSyncService,
	configResolver *config.Resolver,
	runtimeExecutor runtime.RuntimeExecutor,
	archiveUpdater upgrade.KernelArchiveUpdater,
) *upgrade.UpgradeCoordinator {
	return upgrade.BuildUpgradeCoordinator(upgrade.UpgradeCoordinatorDeps{
		PluginRegistry:   pluginReg,
		RuntimeExecutor:  runtimeExecutor,
		ContributionSync: contributionSync,
		ConfigResolver:   configResolver,
		ArchiveUpdater:   archiveUpdater,
	})
}

func ComposeStartupRecovery(gate *startup.StartupGate) *startup.StartupRecoveryCoordinator {
	return startup.NewStartupRecoveryCoordinator(startup.StartupRecoveryDeps{
		Gate: gate,
	})
}

func ComposeRecoveryCoordinator(checkpointStore *checkpoint.FileStore, auditFn func(recovery.RecoveryAuditEvent)) *recovery.RecoveryCoordinator {
	var storeReader recovery.CheckpointStoreReader
	if checkpointStore != nil {
		storeReader = recovery.NewCheckpointStoreAdapter(
			checkpointStore.HasMetadata,
			func(ctx context.Context, runtimeID domain.RuntimeInstanceID) (recovery.RuntimeMetadataView, error) {
				m, err := checkpointStore.LoadMetadata(ctx, runtimeID)
				if err != nil {
					return recovery.RuntimeMetadataView{}, err
				}
				return recovery.RuntimeMetadataView{
					RuntimeID:          m.RuntimeID,
					PluginID:           m.PluginID,
					ExtensionID:        m.ExtensionID,
					DescriptorRevision: m.DescriptorRevision,
				}, nil
			},
			func(ctx context.Context, runtimeID domain.RuntimeInstanceID) (recovery.RuntimeCheckpointView, error) {
				c, err := checkpointStore.LoadCheckpoint(ctx, runtimeID)
				if err != nil {
					return recovery.RuntimeCheckpointView{}, err
				}
				return recovery.RuntimeCheckpointView{
					RuntimeID:          c.RuntimeID,
					PluginID:           c.PluginID,
					RuntimeState:       c.RuntimeState,
					CleanShutdown:      c.CleanShutdown,
					DescriptorRevision: c.DescriptorRevision,
				}, nil
			},
		)
	}
	checkpointClassifier := recovery.NewDefaultCheckpointClassifier(storeReader)
	var auditSink recovery.AuditSink
	if auditFn != nil {
		auditSink = recovery.NewAuditSinkAdapter(auditFn)
	}
	return recovery.NewRecoveryCoordinator(recovery.RecoveryCoordinatorDeps{
		CheckpointClassifier: checkpointClassifier,
		AuditSink:           auditSink,
	})
}

func ComposeGameHost(opts GameHostComposeOptions) (*GameHostContainer, error) {
	dirMgr, err := storage.NewDirectoryManager(opts.DataRoot)
	if err != nil {
		return nil, err
	}

	checkpointStore, err := checkpoint.NewFileStore(dirMgr)
	if err != nil {
		return nil, err
	}

	configStore := config.NewFileStore(dirMgr)
	configResolver := config.NewResolver(configStore, nil, nil)

	pluginReg := registry.NewRegistry()

	policyResolver := stream.NewPolicyResolver()
	streamMgr := stream.NewStreamManager(policyResolver)

	stateStore := state.NewLatestStateStore(state.NewOptions())

	binaryReg := binary.NewObjectRegistry(binary.Options{})

	channelReg := channel.NewRegistry(channel.Options{})

	nsReg := rpc.NewNamespaceRegistry(rpc.NamespaceRegistryConfig{})

	notifBridge := notification.NewBridge(notification.NewCompositeSink())

	connReg := ipc.NewConnectionRegistry()

	readyGate := handshake.NewReadyGate([]string{"control.handshake.hello", "control.request.cancel"})
	startupGate := startup.NewStartupGate()

	nsAdapter := handshake.NewNamespaceAdapter(nsReg)
	channelAdvertiser := handshake.NewChannelAdvertiser(pluginReg)
	handshakeMgr := handshake.NewHandshakeManager(handshake.HandshakeManagerConfig{
		NamespaceAdapter:  nsAdapter,
		ChannelAdvertiser: channelAdvertiser,
	})

	contributionSync, err := integration.NewGamePluginSyncService(
		pluginReg,
		integration.NewDefaultGamePluginContributionMapper(),
		opts.KernelSource,
	)
	if err != nil {
		return nil, err
	}

	var procAdapter runtime.ProcessSupervisorAdapter
	if opts.TrustedSupervisor != nil {
		adapt, err := runtime.NewProcessSupervisorAdapter(opts.TrustedSupervisor)
		if err != nil {
			return nil, err
		}
		procAdapter = adapt
	}

	runtimeManager := runtime.NewManager(runtime.ManagerOptions{})
	topologyStore := runtime.NewTopologyStore()

	resourceMapper := newResourceSubjectMapper(pluginReg)
	resourceGovernor := newRuntimeLimitGovernor()
	resourceAdapter := resource.NewResourceAdmissionAdapter(resourceMapper, nil, binaryReg, resourceGovernor)
	resourceViewer := resource.NewResourcePolicyViewer(newContainerViewResolver(binaryReg, streamMgr))
	resourceLifecycle := resource.NewLifecycleCoordinator(resourceAdapter, resourceViewer)

	authorityAuditSink := control.NewInMemoryAuthorityAuditSink()
	controlManager := control.NewControlAuthorityManager(control.ControlAuthorityManagerOptions{
		Audit: authorityAuditSink,
	})
	pluginOutputGate := control.NewPluginOutputGate(control.PluginOutputGateOptions{
		Authority: controlManager,
	})
	takeoverService := control.NewTakeoverService(control.TakeoverServiceOptions{
		Manager: controlManager,
		Audit:   authorityAuditSink,
	})

	var runtimeProvisioner *integration.RuntimeGraphProvisioner
	var serviceExecutor runtime.ServiceExecutor
	var runtimeExecutor runtime.RuntimeExecutor
	var runtimeHealth runtime.HealthAdapter

	if opts.TrustedSupervisor != nil && procAdapter != nil {
		var err error
		runtimeProvisioner, err = integration.NewRuntimeGraphProvisioner(integration.RuntimeGraphProvisionerOptions{
			Source:           opts.KernelSource,
			Mapper:           integration.NewDefaultGamePluginContributionMapper(),
			PluginRegistry:   pluginReg,
			RuntimeManager:   runtimeManager,
			TopologyStore:    topologyStore,
			Supervisor:       opts.TrustedSupervisor,
			DefinitionMapper: service_definition.NewDefinitionMapper(),
		})
		if err != nil {
			return nil, fmt.Errorf("compose runtime graph provisioner: %w", err)
		}

		serviceExecutor, err = runtime.NewServiceExecutor(
			procAdapter,
			runtime.NewUnavailableExternalServiceAdapter(),
			topologyStore,
			topologyStore,
		)
		if err != nil {
			return nil, fmt.Errorf("compose service executor: %w", err)
		}

		lifecyclePlanner := runtime.NewLifecyclePlanner()
		runtimeExecutor, err = runtime.NewRuntimeExecutor(
			topologyStore,
			runtimeManager,
		serviceExecutor,
			lifecyclePlanner,
		)
		if err != nil {
			return nil, fmt.Errorf("compose runtime executor: %w", err)
		}

		runtimeExecutor.SetResolveDefinition(
			func(definitionID string) (*trusted_service.ServiceRuntimeDefinition, error) {
				return opts.TrustedSupervisor.GetDefinition(definitionID)
			},
		)

		runtimeHealth, err = runtime.NewHealthAdapter(
			topologyStore,
			runtimeManager,
			runtime.NewHealthAggregator(),
		)
		if err != nil {
			return nil, fmt.Errorf("compose health adapter: %w", err)
		}
	}

	container := &GameHostContainer{
		DirectoryManager:    dirMgr,
		CheckpointStore:     checkpointStore,
		ConfigStore:         configStore,
		ConfigResolver:      configResolver,
		PluginRegistry:      pluginReg,
		ContributionSync:    contributionSync,
		RuntimeManager:      runtimeManager,
		RuntimeTopologyStore: topologyStore,
		RuntimeHealth:       runtimeHealth,
		RuntimeExecutor:     runtimeExecutor,
		RuntimeProvisioner:  runtimeProvisioner,
		NamespaceRegistry:   nsReg,
		HandshakeManager:    handshakeMgr,
		ReadyGate:           readyGate,
		ConnectionRegistry:  connReg,
		ChannelRegistry:     channelReg,
		NotificationBridge:  notifBridge,
		StateStore:          stateStore,
		BinaryObjectRegistry: binaryReg,
		StreamManager:       streamMgr,
		procAdapter:         procAdapter,
		HostAPIGateway:      opts.HostAPIGateway,

		ResourceAdapter:    resourceAdapter,
		ResourceViewer:     resourceViewer,
		ResourceLifecycle:  resourceLifecycle,

		AuthorityManager: controlManager,
		OutputGate:       pluginOutputGate,
		TakeoverService:  takeoverService,
		AuthorityAudit:   authorityAuditSink,

		StartupGate:         startupGate,
		UpgradeCoordinator:  ComposeUpgradeCoordinator(pluginReg, contributionSync, configResolver, runtimeExecutor, opts.ArchiveUpdater),
		RecoveryCoordinator: ComposeRecoveryCoordinator(checkpointStore, nil),
		StartupRecovery:     ComposeStartupRecovery(startupGate),
	}

	if opts.HostAPIGateway != nil {
		adapter, err := hostapi.NewProductionHostAPIAdapter(hostapi.ProductionHostAPIAdapterDeps{
			Gateway:        opts.HostAPIGateway,
			PluginRegistry: pluginReg,
			ReadyGate:      readyGate,
		})
		if err != nil {
			return nil, err
		}
		container.HostAPIAdapter = adapter
	}

	return container, nil
}

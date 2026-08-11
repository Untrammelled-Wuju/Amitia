package gamehost

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/config"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/hostapi"
	"github.com/u-ai/backend/internal/gamehost/integration"
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
}

func NewKernelContributionSource(
	instRepo installationLister,
	defRepo definitionLister,
	contribRepo contributionLister,
) integration.KernelContributionSource {
	return newKernelContributionSource(instRepo, defRepo, contribRepo)
}

func ComposeUpgradeCoordinator(
	pluginReg *registry.Registry,
	contributionSync *integration.GamePluginSyncService,
	configResolver *config.Resolver,
	runtimeExecutor runtime.RuntimeExecutor,
) *upgrade.UpgradeCoordinator {
	return upgrade.BuildUpgradeCoordinator(upgrade.UpgradeCoordinatorDeps{
		PluginRegistry:   pluginReg,
		RuntimeExecutor:  runtimeExecutor,
		ContributionSync: contributionSync,
		ConfigResolver:   configResolver,
	})
}

func ComposeStartupRecovery() *startup.StartupRecoveryCoordinator {
	return startup.NewStartupRecoveryCoordinator(startup.StartupRecoveryDeps{})
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

	nsAdapter := handshake.NewNamespaceAdapter(nsReg)
	handshakeMgr := handshake.NewHandshakeManager(handshake.HandshakeManagerConfig{
		NamespaceAdapter:  nsAdapter,
		ChannelAdvertiser: handshake.NoopChannelAdvertiser{},
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

	container := &GameHostContainer{
		DirectoryManager:    dirMgr,
		CheckpointStore:     checkpointStore,
		ConfigStore:         configStore,
		ConfigResolver:      configResolver,
		PluginRegistry:      pluginReg,
		ContributionSync:    contributionSync,
		NamespaceRegistry:   nsReg,
		HandshakeManager:    handshakeMgr,
		ReadyGate:           readyGate,
		ConnectionRegistry:  connReg,
		ChannelRegistry:     channelReg,
		NotificationBridge:  notifBridge,
		StateStore:          stateStore,
		BinaryObjectRegistry: binaryReg,
		StreamManager:       streamMgr,
		RuntimeExecutor:     nil,
		procAdapter:         procAdapter,
		HostAPIGateway:      opts.HostAPIGateway,

		ResourceAdapter:    resourceAdapter,
		ResourceViewer:     resourceViewer,
		ResourceLifecycle:  resourceLifecycle,

		AuthorityManager: controlManager,
		OutputGate:       pluginOutputGate,
		TakeoverService:  takeoverService,
		AuthorityAudit:   authorityAuditSink,

		UpgradeCoordinator:  ComposeUpgradeCoordinator(pluginReg, contributionSync, configResolver, nil),
		RecoveryCoordinator: ComposeRecoveryCoordinator(checkpointStore, nil),
		StartupRecovery:     ComposeStartupRecovery(),
		StartupGate:         startup.NewStartupGate(),
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

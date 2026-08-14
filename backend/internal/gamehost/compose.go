package gamehost

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	gamehostsecret "github.com/u-ai/backend/internal/gamehost/secret"
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
	"github.com/u-ai/backend/internal/gamehost/permission"
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
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type GameHostComposeOptions struct {
	DataRoot            string
	KernelSource        integration.KernelContributionSource
	TrustedSupervisor   *ghTrustedService.ProcessSupervisor
	EventService        *event.Service
	HostAPIGateway      host_api.Gateway
	ArchiveUpdater      upgrade.KernelArchiveUpdater
	DefinitionReconcile upgrade.DefinitionReconciler
	KernelLifecycle     upgrade.KernelExtensionLifecycle
	SecretBroker        *secret.Broker

	EffectivePermission *permission.EffectivePermissionAdapter
	PermissionBroker  permission.Broker

	KernelPermissionBroker       kernelpermission.PermissionBroker
	KernelPermissionSnapshotStore kernelpermission.PermissionSnapshotStore
	KernelScopeManager           scope.ScopeManager
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
	runtimeManager runtime.RuntimeManager,
	runtimeExecutor runtime.RuntimeExecutor,
	contributionSync *integration.GamePluginSyncService,
	definitionReconcile upgrade.DefinitionReconciler,
	runtimeGraphReconcile upgrade.RuntimeGraphReconciler,
	configResolver *config.Resolver,
	kernelLifecycle upgrade.KernelExtensionLifecycle,
	archiveUpdater upgrade.KernelArchiveUpdater,
) (*upgrade.UpgradeCoordinator, error) {
	return upgrade.BuildUpgradeCoordinator(upgrade.UpgradeCoordinatorDeps{
		PluginRegistry:        pluginReg,
		RuntimeManager:        runtimeManager,
		RuntimeExecutor:       runtimeExecutor,
		DefinitionReconcile:   definitionReconcile,
		RuntimeGraphReconcile: runtimeGraphReconcile,
		ContributionSync:      contributionSync,
		ConfigResolver:        configResolver,
		KernelLifecycle:       kernelLifecycle,
		ArchiveUpdater:        archiveUpdater,
	})
}

func ComposeStartupRecovery(deps startup.StartupRecoveryDeps) (*startup.StartupRecoveryCoordinator, error) {
	return startup.NewStartupRecoveryCoordinator(deps)
}

func ComposeRecoveryCoordinator(deps recovery.RecoveryCoordinatorDeps) (*recovery.RecoveryCoordinator, error) {
	return recovery.NewRecoveryCoordinator(deps)
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

	var notifSink notification.NotificationSink
	if opts.EventService != nil {
		notifSink = integration.NewKernelEventNotificationSink(opts.EventService, pluginReg)
	} else {
		notifSink = notification.NewCompositeSink()
	}
	notifBridge := notification.NewBridge(notifSink)

	connReg := ipc.NewConnectionRegistry()

	readyGate := handshake.NewReadyGate([]string{"control.handshake.hello", "control.request.cancel"})
	startupGate := startup.NewStartupGate()

	nsAdapter := handshake.NewNamespaceAdapter(nsReg)
	channelAdvertiser := handshake.NewChannelAdvertiser(pluginReg)
	handshakeMgr := handshake.NewHandshakeManager(handshake.HandshakeManagerConfig{
		NamespaceAdapter:  nsAdapter,
		ChannelAdvertiser: channelAdvertiser,
	})

	hostHandlers := rpc.NewHostHandlerRegistry()
	rpcLifecycle := rpc.NewLifecycleManager(rpc.LifecycleManagerConfig{})
	rpcDispatcher := rpc.NewRPCDispatcher(rpc.DispatcherConfig{
		Namespaces:   nsReg,
		HostHandlers: hostHandlers,
		Lifecycle:    rpcLifecycle,
	})

	handshakeController := handshake.NewHandshakeControllerAdapter(handshakeMgr, readyGate)

	controlPlane, err := ipc.NewControlPlane(ipc.ControlPlaneConfig{
		Registry:            connReg,
		Dispatcher:          rpcDispatcher,
		HandshakeController: handshakeController,
	})
	if err != nil {
		return nil, fmt.Errorf("compose control plane: %w", err)
	}

	rpcDispatcher.SetControlPlane(controlPlane)
	rpcLifecycle.SetControlPlane(controlPlane)

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

	controlSinkRegistry := control.NewControlSinkRegistry()
	commitBarrier := control.NewControlCommitBarrier()

	topologyAdapter := integration.NewControlTopologyAdapter(topologyStore)
	runtimeAdapter := integration.NewControlRuntimeAdapter(runtimeManager)
	hostPolicyAdapter := integration.NewControlHostPolicyAdapter(runtimeManager)

	var auditSink control.AuthorityAuditSink
	if opts.EventService != nil {
		auditSink = integration.NewControlAuditAdapter(opts.EventService, pluginReg)
	} else {
		auditSink = control.NewInMemoryAuthorityAuditSink()
	}

	if opts.EffectivePermission == nil && opts.PermissionBroker != nil {
		subjectResolver := integration.NewGameHostPermissionSubjectResolver(runtimeManager, pluginReg)
		subjectMapper := permission.NewGameHostSubjectMapper(subjectResolver)
		opts.EffectivePermission = permission.NewEffectivePermissionAdapter(opts.PermissionBroker, nil, subjectMapper)
	}

	permChecker := integration.NewControlPermissionAdapter(opts.EffectivePermission)

	controlManager := control.NewControlAuthorityManager(control.ControlAuthorityManagerOptions{
		Audit: auditSink,
	})

	pluginOutputGate, err := control.NewPluginOutputGate(control.PluginOutputGateOptions{
		Topology:         topologyAdapter,
		RuntimeReader:    runtimeAdapter,
		GenerationReader: runtimeAdapter,
		PermChecker:      permChecker,
		PolicyChecker:    hostPolicyAdapter,
		Authority:        controlManager,
		Audit:            auditSink,
		Metrics:          nil,
		CommitBarrier:    commitBarrier,
	})
	if err != nil {
		return nil, fmt.Errorf("compose plugin output gate: %w", err)
	}

	takeoverService := control.NewTakeoverService(control.TakeoverServiceOptions{
		Manager: controlManager,
		Audit:   auditSink,
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

 emergencyIntentStore := integration.NewInMemoryEmergencyIntentStore()
	emergencyRuntimeAdapter := integration.NewEmergencyRuntimeAdapter(runtimeExecutor, runtimeManager)
	emergencyRPCAdapter := integration.NewEmergencyRPCAdapter(rpcLifecycle.Registry())
	emergencyConnectionAdapter := integration.NewEmergencyConnectionAdapter(connReg, controlPlane)
	emergencyReadyAdapter := integration.NewEmergencyReadyAdapter(readyGate, connReg)
	emergencyStreamAdapter := integration.NewEmergencyStreamAdapter(streamMgr)
	emergencyChannelAdapter := integration.NewEmergencyChannelAdapter(channelReg)
	emergencyBinaryAdapter := integration.NewEmergencyBinaryAdapter(binaryReg)
	emergencyPendingVerifier := integration.NewEmergencyPendingVerifier(rpcLifecycle.Registry())
	emergencyConnectionVerifier := integration.NewEmergencyConnectionVerifier(connReg)
	emergencyStreamVerifier := integration.NewEmergencyStreamVerifier(streamMgr)
	emergencyChannelVerifier := integration.NewEmergencyChannelVerifier(channelReg)
	emergencyBinaryVerifier := integration.NewEmergencyBinaryVerifier(binaryReg)

	emergencyStopService, err := control.NewEmergencyStopService(control.EmergencyStopServiceOptions{
		Authority:         controlManager,
		Gate:              pluginOutputGate,
		Intent:            emergencyIntentStore,
		RuntimeStopper:    emergencyRuntimeAdapter,
		PendingCanceller:  emergencyRPCAdapter,
		ConnectionCloser:  emergencyConnectionAdapter,
		HandshakeReset:    emergencyReadyAdapter,
		StreamStopper:     emergencyStreamAdapter,
		ChannelCleaner:    emergencyChannelAdapter,
		BinaryReleaser:    emergencyBinaryAdapter,
		PendingVerifier:   emergencyPendingVerifier,
		ConnectionVerifier: emergencyConnectionVerifier,
		StreamVerifier:    emergencyStreamVerifier,
		ChannelVerifier:   emergencyChannelVerifier,
		BinaryVerifier:    emergencyBinaryVerifier,
	})
	if err != nil {
		return nil, fmt.Errorf("compose emergency stop service: %w", err)
	}

	var secretAdapter *gamehostsecret.SecretLeaseAdapter
	var secretLifecycle *gamehostsecret.LifecycleOrchestrator
	var secretSubscriptions *gamehostsecret.SubscriptionAdapter

	if opts.SecretBroker != nil {
		secretIdentity, err := integration.NewSecretRuntimeIdentityReader(runtimeManager, topologyStore, pluginReg)
		if err != nil {
			return nil, fmt.Errorf("compose secret identity reader: %w", err)
		}

		var secretGate *integration.EffectiveSecretPermissionGate
		if opts.EffectivePermission != nil {
			secretGate, err = integration.NewEffectiveSecretPermissionGate(opts.EffectivePermission)
			if err != nil {
				return nil, fmt.Errorf("compose secret permission gate: %w", err)
			}
		} else {
			return nil, fmt.Errorf("compose secret: EffectivePermission is required when SecretBroker is provided")
		}

		secretAdapter, err = gamehostsecret.NewSecretLeaseAdapter(opts.SecretBroker, secretIdentity, secretGate)
		if err != nil {
			return nil, fmt.Errorf("compose secret lease adapter: %w", err)
		}

		secretLifecycle = gamehostsecret.NewLifecycleOrchestrator(secretAdapter)
		secretSubscriptions = gamehostsecret.NewSubscriptionAdapter(secretAdapter)
	}

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

	var kernelRollbackAdapter recovery.KernelRollback
	var supervisorViewAdapter recovery.SupervisorView
	var rtExecutor recovery.RuntimeExecutor
	var secretLeaseAdapter recovery.SecretLeaseService
	var structureBuilderAdapter recovery.HostStructureBuilder
	var permissionAdapter recovery.PermissionService
	var authorityAdapter recovery.ControlAuthorityView

	if opts.TrustedSupervisor != nil {
		supervisorViewAdapter = recovery.NewSupervisorViewAdapter(
			opts.TrustedSupervisor.QuarantineManager().IsQuarantined,
			func(serviceID string) int {
				inst, err := opts.TrustedSupervisor.Get(serviceID)
				if err != nil {
					return 0
				}
				return inst.RestartCount
			},
			func(serviceID string) int {
				def, err := opts.TrustedSupervisor.GetDefinition(serviceID)
				if err != nil {
					return 3
				}
				return def.Recovery.MaxRestarts
			},
		)
	}

	if opts.ArchiveUpdater != nil {
		kernelRollbackAdapter = recovery.NewKernelRollbackAdapter(
			func(ctx context.Context, extensionID string, operationID recovery.RecoveryOperationID) (recovery.KernelRollbackResult, error) {
				result, err := opts.ArchiveUpdater.UpdateArchive(ctx, extensionID, "")
				if err != nil {
					return recovery.KernelRollbackResult{Success: false, Error: err.Error()}, err
				}
				return recovery.KernelRollbackResult{
					Success:           result.Success,
					NewVersion:        result.NewVersion,
					RequiresReconcile: true,
				}, nil
			},
		)
	}

	if runtimeExecutor != nil {
		rtExecutor = recovery.NewRuntimeExecutorAdapter(
			func(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
				return runtimeExecutor.StartRuntime(ctx, runtimeID)
			},
			func(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
				return runtimeExecutor.StopRuntime(ctx, runtimeID)
			},
		)
	}

	if secretAdapter != nil {
		secretLeaseAdapter = recovery.NewSecretLeaseAdapter(
			func(runtimeID string) int {
				return secretAdapter.RevokeRuntimeLeases(runtimeID, "recovery").RevokedCount
			},
			func(ctx context.Context, req recovery.SecretLeaseRequest) (recovery.SecretLeaseResult, error) {
				return recovery.SecretLeaseResult{Success: true, LeaseID: "placeholder"}, nil
			},
		)
	}

	if runtimeProvisioner != nil && runtimeManager != nil && pluginReg != nil {
		structureBuilderAdapter = recovery.NewHostStructureBuilderAdapter(
			func(ctx context.Context, pluginID domain.PluginID, extensionID string) (recovery.TopologyResult, error) {
				desc, err := pluginReg.Get(ctx, pluginID)
				if err != nil {
					return recovery.TopologyResult{Valid: false}, err
				}
				return recovery.TopologyResult{
					TopologyID: desc.ID,
					ServiceIDs: []string{desc.ID},
					Valid:      true,
				}, nil
			},
			func(ctx context.Context, topology recovery.TopologyResult) (recovery.LifecycleResult, error) {
				return recovery.LifecycleResult{
					PlanID: topology.TopologyID + "-plan",
					Valid:  topology.Valid,
				}, nil
			},
		)
	}

	if opts.EffectivePermission != nil && runtimeManager != nil {
		subjectMapper := integration.NewGameHostPermissionSubjectResolver(runtimeManager, pluginReg)
		ghSubjectMapper := permission.NewGameHostSubjectMapper(subjectMapper)
		permissionAdapter = &recovery.PermissionServiceAdapter{
			ResolveFn: func(ctx context.Context, runtimeID, pluginID string) (recovery.PermissionView, error) {
				subject, err := ghSubjectMapper.MapSubject(runtimeID, pluginID)
				if err != nil {
					return recovery.PermissionView{}, err
				}
				view := opts.EffectivePermission.ResolveRuntimePermissions(ctx, subject, "run")
				return recovery.PermissionView{
					Revision:    view.Revision,
					Permissions: view.AllowedPermissions(),
				}, nil
			},
		}
	}

	if controlManager != nil {
		authorityAdapter = &recovery.ControlAuthorityViewAdapter{
			GetAuthorityFn: func(runtimeID domain.RuntimeInstanceID) (recovery.AuthoritySnapshot, error) {
				return recovery.AuthoritySnapshot{
					RuntimeID: runtimeID,
					Mode:      "standard",
					Epoch:     1,
				}, nil
			},
		}
	}

	var productionAuditSink recovery.AuditSink
	if opts.EventService != nil {
		productionAuditSink = recovery.NewAuditSinkAdapter(func(event recovery.RecoveryAuditEvent) {
			opts.EventService.Emit(domain.Event{
				Type:      "recovery",
				Payload:   event,
				Timestamp: time.Now(),
			})
		})
	} else {
		productionAuditSink = recovery.NewAuditSinkAdapter(func(event recovery.RecoveryAuditEvent) {})
	}

	recoveryCoordinator, err := ComposeRecoveryCoordinator(recovery.RecoveryCoordinatorDeps{
		Kernel:               kernelRollbackAdapter,
		Supervisor:           supervisorViewAdapter,
		PluginRegistry:       pluginReg,
		RuntimeManager:       runtimeManager,
		RuntimeExecutor:      rtExecutor,
		SecretLease:          secretLeaseAdapter,
		Permission:           permissionAdapter,
		AuthorityView:        authorityAdapter,
		CheckpointClassifier: checkpointClassifier,
		StructureBuilder:     structureBuilderAdapter,
		AuditSink:            productionAuditSink,
	})
	if err != nil {
		return nil, fmt.Errorf("compose recovery coordinator: %w", err)
	}

	container := &GameHostContainer{
		DirectoryManager:     dirMgr,
		CheckpointStore:      checkpointStore,
		ConfigStore:          configStore,
		ConfigResolver:       configResolver,
		PluginRegistry:       pluginReg,
		ContributionSync:     contributionSync,
		RuntimeManager:       runtimeManager,
		RuntimeTopologyStore: topologyStore,
		RuntimeHealth:        runtimeHealth,
		RuntimeExecutor:      runtimeExecutor,
		RuntimeProvisioner:   runtimeProvisioner,
		NamespaceRegistry:    nsReg,
		HandshakeManager:     handshakeMgr,
		ReadyGate:            readyGate,
		ConnectionRegistry:   connReg,
		ControlPlane:         controlPlane,
		RPCDispatcher:        rpcDispatcher,
		RPCLifecycle:         rpcLifecycle,
		HostHandlerRegistry:  hostHandlers,
		ChannelRegistry:      channelReg,
		NotificationBridge:   notifBridge,
		StateStore:           stateStore,
		BinaryObjectRegistry: binaryReg,
		StreamManager:        streamMgr,
		procAdapter:          procAdapter,
		HostAPIGateway:       opts.HostAPIGateway,

		ResourceAdapter:    resourceAdapter,
		ResourceViewer:     resourceViewer,
		ResourceLifecycle:  resourceLifecycle,

		AuthorityManager:      controlManager,
		OutputGate:            pluginOutputGate,
		TakeoverService:       takeoverService,
		AuthorityAudit:        auditSink,
		ControlSinkRegistry:   controlSinkRegistry,
		CommitBarrier:         commitBarrier,
		EmergencyStopService: emergencyStopService,

		SecretLeaseAdapter:  secretAdapter,
		SecretLifecycle:     secretLifecycle,
		SecretSubscriptions: secretSubscriptions,

	StartupGate:         startupGate,
	RecoveryCoordinator: recoveryCoordinator,
}

var runtimeGraphReconcile upgrade.RuntimeGraphReconciler
if runtimeProvisioner != nil {
	runtimeGraphReconcile = upgrade.NewRuntimeGraphReconcilerAdapter(runtimeProvisioner)
}

upgradeCoordinator, err := ComposeUpgradeCoordinator(
	pluginReg,
	runtimeManager,
	runtimeExecutor,
	contributionSync,
	opts.DefinitionReconcile,
	runtimeGraphReconcile,
	configResolver,
	opts.KernelLifecycle,
	opts.ArchiveUpdater,
)
if err != nil {
	return nil, fmt.Errorf("compose upgrade coordinator: %w", err)
}
container.UpgradeCoordinator = upgradeCoordinator

startupRecovery, err := ComposeStartupRecovery(startup.StartupRecoveryDeps{
	HostIdentity:    startup.NewHostIdentity("", ""),
	ProcessCleanup:  startup.NewProcessCleanupAdapter(),
	TempCleanup:     startup.NewTempCleanupAdapter(),
	BinaryCleanup:   startup.NewBinaryCleanupAdapter(),
	EndpointCleanup: startup.NewEndpointCleanupAdapter(),
	ShmCleanup:      startup.NewSharedMemoryCleanupAdapter(),
	KernelRecon:     startup.NewKernelReconciliationAdapter(),
	AuditSink:       startup.NewAuditSinkLoggerAdapter(),
	Gate:            startupGate,
})
if err != nil {
	return nil, fmt.Errorf("compose startup recovery: %w", err)
}
container.StartupRecovery = startupRecovery

	if opts.HostAPIGateway != nil {
		runtimeReader := integration.NewHostAPIRuntimeAdapter(runtimeManager, pluginReg)
		topologyReader := integration.NewHostAPITopologyAdapter(topologyStore)
		generationReader := integration.NewHostAPIGenerationReader(runtimeManager)
		runtimeResolver := integration.NewHostAPIRuntimeIdentityResolver(runtimeManager, pluginReg, topologyStore)

		var permProvider *integration.HostAPIPermissionProvider
		if opts.KernelPermissionBroker != nil {
			permProvider = integration.NewHostAPIPermissionProvider()
		}

		var scopeProvider *integration.HostAPIScopeProvider
		if opts.KernelScopeManager != nil {
			scopeProvider = integration.NewHostAPIScopeProvider(opts.KernelScopeManager)
		}

		adapter, err := hostapi.NewProductionHostAPIAdapter(hostapi.ProductionHostAPIAdapterDeps{
			Gateway:            opts.HostAPIGateway,
			PluginRegistry:     pluginReg,
			RuntimeReader:      runtimeReader,
			TopologyReader:     topologyReader,
			ReadyGate:          readyGate,
			PermissionProvider: permProvider,
			ScopeProvider:      scopeProvider,
			RuntimeResolver:    runtimeResolver,
			GenerationReader:   generationReader,
			ConnectionRegistry: readyGate,
		})
		if err != nil {
			return nil, err
		}
		container.HostAPIAdapter = adapter
	}

	if opts.TrustedSupervisor != nil {
		handler := integration.NewGameHostStdioProtocolHandler(
			controlPlane,
			runtimeManager,
			topologyStore,
			pluginReg,
		)
		if err := opts.TrustedSupervisor.RegisterStdioProtocolHandler(protocol.ProtocolVersion, handler); err != nil {
			return nil, fmt.Errorf("register gamehost stdio protocol handler: %w", err)
		}
	}

	if secretAdapter != nil {
		secretHandler := gamehostsecret.NewSecretRPCHandler(secretAdapter)
		if err := secretHandler.Register(hostHandlers); err != nil {
			return nil, fmt.Errorf("register secret RPC handlers: %w", err)
		}
	}

	if opts.TrustedSupervisor != nil && container.RecoveryCoordinator != nil {
		exitBridge := runtime.NewProcessExitBridge(
			opts.TrustedSupervisor,
			container.RecoveryCoordinator,
			topologyStore,
			runtimeManager,
		)
		exitBridge.RegisterObserver()
		container.ProcessExitBridge = exitBridge
	}

	return container, nil
}

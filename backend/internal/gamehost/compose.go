package gamehost

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	kernelpermission "github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	ghTrustedService "github.com/u-ai/backend/internal/extension/kernel/trusted_service"
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
	"github.com/u-ai/backend/internal/gamehost/recovery"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/resource"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/runtime/checkpoint"
	gamehostsecret "github.com/u-ai/backend/internal/gamehost/secret"
	"github.com/u-ai/backend/internal/gamehost/startup"
	"github.com/u-ai/backend/internal/gamehost/state"
	"github.com/u-ai/backend/internal/gamehost/storage"
	"github.com/u-ai/backend/internal/gamehost/stream"
	"github.com/u-ai/backend/internal/gamehost/stream/binary"
	"github.com/u-ai/backend/internal/gamehost/upgrade"
	"github.com/u-ai/backend/internal/desktoppet/plugin"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RollbackPostAction func(ctx context.Context, extensionID string) error

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
	RollbackPostAction  RollbackPostAction

	EffectivePermission *permission.EffectivePermissionAdapter
	PermissionBroker    permission.Broker

	KernelPermissionBroker        kernelpermission.PermissionBroker
	KernelPermissionSnapshotStore kernelpermission.PermissionSnapshotStore
	KernelScopeManager            scope.ScopeManager
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

func getRollbackArchivePath(ctx context.Context, extensionID string, opts GameHostComposeOptions) string {
	if extensionID == "" || opts.ArchiveUpdater == nil {
		return ""
	}
	archivePath, err := opts.ArchiveUpdater.GetPreviousArchivePath(ctx, extensionID)
	if err != nil {
		log.Printf("[gamehost] get rollback archive path failed for %s: %v", extensionID, err)
		return ""
	}
	return archivePath
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
	runtimeManager := runtime.NewManager(runtime.ManagerOptions{})
	topologyStore := runtime.NewTopologyStore()

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
		HostSupportedProtocols: []string{protocol.ProtocolVersion},
		HostCapabilities:       handshake.ProductionHostCapabilities(),
		NamespaceAdapter:       nsAdapter,
		ChannelAdvertiser:      channelAdvertiser,
		RuntimeValidator:       integration.NewHandshakeRuntimeValidator(runtimeManager, topologyStore),
		DescriptorProvider:     pluginReg,
	})

	hostHandlers := rpc.NewHostHandlerRegistry()
	rpcLifecycle := rpc.NewLifecycleManager(rpc.LifecycleManagerConfig{})
	rpcDispatcher := rpc.NewRPCDispatcher(rpc.DispatcherConfig{
		Namespaces:   nsReg,
		HostHandlers: hostHandlers,
		Lifecycle:    rpcLifecycle,
	})

	handshakeController := handshake.NewHandshakeControllerAdapter(handshakeMgr, readyGate)

	responseCorrelator := rpc.NewRPCResponseCorrelator(rpcLifecycle.Registry())

	controlPlane, err := ipc.NewControlPlane(ipc.ControlPlaneConfig{
		Registry:            connReg,
		Resolver:            integration.NewRuntimePeerResolver(topologyStore),
		Dispatcher:          rpcDispatcher,
		HandshakeController: handshakeController,
		ResponseCorrelator:  responseCorrelator,
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

	resourceMapper := newResourceSubjectMapper(pluginReg)
	resourceGovernor := newRuntimeLimitGovernor()
	resourceAdapter := resource.NewResourceAdmissionAdapter(resourceMapper, nil, binaryReg, resourceGovernor)
	resourceViewer := resource.NewResourcePolicyViewer(newContainerViewResolver(binaryReg, streamMgr))
	resourceLifecycle := resource.NewLifecycleCoordinator(resourceAdapter, resourceViewer)

	controlSinkRegistry := control.NewControlSinkRegistry()
	commitBarrier := control.NewControlCommitBarrier()
	metricsSink := control.MetricsSink(control.NewDiscardMetricsSink())
	if opts.EventService != nil {
		metricsSink = integration.NewControlMetricsAdapter(opts.EventService, pluginReg)
	}

	topologyAdapter := integration.NewControlTopologyAdapter(topologyStore)
	runtimeAdapter := integration.NewControlRuntimeAdapter(runtimeManager)
	hostPolicyAdapter := integration.NewControlHostPolicyAdapter(runtimeManager)

	var auditSink control.AuthorityAuditSink
	auditSink = integration.NewControlAuditAdapter(opts.EventService, pluginReg)

	if opts.EffectivePermission == nil && opts.PermissionBroker != nil {
		subjectResolver := integration.NewGameHostPermissionSubjectResolver(runtimeManager, pluginReg)
		subjectMapper := permission.NewGameHostSubjectMapper(subjectResolver)
		opts.EffectivePermission = permission.NewEffectivePermissionAdapter(opts.PermissionBroker, nil, subjectMapper)
	}

	permChecker := integration.NewControlPermissionAdapter(opts.EffectivePermission)

	controlManager := control.NewControlAuthorityManager(control.ControlAuthorityManagerOptions{
		Audit:         auditSink,
		CommitBarrier: commitBarrier,
	})

	pluginOutputGate, err := control.NewPluginOutputGate(control.PluginOutputGateOptions{
		Topology:         topologyAdapter,
		RuntimeReader:    runtimeAdapter,
		GenerationReader: runtimeAdapter,
		PermChecker:      permChecker,
		PolicyChecker:    hostPolicyAdapter,
		Authority:        controlManager,
		Audit:            auditSink,
		Metrics:          metricsSink,
		CommitBarrier:    commitBarrier,
	})
	if err != nil {
		return nil, fmt.Errorf("compose plugin output gate: %w", err)
	}

	takeoverService := control.NewTakeoverService(control.TakeoverServiceOptions{
		Manager: controlManager,
		Audit:   auditSink,
	})

	effectSinkFactory := integration.NewProtocolControlEffectSinkFactory(connReg, controlPlane)
	if err := control.NewControlHandlerWithEffectFactory(pluginOutputGate, controlSinkRegistry, effectSinkFactory.CreateSink).RegisterHandlers(hostHandlers); err != nil {
		return nil, fmt.Errorf("register control RPC handlers: %w", err)
	}

	var runtimeProvisioner *integration.RuntimeGraphProvisioner
	var serviceExecutor runtime.ServiceExecutor
	var runtimeExecutor runtime.RuntimeExecutor
	var runtimeHealth runtime.HealthAdapter
	var secretLifecycle *gamehostsecret.LifecycleOrchestrator

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
			SecretRegistrar:  secretLifecycle,
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

	baseEmergencyStore := integration.NewInMemoryEmergencyIntentStore()
	emergencyIntentStore := integration.NewManagerEmergencyLatchBridge(runtimeManager, baseEmergencyStore)
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
	emergencyHostAPITracker := integration.NewHostAPIInvocationTracker()
	var emergencyStopService *control.EmergencyStopService

	var secretAdapter *gamehostsecret.SecretLeaseAdapter
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

		if secretLifecycle == nil {
			secretLifecycle = gamehostsecret.NewLifecycleOrchestrator(secretAdapter)
		}
		secretSubscriptions = gamehostsecret.NewSubscriptionAdapter(secretAdapter)
		if leaseAwareExecutor, ok := serviceExecutor.(runtime.SecretLeaseAwareServiceExecutor); ok {
			leaseAwareExecutor.SetServiceLeaseLifecycle(secretLifecycle)
		}
		if runtimeExecutor != nil {
			runtimeExecutor.SetRuntimeSubscriptionWatcher(secretSubscriptions)
		}
	}

	var emergencySecretAdapter control.SecretLeaseRevoker
	var emergencySecretVerifier control.SecretLeaseVerifier
	if secretAdapter != nil {
		emergencySecret := integration.NewEmergencySecretLeaseAdapter(secretAdapter)
		emergencySecretAdapter = emergencySecret
		emergencySecretVerifier = emergencySecret
	} else {
		emergencySecretAdapter = &noopSecretLeaseHandler{}
		emergencySecretVerifier = &noopSecretLeaseHandler{}
	}
	emergencyStopService, err = control.NewEmergencyStopService(control.EmergencyStopServiceOptions{
		Authority:          controlManager,
		Gate:               pluginOutputGate,
		Intent:             emergencyIntentStore,
		LifecycleIntent:    runtimeManager,
		RuntimeStopper:     emergencyRuntimeAdapter,
		PendingCanceller:   emergencyRPCAdapter,
		HostAPICanceller:   emergencyHostAPITracker,
		LeaseRevoker:       emergencySecretAdapter,
		ConnectionCloser:   emergencyConnectionAdapter,
		HandshakeReset:     emergencyReadyAdapter,
		StreamStopper:      emergencyStreamAdapter,
		ChannelCleaner:     emergencyChannelAdapter,
		BinaryReleaser:     emergencyBinaryAdapter,
		PendingVerifier:    emergencyPendingVerifier,
		ConnectionVerifier: emergencyConnectionVerifier,
		LeaseVerifier:      emergencySecretVerifier,
		ReadyVerifier:      emergencyReadyAdapter,
		HostAPIVerifier:    emergencyHostAPITracker,
		StreamVerifier:     emergencyStreamVerifier,
		ChannelVerifier:    emergencyChannelVerifier,
		BinaryVerifier:     emergencyBinaryVerifier,
	})
	if err != nil {
		return nil, fmt.Errorf("compose emergency stop service: %w", err)
	}
	if err := control.NewAuthorityRPCHandler(controlManager, takeoverService, emergencyStopService).RegisterHandlers(hostHandlers); err != nil {
		return nil, fmt.Errorf("register authority RPC handlers: %w", err)
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

	kernelRollbackAdapter = recovery.NewKernelRollbackAdapter(
		func(ctx context.Context, extensionID string, operationID recovery.RecoveryOperationID) (recovery.KernelRollbackResult, error) {
			if opts.ArchiveUpdater == nil {
				return recovery.KernelRollbackResult{Success: false, Error: "kernel rollback is unavailable"}, fmt.Errorf("kernel rollback is unavailable")
			}
			archivePath := getRollbackArchivePath(ctx, extensionID, opts)
			if archivePath == "" {
				return recovery.KernelRollbackResult{Success: false, Error: "no previous package archive available for rollback"}, fmt.Errorf("no previous package archive for extension %s", extensionID)
			}
			result, err := opts.ArchiveUpdater.UpdateArchive(ctx, extensionID, archivePath)
			if err != nil {
				return recovery.KernelRollbackResult{Success: false, Error: err.Error()}, err
			}
			if opts.RollbackPostAction != nil {
				if postErr := opts.RollbackPostAction(ctx, extensionID); postErr != nil {
					log.Printf("[gamehost] rollback post-action failed for %s: %v", extensionID, postErr)
				}
			}
			return recovery.KernelRollbackResult{
				Success:           result.Success,
				NewVersion:        result.NewVersion,
				RequiresReconcile: true,
			}, nil
		},
	)

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
				return recovery.SecretLeaseResult{Success: false, Error: "recovery lease issuance is unavailable"}, fmt.Errorf("recovery lease issuance is unavailable")
			},
		)
	}

	if runtimeProvisioner != nil && runtimeManager != nil && pluginReg != nil {
		lifecyclePlanner := runtime.NewLifecyclePlanner()
		structureBuilderAdapter = recovery.NewHostStructureBuilderAdapter(
			func(ctx context.Context, pluginID domain.PluginID, extensionID string) (recovery.TopologyResult, error) {
				if err := runtimeProvisioner.Reconcile(ctx); err != nil {
					return recovery.TopologyResult{Valid: false}, err
				}
				runtimes, err := runtimeManager.List(ctx)
				if err != nil {
					return recovery.TopologyResult{Valid: false}, err
				}
				for _, candidate := range runtimes {
					if candidate.PluginID != pluginID {
						continue
					}
					snapshot, snapshotErr := topologyStore.GetTopologySnapshot(candidate.ID)
					if snapshotErr != nil {
						return recovery.TopologyResult{Valid: false}, snapshotErr
					}
					serviceIDs := make([]string, 0, len(snapshot.Services))
					for _, service := range snapshot.Services {
						serviceIDs = append(serviceIDs, string(service.ID))
					}
					return recovery.TopologyResult{TopologyID: string(candidate.ID), ServiceIDs: serviceIDs, Valid: true}, nil
				}
				return recovery.TopologyResult{Valid: false}, fmt.Errorf("runtime not found for plugin %s", pluginID)
			},
		func(ctx context.Context, topology recovery.TopologyResult) (recovery.LifecycleResult, error) {
			if !topology.Valid || topology.TopologyID == "" {
				return recovery.LifecycleResult{Valid: false}, fmt.Errorf("invalid topology for lifecycle plan")
			}
			runtimeID := domain.RuntimeInstanceID(topology.TopologyID)
			snapshot, err := topologyStore.GetTopologySnapshot(runtimeID)
			if err != nil {
				return recovery.LifecycleResult{Valid: false}, fmt.Errorf("load topology snapshot: %w", err)
			}
			graph := runtime.DependencyGraphSnapshot{RuntimeID: runtimeID, Nodes: make([]runtime.DependencyNodeSnapshot, 0, len(snapshot.Services))}
			for _, svc := range snapshot.Services {
				deps := make([]domain.ServiceID, len(svc.Dependencies))
				copy(deps, svc.Dependencies)
				graph.Nodes = append(graph.Nodes, runtime.DependencyNodeSnapshot{ServiceID: svc.ServiceID, Dependencies: deps})
			}
			plan, planErr := lifecyclePlanner.BuildStartupPlan(snapshot, graph)
			if planErr != nil {
				return recovery.LifecycleResult{Valid: false}, fmt.Errorf("build lifecycle plan: %w", planErr)
			}
			if plan.ServiceCount() != len(snapshot.Services) {
				return recovery.LifecycleResult{Valid: false}, fmt.Errorf("lifecycle plan incomplete: plan=%d total=%d", plan.ServiceCount(), len(snapshot.Services))
			}
			planID := fmt.Sprintf("plan-%s-%d", topology.TopologyID, time.Now().UnixNano())
			return recovery.LifecycleResult{
				PlanID: planID,
				Valid:  true,
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
				snapshot, err := controlManager.Get(context.Background(), runtimeID)
				if err != nil {
					return recovery.AuthoritySnapshot{}, err
				}
				return recovery.AuthoritySnapshot{
					RuntimeID: runtimeID,
					Mode:      string(snapshot.Mode),
					Epoch:     snapshot.Epoch,
				}, nil
			},
		}
	}

	var productionAuditSink recovery.AuditSink
	if opts.EventService != nil {
		productionAuditSink = recovery.NewAuditSinkAdapter(func(auditEvent recovery.RecoveryAuditEvent) {
			payload := map[string]interface{}{
				"operationId": string(auditEvent.OperationID),
				"runtimeId":   string(auditEvent.RuntimeID),
				"extensionId": auditEvent.ExtensionID,
				"pluginId":    string(auditEvent.PluginID),
				"failureClass": string(auditEvent.FailureClass),
				"stage":       string(auditEvent.Stage),
				"attempt":     auditEvent.Attempt,
				"result":      auditEvent.Result,
				"error":       auditEvent.Error,
				"timestamp":   auditEvent.Timestamp.Format(time.RFC3339Nano),
			}
			payloadJSON, _ := json.Marshal(payload)
evtOpts := event.PublishOptions{
			Metadata: json.RawMessage(`"` + string(auditEvent.OperationID) + `"`),
		}
		_, _ = opts.EventService.Publish(nil, "gamehost.recovery.audit", 1, payloadJSON, evtOpts)
		})
	} else {
		productionAuditSink = recovery.NewAuditSinkAdapter(func(event recovery.RecoveryAuditEvent) {
			log.Printf("[recovery-audit] op=%s runtime=%s ext=%s plugin=%s stage=%s result=%s err=%s",
				event.OperationID, event.RuntimeID, event.ExtensionID, event.PluginID, event.Stage, event.Result, event.Error)
		})
	}

	recoveryRuntimeManager := &recoveryRuntimeManagerAdapter{mgr: runtimeManager}

	intentChecker := recovery.NewRuntimeLifecycleIntentCheckerAdapter(
		runtimeManager.IsEmergencyLatched,
		runtimeManager.GetLifecycleIntent,
	)
	extensionChecker := recovery.NewExtensionStateCheckerAdapter(
		func(extensionID string) bool {
			if opts.KernelSource != nil {
				plugins, err := opts.KernelSource.ListEnabledGamePlugins(context.Background())
				if err == nil {
					for _, p := range plugins {
						if string(p.Extension.ID) == extensionID {
							return true
						}
					}
					return false
				}
			}
			plugins, err := pluginReg.ListByExtension(context.Background(), extensionID)
			return err == nil && len(plugins) > 0
		},
		func(extensionID string) bool {
			if opts.KernelSource != nil {
				plugins, err := opts.KernelSource.ListEnabledGamePlugins(context.Background())
				if err == nil {
					for _, p := range plugins {
						if string(p.Extension.ID) == extensionID {
							return true
						}
					}
					return false
				}
			}
			plugins, err := pluginReg.ListByExtension(context.Background(), extensionID)
			return err == nil && len(plugins) > 0
		},
		func(pluginID domain.PluginID) bool {
			_, err := pluginReg.Get(context.Background(), pluginID)
			return err == nil
		},
	)
	eligibilityChecker := recovery.NewRecoveryEligibilityChecker(intentChecker, extensionChecker)

	recoveryCoordinator, err := ComposeRecoveryCoordinator(recovery.RecoveryCoordinatorDeps{
		Kernel:               kernelRollbackAdapter,
		Supervisor:           supervisorViewAdapter,
		PluginRegistry:       pluginReg,
		RuntimeManager:       recoveryRuntimeManager,
		RuntimeExecutor:      rtExecutor,
		SecretLease:          secretLeaseAdapter,
		Permission:           permissionAdapter,
		AuthorityView:        authorityAdapter,
		CheckpointClassifier: checkpointClassifier,
		StructureBuilder:     structureBuilderAdapter,
		AuditSink:            productionAuditSink,
		EligibilityChecker:   eligibilityChecker,
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

		ResourceAdapter:   resourceAdapter,
		ResourceViewer:    resourceViewer,
		ResourceLifecycle: resourceLifecycle,

		AuthorityManager:     controlManager,
		OutputGate:           pluginOutputGate,
		TakeoverService:      takeoverService,
		AuthorityAudit:       auditSink,
		ControlSinkRegistry:  controlSinkRegistry,
		CommitBarrier:        commitBarrier,
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

	var kernelRecon startup.KernelReconciliationProvider
	if pluginReg != nil && runtimeManager != nil {
		var extLookup interface {
			ListEnabledGamePlugins(ctx context.Context) ([]startup.KernelGamePlugin, error)
		}
		if opts.KernelSource != nil {
			extLookup = &startup.KernelGamePluginAdapter{Source: opts.KernelSource}
		}
		kernelRecon = startup.NewKernelReconciliationAdapter(
			pluginReg,
			runtimeManager,
			extLookup,
		)
	}

	var binaryCleanup startup.BinaryCleanupProvider
	if binaryReg != nil && runtimeManager != nil {
		binaryCleanup = startup.NewBinaryCleanupAdapter(binaryReg, runtimeManager)
	}

	if kernelRecon == nil {
		kernelRecon = startup.NewNoopKernelReconciliationProvider()
	}
	if binaryCleanup == nil {
		binaryCleanup = startup.NewNoopBinaryCleanupProvider()
	}

	var startupAuditSink startup.AuditSink
	if opts.EventService != nil {
		evtSvc := opts.EventService
		startupAuditSink = startup.NewEventServiceAuditSinkAdapter(func(startupEvent startup.StartupRecoveryAuditEvent) {
			payload := map[string]interface{}{
				"operationId":   startupEvent.OperationID,
				"stage":         startupEvent.Stage,
				"resourceType":  startupEvent.ResourceType,
				"resourceId":    startupEvent.ResourceID,
				"error":         startupEvent.Error,
				"timestamp":     time.Now().Format(time.RFC3339Nano),
			}
			payloadJSON, _ := json.Marshal(payload)
			evtOpts := event.PublishOptions{
				Metadata: json.RawMessage(`"` + startupEvent.OperationID + `"`),
			}
			_, _ = evtSvc.Publish(nil, "gamehost.startup.audit", 1, payloadJSON, evtOpts)
		})
	} else {
		startupAuditSink = startup.NewAuditSinkLoggerAdapter()
	}

	hostIdentity := startup.NewHostIdentity("", "")
	if opts.TrustedSupervisor != nil {
		opts.TrustedSupervisor.SetHostIdentityProvider(hostIdentity)
	}

	startupRecovery, err := ComposeStartupRecovery(startup.StartupRecoveryDeps{
		HostIdentity:      hostIdentity,
		ProcessCleanup:    startup.NewProcessCleanupAdapterWithIdentity(opts.TrustedSupervisor, hostIdentity, runtimeManager),
		TempCleanup:       startup.NewTempCleanupAdapter(dirMgr),
		BinaryCleanup:     binaryCleanup,
		EndpointCleanup:   startup.NewEndpointCleanupAdapter(),
		ShmCleanup:        startup.NewSharedMemoryCleanupAdapter(),
		KernelRecon:       kernelRecon,
		RuntimeGraphRecon: runtimeProvisioner,
		AuditSink:         startupAuditSink,
		Gate:              startupGate,
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
		if opts.KernelPermissionSnapshotStore != nil {
			permProvider = integration.NewHostAPIPermissionProvider(opts.KernelPermissionSnapshotStore)
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
			InvocationTracker:  emergencyHostAPITracker,
			PermissionChecker:  opts.EffectivePermission,
		})
		if err != nil {
			return nil, err
		}
		container.HostAPIAdapter = adapter
		if err := hostapi.RegisterHostAPIMethods(adapter, hostHandlers, opts.HostAPIGateway.ListMethods(context.Background())); err != nil {
			return nil, fmt.Errorf("register host API RPC handlers: %w", err)
		}
		if err := hostHandlers.Register(hostapi.HostInvokeMethod, hostapi.NewHostInvokeHandler(adapter)); err != nil {
			return nil, fmt.Errorf("register host.invoke handler: %w", err)
		}
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
		if err := opts.TrustedSupervisor.RegisterStdioProtocolHandler(plugin.PetSupportedProtocol, handler); err != nil {
			return nil, fmt.Errorf("register desktop pet stdio protocol handler: %w", err)
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
		if secretLifecycle != nil {
			if leaseAwareBridge, ok := exitBridge.(interface {
				SetRuntimeGenerationLeaseRevoker(runtime.RuntimeGenerationLeaseRevoker)
			}); ok {
				leaseAwareBridge.SetRuntimeGenerationLeaseRevoker(secretLifecycle)
			}
		}
		exitBridge.RegisterObserver()
		container.ProcessExitBridge = exitBridge
	}

	return container, nil
}

type recoveryRuntimeManagerAdapter struct {
	mgr runtime.RuntimeManager
}

func (a *recoveryRuntimeManagerAdapter) GetRuntime(runtimeID domain.RuntimeInstanceID) (*recovery.RuntimeInstanceRef, error) {
	rt, err := a.mgr.GetRuntime(runtimeID)
	if err != nil {
		return nil, err
	}
	return &recovery.RuntimeInstanceRef{ID: rt.ID, PluginID: rt.PluginID, State: rt.State}, nil
}

func (a *recoveryRuntimeManagerAdapter) ListRuntimes() []*recovery.RuntimeInstanceRef {
	rts := a.mgr.ListRuntimes()
	out := make([]*recovery.RuntimeInstanceRef, 0, len(rts))
	for _, rt := range rts {
		out = append(out, &recovery.RuntimeInstanceRef{ID: rt.ID, PluginID: rt.PluginID, State: rt.State})
	}
	return out
}

type noopSecretLeaseHandler struct{}

func (n *noopSecretLeaseHandler) RevokeRuntimeLeases(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	return 0, nil
}

func (n *noopSecretLeaseHandler) CountRuntimeLeases(runtimeID domain.RuntimeInstanceID) int {
	return 0
}

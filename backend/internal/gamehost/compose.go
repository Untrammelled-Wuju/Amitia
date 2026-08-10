package gamehost

import (
	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/config"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/integration"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/internal/gamehost/notification"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/rpc"
	"github.com/u-ai/backend/internal/gamehost/runtime"
	"github.com/u-ai/backend/internal/gamehost/runtime/checkpoint"
	"github.com/u-ai/backend/internal/gamehost/state"
	"github.com/u-ai/backend/internal/gamehost/storage"
	"github.com/u-ai/backend/internal/gamehost/stream"
	"github.com/u-ai/backend/internal/gamehost/stream/binary"
	ghTrustedService "github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type GameHostComposeOptions struct {
	DataRoot          string
	KernelSource      integration.KernelContributionSource
	TrustedSupervisor *ghTrustedService.ProcessSupervisor
	EventService      *event.Service
}

func NewKernelContributionSource(
	instRepo installationLister,
	defRepo definitionLister,
	contribRepo contributionLister,
) integration.KernelContributionSource {
	return newKernelContributionSource(instRepo, defRepo, contribRepo)
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

	return &GameHostContainer{
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
	}, nil
}

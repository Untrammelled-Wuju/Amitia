package gamehost

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/config"
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
	"github.com/u-ai/backend/internal/gamehost/storage"
	"github.com/u-ai/backend/internal/gamehost/stream"
	"github.com/u-ai/backend/internal/gamehost/stream/binary"
	"github.com/u-ai/backend/internal/gamehost/recovery"
	"github.com/u-ai/backend/internal/gamehost/startup"
	"github.com/u-ai/backend/internal/gamehost/upgrade"
)

type GameHostContainer struct {
	DirectoryManager storage.DirectoryManager
	CheckpointStore  checkpoint.CheckpointStore
	ConfigStore      *config.FileStore
	ConfigResolver   *config.Resolver

	PluginRegistry   *registry.Registry
	ContributionSync *integration.GamePluginSyncService
	RuntimeExecutor  runtime.RuntimeExecutor

	NamespaceRegistry   rpc.NamespaceRegistry
	HandshakeManager    *handshake.HandshakeManager
	ReadyGate          *handshake.ReadyGate
	ConnectionRegistry *ipc.ConnectionRegistry
	ChannelRegistry    channel.Registry

	NotificationBridge   *notification.Bridge
	StateStore           *state.LatestStateStore
	BinaryObjectRegistry binary.ObjectRegistry
	StreamManager        *stream.StreamManager

	HostAPIGateway host_api.Gateway
	HostAPIAdapter *hostapi.HostAPIAdapter

	ResourceAdapter      resource.AdmissionAdapter
	ResourceViewer      *resource.ResourcePolicyViewer
	ResourceLifecycle   *resource.LifecycleCoordinator

	procAdapter runtime.ProcessSupervisorAdapter

	UpgradeCoordinator  *upgrade.UpgradeCoordinator
	RecoveryCoordinator *recovery.RecoveryCoordinator
	StartupRecovery     *startup.StartupRecoveryCoordinator
	StartupGate         *startup.StartupGate
}

func (c *GameHostContainer) Shutdown(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.StreamManager != nil {
		c.StreamManager.Shutdown(ctx)
	}
	if c.HandshakeManager != nil {
		c.HandshakeManager.Shutdown(ctx)
	}
	if c.ResourceLifecycle != nil {
		c.ResourceLifecycle.OnHostShutdown()
	}
	return nil
}

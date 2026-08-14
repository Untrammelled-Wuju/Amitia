package gamehost

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	gamehostsecret "github.com/u-ai/backend/internal/gamehost/secret"
	"github.com/u-ai/backend/internal/gamehost/channel"
	"github.com/u-ai/backend/internal/gamehost/config"
	"github.com/u-ai/backend/internal/gamehost/control"
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

	RuntimeManager       *runtime.Manager
	RuntimeTopologyStore *runtime.TopologyStore
	RuntimeHealth        runtime.HealthAdapter
	RuntimeExecutor      runtime.RuntimeExecutor
	RuntimeProvisioner  *integration.RuntimeGraphProvisioner

	NamespaceRegistry    rpc.NamespaceRegistry
	HandshakeManager     *handshake.HandshakeManager
	ReadyGate            *handshake.ReadyGate
	ConnectionRegistry   *ipc.ConnectionRegistry
	ControlPlane         ipc.ControlPlane
	RPCDispatcher        *rpc.RPCDispatcher
	RPCLifecycle         *rpc.RequestLifecycleManager
	HostHandlerRegistry  rpc.HandlerRegistry
	ChannelRegistry      channel.Registry

	NotificationBridge   *notification.Bridge
	StateStore           *state.LatestStateStore
	BinaryObjectRegistry binary.ObjectRegistry
	StreamManager        *stream.StreamManager

	HostAPIGateway host_api.Gateway
	HostAPIAdapter *hostapi.HostAPIAdapter

	ResourceAdapter    resource.AdmissionAdapter
	ResourceViewer    *resource.ResourcePolicyViewer
	ResourceLifecycle *resource.LifecycleCoordinator

	AuthorityManager      *control.ControlAuthorityManager
	OutputGate            *control.PluginOutputGate
	TakeoverService       *control.TakeoverService
	AuthorityAudit        control.AuthorityAuditSink
	EmergencyStopService  *control.EmergencyStopService
	ControlSinkRegistry   *control.ControlSinkRegistry
	CommitBarrier         *control.ControlCommitBarrierImpl

	procAdapter runtime.ProcessSupervisorAdapter

	SecretLeaseAdapter  *gamehostsecret.SecretLeaseAdapter
	SecretLifecycle     *gamehostsecret.LifecycleOrchestrator
	SecretSubscriptions *gamehostsecret.SubscriptionAdapter

	UpgradeCoordinator  *upgrade.UpgradeCoordinator
	RecoveryCoordinator *recovery.RecoveryCoordinator
	StartupRecovery     *startup.StartupRecoveryCoordinator
	StartupGate         *startup.StartupGate
	ProcessExitBridge   runtime.ProcessExitBridge
}

func (c *GameHostContainer) Start(ctx context.Context) error {
	if c == nil {
		return nil
	}

	if c.ContributionSync != nil {
		result := c.ContributionSync.FullSync(ctx)
		if result.HasError() {
			return fmt.Errorf("gamehost: contribution sync failed: %v", result.Errors)
		}
	}

	if c.RuntimeProvisioner != nil {
		if err := c.RuntimeProvisioner.Reconcile(ctx); err != nil {
			return fmt.Errorf("gamehost: runtime graph reconcile: %w", err)
		}
	}

	if c.StartupRecovery != nil {
		c.StartupRecovery.RunStartupRecovery(ctx)
	} else if c.StartupGate != nil {
		c.StartupGate.Open()
	}
	return nil
}

func (c *GameHostContainer) Shutdown(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.StartupGate != nil {
		c.StartupGate.Close()
	}
	if c.SecretLeaseAdapter != nil {
		c.SecretLeaseAdapter.Shutdown()
	}
	if c.RPCLifecycle != nil {
		c.RPCLifecycle.Shutdown(ctx)
	}
	if c.ControlPlane != nil {
		_ = c.ControlPlane.Shutdown(ctx)
	}
	if c.ResourceLifecycle != nil {
		c.ResourceLifecycle.OnHostShutdown()
	}
	if c.HandshakeManager != nil {
		c.HandshakeManager.Shutdown(ctx)
	}
	if c.StreamManager != nil {
		c.StreamManager.Shutdown(ctx)
	}
	return nil
}

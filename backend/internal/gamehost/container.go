package gamehost

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_api"
	"github.com/u-ai/backend/internal/gamehost/agentbridge"
	"github.com/u-ai/backend/internal/gamehost/channel"
	artifact "github.com/u-ai/backend/internal/gamehost/companion"
	"github.com/u-ai/backend/internal/gamehost/config"
	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/hostapi"
	"github.com/u-ai/backend/internal/gamehost/integration"
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
)

type GameHostContainer struct {
	DirectoryManager storage.DirectoryManager
	CheckpointStore  checkpoint.CheckpointStore
	ConfigStore      *config.FileStore
	ConfigResolver   *config.Resolver
	ArtifactManager  *artifact.ArtifactManager

	PluginRegistry   *registry.Registry
	ContributionSync *integration.GamePluginSyncService

	RuntimeManager       *runtime.Manager
	RuntimeTopologyStore *runtime.TopologyStore
	RuntimeHealth        runtime.HealthAdapter
	RuntimeExecutor      runtime.RuntimeExecutor
	RuntimeProvisioner   *integration.RuntimeGraphProvisioner

	NamespaceRegistry   rpc.NamespaceRegistry
	HandshakeManager    *handshake.HandshakeManager
	ReadyGate           *handshake.ReadyGate
	ConnectionRegistry  *ipc.ConnectionRegistry
	ControlPlane        ipc.ControlPlane
	AgentBridge         *agentbridge.RuntimeAdapter
	RPCDispatcher       *rpc.RPCDispatcher
	RPCLifecycle        *rpc.RequestLifecycleManager
	HostHandlerRegistry rpc.HandlerRegistry
	ChannelRegistry     channel.Registry

	NotificationBridge   *notification.Bridge
	AgentEventSink       *notification.AgentEventSink
	StateStore           *state.LatestStateStore
	BinaryObjectRegistry binary.ObjectRegistry
	BinaryResolver       *binary.Resolver
	BinaryTransfer       *binary.BinaryTransferService
	StreamManager        *stream.StreamManager

	HostAPIGateway           host_api.Gateway
	HostAPIAdapter           *hostapi.HostAPIAdapter
	HostAPIInvocationTracker *integration.HostAPIInvocationTracker

	ResourceAdapter     resource.AdmissionAdapter
	ResourceViewer      *resource.ResourcePolicyViewer
	ResourceLifecycle   *resource.LifecycleCoordinator
	PermissionApprovals *permission.ApprovalCoordinator

	AuthorityManager     *control.ControlAuthorityManager
	OutputGate           *control.PluginOutputGate
	TakeoverService      *control.TakeoverService
	AuthorityAudit       control.AuthorityAuditSink
	EmergencyStopService *control.EmergencyStopService
	ControlSinkRegistry  *control.ControlSinkRegistry
	CommitBarrier        *control.ControlCommitBarrierImpl

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

// ReconcileExtension refreshes one extension's plugin registry view and then
// converges the complete GameHost runtime graph. It is safe to call after
// enable, disable, update, rollback, or uninstall.
func (c *GameHostContainer) ReconcileExtension(ctx context.Context, extensionID string) error {
	if c == nil {
		return nil
	}
	if c.ContributionSync != nil {
		result := c.ContributionSync.SyncExtension(ctx, extensionID)
		if result.HasError() {
			return fmt.Errorf("gamehost: sync extension %s failed: %v", extensionID, result.Errors)
		}
	}
	if c.RuntimeProvisioner != nil {
		if err := c.RuntimeProvisioner.ReconcileExtension(ctx, extensionID); err != nil {
			return fmt.Errorf("gamehost: reconcile extension %s runtime graph: %w", extensionID, err)
		}
	}
	c.pruneAgentContexts()
	return nil
}

// QuiesceExtension stops active game runtimes owned by an extension while
// preserving registry/topology state. Package uninstall uses this before moving
// the active generation so Windows and other platforms never quarantine files
// that are still in use by a game process.
func (c *GameHostContainer) QuiesceExtension(ctx context.Context, extensionID string) error {
	if c == nil || c.PluginRegistry == nil || c.RuntimeManager == nil {
		return nil
	}
	plugins, err := c.PluginRegistry.ListByExtension(ctx, extensionID)
	if err != nil {
		return fmt.Errorf("gamehost: list extension plugins before quiesce: %w", err)
	}
	owned := make(map[domain.PluginID]struct{}, len(plugins))
	for _, plugin := range plugins {
		owned[plugin.ID] = struct{}{}
	}
	for _, runtimeRef := range c.RuntimeManager.ListRuntimes() {
		if runtimeRef == nil {
			continue
		}
		if _, ok := owned[runtimeRef.PluginID]; !ok {
			continue
		}
		if !domain.IsActiveRuntimeState(runtimeRef.State) {
			continue
		}
		if c.RuntimeExecutor == nil {
			return fmt.Errorf("gamehost: runtime executor unavailable while quiescing %s", runtimeRef.ID)
		}
		if err := c.RuntimeExecutor.StopRuntime(ctx, runtimeRef.ID); err != nil {
			return fmt.Errorf("gamehost: stop runtime %s before package mutation: %w", runtimeRef.ID, err)
		}
	}
	return nil
}

// BindAgentContext explicitly associates a GameHost runtime/service with the
// current host Agent scope. It is independent from tool execution, allowing a
// UI/session coordinator to establish the target before the first game event.
func (c *GameHostContainer) BindAgentContext(ctx context.Context, binding capability.RuntimeBinding, invocation capability.ToolInvocationContext) error {
	if c == nil || c.AgentBridge == nil {
		return fmt.Errorf("gamehost: Agent bridge is unavailable")
	}
	return c.AgentBridge.BindAgentContext(ctx, binding, invocation)
}

func (c *GameHostContainer) SetAgentWakeupPort(port notification.AgentWakeupPort) {
	if c == nil || c.AgentEventSink == nil {
		return
	}
	c.AgentEventSink.SetPort(port)
}

func (c *GameHostContainer) CountRuntimeProcesses(runtimeID domain.RuntimeInstanceID) int {
	if c == nil || c.RuntimeTopologyStore == nil || c.procAdapter == nil || runtimeID == "" {
		return 0
	}
	topology, err := c.RuntimeTopologyStore.GetTopology(runtimeID)
	if err != nil || topology == nil {
		return 0
	}
	count := 0
	for _, svc := range topology.ListServices() {
		key := runtime.BuildProcessInstanceID(runtimeID, svc.ServiceID)
		if c.procAdapter.IsRunning(key) {
			count++
		}
	}
	return count
}

func (c *GameHostContainer) pruneAgentContexts() {
	if c == nil || c.AgentBridge == nil || c.RuntimeManager == nil {
		return
	}
	sessions := c.AgentBridge.SessionRegistry()
	if sessions == nil {
		return
	}
	runtimes := c.RuntimeManager.ListRuntimes()
	active := make([]domain.RuntimeInstanceID, 0, len(runtimes))
	for _, runtimeRef := range runtimes {
		if runtimeRef != nil && runtimeRef.ID != "" {
			active = append(active, runtimeRef.ID)
		}
	}
	sessions.RetainRuntimes(active)
}

func (c *GameHostContainer) Start(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.AgentEventSink != nil {
		c.AgentEventSink.Start(ctx)
	}
	if c.BinaryTransfer != nil {
		c.BinaryTransfer.Start(ctx)
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
	c.pruneAgentContexts()

	if c.StartupRecovery != nil {
		report := c.StartupRecovery.RunStartupRecovery(ctx)
		if !report.Success {
			return fmt.Errorf("gamehost: startup recovery failed: %s", strings.Join(report.Errors, "; "))
		}
	} else if c.StartupGate != nil {
		c.StartupGate.Open()
	}
	return nil
}

func (c *GameHostContainer) Shutdown(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.AgentEventSink != nil {
		c.AgentEventSink.Shutdown()
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
		if err := c.ControlPlane.Shutdown(ctx); err != nil {
			return fmt.Errorf("gamehost: control plane shutdown: %w", err)
		}
	}
	if c.BinaryTransfer != nil {
		if err := c.BinaryTransfer.Shutdown(ctx); err != nil {
			return fmt.Errorf("gamehost: binary transfer shutdown: %w", err)
		}
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

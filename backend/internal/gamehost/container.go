package gamehost

import (
	"context"

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
)

// GameHostContainer 是 GameHost 子系统的生产对象组合根。
// 每个 Kernel Container 持有唯一的 GameHostContainer 实例，
// 保证所有 GameHost 核心组件在 Server 进程内只有一个生产实例。
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

	procAdapter runtime.ProcessSupervisorAdapter
}

// Shutdown 执行 GameHost 子系统的有序关闭。
// 关闭顺序: Stream → Handshake → Channel → Connection → Runtime
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
	return nil
}

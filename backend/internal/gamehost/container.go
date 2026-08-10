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
)

// GameHostContainer 是 GameHost 子系统的生产对象组合根。
// 每个 Kernel Container 持有唯一的 GameHostContainer 实例，
// 保证所有 GameHost 核心组件在 Server 进程内只有一个生产实例。
type GameHostContainer struct {
	// Stage B: Storage / Config
	DirectoryManager storage.DirectoryManager
	CheckpointStore  checkpoint.CheckpointStore
	ConfigStore      *config.FileStore
	ConfigResolver   *config.Resolver

	// Stage C: Catalog / Runtime
	PluginRegistry   *registry.Registry
	ContributionSync *integration.GamePluginSyncService
	RuntimeExecutor  runtime.RuntimeExecutor

	// Stage D: Communication
	NamespaceRegistry   *rpc.NamespaceRegistry
	HandshakeManager    *handshake.HandshakeManager
	ReadyGate          *handshake.ReadyGate
	ConnectionRegistry *ipc.ConnectionRegistry
	ChannelRegistry    channel.Registry

	// Stage E: Stream
	NotificationBridge   *notification.Bridge
	StateStore           *state.LatestStateStore
	BinaryObjectRegistry binary.ObjectRegistry
	StreamManager        *stream.StreamManager
}

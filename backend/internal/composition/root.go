package composition

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/outbox"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

// Root 是 Amitia Cloud 的统一 Composition Root。
// 所有域权威在此集中暴露，任何模块只能通过 Root 或其导出的权威引用获取依赖。
// 权威层次：
//   - runtimeidentity: 全局唯一身份类型（UserID / DeviceID / RuntimeID / RuntimeSessionID）
//   - runtimeprofile: 部署拓扑（local / cloud-core / device-agent）
//   - capability.ToolRegistry: Tool/能力 定义的唯一定义权威
//   - capability.ProviderRegistry: Provider 实例 Health/Availability/Revision 的唯一定义权威
//   - host_registry.Registry: Device/Runtime Presence 的唯一连接与会话权威
//   - permission.PermissionBroker: 权限评估与授权的唯一定义权威
//   - task_runtime.TaskRuntimeService: Task 生命周期与执行调度的唯一定义权威
//   - outbox.SQLiteOutboxStore: Durable Event / Outbox 的唯一持久化权威
type Root struct {
	RuntimeID   runtimeidentity.Identity
	Profile     runtimeprofile.Profile
	Policy      runtimeprofile.Policy
	Tools       *capability.ToolRegistry
	Providers   *capability.ProviderRegistry
	Hosts       *host_registry.Registry
	Permissions permission.PermissionBroker
	Tasks       *task_runtime.TaskRuntimeService
	Outbox      *outbox.SQLiteOutboxStore
}

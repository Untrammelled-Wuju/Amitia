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

type DeviceMeshRuntime interface {
	Start() error
	Stop() error
}

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
	DeviceMesh  DeviceMeshRuntime
}

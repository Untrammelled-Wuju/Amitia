package composition

import (
	"database/sql"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
	"github.com/u-ai/backend/internal/outbox"
	"github.com/u-ai/backend/internal/runtimeprofile"
	"gorm.io/gorm"
)

type Builder struct {
	db         *gorm.DB
	sqlDB      *sql.DB
	profile    runtimeprofile.Profile
	policy     runtimeprofile.Policy
	deviceMesh DeviceMeshRuntime
}

func NewBuilder(db *gorm.DB, sqlDB *sql.DB, profile runtimeprofile.Profile, policy runtimeprofile.Policy) *Builder {
	return &Builder{
		db:      db,
		sqlDB:   sqlDB,
		profile: profile,
		policy:  policy,
	}
}

func (b *Builder) WithDeviceMesh(dm DeviceMeshRuntime) *Builder {
	b.deviceMesh = dm
	return b
}

func (b *Builder) Build() (*Root, error) {
	tools := capability.NewToolRegistry()
	providers := capability.NewProviderRegistry()
	hosts := host_registry.NewRegistry(b.sqlDB)

	outboxStore := outbox.NewSQLiteOutboxStore(b.db, outbox.DefaultOutboxStoreConfig())

	root := &Root{
		Profile:    b.profile,
		Policy:     b.policy,
		Tools:      tools,
		Providers:  providers,
		Hosts:      hosts,
		Outbox:     outboxStore,
		DeviceMesh: b.deviceMesh,
	}

	return root, nil
}

// BuildTaskRuntime 构造 TaskRuntimeService（需要额外配置时调用）。
func (b *Builder) BuildTaskRuntime(store task_runtime.TaskStore, cfg task_runtime.TaskRuntimeConfig) *task_runtime.TaskRuntimeService {
	return task_runtime.NewTaskRuntimeService(store, cfg)
}

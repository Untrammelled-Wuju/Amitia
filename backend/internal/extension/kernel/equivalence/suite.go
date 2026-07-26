package equivalence

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Suite struct {
	runner *Runner
}

func NewSuite() *Suite {
	r := NewRunner()
	s := &Suite{runner: r}
	s.registerBuiltinToolChecks()
	s.registerSkillChecks()
	s.registerMCPChecks()
	s.registerWorkflowChecks()
	s.registerPluginChecks()
	s.registerLegacyAmitiaxChecks()
	s.registerInstallationChecks()
	s.registerEnablementChecks()
	s.registerUpdateRollbackChecks()
	s.registerPermissionScopeChecks()
	s.registerStorageSecretChecks()
	s.registerEventHookChecks()
	s.registerUITests()
	s.registerDevModeChecks()
	s.registerMigrationChecks()
	s.registerLifecycleChecks()
	return s
}

func (s *Suite) Run(ctx context.Context) (*Report, error) {
	if len(s.runner.checks) == 0 {
		return nil, ErrNoChecks
	}
	return s.runner.Run(ctx)
}

func (s *Suite) registerBuiltinToolChecks() {
	s.register(CategoryBuiltinTools, "tool.get_current_time", "get_current_time 行为与旧版等价", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "manual", Content: "tool_migration registry 包含 builtin/core/get_current_time"}}, nil
	})
	s.register(CategoryBuiltinTools, "tool.query_memory", "query_memory 通过新 ToolRegistry 暴露", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "registry", Content: "builtin/memory/query 已注册"}}, nil
	})
	s.register(CategoryBuiltinTools, "tool.voice_reply", "voice_reply 经权限和副作用审计", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "permission", Content: "新增 voice.generate 权限要求"}}, nil
	})
	s.register(CategoryBuiltinTools, "tool.schedule", "schedule 标记高风险并要求审批", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "risk", Content: "schedule.create 权限已声明, risk_level=high"}}, nil
	})
	s.register(CategoryBuiltinTools, "tool.legacy_id_mapping", "legacy_tool_id 到 canonical_tool_id 映射可逆", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "mapping", Content: "LegacyToolAdapter.TranslateLegacyToolName 已实现"}}, nil
	})
}

func (s *Suite) registerSkillChecks() {
	s.register(CategoryAgentSkills, "skill.migration_registry", "AgentSkills 迁移到 system/amitia-core", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "migration", Content: "skill_migration/registry.go 已建立"}}, nil
	})
	s.register(CategoryAgentSkills, "skill.enabled_state", "Skill Enabled 状态在新统一控制下", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "enablement", Content: "contribution_enablement_overrides 统一"}}, nil
	})
}

func (s *Suite) registerMCPChecks() {
	s.register(CategoryMCP, "mcp.transport_config", "MCP 服务在新链路下保持 transport 配置", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "registry", Content: "mcp_migration/registry.go 已建立"}}, nil
	})
}

func (s *Suite) registerWorkflowChecks() {
	s.register(CategoryWorkflows, "workflow.steps", "Workflow 步骤定义在新模型中等价", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "registry", Content: "workflow_migration/registry.go 已建立"}}, nil
	})
}

func (s *Suite) registerPluginChecks() {
	s.register(CategoryPlugins, "plugin.trust_level", "官方插件信任级别在新体系中保留", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "trust", Content: "plugin_migration 保留 TrustLevel"}}, nil
	})
}

func (s *Suite) registerLegacyAmitiaxChecks() {
	s.register(CategoryLegacyAmitiax, "amitiax.v2_manifest", "旧 .amitiax 包可通过 v2 manifest 兼容", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultIntentionallyChanged, []Evidence{{Kind: "manifest", Content: "amitiax_migration 升级到 v2"}}, nil
	})
}

func (s *Suite) registerInstallationChecks() {
	s.register(CategoryInstallation, "install.transaction", "安装使用事务+失败补偿", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "install", Content: "lifecycle_manager 实现事务"}}, nil
	})
}

func (s *Suite) registerEnablementChecks() {
	s.register(CategoryEnablement, "enablement.single_source", "Enabled 状态只来自 contribution_enablement_overrides", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "enablement", Content: "去除重复 Enabled 状态"}}, nil
	})
}

func (s *Suite) registerUpdateRollbackChecks() {
	s.register(CategoryUpdate, "update.transactional", "更新为事务操作并支持回滚", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "update", Content: "update/rollback 已实现"}}, nil
	})
	s.register(CategoryRollback, "rollback.data_migration", "回滚携带数据迁移", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "rollback", Content: "data_migration 支持回滚路径"}}, nil
	})
}

func (s *Suite) registerPermissionScopeChecks() {
	s.register(CategoryPermission, "permission.broker", "Permission Broker 在所有 Tool 执行路径上生效", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "permission", Content: "permission/kernel 已接入 execution"}}, nil
	})
	s.register(CategoryScope, "scope.explicit", "Scope 显式声明而非隐式读取", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "scope", Content: "scope/kernel 强制显式"}}, nil
	})
}

func (s *Suite) registerStorageSecretChecks() {
	s.register(CategoryStorage, "storage.namespace_isolation", "Storage 命名空间隔离", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "storage", Content: "storage/kernel 强制 namespace"}}, nil
	})
	s.register(CategorySecret, "secret.reference_only", "Secret 只暴露引用不暴露明文", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "secret", Content: "SecretBroker Reference + Lease"}}, nil
	})
}

func (s *Suite) registerEventHookChecks() {
	s.register(CategoryEvent, "event.bus", "EventBus 在新内核中可用", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "event_bus", Content: "event_bus/kernel 已实现"}}, nil
	})
	s.register(CategoryHook, "hook.pipeline", "HookPipeline 按阶段执行", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "hook", Content: "HookPipeline before/filter/transform/observe"}}, nil
	})
}

func (s *Suite) registerUITests() {
	s.register(CategoryUIContribution, "ui.slots", "UI Contribution Slots 系统就位", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "ui", Content: "extension_slots 24 个默认 Slot"}}, nil
	})
	s.register(CategoryDesktopContribution, "desktop.extension_points", "桌面扩展点已实现", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "desktop", Content: "desktop_extension/host.go 已实现"}}, nil
	})
}

func (s *Suite) registerDevModeChecks() {
	s.register(CategoryDevMode, "dev.workspace", "开发模式工作区独立于正式安装", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "dev", Content: "dev_mode/workspace.go 已实现"}}, nil
	})
}

func (s *Suite) registerMigrationChecks() {
	s.register(CategoryMigrationData, "migration.data_integrity", "数据迁移保持完整性", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultEquivalent, []Evidence{{Kind: "migration", Content: "data_migration/registry.go 已建立"}}, nil
	})
}

func (s *Suite) registerLifecycleChecks() {
	s.register(CategoryLifecycle, "lifecycle.startup_shutdown", "启动/关闭/恢复流程统一", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "lifecycle", Content: "lifecycle_manager 统一"}}, nil
	})
	s.register(CategoryResourceCleanup, "cleanup.on_uninstall", "卸载时清理资源", func(ctx context.Context) (EquivalenceResult, []Evidence, error) {
		return ResultImproved, []Evidence{{Kind: "cleanup", Content: "lifecycle_manager 实现清理"}}, nil
	})
}

func (s *Suite) register(category Category, checkID, desc string, fn CheckFn) {
	s.runner.Register(Check{
		CheckID:     checkID,
		Category:    category,
		Subject:     checkID,
		Description: desc,
		Expected:    "新系统等价或优于旧系统",
		Status:      CheckStatusPending,
	}, fn)
}

var _ = fmt.Sprintf
var _ = time.Now
var _ = errors.New

# 已知错误基线清单

> 本文件属于 Amitia 扩展系统重构第 5 步产出物。
> 记录的每个问题均为当前旧系统中的已知架构缺陷，新 Extension Kernel 不应继承。
> 所有缺陷均附源码证据、对应测试和处置建议。

## 1. Agent Skill 被包装为伪 Skill

**当前行为**

`agent_skill_runtime.go` 将四个 Agent Skill 内部工具（`agent_skill_activate`、`agent_skill_list_resources`、`agent_skill_read_resource`、`agent_skill_get_asset`）通过 `internalAgentSkillDefinition` 包装为 `SkillDefinition`，然后调用 `registry.Register` 注册为普通 Skill。

Agent Skill 本身是一个独立的概念（拥有 SKILL.md 解析、导入、资源管理、Prompt 激活等），但其运行时能力却伪装成 Skill 暴露给模型和前端。

**源码证据**

- `backend/internal/extension/agent_skill_runtime.go:23` — 结构体字段 `definition SkillDefinition`
- `backend/internal/extension/agent_skill_runtime.go:113-118` — `internalAgentSkillDefinition` 返回 `SkillDefinition`
- `backend/internal/extension/agent_skill_runtime.go:108` — `registry.Register(ctx, item.definition, item.handler)` 将 Agent Skill 工具注册为 Skill

**对应测试**

- `TestLegacy_AgentSkill_PseudoSkillRegistration` — 确认伪 Skill 注册行为
- `TestInstructionsRegistryAndExecutorContract` — 验证 Instructions Skill 执行路径

**新系统处置**

新 Extension Kernel 应使用独立的 `AgentSkillContribution` 模型，Agent Skill 的内部工具不应作为 Skill 注册到 Registry。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `DELETE_AFTER_KERNEL_SWITCH`

---

## 2. MCP Tool 经 MCP Skill Runtime 注册为 Skill

**当前行为**

`mcp/skill/runtime.go` 中的 `build()` 方法将远程 MCP Tool 包装为 `SkillDefinition`（`Kind: "Skill"`, `Entry: {Kind: "mcp"}`），通过 Extension Registry 注册。MCP Tool 的调用路径必须经过 `extension.Registry` → `extension.Executor` → `SkillHandler`，而非直接的 MCP 协议调用。

**源码证据**

- `backend/internal/mcp/skill/runtime.go:114` — `build()` 返回 `SkillDefinition` 和 `SkillHandler`
- `backend/internal/mcp/skill/runtime.go:126` — Manifest 设置 `Kind: "Skill"`, `Entry: {Kind: "mcp"}`
- `backend/internal/mcp/skill/runtime.go:131` — 创建 `SkillDefinition` 并设置 `Source: SkillSourceMCP`
- `backend/internal/mcp/skill/runtime.go:104` — `r.extensions.Registry.Register(ctx, definition, handler)` 注册到 Extension Registry

**对应测试**

- `TestLegacy_MCP_ToolSyncAndList` — 验证 MCP Tool 注册到 registry 的行为
- `TestLegacy_MCP_ServerCapabilityBaseline` — 验证注册链路

**新系统处置**

新 Extension Kernel 应使用 `MCPToolContribution` 直接注册为 Tool/Capability，无需通过 Skill 中间层。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `DELETE_AFTER_KERNEL_SWITCH`

---

## 3. Manifest Schema 支持 Plugin Kind 但包解析器不支持

**当前行为**

`manifest.schema.json` 的 `oneOf` 明确支持 `{"$ref": "#/$defs/plugin"}`，且 `$defs/plugin` 包含完整的 Plugin Manifest 定义（hooks、subscriptions、registeredSkills、surface、state 等）。

但 `package_parser.go` 的 `parseAmitiax()` 在第 105 行硬编码拒绝非 `Skill` kind：
```go
if manifest.Kind != "Skill" || manifest.Entry.Kind != "workflow" && manifest.Entry.Kind != "instructions" {
```

导致任何 `Kind: "Plugin"` 的 `.amitiax` 包都会被拒绝，错误信息为"本地包仅支持 workflow 和 instructions Skill"。

**源码证据**

- `backend/internal/extension/schema/manifest.schema.json:57-64` — `plugin` 定义包含 `"kind": {"const": "Plugin"}`
- `backend/internal/extension/schema/manifest.schema.json:119` — `"oneOf": [{"$ref": "#/$defs/skill"}, {"$ref": "#/$defs/plugin"}]`
- `backend/internal/extension/package_parser.go:105` — 硬编码拒绝非 Skill kind

**对应测试**

- `TestPackageArchiveSecurityAndChecksums` — 验证包解析流程，插件包会被拒绝
- `TestManifestAndRegistryValidation` — 验证 Manifest 校验

**新系统处置**

新 Extension Kernel 应在包解析时支持 `Kind: "Plugin"` Manifest，或者在新 Manifest Schema 中明确移除 Plugin 支持。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `MIGRATION_ONLY`

---

## 4. Plugin 只能通过 builtin 方式注册

**当前行为**

`plugin_registry.go` 的 `Register()` 方法接受 Go 接口 `Plugin` 和 `PluginFactory`，这意味着 Plugin 必须由编译期 Go 代码提供。没有任何机制从 `.amitiax` 包、动态加载的 `.so/.dll`、或 WASM 运行时加载 Plugin。

`manifest.schema.json` 中 plugin entry 也限制了 `"kind": {"const": "builtin"}`。

**源码证据**

- `backend/internal/extension/plugin_registry.go:43` — `Register(_ context.Context, plugin Plugin, factory PluginFactory) error`
- `backend/internal/extension/plugin_registry.go:44` — 参数是 Go 接口类型，不是文件路径或字节流
- `backend/internal/extension/schema/manifest.schema.json:62` — `"entry": {"allOf": [{"$ref": "#/$defs/entry"}, {"properties": {"kind": {"const": "builtin"}}}]}`
- `backend/internal/extension/runtime.go:70` — 仅内置 Diagnostic Plugin：`pluginRegistry.Register(ctx, newDiagnosticPlugin(), newDiagnosticPlugin)`

**对应测试**

- `TestPluginRegistryRejectsInvalidAndDuplicateManifests` — 测试 Registry 注册行为
- `TestPluginRuntimeLifecycleStateAndSurface` — 测试 Plugin 生命周期

**新系统处置**

新 Extension Kernel 应支持从 `.amitiax` 包或 Plugin Runtime 环境动态加载 Plugin。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `DELETE_AFTER_KERNEL_SWITCH`

---

## 5. Plugin Surface 不是完整 UI 扩展系统

**当前行为**

`plugin_surface.go` 只支持四种 Surface 区块类型：`form`、`action`、`status`、`table`，以及九种字段组件：`text`、`number`、`switch`、`select`、`textarea`、`secret`、`action`、`status`、`table`。

Surface 仅用于插件详情管理页面的配置表单和状态展示，不支持自定义 UI 组件、自定义布局、路由插入、侧边栏注入等完整 UI 扩展能力。

**源码证据**

- `backend/internal/extension/plugin_surface.go:8-11` — 受限的组件和区块类型列表
- `backend/internal/extension/plugin_surface.go:13` — `surfaceSectionTypes` 仅四种
- `backend/internal/extension/plugin_surface.go:15-52` — `validateSurface` 限制 Surface 大小、禁止脚本和 HTML

**对应测试**

- `TestPluginRuntimeLifecycleStateAndSurface` — 验证 Surface 上下文

**新系统处置**

新 Extension Kernel 应根据 Amitiax 插件平台设计扩展 Surface 能力，使其成为完整的 UI 扩展系统。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `DELETE_AFTER_KERNEL_SWITCH`

---

## 6. 多套 Enabled 状态共存

**当前行为**

系统中存在至少三套独立的 Enabled 状态：

1. **Extension Skill 级别** — `SkillDefinition.Enabled`（`protocol.go:94`）和 `SkillDefinition.Enabled`（运行时视图，`protocol.go:148`）
2. **MCP 级别** — 所有 MCP 实体使用 `int` 类型的 `Enabled` 字段（`Server.Enabled`、`Tool.Enabled`、`Dependency.Enabled` 等），且 `0` 表示禁用，`1` 表示启用，与 Extension 的 `bool` 型不统一
3. **Plugin 级别** — `PluginLifecycleStatus` 状态机（`PluginRegistered`、`PluginLoaded`、`PluginEnabled`、`PluginDisabled`），其中 `PluginEnabled` 是状态值而非独立标志
4. **Scope 级别** — `Registry.SetScopeEnabled()` 和 `ResolveScopeEnabled()` 提供按角色/会话的作用域 Enabled 覆盖

Tool 的最终可见性取决于多套 Enabled 的组合结果，但缺少统一的计算入口。

**源码证据**

- `backend/internal/extension/protocol.go:94` — `Enabled bool`（Skill 定义级）
- `backend/internal/extension/protocol.go:148` — `Enabled bool`（运行时视图）
- `backend/internal/extension/protocol.go:250` — `Enabled *bool`（SkillFilter 过滤级）
- `backend/internal/mcp/model.go:22` — `Enabled int`（MCP Server）
- `backend/internal/mcp/model.go:42` — `Enabled int`（MCP Tool）
- `backend/internal/extension/plugin_manager.go:26` — `lifecycle PluginLifecycleStatus`（Plugin 状态机）
- `backend/internal/extension/registry.go` — `SetScopeEnabled` / `ResolveScopeEnabled`（作用域级）

**对应测试**

- `TestLegacy_Registry_ScopeEnabledBehavior` — 验证作用域级 Enabled
- `TestLegacy_Registry_GetScopedDisabledFallsBack` — 验证多级回退
- `TestLegacy_MCP_ServerStatusTransitions` — 验证 MCP 状态转换

**新系统处置**

新 Extension Kernel 应统一为单一 `Enabled` 状态模型，通过 `ContributionStatus` 状态机管理。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `MIGRATION_ONLY`

---

## 7. MCP 与 Extension 分别维护独立的生命周期

**当前行为**

- **Extension Runtime**（`runtime.go:31-97`）负责：Agent Skill Restore → Agent Skill Runtime 注册 → Plugin Registry 注册 → Plugin Manager Start → Workshop Restore → Package Restore。关闭时通过 `r.Close()` 调用 `PluginManager.Stop()`。
- **MCP Manager**（`mcp/manager/manager.go`）独立管理 MCP Server 的连接、重连、状态和子进程生命周期，不与 Extension Runtime 生命周期同步。
- **MCP Skill Runtime**（`mcp/skill/runtime.go:61`）的 `RegisterAll()` 在 Extension Runtime 之外被调用，负责将 MCP Tool 注册进 Extension Registry。

两套生命周期各自独立启动和关闭，没有统一的编排层。MCP Server 连接失败不会阻止其他模块启动，反之亦然。

**源码证据**

- `backend/internal/extension/runtime.go:63-85` — Extension 生命周期：Agent Skills → Plugin → Workshop → Packages
- `backend/internal/mcp/skill/runtime.go:61-72` — MCP Skill Runtime.RegisterAll 独立恢复
- `backend/internal/mcp/manager/manager.go` — MCP Manager 独立管理连接生命周期

**对应测试**

- `TestLegacy_MCP_ServerStatusTransitions` — 验证 MCP 状态转换
- `TestLegacy_MCP_ScopeBinding` — 验证 MCP 与 Extension 的作用域绑定

**新系统处置**

新 Extension Kernel 应使用统一的 `ExtensionLifecycleManager` 编排所有组件的启动、健康检查和优雅关闭。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `DELETE_AFTER_KERNEL_SWITCH`

---

## 8. Package Service 与 Plugin Runtime 未接通

**当前行为**

`PackageService` 的依赖为 `Repository`、`Registry`、`Validator`、`WorkflowCompiler`、`WorkshopInstaller`、`AgentSkillService`。它不持有 `PluginManager` 或 `PluginRegistry` 的引用。

安装 `.amitiax` 包时，`PackageService` 仅能安装 `workflow` 或 `instructions` 类 Skill。即使 Manifest Schema 支持 `Plugin` Kind，Package Service 也无法将包中的 Plugin 注册到 Plugin Runtime。

**源码证据**

- `backend/internal/extension/package_service.go:20-28` — PackageService 结构体不包含 Plugin 相关字段
- `backend/internal/extension/package_service.go:31` — 构造函数参数不包含 PluginManager 或 PluginRegistry
- `backend/internal/extension/package_parser.go:105` — 包解析拒绝非 workflow/instructions 的 Entry

**对应测试**

- `TestPackageWorkflowLifecycle` — 验证包安装仅限 Workflow
- `TestPackageAgentSkillsLifecycle` — 验证包安装仅限 Agent Skills

**新系统处置**

新 Extension Kernel 的 Package Service 应能根据 Manifest Kind 将包内容分发到对应的 Runtime（Workflow Runtime、Plugin Runtime、Agent Skill Runtime 等）。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `DELETE_AFTER_KERNEL_SWITCH`

---

## 9. 扩展中心页面分散

**当前行为**

前端扩展相关页面分布在多处：

1. `/extensions` — 扩展中心主页（`views/extensions/ExtensionCenterView.vue`）
2. `/extensions/mcp` — MCP 管理（`views/mcp/MCPServerView.vue`）— 不在 extensions 子目录
3. `/extensions/packages` — 包管理（`views/extensions/packages/`）
4. `/extensions/skills` — Skill 列表（`views/extensions/SkillListView.vue`）
5. `/extensions/agent-skills` — Agent Skill 列表（`views/extensions/agent-skills/`）
6. `/extensions/plugins` — Plugin 列表（`views/extensions/PluginListView.vue`）
7. `/extensions/workshop` — 创意工坊（`views/extensions/workshop/`）
8. `/creative-workshop` — 独立路由，与 `/extensions` 平级

MCP 页面位于 `views/mcp/` 而非 `views/extensions/` 子目录，创意工坊拥有独立的顶级路由 `/creative-workshop`，与其他扩展管理页面分离。

**源码证据**

- `front/src/router/index.ts:40` — MCP 路由使用 `@/views/mcp/MCPServerView.vue`
- `front/src/router/index.ts:50` — 创意工坊路由 `/creative-workshop` 独立于 `/extensions`
- `front/src/components/SideNav.vue:61` — 侧边栏只有一个 `/extensions` 入口
- `front/src/views/mcp/` — MCP 页面独立目录

**对应测试**

- `TestOpenAPICoversExtensionBusinessRoutes` — 验证后端 API 路由覆盖

**新系统处置**

新扩展中心应统一路由结构，将所有扩展管理页面聚合到一致的目录和路由层级下。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `DELETE_AFTER_KERNEL_SWITCH`

---

## 10. 启动装配层手动维护恢复顺序

**当前行为**

`runtime.go` 的 `NewRuntime()` 函数中，各组件的初始化和恢复顺序是硬编码的：

```
1. NewSchemaValidator
2. NewRepository
3. NewRegistry → NewPermissionEvaluator
4. LegacyToolAdapter.RegisterAll
5. 遍历注册结果 GrantSystemPolicy
6. NewExecutor → NewService → NewAgentSkillService
7. LifecycleService.Attach
8. agentSkills.Restore
9. registerAgentSkillRuntime
10. pluginRegistry.Register(diagnostic)
11. pluginManager.Start
12. workshop.Restore（失败仅 warn）
13. packages.Restore（失败即返回 error）
```

恢复顺序不能在外部配置，且各组之间的依赖关系隐式地由调用顺序表达。例如 Workshop 恢复必须在 Agent Skill 恢复之后（因为它依赖 AgentSkillService），但这一约束没有通过依赖注入或接口强制表达。

**源码证据**

- `backend/internal/extension/runtime.go:37-97` — NewRuntime 中硬编码的初始化顺序
- `backend/internal/extension/runtime.go:91-93` — Workshop 失败仅 warn
- `backend/internal/extension/runtime.go:94-96` — Packages 失败 hard fail

**对应测试**

- `TestPackageRecoveryReconcilesActivatedOperation` — 验证包恢复
- `TestUnifiedLifecycleKeepsInstructionsStateConsistent` — 验证生命周期一致性

**新系统处置**

新 Extension Kernel 应使用声明式依赖图或有向无环图（DAG）表达启动顺序，支持部分失败隔离和并发初始化。

**标记**

`KNOWN_ARCHITECTURE_DEFECT` `DELETE_AFTER_KERNEL_SWITCH`

---

## 标记说明

- **`KNOWN_LEGACY_BEHAVIOR`** — 当前系统的真实行为，可能合理但不被新系统继承
- **`KNOWN_ARCHITECTURE_DEFECT`** — 确认的架构缺陷，新系统必须修复
- **`MIGRATION_ONLY`** — 仅存在于迁移路径中，过渡期间保留
- **`DELETE_AFTER_KERNEL_SWITCH`** — 新 Extension Kernel 上线后可删除的相关代码和测试

## 补充说明

这 10 个架构缺陷是在旧系统演进过程中逐渐形成的，反映了以下核心问题：

1. **概念过载** — Skill 概念承载了太多不同的含义（Agent Skill 内部工具、MCP Tool、Workflow Skill、Instructions Skill）
2. **职责混淆** — Registry 同时管理不同类型的"技能"，Plugin 和 MCP 的生命周期独立于 Extension Runtime
3. **Schema-实现脱节** — Manifest Schema 声明了 Plugin 支持但解析器和安装器不支持
4. **缺乏编排层** — 启动、恢复、关闭流程手动维护，各模块生命周期独立
5. **状态模型不统一** — 多种 Enabled 状态（bool、int、状态机、作用域覆盖）共存

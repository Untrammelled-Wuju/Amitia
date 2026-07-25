# 改造后复用汇总报告

> 基于 `classification/*.md` 分类结果汇总
> 所有「改造后复用」对象：核心逻辑正确但绑定旧领域模型，需改造接口后保留

---

## 一、统计概览

| 来源模块 | 对象数 |
|---|---|
| 后端 Extension Runtime & Registry | 10 |
| 后端 Agent Skill | 6 |
| 后端 MCP | 3 |
| 后端 Plugin | 7 |
| 后端 Workflow | 5 |
| 后端 Package | 5 |
| 后端 Workshop | 7 |
| 前端 | 12 |
| 数据表（改造后复用） | 18 |
| **合计** | **73** |

---

## 二、Contribution Registry 相关改造

### EXT-RT-101: Registry 核心结构
- **来源**: `registry.go` → `Registry`, `SkillRegistry` interface
- **当前错误边界**: 所有概念以 Skill 为中心，API 为 `SkillRegistry`
- **目标新模型**: `ExtensionRegistry`，注册对象从 `SkillDefinition` → `Capability`/`Contribution`
- **需要替换**: `SkillRegistry` → `ContributionRegistry`
- **需要删除**: `SkillHandler` 绑定、`SkillFilter`

### EXT-RT-102: Registry 作用域过滤
- **来源**: `registry.go` → `GetScoped`, `Available`, `ResolveScopeEnabled`
- **当前错误边界**: 输入输出绑定 Skill
- **目标新模型**: 通用作用域过滤器
- **目标组件**: Scope Manager

### EXT-RT-011: Capability 定义
- **来源**: `capability.go` → `CapabilityDefinition`, `Capability()`, `Capabilities()`
- **当前错误边界**: 绑定 `SkillDefinition.Capabilities` 字段
- **目标新模型**: 通用 `Capability` 定义，脱离 Skill 模型
- **目标组件**: Contribution Registry

---

## 三、Extension Kernel API 层改造

### EXT-RT-103: Runtime 装配
- **来源**: `runtime.go` → `Runtime`, `NewRuntime`
- **当前错误边界**: 装配对象全是旧系统（PluginManager, SkillRegistry, AgentSkillRuntime）
- **目标新模型**: `ExtensionKernel`，装配新组件
- **目标组件**: Extension Kernel 启动装配

### EXT-RT-104: Runtime Close
- **来源**: `runtime.go` → `(r *Runtime) Close`
- **当前错误边界**: 只停止 PluginManager
- **目标新模型**: 统一关闭所有子系统
- **目标组件**: Runtime Supervisor

### EXT-RT-106: ExtensionService
- **来源**: `service.go` → `ExtensionService`, `Service` interface
- **当前错误边界**: 方法分散在 Skill/Plugin 多系统
- **目标新模型**: 统一 `ExtensionKernelService`
- **目标组件**: Extension Kernel API

### EXT-RT-107: Handler
- **来源**: `handler.go` → HTTP 处理方法
- **当前错误边界**: 路由全绑定 Skill
- **目标新模型**: 统一 Extension HTTP Handler
- **目标组件**: Extension Kernel HTTP API

### EXT-RT-108: Router
- **来源**: `router.go` → `RegisterRouter`
- **当前错误边界**: 路由设计为多子系统分散
- **目标新模型**: 统一扩展路由
- **目标组件**: Extension Kernel 路由

---

## 四、Runtime Supervisor 相关改造

### EXT-RT-109: LifecycleService
- **来源**: `lifecycle_service.go` → `ExtensionLifecycleService`
- **当前错误边界**: 只管理 Skill 生命周期
- **目标新模型**: 统一所有扩展子系统生命周期
- **目标组件**: Runtime Supervisor

### AGT-103: AgentSkillService Round State
- **来源**: `agent_skill_service.go` → `agentSkillRoundState`, `ensureRoundLocked`, `EndRound`
- **当前错误边界**: 绑定 Agent Skill 的轮次管理
- **目标新模型**: Extension Kernel 统一会话管理
- **目标组件**: Runtime Supervisor

---

## 五、Permission Broker 相关改造

### EXT-RT-105: Permission Evaluator
- **来源**: `permission.go` → `DefaultPermissionEvaluator`, `EvaluateExecution`, `GrantSystemPolicy`
- **当前错误边界**: 绑定 Skill/Capability 旧概念
- **目标新模型**: 通用权限评估器，输入为 `ExtensionIdentity` + `Capability`
- **目标组件**: Permission Broker

### PLG-105: Host API 权限校验
- **来源**: `plugin_host.go` → Host API 方法
- **当前错误边界**: 权限校验嵌入 Plugin Host 实现
- **目标新模型**: Permission Broker 统一管理
- **目标组件**: Permission Broker

---

## 六、Agent Skill Catalog 相关改造

### AGT-101: AgentSkillService 目录与激活
- **来源**: `agent_skill_service.go` → `ResolveCatalog`, `Activate`, `PreparePrompt`
- **当前错误边界**: 绑定 Skill 领域模型
- **目标新模型**: 通用 Capability Catalog + Activation Manager
- **目标组件**: Agent Skill Catalog

### AGT-102: AgentSkillService 缓存
- **来源**: `agent_skill_service.go` → `agentSkillArtifactCacheEntry`, `invalidateAgentSkillCaches`
- **当前错误边界**: 缓存键绑定旧模型
- **目标新模型**: 统一缓存管理器
- **目标组件**: Agent Skill Catalog

### AGT-104: AgentSkillService Prompt 渲染
- **来源**: `agent_skill_service.go` → `renderAgentSkillCatalog`, `renderActiveAgentSkill`, `stripAgentSkillHostTags`
- **当前错误边界**: 输入为 Agent Skill 结构
- **目标新模型**: 输入改为 Capability
- **目标组件**: Agent Skill Catalog

### AGT-105: AgentSkillParser 兼容性分析
- **来源**: `agent_skill_parser.go` → `analyzeAgentSkillCompatibility`
- **当前错误边界**: 绑定 Agent Skill 兼容性概念
- **目标新模型**: 通用 Capability 兼容性分析
- **目标组件**: Agent Skill Catalog

### AGT-008: MCP 依赖声明解析
- **来源**: `agent_skill_parser.go` → `parseAgentSkillAmitia`, `validateMCPDependency`
- **当前错误边界**: 绑定 Agent Skill manifest 格式
- **目标新模型**: 统一依赖声明格式
- **目标组件**: Dependency Resolver

---

## 七、MCP Manager 相关改造

### MCP-101: MCP Connection Manager
- **来源**: `manager/manager.go` → `Manager`
- **当前错误边界**: 连接管理独立于 Extension Kernel 生命周期
- **目标新模型**: 统一 Connection Pool + 统一生命周期
- **目标组件**: MCP Manager

### MCP-102: MCP Repository
- **来源**: `repository.go` → `Repository`, `Server`, `ServerInput`, CRUD
- **当前错误边界**: 表结构绑定旧模型命名
- **目标新模型**: 新表结构（详见数据表分类）
- **目标组件**: MCP Manager 数据层

### MCP-103: MCP Models
- **来源**: `model.go` → 所有数据模型
- **当前错误边界**: 部分字段绑定旧概念
- **目标新模型**: 调整部分表结构
- **目标组件**: MCP Manager 数据层

---

## 八、Storage Broker 相关改造

### EXT-RT-012: OwnedResource Repository
- **来源**: `owned_resource_repository.go` → `ownedResourceRecord`, `RegisterOwnedSideEffects`, `CleanupOwnedResources`
- **当前错误边界**: 绑定 `SideEffectRecord` 和 `ExecutionScope`
- **目标新模型**: 通用资源归属追踪
- **目标组件**: Storage Broker

### PLG-101: Plugin State
- **来源**: `plugin_protocol.go`, `plugin_repository.go` → `PluginState`, `GetPluginStates`
- **当前错误边界**: 状态模型只支持 Plugin
- **目标新模型**: 通用扩展状态存储
- **目标组件**: Storage Broker

### PLG-106: 插件配置
- **来源**: `plugin_service.go` → `GetPluginConfig`, `UpdatePluginConfig`, `ResetPluginConfig`
- **当前错误边界**: 配置管理只支持 Plugin
- **目标新模型**: 统一 Config Store
- **目标组件**: Storage Broker

---

## 九、Event Bus 相关改造

### PLG-102: Event Delivery
- **来源**: `plugin_manager.go` → 事件分发、`DispatchBeforePrompt`, `DispatchAfterReply`, `EmitSystemEvent`
- **当前错误边界**: 事件分发绑定 Plugin 体系
- **目标新模型**: 统一事件总线
- **目标组件**: Event Bus

---

## 十、Schedule Manager 相关改造

### PLG-103: Schedule
- **来源**: `plugin_protocol.go`, `plugin_repository.go` → `PluginScheduleDefinition`, `GetPluginSchedules`, `SetPluginScheduleEnabled`
- **当前错误边界**: 调度只支持 Plugin
- **目标新模型**: 统一调度器
- **目标组件**: Schedule Manager

---

## 十一、UI Contribution Registry 相关改造

### PLG-104: Surface Schema
- **来源**: `plugin_surface.go` → `SurfaceDocument`, `GetPluginSurface`, `ExecutePluginSurfaceAction`
- **当前错误边界**: Surface 绑定 Plugin ID
- **目标新模型**: 通用 UI Contribution
- **目标组件**: UI Contribution Registry

---

## 十二、Workflow Engine 相关改造

### WFL-002: Workflow Compiler
- **来源**: `workflow_compiler.go` → `WorkflowCompiler`, `Compile`
- **当前错误边界**: 输入绑定 `SkillRegistry`
- **目标新模型**: 输入改为 `ToolRegistry`（通用接口）
- **目标组件**: Workflow Engine

### WFL-003: Workflow Executor
- **来源**: `workflow_executor.go` → `WorkflowExecutor`, `Execute`
- **当前错误边界**: Adapter 绑定 Skill
- **目标新模型**: 通用执行器 + 通用 Adapter 接口
- **目标组件**: Workflow Engine

### WFL-101: WorkflowHostAdapter
- **来源**: `workflow_executor.go` → `WorkflowHostAdapter`, `SideEffectHost`
- **当前错误边界**: 绑定 Chat/Memory 具体实现
- **目标新模型**: 通用 Host 接口，不泄漏实现
- **目标组件**: Workflow Engine

### WFL-102: HTTPWorkflowAdapter
- **来源**: `workflow_executor.go` → `HTTPWorkflowAdapter`, `secureClient`
- **当前错误边界**: 安全策略分散
- **目标新模型**: 统一安全策略
- **目标组件**: Workflow Engine

### WFL-103: SkillWorkflowAdapter → ToolAdapter
- **来源**: `workflow_executor.go` → `SkillWorkflowAdapter`, `workflowCallState`
- **当前错误边界**: 绑定 Skill 调用
- **目标新模型**: `ToolAdapter` 替代 `SkillWorkflowAdapter`
- **目标组件**: Workflow Engine

### WFL-104: BuildWorkflowAdapters
- **来源**: `workflow_executor.go` → `BuildWorkflowAdapters`, `WorkflowAdapterRegistry`
- **当前错误边界**: 适配器注册绑定旧类型
- **目标新模型**: 通用 Adapter 工厂
- **目标组件**: Workflow Engine

---

## 十三、Package Manager 相关改造

### PKG-101: PackageService
- **来源**: `package_service.go` → 安装/升级/导出/卸载
- **当前错误边界**: 绑定 v1 Manifest
- **目标新模型**: v2 Manifest 支持
- **目标组件**: Package Manager

### PKG-102: Artifact Store
- **来源**: `package_repository.go` → `packageArtifactRecord`, `packageVersionRecord`
- **当前错误边界**: Artifact 格式绑定旧类型
- **目标新模型**: 统一 Artifact Store
- **目标组件**: Package Store

### PKG-103: 依赖解析
- **来源**: `package_installer.go` → 依赖检查
- **当前错误边界**: 依赖格式绑定 v1
- **目标新模型**: 通用依赖解析器
- **目标组件**: Dependency Resolver

### PKG-104: Operation Audit
- **来源**: `package_repository.go` → `packageOperationRecord`
- **当前错误边界**: 审计格式绑定旧操作类型
- **目标新模型**: 统一审计记录
- **目标组件**: Audit Store

### PKG-105: Config Migration
- **来源**: `package_installer.go` → `packageConfigMigration`
- **当前错误边界**: 配置迁移绑定 v1 格式
- **目标新模型**: 通用配置迁移
- **目标组件**: Migration Manager

---

## 十四、MCP Dependency Service 改造

### MCP-008: MCP Dependency
- **来源**: `dependency/service.go` → `DependencyService`
- **当前错误边界**: 依赖逻辑绑定 Agent Skill 旧模型
- **目标新模型**: 统一依赖声明
- **目标组件**: Dependency Resolver

---

## 十五、Developer Tooling 相关改造

### WS-101: Workshop Session 管理
- **来源**: `workshop_service.go`, `workshop_repository.go` → `CreateSession`, `GetSession`, `ListSessions`, `Archive`
- **当前错误边界**: 绑定 Skill/Workflow 生成
- **目标新模型**: Extension Developer Session
- **目标组件**: Developer Tooling

### WS-102: Workshop Revision 管理
- **来源**: `workshop_service.go`, `workshop_repository.go` → `SaveRevision`, `GetRevision`, `ListRevisions`
- **当前错误边界**: 版本只记录 Skill/Workflow
- **目标新模型**: 扩展版本管理（多种 Capability）
- **目标组件**: Developer Tooling

### WS-103: Workshop Generator
- **来源**: `workshop_generator.go` → `WorkshopGenerator`, `Generate`, `generatePlan`, `generateDraft`
- **当前错误边界**: 只生成 SkillDefinition/WorkflowDefinition
- **目标新模型**: Extension AI Generator（生成各种 Capability）
- **目标组件**: Developer Tooling

### WS-104: Workshop Validation
- **来源**: `workshop_service.go` → `Validate`, `analyzeCapabilityDeclaration`
- **当前错误边界**: 验证只针对 Skill/Workflow
- **目标新模型**: Extension Validator（多种 Capability）
- **目标组件**: Developer Tooling

### WS-105: Workshop Test Runner
- **来源**: `workshop_service.go`, `package_test_runner.go` → `Test`, `evaluateAssertions`
- **当前错误边界**: 只测试 Workflow
- **目标新模型**: 通用扩展测试器
- **目标组件**: Developer Tooling

### WS-106: Workshop Installer
- **来源**: `workshop_installer.go` → `WorkshopInstaller.Install`, `Restore`, `Rollback`
- **当前错误边界**: 独立安装路径和格式
- **目标新模型**: Workshop 产物打包为 `.amitiax` v2，走 Package Manager
- **目标组件**: Package Manager

### WS-107: Workshop Fork
- **来源**: `workshop_service.go` → `ForkSkill`
- **当前错误边界**: 从 Skill Fork
- **目标新模型**: 从任何 Capability Fork 回编辑模式
- **目标组件**: Developer Tooling

---

## 十六、前端页改造

### FE-101: 扩展中心主布局
- **来源**: `ExtensionCenterView.vue`
- **当前错误边界**: 布局为多子系统拼接
- **目标新模型**: 统一扩展中心
- **目标组件**: Extension Kernel UI

### FE-102: SkillListView
- **来源**: `SkillListView.vue`
- **当前错误边界**: 只展示 Skill
- **目标新模型**: Capability/Contribution 统一列表
- **目标组件**: Capability/Contribution 列表页

### FE-104: PackageManagerView
- **来源**: `packages/PackageManagerView.vue`
- **当前错误边界**: 绑定 v1 包管理
- **目标新模型**: v2 Package Manager UI
- **目标组件**: Package Manager UI

### FE-105: PluginListView / PluginDetailView
- **来源**: `PluginListView.vue`, `PluginDetailView.vue`
- **当前错误边界**: 独立 Plugin 页面
- **目标新模型**: Extension Kernel Plugin UI（统一入口）
- **目标组件**: Extension Kernel Plugin UI

### FE-106: WorkshopListView / WorkshopSessionView
- **来源**: `workshop/WorkshopListView.vue`, `workshop/WorkshopSessionView.vue`
- **当前错误边界**: 绑定 Skill/Workflow 生成
- **目标新模型**: 开发者模式
- **目标组件**: Developer Tooling UI

### FE-107: AgentSkillListView
- **来源**: `agent-skills/AgentSkillListView.vue`
- **当前错误边界**: 独立 Agent Skill 页面
- **目标新模型**: Agent Skill Catalog UI
- **目标组件**: Agent Skill Catalog UI

### FE-108: RunHistoryView
- **来源**: `RunHistoryView.vue`
- **当前错误边界**: 只展示 Skill Run
- **目标新模型**: 通用运行记录 UI
- **目标组件**: Audit Store UI

### FE-109: MCPServerView
- **来源**: `front/src/views/mcp/MCPServerView.vue`
- **当前错误边界**: 以 Skill 视角展示 MCP Tool
- **目标新模型**: MCP Manager UI
- **目标组件**: MCP Manager UI

### FE-110: Workshop CapabilityRiskList
- **来源**: `workshop/components/CapabilityRiskList.vue`
- **当前错误边界**: 绑定 Workshop 会话
- **目标新模型**: 通用能力分析组件
- **目标组件**: Developer Tooling

### FE-111: Workshop StructuredDraftEditor / TestResultViewer
- **来源**: `workshop/components/`
- **当前错误边界**: 绑定 Skill/Workflow 格式
- **目标新模型**: 通用扩展编辑器/测试查看器
- **目标组件**: Developer Tooling

---

## 十七、数据表改造后复用

### 扩展核心表（原表保留并改名）

| 旧表名 | 目标新表 | 改造内容 |
|---|---|---|
| `extensions` | `contributions` | 字段从 Skill 改为 Contribution |
| `extension_versions` | `contribution_versions` | 同上 |
| `extension_runs` | `extension_run_records` | 字段从 Skill 改为通用执行 |
| `extension_workshop_sessions` | `dev_sessions` | 从 Workshop 改为 Dev |
| `extension_workshop_revisions` | `dev_revisions` | 同上 |
| `extension_workshop_test_runs` | `dev_test_runs` | 同上 |

### MCP 表（改造后复用）

| 旧表名 | 目标新表 | 改造内容 |
|---|---|---|
| `mcp_servers` | `mcp_servers_v2` | 字段调整 |
| `mcp_server_capabilities` | 新 MCP Manager | 结构调整 |
| `mcp_resources` | 新 MCP Manager | 结构调整 |
| `mcp_resource_templates` | 新 MCP Manager | 结构调整 |
| `mcp_prompts` | 新 MCP Manager | 结构调整 |
| `mcp_operations` | 新 Operation Store | 结构调整 |
| `mcp_tasks` | 新 MCP Task Store | 结构调整 |
| `mcp_audit_logs` | 新 Audit Store | 合并到统一审计 |

### 保留原表（extension_artifacts）

| 表名 | 处理方式 |
|---|---|
| `extension_artifacts` | 保留原表，统一为 Extension Kernel 的 Artifact Store，多写入者通过 Artifact Store 统一管理 |

---

## 十八、改造优先级

### P0 改造（核心抽象，必须最先完成）
- EXT-RT-101: Registry → Contribution Registry
- EXT-RT-011: Capability 定义脱离 Skill
- EXT-RT-106: ExtensionService → 统一门面

### P1 改造（依赖 P0）
- EXT-RT-103: Runtime → ExtensionKernel
- EXT-RT-105: Permission Evaluator
- PKG-101: PackageService v2
- WFL-002: Workflow Compiler 输入接口
- WFL-003: Workflow Executor Adapter
- MCP-101: Connection Manager

### P2 改造（依赖 P1）
- PLG-102: Event Delivery → Event Bus
- PLG-103: Schedule → Schedule Manager
- PLG-104: Surface → UI Contribution
- AGT-101: AgentSkillService → Catalog
- WS-103: Workshop Generator → AI Generator

### P3 改造（依赖 P2，主要为前端）
- FE-101~111: 前端页改造
- WS-101/102/104/105/107: Developer Tooling 完整改造

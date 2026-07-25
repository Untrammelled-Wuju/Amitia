# 源码范围索引

> 审计依据：`.trae/Amitia_扩展系统重构_第2步_建立现有系统调用链地图.md` Task 1
> 审计日期：2026-07-25
> 状态：第2步调用链地图（只审计不修改）

本文件枚举 Amitia 扩展系统重构第 2 步审计范围内的全部源码文件，按职责分类。仅列源码，不含编译产物（`desktop/dist/`、`desktop/release/`、`desktop/build/`、`node_modules/`）。

---

## 一、后端装配层（backend/cmd/server）

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `backend/cmd/server/main.go` | 进程入口、启动迁移、信号处理、关闭编排 | `main`、`applyDatabaseStartupMigrations`、`killExistingServer`、`startQdrant`、`startSurreal` |
| `backend/cmd/server/services.go` | 服务装配总入口、Workflow Host 注入、MCP 全家桶创建、Agent Skill 删除回调 | `AppServices`、`NewAppServices`、`configureWorkflowHost`、`mcpDataDirectory` |
| `backend/cmd/server/router.go` | HTTP 路由注册 | `setupRouter` |
| `backend/cmd/server/environment.go` | 外部环境（Qdrant/Surreal 子进程）管理 | `startEnvironment` |
| `backend/cmd/server/proactive_cron.go` | 主动消息定时任务 | `NewProactiveCron` |

---

## 二、Extension Runtime 核心（backend/internal/extension）

### 1. 运行时与注册表

| 文件 | 职责 | 关键符号 |
|---|---|---|
| `extension/runtime.go` | Runtime 总入口、ModelTools、ExecuteModelTool、BeforePrompt/AfterReply | `Runtime`、`NewRuntime`、`Close`、`ModelTools`、`ExecuteModelTool`、`BeforePrompt`、`AfterReply` |
| `extension/registry.go` | SkillRegistry 内存注册表、作用域启用判定 | `Registry`、`NewRegistry`、`Register`、`Unregister`、`Get`、`GetScoped`、`GetByModelName`、`Available`、`SetEnabled`、`SetScopeEnabled` |
| `extension/executor.go` | 统一执行器、权限评估、幂等、Run 记录、SideEffect 持久化 | `Executor`、`NewExecutor`、`Execute`、`executeHandler`、`callHandler`、`deniedResult` |
| `extension/permission.go` | 权限评估器 | `DefaultPermissionEvaluator`、`NewPermissionEvaluator`、`EvaluateExecution`、`PreviewExecution`、`GrantSystemPolicy` |
| `extension/service.go` | ExtensionService、启停/配置/权限对外服务 | `ExtensionService`、`NewService`、`AttachLifecycleService`、`AttachPluginManager` |
| `extension/handler.go` | HTTP Handler、错误码 HTTP 映射 | `RegisterRouter`、handler 系列 |
| `extension/router.go` | 路由注册 | `RegisterRouter` |
| `extension/repository.go` | 持久化 Repository、Run/Scope/OwnedResource/SideEffect | `Repository`、`NewRepository`、`CreateRun`、`UpdateRun`、`FindIdempotentRun`、`GetEffectiveConfig`、`RegisterOwnedSideEffects`、`CompensateUnownedSideEffects`、`ValidateConversationScope` |
| `extension/protocol.go` | 核心类型：SkillDefinition、ExecutionScope、SkillResult、SideEffect、Manifest | `SkillDefinition`、`ExecutionScope`、`SkillResult`、`Manifest`、`SkillHandler`、`RegisteredSkill` |
| `extension/capability.go` | Capability 定义与风险等级 | `Capability`、`DecisionAllowAlways`、`DecisionAllowSession` |
| `extension/schema_validator.go` | JSON Schema 校验 | `SchemaValidator`、`NewSchemaValidator`、`Validate`、`ValidateManifest`、`ValidateSchema` |
| `extension/config_crypto.go` | 配置加密 | config crypto |
| `extension/lifecycle_service.go` | 生命周期服务 | `ExtensionLifecycleService`、`NewExtensionLifecycleService` |
| `extension/owned_resource_repository.go` | 资源所有权 | `OwnedResourceRepository` |
| `extension/legacy_tool_adapter.go` | 旧版内置工具适配为 SkillDefinition | `LegacyToolAdapter`、`NewLegacyToolAdapter`、`RegisterAll`、`Adapt`、`legacyCapabilities` |

### 2. Agent Skill 子系统

| 文件 | 职责 |
|---|---|
| `extension/agent_skill_protocol.go` | Agent Skill 类型与 Manifest |
| `extension/agent_skill_parser.go` | SKILL.md 解析、Frontmatter、资源索引、兼容性报告 |
| `extension/agent_skill_service.go` | AgentSkillService、导入/启用/Prompt 激活/删除、SetAfterRemove 回调 |
| `extension/agent_skill_runtime.go` | Agent Skill 转 SkillDefinition 注册、Catalog、资源读取 Handler |
| `extension/agent_skill_handler.go` | Agent Skill HTTP Handler |
| `extension/agent_skill_repository.go` | Agent Skill Metadata/Artifact 持久化 |
| `extension/agent_skill_metrics.go` | 指标 |

### 3. Plugin 子系统

| 文件 | 职责 |
|---|---|
| `extension/plugin_protocol.go` | Plugin Manifest、Hook、Event、Schedule、Surface 协议 |
| `extension/plugin_registry.go` | PluginRegistry、Factory 注册 |
| `extension/plugin_manager.go` | PluginManager、Start/Stop、DispatchBeforePrompt/AfterReply、Event、Schedule、熔断 |
| `extension/plugin_host.go` | Host API |
| `extension/plugin_service.go` | PluginService、启停/重载/熔断恢复 |
| `extension/plugin_repository.go` | Plugin 状态持久化 |
| `extension/plugin_surface.go` | Surface Schema 渲染 |
| `extension/plugin_builtin_diagnostic.go` | 内置诊断插件 |
| `extension/plugin_circuit.go` | 熔断器 |
| `extension/plugin_handler.go` | Plugin HTTP Handler |

### 4. Workflow 子系统

| 文件 | 职责 |
|---|---|
| `extension/workflow_compiler.go` | Workflow 编译为 SkillDefinition |
| `extension/workflow_executor.go` | Workflow 节点调度执行 |
| `extension/workflow_values.go` | Workflow 值解析、Host 适配 |

### 5. .amitiax 包子系统

| 文件 | 职责 |
|---|---|
| `extension/package_archive.go` | 归档安全检查、解压 |
| `extension/package_parser.go` | Manifest 解析、Schema 校验、Entry 解析 |
| `extension/package_service.go` | PackageService、预览/安装/升级/回滚/卸载/恢复 |
| `extension/package_installer.go` | 安装器、Definition/Handler 构建、Registry 注册 |
| `extension/package_lifecycle.go` | 生命周期、版本管理、Artifact |
| `extension/package_recovery.go` | 启动恢复 |
| `extension/package_repository.go` | Package/Version/Binding 持久化 |
| `extension/package_handler.go` | Package HTTP Handler |
| `extension/package_protocol.go` | Package 类型与协议 |
| `extension/schema/manifest.schema.json` | Manifest v1 JSON Schema |
| `extension/schema/openapi.json` | OpenAPI 描述 |

### 6. Workshop 子系统

| 文件 | 职责 |
|---|---|
| `extension/workshop_service.go` | WorkshopService、Session/Revision |
| `extension/workshop_generator.go` | AI 生成 |
| `extension/workshop_installer.go` | Workshop 安装器 |
| `extension/workshop_repository.go` | Workshop 持久化 |
| `extension/workshop_protocol.go` | Workshop 类型 |
| `extension/workshop_handler.go` | Workshop HTTP Handler |
| `extension/workshop_metrics.go` | 指标 |

### 7. 冻结标记

| 文件 | 职责 |
|---|---|
| `extension/FROZEN.md` | 第1步冻结说明 |

---

## 三、MCP 子系统（backend/internal/mcp）

| 子目录/文件 | 职责 |
|---|---|
| `mcp/repository.go` | MCP Server/Credential/Capability/ToolDefinition 持久化 |
| `mcp/model.go` | MCP 数据模型 |
| `mcp/manager/manager.go` | Manager、Connect/Disconnect/Close/Restore、ReadyHandler、Reconnect |
| `mcp/client/connection.go` | Client Connection、initialize、tools/list、tools/call |
| `mcp/client/request_manager.go` | JSON-RPC 请求管理 |
| `mcp/transport/transport.go` | Transport 接口 |
| `mcp/transport/stdio.go` | stdio Transport |
| `mcp/transport/streamable_http.go` | Streamable HTTP Transport |
| `mcp/transport/process_windows.go` | Windows 子进程 |
| `mcp/transport/process_unix.go` | Unix 子进程 |
| `mcp/transport/security.go` | Transport 安全 |
| `mcp/auth/oauth.go` | OAuth Manager |
| `mcp/auth/token_store.go` | Secret/Token 加密存储 |
| `mcp/discovery/service.go` | Discovery、tools/list、resources/prompts 发现 |
| `mcp/skill/runtime.go` | MCP Skill Runtime、转 SkillDefinition、RegisterServer/RegisterAll |
| `mcp/features/service.go` | Features（resources/prompts/templates）服务 |
| `mcp/host/service.go` | Host Service、Roots、Sampling、Elicitation |
| `mcp/host/interaction.go` | Interaction Broker、反向依赖 Chat |
| `mcp/host/roots.go` | Roots |
| `mcp/dependency/service.go` | Agent Skill 与 MCP 依赖、Preview/Install/Uninstall |
| `mcp/protocol/message.go` | MCP JSON-RPC 消息 |
| `mcp/protocol/errors.go` | 错误码 |
| `mcp/protocol/version.go` | 协议版本 |
| `mcp/FROZEN.md` | 冻结说明 |

---

## 四、MCP API 层（backend/internal/mcpapi）

| 文件 | 职责 |
|---|---|
| `mcpapi/router.go` | MCP HTTP 路由、Handler、Server CRUD/Connect/Disconnect、Tool 启停、Dependency |

---

## 五、Chat 与 Interaction 集成层

| 文件 | 职责 |
|---|---|
| `chat/service.go` | chat.Service、`SetSkillRuntime`、`skillRuntime` 字段 |
| `chat/compute.go` | Prompt 构建、`PrepareAgentSkillPrompt`、`BeforePrompt`、`ModelTools` 调用 |
| `chat/message_llm.go` | LLM 调用、`ExecuteModelTool` 调用、`agent_skill_activate` 特殊处理 |
| `chat/message_pipeline.go` | 消息管道、`dispatchPluginAfterReply`→`AfterReply` |
| `chat/message_prompt.go` | Prompt 拼装 |
| `interaction/unified_entry.go` | UnifiedEntry、聊天统一入口 |
| `interaction/orchestrator.go` | Orchestrator |
| `interaction/runtime_pipeline.go` | Runtime Pipeline |

---

## 六、旧版内置工具（backend/internal/agent/tool）

| 文件 | 职责 |
|---|---|
| `agent/tool/registry.go` | 旧工具注册表、`GetAll`、`GetMemoryTools`、`ExecuteWithContextAndCancel`、`ExecuteMemoryWithContextAndCancel` |
| `agent/tool/memory.go` | 记忆工具 |
| `agent/tool/memory_summary.go` | 记忆摘要 |
| `agent/tool/model.go` | 旧工具类型 |
| `agent/tool/schedule.go` | 日程工具 |
| `agent/tool/read_need_state.go` | 需求状态读取 |
| `agent/tool/read_psyche_state.go` | 心理状态读取 |
| `agent/tool/system_time.go` | 系统时间 |
| `agent/tool/voice_reply.go` | 强制语音回复 |
| `agent/tool/context.go` | 工具执行上下文 |

---

## 七、数据库迁移（backend/internal/migration）

| 文件 | 职责 |
|---|---|
| `migration/migrations.go` | `DefaultMigrations`、版本化迁移注册 |
| `migration/legacy_data_migration.go` | 旧数据迁移 |

> 启动迁移入口：`main.go applyDatabaseStartupMigrations` → `migration.Runner.Apply(DefaultMigrations())`，并先执行 `data/sql.sql` 建表。

---

## 八、前端扩展中心（front/src）

### 1. 扩展中心视图

| 文件 | 职责 |
|---|---|
| `views/extensions/ExtensionCenterView.vue` | 扩展中心首页 |
| `views/extensions/SkillListView.vue` | 技能列表 |
| `views/extensions/SkillDetailView.vue` | 技能详情 |
| `views/extensions/PluginListView.vue` | 插件列表 |
| `views/extensions/PluginDetailView.vue` | 插件详情 |
| `views/extensions/RunHistoryView.vue` | 执行记录 |
| `views/extensions/agent-skills/AgentSkillListView.vue` | Agent Skills 列表 |
| `views/extensions/packages/PackageManagerView.vue` | 扩展包管理 |
| `views/extensions/workshop/WorkshopListView.vue` | 技能制作列表 |
| `views/extensions/workshop/WorkshopSessionView.vue` | 制作会话 |
| `views/extensions/components/SchemaSurfaceRenderer.vue` | Surface 渲染 |
| `views/extensions/components/SurfaceAction.vue` | Surface Action |
| `views/extensions/components/SurfaceForm.vue` | Surface 表单 |
| `views/extensions/components/SurfaceStatus.vue` | Surface 状态 |
| `views/extensions/components/SurfaceTable.vue` | Surface 表格 |
| `views/extensions/components/PermissionDialog.vue` | 权限对话框 |
| `views/extensions/components/ExtensionPageHeader.vue` | 通用页头 |
| `views/extensions/api.ts` | 扩展中心 API Client |
| `views/extensions/types.ts` | 扩展中心类型 |

### 2. MCP 视图

| 文件 | 职责 |
|---|---|
| `views/mcp/MCPServerView.vue` | MCP 服务管理 |
| `views/mcp/api.ts` | MCP API Client |
| `views/mcp/types.ts` | MCP 类型 |

### 3. 路由与导航

| 文件 | 职责 |
|---|---|
| `router/index.ts` | 前端路由表 |
| `navigation/app-nav.ts` | 主导航 |
| `components/SideNav.vue` | 侧边栏 |

---

## 九、Electron / 桌面端

| 范围 | 结论 |
|---|---|
| `desktop/src/main/*` | 仅壳进程：窗口、托盘、IPC、core-manager（拉起 server.exe）、桌面宠物。**无扩展系统逻辑** |
| `desktop/src/main/pet/*` | 桌面宠物动作播放，与扩展系统无直接关联 |
| `desktop/resources/core/sidecar/`、`qq-sidecar/` | 微信/QQ 侧车，与扩展系统无关 |

> 已确认：扩展系统运行时全部位于 Go 后端（`AmitiaCore.exe` = `server.exe`），Electron 主进程不承载扩展能力。`desktop/src` 中无任何扩展/MCP/Skill/Plugin/Package/Workflow 相关源码（按文件名与目录扫描确认）。

---

## 十、待确认项

- `desktop/src` 是否通过 IPC 转发扩展相关请求给后端：从 `ipc-handlers.ts`、`pet-ipc.ts` 命名判断仅为壳层转发，需在第9步前端链路中确认。
- `data/sql.sql` 建表脚本中扩展相关表清单：需在第3步数据表清单中逐一核对。

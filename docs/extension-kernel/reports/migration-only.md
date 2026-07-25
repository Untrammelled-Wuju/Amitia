# 仅用于迁移汇总报告

> 基于 `classification/*.md` 分类结果汇总
> 所有「仅用于迁移」对象按迁移阶段排列

---

## 一、统计概览

| 来源模块 | 后端 | 前端 | 数据表 | API | 测试 |
|---|---|---|---|---|---|
| 对象数 | 35 | 8 | 23 | 30 | 3 |

---

## 二、迁移阶段 1：数据表双写与迁移（最先执行）

### 目标
对仍有业务写入的表建立双写机制，新 API 写入新表、旧 API 继续写入旧表。批量迁移旧表数据。

### 涉及数据表

| 旧表名 | 目标新表 |
|---|---|
| `extension_scope_bindings` | `scope_bindings` |
| `extension_configs` | `capability_configs` |
| `extension_capability_grants` | `permission_grants` |
| `extension_agent_skill_metadata` | `agent_skill_registry` |
| `extension_agent_skill_activations` | `extension_audit_records` |
| `extension_owned_resources` | `owned_resources` |
| `extension_package_installations` | `package_installations` |
| `extension_package_import_sessions` | `package_import_sessions` |
| `extension_package_signers` | `trusted_signers` |
| `extension_version_dependencies` | `package_dependencies` |
| `extension_package_exports` | `package_exports` |
| `extension_states` | 新 Storage Broker |
| `extension_state_revisions` | 新 Storage Broker |
| `extension_events` | 新 Audit Store |
| `extension_event_deliveries` | 新 Audit Store |
| `extension_schedules` | 新 Schedule Manager |
| `extension_plugin_runs` | 新 Audit Store |
| `extension_audits` | 新 Audit Store |
| `mcp_server_scope_bindings` | 新 Scope Manager |
| `mcp_server_credentials` | 新 Secret Broker |
| `mcp_tools` | 新 Tool Registry |
| `mcp_dependency_links` | 新 Dependency Resolver |
| `mcp_oauth_sessions` | 新 Secret Broker |

### 涉及后端代码

| ID | 文件/函数 | 用途 |
|---|---|---|
| EXT-RT-202 | `repository.go` → `ResolveScopeEnabled`, `SetScopeEnabled`, `DeleteScopeBinding` | 旧作用域绑定读写 |
| EXT-RT-203 | `repository.go` → `GetEffectiveConfig`, `getStoredConfig` | 旧配置读取 |
| AGT-201 | `agent_skill_repository.go` → `agentSkillMetadataRecord`, `ListAgentSkillRecords`, `GetAgentSkillRecord` | Agent Skill 元数据读取 |
| AGT-202 | `agent_skill_repository.go` → `agentSkillActivationRecord`, `SaveAgentSkillActivation`, `ListAgentSkillActivations` | 激活记录读取 |
| AGT-203 | `agent_skill_repository.go` → `encodeAgentSkillArtifact`, `decodeAgentSkillArtifact`, `extractAgentSkillBody` | Artifact 解码 |
| AGT-204 | `agent_skill_repository.go` → `LoadAgentSkill` | 从 DB 加载 Agent Skill |
| AGT-205 | `agent_skill_repository.go` → `SetAgentSkillEnabled`, `RemoveAgentSkill` | 旧启用/删除 |
| AGT-206 | `agent_skill_repository.go` → `InstallAgentSkill` | 旧安装 |
| MCP-201 | `mcp/repository.go` → `GetServer`, `ListServers`, `ListEnabledServers`, `DeleteServer` | 旧 MCP Server CRUD |
| MCP-202 | `mcp/repository.go` → `SetScopeEnabled`, `ResolveScopeEnabled` | 旧 MCP 作用域 |
| MCP-203 | `mcp/repository.go` → `GetToolBySkillID`, `SetToolEnabled` | 旧 MCP Tool 启用 |
| MCP-204 | `mcp/repository.go` → `UpsertDependencyLink`, `ListDependencyLinks`, `RemoveDependencyLinks` | 旧依赖关系 |
| MCP-205 | `mcp/repository.go` → `PutCredentialReference`, `SaveOAuthTokenReference` | 旧凭证引用 |
| MCP-206 | `mcp/repository.go` → `SyncTools`, `SyncResources`, `SyncPrompts` | 旧同步逻辑 |
| PLG-201 | `plugin_repository.go` → `extension_states`, `extension_state_revisions` CRUD | 旧 Plugin 状态 |
| PLG-202 | `plugin_repository.go` → `extension_events`, `extension_event_deliveries` CRUD | 旧事件历史 |
| PLG-203 | `plugin_repository.go` → `extension_schedules` CRUD | 旧调度记录 |
| PLG-204 | `plugin_builtin_diagnostic.go` → `newDiagnosticPlugin` | 旧诊断数据 |
| WS-201 | `workshop_repository.go` → Workshop 历史数据 CRUD | 旧 Workshop 数据 |

---

## 迁移阶段 2：适配器层过渡（依赖阶段 1）

### 目标
在新组件就绪前，保持旧适配器运行以保障功能不中断。

### 涉及适配器

| ID | 文件/函数 | 迁移来源 | 迁移目标 | 删除条件 |
|---|---|---|---|---|
| EXT-RT-201 | `legacy_tool_adapter.go` → `LegacyToolAdapter`, `RegisterAll`, `Adapt` | `agent/tool` 旧工具 | Extension Kernel Tool Registry | 所有旧工具迁移为原生 Capability |
| WFL-201 | `workshop_installer.go` → `workflowHandler` | 旧 Workflow 数据 | Workflow Engine 原生执行 | 旧 Workflow 全部迁移 |
| WFL-202 | `workflow_executor.go` → `SideEffectHost` 绑定 `ExecutionScope` | 旧 Chat/Memory 集成 | 新 Host 接口 | 新 Host 就绪 |
| WS-202 | `workshop_generator.go` → 旧生成逻辑 | 输出 `SkillDefinition`/`WorkflowDefinition` | 新 Capability 格式生成 | 生成器改造完成 |

---

## 迁移阶段 3：Package v1 过渡（依赖阶段 2）

### 目标
保持 v1 包可读，同时支持 v2 格式。

### 涉及代码

| ID | 文件 | 用途 | 删除条件 |
|---|---|---|---|
| PKG-201 | `package_parser.go` → v1 Manifest 解析 | v1 格式解析 | 旧包全部转换 v2 |
| PKG-202 | `package_handler.go` → 所有 HTTP handler | 旧 Package API | 新 API 就绪 |
| PKG-203 | `package_repository.go` → `extension_package_installations` CRUD | 旧安装记录 | 迁移完成 |
| PKG-204 | `package_repository.go` → `extension_package_exports`, `extension_artifacts` | 旧导出和 Artifact | 迁移完成 |
| PKG-205 | `schema/manifest.schema.json` (v1) | v1 格式校验 | v1 包全迁移 |

---

## 迁移阶段 4：API 兼容层（依赖阶段 1-3）

### 目标
旧 API 改为兼容层转发到新 API，前端逐步切换。

### 涉及 API

| 旧 API 路径 | 新 API 路径 | 删除条件 |
|---|---|---|
| `GET /extensions/skills` | `GET /extensions/capabilities` | 前端全部切换 |
| `GET /extensions/skills/:id` | `GET /extensions/capabilities/:id` | 前端全部切换 |
| `POST /extensions/skills/:id/enable` | `POST /extensions/capabilities/:id/enable` | 前端全部切换 |
| `POST /extensions/skills/:id/disable` | `POST /extensions/capabilities/:id/disable` | 前端全部切换 |
| `GET /extensions/skills/:id/permissions` | `GET /extensions/capabilities/:id/permissions` | 前端全部切换 |
| `PUT /extensions/skills/:id/permissions` | `PUT /extensions/capabilities/:id/permissions` | 前端全部切换 |
| `GET /extensions/skills/:id/config` | `GET /extensions/capabilities/:id/config` | 前端全部切换 |
| `PUT /extensions/skills/:id/config` | `PUT /extensions/capabilities/:id/config` | 前端全部切换 |
| `POST /extensions/skills/:id/config/reset` | `POST /extensions/capabilities/:id/config/reset` | 前端全部切换 |
| `POST /extensions/skills/:id/execute` | `POST /extensions/capabilities/:id/execute` | 前端全部切换 |
| `POST /extensions/skills/:id/workshop/fork` | `POST /extensions/capabilities/:id/fork` | 前端全部切换 |
| `POST /extensions/skills/:id/versions/:version/rollback` | `POST /extensions/capabilities/:id/versions/:version/rollback` | 前端全部切换 |
| Agent Skill API 旧路径 | 新 Agent Skill Catalog API | 前端全部切换 |
| Plugin API 旧路径 | 新 Contribution API | 前端全部切换 |
| Workshop API 旧路径 | Developer Tooling API | 前端全部切换 |

### 涉及前端代码

| ID | 文件 | 用途 | 删除条件 |
|---|---|---|---|
| FE-201 | `front/src/views/extensions/api.ts`, `front/src/views/mcp/api.ts` | 旧 API Client | 新 API 就绪 |
| FE-202 | `front/src/views/extensions/types.ts` → `SkillView`, `SkillDetail`, `SkillResult`, `PluginView`, `PluginState`, `PluginSchedule`, `PluginManifest` | 旧类型 | 新类型就绪 |
| FE-203 | `front/src/router/index.ts` → `/extensions/skills`, `/extensions/plugins` 等路由 | 旧路由 | 新路由就绪 |

---

## 迁移阶段 5：清理（依赖阶段 1-4）

### 目标
删除所有仅用于迁移的旧代码、旧表、旧测试。

### 涉及测试文件

| 文件 | 验证目标 | 删除条件 |
|---|---|---|
| `migration/mcp_client_test.go` | MCP 旧表结构 | MCP 旧表删除 |
| `migration/extensions_test.go` | 扩展旧表结构 | 扩展旧表删除 |
| `migration/extension_ecosystem_repair_test.go` | 修复逻辑 | 修复逻辑弃用 |
| `migration/extension_packages_test.go` | Package 旧表结构 | Package 旧表删除 |

### 迁移数据表

所有旧表（23张）在数据验证通过后删除。

---

## 六、迁移风险与缓解

| 风险 | 等级 | 缓解措施 |
|---|---|---|
| 数据迁移中断 | P0 | 事务性迁移，每个表单独迁移+回滚 |
| 双写期间数据不一致 | P0 | 双写期间以新表为准，旧表定期同步校验 |
| 前端切换时期 API 不可用 | P1 | 兼容层保障旧 API 至少可转发，灰度切换 |
| 用户数据和 Secret 丢失 | P0 | `extension_artifacts`, `mcp_server_credentials`, `mcp_oauth_sessions`, `extension_configs` 全量加密备份后操作 |
| MCP 连接中断 | P0 | MCP Server 平滑迁移，连接不中断 |

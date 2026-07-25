# 仅用于迁移对象映射表

> 列出所有「仅用于迁移」对象，标注迁移来源→目标、允许/禁止调用者、停止写入时间、删除条件

---

## 一、后端 Extension Runtime & Registry

| ID | 对象 | 迁移来源 | 迁移目标 | 允许调用者 | 禁止调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|---|---|---|
| EXT-RT-201 | `LegacyToolAdapter` | `agent/tool` 旧工具 | Extension Kernel Tool Registry | 启动迁移脚本 | 新扩展注册流程 | 迁移完成后 | 所有旧工具已迁移为原生 Capability |
| EXT-RT-202 | Repository `ResolveScopeEnabled`, `SetScopeEnabled`, `DeleteScopeBinding` | `extension_scope_bindings` 表 | 新 Scope Manager | 迁移脚本 | 新 Scope 绑定 API | 迁移完成后 | 旧作用域数据全部迁移到新 Scope Manager |
| EXT-RT-203 | Repository `GetEffectiveConfig`, `getStoredConfig` | `extension_configs` 表 | 新 Config Store | 迁移脚本 | 新配置 API | 迁移完成后 | 所有旧配置迁移到新 Config Store |

---

## 二、后端 Agent Skill

| ID | 对象 | 迁移来源 | 迁移目标 | 允许调用者 | 禁止调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|---|---|---|
| AGT-201 | `agentSkillMetadataRecord`, `ListAgentSkillRecords`, `GetAgentSkillRecord` | `extension_agent_skill_metadata` 表 | 新 Agent Skill 存储 | 迁移脚本 | 新 Agent Skill Catalog | 迁移完成后 | 旧元数据全部迁移 |
| AGT-202 | `agentSkillActivationRecord`, `SaveAgentSkillActivation`, `ListAgentSkillActivations` | `extension_agent_skill_activations` 表 | 新 Audit Store | 迁移脚本 | 新审计 API | 迁移完成后 | 激活历史迁移完成 |
| AGT-203 | `encodeAgentSkillArtifact`, `decodeAgentSkillArtifact`, `extractAgentSkillBody` | `extension_artifacts` 表旧格式 | 新 Artifact Store | 迁移脚本 | 新 Artifact API | 迁移完成后 | 旧 Artifact 全部迁移 |
| AGT-204 | `(r *Repository) LoadAgentSkill` | 旧 Agent Skill 存储 | 新 Catalog 加载器 | 迁移脚本 | 新加载流程 | 迁移完成后 | 旧格式数据全部迁移 |
| AGT-205 | `SetAgentSkillEnabled`, `RemoveAgentSkill` | 旧启用/删除逻辑 | 新生命周期管理 | 迁移脚本 | 新 Capability 管理 API | 迁移完成后 | 旧数据表删除 |
| AGT-206 | `(r *Repository) InstallAgentSkill` | 旧安装流程 | 新 Package Manager 安装流程 | 迁移脚本 | 新安装 API | 迁移完成后 | 安装迁移完成 |

---

## 三、后端 MCP

| ID | 对象 | 迁移来源 | 迁移目标 | 允许调用者 | 禁止调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|---|---|---|
| MCP-201 | `GetServer`, `ListServers`, `ListEnabledServers`, `DeleteServer` | `mcp_servers` 表 | 新 MCP Server 存储 | 迁移脚本 | 新 MCP Manager API | 迁移完成后 | 旧 Server 数据全部迁移 |
| MCP-202 | `SetScopeEnabled`, `ResolveScopeEnabled` | `mcp_server_scope_bindings` | 新 Scope Manager | 迁移脚本 | 新 Scope API | 迁移完成后 | 作用域数据迁移完成 |
| MCP-203 | `GetToolBySkillID`, `SetToolEnabled` | `mcp_tools` 表 | 新 Tool Registry | 迁移脚本 | 新 Tool 注册 API | 迁移完成后 | Tool 迁移完成 |
| MCP-204 | `UpsertDependencyLink`, `ListDependencyLinks`, `RemoveDependencyLinks` | `mcp_dependency_links` | 新 Dependency Manager | 迁移脚本 | 新依赖 API | 迁移完成后 | 依赖数据迁移完成 |
| MCP-205 | `PutCredentialReference`, `CredentialReference`, `SaveOAuthTokenReference`, `OAuthTokenReference` | `mcp_server_credentials`, `mcp_oauth_sessions` | 新 Secret Broker | 迁移脚本 | 新 Secret API | 迁移完成后 | 凭证迁移完成 |
| MCP-206 | `SyncTools`, `SyncResources`, `SyncPrompts` | MCP Server 发现缓存 | 新 MCP Manager 同步 | 迁移脚本 | 新 Discovery 流程 | 迁移完成后 | 新 MCP Manager 就绪 |

---

## 四、后端 Plugin

| ID | 对象 | 迁移来源 | 迁移目标 | 允许调用者 | 禁止调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|---|---|---|
| PLG-201 | 旧 Plugin 状态表（`extension_states`, `extension_state_revisions`） | Plugin 持久化状态 | 新 Storage Broker | 迁移脚本 | 新状态 API | 迁移完成后 | 状态迁移完成 |
| PLG-202 | 旧 Event 表（`extension_events`, `extension_event_deliveries`） | Plugin 事件历史 | 新 Audit Store | 迁移脚本 | 新事件 API | 迁移完成后 | 事件历史迁移完成 |
| PLG-203 | 旧 Schedule 表（`extension_schedules`） | Plugin 调度记录 | 新 Schedule Manager | 迁移脚本 | 新调度 API | 迁移完成后 | 调度数据迁移完成 |
| PLG-204 | `newDiagnosticPlugin`（内置诊断） | 内置诊断 Plugin | 新 Developer Tooling | 迁移脚本 | 新诊断工具 | 迁移完成后 | 新诊断工具就绪 |

---

## 五、后端 Workflow

| ID | 对象 | 迁移来源 | 迁移目标 | 允许调用者 | 禁止调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|---|---|---|
| WFL-201 | `workflowHandler`（Workflow → SkillDefinition 包装） | 旧 Workflow 数据 | Workflow Engine 原生执行 | 迁移脚本 | 新 Workflow 注册 | 迁移完成后 | 旧 Workflow 全部迁移 |
| WFL-202 | `SideEffectHost` 绑定 `ExecutionScope` | 旧 Chat/Memory 集成 | 新 Host 接口 | 迁移脚本 | 新 Host 实现 | 迁移完成后 | 新 Host 接口就绪 |

---

## 六、后端 Package

| ID | 对象 | 迁移来源 | 迁移目标 | 允许调用者 | 禁止调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|---|---|---|
| PKG-201 | Manifest v1 Parser | `.amitiax` v1 格式解析 | v2 Manifest Parser | 迁移脚本 | 新包安装流程 | 迁移完成后 | 旧包全部转换为 v2 |
| PKG-202 | 旧 Package HTTP handler | 旧 Package API | 新 Extension Kernel API | 前端（过渡期） | 新 API 客户端 | 新 API 就绪后 | 新 API 就绪且前端切换 |
| PKG-203 | `extension_package_installations` CRUD | 旧安装历史 | 新 Package Store | 迁移脚本 | 新安装记录 API | 迁移完成后 | 安装记录迁移完成 |
| PKG-204 | `extension_package_exports`, `extension_artifacts`（旧格式） | 旧导出和 Artifact | 新 Artifact Store | 迁移脚本 | 新 Artifact API | 迁移完成后 | Artifact 迁移完成 |
| PKG-205 | `schema/manifest.schema.json`（v1） | v1 格式校验 | v2 Schema | 迁移脚本（格式升级验证） | 新包校验 | 迁移完成后 | v1 包全迁移 |

---

## 七、后端 Workshop

| ID | 对象 | 迁移来源 | 迁移目标 | 允许调用者 | 禁止调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|---|---|---|
| WS-201 | 旧 Workshop 数据（`extension_workshop_sessions`, `extension_workshop_revisions`, `extension_workshop_test_runs`） | Workshop 历史数据 | 新 Developer Tooling | 迁移脚本 | 新 Dev Tooling | 迁移完成后 | Workshop 数据迁移完成 |
| WS-202 | 旧 Skill/Workflow 生成逻辑（只输出 `SkillDefinition`/`WorkflowDefinition`） | 旧生成格式 | 新 Capability 格式生成 | 迁移脚本（格式转换） | 新 Generator | 生成器改造完成后 | 生成器改造完成 |

---

## 八、前端

| ID | 对象 | 迁移来源 | 迁移目标 | 允许调用者 | 禁止调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|---|---|---|
| FE-201 | 前端 API Client（旧接口） | 旧 API 端点 | 新 Extension Kernel API | 前端页面（过渡期） | 新页面 | 新 UI 就绪后 | 新 API 就绪且前端全部切换 |
| FE-202 | 旧类型（`SkillView`, `SkillDetail`, `SkillResult`, `PluginView`, `PluginState`, `PluginSchedule`, `PluginManifest`） | Skill/Plugin 旧类型定义 | 新 Extension Kernel 前端类型 | 前端页面（过渡期） | 新组件 | 新 UI 就绪后 | 新类型定义就绪 |
| FE-203 | 路由中的扩展子路径（`/extensions/skills`, `/extensions/skills/:id`, `/extensions/plugins`, `/extensions/plugins/:id` 等） | 旧页面路由 | 新统一路由 | 前端路由系统（过渡期） | 新路由 | 新 UI 就绪后 | 新路由就绪 |

---

## 九、API

| 旧 API | 迁移目标 API | 允许调用者 | 停止写入时间 | 删除条件 |
|---|---|---|---|---|
| `GET /extensions/skills` | `GET /extensions/capabilities` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `GET /extensions/skills/:id` | `GET /extensions/capabilities/:id` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `POST /extensions/skills/:id/enable` | `POST /extensions/capabilities/:id/enable` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `POST /extensions/skills/:id/disable` | `POST /extensions/capabilities/:id/disable` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `GET /extensions/skills/:id/permissions` | `GET /extensions/capabilities/:id/permissions` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `PUT /extensions/skills/:id/permissions` | `PUT /extensions/capabilities/:id/permissions` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `GET /extensions/skills/:id/config` | `GET /extensions/capabilities/:id/config` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `PUT /extensions/skills/:id/config` | `PUT /extensions/capabilities/:id/config` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `POST /extensions/skills/:id/config/reset` | `POST /extensions/capabilities/:id/config/reset` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `POST /extensions/skills/:id/execute` | `POST /extensions/capabilities/:id/execute` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `POST /extensions/skills/:id/workshop/fork` | `POST /extensions/capabilities/:id/fork` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| `POST /extensions/skills/:id/versions/:version/rollback` | `POST /extensions/capabilities/:id/versions/:version/rollback` | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| Agent Skill API 旧路径 | 新 Agent Skill Catalog API | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| Plugin API 旧路径 | 新 Contribution API | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |
| Workshop API 旧路径 | Developer Tooling API | 前端（过渡期） | 新 API 就绪 | 前端全部切换 |

---

## 十、数据表（仅用于迁移）

| 表名 | 迁移来源 | 迁移目标 | 删除条件 |
|---|---|---|---|
| `extension_scope_bindings` | 旧作用域绑定 | 新 Scope Manager（`scope_bindings`） | 双写过渡后旧表数据全部迁移 |
| `extension_configs` | 旧扩展配置 | 新 Config Store（`capability_configs`） | 配置迁移完成 |
| `extension_capability_grants` | 旧权限授权 | 新 Permission Broker（`permission_grants`） | 权限迁移完成 |
| `extension_agent_skill_metadata` | 旧 Agent Skill 元数据 | 新 Agent Skill 存储（`agent_skill_registry`） | 元数据全部迁移 |
| `extension_agent_skill_activations` | 旧激活记录 | 新 Audit Store（`extension_audit_records`） | 激活历史迁移完成 |
| `extension_owned_resources` | 旧资源归属 | 新 Storage Broker（`owned_resources`） | 资源归属迁移完成 |
| `extension_package_installations` | 旧安装记录 | 新 Package Store（`package_installations`） | 安装记录迁移完成 |
| `extension_package_import_sessions` | 旧导入会话 | 新 Package Store（`package_import_sessions`） | 导入记录迁移 |
| `extension_package_signers` | 旧签名者 | 新 Package Store（`trusted_signers`） | 签名者数据迁移 |
| `extension_version_dependencies` | 旧版本依赖 | 新 Dependency Resolver（`package_dependencies`） | 依赖数据迁移 |
| `extension_package_exports` | 旧导出记录 | 新 Package Store（`package_exports`） | 导出记录迁移 |
| `extension_states` | 旧 Plugin 状态 | 新 Storage Broker | Plugin 状态迁移完成 |
| `extension_state_revisions` | 旧状态修订 | 新 Storage Broker | 同上 |
| `extension_events` | 旧事件 | 新 Audit Store | 事件历史迁移完成 |
| `extension_event_deliveries` | 旧事件交付 | 新 Audit Store | 同上 |
| `extension_schedules` | 旧调度 | 新 Schedule Manager | 调度数据迁移完成 |
| `extension_plugin_runs` | 旧 Plugin 运行 | 新 Audit Store | 运行记录迁移完成 |
| `extension_audits` | 旧审计 | 新 Audit Store | 审计记录迁移完成 |
| `mcp_server_scope_bindings` | 旧 MCP 作用域 | 新 Scope Manager | 作用域迁移完成 |
| `mcp_server_credentials` | 旧 MCP 凭证 | 新 Secret Broker | 凭证迁移完成 |
| `mcp_tools` | 旧 MCP Tool | 新 Tool Registry | Tool 迁移完成 |
| `mcp_dependency_links` | 旧依赖关系 | 新 Dependency Resolver | 依赖数据迁移完成 |
| `mcp_oauth_sessions` | 旧 OAuth 会话 | 新 Secret Broker | OAuth 迁移完成 |

---

## 十一、测试（仅用于迁移验证）

| 文件 | 验证目标 | 删除条件 |
|---|---|---|
| `migration/mcp_client_test.go` | 验证 MCP 旧表结构存在 | MCP 旧表删除后移除 |
| `migration/extensions_test.go` | 验证扩展旧表结构存在 | 扩展旧表删除后移除 |
| `migration/extension_ecosystem_repair_test.go` | 验证修复逻辑正确 | 修复逻辑弃用后移除 |
| `migration/extension_packages_test.go` | 验证 Package 旧表结构 | Package 旧表删除后移除 |

---

## 十二、迁移阶段汇总

### 阶段 1：数据表双写过渡
- 对仍有业务写入的表（如 `extension_scope_bindings`, `extension_configs`）建立双写机制
- 新 API 写入新表，旧 API 继续写入旧表
- 迁移脚本将旧表数据批量迁移到新表

### 阶段 2：API 兼容层
- 新 Extension Kernel API 就绪后，旧 API 改为兼容层（转发到新 API）
- 前端逐步从旧 API Client 切换到新 API Client
- 切换完成后停止旧 API 写入

### 阶段 3：批量数据迁移
- 执行全量数据迁移脚本
- 所有旧表数据迁移到新表
- 验证迁移完整性

### 阶段 4：旧表删除
- 数据迁移验证通过后删除旧表
- 删除仅用于迁移的测试

### 阶段 5：旧代码删除
- 删除所有仅用于迁移的 Repository 方法、Adapter、Handler
- 清理迁移脚本本身

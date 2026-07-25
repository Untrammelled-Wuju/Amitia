# 扩展系统数据库表完整清单

> Amitia 扩展系统重构第3步 - 数据表与资源归属清单
> 审计日期: 2026-07-25
> 审计范围: backend/internal/migration/, backend/internal/extension/*repository*.go, backend/internal/mcp/repository.go

## 一、表总览

当前扩展系统共涉及 **38 张数据库表**，按子系统分为 7 组：

| 分组 | 表数量 | 说明 |
|---|---|---|
| Extension Core | 5 | 扩展定义、版本、权限、配置、执行记录 |
| Agent Skills | 2 | Agent Skill 元数据与激活记录 |
| Owned Resources | 1 | 资源所有权追踪 |
| Packages | 5 | .amitiax 包导入、安装、签名、依赖、导出 |
| Scope Bindings | 1 | 扩展作用域绑定 |
| Workshop | 4 | 工坊会话、修订、测试、Artifact |
| Plugin Runtime | 7 | 插件状态、事件、交付、调度、运行、审计 |
| MCP | 13 | MCP Server、凭据、能力、工具、资源、Prompt、依赖、操作、OAuth、任务、审计 |

---

## 二、Extension Core（5 张表）

### 1. extensions

| 字段 | 内容 |
|---|---|
| 表名 | `extensions` |
| 创建迁移 | 初始迁移（随 Extension 系统引入） |
| 领域对象 | Skill / Plugin / Agent Skill 定义 |
| Repository | `extension/repository.go` - `Repository` |
| 创建入口 | `UpsertDefinition()`, AgentSkill Import, Workshop Install, Plugin Register |
| 更新入口 | `UpsertDefinition()`, `SetEnabled()`, `UpdatePluginLifecycle()`, `SetAgentSkillEnabled()`, Workshop Install |
| 删除入口 | `Delete()`, AgentSkill `RemoveAgentSkill()` |
| 主要读取入口 | `Get()`, `List()`, `ListByScope()`, `Available()`, Registry restore |
| 主键 | `extension_id` |
| 逻辑唯一键 | `extension_id` (在 extension_versions 中有版本维度) |
| 外键 | 无物理外键；逻辑引用 `extension_versions`、`extension_artifacts` |
| JSON 字段 | `metadata_json`, `config_schema_json`, `input_schema_json`, `output_schema_json`, `capabilities_json`, `triggers_json`, `openai_metadata_json` |
| 敏感字段 | 无直接敏感字段；通过 `extension_configs` 存储加密配置 |
| 软删除 | 支持 (`archived_at`) |
| 级联删除 | 手动级联：删除时同步标记 `extension_versions`、`extension_artifacts`、`extension_scope_bindings` |
| 启动恢复用途 | Registry restore → 重新注册已启用 Skill |
| 卸载用途 | 标记 `archived_at`，不物理删除 |
| 审计用途 | 否 |
| 数据分类 | 扩展包资产 / 用户资产 |
| 当前问题 | **多写入者**：Repository.UpsertDefinition / Plugin.UpdatePluginLifecycle / AgentSkill.SetAgentSkillEnabled → Review |
| 目标处理 | 保留，拆分 enabled 状态到独立 Runtime State |

### 2. extension_versions

| 字段 | 内容 |
|---|---|
| 表名 | `extension_versions` |
| 创建迁移 | 初始迁移 |
| 领域对象 | 扩展版本记录 |
| Repository | `extension/repository.go` |
| 创建入口 | `UpsertDefinition()`, Package Install, Workshop Install |
| 更新入口 | Workshop `upsertVersion()` |
| 删除入口 | `Delete()` 级联归档 |
| 主要读取入口 | `GetVersion()`, `LatestVersion()` |
| 主键 | `id` |
| 逻辑唯一键 | `extension_id` + `version` |
| JSON 字段 | `manifest_json`, `capabilities_json` |
| 数据分类 | 扩展包资产 / 历史与审计 |
| 当前问题 | 版本记录与 `extension_artifacts` 通过 `artifact_id` 关联，但无物理外键 |
| 目标处理 | 保留，迁移到 Package Store |

### 3. extension_capability_grants

| 字段 | 内容 |
|---|---|
| 表名 | `extension_capability_grants` |
| 领域对象 | 能力授权记录（权限决策） |
| Repository | `extension/repository.go` |
| 创建入口 | `SetCapabilityGrant()`, `GrantSystemPolicy()` |
| 更新入口 | `SetCapabilityGrant()` |
| 删除入口 | `RevokeCapabilityGrant()` |
| 主要读取入口 | `GetCapabilityGrant()`, PermissionEvaluator |
| 主键 | `id` |
| 逻辑唯一键 | `extension_id` + `capability` + `scope_type` + `scope_id` |
| JSON 字段 | `policy_json` |
| 数据分类 | 用户资产（权限配置） |
| 目标处理 | 迁移到 Permission Broker |

### 4. extension_configs

| 字段 | 内容 |
|---|---|
| 表名 | `extension_configs` |
| 领域对象 | 扩展运行时配置 |
| Repository | `extension/repository.go` |
| 创建入口 | `SetConfig()` |
| 更新入口 | `SetConfig()` |
| 删除入口 | `DeleteConfig()` |
| 主要读取入口 | `GetConfig()`, Plugin Host 执行前解密 |
| 主键 | `extension_id` + `scope_type` + `scope_id` |
| 敏感字段 | **是** - `config_json` 字段使用 AES-GCM 加密存储（前缀 `enc:v1:`） |
| 加密方式 | AES-GCM，密钥来源：`AMITIA_EXTENSION_CONFIG_KEY` 环境变量或 `{dbPath}.extension-key` 文件 |
| 数据分类 | 扩展包资产（敏感配置） |
| 当前问题 | 禁用扩展后配置仍可读取 |
| 目标处理 | 迁移到 Secret Broker |

### 5. extension_runs

| 字段 | 内容 |
|---|---|
| 表名 | `extension_runs` |
| 领域对象 | 扩展执行记录 |
| Repository | `extension/repository.go` |
| 创建入口 | `CreateRun()` |
| 更新入口 | `UpdateRun()` |
| 主要读取入口 | `GetRun()`, `ListRuns()` |
| 主键 | `id` |
| JSON 字段 | `input_json`, `output_json`, `side_effects_json` |
| 数据分类 | 历史与审计 |
| 目标处理 | 迁移到 Audit Store |

---

## 三、Agent Skills（2 张表）

### 6. extension_agent_skill_metadata

| 字段 | 内容 |
|---|---|
| 表名 | `extension_agent_skill_metadata` |
| 领域对象 | Agent Skill 元数据 |
| Repository | `extension/agent_skill_repository.go` |
| 创建入口 | `CreateAgentSkill()` |
| 更新入口 | `SetAgentSkillEnabled()`, `RemoveAgentSkill()` |
| 删除入口 | `RemoveAgentSkill()`（软删除 `archived_at`） |
| 主要读取入口 | `GetAgentSkill()`, `ListAgentSkills()` |
| 主键 | `id` |
| 逻辑唯一键 | `extension_id` |
| 外键 | `artifact_id` → `extension_artifacts.artifact_id`（逻辑引用） |
| JSON 字段 | `metadata_json`, `openai_metadata_json`, `raw_frontmatter_json`, `extra_frontmatter_json`, `resource_index_json`, `tool_mappings_json` |
| 数据分类 | 扩展包资产 |
| 当前问题 | 与其他扩展实体共享 `extensions` 表的 `extension_id` 命名空间 |
| 目标处理 | 保留，Agent Skill 元数据独立存储 |

### 7. extension_agent_skill_activations

| 字段 | 内容 |
|---|---|
| 表名 | `extension_agent_skill_activations` |
| 领域对象 | Agent Skill 激活记录 |
| Repository | `extension/agent_skill_repository.go` |
| 创建入口 | `RecordActivation()` |
| 删除入口 | `RemoveAgentSkill()` 级联删除 |
| 主要读取入口 | `ListActivations()` |
| 数据分类 | 运行时临时资源 / 历史与审计 |
| 目标处理 | 并入 Audit Store |

---

## 四、Owned Resources（1 张表）

### 8. extension_owned_resources

| 字段 | 内容 |
|---|---|
| 表名 | `extension_owned_resources` |
| 领域对象 | 扩展拥有的资源追踪 |
| Repository | `extension/owned_resource_repository.go` |
| 创建入口 | `Create()` |
| 删除入口 | `Delete()` |
| 主要读取入口 | `List()` |
| 主键 | `id` |
| 数据分类 | 扩展包资产 |
| 当前问题 | **仅追踪 `schedule_create` 类型**，其他资源类型（文件、通知等）的清理不完整 |
| 目标处理 | 扩展为通用 Owned Resource 模型 |

---

## 五、Packages（5 张表）

### 9. extension_package_import_sessions

| 字段 | 内容 |
|---|---|
| 表名 | `extension_package_import_sessions` |
| 领域对象 | 包导入会话 |
| Repository | `extension/package_repository.go` |
| 创建入口 | `CreateImportSession()` |
| 更新入口 | `UpdateImportSession()` |
| 主要读取入口 | `GetImportSession()` |
| 数据分类 | 运行时临时资源 |
| 目标处理 | 迁移到 Package Store |

### 10. extension_package_installations

| 字段 | 内容 |
|---|---|
| 表名 | `extension_package_installations` |
| 领域对象 | 包安装记录 |
| Repository | `extension/package_repository.go` |
| 创建入口 | `CreateInstallation()` |
| 更新入口 | 升级/回滚操作 |
| 主要读取入口 | `GetInstallation()`, `ListInstallations()` |
| 数据分类 | 扩展包资产 / 历史与审计 |
| 目标处理 | 迁移到 Package Store |

### 11. extension_package_signers

| 字段 | 内容 |
|---|---|
| 表名 | `extension_package_signers` |
| 领域对象 | 包签名者信任信息 |
| Repository | `extension/package_repository.go` |
| 数据分类 | 系统内置资源（信任管理） |
| 目标处理 | 迁移到 Package Store |

### 12. extension_version_dependencies

| 字段 | 内容 |
|---|---|
| 表名 | `extension_version_dependencies` |
| 领域对象 | 版本依赖关系 |
| Repository | `extension/package_repository.go` |
| 数据分类 | 扩展包资产 |
| 目标处理 | 迁移到 Package Store |

### 13. extension_package_exports

| 字段 | 内容 |
|---|---|
| 表名 | `extension_package_exports` |
| 领域对象 | 包导出清单 |
| Repository | `extension/package_repository.go` |
| 数据分类 | 扩展包资产 |
| 目标处理 | 迁移到 Package Store |

---

## 六、Scope Bindings（1 张表）

### 14. extension_scope_bindings

| 字段 | 内容 |
|---|---|
| 表名 | `extension_scope_bindings` |
| 领域对象 | 扩展作用域启用绑定 |
| Repository | `extension/repository.go` |
| 创建入口 | `SetScopeEnabled()` |
| 更新入口 | `SetScopeEnabled()` |
| 删除入口 | `Delete()` 级联 |
| 主要读取入口 | `ListByScope()`, Registry resolve |
| 主键 | `id` |
| 逻辑唯一键 | `extension_id` + `scope_type` + `scope_id` |
| 数据分类 | 用户资产（作用域配置） |
| 当前问题 | **与 `extensions.enabled` 双写**：`SetScopeEnabled` 同时更新两张表 |
| 目标处理 | 统一为单一启用状态源 |

---

## 七、Workshop（4 张表）

### 15. extension_workshop_sessions

| 字段 | 内容 |
|---|---|
| 表名 | `extension_workshop_sessions` |
| 领域对象 | 工坊开发会话 |
| Repository | `extension/workshop_repository.go` |
| 创建入口 | `CreateSession()` |
| 更新入口 | `CASStatus()`, `UpdateSession()` |
| 删除入口 | 归档 (`archived_at`) |
| 主要读取入口 | `GetSession()`, `ListSessions()` |
| 数据分类 | 用户资产 |
| 目标处理 | 保留，归 Workshop Engine |

### 16. extension_workshop_revisions

| 字段 | 内容 |
|---|---|
| 表名 | `extension_workshop_revisions` |
| 领域对象 | 工坊会话修订历史 |
| Repository | `extension/workshop_repository.go` |
| 数据分类 | 历史与审计 |
| 目标处理 | 保留 |

### 17. extension_workshop_test_runs

| 字段 | 内容 |
|---|---|
| 表名 | `extension_workshop_test_runs` |
| 领域对象 | 工坊测试运行记录 |
| Repository | `extension/workshop_repository.go` |
| 数据分类 | 历史与审计 |
| 目标处理 | 迁移到 Audit Store |

### 18. extension_artifacts

| 字段 | 内容 |
|---|---|
| 表名 | `extension_artifacts` |
| 领域对象 | 扩展制品（Agent Skill ZIP 文件 和 Workshop JSON 数据） |
| Repository | `extension/agent_skill_repository.go` 和 `extension/workshop_repository.go` |
| 创建入口 | Agent Skill Import（写入 `content_blob`）/ Workshop Install（写入 JSON 字段） |
| 更新入口 | Workshop `upsertVersion()` |
| 删除入口 | 归档 (`archived_at`) |
| 主要读取入口 | `GetAgentSkill()`, `CurrentArtifacts()`, `GetArtifact()`, `GetSessionArtifact()` |
| 主键 | `id` |
| 逻辑唯一键 | `artifact_id`（但不严格唯一） |
| JSON 字段 | `manifest_json`, `workflow_json`, `schemas_json`, `compiled_workflow_json`, `tests_json`, `resource_index_json` |
| 敏感字段 | `content_blob`（Agent Skill ZIP 二进制） |
| 数据分类 | **混合**：Agent Skill 的 `content_blob`（包资产）、Workshop 的 JSON（用户资产） |
| 当前问题 | **多写入者/多格式**：Agent Skill 使用 `content_blob`（ZIP），Workshop 使用 JSON 字段。两者写入同一张表，通过 `artifact_kind`（`agent-skill` / `workshop`）区分。**跨两个 Repository** |
| 目标处理 | **拆分**：Agent Skill Artifact 独立表，Workshop Artifact 独立表 |

---

## 八、Plugin Runtime（7 张表）

### 19. extension_states

| 字段 | 内容 |
|---|---|
| 表名 | `extension_states` |
| 领域对象 | 插件运行时状态 |
| Repository | `extension/plugin_repository.go` |
| 创建入口 | `UpsertPluginState()` |
| 更新入口 | `UpsertPluginState()`（CAS 版本号） |
| 敏感字段 | **是** - `state_json` 使用 AES-GCM 加密（同上 `config_crypto` 方案） |
| 数据分类 | 运行时状态 |
| 目标处理 | 迁移到 Runtime State Store |

### 20. extension_state_revisions

| 字段 | 内容 |
|---|---|
| 表名 | `extension_state_revisions` |
| 领域对象 | 插件状态版本历史 |
| Repository | `extension/plugin_repository.go` |
| 数据分类 | 历史与审计 |
| 目标处理 | 迁移到 Audit Store |

### 21. extension_events

| 字段 | 内容 |
|---|---|
| 表名 | `extension_events` |
| 领域对象 | 插件事件 |
| Repository | `extension/plugin_repository.go` |
| 数据分类 | 运行时临时资源 |
| 目标处理 | 迁移到 Event Bus |

### 22. extension_event_deliveries

| 字段 | 内容 |
|---|---|
| 表名 | `extension_event_deliveries` |
| 领域对象 | 事件交付记录（含重试状态） |
| Repository | `extension/plugin_repository.go` |
| 数据分类 | 运行时临时资源 / 历史与审计 |
| 当前问题 | 引用的 Plugin 可能已被删除 |
| 目标处理 | 迁移到 Event Bus |

### 23. extension_schedules

| 字段 | 内容 |
|---|---|
| 表名 | `extension_schedules` |
| 领域对象 | 插件调度的定时任务 |
| Repository | `extension/plugin_repository.go` |
| 创建入口 | Schedule Worker 从事件创建 |
| 数据分类 | 运行时状态 |
| 当前问题 | 禁用 Plugin 后 Schedule 可能继续触发 |
| 目标处理 | 迁移到 Runtime Supervisor |

### 24. extension_plugin_runs

| 字段 | 内容 |
|---|---|
| 表名 | `extension_plugin_runs` |
| 领域对象 | 插件执行记录 |
| Repository | `extension/plugin_repository.go` |
| 数据分类 | 历史与审计 |
| 目标处理 | 迁移到 Audit Store |

### 25. extension_audits

| 字段 | 内容 |
|---|---|
| 表名 | `extension_audits` |
| 领域对象 | 扩展审计日志 |
| Repository | `extension/plugin_repository.go` |
| 数据分类 | 历史与审计 |
| 目标处理 | 迁移到 Audit Store |

---

## 九、MCP（13 张表）

### 26. mcp_servers

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_servers` |
| 领域对象 | MCP Server 定义与连接配置 |
| Repository | `mcp/repository.go` |
| 创建入口 | `CreateServer()`（API: POST /api/mcp/servers） |
| 更新入口 | `UpdateServer()`, `SetServerStatus()`, `UpdateServerEnabled()` |
| 删除入口 | `DeleteServer()` |
| 主要读取入口 | `GetServer()`, `ListEnabledServers()`, Manager.Restore() |
| 主键 | `id` |
| 逻辑唯一键 | `normalized_identity` |
| JSON 字段 | `args_json`, `server_info_json`, `capabilities_json` |
| 敏感字段 | `endpoint`, `command`, `args_json`（可能含路径信息） |
| 数据分类 | 用户资产 / 共享资源 |
| 当前问题 | **状态与定义混存**：`status`, `last_error_code`, `last_error_message` 与配置字段混在同一张表 |
| 目标处理 | 拆分：Server 定义存 Package Store，连接状态存 Runtime Supervisor |

### 27. mcp_server_scope_bindings

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_server_scope_bindings` |
| 领域对象 | MCP Server 作用域绑定 |
| Repository | `mcp/repository.go` |
| 数据分类 | 用户资产（权限配置） |
| 目标处理 | 迁移到 Permission Broker |

### 28. mcp_server_credentials

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_server_credentials` |
| 领域对象 | MCP 凭据引用 |
| Repository | `mcp/repository.go` |
| 创建入口 | API 设置凭据时 |
| 删除入口 | DeleteServer 级联 |
| 敏感字段 | `secret_reference` - 指向 EncryptedFileStore 中加密存储的实际 Secret |
| 数据分类 | 用户资产（敏感凭据引用） |
| 当前问题 | Secret 实际值存于文件而非数据库，引用断链后 Secret 无法回收 |
| 目标处理 | 迁移到 Secret Broker |

### 29. mcp_server_capabilities

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_server_capabilities` |
| 领域对象 | MCP Server 能力开关 |
| Repository | `mcp/repository.go` |
| JSON 字段 | `configuration_json` |
| 数据分类 | 用户资产（能力配置） |
| 目标处理 | 保留，归 MCP Manager |

### 30. mcp_tools

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_tools` |
| 领域对象 | MCP Tool 定义（Discovery 缓存） |
| Repository | `mcp/repository.go` |
| 创建入口 | Discovery Service `discover()` |
| 更新入口 | Discovery Service 重新发现 |
| 删除入口 | DeleteServer 级联 |
| 主键 | `id` |
| 逻辑唯一键 | `server_id` + `remote_name` / `skill_id` |
| JSON 字段 | `input_schema_json`, `output_schema_json`, `annotations_json`, `execution_json`, `capability_hints_json` |
| 数据分类 | 派生资源（可从 MCP Server 重新发现） |
| 当前问题 | Tool Discovery 缓存，源 Server 删除后可能残留 |
| 目标处理 | 迁移到 Tool Registry |

### 31. mcp_resources

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_resources` |
| 领域对象 | MCP Resource 定义（Discovery 缓存） |
| Repository | `mcp/repository.go` |
| 数据分类 | 派生资源 |
| 目标处理 | 迁移到 Tool Registry |

### 32. mcp_resource_templates

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_resource_templates` |
| 领域对象 | MCP Resource Template（Discovery 缓存） |
| Repository | `mcp/repository.go` |
| 数据分类 | 派生资源 |
| 目标处理 | 迁移到 Tool Registry |

### 33. mcp_prompts

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_prompts` |
| 领域对象 | MCP Prompt 定义（Discovery 缓存） |
| Repository | `mcp/repository.go` |
| JSON 字段 | `arguments_json` |
| 数据分类 | 派生资源 |
| 目标处理 | 迁移到 Tool Registry |

### 34. mcp_dependency_links

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_dependency_links` |
| 领域对象 | Agent Skill 对 MCP Server 的依赖声明 |
| Repository | `mcp/repository.go` |
| 创建入口 | Dependency Service `Install()` |
| 删除入口 | Dependency Service `Uninstall()` |
| 主键 | `id` |
| 逻辑唯一键 | `agent_skill_extension_id` + `server_id` + `dependency_name` |
| 外键 | 逻辑引用 `mcp_servers`（server_id）和 `extensions`（agent_skill_extension_id），**无物理外键** |
| 数据分类 | 扩展包资产（依赖声明） |
| 当前问题 | 可能指向已删除的 Server 或 Agent Skill |
| 目标处理 | 迁移到 Package Store / Dependency Resolver |

### 35. mcp_operations

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_operations` |
| 领域对象 | MCP 操作记录 |
| Repository | `mcp/repository.go` |
| JSON 字段 | `plan_json`, `result_json` |
| 数据分类 | 历史与审计 |
| 目标处理 | 迁移到 Audit Store |

### 36. mcp_oauth_sessions

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_oauth_sessions` |
| 领域对象 | MCP OAuth 授权会话 |
| Repository | `mcp/repository.go` |
| 敏感字段 | `code_verifier_reference` - 指向 Secret Store 中的 `code_verifier` |
| JSON 字段 | `requested_scopes_json` |
| 数据分类 | 运行时临时资源 |
| 目标处理 | 迁移到 Secret Broker |

### 37. mcp_tasks

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_tasks` |
| 领域对象 | MCP Task（长时间运行任务） |
| Repository | `mcp/repository.go` |
| JSON 字段 | `result_json` |
| 数据分类 | 运行时状态 / 历史与审计 |
| 目标处理 | 迁移到 Workflow Engine / Audit Store |

### 38. mcp_audit_logs

| 字段 | 内容 |
|---|---|
| 表名 | `mcp_audit_logs` |
| 领域对象 | MCP 审计日志 |
| Repository | `mcp/repository.go` |
| JSON 字段 | `summary_json` |
| 数据分类 | 历史与审计 |
| 目标处理 | 迁移到 Audit Store |

---

## 十、关键索引清单

| 表 | 索引 | 类型 |
|---|---|---|
| extensions | `extension_id` | PRIMARY |
| extension_versions | `extension_id` + `version` | UNIQUE (逻辑) |
| extension_capability_grants | `extension_id` + `capability` + `scope_type` + `scope_id` | UNIQUE (逻辑) |
| extension_scope_bindings | `extension_id` + `scope_type` + `scope_id` | UNIQUE (逻辑) |
| mcp_servers | `normalized_identity` | UNIQUE |
| mcp_tools | `server_id` + `remote_name` | UNIQUE |
| mcp_tools | `skill_id` | UNIQUE |
| mcp_resources | `server_id` + `uri` | UNIQUE |
| mcp_resource_templates | `server_id` + `uri_template` | UNIQUE |
| mcp_prompts | `server_id` + `remote_name` | UNIQUE |
| mcp_dependency_links | `agent_skill_extension_id` + `server_id` + `dependency_name` | UNIQUE |
| mcp_server_scope_bindings | `server_id` + `scope_type` + `scope_id` | UNIQUE |
| mcp_tasks | `server_id` + `remote_task_id` | UNIQUE |
| mcp_oauth_sessions | `state_hash` | UNIQUE |

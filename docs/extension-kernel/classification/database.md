# 数据表分类

> 范围：所有扩展相关数据库表

---

## 一、改造后复用（原表保留并改名）

| 表名 | 当前数据量 | 有用户数据 | 有敏感数据 | 参与恢复 | 参与回滚 | 多系统写入 | 目标新表 | 迁移方式 | 删除条件 | 备份要求 |
|---|---|---|---|---|---|---|---|---|---|---|
| `extensions` | 中等 | 是 | 否 | 是 | 是 | 否 | `contributions` | 迁移后重命名 | 数据全部迁移 | 全量备份 |
| `extension_versions` | 中等 | 是 | 否 | 是 | 是 | 否 | `contribution_versions` | 迁移后重命名 | 数据全部迁移 | 全量备份 |
| `extension_runs` | 大量 | 是 | 否 | 否 | 否 | 否 | `extension_run_records` | 迁移后重命名 | 数据全部迁移 | 增量备份 |
| `extension_artifacts` | 中等 | 是 | 否 | 是 | 是 | 是（Workshop+Package） | `extension_artifacts` | 保留原表 | 多写入合并 | 全量备份 |
| `extension_workshop_sessions` | 少量 | 是 | 否 | 否 | 否 | 否 | `dev_sessions` | 迁移后重命名 | 数据全部迁移 | 全量备份 |
| `extension_workshop_revisions` | 少量 | 是 | 否 | 否 | 否 | 否 | `dev_revisions` | 迁移后重命名 | 数据全部迁移 | 全量备份 |
| `extension_workshop_test_runs` | 少量 | 否 | 否 | 否 | 否 | 否 | `dev_test_runs` | 迁移后重命名 | 数据全部迁移 | 可选 |

---

## 二、保留数据并迁入新表

| 表名 | 目标新表 | 迁移方式 | 删除条件 |
|---|---|---|---|
| `extension_scope_bindings` | `scope_bindings` | 批量迁移 + 双写过渡 | 新 Scope Manager 就绪 |
| `extension_configs` | `capability_configs` | 批量迁移 | 配置迁移完成 |
| `extension_capability_grants` | `permission_grants` | 批量迁移 | 权限迁移完成 |
| `extension_agent_skill_metadata` | `agent_skill_registry` | 批量迁移 | Agent Skill 迁移完成 |
| `extension_agent_skill_activations` | `extension_audit_records` | 迁移为审计记录 | 激活历史迁移完成 |
| `extension_owned_resources` | `owned_resources` | 批量迁移 | 资源归属迁移完成 |
| `extension_package_installations` | `package_installations` | 批量迁移 | 安装记录迁移完成 |
| `extension_package_import_sessions` | `package_import_sessions` | 批量迁移 | 导入记录迁移 |
| `extension_package_signers` | `trusted_signers` | 批量迁移 | 签名者数据迁移 |
| `extension_version_dependencies` | `package_dependencies` | 批量迁移 | 依赖数据迁移 |
| `extension_package_exports` | `package_exports` | 批量迁移 | 导出记录迁移 |

---

## 三、迁移完成后删除

| 表名 | 当前数据量 | 有用户数据 | 删除条件 |
|---|---|---|---|
| `extension_states` | 极少（仅诊断Plugin） | 否 | Plugin 状态迁入新 Storage Broker |
| `extension_state_revisions` | 极少 | 否 | 同上 |
| `extension_events` | 少量 | 否 | 事件历史迁入新 Audit Store |
| `extension_event_deliveries` | 少量 | 否 | 同上 |
| `extension_schedules` | 极少 | 否 | 调度数据迁入新 Schedule Manager |
| `extension_plugin_runs` | 极少 | 否 | 运行记录迁入新 Audit Store |
| `extension_audits` | 少量 | 否 | 审计记录迁入新 Audit Store |
| `schedules`（迁移修复创建） | 极少 | 否 | 调度数据迁入新 Schedule Manager |

---

## 四、直接删除（空表或诊断表）

无。所有扩展相关表均包含用户数据或迁移价值。

---

## 五、MCP 数据表分类

| 表名 | 处理方式 | 目标新表 | 删除条件 |
|---|---|---|---|
| `mcp_servers` | 改造后复用 | `mcp_servers_v2` | 新 MCP Manager 就绪 |
| `mcp_server_scope_bindings` | 仅用于迁移 | 新 Scope Manager | 作用域迁移完成 |
| `mcp_server_credentials` | 仅用于迁移 | 新 Secret Broker | 凭证迁移完成 |
| `mcp_server_capabilities` | 改造后复用 | 新 MCP Manager | 能力数据迁移 |
| `mcp_tools` | 仅用于迁移 | 新 Tool Registry | Tool 迁移完成 |
| `mcp_resources` | 改造后复用 | 新 MCP Manager | 资源数据迁移 |
| `mcp_resource_templates` | 改造后复用 | 新 MCP Manager | 模板数据迁移 |
| `mcp_prompts` | 改造后复用 | 新 MCP Manager | Prompts 数据迁移 |
| `mcp_dependency_links` | 仅用于迁移 | 新 Dependency Resolver | 依赖迁移完成 |
| `mcp_operations` | 改造后复用 | 新 Operation Store | 操作记录迁移 |
| `mcp_oauth_sessions` | 仅用于迁移 | 新 Secret Broker | OAuth 迁移完成 |
| `mcp_tasks` | 改造后复用 | 新 MCP Task Store | 任务数据迁移 |
| `mcp_audit_logs` | 改造后复用 | 新 Audit Store | 审计日志迁移 |

---

## 六、高风险表（P0 级审查）

### P0-001: `extension_artifacts`
- **风险**: 被 Workshop 和 Package 多系统写入，删除可能丢失用户扩展包
- **处理**: 保留原表，统一为 Extension Kernel 的 Artifact Store
- **备份**: 全量备份后再操作

### P0-002: `mcp_server_credentials`
- **风险**: 包含 Secret 引用，删除可能丢失 MCP 连接凭证
- **处理**: 迁移到 Secret Broker 后再删除
- **备份**: 全量加密备份

### P0-003: `mcp_oauth_sessions`
- **风险**: 包含 OAuth Token 引用
- **处理**: 迁移到 Secret Broker 后再删除
- **备份**: 全量加密备份

### P0-004: `extension_configs`
- **风险**: 包含用户加密配置，删除可能丢失用户自定义配置
- **处理**: 批量迁移后再删除
- **备份**: 全量加密备份

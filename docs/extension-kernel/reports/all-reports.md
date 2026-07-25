# 问题报告

> 审计日期: 2026-07-25
> 基于源码审计和运行时分析

---

## 报告 1：多写入者与重复所有权

### P0: extensions 表三写入者

**涉及表**: `extensions`
**写入者**:
1. `Repository.UpsertDefinition()` - Skill 定义更新
2. `PluginManager.UpdatePluginLifecycle()` - Plugin 生命周期
3. `AgentSkillRepository.SetAgentSkillEnabled()` / `RemoveAgentSkill()` - Agent Skill 启用状态

**风险**: 并发写入冲突，特别是 `enabled` 字段。例如：用户通过 Extension API 禁用 Skill，同时 AgentSkillService 也在更新启用状态。

**建议**: 将 `enabled` 状态拆分为独立的 Runtime State 表，各 Service 只写入自己负责的字段。

---

### P0: extension_scope_bindings 与 extensions.enabled 双写

**涉及表**: `extension_scope_bindings`, `extensions`
**问题**: `SetScopeEnabled()` 同时更新两张表的 enabled 状态。
**风险**: 两张表状态不一致，Registry Restore 依赖 `extension_scope_bindings` 但直接查询 `extensions.enabled` 也可能出问题。

**建议**: 统一启用状态来源，`extensions.enabled` 作为全局开关，`extension_scope_bindings` 作为作用域开关。

---

### P1: extension_artifacts 三系统共享

**涉及表**: `extension_artifacts`
**写入者**:
1. `AgentSkillRepository` - Agent Skill ZIP blob (`content_blob`, `artifact_kind: "agent-skill"`)
2. `WorkshopRepository` - Workshop JSON 字段 (`artifact_kind: "workshop"`)
3. `PackageRepository` - Package Artifact (`artifact_kind: "package"`)

**风险**:
- 三种不同数据结构存于同一张表，通过字符串枚举区分
- `content_blob` 和 JSON 字段的使用方式完全不同
- 查询性能受混合数据影响
- 数据迁移复杂度高

**建议**: 拆分为独立表：`agent_skill_artifacts`, `workshop_artifacts`, `package_artifacts`

---

### P1: mcp_dependency_links 无物理外键

**涉及表**: `mcp_dependency_links`
**逻辑外键**: `agent_skill_extension_id` → `extensions.extension_id`, `server_id` → `mcp_servers.id`
**风险**: 删除 Agent Skill 或 MCP Server 后，依赖链接成为孤儿记录。虽然有 `DependencyService.Uninstall()`，但直接通过 Repository 删除时不触发。

**建议**: 增加软删除检查；或在 Extension Kernel 中实现依赖解析器自动清理。

---

## 报告 2：清理缺口与资源泄漏

### P0: MCP Secret 不随 Server 删除清理

**问题**: `DeleteServer()` 删除 `mcp_server_credentials` 记录，但 `mcp-secrets.json` 中的实际加密值不删除。
**影响**: Secret 文件无限增长；旧凭据永不过期；安全审计时无法确认哪些 Secret 仍在使用。
**当前修复可行性**: 低（需要知道 `secret_reference` 才能删除，但表记录已删除）
**新内核建议**: Secret Broker 实现引用计数和自动回收。

---

### P1: Plugin 禁用后 Schedule 继续触发

**问题**: `scheduleWorker` 从 DB 读取到期 Schedule 时不检查 Plugin 的 enabled 状态。
**影响**: 禁用 Plugin 后，其 Schedule 继续触发并失败。
**新内核建议**: Runtime Supervisor 在执行前检查 Plugin 启用状态。

---

### P1: Plugin 卸载后 Event/Delivery 不清理

**问题**: 卸载 Plugin 不清理其关联的 `extension_events` 和 `extension_event_deliveries`。
**影响**: 无效事件和交付记录占用存储空间。
**新内核建议**: 卸载时级联清理所有运行时数据。

---

### P2: extension_owned_resources 仅追踪 schedule_create

**问题**: `OwnedResourceRepository` 只追踪 `schedule_create` 类型的资源。其他副作用（文件创建、通知发送、内存写入）不记录所有权。
**影响**: 卸载/禁用时无法完整清理扩展产生的资源。
**新内核建议**: 扩展为通用 Owned Resource 模型，覆盖所有副作用类型。

---

### P2: MCP 子进程无孤儿清理

**问题**: stdio 子进程在应用非正常退出时可能残留（Windows 上父进程退出不保证子进程终止）。
**影响**: 孤儿进程占用端口和资源。
**新内核建议**: Runtime Supervisor 启动时执行孤儿进程清理。

---

## 报告 3：临时资源泄漏

### P2: afterReplyQ 内存队列无持久化

**问题**: `PluginManager.afterReplyQ` 是内存 channel，应用崩溃后队列内容丢失。
**影响**: 已触发的 AfterReply Hook 不会执行，不会重试。
**新内核建议**: 改为持久化队列或 DB-backed 队列。

---

### P2: Plugin Circuit Breaker 状态不持久化

**问题**: 熔断器状态纯内存，重启后重置。
**影响**: 持续出问题的插件在每次重启后都有机会被调用（half_open 状态丢失）。
**新内核建议**: Runtime Supervisor 持久化熔断状态。

---

## 报告 4：迁移目标汇总

### Extension Kernel 组件归属

| 新内核组件 | 接管资源 | 来源 |
|---|---|---|
| Package Store | extensions, extension_versions, extension_package_*, extension_artifacts (package) | 当前: extension/repository.go, package_repository.go |
| Runtime Supervisor | extension_states, extension_state_revisions, extension_events, extension_event_deliveries, extension_schedules, extension_plugin_runs, extension_audits, PluginManager workers | 当前: plugin_repository.go, plugin_manager.go |
| Tool Registry | extensions (Tool 定义部分), extension_versions (无) | 当前: registry.go |
| Agent Skill Catalog | extension_agent_skill_metadata, extension_agent_skill_activations, extension_artifacts (agent-skill) | 当前: agent_skill_repository.go, agent_skill_service.go |
| MCP Manager | mcp_servers, mcp_server_scope_bindings, mcp_server_capabilities, mcp_tools, mcp_resources, mcp_resource_templates, mcp_prompts, mcp_dependency_links, mcp_operations, mcp_tasks, mcp_audit_logs | 当前: mcp/repository.go, manager/manager.go |
| Workflow Engine | extension_workshop_*, extension_artifacts (workshop, compiled_workflow), workflow_compiler, workflow_executor | 当前: workshop_repository.go, workshop_service.go |
| UI Registry | 前端扩展页面组件注册 | 当前: router.go, 前端 views |
| Permission Broker | extension_capability_grants, extension_scope_bindings, mcp_server_scope_bindings | 当前: repository.go, permission.go |
| Storage Broker | (新组件) 扩展自有存储 | 当前: 无独立实现 |
| Secret Broker | extension_configs, extension_states (加密部分), mcp_secrets.json, mcp_server_credentials, mcp_oauth_sessions | 当前: config_crypto.go, token_store.go |
| Audit Store | extension_runs, extension_agent_skill_activations, extension_workshop_test_runs, extension_plugin_runs, extension_audits, mcp_operations, mcp_audit_logs | 当前: 各 repository |

### 表迁移决策

| 处理方式 | 表 | 原因 |
|---|---|---|
| **保留** | extensions, extension_versions, mcp_servers, extension_workshop_sessions, extension_workshop_revisions | 核心业务数据，结构合理 |
| **拆分** | extension_artifacts → agent_skill_artifacts + workshop_artifacts + package_artifacts | 三系统共享一张表 |
| **合并** | extension_runs + extension_plugin_runs + extension_audits + mcp_audit_logs + mcp_operations → 统一 audit_logs | 同质化的审计/运行记录 |
| **合并** | extension_events + extension_event_deliveries → 统一 event_bus 表 | 事件系统统一 |
| **只读历史** | extension_state_revisions | 历史版本，只读保留 |
| **删除** | extension_owned_resources | 当前实现不完整，新内核重新设计 |

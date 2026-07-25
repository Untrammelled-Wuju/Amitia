# Repository 到数据表映射

> 审计日期: 2026-07-25

## 一、Repository 清单

当前扩展系统共有 **7 个 Repository**，负责 38 张表的 CRUD 操作。

---

### 1. extension/repository.go - `Repository`

**文件**: `backend/internal/extension/repository.go`

| 方法 | 操作 | 涉及表 |
|---|---|---|
| `UpsertDefinition()` | INSERT/UPDATE | `extensions`, `extension_versions` |
| `Get()` | SELECT | `extensions` |
| `List()` | SELECT | `extensions` |
| `ListByScope()` | SELECT | `extensions` JOIN `extension_scope_bindings` |
| `Available()` | SELECT | `extensions` JOIN `extension_scope_bindings` |
| `SetEnabled()` | UPDATE | `extensions` |
| `SetScopeEnabled()` | INSERT/UPDATE | `extension_scope_bindings`, `extensions` |
| `Delete()` | UPDATE (软删除) | `extensions`, `extension_versions`, `extension_artifacts`, `extension_scope_bindings` |
| `SetCapabilityGrant()` | INSERT/UPDATE | `extension_capability_grants` |
| `RevokeCapabilityGrant()` | DELETE | `extension_capability_grants` |
| `GetCapabilityGrant()` | SELECT | `extension_capability_grants` |
| `SetConfig()` | INSERT/UPDATE | `extension_configs` |
| `GetConfig()` | SELECT | `extension_configs` |
| `DeleteConfig()` | DELETE | `extension_configs` |
| `CreateRun()` | INSERT | `extension_runs` |
| `UpdateRun()` | UPDATE | `extension_runs` |
| `GetRun()` | SELECT | `extension_runs` |
| `ListRuns()` | SELECT | `extension_runs` |
| `GetVersion()` | SELECT | `extension_versions` |
| `LatestVersion()` | SELECT | `extension_versions` |

**覆盖表**: extensions, extension_versions, extension_capability_grants, extension_configs, extension_scope_bindings, extension_runs (6 张)

---

### 2. extension/plugin_repository.go - `PluginRepository`

**文件**: `backend/internal/extension/plugin_repository.go`

| 方法 | 操作 | 涉及表 |
|---|---|---|
| `UpsertPluginState()` | INSERT/UPDATE (CAS) | `extension_states` |
| `GetPluginState()` | SELECT | `extension_states` |
| `RecordStateRevision()` | INSERT | `extension_state_revisions` |
| `CreateEvent()` | INSERT | `extension_events` |
| `CreateDelivery()` | INSERT | `extension_event_deliveries` |
| `UpdateDelivery()` | UPDATE | `extension_event_deliveries` |
| `GetPendingDeliveries()` | SELECT | `extension_event_deliveries` |
| `CreateSchedule()` | INSERT | `extension_schedules` |
| `GetDueSchedules()` | SELECT | `extension_schedules` |
| `DeleteSchedule()` | DELETE | `extension_schedules` |
| `CreatePluginRun()` | INSERT | `extension_plugin_runs` |
| `UpdatePluginRun()` | UPDATE | `extension_plugin_runs` |
| `CreateAudit()` | INSERT | `extension_audits` |

**覆盖表**: extension_states, extension_state_revisions, extension_events, extension_event_deliveries, extension_schedules, extension_plugin_runs, extension_audits (7 张)

---

### 3. extension/package_repository.go - `PackageRepository`

**文件**: `backend/internal/extension/package_repository.go`

| 方法 | 操作 | 涉及表 |
|---|---|---|
| `CreateImportSession()` | INSERT | `extension_package_import_sessions` |
| `UpdateImportSession()` | UPDATE | `extension_package_import_sessions` |
| `GetImportSession()` | SELECT | `extension_package_import_sessions` |
| `CreateInstallation()` | INSERT | `extension_package_installations` |
| `GetInstallation()` | SELECT | `extension_package_installations` |
| `ListInstallations()` | SELECT | `extension_package_installations` |
| `UpsertSigner()` | INSERT/UPDATE | `extension_package_signers` |
| `CreateDependency()` | INSERT | `extension_version_dependencies` |
| `CreateExport()` | INSERT | `extension_package_exports` |
| `UpsertPackageVersion()` | INSERT/UPDATE | `extension_versions` |
| `UpsertPackageArtifact()` | INSERT/UPDATE | `extension_artifacts` |

**覆盖表**: extension_package_import_sessions, extension_package_installations, extension_package_signers, extension_version_dependencies, extension_package_exports, extension_versions, extension_artifacts (7 张)

---

### 4. extension/agent_skill_repository.go - `AgentSkillRepository`

**文件**: `backend/internal/extension/agent_skill_repository.go`

| 方法 | 操作 | 涉及表 |
|---|---|---|
| `CreateAgentSkill()` | INSERT | `extensions`（通过 `extension_agent_skill_metadata`）, `extension_artifacts` |
| `GetAgentSkill()` | SELECT | `extension_agent_skill_metadata` JOIN `extension_artifacts` |
| `ListAgentSkills()` | SELECT | `extension_agent_skill_metadata` |
| `SetAgentSkillEnabled()` | UPDATE | `extensions`, `extension_agent_skill_metadata` |
| `RemoveAgentSkill()` | UPDATE (归档) | `extension_agent_skill_metadata`, `extension_artifacts`, `extensions` |
| `RecordActivation()` | INSERT | `extension_agent_skill_activations` |
| `ListActivations()` | SELECT | `extension_agent_skill_activations` |

**覆盖表**: extensions, extension_agent_skill_metadata, extension_agent_skill_activations, extension_artifacts (4 张)

---

### 5. extension/workshop_repository.go - `WorkshopRepository`

**文件**: `backend/internal/extension/workshop_repository.go`

| 方法 | 操作 | 涉及表 |
|---|---|---|
| `CreateSession()` | INSERT | `extension_workshop_sessions` |
| `GetSession()` | SELECT | `extension_workshop_sessions` |
| `ListSessions()` | SELECT | `extension_workshop_sessions` |
| `CASStatus()` | UPDATE (CAS) | `extension_workshop_sessions` |
| `UpdateSession()` | UPDATE | `extension_workshop_sessions` |
| `CreateRevision()` | INSERT | `extension_workshop_revisions` |
| `CreateTestRun()` | INSERT | `extension_workshop_test_runs` |
| `GetArtifact()` | SELECT | `extension_artifacts` |
| `GetSessionArtifact()` | SELECT | `extension_artifacts` |
| `CurrentArtifacts()` | SELECT | `extension_artifacts` JOIN `extensions` |

**覆盖表**: extension_workshop_sessions, extension_workshop_revisions, extension_workshop_test_runs, extension_artifacts, extensions (5 张)

---

### 6. extension/owned_resource_repository.go - `OwnedResourceRepository`

**文件**: `backend/internal/extension/owned_resource_repository.go`

| 方法 | 操作 | 涉及表 |
|---|---|---|
| `Create()` | INSERT | `extension_owned_resources` |
| `Delete()` | DELETE | `extension_owned_resources` |
| `List()` | SELECT | `extension_owned_resources` |

**覆盖表**: extension_owned_resources (1 张)

---

### 7. mcp/repository.go - `mcp.Repository`

**文件**: `backend/internal/mcp/repository.go`

| 方法 | 操作 | 涉及表 |
|---|---|---|
| `CreateServer()` | INSERT | `mcp_servers` |
| `GetServer()` | SELECT | `mcp_servers` |
| `UpdateServer()` | UPDATE | `mcp_servers` |
| `DeleteServer()` | DELETE | `mcp_servers`（级联删除关联表） |
| `ListEnabledServers()` | SELECT | `mcp_servers` |
| `SetServerStatus()` | UPDATE | `mcp_servers` |
| `UpdateServerEnabled()` | UPDATE | `mcp_servers` |
| `SetScopeBinding()` | INSERT/UPDATE | `mcp_server_scope_bindings` |
| `CreateCredential()` | INSERT | `mcp_server_credentials` |
| `CredentialReference()` | SELECT | `mcp_server_credentials` |
| `DeleteCredential()` | DELETE | `mcp_server_credentials` |
| `UpsertCapability()` | INSERT/UPDATE | `mcp_server_capabilities` |
| `ServerCapabilityEnabled()` | SELECT | `mcp_server_capabilities` |
| `UpsertTool()` | INSERT/UPDATE | `mcp_tools` |
| `DeleteToolsByServer()` | DELETE | `mcp_tools` |
| `UpsertResource()` | INSERT/UPDATE | `mcp_resources` |
| `UpsertResourceTemplate()` | INSERT/UPDATE | `mcp_resource_templates` |
| `UpsertPrompt()` | INSERT/UPDATE | `mcp_prompts` |
| `CreateDependencyLink()` | INSERT | `mcp_dependency_links` |
| `DeleteDependencyLink()` | DELETE | `mcp_dependency_links` |
| `CreateOperation()` | INSERT | `mcp_operations` |
| `UpdateOperation()` | UPDATE | `mcp_operations` |
| `CreateOAuthSession()` | INSERT | `mcp_oauth_sessions` |
| `GetOAuthSession()` | SELECT | `mcp_oauth_sessions` |
| `CreateTask()` | INSERT | `mcp_tasks` |
| `UpdateTask()` | UPDATE | `mcp_tasks` |
| `CreateAuditLog()` | INSERT | `mcp_audit_logs` |

**覆盖表**: mcp_servers, mcp_server_scope_bindings, mcp_server_credentials, mcp_server_capabilities, mcp_tools, mcp_resources, mcp_resource_templates, mcp_prompts, mcp_dependency_links, mcp_operations, mcp_oauth_sessions, mcp_tasks, mcp_audit_logs (13 张)

---

## 二、跨 Repository 共享表

以下表被多个 Repository 同时操作：

### extension_artifacts

| Repository | 操作 | artifact_kind |
|---|---|---|
| `agent_skill_repository.go` | INSERT (blob: content_blob), UPDATE (归档), SELECT | `agent-skill` |
| `workshop_repository.go` | SELECT, JOIN | `workshop` |
| `package_repository.go` | INSERT/UPDATE | `package` |

**风险**: 三种不同子系统写入同一张表，`artifact_kind` 字符串枚举无数据库约束。

### extensions

| Repository | 操作 |
|---|---|
| `repository.go` | UpsertDefinition, SetEnabled, Delete |
| `agent_skill_repository.go` | SetAgentSkillEnabled, RemoveAgentSkill |

**风险**: 启停状态多写入者。

### extension_versions

| Repository | 操作 |
|---|---|
| `repository.go` | UpsertDefinition |
| `package_repository.go` | UpsertPackageVersion |
| `workshop_repository.go` (间接) | Workshop Install |

---

## 三、绕过 Repository 的直接 SQL

未发现绕过 Repository 的直接 SQL 操作，所有数据库访问均通过 Repository 层。

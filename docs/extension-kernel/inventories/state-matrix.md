# 状态判定矩阵（地图 L）

> 审计依据：.trae/Amitia_扩展系统重构_第2步_建立现有系统调用链地图.md 第四部分 地图 L
> 审计日期：2026-07-25
> 状态：第2步调用链地图（只审计不修改）

---

## 一、状态维度总览

一个扩展能力（Skill/MCP Tool/Plugin Skill/Workflow Skill）能否被模型看到并真正执行，由以下状态共同决定：

| 状态 | 持有者 | 数据库表 | 内存位置 | 写入入口 | 说明 |
|---|---|---|---|---|---|
| S1 Package Installed | PackageService | `extension_versions`、`extension_artifacts` | — | Install/Rollback | 包是否已安装 |
| S2 Extension Enabled | 各子系统 Service | `extension_records.enabled` | — | Enable/Disable API | 扩展记录全局启用 |
| S3 Scope Binding Enabled | Registry | `extension_scope_bindings.enabled` | `Registry.scopes` | SetScopeEnabled | 作用域绑定启用 |
| S4 Plugin State Enabled | PluginManager | `extension_plugin_lifecycle`（通过 UpdatePluginLifecycle） | `PluginManager.entries[].enabled` | Plugin Enable/Disable | 插件运行时启用 |
| S5 Agent Skill Enabled | AgentSkillService | `extension_agent_skill_metadata.enabled` | `Registry.items[].Definition.Enabled` | AgentSkill Enable/Disable | Agent Skill 启用 |
| S6 MCP Server Enabled | MCP Repository | `mcp_servers.enabled` | — | MCP Server Create/Enable | MCP Server 启用 |
| S7 MCP Character Binding | MCP Repository | `mcp_server_bindings` | — | SetScopeEnabled | MCP Server 角色绑定 |
| S8 MCP Tool Enabled | MCP Repository | `mcp_tool_definitions.enabled` | — | SetToolEnabled | 单个 MCP Tool 启用 |
| S9 Registry Scope Enabled | Registry | `extension_scope_bindings.enabled` | `Registry.scopes[scopeKey]` | SetScopeEnabled | Registry 作用域启用 |
| S10 Connection Ready | MCP Manager | `mcp_servers.state` | `MCPConnections.conns` | Connect/Reconnect | MCP 连接就绪 |
| S11 Compatible | 各子系统 | `extension_records.compatible` | `Registry.items[].Definition.Compatible` | 导入时分析 | 兼容性判定 |
| S12 Permission Granted | PermissionEvaluator | `extension_permission_grants` | — | UpdatePermissions | 权限授予 |
| S13 Discovery Done | MCP Discovery | `mcp_tool_definitions` 存在 | — | Discover | 工具已发现并入库 |
| S14 Plugin Circuit Closed | PluginManager | — | `pluginCircuit.state` | 断路器自动 | 断路器闭合允许调用 |
| S15 Round Active | AgentSkillService | `extension_agent_skill_activations` | `state.active[extID]` | Activate/EndRound | Agent Skill 当前 round 激活 |

---

## 二、能力可见性判定（模型是否能看到工具）

模型工具列表由 `Runtime.ModelTools`（runtime.go:137）生成，判定逻辑：

```text
工具可见 = S9(Registry Scope Enabled)
         AND S11(Compatible)
         AND S2(Extension Enabled)
         AND (
           工具来源为 LegacyToolAdapter（内置工具，始终可见，受 S12 权限过滤）
           OR 工具来源为 Workflow（S1+S2+S3+S11+S12）
           OR 工具来源为 Plugin Skill（S1+S2+S3+S4+S11+S12+S14）
           OR 工具来源为 MCP Tool（S6+S7+S8+S9+S10+S11+S13+S12）
           OR 工具来源为 Agent Skill Internal Tool（S5 任意 scope 有启用项 → agentSkillToolsAvailable=true）
         )
         AND S12(Permission PreviewExecution != Deny)
```

### 按工具来源的可见性矩阵

| 工具来源 | S1 Package | S2 Ext Enabled | S3 Scope Binding | S4 Plugin Enabled | S5 AgentSkill Enabled | S6 MCP Server | S7 MCP Binding | S8 MCP Tool | S9 Registry Scope | S10 Connection | S11 Compatible | S13 Discovery | S14 Circuit | S12 Permission | 可见 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| LegacyToolAdapter（内置） | — | — | — | — | — | — | — | — | — | — | — | — | — | — | Allow | ✓ |
| Workflow Skill | ✓ | ✓ | ✓ | — | — | — | — | — | ✓ | — | ✓ | — | — | — | Allow | ✓ |
| Plugin Skill | ✓ | ✓ | ✓ | ✓ | — | — | — | — | ✓ | — | ✓ | — | — | ✓ | Allow | ✓ |
| MCP Tool | — | ✓ | ✓ | — | — | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | — | — | Allow | ✓ |
| Agent Skill Internal（4 个） | — | — | — | — | ✓(任意 scope) | — | — | — | — | — | — | — | — | — | Allow | ✓ |

> **关键发现**：Agent Skill 的 4 个 internal 工具（`agent_skill_activate`/`list_resources`/`read_resource`/`get_asset`）可见性仅取决于 `agentSkillToolsAvailable`（runtime.go:140-143），该标志由 `AgentSkills.ResolveCatalog` 返回非空决定。**scope 隔离不严格**：任何 scope 有启用项则所有聊天会话都能看到 internal 工具（见 03-agent-skill.md P2-4）。

---

## 三、能力可执行性判定（模型调用工具能否真正执行）

模型调用工具后，由 `Runtime.ExecuteModelTool`（runtime.go:178）→ `Executor.Execute`（executor.go:40）执行，判定逻辑：

```text
可执行 = GetByModelName 找到 RegisteredSkill
       AND GetScoped(scope) 返回非 nil（S9 作用域存在）
       AND scoped.Definition.Enabled（S2/S3/S9 组合判定 Enabled）
       AND scoped.Definition.Compatible（S11）
       AND Trigger 匹配（LLM 触发/Manual 触发）
       AND 输入 Schema 校验通过
       AND 逐 capability EvaluateExecution != Deny（S12）
       THEN
         callHandler 执行
         AND 输出 Schema 校验
         AND RegisterOwnedSideEffects
         AND UpdateRun 审计
```

### 执行链状态检查矩阵

| 检查点 | 代码位置 | 检查状态 | 失败结果 |
|---|---|---|---|
| 1. 工具定位 | `registry.go:144` GetByModelName | modelName 存在于 Registry | `ErrSkillNotFound`，返回 `found=false` |
| 2. 作用域获取 | `registry.go` GetScoped | S9 Scope 存在 | `ErrSkillScopeForbidden` |
| 3. Enabled 检查 | `executor.go` | S2+S3+S9 组合 Enabled | `ErrSkillDisabled` |
| 4. Compatible 检查 | `executor.go` | S11 Compatible | `ErrSkillIncompatible` |
| 5. Trigger 检查 | `executor.go` | Trigger 匹配 | `ErrSkillTriggerMismatch` |
| 6. 配置获取 | `executor.go` GetEffectiveConfig | 配置存在（可为默认） | 使用默认配置 |
| 7. 输入 Schema | `executor.go` | JSON Schema 校验 | `ErrSkillInputInvalid` |
| 8. 权限评估 | `permission.go` EvaluateExecution | S12 逐 capability Allow | `ErrSkillPermissionDenied`，写 denied Run 审计 |
| 9. 幂等检查 | `executor.go:86-130` | Idempotent && idempotencyKey 命中缓存 | 返回缓存结果 |
| 10. Handler 执行 | `executor.go:250` callHandler | handlerSlots(64) 可用 + recover | panic recover，`ErrSkillExecutionFailed` |
| 11. 输出 Schema | `executor.go` | JSON Schema 校验 | `ErrSkillOutputInvalid` |
| 12. SideEffect | `executor.go` RegisterOwnedSideEffects | 资源归属校验 | SideEffect 记录失败 |
| 13. Run 审计 | `repository.go` CreateRun/UpdateRun | DB 写入 | 审计丢失（不阻断） |

---

## 四、MCP Tool 完整状态组合表

MCP Tool 的可见性与可执行性涉及最多状态维度，单独列出所有组合：

| S6 Server Enabled | S7 Character Binding | S8 Tool Enabled | S9 Registry Scope | S10 Connection Ready | S13 Discovery Done | S12 Permission | 模型可见 | 可执行 | 说明 |
|---|---|---|---|---|---|---|---|---|---|
| ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | Allow | ✓ | ✓ | 完全可用 |
| ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | Allow | ✗ | ✗ | Server 未启用 |
| ✓ | ✗ | ✓ | ✓ | ✓ | ✓ | Allow | ✗ | ✗ | 无角色绑定 |
| ✓ | ✓ | ✗ | ✓ | ✓ | ✓ | Allow | ✗ | ✗ | Tool 未启用 |
| ✓ | ✓ | ✓ | ✗ | ✓ | ✓ | Allow | ✗ | ✗ | Registry Scope 未启用 |
| ✓ | ✓ | ✓ | ✓ | ✗ | ✓ | Allow | ✗ | ✗ | 连接断开 |
| ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | Allow | ✗ | ✗ | 未发现工具 |
| ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | Deny | ✗ | ✗ | 权限拒绝 |
| ✓ | ✓ | ✓ | ✓ | ✗ | ✓ | Allow | ✗ | ✗ | **断线时工具残留**：Registry 中 SkillDefinition 仍在，模型可见但调用失败（见 04-mcp.md P1-1） |

> **关键发现**：S10 Connection Ready 与 Registry 注册项不同步。Server 断线后 `MCPConnections` 移除连接，但 `mcp/skill.Runtime` 未注销 Registry 中的 SkillDefinition，导致模型仍能看到工具但调用时返回 `ErrSkillNotFound`（GetToolBySkillID 查不到）。

---

## 五、状态双写/多写清单

| 编号 | 操作 | 写入的状态 | 写入位置 | 一致性风险 | 风险等级 |
|---|---|---|---|---|---|
| DW-1 | MCP Tool 启用 | S8（MCP Repository `mcp_tool_definitions.enabled`）+ S9（Registry Scope） | `mcpapi/router.go` + `extension/router.go` | 两个 API 入口分别写，可能不一致 | P1 |
| DW-2 | Agent Skill 启用 | S5（metadata.enabled）+ S2（extension_records.enabled）+ S9（Registry Scope） | `agent_skill_service.go:346` Enable | 三处写入，失败补偿不完整 | P2 |
| DW-3 | Plugin 启用 | S4（PluginManager.entries[].enabled）+ S2（extension_records.lifecycle）+ S9（Registry skills.SetEnabled） | `plugin_manager.go:180` Enable | 内存与 DB 双写，Registry 第三写 | P2 |
| DW-4 | Package 安装 | S1（extension_versions/artifacts）+ S2（extension_records）+ S9（Registry Register） | `package_installer.go` | 事务内写 DB，事务外写 Registry，失败补偿回滚 | P2 |
| DW-5 | Agent Skill 删除 | S5（metadata 软删）+ S2（extension_records 软删）+ S9（Registry Unregister）+ S15（round 清理）+ MCP link 删除 | `agent_skill_service.go:372` Remove | 多表+内存，Registry.Unregister 失败则 return 不删 DB | P1 |

---

## 六、状态恢复（Restore）矩阵

服务重启后各状态的恢复路径：

| 状态 | 是否持久化 | 恢复入口 | 恢复路径 | 失败策略 | 风险等级 |
|---|---|---|---|---|---|
| S1 Package Installed | ✓ DB | `PackageService.Restore` | `runtime.go:92` → 遍历 CurrentArtifacts → 重新 Register | **失败终止启动** | P2 |
| S2 Extension Enabled | ✓ DB | 各子系统 Restore | 读取 extension_records.enabled | — | — |
| S3 Scope Binding | ✓ DB | Registry Restore | 读取 extension_scope_bindings | — | — |
| S4 Plugin Enabled | ✓ DB | `PluginManager.Start` | `migrateStates` + `load` → 按 lifecycle 启用 | **失败仅 Warn** | P2 |
| S5 Agent Skill Enabled | ✓ DB | `AgentSkillService.Restore` | `runtime.go:63` → LoadAgentSkill → Register | **失败终止启动** | P2 |
| S6 MCP Server Enabled | ✓ DB | `MCPManager.Restore` | `services.go:318` → 遍历 servers → Connect | **失败仅 Warn** | P2 |
| S7 MCP Binding | ✓ DB | MCP Restore | 随 Server 恢复 | — | — |
| S8 MCP Tool Enabled | ✓ DB | MCP Discovery | Connect 成功后 Ready Handler → Discover → SyncTools | **失败仅 Warn** | P2 |
| S9 Registry Scope | ✓ DB + 内存 | 各子系统 Restore | Register + SetScopeEnabled | 依赖各子系统 Restore 成功 | P1 |
| S10 Connection Ready | ✗ 内存 | `MCPManager.Restore` | 遍历 enabled servers → Connect | **失败仅 Warn**，Server 保持 disconnected | P1 |
| S11 Compatible | ✓ DB | 导入时分析 | 读取 extension_records.compatible | — | — |
| S12 Permission | ✓ DB | — | 运行时按需读取 | — | — |
| S13 Discovery Done | ✓ DB | MCP Ready Handler | Connect → Discover → SyncTools | **失败仅 Warn** | P2 |
| S14 Circuit | ✗ 内存 | — | 重启后默认 Closed | — | P3 |
| S15 Round Active | ✗ 内存 | — | 重启后清空（round 不持久化） | — | P3 |

> **关键发现**：恢复失败策略不一致（见 01-startup-shutdown.md P2-SD-1）。AgentSkill.Restore 与 Package.Restore 失败终止启动；Workshop.Restore、MCP RegisterAll、MCP Restore 失败仅 Warn。这导致部分子系统恢复失败后功能静默缺失。

---

## 七、Agent Skill MCP 依赖状态流转

Agent Skill 的 MCP 依赖声明 → 安装 → 卸载的状态流转：

```text
导入期:
  Agent Skill 安装 → S5(enabled) + MCPDependencies(内存，未持久化 P0-2)
  
依赖安装期:
  Preview → Plan(风险评级)
  Install → S6(Server 创建/复用) + S7(绑定) + S8(Tool Allowlist) + S10(Connect) + S13(Discovery) + S9(Registry RegisterServer)
  OAuth 分支 → InstallStatus="awaiting_authorization" → 回调 → "installed"

删除期:
  Agent Skill Remove → S5(软删) + S9(Unregister)
  afterRemove → dependency.Uninstall → 仅删 mcp_dependency_links 表行
  ⚠️ S6/S7/S8/S9/S10/S13 均未清理（P0-1）
  ⚠️ MCP Server 保持 enabled，工具仍注册到 Registry，连接仍保持
```

---

## 八、关键结论

1. **能力可见性由 8-10 个状态共同决定**，分散在 5 个子系统和 2 个数据库表中，无统一判定入口。
2. **MCP Tool 状态维度最多（8 个）**，且 S10 Connection Ready 与 Registry 注册项不同步，导致断线后工具残留（P1）。
3. **存在 5 处状态双写/多写**，其中 MCP Tool 启用双写（DW-1）与 Agent Skill 删除多写（DW-5）风险最高。
4. **恢复失败策略不一致**：AgentSkill/Package 失败终止启动，Plugin/MCP 失败仅 Warn，可能导致功能静默缺失。
5. **Agent Skill MCPDependencies 未持久化**（P0-2），重启后丢失依赖声明，前端无法发起 Preview/Install。
6. **Agent Skill 删除不清理 MCP Server**（P0-1），导致 MCP Server、Registry 注册项、连接、Discovery 数据全部残留。
7. **Agent Skill Internal 工具 scope 隔离不严格**（P2-4），任何 scope 有启用项则所有聊天都能看到 4 个 internal 工具。

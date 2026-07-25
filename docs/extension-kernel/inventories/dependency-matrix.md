# 交叉依赖矩阵（地图 K）

> 审计依据：.trae/Amitia_扩展系统重构_第2步_建立现有系统调用链地图.md 第四部分 地图 K
> 审计日期：2026-07-25
> 状态：第2步调用链地图（只审计不修改）

---

## 一、依赖类型图例

| 标记 | 含义 |
|---|---|
| `→` | 正向依赖（A 正常调用 B） |
| `←` | 反向依赖（A 被迫知道 B，或 B 回调 A） |
| `↔` | 双向依赖（循环依赖风险） |
| `[装配]` | 依赖由装配层（services.go）注入，非子系统自身声明 |
| `[桥接]` | 不可直接删除的桥接点（删除会导致断链） |
| `[兼容]` | 可删除的兼容层 |
| `[隐式]` | 隐式全局状态依赖 |

---

## 二、子系统依赖总矩阵

| 依赖方（行） \ 被依赖方（列） | Extension Runtime | Registry | Executor | Chat | Memory | Delivery | MCP Manager | MCP Skill Runtime | MCP Dependency | Discovery | Plugin Manager | Workflow Host | Agent Skill Service | Package Service | Workshop | Repository(DB) | Permission |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **Extension Runtime** | — | 持有 | 持有 | — | — | — | — | — | — | — | 持有+Start | 持有 | 持有+Restore | 持有+Restore | 持有+Restore | 持有 | 持有 |
| **Chat Service** | `→` SetSkillRuntime | — | — | — | `→` | `→` | — | — | — | — | — | — | — | — | — | `→` | — |
| **Registry** | — | — | — | — | — | — | — | — | — | — | — | — | — | — | — | — | — |
| **Executor** | — | `→` GetScoped | — | — | — | — | — | — | — | — | — | — | — | — | — | `→` CreateRun | `→` Evaluate |
| **MCP Skill Runtime** | `←[装配]` 反向依赖 | `→` Register | — | — | — | — | `→` Connections | — | — | `→` Discover | — | — | — | — | — | `→` | — |
| **MCP Dependency Service** | — | — | — | — | — | — | `→` Connect | `→` RegisterServer | — | `→` Discover | — | — | `←[装配]` afterRemove 回调 | — | — | `→` | — |
| **MCP API Router** | `←[装配]` 注入 Extension | — | — | — | — | — | `→` | `→` | `→` | `→` | — | — | — | — | — | `→` | — |
| **Plugin Manager** | — | `→` SetEnabled/Unregister | `→`（通过 skills） | — | — | — | — | — | — | — | — | — | — | — | — | `→` | — |
| **Plugin Host** | — | `→` RegisterSkill/CallSkill | — | — | — | — | — | — | — | — | `→` persistEvent | — | — | — | — | `→` | — |
| **Workflow Host Adapter** | — | `→` Get | `→` Execute | `←[装配]` 注入 | `←[装配]` 注入 | `←[装配]` 注入 | — | — | — | — | — | — | — | — | — | — | — |
| **Workflow Executor** | — | — | — | — | — | — | — | — | — | — | — | — | — | — | — | `→` CreateRun | — |
| **Agent Skill Service** | — | `→` Register/SetScopeEnabled | — | — | — | — | — | — | `←[装配]` SetAfterRemove | — | — | — | — | — | — | `→` | — |
| **Agent Skill Runtime** | — | `→`（通过 Executor） | `→` Execute | — | — | — | — | — | — | — | — | — | `→` Activate/ReadResource | — | — | — | — |
| **Package Service** | — | `→` Register/Unregister | — | — | — | — | — | — | — | — | — | — | `→` Install/Remove | — | — | `→` | — |
| **Package Installer** | — | `→` Register | — | — | — | — | — | — | — | — | — | `→` Compile | `→` Install | — | — | `→` | — |
| **Workshop Service** | — | `→`（通过 WorkshopInstaller.Install 直接写 Registry） | — | `←[装配]` SetModelGenerator | — | — | — | — | — | — | — | `→` Compile | `→` storePreview（仅 Instructions 分支） | 反向无引用（PackageService 持有 WorkshopInstaller.definitionFromArtifact/workflowHandler，Workshop Service 不调 PackageService） | — | `→` | — |
| **Extension API Router** | — | — | — | — | — | — | — | — | — | — | — | — | — | — | — | — | — |

---

## 三、反向依赖清单（重点标记）

反向依赖指子系统 A 被迫知道或回调子系统 B，违反单向依赖原则。

| 编号 | 依赖方 | 被依赖方 | 注入位置 | 注入方式 | 影响链路 | 风险等级 | 是否可删除桥接点 |
|---|---|---|---|---|---|---|---|
| RD-1 | MCP Skill Runtime | Extension Runtime | `services.go:297` `mcpskill.New(repo, connMgr, extRuntime)` | 构造函数参数 | MCP 工具注册到 Extension Registry | P1 | 否[桥接]：MCP 工具必须进入 Registry 才能被模型看到 |
| RD-2 | Agent Skill Service | MCP Dependency Service | `services.go:302` `ext.AgentSkills.SetAfterRemove(dep.Uninstall)` | 回调函数注入 | Agent Skill 删除时清理 MCP 依赖 | P0 | 否[桥接]：删除 Agent Skill 需联动清理 MCP Server |
| RD-3 | Workflow Host Adapter | Chat/Memory/Delivery | `services.go:389-470` `configureWorkflowHost` | 闭包注入 4 个业务函数 | Workflow 节点调用业务能力 | P1 | 否[桥接]：Workflow Host 函数依赖业务服务 |
| RD-4 | Workshop Service | Chat Service | `services.go:172` `ext.Workshop.SetModelGenerator(chatSvc)` | 接口注入 | Workshop AI 生成指令需要调用模型 | P1 | 否[桥接]：Workshop 生成依赖模型能力 |
| RD-5 | MCP API Router | Extension Runtime | `cmd/server/router.go:92` `mcpapi.RegisterRouter(..., services.Extension)` | 路由注册参数 | MCP API 反向操作 Extension Registry | P2 | 部分：MCP Tool 启停可统一到 Extension API |
| RD-6 | Chat Service | Extension Runtime | `services.go:171` `chatSvc.SetSkillRuntime(extRuntime)` | Setter 注入 | 聊天流程调用工具执行 | P1 | 否[桥接]：聊天必须能执行工具 |
| RD-7 | Workflow Host (Schedule) | Extension Registry | `services.go:400` `Registry.Get("dev.amitia.skill.create-schedule")` | Registry 查询 | Schedule Host 回到 Executor 链路 | P0 | 否[桥接]：但存在循环依赖风险（Host→Handler→Executor→Host） |

---

## 四、循环依赖标记

| 编号 | 循环路径 | 触发条件 | 风险等级 | 说明 |
|---|---|---|---|---|
| CD-1 | Workflow Host → Registry.Get(create-schedule) → Executor.Execute → WorkflowExecutor → call_skill → Executor.Execute → Workflow Host | Workflow 中调用 `create_schedule` 节点且该 Skill 为 Workflow 类型 | P0 | `services.go:400` 通过 Registry 回到 Executor；`SkillWorkflowAdapter.Execute`（workflow_executor.go:476）递归调用 `Executor.Execute`。当前靠 `MaxSkillCallDepth` 与 `MaxSkillCalls` 限制，无显式调用栈审计。 |
| CD-2 | Agent Skill Service → MCP Dependency Service.Uninstall → (仅删 link 行，不断链) | Agent Skill 删除 | P0 | `afterRemove` 回调 `dep.Uninstall`，但 `Uninstall` 仅删 `mcp_dependency_links` 表行，不调 Disconnect/DeleteServer/UnregisterServer，导致 MCP Server 与 Registry 注册项残留（见 03-agent-skill.md P0-1）。非严格循环，但存在清理不完整导致的隐式残留链。 |

---

## 五、装配层泄漏清单

装配层（`services.go`）直接知晓并组装的子系统内部组件：

| 编号 | 泄漏内容 | 代码位置 | 影响子系统 | 风险等级 |
|---|---|---|---|---|
| AL-1 | MCP 全部 11 个组件（repo/factory/cfg/connMgr/discovery/skillRuntime/depService/host/features/oauth）手动创建与注入 | `services.go:288-314` | MCP | P1 |
| AL-2 | Workflow Host 4 个业务函数（Schedule/Notification/MemoryCandidate/ContextContribution）闭包注入 | `services.go:389-470` | Workflow | P1 |
| AL-3 | Agent Skill 删除回调注入 MCP Dependency | `services.go:302` | Agent Skill + MCP | P0 |
| AL-4 | Workshop 模型生成器注入 Chat Service | `services.go:172` | Workshop + Chat | P1 |
| AL-5 | Chat Service 注入 Extension Runtime | `services.go:171` | Chat + Extension | P1 |
| AL-6 | MCP Ready Handler 链（Attach→Discover→RegisterServer）在装配层串联 | `services.go:307-312` | MCP | P1 |
| AL-7 | MCP RegisterAll + Restore 在装配层调用 | `services.go:315-319` | MCP | P2 |

---

## 六、隐式全局状态依赖

| 编号 | 状态 | 持有者 | 访问者 | 生命周期 | 风险等级 |
|---|---|---|---|---|---|
| GS-1 | 内存 Registry（items map） | `extension.Registry` | Runtime/Executor/MCP Skill/Plugin/Package/Agent Skill | 进程内，重启靠 Restore 重建 | P1 |
| GS-2 | Agent Skill Round State（rounds map） | `AgentSkillService.state` | Agent Skill Service（同一进程） | 单次聊天 round，`EndAgentSkillRound` 清理 | P2 |
| GS-3 | Agent Skill Catalogs 缓存 | `AgentSkillService.catalogs` | Agent Skill Service | 进程内，`invalidateAgentSkillCaches` 清空 | P2 |
| GS-4 | Agent Skill Artifacts 缓存 | `AgentSkillService.artifacts` | Agent Skill Service | 进程内，LRU 淘汰 | P3 |
| GS-5 | Plugin afterReplyQ / eventIngress / eventWake channel | `PluginManager` | PluginManager Worker | 进程内，Stop 时关闭 | P2 |
| GS-6 | Plugin entries（运行时实例 map） | `PluginManager.entries` | PluginManager | 进程内，Stop 时清空 | P2 |
| GS-7 | MCP Connections（serverID→Connection map） | `MCPConnections` | MCP Manager/Skill Runtime | 进程内，Close 时清理 | P1 |
| GS-8 | MCP reconnecting 标志 map | `MCPManager.reconnecting` | MCP Manager | 进程内 | P3 |
| GS-9 | Registry modelNames map（冲突检测） | `Registry.modelNames` | Registry.Register | 进程内，重启重建 | P2 |
| GS-10 | PluginManager accepting 标志 | `PluginManager.accepting` | PluginManager | 进程内 | P3 |
| GS-11 | agentSkillMetricsSnapshot（指标） | 全局变量 | Agent Skill Metrics API | 进程内，不持久化 | P3 |
| GS-12 | Preview 缓存（AgentSkill/Workshop 共享） | `AgentSkillService.previews` | Agent Skill + Workshop | 30min TTL | P2 |

---

## 七、可删除兼容层

| 编号 | 兼容层 | 文件 | 替代方 | 风险等级 | 说明 |
|---|---|---|---|---|---|
| CL-1 | LegacyToolAdapter | `legacy_tool_adapter.go` | 新 Tool/Capability 模型 | P2 | 旧工具适配为 SkillDefinition，handler 包装旧工具调用；重构后应由统一 Tool 模型替代 |
| CL-2 | ExtensionCenterView 旧扩展中心 | `front/.../ExtensionCenterView.vue` | Extension Kernel 新入口 | P3 | 已标记 Deprecated，仅保留兼容 |
| CL-3 | 旧扩展路由 `/extensions/*` | `front/.../router/index.ts:39-49` | 新路由 | P3 | 已标记 Deprecated |

---

## 八、不可直接删除的桥接点

| 编号 | 桥接点 | 文件 | 连接的系统 | 删除后果 | 风险等级 |
|---|---|---|---|---|---|
| BP-1 | `Runtime.ModelTools` | `runtime.go:137` | Chat ↔ Registry/MCP/Plugin/AgentSkill | 模型无法看到任何工具 | P0 |
| BP-2 | `Runtime.ExecuteModelTool` | `runtime.go:178` | Chat ↔ Registry/Executor/MCP | 模型无法执行工具 | P0 |
| BP-3 | `Runtime.BeforePrompt` | `runtime.go:105` | Chat ↔ Plugin/AgentSkill | Prompt 无插件贡献与 Agent Skill 指令 | P0 |
| BP-4 | `Runtime.AfterReply` | `runtime.go:116` | Chat ↔ Plugin | 回复后无插件 Hook | P0 |
| BP-5 | `mcp/skill.Runtime.RegisterServer` | `mcp/skill/runtime.go:92` | MCP ↔ Extension Registry | MCP 工具不注册到 Registry | P0 |
| BP-6 | `AgentSkillService.SetAfterRemove` | `services.go:302` | AgentSkill ↔ MCP Dependency | Agent Skill 删除不清理 MCP | P0 |
| BP-7 | `configureWorkflowHost` | `services.go:389` | Workflow ↔ Chat/Memory/Delivery | Workflow 无法调用业务能力 | P0 |
| BP-8 | `chatSvc.SetSkillRuntime` | `services.go:171` | Chat ↔ Extension | 聊天无法执行工具 | P0 |
| BP-9 | `PluginManager.skills`（Registry 引用） | `plugin_manager.go` | Plugin ↔ Registry | 插件 Skill 贡献无法注册 | P0 |
| BP-10 | `PackageInstaller` 调用 `Registry.Register` | `package_installer.go` | Package ↔ Registry | 安装后不注册 | P0 |

---

## 九、依赖方向性总结

```text
装配层(services.go)
  ├─→ Chat Service ←→ Extension Runtime（双向：Chat 调用 Runtime，Runtime 不回调 Chat）
  ├─→ Extension Runtime
  │    ├─→ Registry（持有）
  │    ├─→ Executor（持有）
  │    ├─→ PluginManager（持有+Start）→ Registry/Executor
  │    ├─→ AgentSkillService（持有+Restore）→ Registry ←[反向] MCP Dependency
  │    ├─→ Workshop（持有+Restore）→ Chat[反向] / Compiler / PackageService
  │    └─→ PackageService（持有+Restore）→ Registry / AgentSkillService / WorkflowCompiler
  ├─→ MCP Manager（独立创建）
  │    ├─→ MCP Skill Runtime ←[反向] Extension Runtime
  │    ├─→ MCP Dependency Service ←[反向] Agent Skill afterRemove
  │    ├─→ MCP Discovery
  │    └─→ MCP Host ←[反向] Chat（Sampling）
  ├─→ Workflow Host Adapter ←[反向] Chat/Memory/Delivery + Registry(Schedule)
  └─→ Router
       ├─→ Extension API → Extension Runtime
       └─→ MCP API ←[反向] Extension Runtime
```

---

## 十、关键结论

1. **装配层是所有反向依赖的汇聚点**：7 处反向依赖中 6 处由 `services.go` 注入，重构装配层是解除耦合的关键。
2. **存在 1 个 P0 循环依赖**（CD-1 Workflow Schedule Host → Registry → Executor → WorkflowExecutor），需在第 3 步重构时打断。
3. **存在 1 个 P0 清理不完整链**（CD-2 Agent Skill 删除不清理 MCP Server），需在 `dependency.Service.Uninstall` 中补全。
4. **10 个不可删除桥接点**构成系统骨架，重构必须逐一提供替代方案后才能移除。
5. **3 个可删除兼容层**（LegacyToolAdapter、旧扩展中心、旧路由）可在新 Extension Kernel 就绪后移除。
6. **12 项隐式全局状态**中，Registry（GS-1）与 MCP Connections（GS-7）风险最高，重启恢复依赖 Restore 链完整性。

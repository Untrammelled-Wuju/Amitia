# 前端扩展中心调用链地图

> 审计依据：`.trae/Amitia_扩展系统重构_第2步_建立现有系统调用链地图.md` Task 9（地图 J）
> 审计日期：2026-07-25
> 状态：第2步调用链地图（只审计不修改）

---

## 一、涉及文件

| 文件 | 职责 |
|---|---|
| `front/src/router/index.ts` | 前端路由表（11 条扩展路由 L39-49） |
| `front/src/navigation/app-nav.ts` | 主导航 `desktopNavGroups` + 页面标题 `extraTitles` |
| `front/src/views/extensions/ExtensionCenterView.vue` | 扩展中心首页（6 卡片导航） |
| `front/src/views/extensions/api.ts` | 扩展中心 API Client（约 60 个函数） |
| `front/src/views/extensions/types.ts` | 扩展中心类型 |
| `front/src/views/mcp/api.ts` | MCP API Client（约 30 个函数） |
| `front/src/App.vue` | MCPInteractionGuard 全局挂载 |
| `front/src/components/ChatInput.vue` | 聊天输入，直接依赖 Agent Skill API |
| 各业务 Vue 视图 | 见页面矩阵 |
| `backend/internal/extension/router.go` | 后端 `/api/extensions/*` 路由（L12-105） |
| `backend/internal/mcpapi/router.go` | 后端 `/api/mcp/*` 路由 |

---

## 二、页面矩阵（地图 J 核心）

| 页面 | 路由 name | 路由 path | 主导航入口 | 首页卡片 | API 文件 | 后端路由前缀 | 实际系统 |
|---|---|---|---|---|---|---|---|
| 扩展中心首页 | extensionCenter | `/extensions` | ❌ 无 | — | — | — | 入口聚合页 |
| MCP 服务 | extensionMCP | `/extensions/mcp` | ❌ 无 | ✅ | `views/mcp/api.ts` | `/api/mcp/*` | MCP |
| 扩展包管理 | extensionPackages | `/extensions/packages` | ❌ 无 | ❌ 未链接 | `views/extensions/api.ts` | `/api/extensions/packages/*`、`/:id/*` | Package |
| 技能管理 | extensionSkills | `/extensions/skills` | ❌ 无 | ✅ | `views/extensions/api.ts` | `/api/extensions/skills/*` | Skill（含旧工具/Workflow/Agent Skill 汇总） |
| 技能详情 | extensionSkillDetail | `/extensions/skills/:id` | ❌ 无 | — | `views/extensions/api.ts` | `/api/extensions/skills/:id/*` | Skill |
| Agent Skills | extensionAgentSkills | `/extensions/agent-skills` | ❌ 无 | ✅ | `views/extensions/api.ts` | `/api/extensions/agent-skills/*` | Agent Skill |
| 插件管理 | extensionPlugins | `/extensions/plugins` | ❌ 无 | ✅ | `views/extensions/api.ts` | `/api/extensions/plugins/*` | Plugin |
| 插件详情 | extensionPluginDetail | `/extensions/plugins/:id` | ❌ 无 | — | `views/extensions/api.ts` | `/api/extensions/plugins/:id/*` | Plugin |
| 技能制作 | extensionWorkshop | `/extensions/workshop` | ❌ 无 | ✅ | `views/extensions/api.ts` | `/api/extensions/workshop/*` | Workshop |
| 制作会话 | extensionWorkshopSession | `/extensions/workshop/:id` | ❌ 无 | — | `views/extensions/api.ts` | `/api/extensions/workshop/sessions/:id/*` | Workshop |
| 执行记录 | extensionRuns | `/extensions/runs` | ❌ 无 | ✅ | `views/extensions/api.ts` | `/api/extensions/runs/*` | Executor Run |

> 已确认：**全部 11 条扩展路由均无主导航入口**（`desktopNavGroups` 仅含聊天/概览/创意工坊/微信/QQ/运行时调试/决策可视化/设置）。扩展中心仅通过 `extraTitles`（页面标题）和 `ExtensionCenterView` 首页卡片可达；用户需直接输入 `/extensions` URL 或经其他页面跳转进入。

---

## 三、API 路由矩阵（前端 API → 后端 Handler → 实际系统）

### 1. 扩展中心 API（`/api/extensions/*`，后端 `extension/router.go`）

| 前端函数 | HTTP | 后端路由 | 后端 Handler | 实际系统 |
|---|---|---|---|---|
| fetchSkills | GET | `/skills` | `handler.ListSkills` | Skill Registry |
| fetchSkill | GET | `/skills/:id` | `handler.GetSkill` | Skill Registry |
| setSkillEnabled | POST | `/skills/:id/enable`/`disable` | `handler.EnableSkill`/`DisableSkill` | Skill Registry+Repository |
| fetchCapabilities | GET | `/capabilities` | `handler.Capabilities` | Capability 定义 |
| updatePermissions | PUT | `/skills/:id/permissions` | `handler.UpdatePermissions` | Permission |
| updateConfig | PUT | `/skills/:id/config` | `handler.UpdateConfig` | Repository |
| resetConfig | POST | `/skills/:id/config/reset` | `handler.ResetConfig` | Repository |
| executeSkill | POST | `/skills/:id/execute` | `handler.Execute` | Executor |
| fetchRuns | GET | `/runs` | `handler.ListRuns` | Repository Run |
| fetchRun | GET | `/runs/:runId` | `handler.GetRun` | Repository Run |
| fetchPlugins | GET | `/plugins` | `handler.ListPlugins` | PluginManager |
| fetchPlugin | GET | `/plugins/:id` | `handler.GetPlugin` | PluginManager |
| setPluginEnabled | POST | `/plugins/:id/enable`/`disable` | `handler.EnablePlugin`/`DisablePlugin` | PluginManager |
| reloadPlugin | POST | `/plugins/:id/reload` | `handler.ReloadPlugin` | PluginManager |
| updatePluginConfig | PUT | `/plugins/:id/config` | `handler.UpdatePluginConfig` | PluginRepository |
| resetPluginConfig | POST | `/plugins/:id/config/reset` | `handler.ResetPluginConfig` | PluginRepository |
| updatePluginPermissions | PUT | `/plugins/:id/permissions` | `handler.UpdatePluginPermissions` | Permission |
| fetchPluginHealth | GET | `/plugins/:id/health` | `handler.GetPluginHealth` | PluginManager |
| resetPluginCircuit | POST | `/plugins/:id/circuit/reset` | `handler.ResetPluginCircuit` | PluginManager 熔断 |
| fetchPluginState | GET | `/plugins/:id/state` | `handler.GetPluginState` | PluginRepository |
| fetchPluginSurface | GET | `/plugins/:id/surface` | `handler.GetPluginSurface` | Plugin Surface |
| fetchPluginSchedules | GET | `/plugins/:id/schedules` | `handler.GetPluginSchedules` | Plugin Schedule |
| setPluginScheduleEnabled | POST | `/plugins/:id/schedules/:sid/pause`/`resume` | `handler.PausePluginSchedule`/`ResumePluginSchedule` | Plugin Schedule |
| fetchPluginEvents | GET | `/plugins/:id/events` | `handler.GetPluginEvents` | Plugin Event |
| retryPluginEvent | POST | `/plugins/:id/events/:eid/retry` | `handler.RetryPluginEvent` | Plugin Event |
| executePluginAction | POST | `/plugins/:id/surface/actions/:aid` | `handler.ExecutePluginSurfaceAction` | Plugin Surface |
| fetchWorkshopSessions | GET | `/workshop/sessions` | `workshopHandler.ListSessions` | Workshop |
| createWorkshopSession | POST | `/workshop/sessions` | `workshopHandler.CreateSession` | Workshop |
| fetchWorkshopSession | GET | `/workshop/sessions/:id` | `workshopHandler.GetSession` | Workshop |
| archiveWorkshopSession | POST | `/workshop/sessions/:id/archive` | `workshopHandler.Archive` | Workshop |
| generateWorkshopDraft | POST | `/workshop/sessions/:id/generate` | `workshopHandler.Generate` | Workshop Generator（依赖 Chat） |
| validateWorkshopDraft | POST | `/workshop/sessions/:id/revisions/:r/validate` | `workshopHandler.Validate` | Workshop |
| confirmWorkshopPermissions | POST | `/workshop/sessions/:id/revisions/:r/permissions/confirm` | `workshopHandler.ConfirmPermissions` | Workshop |
| testWorkshopDraft | POST | `/workshop/sessions/:id/revisions/:r/test` | `workshopHandler.Test` | Workshop TestRunner |
| installWorkshopDraft | POST | `/workshop/sessions/:id/revisions/:r/install` | `workshopHandler.Install` | WorkshopInstaller→Registry |
| forkWorkflowSkill | POST | `/skills/:id/workshop/fork` | `workshopHandler.Fork` | Workshop |
| rollbackWorkflowSkill | POST | `/skills/:id/versions/:v/rollback` | `workshopHandler.Rollback` | Workshop |
| generateWorkshopInstruction | POST | `/workshop/instructions/generate` | `workshopHandler.GenerateInstruction` | Workshop Generator |
| previewAgentSkillZIP | POST | `/agent-skills/import/preview` | `agentSkillHandler.Preview` | Agent Skill Parser |
| previewAgentSkillDirectory | POST | `/agent-skills/import/preview` | `agentSkillHandler.Preview` | Agent Skill Parser |
| installAgentSkill | POST | `/agent-skills/import/install` | `agentSkillHandler.Install` | Agent Skill Service |
| fetchAgentSkills | GET | `/agent-skills` | `agentSkillHandler.List` | Agent Skill Service |
| fetchAgentSkill | GET | `/agent-skills/:id` | `agentSkillHandler.Get` | Agent Skill Service |
| setAgentSkillEnabled | POST | `/agent-skills/:id/enable`/`disable` | `agentSkillHandler.Enable`/`Disable` | Agent Skill Service |
| removeAgentSkill | DELETE | `/agent-skills/:id` | `agentSkillHandler.Remove` | Agent Skill Service（→AfterRemove→MCP Dependency） |
| previewAgentSkillMCPDependencies | POST | `/agent-skills/:id/...` | `agentSkillHandler` 系列 | MCP Dependency |
| installAgentSkillMCPDependencies | POST | `/agent-skills/:id/...` | `agentSkillHandler` 系列 | MCP Dependency |
| removeAgentSkillMCPDependencies | DELETE | `/agent-skills/:id/...` | `agentSkillHandler` 系列 | MCP Dependency |
| previewExtensionPackage | POST | `/packages/import/preview` | `packageHandler.Preview` | Package Parser |
| previewExtensionDirectory | POST | `/packages/import/preview` | `packageHandler.Preview` | Package Parser |
| installExtensionPackage | POST | `/packages/import/install` | `packageHandler.Install` | Package Installer |
| fetchPackageVersions | GET | `/:id/versions` | `packageHandler.Versions` | Package Lifecycle |
| comparePackageVersions | GET | `/:id/versions/compare` | `packageHandler.Compare` | Package Lifecycle |
| exportExtensionPackage | POST | `/:id/export` | `packageHandler.Export` | Package Lifecycle |
| downloadExtensionPackage | GET | `/:id/exports/:eid` | `packageHandler.Download` | Package Lifecycle |
| rollbackExtensionPackage | POST | `/:id/versions/:v/rollback` | `packageHandler.Rollback` | Package Recovery |
| fetchPackageDependencies | GET | `/:id/dependencies` | `packageHandler.Dependencies` | Package Lifecycle |
| previewPackageUninstall | GET | `/:id/uninstall/preview` | `packageHandler.PreviewUninstall` | Package Lifecycle |
| uninstallExtensionPackage | DELETE | `/:id` | `packageHandler.Uninstall` | Package Lifecycle |
| fetchPackageOperations | GET | `/package-operations` | `packageHandler.Operations` | Package Lifecycle |
| fetchPackageSigners | GET | `/signers` | `packageHandler.Signers` | Package 签名 |

### 2. MCP API（`/api/mcp/*`，后端 `mcpapi/router.go`）

| 前端函数 | HTTP | 后端路由 | 实际系统 |
|---|---|---|---|
| listMCPServers | GET | `/servers` | MCP Repository |
| createMCPServer | POST | `/servers` | MCP Repository+Secret |
| updateMCPServer | PUT | `/servers/:id` | MCP Repository |
| deleteMCPServer | DELETE | `/servers/:id` | MCP Repository（不清理 Registry，见 MCP P1-1） |
| connectMCPServer | POST | `/servers/:id/connect` | MCP Manager.Connect |
| disconnectMCPServer | POST | `/servers/:id/disconnect` | MCP Manager.Disconnect |
| reconnectMCPServer | POST | `/servers/:id/reconnect` | MCP Manager.Reconnect |
| refreshMCPServer | POST | `/servers/:id/refresh` | MCP Discovery |
| listMCPTools | GET | `/servers/:id/tools` | MCP Repository ToolDefinition |
| setMCPToolEnabled | PUT | `/servers/:id/tools/:tool/enabled` | MCP Repository+Registry Scope |
| listMCPResources | GET | `/servers/:id/resources` | MCP Features |
| readMCPResource | POST | `/servers/:id/resources/read` | MCP Features |
| subscribeMCPResource | POST | `/servers/:id/resources/subscribe` | MCP Features |
| listMCPPrompts | GET | `/servers/:id/prompts` | MCP Features |
| getMCPPrompt | POST | `/servers/:id/prompts/get` | MCP Features |
| completeMCPArgument | POST | `/servers/:id/completion` | MCP Features |
| listMCPLogs | GET | `/servers/:id/logs` | MCP Repository |
| listMCPTasks | GET | `/servers/:id/tasks` | MCP Host |
| cancelMCPTask | POST | `/servers/:id/tasks/:tid/cancel` | MCP Host |
| listMCPCapabilities | GET | `/servers/:id/capabilities` | MCP Repository |
| setMCPCapability | PUT | `/servers/:id/capabilities/:cap` | MCP Repository |
| startMCPOAuth | POST | `/servers/:id/oauth/start` | MCP Auth |
| revokeMCPOAuth | POST | `/servers/:id/oauth/revoke` | MCP Auth |
| listMCPInteractions | GET | `/interactions` | MCP Host Broker |
| resolveMCPInteraction | POST | `/interactions/:iid/resolve` | MCP Host Broker |

---

## 四、关键前端集成点

### 1. MCPInteractionGuard 全局组件

| 项 | 结论 |
|---|---|
| 挂载位置 | `front/src/App.vue` |
| 作用 | 全局拦截 MCP 交互确认 |
| 证据 | `rg MCPInteractionGuard` 仅命中 `App.vue` |

### 2. ChatInput 直接依赖 Agent Skill API

| 项 | 结论 |
|---|---|
| 文件 | `front/src/components/ChatInput.vue` |
| 依赖 | 引用 `agent-skill`/`agentSkill`/`AgentSkill` |
| 影响 | 聊天输入组件直接耦合 Agent Skill API，非扩展中心页面 |

### 3. Skill 与 MCP Tool 展示分离

| 项 | 结论 |
|---|---|
| SkillListView/SkillDetailView | **不调用** `/api/mcp/*`（rg 确认无命中） |
| MCP Tool 展示 | 仅在 `MCPServerView.vue` 内展示 |
| 结论 | 前端未把 MCP Tool 当 Skill 展示；但后端 Registry 中 MCP Tool 与 Skill 共存，技能管理页面展示的 Skill 列表实际包含 MCP Tool（待确认 ListSkills 是否过滤 Source） |

### 4. 同一能力跨两个 API 命名空间

| 能力 | 扩展中心 API | MCP API | 说明 |
|---|---|---|---|
| 启用/禁用工具 | `/api/extensions/skills/:id/enable`（Registry Scope） | `/api/mcp/servers/:id/tools/:tool/enabled` | MCP Tool 启用走 MCP API，但 Registry Scope 启用走 Extension API，存在状态双写（见状态矩阵） |
| Agent Skill MCP 依赖 | `/api/extensions/agent-skills/:id/...` | `/api/mcp/servers/*` | Agent Skill 依赖管理跨两个命名空间 |

---

## 五、Mermaid 图

### 地图 J：前端到后端 API 图（frontend-api.mmd）

```mermaid
flowchart LR
    Nav[主导航 desktopNavGroups<br/>app-nav.ts]:::nav
    Nav -.->|无扩展入口| Note[扩展中心无主导航入口]:::risk

    URL[/直接输入 /extensions]:::nav
    URL --> EC[ExtensionCenterView<br/>6 卡片]:::page

    EC -->|卡片| MCPPage[MCPServerView<br/>/extensions/mcp]:::page
    EC -->|卡片| ASPage[AgentSkillListView<br/>/extensions/agent-skills]:::page
    EC -->|卡片| PLGPage[PluginListView<br/>/extensions/plugins]:::page
    EC -->|卡片| WSPage[WorkshopListView<br/>/extensions/workshop]:::page
    EC -->|卡片| SKPage[SkillListView<br/>/extensions/skills]:::page
    EC -->|卡片| RUNPage[RunHistoryView<br/>/extensions/runs]:::page
    EC -.->|未链接| PKGPage[PackageManagerView<br/>/extensions/packages]:::risk

    MCPPage -->|api.ts| MCPAPI["/api/mcp/*"]:::api
    ASPage -->|api.ts| ExtAPI["/api/extensions/*"]:::api
    PLGPage -->|api.ts| ExtAPI
    WSPage -->|api.ts| ExtAPI
    SKPage -->|api.ts| ExtAPI
    RUNPage -->|api.ts| ExtAPI
    PKGPage -->|api.ts| ExtAPI

    MCPAPI --> MCPRouter[mcpapi/router.go]:::be
    ExtAPI --> ExtRouter[extension/router.go]:::be

    ChatInput[ChatInput.vue<br/>聊天输入]:::chat -->|直接依赖| ExtAPI
    AppVue[App.vue<br/>MCPInteractionGuard]:::chat -->|全局| MCPAPI

    classDef nav fill:#e1f5ff,stroke:#0288d1
    classDef page fill:#e8f5e9,stroke:#388e3c
    classDef api fill:#fff4e1,stroke:#f57c00
    classDef be fill:#f3e5f5,stroke:#7b1fa2
    classDef chat fill:#fce4ec,stroke:#c2185b
    classDef risk fill:#ffebee,stroke:#c62828
```

---

## 六、关键发现与风险

### P0（重构阻塞）

- 无。

### P1（高风险历史债务）

- **P1-FE-1 扩展中心全部页面无主导航入口**：`desktopNavGroups`（`app-nav.ts:27-84`）不含任何 `/extensions/*` 项，11 条扩展路由仅靠 `extraTitles` 标题与 `ExtensionCenterView` 卡片可达。证据：`app-nav.ts:27-84`、`router/index.ts:39-49`。影响：扩展中心可达性差，用户需知 URL；重构前端入口时易遗漏。建议：第4步划分范围时将"扩展中心入口聚合"列为保留项。
- **P1-FE-2 扩展包管理页面有路由但首页未链接**：`ExtensionCenterView.vue:39-76` 的 6 个卡片不含 `/extensions/packages`。证据：`ExtensionCenterView.vue:39-76`、`router/index.ts:41`。影响：扩展包管理页面仅能通过 URL 或其他页面跳转进入，属于"后端有能力但前端入口缺失"。建议：确认是否设计如此，或补入口。
- **P1-FE-3 MCP Tool 启用状态双写**：前端 MCP Tool 启用走 `/api/mcp/servers/:id/tools/:tool/enabled`（MCP Repository），而 Registry Scope 启用走 `/api/extensions/skills/:id/enable`（Extension Registry+Repository）。同一 MCP Tool 的启用状态存在两个写入入口。证据：`mcp/api.ts:176`、`extensions/api.ts:96`。影响：状态不一致风险。建议：第11步统一运行记录与状态。

### P2（中风险结构问题）

- **P2-FE-1 ChatInput 直接依赖 Agent Skill API**：`ChatInput.vue` 引用 Agent Skill，聊天组件与扩展系统耦合。证据：`rg agent-skill ChatInput.vue`。影响：聊天链路与扩展系统边界模糊。建议：第10步交叉依赖矩阵标注。
- **P2-FE-2 装配层 MCP API 注入 Extension Runtime**：`router.go:92` `mcpapi.RegisterRouter` 注入 `services.Extension`，MCP API 反向依赖 Extension Runtime。证据：`cmd/server/router.go:92`。影响：MCP API 与 Extension 强耦合。建议：第10步标注。

### P3（低风险）

- **P3-FE-1** `extensionAuth`（`extension/router.go:107-122`）对 `/api/extensions/*` 统一鉴权，但 `/openapi.json` 在鉴权前注册（L19）。建议：文档化。

---

## 七、未确认项

- `handler.ListSkills`（`extension/router.go:40`）是否按 Source 过滤 MCP Tool：需读 `handler.go` 确认技能管理页面是否展示 MCP Tool。
- `ChatInput.vue` 具体调用哪些 Agent Skill API 函数：需读 ChatInput.vue 全文确认依赖深度。
- `MCPInteractionGuard` 的具体拦截逻辑与触发条件：需读 App.vue 与该组件实现。
- 各 Vue 页面之间的跳转关系（如 SkillDetail → Workshop fork）：需逐页确认 router.push 调用。

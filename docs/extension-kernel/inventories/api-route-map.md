# API 路由矩阵

> 审计依据：.trae/Amitia_扩展系统重构_第2步_建立现有系统调用链地图.md 第十四部分
> 审计日期：2026-07-25
> 状态：第2步调用链地图（只审计不修改）
> 详细页面矩阵见 call-chains/09-frontend-api.md

---

## 一、Extension API 路由矩阵（`/api/extensions/*`）

后端入口：`backend/internal/extension/router.go` `RegisterRouter`
鉴权：`extensionAuth` 中间件（Bearer Token，L20-121）

| HTTP | 路由 | Handler | Service | 实际系统 | 风险标记 |
|---|---|---|---|---|---|
| GET | `/openapi.json` | `handler.OpenAPI` | — | OpenAPI 文档 | 鉴权前注册（P3-FE-1） |
| GET | `/capabilities` | `handler.Capabilities` | — | Capability 定义 | — |
| POST | `/packages/import/preview` | `packageHandler.Preview` | PackageService | Package Parser | — |
| POST | `/packages/import/install` | `packageHandler.Install` | PackageService | Package Installer | — |
| GET | `/packages/metrics` | `packageHandler.Metrics` | PackageService | Package 指标 | — |
| GET | `/package-operations` | `packageHandler.Operations` | PackageService | Package 操作历史 | — |
| GET | `/package-operations/:operationId` | `packageHandler.Operation` | PackageService | Package 单操作 | — |
| GET | `/signers` | `packageHandler.Signers` | PackageService | Package 签名 | — |
| POST | `/signers/:fingerprint/trust` | `packageHandler.TrustSigner` | PackageService | Package 签名信任 | — |
| POST | `/signers/:fingerprint/untrust` | `packageHandler.UntrustSigner` | PackageService | Package 签名信任 | — |
| GET | `/:id/exports/:exportId` | `packageHandler.Download` | PackageService | Package 导出下载 | — |
| POST | `/:id/export` | `packageHandler.Export` | PackageService | Package 导出 | 不重新签名（P1-3） |
| POST | `/:id/upgrade/preview` | `packageHandler.PreviewUpgrade` | PackageService | Package 升级预览 | — |
| POST | `/:id/upgrade` | `packageHandler.Upgrade` | PackageService | Package 升级 | — |
| GET | `/:id/versions` | `packageHandler.Versions` | PackageService | Package 版本 | — |
| GET | `/:id/versions/compare` | `packageHandler.Compare` | PackageService | Package 版本对比 | — |
| POST | `/:id/versions/:version/rollback` | `packageHandler.Rollback` | PackageService | Package 回滚 | 状态直接置 succeeded（P2-1） |
| GET | `/:id/dependencies` | `packageHandler.Dependencies` | PackageService | Package 依赖 | — |
| GET | `/:id/uninstall/preview` | `packageHandler.PreviewUninstall` | PackageService | Package 卸载预览 | 不做来源预检（P1-2） |
| DELETE | `/:id` | `packageHandler.Uninstall` | PackageService | Package 卸载 | — |
| GET | `/skills` | `handler.ListSkills` | ExtensionService | Skill Registry | 可能含 MCP Tool（待确认） |
| GET | `/skills/:id` | `handler.GetSkill` | ExtensionService | Skill Registry | — |
| POST | `/skills/:id/enable` | `handler.EnableSkill` | ExtensionService | Registry+Repository | 状态双写（DW-1） |
| POST | `/skills/:id/disable` | `handler.DisableSkill` | ExtensionService | Registry+Repository | 状态双写（DW-1） |
| GET | `/skills/:id/permissions` | `handler.GetPermissions` | ExtensionService | Permission | — |
| PUT | `/skills/:id/permissions` | `handler.UpdatePermissions` | ExtensionService | Permission | — |
| GET | `/skills/:id/config` | `handler.GetConfig` | ExtensionService | Repository | — |
| PUT | `/skills/:id/config` | `handler.UpdateConfig` | ExtensionService | Repository | — |
| POST | `/skills/:id/config/reset` | `handler.ResetConfig` | ExtensionService | Repository | — |
| POST | `/skills/:id/execute` | `handler.Execute` | ExtensionService | Executor | — |
| POST | `/skills/:id/workshop/fork` | `workshopHandler.Fork` | WorkshopService | Workshop | — |
| POST | `/skills/:id/versions/:version/rollback` | `workshopHandler.Rollback` | WorkshopService | Workshop | — |
| POST | `/agent-skills/import/preview` | `agentSkillHandler.Preview` | AgentSkillService | Agent Skill Parser | — |
| POST | `/agent-skills/import/install` | `agentSkillHandler.Install` | AgentSkillService | Agent Skill Service | 同名同 hash 跳过 Register（P2-2） |
| GET | `/agent-skills` | `agentSkillHandler.List` | AgentSkillService | Agent Skill Service | — |
| GET | `/agent-skills/metrics` | `agentSkillHandler.Metrics` | AgentSkillService | Agent Skill 指标 | 不持久化（P3-1） |
| GET | `/agent-skills/:id` | `agentSkillHandler.Get` | AgentSkillService | Agent Skill Service | MCPDependencies 未持久化（P0-2） |
| POST | `/agent-skills/:id/enable` | `agentSkillHandler.Enable` | AgentSkillService | Agent Skill Service | 状态三写（DW-2） |
| POST | `/agent-skills/:id/disable` | `agentSkillHandler.Disable` | AgentSkillService | Agent Skill Service | — |
| DELETE | `/agent-skills/:id` | `agentSkillHandler.Remove` | AgentSkillService | Agent Skill Service | 不清理 MCP Server（P0-1） |
| GET | `/agent-skills/:id/compatibility` | `agentSkillHandler.Compatibility` | AgentSkillService | Agent Skill 兼容性 | — |
| GET | `/agent-skills/:id/resources` | `agentSkillHandler.Resources` | AgentSkillService | Agent Skill 资源 | — |
| GET | `/agent-skills/:id/resources/content` | `agentSkillHandler.ResourceContent` | AgentSkillService | Agent Skill 资源内容 | — |
| GET | `/agent-skills/:id/assets/content` | `agentSkillHandler.AssetContent` | AgentSkillService | Agent Skill Asset | — |
| GET | `/agent-skills/:id/activations` | `agentSkillHandler.Activations` | AgentSkillService | Agent Skill 激活记录 | — |
| GET | `/workshop/metrics` | `workshopHandler.Metrics` | WorkshopService | Workshop 指标 | — |
| POST | `/workshop/instructions/generate` | `workshopHandler.GenerateInstruction` | WorkshopService | Workshop Generator | 依赖 Chat（RD-4） |
| GET | `/workshop/sessions` | `workshopHandler.ListSessions` | WorkshopService | Workshop | — |
| POST | `/workshop/sessions` | `workshopHandler.CreateSession` | WorkshopService | Workshop | — |
| GET | `/workshop/sessions/:id` | `workshopHandler.GetSession` | WorkshopService | Workshop | — |
| POST | `/workshop/sessions/:id/archive` | `workshopHandler.Archive` | WorkshopService | Workshop | — |
| GET | `/workshop/sessions/:id/revisions` | `workshopHandler.ListRevisions` | WorkshopService | Workshop | — |
| GET | `/workshop/sessions/:id/revisions/:revision` | `workshopHandler.GetRevision` | WorkshopService | Workshop | — |
| POST | `/workshop/sessions/:id/generate` | `workshopHandler.Generate` | WorkshopService | Workshop Generator | 依赖 Chat（RD-4） |
| POST | `/workshop/sessions/:id/revisions/:revision/validate` | `workshopHandler.Validate` | WorkshopService | Workshop Compiler | — |
| POST | `/workshop/sessions/:id/revisions/:revision/permissions/confirm` | `workshopHandler.ConfirmPermissions` | WorkshopService | Workshop 权限 | — |
| POST | `/workshop/sessions/:id/revisions/:revision/test` | `workshopHandler.Test` | WorkshopService | Workshop TestRunner | — |
| POST | `/workshop/sessions/:id/revisions/:revision/install` | `workshopHandler.Install` | WorkshopService | WorkshopInstaller→Registry | — |
| GET | `/workshop/sessions/:id/tests` | `workshopHandler.ListTests` | WorkshopService | Workshop 测试 | — |
| GET | `/workshop/sessions/:id/artifact` | `workshopHandler.GetArtifact` | WorkshopService | Workshop Artifact | — |
| POST | `/workshop/sessions/:id/export` | `workshopHandler.Export` | WorkshopService | Workshop 导出 | — |
| GET | `/workshop/tests/:testRunId` | `workshopHandler.GetTest` | WorkshopService | Workshop 测试详情 | — |
| GET | `/plugins` | `handler.ListPlugins` | ExtensionService | PluginManager | — |
| GET | `/plugins/:id` | `handler.GetPlugin` | ExtensionService | PluginManager | — |
| POST | `/plugins/:id/enable` | `handler.EnablePlugin` | ExtensionService | PluginManager | 状态三写（DW-3） |
| POST | `/plugins/:id/disable` | `handler.DisablePlugin` | ExtensionService | PluginManager | — |
| POST | `/plugins/:id/reload` | `handler.ReloadPlugin` | ExtensionService | PluginManager | — |
| GET | `/plugins/:id/config` | `handler.GetPluginConfig` | ExtensionService | PluginRepository | — |
| PUT | `/plugins/:id/config` | `handler.UpdatePluginConfig` | ExtensionService | PluginRepository | — |
| POST | `/plugins/:id/config/reset` | `handler.ResetPluginConfig` | ExtensionService | PluginRepository | — |
| GET | `/plugins/:id/permissions` | `handler.GetPluginPermissions` | ExtensionService | Permission | — |
| PUT | `/plugins/:id/permissions` | `handler.UpdatePluginPermissions` | ExtensionService | Permission | — |
| GET | `/plugins/:id/health` | `handler.GetPluginHealth` | ExtensionService | PluginManager 断路器 | — |
| POST | `/plugins/:id/circuit/reset` | `handler.ResetPluginCircuit` | ExtensionService | PluginManager 断路器 | — |
| GET | `/plugins/:id/state` | `handler.GetPluginState` | ExtensionService | PluginRepository | — |
| GET | `/plugins/:id/surface` | `handler.GetPluginSurface` | ExtensionService | Plugin Surface | 管理表单非 UI 扩展 |
| GET | `/plugins/:id/schedules` | `handler.GetPluginSchedules` | ExtensionService | Plugin Schedule | — |
| POST | `/plugins/:id/schedules/:scheduleId/pause` | `handler.PausePluginSchedule` | ExtensionService | Plugin Schedule | — |
| POST | `/plugins/:id/schedules/:scheduleId/resume` | `handler.ResumePluginSchedule` | ExtensionService | Plugin Schedule | — |
| GET | `/plugins/:id/events` | `handler.GetPluginEvents` | ExtensionService | Plugin Event | — |
| GET | `/plugins/:id/events/dead-letter` | `handler.GetPluginDeadLetters` | ExtensionService | Plugin Event | — |
| POST | `/plugins/:id/events/:eventId/retry` | `handler.RetryPluginEvent` | ExtensionService | Plugin Event | — |
| POST | `/plugins/:id/surface/actions/:actionId` | `handler.ExecutePluginSurfaceAction` | ExtensionService | Plugin Surface→Host.CallSkill | — |
| GET | `/runs` | `handler.ListRuns` | ExtensionService | Repository Run | — |
| GET | `/runs/:runId` | `handler.GetRun` | ExtensionService | Repository Run | — |

---

## 二、MCP API 路由矩阵（`/api/mcp/*`）

后端入口：`backend/internal/mcpapi/router.go`
鉴权：独立鉴权中间件
反向依赖：注入 `services.Extension`（RD-5）

| HTTP | 路由 | Handler | 实际系统 | 风险标记 |
|---|---|---|---|---|
| GET | `/servers` | `Handler.listServers` | MCP Repository | — |
| POST | `/servers` | `Handler.createServer` | MCP Repository+Secret | 错误回滚不删 Secret（P2-1） |
| PUT | `/servers/:id` | `Handler.updateServer` | MCP Repository | — |
| DELETE | `/servers/:id` | `Handler.deleteServer` | MCP Repository | 不清理 Registry（P1-1）、不撤销 OAuth（P1-3） |
| POST | `/servers/:id/connect` | `Handler.connect` | MCP Manager.Connect | — |
| POST | `/servers/:id/disconnect` | `Handler.disconnect` | MCP Manager.Disconnect | — |
| POST | `/servers/:id/reconnect` | `Handler.reconnect` | MCP Manager.Reconnect | — |
| POST | `/servers/:id/refresh` | `Handler.refresh` | MCP Discovery | 不触发 RegisterServer（P1-2） |
| GET | `/servers/:id/tools` | `Handler.listTools` | MCP Repository ToolDefinition | — |
| PUT | `/servers/:id/tools/:tool/enabled` | `Handler.setToolEnabled` | MCP Repository+Registry Scope | 状态双写（DW-1） |
| GET | `/servers/:id/resources` | `Handler.listResources` | MCP Features | — |
| POST | `/servers/:id/resources/read` | `Handler.readResource` | MCP Features | — |
| POST | `/servers/:id/resources/subscribe` | `Handler.subscribeResource` | MCP Features | — |
| GET | `/servers/:id/prompts` | `Handler.listPrompts` | MCP Features | — |
| POST | `/servers/:id/prompts/get` | `Handler.getPrompt` | MCP Features | — |
| POST | `/servers/:id/completion` | `Handler.completeArgument` | MCP Features | — |
| GET | `/servers/:id/logs` | `Handler.listLogs` | MCP Repository | — |
| GET | `/servers/:id/tasks` | `Handler.listTasks` | MCP Host | — |
| POST | `/servers/:id/tasks/:tid/cancel` | `Handler.cancelTask` | MCP Host | — |
| GET | `/servers/:id/capabilities` | `Handler.listCapabilities` | MCP Repository | — |
| PUT | `/servers/:id/capabilities/:cap` | `Handler.setCapability` | MCP Repository | — |
| POST | `/servers/:id/oauth/start` | `Handler.startOAuth` | MCP Auth | — |
| POST | `/servers/:id/oauth/revoke` | `Handler.revokeOAuth` | MCP Auth | — |
| GET | `/oauth/callback` | `Handler.oauthCallback` | MCP Auth→Connect→Discover→Register | — |
| GET | `/interactions` | `Handler.listInteractions` | MCP Host Broker | — |
| POST | `/interactions/:iid/resolve` | `Handler.resolveInteraction` | MCP Host Broker | — |
| POST | `/agent-skills/dependencies/preview` | `Handler.dependencyPreview` | MCP Dependency Service | — |
| POST | `/agent-skills/dependencies/install` | `Handler.dependencyInstall` | MCP Dependency Service | — |
| DELETE | `/agent-skills/:skillId/dependencies` | `Handler.removeDependencies` | MCP Dependency Service | 仅删 link 行（P0-1） |

---

## 三、跨命名空间能力对照

| 能力 | Extension API | MCP API | 说明 |
|---|---|---|---|
| MCP Tool 启用 | `/skills/:id/enable`（Registry Scope） | `/servers/:id/tools/:tool/enabled`（Tool Enabled） | 状态双写，两个入口（DW-1，P1-FE-3） |
| Agent Skill MCP 依赖 | `/agent-skills/:id/...` | `/agent-skills/dependencies/...` | 依赖管理跨两个命名空间 |
| MCP Server 删除 | — | `/servers/:id` | 不联动清理 Extension Registry（P1-1） |
| Agent Skill 删除 | `/agent-skills/:id` | `/agent-skills/:skillId/dependencies` | 删除链跨两个命名空间，清理不完整（P0-1） |

---

## 四、路由统计

| 命名空间 | 路由数 | Handler 文件 | 鉴权方式 |
|---|---|---|---|
| `/api/extensions/*` | 79 | `extension/router.go` | `extensionAuth`（Bearer Token） |
| `/api/mcp/*` | 29 | `mcpapi/router.go` | 独立鉴权 |
| **合计** | **108** | — | — |

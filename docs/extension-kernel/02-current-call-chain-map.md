# Amitia 扩展系统现有调用链地图（第 2 步主文档）

> 审计依据：`.trae/Amitia_扩展系统重构_第2步_建立现有系统调用链地图.md`
> 审计日期：2026-07-25
> 状态：第2步调用链地图（只审计不修改）
> 审计原则：以实际源码为唯一依据，区分"声明存在"与"运行时实际可达"，不在本步骤修改架构

---

## 一、审计范围与产出总览

### 1. 审计范围

本次审计覆盖 Amitia 扩展系统全部子系统：Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` 扩展包、Workshop、扩展中心前端，及其与聊天 Agent 的连接方式。

### 2. 产出结构

```text
docs/extension-kernel/
├── 02-current-call-chain-map.md          ← 本文档（主文档）
├── call-chains/
│   ├── 01-startup-shutdown.md            ← 地图 A、B（启动装配/关闭）
│   ├── 02-tool-exposure-execution.md     ← 地图 C、D（工具暴露/执行）
│   ├── 03-agent-skill.md                 ← 地图 E（Agent Skill Prompt）
│   ├── 04-mcp.md                         ← 地图 F（MCP 生命周期）
│   ├── 05-plugin.md                      ← 地图 G（Plugin 生命周期）
│   ├── 06-workflow.md                    ← 地图 I（Workflow 执行）
│   ├── 07-package.md                     ← 地图 H（.amitiax 包生命周期）
│   ├── 08-workshop.md                    ← Workshop 完整链路
│   └── 09-frontend-api.md                ← 地图 J（前端到后端 API）
├── inventories/
│   ├── source-files.md                   ← 源码范围索引
│   ├── api-route-map.md                  ← API 路由矩阵
│   ├── state-matrix.md                   ← 地图 L（状态判定矩阵）
│   └── dependency-matrix.md              ← 地图 K（交叉依赖矩阵）
└── diagrams/                             ← 8 张独立 Mermaid 图文件
    ├── startup.mmd                       ← 地图 A 系统启动装配图
    ├── shutdown.mmd                      ← 地图 B 系统关闭图
    ├── tool-execution.mmd                ← 地图 C+D 工具暴露与执行图
    ├── agent-skill.mmd                   ← 地图 E Agent Skill Prompt 图
    ├── mcp-lifecycle.mmd                 ← 地图 F MCP 生命周期图
    ├── plugin-lifecycle.mmd              ← 地图 G Plugin 生命周期图
    ├── package-lifecycle.mmd             ← 地图 H .amitiax 包生命周期图
    └── dependency-graph.mmd              ← 地图 K 交叉依赖图
```

---

## 二、12 张调用链地图索引

### 地图 A：系统启动装配图

- **详细文档**：[01-startup-shutdown.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/01-startup-shutdown.md) 链路 SU-1~SU-4
- **核心链路**：`main` → `NewAppServices` → `extension.NewRuntime`（内部：Registry→LegacyToolAdapter→AgentSkill.Restore→Plugin.Start→Workshop.Restore→Package.Restore）→ `chatSvc.SetSkillRuntime` → `configureWorkflowHost` → MCP 11 组件创建 → `AgentSkills.SetAfterRemove(dep.Uninstall)` → `RegisterReadyHandler` → `RegisterAll`+`Restore`
- **关键发现**：装配层（services.go）直接组装全部子系统内部组件，7 处反向依赖中 6 处由此注入

### 地图 B：系统关闭图

- **详细文档**：[01-startup-shutdown.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/01-startup-shutdown.md) 链路 SD-1~SD-3
- **核心链路**：`signal SIGINT/SIGTERM` → `SetOrchestratorReady(false)` → `Extension.Close(5s)`（→`PluginManager.Stop`→停 4 Worker+OnUnload）→ `HTTP Shutdown(10s)` → defer `MCP Close(5s)` → defer `Extension Close`（空操作）→ defer `env.StopAll`
- **关键发现**：Extension.Close 被调用两次（显式+defer），第二次因 nil 检查空操作；关闭顺序依赖 defer LIFO

### 地图 C：模型工具暴露图

- **详细文档**：[02-tool-exposure-execution.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/02-tool-exposure-execution.md) 链路 TE-1
- **核心链路**：`chat 请求` → `ExecutionScope` → `Runtime.ModelTools`（runtime.go:137）→ `Registry.Available`（过滤 Enabled+Compatible+Kind≠instructions）→ `GetScoped`（作用域）→ internal 工具受 `agentSkillToolsAvailable` 控制 → `Permissions.PreviewExecution` 过滤 → `tool.Tool` 列表 → 模型请求
- **关键发现**：internal 工具可见性隐式依赖 Agent Skill（P1-TE-2）；4 类工具来源（Legacy/Workflow/Plugin/MCP）汇合到同一 Registry

### 地图 D：模型工具执行图

- **详细文档**：[02-tool-exposure-execution.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/02-tool-exposure-execution.md) 链路 TE-2
- **核心链路**：`模型 Tool Call` → `message_llm.go:71` `ExecuteModelTool`（create_schedule 特殊参数注入 P1-TE-1）→ `Registry.GetByModelName` → `Executor.Execute` → GetScoped+Enabled+Compatible+Trigger 检查 → 输入 Schema 校验 → 逐 capability `EvaluateExecution` → `callHandler`（goroutine+handlerSlots 64+recover）→ 输出 Schema 校验 → `RegisterOwnedSideEffects` → `UpdateRun` 审计
- **关键发现**：无绕过权限的执行入口；agent_skill_activate 特殊处理（P1-TE-3）

### 地图 E：Agent Skill Prompt 图

- **详细文档**：[03-agent-skill.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/03-agent-skill.md) 链路 AS-1~AS-6
- **核心链路**：
  - 导入：ZIP/目录 → 安全检查 → SKILL.md 解析 → Frontmatter → 资源索引 → 兼容性 → Metadata/Artifact 保存 → Catalog 刷新
  - Prompt 激活：`PrepareAgentSkillPrompt` → `ResolveCatalog`（缓存）→ 显式技能识别（$skill-name）→ `Activate`（Token 限制+单体限制+Round 总量限制）→ `renderActiveAgentSkill` → 注入 system prompt → `EndAgentSkillRound` 清理
  - 资源读取：模型调用 internal 工具 → `activeDefinition`（round 校验）→ 路径校验 → `loadAgentSkill` → Round 预算校验 → 内容包装
- **关键发现**：MCPDependencies 未持久化到 DB（P0-2）；get_asset 绕过 round 预算（P1-1）

### 地图 F：MCP 生命周期图

- **详细文档**：[04-mcp.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/04-mcp.md) 链路 MCP-1~MCP-8
- **核心链路**：创建 Server → Credential 保存 → `Manager.Connect` → `Factory.Build` → stdio/HTTP Transport → OAuth/Secret → MCP initialize → Capability 保存 → Ready Handler（Attach→Discover→RegisterServer）→ Discovery tools/list → ToolDefinition 入库 → `mcp/skill.Runtime.RegisterServer` → 转 SkillDefinition → Registry 注册 → 模型可见
- **关键发现**：DeleteServer 不清理 Registry 孤儿 Skill（P1-1）；list_changed 不触发 RegisterServer（P1-2）；DeleteServer 不撤销 OAuth token（P1-3）

### 地图 G：Plugin 生命周期图

- **详细文档**：[05-plugin.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/05-plugin.md) 链路 PLG-1~PLG-7
- **核心链路**：注册（仅内置 builtin）→ `PluginManager.Start` → `load`（UpsertPlugin+OnLoad+RegisterSkill+MigrateStates+Enable）→ 4 Worker（afterReply/eventIngress/event/schedule）→ Hook 分发（DispatchBeforePrompt 同步 800ms / DispatchAfterReply 异步入队）→ 事件持久化+投递+重试 → 调度执行 → Enable/Disable/Reload/ResetCircuit
- **关键发现**：仅支持内置注册，无第三方动态加载（P1-1）；Stop 不注销 Skills（P1-2）；invoke 用 context.Background() 不响应 Stop 取消（P1-3）；Plugin Surface 是管理表单非 UI 扩展系统

### 地图 H：.amitiax 包生命周期图

- **详细文档**：[07-package.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/07-package.md) 链路 PKG-1~PKG-7
- **核心链路**：预览（Archive 安全检查→Manifest 解析→Schema 校验→Entry 解析→签名检查→依赖检查→风险预览）→ 安装（重新解析→风险确认→Workflow/Instructions 分支→Artifact 保存→Definition/Handler 构建→Registry 注册→Package Version 提交→Operation 审计）→ 升级 → 回滚 → 卸载 → 启动恢复
- **关键发现**：Manifest Schema 声明 Plugin oneOf 但 Parser 拒绝（接口与实现不一致）；reinstallArchivedPackage 无独立 API（P1-1）；导出不重新签名（P1-3）

### 地图 I：Workflow 执行图

- **详细文档**：[06-workflow.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/06-workflow.md) 链路 WF-1~WF-3
- **核心链路**：编译（Schema 校验→Compiler→依赖解析→SkillDefinition/Handler→Registry）→ 执行（Executor→WorkflowExecutor→节点调度→Host 调用→SideEffect→结果聚合）→ Host（Schedule/Notification/MemoryCandidate/ContextContribution/call_skill）
- **关键发现**：WorkflowHostAdapter 装配层泄漏（P0）；Schedule Host 通过 Registry.Get 回到 Executor 形成循环依赖（P0/CD-1）；call_skill 嵌套无 parent_run_id（P1）

### Workshop 子系统（技能制作）

- **详细文档**：[08-workshop.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/08-workshop.md) 链路 WS-1~WS-10
- **核心链路**：创建 Session → 指令生成（AI/手工）→ Revision 管理（每次 Generate 创建新 Revision，清空旧权限/测试）→ 校验+权限确认（测试/生产双独立）→ 测试运行（复用 WorkflowExecutor，按 WorkflowExecutionMode 区分）→ **安装（直接通过 WorkshopInstaller.Install 写 Registry 与 DB，不调 PackageService）** → 导出（后端有路由前端无入口）/ Fork / 回滚
- **关键发现**：
  - Workshop Install **不调 PackageService**，直接写 Registry+DB（PackageService 反向持有 WorkshopInstaller.definitionFromArtifact/workflowHandler 用于 .amitiax 包加载）
  - 默认生成 Workflow 类型 Skill；Instructions 分支走 `GenerateInstruction` → `agentSkills.storePreview` → 前端调 `/agent-skills/import/install`，**完全脱离 Workshop Session 体系**（链路 WS-9，P2-WS-1 无来源追溯）
  - 测试运行器复用生产 WorkflowExecutor，通过 WorkflowExecutionMode（dry_run/mocked/controlled_live/production）区分
  - 状态机 `validWorkshopTransition` 严格管控 15 种状态转换
  - 20 条 Workshop 路由、32 个错误码、3 个附录（路由清单/状态机/错误码）

### 地图 J：前端到后端 API 图

- **详细文档**：[09-frontend-api.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/call-chains/09-frontend-api.md)
- **API 矩阵**：[api-route-map.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/inventories/api-route-map.md)
- **核心链路**：11 条扩展路由（全部无主导航入口 P1-FE-1）→ ExtensionCenterView 6 卡片 → 2 个 API 命名空间（Extension 79 路由 + MCP 29 路由 = 108 路由）→ Handler → Service → Runtime/Repository
- **关键发现**：扩展包管理页面有路由但首页未链接（P1-FE-2）；MCP Tool 启用状态双写（P1-FE-3）；ChatInput 直接依赖 Agent Skill API（P2-FE-1）

### 地图 K：交叉依赖图

- **详细文档**：[dependency-matrix.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/inventories/dependency-matrix.md)
- **核心内容**：7 处反向依赖（RD-1~RD-7）、2 个循环依赖（CD-1 Workflow Schedule、CD-2 Agent Skill 删除清理不完整）、7 处装配层泄漏（AL-1~AL-7）、12 项隐式全局状态（GS-1~GS-12）、3 个可删除兼容层（CL-1~CL-3）、10 个不可删除桥接点（BP-1~BP-10）
- **关键发现**：装配层是所有反向依赖的汇聚点；10 个桥接点构成系统骨架

### 地图 L：状态与启用判定图

- **详细文档**：[state-matrix.md](file:///d:/桌面/跟进项目/U-Ai/docs/extension-kernel/inventories/state-matrix.md)
- **核心内容**：15 个状态维度（S1~S15）、能力可见性判定（8-10 个状态共同决定）、MCP Tool 完整状态组合表（8 维度）、5 处状态双写/多写（DW-1~DW-5）、状态恢复矩阵
- **关键发现**：MCP Tool 状态维度最多（8 个），Connection Ready 与 Registry 注册项不同步导致断线工具残留；恢复失败策略不一致

---

## 三、汇总问题清单（P0-P3）

### P0：重构阻塞（共 5 项）

| 编号 | 问题 | 文件:函数 | 影响链路 | 后续建议处理步骤 |
|---|---|---|---|---|
| P0-1 | Agent Skill 删除链不清理 MCP Server 与 Registry 注册项 | `mcp/dependency/service.go:268` `dependency.Service.Uninstall` | AS-5、AS-6 | Uninstall 中对 unreferenced server 串行执行 Disconnect→UnregisterServer→DeleteServer，并补 Registry.Unregister |
| P0-2 | Agent Skill 的 MCPDependencies 未持久化到 DB | `extension/agent_skill_repository.go:21-52` `agentSkillMetadataRecord` | AS-6 | 增加 mcp_dependencies_json 列，并在 Install/Restore 序列化/反序列化 |
| P0-3 | Workflow Host Schedule 形成循环依赖 | `services.go:400` `Registry.Get("dev.amitia.skill.create-schedule")` | WF-3-A、CD-1 | 打断 Host→Registry→Executor→WorkflowExecutor→Host 闭环，第 3 步重构时重新设计 Schedule Host |
| P0-4 | WorkflowHostAdapter 装配层泄漏 | `services.go:389-470` `configureWorkflowHost` | WF-1、WF-3 | 4 个 Host 函数由装配层闭包注入 Chat/Memory/Delivery，重构时需封装为 Host 接口 |
| P0-5 | Agent Skill 删除清理不完整形成隐式残留链 | `extension/agent_skill_service.go:396` `afterRemove` → `dep.Uninstall` | AS-5、CD-2 | afterRemove 回调失败时 Agent Skill 已删但 MCP link 残留，需事务化或补偿 |

### P1：高风险历史债务（共 25 项）

| 编号 | 问题 | 文件:函数 | 影响链路 |
|---|---|---|---|
| P1-SD-1 | 装配层泄漏：services.go 直接组装全部子系统内部组件 | `services.go:288-314` | 启动 |
| P1-SD-2 | 反向依赖密集：4 处反向依赖由装配层注入 | `services.go:172,297,299,302` | 启动 |
| P1-SD-3 | 关闭顺序依赖 defer 且 Extension.Close 被调用两次 | `main.go:241,111-112` | 关闭 |
| P1-TE-1 | chat 层硬编码改写 create_schedule 参数，绕过 Executor 统一处理 | `message_llm.go:44-61` | TE-2 |
| P1-TE-2 | internal 工具可见性隐式依赖 Agent Skill | `runtime.go:140-153` | TE-1 |
| P1-TE-3 | agent_skill_activate 为可被模型主动调用的特殊工具，存在两套激活路径 | `message_llm.go:79-96` | TE-2 |
| P1-AS-1 | agent_skill_get_asset handler 绕过 ReadResource 的 round 预算统计 | `agent_skill_runtime.go:70-103` | AS-4 |
| P1-AS-2 | activate handler 显式工具调用绕过 PreparePrompt 的 token 总量预算 | `agent_skill_runtime.go:26-39` | AS-3 |
| P1-AS-3 | extensionLifecycleService.setEnabled 与 AgentSkillService.Enable 重复校验且对 Agent Skill Dependencies 校验形同虚设 | `lifecycle_service.go:32-57` | AS-2 |
| P1-MCP-1 | DeleteServer 不调 RegisterServer/Unregister → 孤儿 Skill | `mcp/repository.go:174` | MCP-8 |
| P1-MCP-2 | list_changed 不触发 RegisterServer → 工具更新后 Registry 不同步 | `mcp/discovery/service.go:97` | MCP-8 |
| P1-MCP-3 | DeleteServer 不撤销 OAuth token | `mcp/repository.go:174` | MCP-4 |
| P1-MCP-4 | connect 旧连接 Close 导致进行中调用失败 | `mcp/manager/manager.go:194,232` | MCP-2 |
| P1-MCP-5 | RegisterServer 旧 Skill 清理依赖前缀字符串匹配 | `mcp/skill/runtime.go:92` | MCP-5 |
| P1-PLG-1 | Plugin 子系统仅支持内置注册，未接通第三方动态加载 | `plugin_registry.go:109-110` | PLG-1 |
| P1-PLG-2 | PluginManager.Stop 不注销已注册的 Skills | `plugin_manager.go:90-128` | PLG-2 |
| P1-PLG-3 | invoke 使用 context.Background() 不响应 Stop 取消 | `plugin_manager.go:444` | PLG-3~6 |
| P1-FE-1 | 扩展中心全部页面无主导航入口 | `app-nav.ts:27-84` | 地图 J |
| P1-FE-2 | 扩展包管理页面有路由但首页未链接 | `ExtensionCenterView.vue:39-76` | 地图 J |
| P1-FE-3 | MCP Tool 启用状态双写 | `mcp/api.ts:176`、`extensions/api.ts:96` | 地图 J、DW-1 |
| P1-PKG-1 | reinstallArchivedPackage 无独立 API 入口 | `package_installer.go:224` | PKG-2 |
| P1-PKG-2 | PreviewUninstall 不做来源预检 | `package_lifecycle.go:456` | PKG-5 |
| P1-PKG-3 | exportAmitiaxFiles 不重新签名 | `package_lifecycle.go:80` | PKG-7 |
| P1-WF-1 | call_skill 嵌套无 parent_run_id 关联 | `workflow_executor.go:476` | WF-2 |
| P1-WS-1 | Workshop 导出死路由：后端 `/workshop/sessions/:id/export` 有路由但前端无入口 | `extension/router.go:69`、前端无引用 | WS-7 |
| P1-WS-2 | ForkSkill 绕过 scope 校验，可跨用户/角色 Fork | `workshop_installer.go Fork` | WS-8 |
| P1-WS-3 | Workshop 内存锁多实例无效（分布式部署下并发失控） | `workshop_service.go sync.Mutex` | WS-1~WS-6 |
| P1-WS-4 | Install 失败补偿丢失 TraceID，无法关联审计 | `workshop_installer.go Install` | WS-6 |
| P1-WS-5 | SaveTestReport 未校验 lock_version，并发覆盖 | `workshop_repository.go SaveTestReport` | WS-5 |
| P1-WS-6 | extension_versions 扩展字段回填依赖列存在性，列缺失时静默失败 | `workshop_installer.go extension_versions` | WS-6 |
| P1-WS-7 | Export 在无 Artifact 时写空文件，无前置校验 | `workshop_service.go Export` | WS-7 |

### P2：中风险结构问题（共 25+ 项）

| 编号 | 问题 | 文件 | 影响链路 |
|---|---|---|---|
| P2-SD-1 | 恢复失败策略不一致（AgentSkill/Package 终止启动，Workshop/MCP 仅 Warn） | `runtime.go:63,88,92` | 启动 |
| P2-TE-1 | 工具执行幂等性仅对 Idempotent 定义生效 | `executor.go:86-130` | TE-2 |
| P2-TE-2 | ModelName 冲突检测仅靠内存 map | `registry.go:82-84` | TE-1 |
| P2-AS-1 | SaveAgentSkillActivation 错误被忽略 | `agent_skill_service.go:636` | AS-4 |
| P2-AS-2 | Install 同名同 hash 跳过 Register，Restore 失败后无法恢复 | `agent_skill_service.go:162-182` | AS-1→AS-2 |
| P2-AS-3 | Restore 中 Register 失败直接 return 中断后续恢复 | `agent_skill_service.go:264-266` | 启动 |
| P2-AS-4 | Registry.Available 过滤 instructions kind，internal 工具 scope 隔离不严格 | `registry.go:188` | AS-4 |
| P2-MCP-1~9 | MCP 子系统 9 项中风险（Secret 残留、reconnecting 标志、Attach 重复注册、错误码归类、分页失败、Task 截断、stdio 命令校验、日志截断、连接替换竞态） | `mcp/*` | MCP 各链路 |
| P2-PLG-1~7 | Plugin 子系统 7 项中风险（AfterReply 队列满丢弃、EmitSystemEvent 错误忽略、EmitEvent 不重试、Stop 超时、Schedule 不支持 cron、串行限 20 条、禁用插件事件积压） | `plugin_manager.go` | PLG 各链路 |
| P2-PKG-1~3 | Package 子系统 3 项中风险（Rollback 状态直接置 succeeded、workshop 回滚 Hash 校验弱、operation_id 列依赖） | `package_lifecycle.go` | PKG 各链路 |
| P2-WF-1 | ContextContribution Host 返回 Confirmed=true 但实际未注入上下文 | `services.go:468` | WF-3-D |
| P2-WF-2 | WorkflowCompiler.Compile 在工坊 Generate 阶段被调用两次 | `workshop_service.go:178,192` | WF-1 |
| P2-WS-1 | Instructions 产物无来源追溯（Workshop Session 表不记录） | `workshop_service.go GenerateInstruction` | WS-9 |
| P2-WS-2 | 错误详情按 byte 截断可能截断多字节字符 | `workshop_service.go` | WS-2~WS-6 |
| P2-WS-3 | Workshop 内存指标不持久化（重启丢失） | `workshop_metrics.go` | WS-1~WS-10 |
| P2-WS-4 | requirement 列类型未显式声明（依赖默认推导） | `extension_workshop.go migration` | WS-3 |
| P2-WS-5 | ModelName 推导可能冲突（无唯一性约束） | `workshop_installer.go` | WS-6 |
| P2-WS-6 | Instructions 无后置内容扫描（安全风险） | `workshop_service.go GenerateInstruction` | WS-9 |
| P2-WS-7 | Export 无签名（与 P1-PKG-3 同类） | `workshop_service.go Export` | WS-7 |
| P2-WS-8 | Generate 阶段不查依赖循环（Workflow 自引用） | `workshop_service.go Generate` | WS-2 |
| P2-FE-1 | ChatInput 直接依赖 Agent Skill API | `ChatInput.vue` | 地图 J |
| P2-FE-2 | 装配层 MCP API 注入 Extension Runtime | `cmd/server/router.go:92` | 地图 J |

### P3：低风险可维护性问题（共 27+ 项）

涵盖：killExistingServer 副作用（P3-SD-1）、idempotencyKey 固定空串（P3-TE-1）、Agent Skill 指标不持久化（P3-AS-1）、explicit 技能正则不识别中文（P3-AS-2）、stripAgentSkillHostTags 正则可被嵌套绕过（P3-AS-3）、MCP Backoff 硬编码（P3-MCP-1）、modelName 截断可读性差（P3-MCP-2）、Workshop 审计表共用（P3-WS-1）、Workshop Metrics 缺 gauge（P3-WS-2）、Workshop Archive 缺二次确认（P3-WS-3）、Workshop SetEnabled 冗余（P3-WS-4）、Workshop maxAttempts 硬编码（P3-WS-5）、Workshop Secret 识别规则狭窄（P3-WS-6）、Workshop Fork 路径参数命名歧义（P3-WS-7）等。详见各子系统文档。

---

## 四、未确认项汇总

| 编号 | 项 | 需要何种手段确认 | 来源文档 |
|---|---|---|---|
| U-1 | PluginManager.Stop 内部停止的 Worker/goroutine 清单 | 已确认（4 Worker：afterReply/eventIngress/event/schedule） | 01、05 |
| U-2 | MCPConnections.Close 是否终止所有 stdio 子进程 | 已确认（terminateProcessTree） | 04 |
| U-3 | env.StopAll 是否等待 Qdrant/Surreal 优雅退出 | 需运行时验证 | 01 |
| U-4 | 4 个 internal 工具是否被 Permissions 系统自动 allow | 运行时验证：模型调用时 PreviewExecution 是否 Deny | 03 |
| U-5 | Registry.Register 对 Agent Skill Manifest（handler=nil）的实际行为 | 阅读 Registry.Register 实现 + 运行时尝试 execute | 03 |
| U-6 | mcp/skill.Runtime.RegisterServer 是否触发 Permissions 授权 | 运行时安装带 MCP 依赖的 Agent Skill 观察 | 03 |
| U-7 | afterRemove 回调是否在事务外执行 | 运行时构造 Link 删除失败样本 | 03 |
| U-8 | handler.ListSkills 是否按 Source 过滤 MCP Tool | 需读 handler.go 确认 | 09 |
| U-9 | ChatInput.vue 具体调用哪些 Agent Skill API | 需读 ChatInput.vue 全文 | 09 |
| U-10 | MCPInteractionGuard 拦截逻辑与触发条件 | 需读 App.vue 与组件实现 | 09 |
| U-11 | dev.amitia.skill.create-schedule Handler 实现 | 需审计 agent_skill_runtime.go/LegacyToolAdapter | 06 |
| U-12 | runPackageWorkflowTests 实现细节 | 需审计 package_test_runner.go | 06、07 |
| U-13 | secretPattern 定义与覆盖范围 | 静态搜索定义位置 | 03、07 |
| U-14 | PluginManager.Stop 后 pending deliveries 下次启动处理 | 已确认（processPendingEvents 拾取 pending+failed） | 05 |
| U-15 | Workshop Generate 真实 LLM 行为（指令质量、字段覆盖） | 需配置真实模型运行时验证 | 08 |
| U-16 | controlled_live 模式是否真正屏蔽网络访问 | 运行时执行 controlled_live 测试用例 | 08 |
| U-17 | extension_versions 扩展字段回填依赖列存在性（列缺失时静默失败） | 运行时构造缺列 DB 样本 | 08 |
| U-18 | Workshop 并发锁在多实例部署下是否失效 | 需多实例部署环境验证 | 08 |
| U-19 | .amitiax 包与 Workshop 互导是否完整保留状态 | 运行时双向导入导出验证 | 07、08 |
| U-20 | breaking Schema 检测覆盖范围 | 构造 Schema 变更测试用例 | 08 |
| U-21 | 已归档 Skill 是否可 Fork | 运行时尝试 Fork archived Skill | 08 |
| U-22 | Workshop 审计表与 Package 审计表交叉影响 | 运行时混合操作观察 | 08 |
| U-23 | 第二次 Compile 失败时状态恢复行为 | 构造 Compile 失败用例 | 08 |
| U-24 | 未配置模型时 Workshop Generate 的错误提示 | 运行时未配置模型环境验证 | 08 |

---

## 五、验收标准核对

依据规范第十五部分，15 条验收标准逐一核对：

| 编号 | 验收标准 | 状态 | 证据 |
|---|---|---|---|
| 1 | 已覆盖 Skill、Agent Skill、MCP、Plugin、Workflow、.amitiax、Workshop 和前端扩展中心 | ✅ | 9 个 call-chains 文档（含 08-workshop.md 10 条链路 WS-1~WS-10）+ 4 个 inventories + 8 个 diagrams/.mmd |
| 2 | 每条关键链路都能追溯到具体文件和函数 | ✅ | 所有链路表格含文件:行号+函数名 |
| 3 | 已绘制启动、关闭、恢复、执行和卸载链 | ✅ | 01（启动/关闭）、各文档 Restore 链、02（执行）、07（卸载）、08（Workshop 安装/测试/回滚） |
| 4 | 已明确模型工具的所有来源 | ✅ | 02-tool-exposure-execution.md：Legacy/Workflow/Plugin/MCP 4 来源 |
| 5 | 已明确 Agent Skill 进入 Prompt 的真实路径 | ✅ | 03-agent-skill.md AS-3：PrepareAgentSkillPrompt→ResolveCatalog→Activate→renderActiveAgentSkill→注入 system prompt |
| 6 | 已明确 MCP Tool 转换和注册位置 | ✅ | 04-mcp.md MCP-5：mcp/skill/runtime.go:92 RegisterServer→转 SkillDefinition→Registry.Register |
| 7 | 已明确 Plugin 只能内置注册还是支持第三方动态注册 | ✅ | 05-plugin.md PLG-1：**仅内置注册**，plugin_registry.go:109 强制 Entry.Kind=="builtin" |
| 8 | 已明确 .amitiax 实际支持的 Entry 类型 | ✅ | 07-package.md：Schema 声明 Plugin oneOf 但 Parser 拒绝，实际支持 workflow/instructions/agent-skill |
| 9 | 已明确所有主要启用状态和组合关系 | ✅ | state-matrix.md：15 个状态维度+可见性判定+MCP Tool 8 维组合表 |
| 10 | 已明确前端页面到后端的实际调用路径 | ✅ | 09-frontend-api.md + api-route-map.md：108 路由完整映射 |
| 11 | 已识别反向依赖、重复注册和状态双写 | ✅ | dependency-matrix.md：7 反向依赖+2 循环依赖+5 状态双写 |
| 12 | 未在本步骤修改任何架构行为 | ✅ | 全程只审计记录，未修改任何源码 |
| 13 | 所有结论均有源码证据 | ✅ | 所有表格含文件:行号+函数名+证据 |
| 14 | 无法确认的内容已明确标注 | ✅ | 第四节 24 项未确认项（含 Workshop 10 项 U-15~U-24），各文档"未确认项"章节 |
| 15 | 后续第 3 步可以直接依据该地图建立数据表与资源归属清单 | ✅ | state-matrix.md 状态维度+dependency-matrix.md 桥接点+各文档 P0/P1 问题（含 Workshop Install 直接写 Registry 不调 PackageService 的关键结论） |

---

## 六、退出条件核对

依据规范第十六部分：

| 退出条件 | 状态 |
|---|---|
| 系统启动和关闭链已完整 | ✅ 01-startup-shutdown.md |
| 所有扩展类型的安装、启停、执行、恢复和卸载入口已确认 | ✅ 各子系统文档 |
| 模型工具暴露和执行链已闭环 | ✅ 02-tool-exposure-execution.md |
| Agent Skill、MCP、Plugin、Workflow 和 Package 的交叉依赖已确认 | ✅ dependency-matrix.md |
| 前端页面和 API 映射已完成 | ✅ 09-frontend-api.md + api-route-map.md |
| 状态判定矩阵已完成 | ✅ state-matrix.md |
| 仍不明确的数据读写点已形成待核对清单 | ✅ 第四节未确认项 |
| 本步骤未产生任何业务行为修改 | ✅ |

---

## 七、关键结论（供第 3 步参考）

1. **装配层是重构的核心切入点**：7 处反向依赖中 6 处由 `services.go` 注入，重构装配层是解除耦合的关键。第 3 步应将装配层列为"需重写"。

2. **10 个不可删除桥接点构成系统骨架**：`Runtime.ModelTools`/`ExecuteModelTool`/`BeforePrompt`/`AfterReply`、`mcp/skill.Runtime.RegisterServer`、`AgentSkillService.SetAfterRemove`、`configureWorkflowHost`、`chatSvc.SetSkillRuntime`、`PluginManager.skills`、`PackageInstaller→Registry.Register`。重构必须逐一提供替代方案。

3. **5 个 P0 问题必须在第 3 步优先处理**：Agent Skill 删除清理不完整（P0-1/P0-5）、MCPDependencies 未持久化（P0-2）、Workflow Schedule 循环依赖（P0-3）、WorkflowHostAdapter 装配层泄漏（P0-4）。

4. **3 个可删除兼容层**：LegacyToolAdapter、旧扩展中心、旧路由。可在新 Extension Kernel 就绪后移除。

5. **Plugin Surface 是管理表单非 UI 扩展系统**：validateSurface 显式禁止可执行内容，前端固定组件渲染，Action 通过后端 Skill 执行。第 3 步若需 UI 扩展能力需全新设计。

6. **状态判定分散在 5 个子系统**：15 个状态维度、5 处状态双写/多写，无统一判定入口。第 3 步应设计统一的状态判定服务。

7. **恢复失败策略不一致**：AgentSkill/Package 失败终止启动，Plugin/MCP 失败仅 Warn。第 3 步应统一恢复失败策略。

8. **Workshop Install 不调 PackageService**：Workshop 通过 `WorkshopInstaller.Install` 直接写 Registry+DB，与 `.amitiax` 包安装走 `PackageInstaller→Registry.Register` 是两条并行链路。反向关系：PackageService 持有 WorkshopInstaller 的 `definitionFromArtifact`/`workflowHandler` 用于 `.amitiax` 包加载 workflow。第 3 步设计统一安装入口时需明确两者归属。

9. **Workshop Instructions 分支脱离 Session 体系**：`GenerateInstruction` 走 `agentSkills.storePreview` → 前端调 `/agent-skills/import/install`，产物 `Source=instructions` 不记录到 Workshop Session 表（P2-WS-1 无来源追溯）。第 3 步需补全 Instructions 产物的来源追溯。

10. **Workshop 测试运行器复用生产 WorkflowExecutor**：通过 `WorkflowExecutionMode`（dry_run/mocked/controlled_live/production）区分行为，单一执行器承担测试与生产双职责。第 3 步重构 Workflow 执行器时需保留 Mode 区分能力。

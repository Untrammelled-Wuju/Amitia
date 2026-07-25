# Tasks

> 实施依据：`d:\桌面\跟进项目\U-Ai\.trae\Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`
> 顺序约束：Task 1 完成后才能开始 Task 2/3/4/5；Task 6 依赖 Task 1-5 全部完成。

- [x] Task 1: 新建 `docs/extension-kernel/01-system-freeze.md` 冻结总说明文档
  - [x] SubTask 1.1: 新建 `docs/extension-kernel/` 目录
  - [x] SubTask 1.2: 写入冻结开始原因与 7 大冻结对象及其子模块清单（Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` v1、扩展中心与创意工坊）
  - [x] SubTask 1.3: 写入 6 类允许修改类型及边界（阻塞性缺陷、安全、回归测试、可观测性、迁移辅助、文档注释）
  - [x] SubTask 1.4: 写入 8 类禁止修改类型（新增扩展类型、扩展旧 Manifest、新增旧系统数据库表、新增平行 Registry、新增永久兼容层、增加旧 Plugin 能力、让 `.amitiax` v1 承担新职责、前端增加分散入口）
  - [x] SubTask 1.5: 写入代码级冻结标记要求、分支与提交策略、代码评审约束、自动检查建议
  - [x] SubTask 1.6: 写入退出条件，明确只有满足条件才能进入第 2 步"建立现有系统调用链地图"
  - [x] SubTask 1.7: 写入执行约束（只建立冻结边界，不重构业务实现，不修改现有行为，不提前实现新插件系统）

- [x] Task 2: 新建后端冻结标记文件并为核心旧类型增加弃用注释
  - [x] SubTask 2.1: 新建 `backend/internal/extension/FROZEN.md`，内容覆盖冻结开始原因、范围、允许/禁止修改类型、新系统规划文档位置、旧系统预计删除说明、代码评审要求
  - [x] SubTask 2.2: 新建 `backend/internal/mcp/FROZEN.md`，覆盖 MCP 子系统（manager、client、transport、auth、discovery、features、host、skill、dependency、protocol、repository）的冻结说明
  - [x] SubTask 2.3: 在 `backend/internal/extension` 核心旧类型与入口增加统一弃用注释（`Deprecated: Legacy extension architecture.`），对象至少包括：`SkillDefinition`/registry.go/executor.go/runtime.go、Plugin Factory/Registry/Manager/Host/Hook、MCP Skill Adapter、Agent Skill 到 SkillDefinition 的适配（`legacy_tool_adapter.go`、`agent_skill_*.go` 适配部分）、Manifest v1 schema 与校验、旧 Package Parser（`package_parser.go`）、旧 Package Installer（`package_installer.go`）
  - [x] SubTask 2.4: 在 `backend/internal/mcp` 核心入口（`manager`、`client`、`host`、`skill`、`dependency`、`protocol`、`model.go`、`repository.go`）增加弃用注释
  - [x] SubTask 2.5: 确保注释只增加说明，不修改任何运行逻辑、不删除任何代码

- [x] Task 3: 新建前端冻结标记文件并为旧页面/路由/API Client 增加弃用说明
  - [x] SubTask 3.1: 新建 `front/src/views/extensions/FROZEN.md`，列出被冻结的前端子模块（ExtensionCenterView、PluginList/Detail、SkillList/Detail、RunHistory、PackageManager、AgentSkillList、Workshop、api.ts、types.ts、components/* 以及 `front/src/views/mcp/`、`front/src/views/creative-workshop/` 相关页面），并指向 `docs/extension-kernel/01-system-freeze.md`
  - [x] SubTask 3.2: 在前端扩展中心核心入口文件增加弃用说明注释：`ExtensionCenterView.vue`、`PluginListView.vue`、`PluginDetailView.vue`、`SkillListView.vue`、`SkillDetailView.vue`、`RunHistoryView.vue`、`packages/PackageManagerView.vue`、`agent-skills/AgentSkillListView.vue`、`workshop/WorkshopListView.vue`、`workshop/WorkshopSessionView.vue`、`api.ts`、`types.ts`
  - [x] SubTask 3.3: 在 `front/src/views/mcp/` 核心入口增加弃用说明：`MCPServerView.vue`、`api.ts`、`types.ts`
  - [x] SubTask 3.4: 在 `front/src/views/creative-workshop/` 入口增加弃用说明：`CreativeWorkshopView.vue` 及 Pet* 系列页面
  - [x] SubTask 3.5: 在前端路由配置中扩展中心相关静态路由与导航项入口增加弃用注释（不修改路由行为）
  - [x] SubTask 3.6: 确保前端注释只增加说明，不修改任何组件行为、不删除任何代码

- [x] Task 4: 新建扩展系统专用 PR 评审模板与旧系统变更策略文档
  - [x] SubTask 4.1: 新建 `.github/pull_request_template_extension.md`，固化提交者必填说明（修改原因、是否缺陷/安全/测试/迁移、是否新增字段、是否新增数据表、是否改变已有行为、是否增加兼容层、后续是否需要删除、对新 Extension Kernel 的影响）
  - [x] SubTask 4.2: 在模板中列出 13 项必查项与 9 项直接拒绝条件
  - [x] SubTask 4.3: 新建 `docs/extension-kernel/legacy-change-policy.md`，写入允许/禁止修改范围、提交类型约束（仅允许 `fix(extension)`、`fix(mcp)`、`test(extension)`、`test(mcp)`、`docs(extension)`、`refactor(extension-foundation)`、`migration(extension)`、`security(extension)`）、提交说明 8 项约束、迁移辅助结构命名与删除计划要求

- [x] Task 5: 新增最小自动冻结检查脚本
  - [x] SubTask 5.1: 新建 `scripts/check-extension-freeze.ps1`（兼容 PowerShell 7）与 `scripts/check-extension-freeze.sh`（如有跨平台需求）
  - [x] SubTask 5.2: 实现 Manifest v1 schema 变更检查：当 `backend/internal/extension/schema/manifest.schema.json` 出现新增 Entry 类型、Runtime、UI、Hook、Provider 时输出违规并以非零退出
  - [x] SubTask 5.3: 实现扩展相关新增数据库表/字段检查：要求提交中明确标记 `permanent` / `migration-only` / `temporary`，迁移辅助结构必须附带删除步骤编号
  - [x] SubTask 5.4: 实现 Registry 增量检查：检测 `NewRegistry`、`RegisterSkill`、`RegisterPlugin`、`RegisterMCP`、`RegisterWorkflow`、`RegisterProvider` 关键词新增，要求证明属于新 Extension Kernel
  - [x] SubTask 5.5: 实现前端路由检查：检测扩展中心新增静态路由和导航项
  - [x] SubTask 5.6: 脚本以非零退出码表示发现违规，并输出违规文件与原因；当前代码基线运行脚本应正常退出（不误报）

- [x] Task 6: 验证冻结不影响现有功能并产出验收记录
  - [x] SubTask 6.1: 执行后端扩展相关测试并记录结果（通过/失败均如实记录）
  - [x] SubTask 6.2: 执行 MCP 相关测试并记录结果
  - [x] SubTask 6.3: 执行前端构建或类型检查并记录结果
  - [x] SubTask 6.4: 启动完整服务（前端、后端、Qdrant、SurrealDB、微信侧车、QQ 侧车）并按 AGENTS.md 规则先清理项目占用，验证现有扩展中心基本访问正常
  - [x] SubTask 6.5: 运行 `scripts/check-extension-freeze` 脚本，确认在当前代码基线不误报
  - [x] SubTask 6.6: 整理弃用标记清单（类型、函数、接口、解析器、适配器、前端页面、路由、API Client）与未解决问题清单，作为验收产物

# Task Dependencies
- [Task 2] depends on [Task 1]
- [Task 3] depends on [Task 1]
- [Task 4] depends on [Task 1]
- [Task 5] depends on [Task 1]
- [Task 6] depends on [Task 1, Task 2, Task 3, Task 4, Task 5]
- [Task 2]、[Task 3]、[Task 4]、[Task 5] 之间相互独立，可并行

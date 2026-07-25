# 冻结现有扩展系统功能开发 Spec

> 来源实施文档：`d:\桌面\跟进项目\U-Ai\.trae\Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`
> 总体规划：`d:\桌面\跟进项目\U-Ai\Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`

## Why

Amitia 扩展系统重构是分步推进的多阶段工程，第 1 步必须在正式重构前建立严格的"功能冻结"保护边界，防止在重构期间继续向旧架构（Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` v1、扩展中心/创意工坊）增加新功能、字段、接口、数据库表和兼容逻辑，导致历史负担进一步扩大。

本步骤不删除旧代码、不重写业务实现、不修改现有行为，只建立冻结说明、弃用标记、评审约束和最小自动检查，确保后续步骤可以清晰区分"旧系统维护"与"新 Extension Kernel 开发"。

## What Changes

- 新增 `docs/extension-kernel/01-system-freeze.md` 冻结总说明文档，覆盖冻结范围、允许修改、禁止修改、退出条件。
- 新增后端冻结标记文件 `backend/internal/extension/FROZEN.md`、`backend/internal/mcp/FROZEN.md`，明确冻结开始原因、范围、允许/禁止修改类型、新系统规划文档位置、旧系统预计删除说明、代码评审要求。
- 新增前端冻结标记文件 `front/src/views/extensions/FROZEN.md`，覆盖前端旧扩展页面、路由和 API Client。
- 在核心旧类型、入口文件、解析器、适配器、Manifest v1、旧 Package Parser/Installer、前端旧页面/路由/API Client 增加 `Deprecated: Legacy extension architecture.` 弃用注释（暂不删除代码）。
- 新增扩展系统专用 PR 评审模板 `.github/pull_request_template_extension.md`，固化必查项与直接拒绝条件。
- 新增最小自动冻结检查脚本 `scripts/check-extension-freeze.*`，至少识别 Manifest v1 新增类型、扩展相关新增表、旧系统新增 Registry、扩展中心新增分散路由。
- 新增 `docs/extension-kernel/legacy-change-policy.md` 旧系统变更策略文档。
- 执行现有测试与最小手工验证，证明新增冻结文件、注释和 CI 规则不改变运行逻辑。

非变更项（本步骤明确不做）：
- 不重写 Skill/MCP/Plugin/Workflow。
- 不设计 Manifest v2 具体字段、不实现 JavaScript Runtime、不实现 UI Contribution。
- 不迁移数据库、不删除旧代码、不修改现有产品结构、不开始创建 `.amitiax` v2 插件、不改变当前用户可见功能。

## Impact

- Affected specs：本仓库当前无对应 capability spec；本规范为该重构步骤 1 自身规范。
- Affected code：
  - 后端：`backend/internal/extension/`（registry.go、runtime.go、executor.go、protocol.go、service.go、handler.go、router.go、package_parser.go、package_installer.go、package_lifecycle.go、plugin_*.go、workflow_*.go、workshop_*.go、agent_skill_*.go、legacy_tool_adapter.go 等）
  - 后端：`backend/internal/mcp/`（manager、client、transport、auth、discovery、features、host、skill、dependency、protocol、model.go、repository.go）
  - 后端：`backend/internal/extension/schema/manifest.schema.json`
  - 前端：`front/src/views/extensions/`（ExtensionCenterView、PluginListView/Detail、SkillListView/Detail、RunHistoryView、packages/PackageManagerView、agent-skills/AgentSkillListView、workshop/WorkshopListView/SessionView、api.ts、types.ts、components/*）
  - 前端：`front/src/views/mcp/`（MCPServerView、api.ts、types.ts）
  - 前端：`front/src/views/creative-workshop/`（CreativeWorkshopView 及 Pet* 系列）
  - 前端路由：扩展中心相关静态路由与导航项
  - 文档：`docs/extension-kernel/`（新建目录）
  - CI/脚本：`.github/pull_request_template_extension.md`、`scripts/check-extension-freeze.*`
- 兼容性：纯文档/注释/CI 改动，不引入任何行为变化，无 **BREAKING**。

## ADDED Requirements

### Requirement: 冻结范围总说明

系统 SHALL 在 `docs/extension-kernel/01-system-freeze.md` 中明确冻结 Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` v1 旧扩展包系统、扩展中心与创意工坊，并覆盖冻结开始原因、冻结范围、允许修改类型、禁止修改类型、新系统规划文档位置、旧系统预计删除说明、代码评审要求。

#### Scenario: 任何人查阅冻结总说明
- **WHEN** 开发者打开 `docs/extension-kernel/01-system-freeze.md`
- **THEN** 文档完整列出被冻结的 7 大对象及其子模块清单
- **AND** 文档列出 6 类允许的修改（阻塞性缺陷、安全、回归测试、可观测性、迁移辅助、文档注释）及其边界
- **AND** 文档列出 8 类禁止的修改（新增扩展类型、扩展旧 Manifest、新增旧系统数据库表、新增平行 Registry、新增永久兼容层、增加旧 Plugin 能力、让 `.amitiax` v1 承担新职责、前端增加分散入口）
- **AND** 文档列出退出条件，明确只有满足条件才能进入第 2 步"建立现有系统调用链地图"

### Requirement: 后端冻结标记文件

系统 SHALL 在 `backend/internal/extension/FROZEN.md` 和 `backend/internal/mcp/FROZEN.md` 中分别提供后端扩展与 MCP 子系统的冻结说明，内容 SHALL 至少包含冻结开始原因、冻结范围、允许修改类型、禁止修改类型、新系统规划文档位置、旧系统预计删除说明、代码评审要求。

#### Scenario: 在后端扩展目录查阅冻结说明
- **WHEN** 开发者打开 `backend/internal/extension/FROZEN.md`
- **THEN** 文档列出本目录被冻结的具体子模块（Registry、Runtime、Executor、Plugin Factory/Manager/Host/Hook、Workflow Compiler/Executor、Agent Skill、Package Parser/Installer、Workshop 等）
- **AND** 文档明确指向 `docs/extension-kernel/01-system-freeze.md` 与新系统规划文档
- **AND** 文档给出旧系统预计删除说明与代码评审要求

#### Scenario: 在后端 MCP 目录查阅冻结说明
- **WHEN** 开发者打开 `backend/internal/mcp/FROZEN.md`
- **THEN** 文档列出被冻结的 MCP 子模块（manager、client、transport、auth、discovery、features、host、skill、dependency、protocol、repository）
- **AND** 文档明确 MCP 不再新增能力，仅允许阻塞性/安全/测试/迁移辅助修改

### Requirement: 前端冻结标记文件

系统 SHALL 在 `front/src/views/extensions/FROZEN.md` 中提供前端扩展中心的冻结说明，覆盖旧扩展页面、路由和 API Client。

#### Scenario: 在前端扩展中心查阅冻结说明
- **WHEN** 开发者打开 `front/src/views/extensions/FROZEN.md`
- **THEN** 文档列出被冻结的前端子模块（ExtensionCenterView、PluginList/Detail、SkillList/Detail、RunHistory、PackageManager、AgentSkillList、Workshop、api.ts、types.ts、components/* 以及 mcp/、creative-workshop/ 相关页面）
- **AND** 文档明确禁止新增独立子页面与分散入口，旧页面仅允许修复不可用、数据错误和安全问题
- **AND** 文档指向 `docs/extension-kernel/01-system-freeze.md`

### Requirement: 旧架构核心入口弃用标记

系统 SHALL 在以下核心旧类型与入口的源码注释中增加统一的弃用说明（暂不删除代码）：

```go
// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.
```

适用对象至少包括：旧 `SkillDefinition`、旧 Plugin Factory、旧 Plugin Registry、MCP Skill Adapter、Agent Skill 到 SkillDefinition 的适配、Manifest v1、旧 Package Parser、旧 Package Installer。前端旧页面、路由和 API Client SHALL 增加对应弃用注释。

#### Scenario: 在旧类型上看到弃用注释
- **WHEN** 开发者打开任意被冻结的核心旧类型/入口源码
- **THEN** 文件顶部或类型/函数声明处能看到 `Deprecated: Legacy extension architecture.` 注释块
- **AND** 注释明确"不新增能力，仅保留兼容/维护/测试/迁移用途"

### Requirement: 扩展系统变更评审模板

系统 SHALL 新增 `.github/pull_request_template_extension.md`，固化涉及冻结范围 PR 的必查项与直接拒绝条件。

#### Scenario: 提交涉及旧扩展系统的 PR
- **WHEN** 开发者创建涉及 `backend/internal/extension`、`backend/internal/mcp`、`front/src/views/extensions`、`front/src/views/mcp`、`front/src/views/creative-workshop`、`backend/internal/extension/schema/manifest.schema.json` 的 PR
- **THEN** PR 模板要求提交者说明修改原因、是否属于缺陷/安全/测试/迁移、是否新增字段、是否新增数据表、是否改变已有行为、是否增加兼容层、后续是否需要删除、对新 Extension Kernel 的影响
- **AND** 模板列出必查项（是否增加新产品能力、扩大旧架构职责、增加永久兼容层、增加重复状态、新增数据库表、新增 Registry、新增权限判断、新增独立生命周期、直接修改 Manifest v1、增加新 UI 入口、影响后续迁移、是否包含测试、是否有回滚方式）
- **AND** 模板列出直接拒绝条件（在旧 Plugin 上增加第三方插件能力、在旧 `.amitiax` 中增加代码运行时、新建另一套 Tool/Skill Registry、新增重复权限系统、新增与现有表功能重复的数据表、通过双写维持新旧系统长期并行、修改旧系统但未提供必要测试、修改核心执行链但未说明迁移影响、将前端页面状态继续绑定到旧系统内部实现）

### Requirement: 最小自动冻结检查

系统 SHALL 新增最小自动冻结检查脚本 `scripts/check-extension-freeze.*`，至少识别以下违规：
- `backend/internal/extension/schema/manifest.schema.json` 出现功能性字段增加（新增 Entry 类型、Runtime、UI、Hook、Provider）
- 扩展相关新增数据库表/字段（要求提交中明确标记 permanent / migration-only / temporary，迁移辅助结构必须附带删除步骤编号）
- 旧系统新增 Registry（关键词 `NewRegistry`、`RegisterSkill`、`RegisterPlugin`、`RegisterMCP`、`RegisterWorkflow`、`RegisterProvider`）
- 扩展中心新增静态路由和导航项

#### Scenario: 检查脚本发现 Manifest v1 新增 Entry 类型
- **WHEN** Manifest v1 schema 增加了新的 Entry 类型
- **AND** 运行 `scripts/check-extension-freeze` 脚本
- **THEN** 脚本以非零退出码退出
- **AND** 输出指出违规文件与原因

#### Scenario: 检查脚本发现旧系统新增 Registry
- **WHEN** 后端代码在旧扩展系统目录新增 `RegisterSkill` / `RegisterPlugin` / `RegisterMCP` / `RegisterWorkflow` / `RegisterProvider` 等注册点
- **AND** 该注册点不属于新 Extension Kernel
- **AND** 运行检查脚本
- **THEN** 脚本以非零退出码退出并输出违规位置

### Requirement: 旧系统变更策略文档

系统 SHALL 新增 `docs/extension-kernel/legacy-change-policy.md`，说明冻结期间旧系统修改的允许范围、禁止范围、提交类型约束、提交说明约束、迁移辅助结构命名与删除计划要求。

#### Scenario: 开发者需要修改旧系统
- **WHEN** 开发者准备修改旧扩展系统
- **THEN** 文档明确允许的修改类型与边界
- **AND** 文档明确禁止的修改类型
- **AND** 文档明确提交类型只能使用 `fix(extension)`、`fix(mcp)`、`test(extension)`、`test(mcp)`、`docs(extension)`、`refactor(extension-foundation)`、`migration(extension)`、`security(extension)`，禁止 `feat(skill)`、`feat(plugin)`、`feat(mcp)`、`feat(workflow)`、`feat(package)`
- **AND** 文档明确每个提交必须说明的 8 项约束

### Requirement: 冻结不改变运行行为

冻结相关改动 SHALL 不改变现有运行逻辑、不新增产品能力、不新增永久业务数据表、不新增 Registry/Runtime/独立生命周期。

#### Scenario: 完成冻结后回归测试
- **WHEN** 完成所有冻结文件、弃用注释和检查脚本
- **THEN** 后端扩展相关测试通过（或如实记录失败）
- **AND** MCP 相关测试通过（或如实记录失败）
- **AND** 前端构建或类型检查通过（或如实记录失败）
- **AND** 现有扩展中心基本访问正常（或如实记录失败）
- **AND** 未引入任何新产品能力

## MODIFIED Requirements

本步骤为新增冻结约束，无对既有 spec 的修改。

## REMOVED Requirements

本步骤不删除任何需求；旧系统代码也暂不删除。

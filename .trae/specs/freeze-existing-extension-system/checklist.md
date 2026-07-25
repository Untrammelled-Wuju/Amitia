# Checklist

## 冻结总说明
- [x] `docs/extension-kernel/01-system-freeze.md` 已创建
- [x] 冻结总说明覆盖 7 大冻结对象（Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` v1、扩展中心与创意工坊）及其子模块清单
- [x] 冻结总说明列出 6 类允许修改类型及边界
- [x] 冻结总说明列出 8 类禁止修改类型
- [x] 冻结总说明包含代码级冻结标记要求、分支与提交策略、代码评审约束、自动检查建议
- [x] 冻结总说明包含退出条件（满足后才能进入第 2 步）
- [x] 冻结总说明包含执行约束（不重构业务实现、不修改现有行为、不提前实现新插件系统）

## 后端冻结标记
- [x] `backend/internal/extension/FROZEN.md` 已创建
- [x] `backend/internal/mcp/FROZEN.md` 已创建
- [x] 两个 FROZEN.md 至少包含：冻结开始原因、冻结范围、允许修改类型、禁止修改类型、新系统规划文档位置、旧系统预计删除说明、代码评审要求
- [x] `backend/internal/extension` 核心旧类型已增加 `Deprecated: Legacy extension architecture.` 注释（至少覆盖 SkillDefinition/Registry/Executor/Runtime、Plugin Factory/Registry/Manager/Host/Hook、MCP Skill Adapter、Agent Skill 适配、Manifest v1、Package Parser、Package Installer）
- [x] `backend/internal/mcp` 核心入口已增加弃用注释（manager/client/host/skill/dependency/protocol/model.go/repository.go）
- [x] 注释只增加说明，未修改任何运行逻辑、未删除任何代码

## 前端冻结标记
- [x] `front/src/views/extensions/FROZEN.md` 已创建
- [x] 前端 FROZEN.md 列出被冻结的前端子模块并指向 `docs/extension-kernel/01-system-freeze.md`
- [x] 扩展中心核心入口已增加弃用说明（ExtensionCenterView、PluginList/Detail、SkillList/Detail、RunHistory、PackageManager、AgentSkillList、Workshop List/Session、api.ts、types.ts）
- [x] `front/src/views/mcp/` 核心入口已增加弃用说明（MCPServerView、api.ts、types.ts）
- [x] `front/src/views/creative-workshop/` 入口已增加弃用说明（CreativeWorkshopView 及 Pet* 系列）
- [x] 前端路由配置中扩展中心相关静态路由与导航项入口已增加弃用注释
- [x] 前端注释只增加说明，未修改任何组件行为、未删除任何代码

## 评审模板与变更策略
- [x] `.github/pull_request_template_extension.md` 已创建
- [x] PR 模板要求提交者填写 8 项说明（修改原因、缺陷/安全/测试/迁移归属、新增字段、新增数据表、改变已有行为、增加兼容层、后续是否删除、对新 Extension Kernel 影响）
- [x] PR 模板列出 13 项必查项
- [x] PR 模板列出 9 项直接拒绝条件
- [x] `docs/extension-kernel/legacy-change-policy.md` 已创建
- [x] 变更策略文档列出允许的提交类型与禁止的提交类型
- [x] 变更策略文档列出提交说明 8 项约束
- [x] 变更策略文档明确迁移辅助结构命名（migration/snapshot/temporary）与删除计划要求

## 最小自动冻结检查
- [x] `scripts/check-extension-freeze.ps1` 已创建（PowerShell 7 兼容）
- [x] 脚本能检测 Manifest v1 schema 新增 Entry 类型/Runtime/UI/Hook/Provider
- [x] 脚本能检测扩展相关新增数据库表/字段并要求标记 permanent/migration-only/temporary 及删除步骤
- [x] 脚本能检测旧系统新增 Registry（NewRegistry/RegisterSkill/RegisterPlugin/RegisterMCP/RegisterWorkflow/RegisterProvider）
- [x] 脚本能检测扩展中心新增静态路由和导航项
- [x] 脚本以非零退出码表示违规并输出违规文件与原因
- [x] 在当前代码基线运行脚本不误报（正常退出）

## 验证与回归
- [x] 后端扩展相关测试已执行并如实记录结果
- [x] MCP 相关测试已执行并如实记录结果
- [x] 前端构建或类型检查已执行并如实记录结果
- [x] 完整服务（前端、后端、Qdrant、SurrealDB、微信侧车、QQ 侧车）已按 AGENTS.md 规则清理占用后重启并验证扩展中心基本访问
- [x] `scripts/check-extension-freeze` 脚本已在当前代码基线运行且不误报
- [x] 弃用标记清单已整理（类型、函数、接口、解析器、适配器、前端页面、路由、API Client）
- [x] 未解决问题清单已如实记录，无隐瞒或伪报

## 验收标准（与实施文档第十三节对齐）
- [x] 已明确冻结 Skill、Agent Skill、MCP、Plugin、Workflow、旧 `.amitiax` 和扩展中心
- [x] 已建立正式冻结说明文件
- [x] 核心旧架构入口已增加弃用标记
- [x] 不包含任何新扩展功能
- [x] 不新增永久业务数据表
- [x] 不新增 Registry、Runtime 或独立生命周期
- [x] 不改变现有扩展功能行为
- [x] 已建立旧系统修改的评审约束
- [x] 已具备最低限度自动冻结检查
- [x] 已完成现有功能的基础回归验证
- [x] 所有失败测试和无法验证项均被如实记录
- [x] 后续开发能够明确区分"旧系统维护"与"新 Extension Kernel 开发"

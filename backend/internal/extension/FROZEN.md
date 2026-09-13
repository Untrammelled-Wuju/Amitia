# 旧扩展系统功能冻结说明（backend/internal/extension）

> 冻结开始日期：2026-07-25
> 本步骤地位：Amitia 扩展系统重构与 Amitiax 插件平台规划的第 1 步
> 总体规划文档：[`Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`](../../../Amitia_扩展系统重构与Amitiax插件平台步骤规划.md)
> 冻结总说明：[`docs/extension-kernel/01-system-freeze.md`](../../../docs/extension-kernel/01-system-freeze.md)
> 实施依据：[`.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`](../../../.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md)

---

## 一、冻结开始原因

本目录为 Amitia 旧扩展系统后端核心实现所在。旧 Skill Registry、Executor、Handler、Legacy Tool Adapter、Workflow Compiler/Executor 与 Workshop 已完成迁移和物理删除；Agent Skill 已切换到数据库状态与 Extension Kernel ToolFacade，Workflow 使用 Extension Kernel。

为推进 Amitia 扩展系统重构与 Amitiax 插件平台规划的第 1 步"冻结现有扩展系统功能开发"，自 2026-07-25 起对本目录建立重构保护边界，防止在重构期间继续向旧架构增加功能、字段、接口、数据库表和兼容逻辑，避免历史遗留问题继续扩大。

本步骤只建立重构保护边界，不重构业务实现、不修改现有行为、不提前实现新插件系统。本目录进入如下状态：

> 只允许修复阻塞性缺陷、补充测试和增加迁移辅助能力，不再允许新增产品能力和扩展旧架构。

---

## 二、冻结范围

本目录被冻结的具体子模块包括：

- **Runtime（运行时装配）**：`runtime.go`
- **Protocol（Skill 协议与类型）**：`protocol.go`
- **Handler（HTTP 处理器）**：`handler.go`
- **Router（路由注册）**：`router.go`
- **Agent Skill**：`agent_skill_handler.go`、`agent_skill_runtime.go`、`agent_skill_parser.go`、`agent_skill_protocol.go`、`agent_skill_repository.go`、`agent_skill_service.go`、`agent_skill_metrics.go`
- **Package Parser / Installer / Lifecycle（`.amitiax` 旧扩展包系统）**：`package_parser.go`、`package_installer.go`、`package_lifecycle.go`、`package_archive.go`、`package_handler.go`、`package_protocol.go`、`package_recovery.go`、`package_repository.go`、`package_service.go`、`package_test_runner.go`
- **Schema（Manifest v1 校验）**：`schema_validator.go` 与 `schema/manifest.schema.json`

冻结范围还覆盖本目录下与上述模块相关的权限、能力、生命周期、归属资源、配置加密等实现文件（`capability.go`、`permission.go`、`lifecycle_service.go`、`owned_resource_repository.go`、`config_crypto.go`、`repository.go` 等）。

---

## 三、允许修改类型

冻结不是完全禁止修改。以下 6 类修改允许进行，但必须严格限制范围（详见 `docs/extension-kernel/01-system-freeze.md` 第三节）：

1. **阻塞性缺陷修复**：仅允许修复会导致应用无法启动、扩展系统启动崩溃、数据损坏、数据丢失、权限绕过、密钥泄露、路径穿越、任意代码执行、MCP 连接导致主程序崩溃、扩展执行导致主聊天链路不可用、无法卸载或恢复扩展、已有功能完全不可用、阻塞后续重构或迁移的缺陷。不得借"修复缺陷"名义顺带增加新功能。
2. **安全修复**：包解析安全、归档解压安全、签名校验、Secret 加密、权限校验、作用域越权、插件运行隔离、MCP 认证安全、敏感日志泄露、输入参数验证、SQL 注入、命令注入、路径注入等。安全修复必须保持对现有行为的最小影响。
3. **回归测试补充**：单元测试、集成测试、数据库迁移测试、包解析测试、权限测试、Agent Skill 解析测试、Workflow 执行测试、Plugin 生命周期测试、启动恢复测试、卸载清理测试等。测试代码不得推动旧架构继续扩展。
4. **可观测性补充**：日志、Trace、Metrics、调试开关、执行耗时统计、启动恢复日志、权限拒绝日志、数据迁移诊断信息。新增日志不得记录 API Key、OAuth Token、Cookie、完整聊天内容、用户私密数据、插件 Secret、数据库连接凭据。
5. **迁移辅助能力**：只读查询接口、数据导出、状态快照、旧数据扫描、旧资源所有权识别、依赖关系分析、数据一致性检测、临时迁移标记、旧版本数据转换工具。迁移辅助能力不得成为新的永久业务接口。
6. **文档与注释**：架构说明、历史兼容说明、弃用标记、迁移说明、风险说明、测试说明、数据表说明、启动顺序说明。

---

## 四、禁止修改类型

以下 8 类修改全部禁止（详见 `docs/extension-kernel/01-system-freeze.md` 第四节）：

1. **禁止新增扩展类型**：不得在旧系统新增新 Skill 类型、新 Plugin 类型、新 MCP 包装类型、新 Workflow 类型、新扩展包 Entry、新 Provider 类型、新 UI 扩展类型、新 Hook 类型。
2. **禁止扩展旧 Manifest**：不得继续向旧 Manifest v1 增加新顶层字段、新 Entry 类型、新权限语法、新依赖语法、新 Runtime 字段、新 UI 声明、新 Hook 声明、新 Provider 声明。旧 Manifest 只允许修复解析错误和安全问题。
3. **禁止新增旧系统数据库表**：不得为旧系统新增 Skill 表、Plugin 表、MCP 表、Workflow 表、Package 表、Hook 表、UI Contribution 表、Provider 表、新的重复审计表、新的重复启用状态表。确实需要迁移辅助表时，必须满足：名称明确带有 migration、snapshot 或 temporary；有删除计划；不承载永久业务功能；在后续迁移步骤中明确清理。
4. **禁止新增平行 Registry**：不得新增第二套 Tool/Skill/MCP/Plugin/Workflow/UI Registry、第二套权限中心、第二套执行器。
5. **禁止新增永久兼容层**：不得为了新功能增加新旧字段双写、新旧状态同步、新旧接口桥接、新旧 Registry 双注册、新旧权限双判定、新旧数据双存储。后续允许存在一次性迁移适配器，但必须可删除并有明确退出条件。
6. **禁止恢复旧 Plugin 能力**：不得恢复已删除的 Go 内置 Plugin Runtime、Plugin Manager、Plugin Registry、Plugin Host、Plugin Service、Plugin Hook、Plugin Surface、Plugin Event、Plugin Schedule 或旧 Plugin API。
7. **禁止让 `.amitiax` v1 承担新职责**：不得让旧 `.amitiax` 支持第三方运行时代码、JavaScript、WASM、UI 页面、Electron 扩展、Provider、后台服务、消息渲染器、桌面组件。这些能力必须在后续 `.amitiax` Manifest v1 中统一设计。
8. **禁止前端继续增加分散入口**：不得新增独立 Skill 子页面、独立 MCP 子系统入口、独立 Plugin 子系统入口、独立 Workflow 子系统入口、新的扩展包入口、新的重复运行记录页面。旧页面只允许修复无法使用、数据错误和安全问题。

---

## 五、新系统规划文档位置

- 冻结总说明：[`docs/extension-kernel/01-system-freeze.md`](../../../docs/extension-kernel/01-system-freeze.md)
- 总体规划：[`Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`](../../../Amitia_扩展系统重构与Amitiax插件平台步骤规划.md)
- 第 1 步实施文档：[`.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`](../../../.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md)

---

## 六、旧系统预计删除说明

旧 Skill Registry、Executor、Handler、Legacy Tool Adapter、Workflow Compiler/Executor 与 Workshop 已完成迁移和物理删除。其余旧 `.amitiax` 包启动迁移链路继续作为一次性迁移辅助保留。

删除前保留用于：

- 兼容现有用户可见功能；
- 旧版本维护与缺陷修复；
- 回归测试基线；
- 迁移到新 Extension Kernel 的对照参考与数据来源。

删除前不得新增能力。任何新增能力必须直接进入新 Extension Kernel，不得进入本目录。

---

## 七、代码评审要求

任何涉及本目录的 Pull Request 必须对照 `docs/extension-kernel/01-system-freeze.md` 第七节进行评审：

### 必查项（13 项）

- 是否增加新产品能力；
- 是否扩大旧架构职责；
- 是否增加永久兼容层；
- 是否增加重复状态；
- 是否新增数据库表；
- 是否新增 Registry；
- 是否新增权限判断；
- 是否新增独立生命周期；
- 是否直接修改 Manifest v1；
- 是否增加新 UI 入口；
- 是否影响后续迁移；
- 是否包含测试；
- 是否有回滚方式。

### 直接拒绝条件（9 项）

出现以下任意情况，PR 应直接拒绝：

- 恢复已删除的旧 Plugin Runtime 或旧 Plugin API；
- 在旧 `.amitiax` 中增加代码运行时；
- 新建另一套 Tool/Skill Registry；
- 新增重复权限系统；
- 新增与现有表功能重复的数据表；
- 通过双写维持新旧系统长期并行；
- 修改旧系统但未提供必要测试；
- 修改核心执行链但未说明迁移影响；
- 将前端页面状态继续绑定到旧系统内部实现。

### 评审模板

涉及本目录的 PR 必须使用 `.github/pull_request_template_extension.md` 模板（若该模板暂不存在，按上述必查项与拒绝条件逐项说明）。

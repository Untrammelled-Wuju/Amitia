# 旧 MCP 系统功能冻结说明（backend/internal/mcp）

> 冻结开始日期：2026-07-25
> 本步骤地位：Amitia 扩展系统重构与 Amitiax 插件平台规划的第 1 步
> 总体规划文档：[`Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`](../../../Amitia_扩展系统重构与Amitiax插件平台步骤规划.md)
> 冻结总说明：[`docs/extension-kernel/01-system-freeze.md`](../../../docs/extension-kernel/01-system-freeze.md)
> 实施依据：[`.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`](../../../.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md)

---

## 一、冻结开始原因

本目录为 Amitia 旧 MCP（Model Context Protocol）系统后端实现所在，承载 MCP Server 管理、MCP Client、stdio/Streamable HTTP Transport、OAuth、Discovery、Tools/Resources/Prompts/Tasks/Sampling/Elicitation/Roots、MCP 与角色绑定、MCP 与 Agent Skill 依赖、MCP 管理接口等历史职责。

为推进 Amitia 扩展系统重构与 Amitiax 插件平台规划的第 1 步"冻结现有扩展系统功能开发"，自 2026-07-25 起对本目录建立重构保护边界，防止在重构期间继续向旧 MCP 架构增加功能、字段、接口、数据库表和兼容逻辑，避免历史遗留问题继续扩大。

本步骤只建立重构保护边界，不重构业务实现、不修改现有行为、不提前实现新插件系统。本目录进入如下状态：

> 只允许修复阻塞性缺陷、补充测试和增加迁移辅助能力，不再允许新增产品能力和扩展旧架构。MCP 不再新增能力，仅允许阻塞性/安全/测试/迁移辅助修改。

---

## 二、冻结范围

本目录被冻结的具体子模块包括：

- **manager/**：MCP Server 管理器（连接生命周期、配置增删改查、启停）
- **client/**：MCP Client（连接、请求管理、JSON-RPC 调用）
- **transport/**：stdio Transport、Streamable HTTP Transport、Transport 抽象与安全控制
- **auth/**：OAuth 流程、PKCE、Token 存储
- **discovery/**：MCP 工具发现与元数据缓存
- **features/**：Tools、Resources、Prompts、Tasks、Sampling、Elicitation 等能力封装
- **host/**：Roots、Host 交互、Host Service
- **skill/**：MCP Skill 适配 Runtime（MCP Skill 到 SkillDefinition 适配）
- **dependency/**：MCP 与 Agent Skill 依赖解析与启动恢复
- **protocol/**：JSON-RPC 消息、错误码、协议版本
- **model.go**：MCP Server 数据模型
- **repository.go**：MCP Server 持久化仓库

冻结范围覆盖上述子目录下所有 `.go` 源码（含测试代码），以及本目录下与 MCP 相关的数据库结构、迁移脚本和 API 处理器。

---

## 三、允许修改类型

冻结不是完全禁止修改。以下 6 类修改允许进行，但必须严格限制范围（详见 `docs/extension-kernel/01-system-freeze.md` 第三节）：

1. **阻塞性缺陷修复**：仅允许修复会导致应用无法启动、MCP 子系统启动崩溃、数据损坏、数据丢失、权限绕过、密钥泄露、路径穿越、任意代码执行、MCP 连接导致主程序崩溃、扩展执行导致主聊天链路不可用、无法卸载或恢复 MCP Server、已有功能完全不可用、阻塞后续重构或迁移的缺陷。不得借"修复缺陷"名义顺带增加新功能。
2. **安全修复**：MCP 认证安全、OAuth 安全、Token 存储安全、Streamable HTTP 安全、stdio 启动安全、敏感日志泄露、输入参数验证、SQL 注入、命令注入、路径注入等。安全修复必须保持对现有行为的最小影响。
3. **回归测试补充**：单元测试、集成测试、MCP 连接测试、OAuth 流程测试、Transport 测试、Discovery 测试、依赖解析测试、启动恢复测试等。测试代码不得推动旧架构继续扩展。
4. **可观测性补充**：日志、Trace、Metrics、调试开关、连接耗时统计、启动恢复日志、权限拒绝日志、数据迁移诊断信息。新增日志不得记录 API Key、OAuth Token、Cookie、完整聊天内容、用户私密数据、插件 Secret、数据库连接凭据。
5. **迁移辅助能力**：只读查询接口、数据导出、状态快照、旧数据扫描、旧资源所有权识别、依赖关系分析、数据一致性检测、临时迁移标记、旧版本数据转换工具。迁移辅助能力不得成为新的永久业务接口。
6. **文档与注释**：架构说明、历史兼容说明、弃用标记、迁移说明、风险说明、测试说明、数据表说明、启动顺序说明。

---

## 四、禁止修改类型

以下 8 类修改全部禁止（详见 `docs/extension-kernel/01-system-freeze.md` 第四节）：

1. **禁止新增扩展类型**：不得在旧 MCP 系统新增新 MCP 包装类型、新 Provider 类型、新 Hook 类型、新 Tool 类型、新 Resource 类型、新 Prompt 类型、新 Task 类型等。
2. **禁止扩展旧 Manifest**：不得继续向旧 MCP 配置或 Manifest 增加新顶层字段、新能力声明、新权限语法、新依赖语法、新 Runtime 字段、新 UI 声明、新 Hook 声明、新 Provider 声明。
3. **禁止新增旧系统数据库表**：不得为旧 MCP 系统新增 MCP 表、Server 表、连接审计表、新的重复审计表、新的重复启用状态表。确实需要迁移辅助表时，必须满足：名称明确带有 migration、snapshot 或 temporary；有删除计划；不承载永久业务功能；在后续迁移步骤中明确清理。
4. **禁止新增平行 Registry**：不得新增第二套 MCP Tool Registry、第二套 MCP Server Registry、第二套权限中心、第二套执行器、第二套 Discovery 缓存。
5. **禁止新增永久兼容层**：不得为了新功能增加新旧字段双写、新旧状态同步、新旧接口桥接、新旧 Registry 双注册、新旧权限双判定、新旧数据双存储。后续允许存在一次性迁移适配器，但必须可删除并有明确退出条件。
6. **禁止增加旧 Plugin 能力**：不得继续扩展当前 MCP 系统的 Host 能力、Hook、Surface、事件、定时任务、动态加载方式、第三方入口。当前 MCP Runtime 只允许修复稳定性、安全性和迁移阻塞问题。
7. **禁止让 `.amitiax` v1 承担新职责**：不得让旧 `.amitiax` 包通过 MCP 承担第三方运行时代码、JavaScript、WASM、UI 页面、Electron 扩展、Provider、后台服务、消息渲染器、桌面组件等新职责。这些能力必须在后续 `.amitiax` Manifest v2 中统一设计。
8. **禁止前端继续增加分散入口**：不得新增独立 MCP 子系统入口、新的重复 MCP 管理页面、新的重复运行记录页面。旧页面只允许修复无法使用、数据错误和安全问题。

---

## 五、新系统规划文档位置

- 冻结总说明：[`docs/extension-kernel/01-system-freeze.md`](../../../docs/extension-kernel/01-system-freeze.md)
- 总体规划：[`Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`](../../../Amitia_扩展系统重构与Amitiax插件平台步骤规划.md)
- 第 1 步实施文档：[`.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`](../../../.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md)

---

## 六、旧系统预计删除说明

旧 MCP 系统（本目录下 manager、client、transport、auth、discovery、features、host、skill、dependency、protocol、model.go、repository.go 等）将在后续 Extension Kernel 重构步骤完成迁移后删除。

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

- 在旧 Plugin 上增加第三方插件能力；
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

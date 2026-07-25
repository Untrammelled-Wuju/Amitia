# 前端旧扩展系统功能冻结说明

> 冻结开始日期：2026-07-25
> 本文件地位：Amitia 扩展系统重构第 1 步前端冻结标记
> 总体规划文档：[`../../../Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`](../../../Amitia_扩展系统重构与Amitiax插件平台步骤规划.md)
> 冻结总说明：[`../../../docs/extension-kernel/01-system-freeze.md`](../../../docs/extension-kernel/01-system-freeze.md)
> 实施依据：[`../../../.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`](../../../.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md)

---

## 一、冻结开始原因

本文件是 Amitia 扩展系统重构第 1 步"冻结现有扩展系统功能开发"的前端冻结标记，目的是在正式重构前建立明确的重构保护边界。

冻结开始日期为 2026-07-25。冻结后，本文件覆盖范围内的前端旧页面、路由和 API Client 不再允许新增产品能力和扩展旧架构，只允许修复阻塞性缺陷、安全问题、补充测试和增加迁移辅助能力。

冻结边界与后续步骤的关系详见 `docs/extension-kernel/01-system-freeze.md`，新系统整体规划详见 `Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`。

---

## 二、冻结范围

### 1. 扩展中心（`front/src/views/extensions/`）

被冻结的入口文件与子模块：

- `ExtensionCenterView.vue`：扩展中心主入口；
- `PluginListView.vue`：插件列表页；
- `PluginDetailView.vue`：插件详情页；
- `SkillListView.vue`：技能列表页；
- `SkillDetailView.vue`：技能详情页；
- `RunHistoryView.vue`：执行记录页；
- `packages/PackageManagerView.vue`：扩展包管理页；
- `agent-skills/AgentSkillListView.vue`：Agent Skill 列表页；
- `workshop/WorkshopListView.vue`：工坊列表页；
- `workshop/WorkshopSessionView.vue`：工坊会话页；
- `api.ts`：扩展中心 API Client；
- `types.ts`：扩展中心类型定义；
- `components/`：扩展中心共用组件，包括：
  - `ExtensionPageHeader.vue`
  - `PermissionDialog.vue`
  - `SchemaSurfaceRenderer.vue`
  - `SurfaceAction.vue`
  - `SurfaceForm.vue`
  - `SurfaceStatus.vue`
  - `SurfaceTable.vue`

### 2. MCP 页面（`front/src/views/mcp/`）

被冻结的入口文件：

- `MCPServerView.vue`：MCP Server 管理页；
- `api.ts`：MCP API Client；
- `types.ts`：MCP 类型定义。

### 3. 创意工坊（`front/src/views/creative-workshop/`）

被冻结的入口文件：

- `CreativeWorkshopView.vue`：创意工坊主入口；
- `PetCreationView.vue`：宠物创建页；
- `PetHubView.vue`：宠物中心页；
- `PetInstallationsView.vue`：宠物安装页；
- `PetProcessingReviewView.vue`：宠物处理审阅页；
- `PetTaskListView.vue`：宠物任务列表页。

### 4. 前端路由与导航

被冻结的路由配置：

- `front/src/router/index.ts` 中扩展中心、MCP、创意工坊相关静态路由定义与导航项入口（仅注释标记，路由行为不改变）。

---

## 三、允许的修改类型

冻结期间仅允许以下 6 类修改，详细定义见 `docs/extension-kernel/01-system-freeze.md` 第三章：

1. **阻塞性缺陷修复**：仅限导致应用无法启动、扩展系统启动崩溃、数据损坏或丢失、权限绕过、密钥泄露、路径穿越、任意代码执行、MCP 连接导致主程序崩溃、扩展执行导致主聊天链路不可用、无法卸载或恢复扩展、已有功能完全不可用、阻塞后续重构或迁移的缺陷。不得借修复缺陷名义顺带增加新功能。
2. **安全修复**：包解析安全、归档解压安全、签名校验、Secret 加密、权限校验、作用域越权、插件运行隔离、MCP 认证安全、敏感日志泄露、输入参数验证、SQL 注入、命令注入、路径注入等。安全修复必须保持对现有行为的最小影响。
3. **回归测试补充**：单元测试、集成测试、数据库迁移测试、包解析测试、权限测试、MCP 连接测试、Agent Skill 解析测试、Workflow 执行测试、Plugin 生命周期测试、启动恢复测试、卸载清理测试。测试代码不得推动旧架构继续扩展。
4. **可观测性补充**：日志、Trace、Metrics、调试开关、执行耗时统计、启动恢复日志、权限拒绝日志、数据迁移诊断信息。新增日志不得记录 API Key、OAuth Token、Cookie、完整聊天内容、用户私密数据、插件 Secret、数据库连接凭据。
5. **迁移辅助能力**：只读查询接口、数据导出、状态快照、旧数据扫描、旧资源所有权识别、依赖关系分析、数据一致性检测、临时迁移标记、旧版本数据转换工具。迁移辅助能力不得成为新的永久业务接口。
6. **文档与注释**：架构说明、历史兼容说明、弃用标记、迁移说明、风险说明、测试说明、数据表说明、启动顺序说明。

> 旧页面仅允许修复无法使用、数据错误和安全问题。

---

## 四、禁止的修改类型

冻结期间严禁以下 8 类修改，详细定义见 `docs/extension-kernel/01-system-freeze.md` 第四章：

1. **禁止新增扩展类型**：不得在旧系统新增新 Skill 类型、新 Plugin 类型、新 MCP 包装类型、新 Workflow 类型、新扩展包 Entry、新 Provider 类型、新 UI 扩展类型、新 Hook 类型。
2. **禁止扩展旧 Manifest**：不得继续向旧 Manifest v1 增加新顶层字段、新 Entry 类型、新权限语法、新依赖语法、新 Runtime 字段、新 UI 声明、新 Hook 声明、新 Provider 声明。旧 Manifest 只允许修复解析错误和安全问题。
3. **禁止新增旧系统数据库表**：不得为旧系统新增 Skill、Plugin、MCP、Workflow、Package、Hook、UI Contribution、Provider 等业务表，也不得新增重复审计表或重复启用状态表。迁移辅助表必须满足命名、删除计划、不承载永久业务功能、明确清理步骤等条件。
4. **禁止新增平行 Registry**：不得新增第二套 Tool/Skill/MCP Tool/Plugin/Workflow/UI Registry、第二套权限中心或第二套执行器。
5. **禁止新增永久兼容层**：不得为了新功能增加新旧字段双写、新旧状态同步、新旧接口桥接、新旧 Registry 双注册、新旧权限双判定、新旧数据双存储。后续允许存在一次性迁移适配器，但必须可删除并有明确退出条件。
6. **禁止增加旧 Plugin 能力**：不得继续扩展当前 Go 内置 Plugin 模型，包括新 Host API、新 Hook、新 Surface、新事件、新定时任务类型、新动态加载方式、新第三方 Plugin 入口。
7. **禁止让 `.amitiax` v1 承担新职责**：不得让旧 `.amitiax` 支持第三方运行时代码、JavaScript、WASM、UI 页面、Electron 扩展、Provider、后台服务、消息渲染器、桌面组件。这些能力必须在后续 `.amitiax` Manifest v2 中统一设计。
8. **禁止前端继续增加分散入口**：不得新增独立 Skill 子页面、独立 MCP 子系统入口、独立 Plugin 子系统入口、独立 Workflow 子系统入口、新的扩展包入口、新的重复运行记录页面。旧页面只允许修复无法使用、数据错误和安全问题。

---

## 五、新系统规划文档位置

- 总体规划：`Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`（位于项目根目录）；
- 冻结总说明：`docs/extension-kernel/01-system-freeze.md`；
- 第 1 步实施文档：`.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`。

新 Extension Kernel 与 `.amitiax` Manifest v2 的具体设计在后续步骤中统一进行，本步骤不提前实现。

---

## 六、旧系统预计删除说明

本文件覆盖的前端旧页面、路由和 API Client 在重构期间保留，目的是兼容、维护、测试和迁移到 Extension Kernel。

- 旧系统不立即删除，避免在重构期间破坏现有用户可见功能；
- 旧系统预计在新 Extension Kernel 完成对应能力并完成数据迁移、用户引导迁移后，按总体规划文档中的步骤分批下线；
- 下线前必须满足：新系统已具备等价或更优能力、数据迁移已完成、用户引导已切换、回归测试已通过；
- 下线操作必须以独立提交进行，并在提交说明中明确删除范围、删除原因、回滚方式和对新 Extension Kernel 的影响。

在旧系统正式删除前，本 FROZEN.md 持续有效，任何对本文件覆盖范围的修改都必须遵守本文件的允许与禁止约束。

---

## 七、代码评审要求

任何涉及本文件覆盖范围的 Pull Request 都必须按 `docs/extension-kernel/01-system-freeze.md` 第七章进行评审。

### 1. 必查项

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

### 2. 直接拒绝条件

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

### 3. 提交约束

每个涉及本文件覆盖范围的提交必须说明：

- 修改原因；
- 是否属于缺陷、安全、测试或迁移；
- 是否新增字段；
- 是否新增数据表；
- 是否改变已有行为；
- 是否增加兼容层；
- 后续是否需要删除；
- 对新 Extension Kernel 的影响。

### 4. 允许与禁止的提交类型

允许：

```text
fix(extension):
fix(mcp):
test(extension):
test(mcp):
docs(extension):
refactor(extension-foundation):
migration(extension):
security(extension):
```

禁止（除非明确属于新 Extension Kernel 且不修改旧系统职责）：

```text
feat(skill):
feat(plugin):
feat(mcp):
feat(workflow):
feat(package):
```

---

## 八、本文件覆盖范围内的弃用注释

为便于在代码中识别冻结范围，本文件覆盖的核心入口文件已在文件顶部增加统一弃用注释。

`.vue` 文件使用：

```vue
<!--
Deprecated: Legacy extension architecture.
Do not add new capabilities. This view is retained only for
compatibility, maintenance, testing, and migration to Extension Kernel.
-->
```

`.ts` 文件使用：

```ts
/**
 * Deprecated: Legacy extension architecture.
 * Do not add new capabilities. This module is retained only for
 * compatibility, maintenance, testing, and migration to Extension Kernel.
 */
```

前端路由配置中使用：

```ts
/**
 * Deprecated: Legacy extension architecture.
 * Do not add new static routes or navigation entries for the legacy
 * extension center. Retained only for compatibility, maintenance,
 * testing, and migration to Extension Kernel.
 */
```

弃用注释仅作为冻结标记，不改变任何组件行为、不删除任何代码、不修改路由行为。

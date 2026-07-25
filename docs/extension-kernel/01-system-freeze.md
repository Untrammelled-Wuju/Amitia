# Amitia 扩展系统重构第 1 步：现有扩展系统功能冻结总说明

> 冻结开始日期：2026-07-25
> 本步骤地位：Amitia 扩展系统重构与 Amitiax 插件平台规划的第 1 步
> 总体规划文档：[`Amitia_扩展系统重构与Amitiax插件平台步骤规划.md`](../../Amitia_扩展系统重构与Amitiax插件平台步骤规划.md)
> 实施依据：[`.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md`](../../.trae/Amitia_扩展系统重构_第1步_冻结现有扩展系统功能开发.md)

---

## 一、本步骤目标

在正式开始扩展系统重构前，先冻结现有 Skill、Agent Skill、MCP、Plugin、Workflow、`.amitiax` 旧扩展包以及扩展中心与创意工坊相关功能开发。

本步骤的目标不是停止维护，也不是立即删除旧系统，而是建立明确的重构保护边界，防止在重构期间继续向旧架构增加功能、字段、接口、数据库表和兼容逻辑，避免历史遗留问题继续扩大。

完成本步骤后，现有扩展系统进入如下状态：

> 只允许修复阻塞性缺陷、补充测试和增加迁移辅助能力，不再允许新增产品能力和扩展旧架构。

本步骤只建立重构保护边界，不重构业务实现、不修改现有行为、不提前实现新插件系统。若在执行过程中发现现有代码存在问题，只允许记录问题、标记风险、补充测试、修复阻塞性或安全性缺陷，其余问题统一进入后续步骤处理。

---

## 二、冻结对象

以下 7 大系统及其子模块全部进入功能冻结状态。

### 1. Skill 系统

包括但不限于：

- 旧版内置工具适配；
- Skill Registry；
- SkillDefinition；
- Skill 执行器；
- Skill 权限；
- Skill 作用域；
- Skill 管理接口；
- Skill 管理页面；
- Skill 相关数据库结构。

### 2. Agent Skill 系统

包括但不限于：

- `SKILL.md` 导入；
- Agent Skill 解析；
- Agent Skill 激活；
- Agent Skill 资源读取；
- Tool Mapping；
- Agent Skill 与 MCP 依赖；
- Agent Skill 管理页面；
- Agent Skill 相关数据库结构。

### 3. MCP 系统

包括但不限于：

- MCP Server 管理；
- MCP Client；
- stdio Transport；
- Streamable HTTP Transport；
- OAuth；
- Discovery；
- Tools；
- Resources；
- Prompts；
- Tasks；
- Sampling；
- Elicitation；
- Roots；
- MCP 与角色绑定；
- MCP 与 Agent Skill 依赖；
- MCP 管理页面；
- MCP 相关数据库结构。

### 4. Plugin 系统

包括但不限于：

- Go 内置 Plugin；
- Plugin Registry；
- Plugin Factory；
- PluginManager；
- Plugin Host API；
- Plugin Hook；
- Plugin 状态；
- Plugin 定时任务；
- Plugin Event；
- Plugin Surface；
- Plugin 管理页面；
- Plugin 相关数据库结构。

### 5. Workflow 系统

包括但不限于：

- Workflow 定义；
- Workflow 导入；
- Workflow 执行；
- Workflow 与 Skill 的适配；
- Workflow 权限；
- Workflow 管理页面；
- Workflow 相关数据库结构。

### 6. `.amitiax` 旧扩展包系统

包括但不限于：

- Manifest v1；
- Workflow/Instructions 二选一包结构；
- 包解析；
- 包签名；
- 包安装；
- 包更新；
- 包回滚；
- 包卸载；
- 包依赖；
- 包 Artifact；
- 扩展包管理接口；
- 扩展包管理页面。

### 7. 扩展中心与创意工坊

包括但不限于：

- 扩展中心导航；
- MCP 页面；
- Agent Skills 页面；
- 插件页面；
- 技能页面；
- 扩展包页面；
- 创意工坊；
- 执行记录；
- 与旧扩展系统绑定的前端状态管理。


---

## 三、冻结期间允许的修改

冻结不是完全禁止修改。以下 6 类修改允许进行，但必须严格限制范围。

### 1. 阻塞性缺陷修复

仅允许修复会导致以下问题的缺陷：

- 应用无法启动；
- 扩展系统启动时崩溃；
- 数据损坏；
- 数据丢失；
- 权限绕过；
- 密钥泄露；
- 路径穿越；
- 任意代码执行；
- MCP 连接导致主程序崩溃；
- 扩展执行导致主聊天链路不可用；
- 无法卸载或恢复扩展；
- 已有功能完全不可用；
- 阻塞后续重构或迁移。

> 不得借"修复缺陷"名义顺带增加新功能。

### 2. 安全修复

允许处理：

- 包解析安全；
- 归档解压安全；
- 签名校验；
- Secret 加密；
- 权限校验；
- 作用域越权；
- 插件运行隔离；
- MCP 认证安全；
- 敏感日志泄露；
- 输入参数验证；
- SQL 注入；
- 命令注入；
- 路径注入。

安全修复必须保持对现有行为的最小影响。

### 3. 回归测试补充

允许增加：

- 单元测试；
- 集成测试；
- 数据库迁移测试；
- 包解析测试；
- 权限测试；
- MCP 连接测试；
- Agent Skill 解析测试；
- Workflow 执行测试；
- Plugin 生命周期测试；
- 启动恢复测试；
- 卸载清理测试。

测试代码不得推动旧架构继续扩展。

### 4. 可观测性补充

允许增加：

- 日志；
- Trace；
- Metrics；
- 调试开关；
- 执行耗时统计；
- 启动恢复日志；
- 权限拒绝日志；
- 数据迁移诊断信息。

新增日志不得记录：

- API Key；
- OAuth Token；
- Cookie；
- 完整聊天内容；
- 用户私密数据；
- 插件 Secret；
- 数据库连接凭据。

### 5. 迁移辅助能力

允许增加只服务于后续迁移的：

- 只读查询接口；
- 数据导出；
- 状态快照；
- 旧数据扫描；
- 旧资源所有权识别；
- 依赖关系分析；
- 数据一致性检测；
- 临时迁移标记；
- 旧版本数据转换工具。

> 迁移辅助能力不得成为新的永久业务接口。

### 6. 文档与注释

允许增加：

- 架构说明；
- 历史兼容说明；
- 弃用标记；
- 迁移说明；
- 风险说明；
- 测试说明；
- 数据表说明；
- 启动顺序说明。

---

## 四、冻结期间禁止的修改

以下 8 类修改全部禁止。

### 1. 禁止新增扩展类型

不得在旧系统新增：

- 新 Skill 类型；
- 新 Plugin 类型；
- 新 MCP 包装类型；
- 新 Workflow 类型；
- 新扩展包 Entry；
- 新 Provider 类型；
- 新 UI 扩展类型；
- 新 Hook 类型。

### 2. 禁止扩展旧 Manifest

不得继续向旧 Manifest v1 增加：

- 新顶层字段；
- 新 Entry 类型；
- 新权限语法；
- 新依赖语法；
- 新 Runtime 字段；
- 新 UI 声明；
- 新 Hook 声明；
- 新 Provider 声明。

> 旧 Manifest 只允许修复解析错误和安全问题。

### 3. 禁止新增旧系统数据库表

不得为旧系统新增：

- Skill 表；
- Plugin 表；
- MCP 表；
- Workflow 表；
- Package 表；
- Hook 表；
- UI Contribution 表；
- Provider 表；
- 新的重复审计表；
- 新的重复启用状态表。

确实需要迁移辅助表时，必须满足：

- 名称明确带有 migration、snapshot 或 temporary；
- 有删除计划；
- 不承载永久业务功能；
- 在后续迁移步骤中明确清理。

### 4. 禁止新增平行 Registry

不得新增：

- 第二套 Tool Registry；
- 第二套 Skill Registry；
- 第二套 MCP Tool Registry；
- 第二套 Plugin Registry；
- 第二套 Workflow Registry；
- 第二套 UI Registry；
- 第二套权限中心；
- 第二套执行器。

### 5. 禁止新增永久兼容层

不得为了新功能增加：

- 新旧字段双写；
- 新旧状态同步；
- 新旧接口桥接；
- 新旧 Registry 双注册；
- 新旧权限双判定；
- 新旧数据双存储。

> 后续允许存在一次性迁移适配器，但必须可删除并有明确退出条件。

### 6. 禁止增加旧 Plugin 能力

不得继续扩展当前 Go 内置 Plugin 模型，包括：

- 新 Host API；
- 新 Hook；
- 新 Surface；
- 新事件；
- 新定时任务类型；
- 新动态加载方式；
- 新第三方 Plugin 入口。

> 当前 Plugin Runtime 只允许修复稳定性、安全性和迁移阻塞问题。

### 7. 禁止让 `.amitiax` v1 承担新职责

不得让旧 `.amitiax` 支持：

- 第三方运行时代码；
- JavaScript；
- WASM；
- UI 页面；
- Electron 扩展；
- Provider；
- 后台服务；
- 消息渲染器；
- 桌面组件。

这些能力必须在后续 `.amitiax` Manifest v2 中统一设计。

### 8. 禁止前端继续增加分散入口

不得新增：

- 独立 Skill 子页面；
- 独立 MCP 子系统入口；
- 独立 Plugin 子系统入口；
- 独立 Workflow 子系统入口；
- 新的扩展包入口；
- 新的重复运行记录页面。

> 旧页面只允许修复无法使用、数据错误和安全问题。

---

## 五、代码级冻结标记要求

### 1. FROZEN.md 文件位置

应在以下核心目录增加统一冻结说明文件：

```text
backend/internal/extension/FROZEN.md
backend/internal/mcp/FROZEN.md
front/src/views/extensions/FROZEN.md
```

### 2. FROZEN.md 至少包含的内容

每个 FROZEN.md 至少包含以下 7 项：

- 冻结开始原因；
- 冻结范围；
- 允许修改类型；
- 禁止修改类型；
- 新系统规划文档位置；
- 旧系统预计删除说明；
- 代码评审要求。

### 3. 统一弃用注释格式

在关键旧类型上增加弃用注释，但暂时不得删除。统一格式如下：

```go
// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.
```

### 4. 弃用注释适用对象

适用对象包括但不限于：

- 旧 `SkillDefinition`；
- 旧 Plugin Factory；
- 旧 Plugin Registry；
- MCP Skill Adapter；
- Agent Skill 到 SkillDefinition 的适配；
- Manifest v1；
- 旧 Package Parser；
- 旧 Package Installer。

前端旧页面和旧 API Client 也应增加对应弃用注释。

---

## 六、分支与提交策略

### 1. 专用重构分支

建立专用重构分支：

```text
refactor/extension-kernel
```

旧系统紧急缺陷修复应从当前稳定分支单独处理，再选择性合并到重构分支。

### 2. 允许的提交类型

冻结后，扩展相关提交只允许使用以下类型：

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

### 3. 禁止的提交类型

禁止出现：

```text
feat(skill):
feat(plugin):
feat(mcp):
feat(workflow):
feat(package):
```

除非该功能明确属于新的 Extension Kernel，且不修改旧系统职责。

### 4. 提交约束

每个涉及旧扩展系统的提交必须说明以下 8 项：

- 修改原因；
- 是否属于缺陷、安全、测试或迁移；
- 是否新增字段；
- 是否新增数据表；
- 是否改变已有行为；
- 是否增加兼容层；
- 后续是否需要删除；
- 对新 Extension Kernel 的影响。

---

## 七、代码评审约束

扩展系统冻结后，任何涉及冻结范围的 Pull Request 都必须按以下要求评审。

### 1. 必查项（13 项）

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

### 2. 直接拒绝条件（9 项）

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

---

## 八、自动检查建议

应增加静态检查或 CI 规则，尽量自动阻止旧系统继续扩展。

### 1. Manifest v1 Schema 变更检查

当以下文件发生功能性字段增加时阻止合并：

```text
backend/internal/extension/schema/manifest.schema.json
```

允许：

- 修正文档；
- 修复错误约束；
- 增加安全限制。

禁止：

- 新增 Entry 类型；
- 新增 Runtime；
- 新增 UI；
- 新增 Hook；
- 新增 Provider。

### 2. 数据库迁移检查

检测扩展相关新表和字段，要求提交中明确标记：

```text
permanent
migration-only
temporary
```

迁移辅助结构必须附带删除步骤编号。

### 3. Registry 增量检查

检测以下关键词新增：

```text
NewRegistry
RegisterSkill
RegisterPlugin
RegisterMCP
RegisterWorkflow
RegisterProvider
```

新增注册点必须证明属于新 Extension Kernel，而不是旧系统。

### 4. 前端路由检查

检测扩展中心新增静态路由和导航项，禁止继续增加分散管理入口。

---

## 九、退出条件

只有满足以下条件后，才能进入第 2 步"建立现有系统调用链地图"：

- 冻结文档已提交；
- 冻结范围已覆盖后端、前端和数据库；
- 团队或执行 Agent 已明确禁止继续扩展旧系统；
- 旧系统核心入口已有标记；
- 自动检查可以发现最明显的违规修改；
- 当前代码仍可正常构建或已如实记录构建阻塞；
- 未在冻结过程中引入任何新产品能力。

---

## 十、执行约束

执行本步骤时必须遵守：

> 只建立冻结边界和变更约束，不重构业务实现，不修改现有行为，不提前实现新插件系统。

若发现现有代码存在问题，只允许：

- 记录问题；
- 标记风险；
- 补充测试；
- 修复阻塞性或安全性缺陷。

其余问题统一进入后续步骤处理。

本步骤明确不进行以下工作：

- 不重写 Skill；
- 不重写 MCP；
- 不重写 Plugin；
- 不重写 Workflow；
- 不设计 Manifest v2 具体字段；
- 不实现 JavaScript Runtime；
- 不实现 UI Contribution；
- 不迁移数据库；
- 不删除旧代码；
- 不修改现有产品结构；
- 不开始创建 `.amitiax` v2 插件；
- 不改变当前用户可见功能。

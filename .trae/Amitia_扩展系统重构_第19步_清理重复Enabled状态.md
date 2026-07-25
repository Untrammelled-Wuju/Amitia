# Amitia 扩展系统重构第 19 步实施文档

## 第 19 步：清理重复 Enabled 状态

---

## 一、步骤目标

在第 7 步已经建立统一 Tool/Capability 模型、第 9 步已经建立统一 Permission Broker、第 10 步已经统一 Scope、第 17 步已经抽取 Plugin Runtime 安全保护能力、第 18 步已经统一启动、恢复和关闭流程的基础上，清理 Amitia 扩展系统中所有重复、重叠、互相覆盖、语义不清的 Enabled 状态。

本步骤的目标是：

> 为 Extension、Module、Contribution、Tool、Agent Skill、MCP Server、Workflow、Schedule、Runtime 和用户作用域分别建立唯一、明确、不可混用的状态真值，并通过统一状态解析器计算最终可见性和可执行性。

当前系统中可能同时存在：

- Extension Enabled；
-Package Enabled；
-Plugin Enabled；
-Module Enabled；
-Tool Enabled；
-Skill Enabled；
-Agent Skill Enabled；
-MCP Server Enabled；
-MCP Tool Enabled；
-MCP Tool Scope Enabled；
-Workflow Enabled；
-Workflow Schedule Enabled；
-Registry Enabled；
-Character Binding Enabled；
-Conversation Enabled；
-Plugin Runtime Enabled；
-Connection Enabled；
-UI Contribution Enabled；
-前端本地 Enabled；
-数据库多表 Enabled；
-缓存中的 Enabled；
-旧兼容层中的 Enabled；
-运行时推导 Enabled。

这些状态会导致：

- 同一 Tool 在一个表中启用、另一个表中禁用；
- MCP Server 已启用但 Tool 被旧 Registry 禁用；
- Extension 禁用后 Module 仍显示启用；
- Tool Enabled 被误当作权限；
- Scope Binding 被误当作启用；
- Runtime Ready 被误当作启用；
- 前端切换按钮只改了其中一处；
- 应用重启后不同 Manager 恢复出不同状态；
- 迁移时不清楚哪张表是真值；
- 禁用扩展后 Schedule、Hook、UI、Background Task 仍运行；
- Tool 在模型列表中可见但执行时被另一套 Enabled 拒绝；
- 旧系统 Enabled 和新系统 Desired State 长期双写。

本步骤完成后，必须建立以下清晰关系：

```text
Installed
≠ Enabled
≠ Bound to Scope
≠ Permission Granted
≠ Runtime Ready
≠ Healthy
≠ Visible
≠ Executable
```

---

## 二、核心原则

### 1. 每个层级只有一个 Enabled 真值

建议保留：

```text
Extension Enabled
Module Enabled
Contribution Enabled
Resource Enabled
Schedule Enabled
```

但每类对象只能有一处持久化真值。

不得同一对象同时存在：

```text
repository.enabled
registry.enabled
runtime.enabled
frontend.enabled
scope.enabled
```

---

### 2. Runtime 不保存业务 Enabled

Runtime 只保存：

```text
desired_state
actual_state
health
circuit
```

Runtime 不应再保存业务字段：

```text
enabled=true
```

业务 Enabled 由 Definition/Repository 提供。

---

### 3. Scope 不保存 Enabled

Scope Binding 只保存：

```text
active
inactive
expired
revoked
```

表示绑定本身状态，不表示 Subject 的业务启用状态。

---

### 4. Permission 不保存 Enabled

Permission Grant 只表示：

```text
allow
deny
expired
revoked
```

不能决定 Extension 或 Tool 是否启用。

---

### 5. Connection 不等于 Enabled

MCP：

```text
Server Enabled
```

表示系统期望该 Server 可参与运行。

```text
Connection Ready
```

表示当前连接成功。

二者必须分离。

---

### 6. 最终状态全部派生

最终：

```text
visible
executable
active
available
```

必须通过统一 Resolver 计算，不能单独持久化为长期真值。

---

## 三、状态分层模型

建议统一为以下层级：

```text
Installation State
Definition State
Enablement State
Scope State
Permission State
Desired Runtime State
Actual Runtime State
Health State
Circuit State
Availability State
Exposure State
Execution State
```

---

## 四、Installation State

表示资源是否存在于系统中。

建议：

```go
type InstallationState string

const (
    InstallationStateNotInstalled InstallationState = "not_installed"
    InstallationStateInstalling   InstallationState = "installing"
    InstallationStateInstalled    InstallationState = "installed"
    InstallationStateUpdating     InstallationState = "updating"
    InstallationStateRollingBack  InstallationState = "rolling_back"
    InstallationStateUninstalling InstallationState = "uninstalling"
    InstallationStateFailed       InstallationState = "failed"
)
```

Installation State 只适用于：

- Extension Package；
-独立 Agent Skill Artifact；
-独立 Workflow Package；
-受管理 MCP Package；
-Provider Package。

Tool 等 Contribution 通常不单独使用安装状态，而继承所属 Extension/Module。

---

## 五、Definition State

表示定义是否有效。

建议：

```text
valid
invalid
incompatible
migration_required
missing_dependency
corrupted
```

Definition State 由：

- Parser；
-Validator；
-Compatibility；
-Integrity；
-Dependency Resolver；

计算。

Definition Invalid 时，即使 Enabled=true，也不可运行。

---

## 六、Enablement State

建议定义：

```go
type EnablementState string

const (
    EnablementEnabled  EnablementState = "enabled"
    EnablementDisabled EnablementState = "disabled"
)
```

第一阶段只使用二态，避免引入：

```text
auto
inherit
partial
unknown
```

复杂语义。

继承通过 Resolver 计算，不作为持久化状态。

---

## 七、哪些对象需要 Enabled

建议：

### Extension

需要。

### Module

需要。

### Contribution

需要，但可默认继承 Module；只有用户可单独控制时才持久化。

### Tool

仅在产品明确支持“单 Tool 启停”时需要。

### Agent Skill

需要。

### Workflow

需要。

### MCP Server

需要。

### MCP Tool

仅在支持单 Tool 控制时需要。

### Schedule

需要。

### UI Contribution

若支持单独关闭则需要，否则继承 Module。

### Hook/Event Subscription

通常继承 Module，必要时支持单独启停。

---

## 八、不应存在 Enabled 的对象

以下对象不应保存业务 Enabled：

- Runtime Instance；
-MCP Session；
-Connection；
-Invocation；
-Attempt；
-Resource Reference；
-Permission Grant；
-Scope Snapshot；
-Audit Record；
-Cache；
-Staging；
-Recovery Journal；
-Tool Result；
-Health Snapshot；
-Circuit State。

---

## 九、Extension Enabled

Extension Enabled 是扩展级总开关。

含义：

> 用户是否允许该 Extension 的模块和 Contribution 参与 Amitia 运行。

Extension Disabled 后：

- 所有 Module 最终不可激活；
-Tool 不可执行；
-Agent Skill 不可激活；
-Workflow 不可运行；
-MCP Extension-owned Server 不自动连接；
-UI Contribution 不显示；
-Hook/Event 不投递；
-Schedule 暂停；
-Background Task 停止；
-Persistent Data 保留；
-Scope Binding 保留；
-Permission Grant 保留但不可使用；
-Runtime Desired State 变为 stopped。

Module 自身 Enabled 值保留，不自动改写为 false。

---

## 十、Module Enabled

Module Enabled 是 Extension 内子模块开关。

含义：

> 在 Extension 已启用前提下，该 Module 是否允许激活。

最终：

```text
module_effective_enabled
=
extension.enabled
AND module.enabled
```

Extension Disabled 时，不应批量修改所有 Module Enabled。

---

## 十一、Contribution Enabled

Contribution 包括：

- Tool；
-Agent Skill；
-Workflow；
-UI；
-Hook；
-Event Subscription；
-Background Task；
-Provider Action；
-Schedule。

推荐持久化规则：

### 默认继承 Module

若用户未单独修改：

```text
contribution_override = null
```

### 显式启用

```text
contribution_override = enabled
```

### 显式禁用

```text
contribution_override = disabled
```

这不是第三种 Enabled 状态，而是“是否存在用户覆盖”。

最终值：

```text
effective_enabled
=
extension.enabled
AND module.enabled
AND contribution_override != disabled
AND definition.valid
```

显式 enabled 不得绕过 Extension/Module Disabled。

---

## 十二、是否保留 Inherit

建议数据库不使用：

```text
enabled = inherit
```

而使用：

```text
override_state nullable
```

例如：

```go
type EnablementOverride *EnablementState
```

其中：

- `nil`：继承；
- enabled：显式启用；
- disabled：显式禁用。

这样避免将业务状态和覆盖语义混在一个枚举里。

---

## 十三、Tool Enabled

Tool Enabled 必须只表示：

> 用户是否允许该 Tool 作为当前 Contribution 保持启用。

Tool Enabled 不表示：

- 有权限；
-当前 Scope 匹配；
-Runtime Ready；
-MCP 已连接；
-模型可见；
-可执行。

Tool 最终可执行：

```text
extension enabled
AND module enabled
AND tool enabled
AND definition valid
AND scope allowed
AND permission allowed
AND runtime ready
AND dependency ready
AND health acceptable
AND circuit closed/half-open allowed
```

---

## 十四、Agent Skill Enabled

Agent Skill Enabled 只表示：

> 该 Agent Skill 是否可进入激活候选。

不表示：

- 当前轮已激活；
-依赖 Tool 可用；
-MCP 可用；
-Token Budget 足够；
-当前 Scope 匹配。

最终可激活：

```text
effective_enabled
AND scope allowed
AND dependencies available
AND compatibility valid
AND token plan accepted
```

---

## 十五、Workflow Enabled

Workflow Enabled 只表示：

> 是否允许启动新的 Workflow Execution。

禁用后：

- 不允许新运行；
-Schedule 暂停；
-Workflow Tool 不可执行；
-运行中实例按策略继续、暂停或取消；
-历史记录保留；
-Definition 保留。

禁用不等于删除。

---

## 十六、MCP Server Enabled

MCP Server Enabled 只表示：

> 系统是否期望该 MCP Server 可被连接和参与运行。

最终是否连接：

```text
server.enabled
AND owner enabled
AND module enabled
AND desired_state=connected
AND permission valid
AND credential valid
AND platform supported
AND not quarantined
```

Connection Ready 不回写 Enabled。

---

## 十七、MCP Tool Enabled

如果支持单 Tool 开关：

```text
mcp_tool_override
```

只控制该 Tool Contribution。

不得影响：

- Server 连接；
-其他 Tool；
-Resources；
-Prompts；
-Server Enabled；
-角色 Scope；
-Permission Grant。

---

## 十八、Schedule Enabled

Schedule Enabled 是独立开关。

最终可触发：

```text
schedule.enabled
AND owner enabled
AND subject enabled
AND scope valid
AND permission valid
AND scheduler ready
AND not expired
```

禁用 Workflow 或 Module 时：

- Schedule Effective Enabled=false；
-Schedule 自身 Enabled 值保留。

---

## 十九、Runtime Desired State

建议：

```text
running
stopped
disconnected
paused
```

由 Lifecycle Coordinator 计算。

例如 Plugin Runtime：

```text
desired_state=running
```

条件：

- Extension Enabled；
-Module Enabled；
-至少一个需要 Runtime 的 Contribution Effective Enabled；
-未 Quarantine；
-兼容；
-依赖可用。

MCP：

```text
desired_state=connected
```

不是 Enabled 的替代字段。

---

## 二十、Actual Runtime State

实际状态：

```text
created
starting
ready
degraded
stopping
stopped
crashed
failed
quarantined
```

只由 Supervisor 管理。

不得由前端切换按钮直接写 Actual State。

---

## 二十一、Health 与 Circuit

Health：

```text
healthy
degraded
unhealthy
unknown
```

Circuit：

```text
closed
open
half_open
```

这两个都是运行派生状态。

不得持久化为 Enabled。

---

## 二十二、统一 Effective State Resolver

建议：

```go
type EffectiveStateResolver interface {
    Resolve(
        ctx context.Context,
        subject StateSubject,
        runtimeContext StateRuntimeContext,
    ) EffectiveState
}
```

输出：

```go
type EffectiveState struct {
    Installed       bool
    DefinitionValid bool
    Enabled         bool
    ScopeAllowed    bool
    PermissionAllowed bool
    DesiredReady    bool
    RuntimeReady    bool
    DependencyReady bool
    Healthy         bool
    CircuitAllows   bool
    Visible         bool
    Executable      bool
    Reasons         []StateReason
}
```

---

## 二十三、State Subject

支持：

```text
extension
module
tool
agent_skill
workflow
mcp_server
mcp_tool
ui_contribution
hook
event_subscription
schedule
background_task
provider
```

---

## 二十四、Reason 模型

统一不可用原因：

```text
not_installed
installation_in_progress
definition_invalid
incompatible
extension_disabled
module_disabled
contribution_disabled
tool_disabled
agent_skill_disabled
workflow_disabled
mcp_server_disabled
mcp_tool_disabled
schedule_disabled
scope_denied
permission_denied
approval_required
runtime_not_ready
runtime_stopped
runtime_crashed
runtime_quarantined
dependency_missing
mcp_disconnected
health_unhealthy
circuit_open
platform_unsupported
migration_required
```

前端、模型暴露、执行前校验使用同一 Reason。

---

## 二十五、Resolver 固定优先级

建议按以下顺序解析：

```text
1. Installation
2. Definition
3. Extension Enablement
4. Module Enablement
5. Contribution Override
6. Scope
7. Permission
8. Dependency
9. Desired Runtime
10. Actual Runtime
11. Health
12. Circuit
13. Exposure
14. Execution
```

这样错误原因稳定，避免不同入口返回不同结果。

---

## 二十六、持久化状态真值

建议建立或统一：

```text
extensions.enablement_state
extension_modules.enablement_state
contribution_enablement_overrides
mcp_servers.enablement_state
agent_skill_definitions.enablement_state
workflow_definitions.enablement_state
schedules.enablement_state
```

若 Tool、MCP Tool 使用统一 Contribution：

```text
contribution_enablement_overrides
```

即可，不再单独建表。

---

## 二十七、禁止重复字段

必须审计并移除或废弃：

```text
plugin.enabled
plugin_runtime.enabled
registry.enabled
scope.enabled
mcp_tool.scope_enabled
mcp_tool.registry_enabled
workflow.runtime_enabled
agent_skill.runtime_enabled
package.enabled
frontend_local_enabled
```

如旧字段暂时保留：

- 标记 deprecated；
-禁止新写；
-只读迁移；
-增加调用统计；
-设定删除步骤。

---

## 二十八、状态写入服务

建议：

```go
type EnablementService interface {
    SetExtension(
        ctx context.Context,
        extensionID string,
        state EnablementState,
    ) error

    SetModule(
        ctx context.Context,
        extensionID string,
        moduleID string,
        state EnablementState,
    ) error

    SetContributionOverride(
        ctx context.Context,
        contributionID string,
        override *EnablementState,
    ) error

    SetSchedule(
        ctx context.Context,
        scheduleID string,
        state EnablementState,
    ) error
}
```

所有 Enabled 修改必须经过统一 Service。

---

## 二十九、状态修改流程

以禁用 Extension 为例：

```text
1. Validate Extension
2. Persist Enablement State
3. Increment Generation
4. Invalidate Effective State Cache
5. Recompute Desired Runtime State
6. Pause Schedule
7. Deactivate Contributions
8. Drain/Stop Runtime
9. Update Frontend State
10. Audit
```

不得由前端分别调用多个 Manager。

---

## 三十、启用流程

启用 Extension：

```text
1. Validate Installed
2. Validate Definition
3. Validate Compatibility
4. Persist Enabled
5. Increment Generation
6. Resolve Dependencies
7. Recompute Desired State
8. Start Runtime
9. Activate Contributions
10. Resume Eligible Schedules
11. Audit
```

若 Runtime 启动失败：

- Extension Enabled 可保持 true；
-Effective Executable=false；
-Reason=runtime_failed；
-前端显示启用但故障；
-不得自动把 Enabled 回写 false，除非产品明确采用事务式启用。

---

## 三十一、是否回滚 Enabled

推荐：

### 用户启用成功写入，但 Runtime 启动失败

保留：

```text
enabled=true
```

并显示：

```text
actual_state=failed
```

原因：

- 用户意图仍是启用；
-下次可自动恢复；
-避免隐藏故障。

### Definition Invalid

启用请求应拒绝，不写 Enabled。

---

## 三十二、前端开关语义

每个开关必须只修改一层：

### Extension 开关

只修改 Extension Enabled。

### Module 开关

只修改 Module Enabled。

### Tool 开关

只修改 Contribution Override。

### Agent Skill 开关

修改 Agent Skill Enablement 或 Contribution Override。

### MCP Server 开关

只修改 Server Enabled。

### Schedule 开关

只修改 Schedule Enabled。

前端不得一个开关同时修改：

- Enabled；
-Scope；
-Permission；
-Connection；
-Runtime；
-Registry。

---

## 三十三、前端状态展示

建议同时显示：

```text
用户设置：已启用
实际状态：未运行
原因：MCP 认证失效
```

或：

```text
用户设置：已启用
实际状态：不可用
原因：扩展模块已禁用
```

不能把 Effective false 直接显示为开关关闭。

---

## 三十四、Inherited 状态展示

对于 Contribution：

```text
状态来源：继承模块
```

或：

```text
状态来源：用户显式禁用
```

前端应允许：

- 启用；
-禁用；
-恢复继承。

恢复继承即删除 Override，不是写入 `inherit` 状态。

---

## 三十五、MCP 页面调整

MCP Server 页面分开显示：

```text
启用状态
期望连接状态
实际连接状态
协议状态
Tool 状态
```

示例：

```text
已启用
期望：已连接
实际：认证失败
Tool：0/6 可执行
```

---

## 三十六、Agent Skill 页面调整

显示：

```text
Enabled
Scope
依赖
当前是否可激活
当前轮是否已激活
```

不得将“当前未激活”显示为“已禁用”。

---

## 三十七、Workflow 页面调整

显示：

```text
Enabled
Tool Exposure
Schedule Enabled
Runtime Dependency
Running Executions
```

Workflow Disabled 与 Schedule Disabled 分开。

---

## 三十八、模型工具暴露

模型工具列表必须使用 EffectiveStateResolver。

暴露条件建议：

```text
visible=true
```

不是直接查询：

```text
tool.enabled=true
```

执行前再次解析：

```text
executable=true
```

防止状态变化。

---

## 三十九、缓存

可缓存 Effective State，但缓存键必须包含：

```text
subject_id
generation
character_id
conversation_id
runtime_generation
permission_generation
scope_generation
dependency_generation
```

触发失效：

- Enabled 变化；
-Extension/Module 变化；
-Scope 变化；
-Permission 变化；
-Runtime 状态变化；
-Health 变化；
-Circuit 变化；
-依赖变化；
-MCP 连接变化；
-Definition 变化；
-版本更新；
-角色/会话变化。

---

## 四十、Generation 统一

建议至少维护：

```text
enablement_generation
scope_generation
permission_generation
runtime_generation
definition_generation
dependency_generation
```

Effective State Cache 使用组合版本。

不得依赖固定 TTL 解决状态一致性。

---

## 四十一、事件通知

状态变化发布统一事件：

```text
extension.enablement.changed
module.enablement.changed
contribution.override.changed
mcp_server.enablement.changed
schedule.enablement.changed
effective_state.changed
runtime.desired_state.changed
```

事件只用于通知，不作为真值。

---

## 四十二、Enablement 与生命周期协调

EnablementService 修改后，不直接启动各 Manager，而是通知 Lifecycle Coordinator：

```text
Enablement Changed
→ Desired State Recompute
→ Lifecycle Plan
→ Start/Stop/Pause/Resume
```

这样避免状态写入与运行操作分散。

---

## 四十三、旧状态迁移策略

需要为每种旧对象建立优先级。

### Extension

选定唯一真值表。

### Plugin

旧 Plugin Enabled 映射到 Module 或 Extension。

### MCP Server

保留 Server Enabled。

### MCP Tool

旧 Tool Enabled 映射 Contribution Override。

### MCP Scope Enabled

拆分：

- Scope Binding；
-Contribution Override。

### Agent Skill

旧 Global/Character Enabled 拆分：

- Enablement；
-Scope Binding。

### Workflow

旧 Skill Enabled 映射 Workflow Enabled。

### Schedule

单独迁移。

---

## 四十四、迁移冲突规则

若多处状态冲突：

```text
old_a=true
old_b=false
```

不得随意取 OR 或 AND。

必须根据第 3、4 步审计确定旧业务优先级。

无法确认时：

- 标记 migration_conflict；
-默认采用更安全状态：disabled；
-前端提示；
-保留原值；
-写迁移报告；
-允许用户确认。

---

## 四十五、迁移不扩大原则

迁移结果不得比旧系统实际可用范围更宽。

例如：

- 旧 MCP Tool 仅角色 A 启用；
-不能迁移为 Global Enabled。

需要拆分：

```text
Tool Enabled=true
Scope Binding=character A
```

---

## 四十六、旧字段冻结

本步骤开始后：

- 旧 Enabled 字段禁止新增写入；
-旧 API 改为调用 EnablementService；
-旧 Manager 只能读取兼容视图；
-增加日志统计；
-CI 可扫描直接写旧字段代码；
-数据库 Trigger 不建议作为长期方案。

---

## 四十七、兼容视图

迁移期间可以提供：

```text
legacy_enabled_view
```

由新状态派生旧响应。

允许：

```text
旧 API
→ EffectiveStateResolver
→ 旧格式
```

禁止：

```text
新 Service
→ 同时写旧字段
```

---

## 四十八、数据库迁移建议

步骤：

```text
1. 新增统一字段/表
2. 回填
3. 生成冲突报告
4. 切换读取
5. 冻结旧写入
6. 观察
7. 删除兼容读取
8. 后续删除旧字段
```

本步骤不立即物理删除全部旧字段。

---

## 四十九、状态审计

必须记录：

- 谁修改；
-对象；
-旧值；
-新值；
-Override；
-来源；
-角色/会话；
-时间；
-触发生命周期计划；
-结果；
-失败；
-恢复；
-迁移冲突。

不得记录 Secret。

---

## 五十、批量操作

支持：

- 启用/禁用整个 Extension；
-批量恢复继承；
-批量禁用 MCP Tool；
-批量暂停 Schedule。

批量操作必须：

- 原子写入或有补偿；
-生成预览；
-限制数量；
-写 Audit；
-不绕过依赖；
-不直接并行启动无限 Runtime。

---

## 五十一、删除语义

删除 Extension/Module/Contribution 时：

- Enabled 状态随定义删除；
-历史 Audit 保留；
-用户资产按 Resource Ownership 处理；
-缓存失效；
-Desired State 停止；
-不得将删除转换为 disabled 长期保留垃圾定义。

---

## 五十二、测试要求

必须新增：

### 1. Extension Enabled

- 启用；
-禁用；
-Runtime 失败；
-Module 保留；
-Schedule 暂停；
-恢复。

### 2. Module Enabled

- 独立启停；
-Extension Disabled；
-共享 Runtime；
-其他 Module 不受影响。

### 3. Contribution Override

- nil；
-enabled；
-disabled；
-恢复继承；
-Module Disabled；
-Extension Disabled。

### 4. Tool

- Enabled 但无权限；
-Enabled 但 Scope 不匹配；
-Enabled 但 Runtime Failed；
-Disabled 但 Runtime Ready；
-模型可见性。

### 5. Agent Skill

- Enabled；
-未激活；
-依赖缺失；
-Token 不足；
-Scope；
-Extension Disabled。

### 6. Workflow

- Enabled；
-Tool Exposure；
-Schedule；
-运行中禁用；
-恢复。

### 7. MCP Server

- Enabled/Disconnected；
-Disabled/Ready 残留；
-手动断开；
-认证失败；
-重连；
-Tool Override。

### 8. Schedule

- Enabled；
-Owner Disabled；
-Subject Disabled；
-Permission Revoke；
-Scope Expired。

### 9. Resolver

覆盖所有 Reason 和优先级。

### 10. Cache

- Enabled；
-Scope；
-Permission；
-Runtime；
-Health；
-Circuit；
-Dependency；
-Definition；
-Generation。

### 11. Migration

- 一致状态；
-冲突；
-旧 Scope Enabled；
-旧 Skill Enabled；
-无效对象；
-更安全默认。

### 12. Frontend

- 开关只修改一层；
-显示用户设置与实际状态；
-恢复继承；
-错误原因。

### 13. Lifecycle

- Enabled 变化触发 Desired State；
-旧回调不覆盖新 Generation；
-重复切换；
-并发切换；
-失败恢复。

---

## 五十三、实施任务

### Task 1：完成 Enabled 字段全量盘点

列出所有表、模型、缓存、API、前端状态和 Manager。

### Task 2：定义统一状态分层

锁定 Installation、Definition、Enablement、Scope、Permission、Runtime、Health、Circuit、Availability。

### Task 3：定义 EnablementState 与 Override

禁止新增模糊状态。

### Task 4：建立 EnablementService

统一 Extension、Module、Contribution、MCP Server 和 Schedule 写入。

### Task 5：建立 EffectiveStateResolver

统一 Visible、Executable 和 Reasons。

### Task 6：建立 State Subject 模型

覆盖全部扩展对象。

### Task 7：接入 Lifecycle Coordinator

Enabled 变化转换为 Desired State Plan。

### Task 8：接入 ToolRegistry

模型暴露和执行前状态使用 Resolver。

### Task 9：接入 AgentSkillCatalog

激活候选使用 Resolver。

### Task 10：接入 WorkflowRegistry

运行和 Tool Exposure 使用 Resolver。

### Task 11：接入 MCP Service

分离 Enabled、Desired Connection、Actual Connection。

### Task 12：接入 Plugin Runtime

分离 Module Enabled、Desired Runtime、Actual Runtime、Health 和 Circuit。

### Task 13：接入 Scheduler

Schedule Effective State 使用 Resolver。

### Task 14：统一前端开关 API

每个开关只修改单层状态。

### Task 15：重构前端状态展示

显示用户设置、实际状态和 Reason。

### Task 16：实现 Override 恢复继承

删除覆盖值。

### Task 17：实现状态缓存与 Generation

避免 TTL 型不一致。

### Task 18：冻结旧 Enabled 写入

旧 API 转统一 Service。

### Task 19：建立迁移器

回填新状态并生成冲突报告。

### Task 20：建立兼容视图

只读派生旧响应。

### Task 21：增加直接写旧字段统计

识别剩余入口。

### Task 22：完成回归、并发和迁移测试

保证状态不漂移。

---

## 五十四、建议目录结构

建议：

```text
backend/internal/extension/kernel/state/
├── installation.go
├── definition.go
├── enablement.go
├── override.go
├── subject.go
├── effective.go
├── reason.go
├── resolver.go
├── service.go
├── generation.go
├── cache.go
├── migration.go
├── compatibility.go
└── audit.go
```

前端：

```text
front/src/views/extensions/state/
├── EffectiveStateBadge.vue
├── EnablementToggle.vue
├── EnablementSource.vue
├── StateReasonList.vue
├── RuntimeStateView.vue
└── MigrationConflictView.vue
```

目录仅为建议。

---

## 五十五、性能要求

建议：

- Effective State 批量解析；
-按 Extension/Module 批量查询；
-不为每个 Tool 单独查询数据库；
-Generation 驱动缓存失效；
-前端列表使用状态快照；
-Reason 按需加载；
-状态事件合并；
-批量 Enabled 操作有界；
-避免每次角色切换全量重建全部 Registry；
-仅重算受影响 Subject。

---

## 五十六、风险控制

### P0：权限与可用性扩大

- Enabled 被当授权；
-迁移 OR 合并导致全局启用；
-Scope Enabled 丢失角色限制；
-Extension Disabled 仍可执行 Tool；
-Schedule 在 Owner Disabled 时运行。

### P1：状态漂移

- 前端开关写多处；
-旧字段继续写；
-缓存未失效；
-Runtime 回写 Enabled；
-重连覆盖用户 Disabled。

### P2：迁移错误

- Plugin Enabled 映射错误；
-Agent Skill Global/Character 混合；
-MCP Tool 状态丢失；
-Workflow Skill 状态错误；
-冲突静默。

### P3：体验问题

- 开关显示与实际不一致；
-用户无法理解继承；
-启用失败后开关行为混乱；
-错误原因过多；
-批量操作慢。

---

## 五十七、本步骤不做的事情

本步骤明确不做：

- 不物理删除全部旧 Enabled 字段；
-不删除旧 Registry；
-不建立完整 Extension Kernel 生命周期；
-不实现 `.amitiax` v2；
-不实现新 Plugin Runtime；
-不实现 UI Contribution；
-不迁移全部生产数据；
-不改变 Permission Grant；
-不改变 Scope 业务语义；
-不把 Enabled 改成复杂策略语言；
-不使用前端状态作为真值。

---

## 五十八、验收产物

完成后必须提交：

### 1. 状态真值主文档

```text
docs/extension-kernel/19-enable-state-cleanup.md
```

### 2. Enabled 字段审计清单

包含：

- 数据库；
-领域模型；
-Repository；
-Manager；
-Registry；
-Cache；
-API；
-前端；
-Electron；
-兼容层。

### 3. 状态分层定义

明确：

- Installed；
-Definition Valid；
-Enabled；
-Scope；
-Permission；
-Desired Runtime；
-Actual Runtime；
-Health；
-Circuit；
-Visible；
-Executable。

### 4. EnablementService

统一所有状态写入。

### 5. EffectiveStateResolver

输出：

- 各层状态；
-Visible；
-Executable；
-Reasons。

### 6. Override 模型

支持：

- 继承；
-显式启用；
-显式禁用；
-恢复继承。

### 7. 生命周期接入

Enabled 变化转换为 Desired State。

### 8. 各系统接入报告

覆盖：

- Extension；
-Module；
-Tool；
-Agent Skill；
-Workflow；
-MCP Server；
-MCP Tool；
-Plugin Runtime；
-Schedule；
-UI；
-Hook；
-Event；
-Background Task。

### 9. 前端状态改造

开关与实际状态分离显示。

### 10. 迁移冲突报告

列出：

- 一致项；
-冲突项；
-默认禁用项；
-Scope 拆分项；
-无法映射项；
-仍写旧字段的入口。

### 11. 兼容视图

旧 API 只读派生新状态。

### 12. 测试报告

覆盖状态、Resolver、缓存、Generation、前端、生命周期、迁移、并发和安全。

---

## 五十九、验收标准

本步骤通过必须满足：

1. Installation、Definition、Enabled、Scope、Permission、Runtime、Health 和 Circuit 已分离。
2. 每类对象只有一个 Enabled 持久化真值。
3. Runtime 不再写业务 Enabled。
4. Scope 不再保存业务 Enabled。
5. Permission 不再表示 Enabled。
6. Connection Ready 不再回写 MCP Server Enabled。
7. Extension Disabled 可使全部 Contribution Effective Disabled。
8. Module Enabled 值不会因 Extension Disabled 被覆盖。
9. Contribution 支持显式 Override 和恢复继承。
10. Tool Enabled 不再表示可执行。
11. Agent Skill Enabled 不再表示当前已激活。
12. Workflow Enabled 与 Schedule Enabled 已分离。
13. MCP Server Enabled、Desired Connection 和 Actual Connection 已分离。
14. 模型 Tool 暴露使用 EffectiveStateResolver。
15. 执行前再次使用 Resolver。
16. 前端开关只修改一层状态。
17. 前端能显示用户设置、实际状态和原因。
18. 新代码不再直接写旧 Enabled 字段。
19. 迁移不扩大旧系统实际可用范围。
20. 冲突状态不会静默处理。
21. 缓存通过 Generation 正确失效。
22. 并发切换和重启恢复测试通过。
23. 后续第 20 步可以建立只读迁移接口。

---

## 六十、退出条件

只有满足以下条件后，才能进入第 20 步“建立只读迁移接口”：

- Enabled 字段全量盘点完成；
-EnablementService 已落地；
-EffectiveStateResolver 已落地；
-Extension、Module、Tool、Agent Skill、Workflow、MCP、Schedule 已接入；
-Lifecycle Coordinator 已接入；
-前端开关语义已统一；
-旧 Enabled 写入已冻结；
-兼容视图已建立；
-迁移冲突报告可生成；
-状态缓存与 Generation 已验证；
-关键回归测试通过。

---

## 六十一、执行约束

执行本步骤时必须遵守：

> Enabled 只表达用户或系统对某个定义对象的启用意图，不得再承担权限、作用域、连接、健康、运行状态或最终可执行性。

禁止出现：

- Runtime Ready 回写 Enabled=true；
-MCP 断线回写 Server Enabled=false；
-Extension Disabled 批量覆盖 Module Enabled；
-前端一个开关同时修改 Enabled、Scope 和 Permission；
-Tool Enabled 直接决定模型暴露；
-Agent Skill 未激活显示为 Disabled；
-Workflow Schedule Enabled 与 Workflow Enabled 共用字段；
-迁移时取多个旧状态的 OR；
-新旧 Enabled 长期双写；
-使用 TTL 代替状态 Generation；
-兼容层重新成为写入主入口。

本步骤完成后，Amitia 必须具备一套唯一、明确、可解释、可迁移、不会互相覆盖的扩展启用状态模型。

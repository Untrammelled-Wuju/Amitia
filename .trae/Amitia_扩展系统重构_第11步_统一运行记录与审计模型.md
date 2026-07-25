# Amitia 扩展系统重构第 11 步实施文档

## 第 11 步：统一运行记录与审计模型

---

## 一、步骤目标

在第 8 步已经建立统一执行安全内核、第 9 步已经建立统一 Permission Broker、第 10 步已经建立统一 Scope Manager 的基础上，重构当前分散在 Skill、Plugin、MCP、Workflow、Package、Agent Skill、后台任务和前端运行记录中的日志、运行状态与审计结构，建立唯一的 Runtime Record & Audit Model。

本步骤的目标是：

> 让所有扩展相关执行、生命周期、权限、作用域、资源、安装、升级、回滚、卸载、事件、Hook、后台任务和外部连接都使用同一套追踪标识、状态枚举、审计结构和查询入口。

当前系统中可能同时存在：

- Extension Run；
- Plugin Run；
- Plugin Event；
- Plugin Delivery；
- Plugin Schedule Run；
- MCP Operation；
- MCP Audit Log；
- MCP Task；
- Workflow Run；
- Workflow Node Run；
- Package Operation；
- Package Recovery；
- Agent Skill Activation；
- Skill Execution；
- Tool Side Effect；
- Permission Audit；
- Scope Audit；
-前端“执行记录”页面；
-日志文件中的独立 Trace；
-不同模块使用不同状态文本。

这些结构需要重新归一，否则会持续产生：

- 同一次调用被记录多次但无法关联；
- 同一执行在不同表中状态不一致；
- MCP 显示成功但 Tool 审计显示失败；
- Plugin Hook 超时只存在日志文件；
- Workflow 子节点无法关联父 Tool Invocation；
- Package 安装失败但已写入部分成功记录；
- 权限拒绝和执行失败混在一起；
- 无法从一个 Invocation 追踪到副作用、事件和子调用；
- 无法准确判断一个扩展为何被自动熔断；
- 前端执行记录依赖多套 API；
- 迁移后旧表仍作为事实来源；
- Secret、用户输入和内部堆栈可能被过度记录。

本步骤完成后，系统必须形成统一追踪链：

```text
Trace
→ Operation
→ Invocation
→ Attempt
→ Runtime Event
→ Side Effect
→ Audit Event
→ Final Outcome
```

---

## 二、统一模型的职责边界

统一运行记录与审计系统负责：

- Trace ID；
-Operation ID；
-Invocation ID；
-Parent/Child 调用关系；
-运行状态；
-执行尝试；
-时间线；
-权限决策记录；
-作用域决策记录；
-审批记录引用；
-Runtime 调度记录；
-重试与超时；
-取消；
-副作用；
-错误分类；
-资源使用；
-扩展生命周期操作；
-包生命周期操作；
-MCP 连接与协议操作；
-Workflow 节点运行；
-Plugin Hook、Event、Schedule；
-Agent Skill 激活；
-查询与筛选；
-保留策略；
-脱敏；
-导出；
-迁移；
-前端统一展示。

该系统不负责：

- 业务状态真值；
-Tool Enabled；
-扩展安装状态；
-MCP 连接管理；
-权限授权本身；
-Scope Binding 本身；
-具体错误恢复；
-执行调度；
-资源删除；
-用户消息存储；
-完整模型 Prompt 存储。

---

## 三、核心术语

## 1. Trace

表示一次跨模块、跨 Runtime、跨父子调用的完整业务链。

例如：

```text
用户发消息
→ 模型调用 Plugin Tool
→ Plugin Tool 调用 Workflow
→ Workflow 调用 MCP Tool
→ MCP Tool 写入记忆
```

整个链共享：

```text
trace_id
```

Trace 用于：

- 全链路诊断；
-性能分析；
-故障定位；
-安全审计；
-父子关系；
-前端时间线。

---

## 2. Operation

表示一次用户或系统层面的高阶操作。

示例：

- 安装扩展；
-升级扩展；
-回滚扩展；
-卸载扩展；
-连接 MCP；
-执行 Workflow；
-运行 Tool；
-处理 Plugin Event；
-执行定时任务；
-迁移旧数据；
-恢复 Extension Kernel。

Operation 可以包含多个 Invocation。

---

## 3. Invocation

表示一次可执行 Capability 的调用。

对应：

```text
Tool
Workflow Entry
Internal Action
Plugin Hook
MCP Operation
Background Task
```

Invocation 必须有唯一：

```text
invocation_id
```

---

## 4. Attempt

表示一次实际执行尝试。

由于重试，一个 Invocation 可以有多个 Attempt。

示例：

```text
Invocation
├── Attempt 1：MCP 连接断开
├── Attempt 2：重连后超时
└── Attempt 3：成功
```

---

## 5. Runtime Event

表示运行过程中的状态事件。

例如：

- queued；
-awaiting_approval；
-running；
-retrying；
-timeout_requested；
-cancel_requested；
-runtime_started；
-runtime_finished；
-circuit_opened；
-result_validated；
-audit_failed。

---

## 6. Audit Event

表示需要长期保存、可检索、不可静默修改的安全或管理事件。

例如：

- 权限授权；
-权限撤销；
-扩展安装；
-扩展卸载；
-高风险 Tool 执行；
-Secret 访问；
-Computer Use；
-MCP stdio 命令启动；
-角色范围越权拒绝；
-包签名失败；
-扩展自动熔断；
-迁移异常。

---

## 四、统一状态模型

建议建立唯一状态枚举：

```go
type ExecutionStatus string

const (
    StatusCreated          ExecutionStatus = "created"
    StatusQueued           ExecutionStatus = "queued"
    StatusAwaitingApproval ExecutionStatus = "awaiting_approval"
    StatusRunning          ExecutionStatus = "running"
    StatusRetrying         ExecutionStatus = "retrying"
    StatusSucceeded        ExecutionStatus = "succeeded"
    StatusFailed           ExecutionStatus = "failed"
    StatusCancelled        ExecutionStatus = "cancelled"
    StatusTimedOut         ExecutionStatus = "timed_out"
    StatusDenied           ExecutionStatus = "denied"
    StatusRateLimited      ExecutionStatus = "rate_limited"
    StatusCircuitOpen      ExecutionStatus = "circuit_open"
    StatusInvalid          ExecutionStatus = "invalid"
    StatusPartiallySucceeded ExecutionStatus = "partially_succeeded"
)
```

要求：

- 后端执行记录统一使用；
-前端统一使用；
-MCP、Plugin、Workflow 不再各自定义状态；
-状态迁移必须合法；
-终态不可被普通更新覆盖；
-状态变化必须记录 Runtime Event；
-失败和拒绝必须区分；
-超时和取消必须区分；
-部分成功必须记录副作用。

---

## 五、状态迁移规则

建议：

```text
created
→ queued
→ awaiting_approval
→ running
→ retrying
→ succeeded / failed / cancelled / timed_out / denied / rate_limited / circuit_open / invalid
```

允许直接：

```text
created → denied
created → invalid
queued → cancelled
awaiting_approval → denied
running → partially_succeeded
```

禁止：

```text
succeeded → running
failed → succeeded
cancelled → retrying
denied → running
```

若恢复任务需要继续执行，必须创建新的 Attempt 或新的 Invocation，不修改已结束记录。

---

## 六、Operation 类型

建议：

```go
type OperationType string
```

至少包括：

```text
tool.execute
workflow.execute
workflow.schedule
plugin.hook
plugin.event
plugin.schedule
mcp.connect
mcp.disconnect
mcp.discover
mcp.tool.execute
extension.install
extension.enable
extension.disable
extension.update
extension.rollback
extension.uninstall
extension.restore
agent_skill.import
agent_skill.activate
agent_skill.remove
permission.grant
permission.revoke
scope.bind
scope.unbind
migration.execute
runtime.start
runtime.stop
runtime.crash
```

---

## 七、统一 Invocation Record

建议：

```go
type InvocationRecord struct {
    InvocationID    string
    TraceID         string
    OperationID     string
    ParentID        string
    RootID          string

    CapabilityID    string
    CapabilityType  string
    Source          string
    OwnerType       string
    OwnerID         string
    ExtensionID     string
    ModuleID        string
    RuntimeType     string
    RuntimeID       string

    UserID          string
    CharacterID     string
    ConversationID  string
    ScopeSnapshotID string

    Status          ExecutionStatus
    RiskLevel       RiskLevel
    ApprovalMode    ApprovalMode

    InputHash       string
    OutputHash      string
    ErrorCode       string
    ErrorSummary    string

    RetryCount      int
    SideEffectCount int

    CreatedAt       time.Time
    QueuedAt        *time.Time
    StartedAt       *time.Time
    FinishedAt      *time.Time
    DurationMs      int64

    Metadata        map[string]any
}
```

不得存储：

- 完整 Secret；
-完整 Prompt；
-完整聊天历史；
-完整二进制；
-未经脱敏的命令环境；
-完整 OAuth Token；
-用户隐私原文。

---

## 八、Attempt Record

建议：

```go
type ExecutionAttempt struct {
    AttemptID      string
    InvocationID   string
    AttemptNumber  int
    RuntimeType    string
    RuntimeID      string
    Status         ExecutionStatus
    StartedAt      time.Time
    FinishedAt     *time.Time
    DurationMs     int64
    ErrorCode      string
    Retryable      bool
    BackoffMs      int64
    ResourceUsage  ResourceUsageSummary
    Metadata       map[string]any
}
```

用途：

- 重试分析；
-Runtime 诊断；
-MCP 重连；
-Plugin 崩溃；
-Workflow 节点失败；
-性能统计。

---

## 九、Runtime Event Record

建议：

```go
type RuntimeEventRecord struct {
    EventID       string
    TraceID       string
    InvocationID  string
    AttemptID     string
    EventType     string
    Severity      string
    Timestamp     time.Time
    Data          map[string]any
}
```

事件示例：

```text
invocation.created
invocation.queued
approval.requested
approval.granted
permission.denied
scope.denied
runtime.dispatch.started
runtime.dispatch.finished
retry.scheduled
timeout.triggered
cancel.requested
result.invalid
side_effect.recorded
circuit.opened
circuit.closed
```

---

## 十、统一审计事件

建议：

```go
type AuditEvent struct {
    AuditID        string
    TraceID        string
    OperationID    string
    InvocationID   string

    ActorType      string
    ActorID        string
    SubjectType    string
    SubjectID      string

    Action         string
    Decision       string
    RiskLevel      string
    ScopeSummary   string
    PermissionIDs  []string

    TargetType     string
    TargetID       string

    Result         string
    ErrorCode      string

    CreatedAt      time.Time
    Metadata       map[string]any
}
```

Audit Event 应具备：

- 稳定 Action；
-明确 Actor；
-明确 Subject；
-明确 Target；
-明确 Decision；
-不可存 Secret；
-不可由插件覆盖；
-不可被普通用户删除；
-支持保留策略；
-支持导出。

---

## 十一、Actor 与 Subject

### Actor

表示谁发起：

```text
user
system
model
extension
plugin
workflow
scheduler
migration
mcp_server
```

### Subject

表示谁被执行或被管理：

```text
tool
extension
module
mcp_server
workflow
agent_skill
permission_grant
scope_binding
runtime
package
```

必须避免将 Owner、Actor 和 Subject 混为同一字段。

---

## 十二、错误记录

统一 Error Record：

```go
type ErrorRecord struct {
    ErrorID        string
    InvocationID   string
    AttemptID      string
    Code           string
    Category       string
    Retryable      bool
    UserVisible    bool
    SanitizedMessage string
    InternalReference string
    CreatedAt      time.Time
}
```

InternalReference 可以指向受限日志或崩溃报告，不直接包含堆栈。

错误分类必须与第 8 步 ToolError 一致。

---

## 十三、副作用记录关联

第 8 步 SideEffectRecorder 产生的副作用必须关联：

```text
trace_id
operation_id
invocation_id
attempt_id
```

Workflow 聚合时：

- 子节点副作用归属子 Invocation；
-父 Workflow 记录汇总；
-不得重复计数；
-部分成功时必须保留已发生副作用；
-回滚时记录反向副作用。

---

## 十四、权限与 Scope 审计关联

Permission Broker 与 Scope Manager 不再独立形成无法关联的日志。

权限审计必须关联：

```text
trace_id
operation_id
invocation_id
grant_id
approval_id
```

Scope 审计必须关联：

```text
scope_snapshot_id
binding_id
invocation_id
```

从任一 Invocation 必须能够查询：

- 使用了哪些 Grant；
-Scope 如何解析；
-为何允许或拒绝；
-审批是谁做出的。

---

## 十五、Plugin 记录迁移

当前 Plugin 相关记录可能包含：

- Plugin Run；
-Hook Run；
-Event；
-Delivery；
-Schedule；
-State；
-Circuit 状态。

目标映射：

```text
Plugin Hook
→ InvocationRecord + OperationType(plugin.hook)

Plugin Event
→ OperationRecord + RuntimeEvent

Event Delivery
→ AttemptRecord

Plugin Schedule
→ OperationRecord + InvocationRecord

Circuit
→ RuntimeEvent + Runtime Health Snapshot
```

Plugin State 本身不是运行记录，不迁入 Audit Store。

---

## 十六、MCP 记录迁移

当前 MCP 记录可能包含：

- MCP Operation；
-MCP Audit Log；
-MCP Task；
-连接状态；
-Discovery；
-Tool Call；
-重连；
-OAuth。

目标映射：

```text
Connect/Disconnect
→ OperationRecord

Discovery
→ InvocationRecord / RuntimeEvent

Tool Call
→ Tool InvocationRecord

MCP Task
→ Long-running Operation + child Attempts

Reconnect
→ RuntimeEvent

OAuth Grant
→ Permission/Audit Event
```

MCP Server 当前连接状态仍属于 MCP Manager，不以审计表作为真值。

---

## 十七、Workflow 记录迁移

目标结构：

```text
Workflow Run
→ Root Invocation

Workflow Node
→ Child Invocation

Tool Node
→ Child Tool Invocation

Retry
→ Attempt

Workflow Side Effect
→ Aggregated Side Effect

Workflow Final Result
→ Root Outcome
```

必须支持从根 Workflow 查看完整节点树。

---

## 十八、Package 与 Extension 生命周期审计

所有 Extension 生命周期操作必须有 OperationRecord。

### 安装

记录：

- 包 ID；
-版本；
-来源；
-Checksum；
-签名结果；
-发布者；
-权限差异；
-依赖；
-安装结果；
-回滚引用。

### 升级

记录：

- 旧版本；
-新版本；
-权限变化；
-数据迁移；
-资源变化；
-失败补偿。

### 回滚

记录：

- 回滚来源；
-目标版本；
-触发原因；
-数据恢复；
-结果。

### 卸载

记录：

- 删除资源；
-保留资源；
-用户资产；
-Secret；
-依赖阻塞；
-结果。

---

## 十九、Agent Skill 审计

Agent Skill 不是 Tool，但以下操作需要记录：

- 导入；
-解析；
-启用；
-禁用；
-激活；
-资源读取；
-MCP 依赖安装；
-删除；
-迁移。

Agent Skill Activation 建议记录为：

```text
OperationType: agent_skill.activate
```

包含：

- Skill ID；
-角色；
-会话；
-触发原因；
-Token 使用；
-引用 Tool；
-资源读取；
-结果。

不得记录完整 SKILL.md 到普通审计表。

---

## 二十、统一 Operation Record

建议：

```go
type OperationRecord struct {
    OperationID   string
    TraceID       string
    Type          OperationType
    ActorType     string
    ActorID       string
    SubjectType   string
    SubjectID     string
    Status        ExecutionStatus
    StartedAt     time.Time
    FinishedAt    *time.Time
    Summary       string
    ErrorCode     string
    Metadata      map[string]any
}
```

Operation 可关联多个 Invocation。

---

## 二十一、时间线查询

统一查询应支持：

- 按 Trace；
-按 Operation；
-按 Invocation；
-按 Extension；
-按 Module；
-按 Tool；
-按 Character；
-按 Conversation；
-按 MCP Server；
-按 Workflow；
-按 Agent Skill；
-按时间；
-按状态；
-按风险；
-按错误码；
-按副作用；
-按 Actor；
-按 Runtime。

必须支持分页和游标。

---

## 二十二、前端统一执行记录

扩展中心只保留一个“运行与审计”入口。

建议视图：

### 1. 运行列表

展示：

- 时间；
-类型；
-对象；
-来源；
-角色；
-状态；
-耗时；
-风险；
-副作用；
-错误。

### 2. 运行详情

展示：

- Timeline；
-父子调用树；
-权限决策；
-Scope；
-Runtime；
-重试；
-错误；
-副作用；
-关联扩展；
-关联 MCP；
-关联 Workflow。

### 3. 审计列表

展示高风险与管理事件。

### 4. 诊断视图

展示 Runtime、熔断、重连、失败趋势。

前端不得再分别请求 Plugin、MCP、Workflow 多套运行日志。

---

## 二十三、数据敏感性分级

建议：

```text
public
internal
sensitive
restricted
secret
```

记录字段必须标记敏感级别。

### Public

- Tool 显示名称；
-状态；
-耗时。

### Internal

- Runtime ID；
-错误码；
-调用链。

### Sensitive

- 文件路径摘要；
-目标资源；
-角色和会话 ID。

### Restricted

- 输入摘要；
-权限范围；
-内部服务地址。

### Secret

- Token；
-Key；
-Cookie；
-完整 Header；
-密码。

Secret 不允许进入统一审计库。

---

## 二十四、输入与输出摘要

不得默认保存完整 Input/Output。

建议保存：

```text
input_hash
output_hash
input_size
output_size
input_summary
output_summary
```

Summary 必须：

- 有长度限制；
-可按 Tool 自定义安全摘要器；
-默认不包含 Secret；
-高风险 Tool 可只存目标；
-用户可配置关闭内容摘要；
-审计导出时遵守敏感级别。

---

## 二十五、保留策略

建议按类型制定：

### 普通 Invocation

保留一定周期，可由用户配置。

### 高风险 Audit

长期保留。

### 调试 Runtime Event

短期保留。

### Metrics

聚合后删除明细。

### 失败与崩溃

中长期保留。

### Package 生命周期

长期保留。

### Secret 访问

长期保留，但只记录动作，不记录 Secret。

### 用户删除

允许删除普通运行记录，但高风险审计应按系统策略保留。

---

## 二十六、不可篡改性

高风险审计应支持：

- Append-only；
-序列 Hash；
-批次校验；
-删除审计；
-导出校验；
-数据库事务；
-防普通业务代码更新。

第一阶段不必实现复杂外部账本，但必须防止普通 Update 覆盖历史记录。

---

## 二十七、统一存储建议

建议目标表：

```text
runtime_operations
runtime_invocations
runtime_attempts
runtime_events
runtime_errors
runtime_side_effects
audit_events
audit_exports
audit_retention_jobs
```

是否复用现有表需结合第 4 步分类决定。

原则：

- 运行状态与审计事件分离；
-定义状态与运行状态分离；
-当前连接状态不存成审计真值；
-事件明细与聚合指标分离；
-旧表只读迁移；
-新数据不双写旧表。

---

## 二十八、写入可靠性

### 普通运行记录

可异步批量写入，但：

- 有界队列；
-失败重试；
-进程关闭前 Flush；
-队列满有降级策略；
-不得阻塞低风险 Tool 太久。

### 高风险审计

必须在操作完成前确认写入，或进入持久化本地队列。

不得静默丢失。

### 审计失败

根据风险：

- 低风险：执行可成功但标记 audit_degraded；
-高风险：可阻止最终成功状态；
-破坏性操作：审计写入必须可靠。

---

## 二十九、日志与审计分离

日志用于开发和诊断，可包含：

- 内部堆栈；
-Runtime stderr；
-协议帧摘要；
-调试字段。

审计用于安全和用户追踪，必须：

- 结构化；
-脱敏；
-稳定；
-可解释；
-长期兼容。

不得把日志文件当审计数据库。

---

## 三十、Trace 传播

Trace Context 必须跨：

- ToolExecutor；
-Runtime Adapter；
-Plugin Runtime；
-MCP Client；
-Workflow；
-后台任务；
-Schedule；
-事件；
-Electron Bridge；
-前端请求；
-迁移任务。

跨进程传播至少包含：

```text
trace_id
operation_id
invocation_id
parent_id
```

不得传播 Secret 或完整 Context。

---

## 三十一、崩溃与恢复

应用或 Runtime 崩溃后：

- running Invocation 标记为 interrupted；
-若状态枚举不包含 interrupted，可映射为 failed + error_code=runtime_interrupted；
-记录最后 Runtime Event；
-后台可恢复任务创建新 Attempt；
-不可恢复任务终止；
-副作用未知时标记 outcome_unknown；
-用户可查看；
-不得静默改为成功或失败。

建议增加：

```text
outcome_unknown
```

作为审计结果，而非执行主状态。

---

## 三十二、迁移旧运行记录

迁移步骤需处理：

- Extension Runs；
-Plugin Runs；
-Plugin Events；
-Plugin Deliveries；
-Plugin Schedules；
-MCP Operations；
-MCP Audit Logs；
-MCP Tasks；
-Workflow Runs；
-Package Operations；
-Agent Skill Activations；
-Side Effects；
-Permission Audits；
-Scope Audits。

迁移要求：

- 保留原始 ID 作为 legacy_reference；
-生成新的 Trace/Operation/Invocation；
-无法建立父子关系时明确标记；
-无法映射状态时进入 legacy_unknown；
-不伪造不存在的成功链；
-旧表迁移后只读；
-新写入只进入统一模型。

---

## 三十三、兼容 API

迁移期间旧页面可通过兼容 API 查询统一存储。

允许：

```text
旧 Plugin Run API
→ 统一运行查询
→ 转换旧响应
```

禁止：

```text
统一运行查询
→ 同时查新旧表并长期合并
```

兼容 API 必须有删除计划。

---

## 三十四、测试要求

必须新增：

### 1. ID 关联测试

验证 Trace、Operation、Invocation、Attempt 父子关系。

### 2. 状态迁移测试

覆盖合法与非法迁移。

### 3. 多来源测试

覆盖 Tool、MCP、Plugin、Workflow、Package、Agent Skill。

### 4. 重试测试

验证多 Attempt 归一 Invocation。

### 5. 权限与 Scope 关联测试

能从 Invocation 查到 Grant 和 Snapshot。

### 6. 副作用关联测试

父子调用不重复计数。

### 7. 脱敏测试

验证 Secret 不进入记录。

### 8. 崩溃恢复测试

验证 running 状态处理。

### 9. 高风险审计可靠性测试

模拟数据库失败和队列满。

### 10. 查询测试

覆盖分页、筛选、时间线和树。

### 11. 迁移测试

旧表到新模型映射。

### 12. 前端测试

统一运行页面不依赖旧 API。

---

## 三十五、实施任务

### Task 1：定义统一记录领域模型

完成 Trace、Operation、Invocation、Attempt、Runtime Event、Audit Event、Error Record。

### Task 2：统一状态枚举

替换 Plugin、MCP、Workflow 等独立状态映射。

### Task 3：建立统一存储

实现运行记录与审计存储。

### Task 4：接入 Execution Security Kernel

调用开始、Attempt、结束、副作用统一写入。

### Task 5：接入 Permission Broker

关联 Grant、Decision 和 Approval。

### Task 6：接入 Scope Manager

关联 Scope Snapshot 和 Binding。

### Task 7：接入 Plugin

迁移 Hook、Event、Delivery、Schedule 记录。

### Task 8：接入 MCP

迁移连接、Discovery、Tool、Task 和重连记录。

### Task 9：接入 Workflow

建立根 Invocation 与节点子 Invocation。

### Task 10：接入 Package 生命周期

统一安装、升级、回滚、卸载和恢复操作。

### Task 11：接入 Agent Skill

统一导入、激活、资源读取和删除记录。

### Task 12：建立 Trace 传播

覆盖进程、Runtime、MCP 和前端。

### Task 13：实现脱敏与摘要

统一处理 Input、Output、Error 和 Metadata。

### Task 14：实现保留策略

建立清理任务和审计保留规则。

### Task 15：建立统一查询 API

支持列表、详情、时间线和调用树。

### Task 16：重建前端运行记录页面

改为统一数据源。

### Task 17：建立旧数据迁移器

只读转换旧运行记录。

### Task 18：建立迁移统计

记录仍写旧表的入口。

### Task 19：完成可靠性和性能测试

验证高并发与写入失败。

---

## 三十六、建议目录结构

建议：

```text
backend/internal/extension/kernel/observability/
├── trace.go
├── operation.go
├── invocation.go
├── attempt.go
├── runtime_event.go
├── audit_event.go
├── error.go
├── status.go
├── transition.go
├── storage.go
├── writer.go
├── query.go
├── retention.go
├── sanitizer.go
├── migration.go
└── export.go
```

前端：

```text
front/src/views/extensions/runtime/
├── RuntimeListView.vue
├── RuntimeDetailView.vue
├── RuntimeTimeline.vue
├── InvocationTree.vue
├── AuditListView.vue
└── RuntimeFilters.vue
```

目录仅为建议。

---

## 三十七、性能要求

统一记录系统不得明显拖慢 Tool 执行。

建议：

- 普通运行记录支持批量写；
-高风险审计可靠写；
-查询使用索引；
-Trace Tree 查询避免 N+1；
-事件明细分页；
-大 Metadata 限制；
-保留任务增量清理；
-前端时间线按需加载；
-日志与审计分开存储；
-高基数字段不进入 Metrics 标签。

具体指标应基于现有基线测量。

---

## 三十八、风险控制

### P0：审计缺失或泄密

- 高风险操作无审计；
-Secret 写入；
-审计可被普通更新；
-执行成功但记录失败且无标记。

### P1：事实不一致

- 同一 Invocation 多个最终状态；
-MCP 与 Tool 记录冲突；
-Workflow 父子状态不一致；
-副作用丢失；
-重试被记录为多次独立成功。

### P2：迁移丢失

- 旧审计无法关联；
-历史版本操作丢失；
-Plugin Event 丢失；
-MCP Task 状态丢失。

### P3：性能问题

- 每个 Runtime Event 同步写数据库；
-Trace 查询过慢；
-前端加载全部明细；
-保留任务锁表。

---

## 三十九、本步骤不做的事情

本步骤明确不做：

- 不实现完整外部日志平台；
-不实现云端审计同步；
-不实现多用户审计；
-不实现复杂合规认证；
-不删除旧运行表；
-不迁移全部历史数据到生产环境；
-不实现插件 Runtime；
-不实现 `.amitiax` v2；
-不实现 UI Contribution；
-不重构业务日志；
-不存储完整模型 Prompt；
-不记录 Secret。

---

## 四十、验收产物

完成后必须提交：

### 1. 统一运行与审计主文档

```text
docs/extension-kernel/11-unified-runtime-audit-model.md
```

### 2. 核心领域类型

至少包含：

- Trace；
-Operation；
-Invocation；
-Attempt；
-Runtime Event；
-Audit Event；
-Error Record；
-统一状态。

### 3. 统一存储

包含运行记录、事件、副作用和审计表。

### 4. Trace 传播

覆盖 Tool、Plugin、MCP、Workflow、Package 和 Agent Skill。

### 5. 统一查询 API

支持：

- 列表；
-详情；
-时间线；
-父子调用树；
-审计；
-筛选。

### 6. 脱敏与摘要规则

列出所有敏感字段和处理方式。

### 7. 旧数据迁移映射

覆盖全部旧运行与审计表。

### 8. 前端统一运行页面

不再依赖多套运行记录 API。

### 9. 迁移统计报告

列出：

- 已切换写入；
-仍写旧表；
-旧数据数量；
-无法映射状态；
-孤儿记录；
-重复记录。

### 10. 测试报告

覆盖：

- 状态；
-ID；
-父子关系；
-重试；
-副作用；
-权限；
-Scope；
-脱敏；
-崩溃；
-迁移；
-查询；
-性能。

---

## 四十一、验收标准

本步骤通过必须满足：

1. Trace、Operation、Invocation 和 Attempt 已统一定义。
2. Tool、Plugin、MCP、Workflow、Package 和 Agent Skill 使用同一追踪标识。
3. 执行状态使用同一枚举。
4. 所有状态迁移经过校验。
5. 重试记录为同一 Invocation 的多个 Attempt。
6. 权限与 Scope 可关联到 Invocation。
7. 副作用可关联到父子调用。
8. 高风险操作产生 Audit Event。
9. Secret 不进入统一记录。
10. 前端运行记录使用统一查询入口。
11. 新数据不再写入旧运行表。
12. 旧表只读迁移。
13. 崩溃后运行状态有明确处理。
14. 审计失败不会静默。
15. 保留策略已实现。
16. 迁移统计可识别剩余旧入口。
17. 关键测试通过。
18. 后续第 12 步可以建立统一资源所有权模型。

---

## 四十二、退出条件

只有满足以下条件后，才能进入第 12 步“建立统一资源所有权模型”：

- 统一记录模型已落地；
-Execution Security Kernel 已接入；
-Permission Broker 已接入；
-Scope Manager 已接入；
-Plugin、MCP、Workflow、Package、Agent Skill 已接入；
-前端统一查询可用；
-新写入已停止进入旧运行表；
-旧数据迁移映射已完成；
-高风险审计可靠；
-脱敏测试通过；
-性能无不可接受退化。

---

## 四十三、执行约束

执行本步骤时必须遵守：

> 运行记录用于描述“发生了什么”，审计用于回答“谁在什么范围内对什么做了什么以及结果如何”，二者都不能替代业务状态真值。

禁止出现：

- 以审计记录恢复 MCP 当前连接状态；
-以运行记录判断扩展是否安装；
-插件自行写审计表；
-前端自行拼接不同来源日志；
-同一执行创建多个无关联成功记录；
-将重试当成多次独立调用；
-将 Secret 写入 Metadata；
-普通业务代码更新历史审计；
-新旧运行表长期双写；
-日志文件替代统一审计存储。

本步骤完成后，Amitia 必须具备一套统一、可追踪、可解释、可脱敏、可查询、可迁移的运行与审计基础。

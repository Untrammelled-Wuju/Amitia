# Amitia 扩展系统重构第 16 步实施文档

## 第 16 步：重构 Workflow 执行器

---

## 一、步骤目标

在第 6 步已经解除 Skill 概念过载、第 7 步已经建立统一 Tool/Capability 模型、第 8 步已经建立统一执行安全内核、第 10 步已经统一 Scope、第 11 步已经统一运行与审计、第 12 步已经建立统一资源所有权、第 15 步已经重构 Agent Skill Loader 的基础上，正式重构 Amitia Workflow Executor。

本步骤的目标是：

> 将 Workflow 从旧 Skill Runtime、独立执行状态、独立权限逻辑、独立作用域逻辑和分散定时任务中彻底分离，建立独立的 Workflow Definition、Validator、Compiler、Executor、Node Runtime、Tool Adapter、Schedule Binding、Compensation 和 Recovery 模型。

完成本步骤后，Workflow 必须被明确定位为：

```text
声明式执行图 + 输入输出契约 + 节点依赖 + 执行策略 + 补偿策略
```

而不是：

```text
Skill
Tool 本体
Plugin Runtime
任意脚本容器
```

Workflow 可以通过适配器暴露为 Tool，但：

```text
WorkflowDefinition
≠
ToolDefinition
```

目标链路：

```text
WorkflowDefinition
→ WorkflowRegistry
→ WorkflowCompiler
→ WorkflowExecutionPlan
→ WorkflowExecutor
→ NodeExecutor
→ ExecutionSecurityKernel
→ Runtime Adapter
→ WorkflowResult
→ Audit
```

当 Workflow 需要被 Agent 调用时：

```text
WorkflowDefinition
→ WorkflowToolAdapter
→ ToolDefinition
→ ToolRegistry
```

---

## 二、当前需要解决的问题

当前 Workflow 实现可能存在以下结构问题：

1. Workflow 被包装为 Skill 或通过 Skill Runtime 执行。
2. Workflow Definition、运行实例、节点状态和 Tool 暴露混在同一结构。
3. Workflow 节点直接调用旧 Skill Handler。
4. Workflow 自行实现权限、超时、重试和审计。
5. Workflow 子调用缺少统一 Invocation 父子链。
6. Workflow Scope 与角色、会话上下文分散。
7. Schedule 运行时读取“当前角色”或当前页面状态。
8. 节点重试可能导致重复副作用。
9. 并行节点缺乏统一并发控制。
10. Workflow 失败后没有明确补偿语义。
11. Workflow 更新时运行中的实例使用哪个版本不明确。
12. Workflow 删除时 Schedule、运行中实例、缓存和引用可能残留。
13. 用户编辑包内 Workflow 时所有权不明确。
14. Workflow 输出结构不统一，模型调用时难以稳定映射。
15. 运行记录、节点记录和 Tool 调用记录重复。
16. 条件、循环和表达式可能使用任意脚本，存在安全风险。
17. 节点输入映射缺乏 Schema 校验。
18. 节点输出可能被未声明地传递给后续节点。
19. Workflow 依赖的 Tool 被删除或改版后缺乏预检。
20. 应用崩溃后运行实例无法明确恢复、失败或重放。

---

## 三、职责边界

Workflow 系统负责：

- Workflow Definition；
-版本；
-输入输出 Schema；
-节点图；
-依赖图；
-编译；
-静态校验；
-循环检测；
-节点顺序；
-条件；
-并行；
-分支；
-聚合；
-等待；
-人工确认节点；
-Tool 节点引用；
-子 Workflow；
-运行实例；
-节点状态；
-变量上下文；
-结果汇总；
-补偿计划；
-Schedule Definition；
-暂停、恢复、取消；
-崩溃恢复；
-变更检测；
-运行版本固定；
-统一审计接入。

Workflow 系统不负责：

- 直接执行 Tool Handler；
-绕过 Execution Security Kernel；
-授予权限；
-自行解析最终 Scope；
-管理 MCP 连接；
-管理 Plugin Runtime；
-执行任意宿主脚本；
-管理 `.amitiax` 安装事务；
-将 Workflow 本体注册为 Skill；
-直接拼接模型 Prompt；
-管理 Extension 生命周期。

---

## 四、目标组件

建议拆分为：

```text
WorkflowSystem
├── WorkflowDefinitionParser
├── WorkflowDefinitionValidator
├── WorkflowRegistry
├── WorkflowCompiler
├── WorkflowExecutionPlanner
├── WorkflowExecutor
├── WorkflowNodeExecutor
├── WorkflowContextStore
├── WorkflowExpressionEngine
├── WorkflowToolAdapter
├── WorkflowScheduleService
├── WorkflowCompensationService
├── WorkflowRecoveryService
├── WorkflowChangeDetector
├── WorkflowMigrationAdapter
└── WorkflowAuditWriter
```

---

## 五、Workflow 领域定义

建议定义：

```go
type WorkflowDefinition struct {
    ID             string
    ExtensionID    string
    ModuleID       string
    Name           LocalizedText
    Description    LocalizedText
    Version        string
    SchemaVersion  int

    InputSchema    json.RawMessage
    OutputSchema   json.RawMessage

    Nodes          []WorkflowNodeDefinition
    Edges          []WorkflowEdgeDefinition
    EntryNodeID    string
    ExitNodeIDs    []string

    ExecutionPolicy WorkflowExecutionPolicy
    Compensation    WorkflowCompensationPolicy
    SchedulePolicy  *WorkflowSchedulePolicy
    ScopeRule       ScopeRule
    Compatibility   WorkflowCompatibility
    Integrity       WorkflowIntegrity
    Metadata        map[string]any
}
```

要求：

- 不包含运行实例；
-不包含当前角色；
-不包含当前会话；
-不包含 Handler；
-不包含 Tool Executor；
-不包含 Runtime Client；
-不包含 Permission Grant；
-不包含当前节点状态；
-不包含 Schedule 的下一次运行时间。

---

## 六、稳定 ID

Workflow ID 建议：

```text
workflow/<owner-namespace>/<workflow-name>
```

示例：

```text
workflow/com.example.daily-summary/run
workflow/user/morning-routine
workflow/system/message-delivery
```

要求：

- 全局稳定；
-不依赖显示名称；
-不依赖数据库自增 ID；
-版本升级不变化；
-用户 Fork 生成新 ID；
-Extension 与 User Workflow 不得共享同一 ID；
-旧 ID 有迁移映射；
-Workflow Tool Adapter 的 Tool ID 稳定派生。

---

## 七、Workflow 版本

Definition 必须有：

```text
workflow_id
version
definition_hash
```

运行实例启动时固定版本：

```text
execution_definition_version
execution_definition_hash
```

运行过程中更新 Workflow：

- 不影响已运行实例；
-新实例使用新版本；
-旧版本在仍有运行实例或回滚需要时保留；
-删除旧版本必须检查引用。

禁止运行时每个节点重新读取最新 Definition。

---

## 八、节点类型

第一阶段建议支持：

```go
type WorkflowNodeType string

const (
    WorkflowNodeStart          WorkflowNodeType = "start"
    WorkflowNodeEnd            WorkflowNodeType = "end"
    WorkflowNodeTool           WorkflowNodeType = "tool"
    WorkflowNodeCondition      WorkflowNodeType = "condition"
    WorkflowNodeTransform      WorkflowNodeType = "transform"
    WorkflowNodeParallel       WorkflowNodeType = "parallel"
    WorkflowNodeJoin           WorkflowNodeType = "join"
    WorkflowNodeDelay          WorkflowNodeType = "delay"
    WorkflowNodeApproval       WorkflowNodeType = "approval"
    WorkflowNodeSubWorkflow    WorkflowNodeType = "sub_workflow"
    WorkflowNodeSetVariable    WorkflowNodeType = "set_variable"
    WorkflowNodeEmitEvent      WorkflowNodeType = "emit_event"
)
```

第一阶段不支持任意脚本节点。

---

## 九、节点定义

建议：

```go
type WorkflowNodeDefinition struct {
    ID              string
    Type            WorkflowNodeType
    Name            string
    InputMapping    map[string]WorkflowExpression
    OutputMapping   map[string]WorkflowExpression
    Config          json.RawMessage
    RetryPolicy     *RetryPolicy
    Timeout         *time.Duration
    CompensationNodeID string
    ContinueOnError bool
    Metadata        map[string]any
}
```

要求：

- Node ID 在 Workflow 内唯一；
-Node Type 必须受支持；
-Config 使用节点类型专用 Schema；
-输入映射必须可静态校验；
-输出映射必须可静态校验；
-Tool 节点必须使用稳定 Tool ID；
-Sub Workflow 使用稳定 Workflow ID；
-不得保存 Handler 名称替代 Tool ID；
-不得保存旧 Skill ID。

---

## 十、边定义

建议：

```go
type WorkflowEdgeDefinition struct {
    FromNodeID string
    ToNodeID   string
    Condition  *WorkflowExpression
    Priority   int
}
```

要求：

- 节点必须存在；
-禁止重复边；
-条件必须使用安全表达式；
-默认边规则明确；
-多个命中分支的行为必须明确；
-不允许隐式跳转；
-必须检测不可达节点；
-必须检测无出口路径；
-必须检测非法循环。

---

## 十一、图结构校验

Validator 必须检查：

- Entry 存在；
-Exit 存在；
-Node ID 唯一；
-Edge 合法；
-不可达节点；
-死路；
-未连接节点；
-非法循环；
-并行与 Join 配对；
-Approval 后续路径；
-Delay 最大值；
-Sub Workflow 循环依赖；
-Tool 依赖；
-Output 可生成；
-补偿节点合法；
-版本兼容；
-Schema 合法。

---

## 十二、循环策略

第一阶段建议：

- 默认禁止任意图循环；
-如需循环，使用显式受控循环节点；
-设置最大迭代次数；
-设置总超时；
-循环变量结构化；
-每次迭代记录；
-支持取消；
-不得通过条件边构造隐式无限循环。

若当前版本没有受控循环节点，则本步骤直接禁止循环。

---

## 十三、表达式引擎

需要建立受限 Workflow Expression Engine。

允许读取：

```text
workflow.input
workflow.variables
nodes.<node_id>.output
nodes.<node_id>.status
invocation.character_id
invocation.conversation_id
system.current_time
```

允许操作：

```text
字段访问
数组索引
布尔比较
数值比较
字符串比较
空值判断
逻辑 AND/OR/NOT
受控默认值
简单对象构造
简单数组构造
```

禁止：

- 任意代码；
-函数反射；
-文件访问；
-网络访问；
-环境变量；
-Secret；
-动态 Tool 调用；
-SQL；
-Shell；
-无限递归；
-正则灾难性回溯；
-宿主对象访问。

---

## 十四、表达式确定性

表达式必须：

- 相同输入产生相同结果；
-无外部副作用；
-有执行步数上限；
-有嵌套深度上限；
-有输出大小限制；
-错误结构化；
-可在编译期验证引用路径；
-运行时缺失字段行为明确；
-不静默吞错。

---

## 十五、Workflow Registry

建议：

```go
type WorkflowRegistry interface {
    Register(ctx context.Context, definition WorkflowDefinition) error
    Replace(ctx context.Context, definition WorkflowDefinition) error
    Unregister(ctx context.Context, workflowID string) error
    Get(ctx context.Context, workflowID string, version string) (WorkflowDefinition, error)
    GetLatest(ctx context.Context, workflowID string) (WorkflowDefinition, error)
    List(ctx context.Context, filter WorkflowFilter) ([]WorkflowDefinition, error)
}
```

Registry 负责：

- Definition；
-版本；
-Owner；
-Enabled 查询；
-Scope 查询；
-兼容性；
-依赖索引；
-引用检查。

Registry 不负责：

- 执行；
-Schedule 触发；
-权限；
-MCP 连接；
-Tool Handler；
-运行状态。

---

## 十六、Workflow Compiler

Compiler 输入 Definition，输出不可变 Execution Plan。

建议：

```go
type WorkflowExecutionPlan struct {
    WorkflowID      string
    Version         string
    DefinitionHash  string
    TopologicalOrder []string
    Nodes           map[string]CompiledWorkflowNode
    Edges           map[string][]CompiledWorkflowEdge
    DependencySet   WorkflowDependencySet
    CompensationPlan WorkflowCompensationPlan
    PlanHash        string
}
```

Compiler 负责：

- Schema 编译；
-图校验；
-表达式编译；
-Tool 引用解析；
-Sub Workflow 引用解析；
-并行计划；
-补偿计划；
-依赖快照；
-确定性 Plan Hash。

---

## 十七、编译与执行分离

必须遵守：

```text
Definition 变化
→ 重新编译
```

执行时：

- 不重新解析 YAML/JSON；
-不重新扫描 Tool ID；
-不重新构建图；
-只加载已验证 Plan；
-Plan 与 Definition Hash 绑定；
-Plan Cache 可重建。

---

## 十八、Workflow 输入

Workflow 启动前必须：

- 解析 Workflow；
-确定版本；
-校验 Input Schema；
-应用默认值；
-生成 Scope Snapshot；
-进行 Permission 预检；
-进行 Tool 依赖预检；
-进行 Runtime 可用性预检；
-创建 Root Invocation；
-创建 Context。

非法输入不得创建运行中实例。

---

## 十九、Workflow Context

建议：

```go
type WorkflowExecutionContext struct {
    ExecutionID      string
    TraceID          string
    RootInvocationID string
    WorkflowID       string
    WorkflowVersion  string
    DefinitionHash   string

    Input            json.RawMessage
    Variables        map[string]json.RawMessage
    NodeStates       map[string]WorkflowNodeState
    ScopeSnapshotID  string

    UserID           string
    CharacterID      string
    ConversationID   string

    StartedAt        time.Time
    Deadline         time.Time
    Metadata         map[string]any
}
```

不得通过全局变量读取当前角色或会话。

---

## 二十、Workflow Execution 状态

建议：

```text
created
validated
queued
awaiting_approval
running
paused
waiting
compensating
succeeded
partially_succeeded
failed
cancelled
timed_out
recovery_required
```

这些是 Workflow 领域状态，必须映射到统一 `ExecutionStatus`。

统一审计仍以 Root Invocation 为主。

---

## 二十一、节点状态

建议：

```text
pending
ready
queued
running
waiting
succeeded
failed
skipped
cancelled
timed_out
compensated
compensation_failed
```

每个 Node Run 必须关联：

```text
workflow_execution_id
node_id
invocation_id
attempt_id
```

Tool 节点的实际 Tool Invocation 作为子 Invocation。

---

## 二十二、Workflow Executor

建议：

```go
type WorkflowExecutor interface {
    Start(
        ctx context.Context,
        request WorkflowStartRequest,
    ) (WorkflowExecution, error)

    Resume(
        ctx context.Context,
        executionID string,
    ) error

    Pause(
        ctx context.Context,
        executionID string,
    ) error

    Cancel(
        ctx context.Context,
        executionID string,
        reason string,
    ) error

    Get(
        ctx context.Context,
        executionID string,
    ) (WorkflowExecution, error)
}
```

---

## 二十三、固定执行流程

```text
1. Resolve Definition Version
2. Load/Compile Plan
3. Validate Input
4. Resolve Scope
5. Preflight Dependencies
6. Create Root Operation/Invocation
7. Persist Execution Context
8. Mark Running
9. Schedule Ready Nodes
10. Execute Nodes
11. Persist Node Results
12. Resolve Next Nodes
13. Aggregate Output
14. Validate Output Schema
15. Run Compensation if Required
16. Finalize Root Invocation
17. Cleanup Temporary State
18. Audit
```

---

## 二十四、Node Executor

建议：

```go
type WorkflowNodeExecutor interface {
    Supports(nodeType WorkflowNodeType) bool

    Execute(
        ctx context.Context,
        node CompiledWorkflowNode,
        workflowCtx WorkflowExecutionContext,
    ) WorkflowNodeResult
}
```

首批实现：

```text
StartNodeExecutor
EndNodeExecutor
ToolNodeExecutor
ConditionNodeExecutor
TransformNodeExecutor
ParallelNodeExecutor
JoinNodeExecutor
DelayNodeExecutor
ApprovalNodeExecutor
SubWorkflowNodeExecutor
SetVariableNodeExecutor
EmitEventNodeExecutor
```

---

## 二十五、Tool 节点

ToolNodeExecutor 不直接执行 Runtime。

正确链路：

```text
Tool Node
→ Build ToolExecutionRequest
→ ExecutionSecurityKernel
→ ToolExecutor
→ Runtime Adapter
```

必须继承或收窄：

- Trace；
-Parent Invocation；
-Scope；
-Deadline；
-Approval Mode；
-Permission Context；
-Idempotency；
-风险策略。

---

## 二十六、Tool 节点输入

Tool 节点输入由：

```text
InputMapping
```

生成。

执行前必须：

- 计算表达式；
-限制输出大小；
-校验 Tool Input Schema；
-生成 Input Hash；
-记录映射错误；
-禁止未声明字段；
-不得把完整 Workflow Context 全量传入 Tool。

---

## 二十七、Tool 节点结果

ToolResult 必须转换为结构化 Node Output。

需要明确：

- 使用 Structured；
-使用 Text；
-使用 Resource Reference；
-使用 Error；
-使用 Side Effects；
-输出 Schema；
-敏感字段；
-结果大小。

禁止后续节点默认访问完整内部 ToolResult。

只暴露 Workflow Definition 声明允许的输出字段。

---

## 二十八、条件节点

Condition Node：

- 无副作用；
-使用受限表达式；
-必须有明确默认分支；
-表达式错误应失败或走显式错误分支；
-不得调用模型决定条件，除非单独使用 Tool 节点；
-不得读取 Secret；
-必须记录命中分支。

---

## 二十九、Transform 节点

Transform Node 用于：

- 字段映射；
-对象构造；
-数组变换；
-简单格式转换。

不得：

- 执行任意脚本；
-访问外部系统；
-调用 Tool；
-修改全局状态；
-写文件；
-执行 SQL。

复杂转换应通过正式 Tool 实现。

---

## 三十、Parallel 与 Join

Parallel 节点必须：

- 明确分支；
-限制最大并发；
-继承父 Scope；
-继承剩余 Deadline；
-可取消；
-失败策略明确；
-结果顺序稳定；
-不共享可变变量；
-避免数据竞争。

Join 策略可包括：

```text
all_success
all_finished
any_success
first_finished
quorum
```

第一阶段建议只实现：

```text
all_success
all_finished
```

---

## 三十一、并发安全

Workflow 内并发必须接入统一 ConcurrencyController。

同时考虑：

- 全局 Workflow 并发；
-每 Workflow；
-每 Execution；
-每节点类型；
-每 Tool；
-每扩展；
-每角色；
-每会话。

不得单独再实现不可见信号量体系。

---

## 三十二、Delay 节点

Delay 节点不得长期占用执行线程。

需要持久化：

- 唤醒时间；
-Execution ID；
-Node ID；
-Scope Snapshot；
-Definition Version；
-剩余 Deadline；
-取消状态。

应用重启后：

- 可恢复；
-重新校验 Extension、Workflow、Scope、Permission；
-不读取当前前端上下文；
-过期后按策略失败或立即继续。

---

## 三十三、Approval 节点

Approval Node 用于 Workflow 业务流程中的人工确认，不替代 Tool 权限审批。

必须区分：

### Permission Approval

由 Permission Broker 处理。

### Workflow Approval

由 Workflow 节点处理业务决策。

Workflow Approval 必须记录：

- 标题；
-说明；
-可选项；
-超时；
-默认行为；
-用户决策；
-Trace；
-Execution；
-Node；
-Scope；
-审计。

---

## 三十四、Sub Workflow

Sub Workflow 调用：

- 使用稳定 Workflow ID；
-固定子 Workflow 版本策略；
-创建子 Invocation；
-子 Scope 不得扩大；
-子 Deadline 不得超过父 Deadline；
-输入输出 Schema 校验；
-检测递归；
-检测依赖环；
-限制最大嵌套深度；
-父取消传播；
-子副作用进入调用树。

---

## 三十五、Emit Event 节点

Emit Event 必须通过统一 Event Bus。

不得直接调用 Plugin Handler。

必须：

- 声明 Event Type；
-Schema 校验；
-Owner；
-Scope；
-Permission；
-审计；
-投递策略；
-幂等；
-最大递归深度；
-避免 Event → Workflow → Event 无限循环。

详细 Event Bus 在后续第 28 步实现，本步骤只建立适配边界。

---

## 三十六、Workflow 输出

成功前必须：

- 汇总 Exit Node；
-生成 Workflow Output；
-校验 Output Schema；
-限制大小；
-脱敏；
-记录 Hash；
-记录副作用摘要；
-返回结构化结果。

建议：

```go
type WorkflowResult struct {
    ExecutionID   string
    Status        string
    Output        json.RawMessage
    Error         *ToolError
    SideEffects   []RecordedSideEffect
    Metadata      map[string]any
}
```

---

## 三十七、Workflow Tool Adapter

只有声明为可调用的 Workflow 才生成 ToolDefinition。

建议字段：

```go
type WorkflowToolExposure struct {
    Enabled        bool
    ToolName       string
    Description    string
    InputSchema    json.RawMessage
    OutputSchema   json.RawMessage
    RiskLevel      RiskLevel
    SideEffectLevel SideEffectLevel
    ApprovalPolicy ApprovalMode
}
```

Adapter 负责：

```text
WorkflowDefinition
→ ToolDefinition
```

RuntimeBinding：

```text
runtime_type=workflow
runtime_id=<workflow-id>@<version-policy>
handler_name=run
```

---

## 三十八、Workflow Tool ID

建议：

```text
workflow/<owner-namespace>/<workflow-name>/run
```

必须稳定，不因当前版本变化。

版本由 Runtime Binding 或执行请求解析。

---

## 三十九、Workflow Tool 执行

正确链路：

```text
模型调用 Workflow Tool
→ ToolExecutor
→ WorkflowRuntimeAdapter
→ WorkflowExecutor.Start
```

禁止：

```text
模型调用 Workflow Skill
→ Skill Handler
→ Workflow Executor
```

---

## 四十、重试策略

Workflow 重试分两层：

### 节点重试

针对单节点。

### Workflow 重试

通常不自动重跑整个 Workflow。

Tool 节点重试必须遵守 ToolExecutionPolicy。

Workflow 不得覆盖 Tool 的幂等和重试限制。

---

## 四十一、幂等

每个 Workflow Execution 必须支持：

```text
execution_idempotency_key
```

Tool 节点可派生：

```text
workflow_execution_id + node_id + attempt
```

高风险节点必须：

- 明确幂等；
-或禁止自动重试；
-记录部分副作用；
-补偿前判断执行结果是否未知。

---

## 四十二、补偿模型

Workflow Compensation 不是数据库事务回滚，而是业务补偿。

建议：

```go
type WorkflowCompensationPolicy struct {
    Mode                 string
    ReverseSuccessfulNodes bool
    StopOnFailure        bool
    MaxDuration          time.Duration
}
```

模式：

```text
none
explicit
reverse_order
manual
```

---

## 四十三、补偿节点

Tool 节点可声明：

```text
compensation_node_id
```

要求：

- 补偿节点必须存在；
-不能是 Start/End；
-输入来源明确；
-可访问原节点输出摘要；
-不能访问未授权上下文；
-经过 Execution Security Kernel；
-写入独立 Invocation；
-记录补偿副作用；
-补偿失败不能覆盖原始失败。

---

## 四十四、部分成功

当部分节点已产生副作用、后续失败且补偿不完整时：

```text
Workflow Status = partially_succeeded
```

必须返回：

- 已成功节点；
-已失败节点；
-已补偿节点；
-补偿失败；
-剩余副作用；
-人工处理建议；
-审计引用。

不得简单标记为 failed 而隐藏已发生副作用。

---

## 四十五、取消

取消必须传播：

```text
Workflow Root
→ Ready Queue
→ Running Nodes
→ Tool Invocation
→ Sub Workflow
→ Delay
→ Approval
```

取消后：

- 不再调度新节点；
-运行中节点收到 Context Cancel；
-是否补偿由 Policy 决定；
-等待节点关闭；
-状态持久化；
-审计；
-释放并发资源。

---

## 四十六、超时

支持：

- Workflow 总超时；
-节点超时；
-Tool 超时；
-Approval 超时；
-Delay 截止时间；
-Sub Workflow 剩余时间；
-补偿总超时。

子节点 Deadline 不得超过父 Workflow Deadline。

---

## 四十七、暂停与恢复

第一阶段只允许在以下节点安全暂停：

- Delay；
-Approval；
-节点边界；
-未调度状态。

不得在任意 Tool 执行中强制冻结进程状态。

Resume 时必须重新校验：

- Workflow Enabled；
-Definition Version 存在；
-Extension/Module Enabled；
-Scope；
-Permission；
-依赖；
-Deadline；
-Owner；
-资源。

---

## 四十八、崩溃恢复

启动时扫描：

```text
running
waiting
paused
compensating
```

处理策略：

### 可安全恢复

- Delay；
-Approval；
-尚未执行节点；
-幂等且明确失败的 Tool 节点。

### 不确定结果

Tool 调用中崩溃且副作用未知：

```text
node outcome = unknown
workflow = recovery_required
```

不得自动重试破坏性节点。

---

## 四十九、运行持久化

建议目标表：

```text
workflow_definitions
workflow_versions
workflow_execution_plans
workflow_executions
workflow_node_runs
workflow_context_snapshots
workflow_schedules
workflow_compensation_runs
workflow_migrations
```

统一运行与审计模型仍保存：

```text
Operation
Invocation
Attempt
Runtime Event
Audit Event
Side Effect
```

Workflow 专用表只保存领域运行状态，不重复保存完整审计。

---

## 五十、Schedule

Workflow Schedule 必须是独立资源。

建议：

```go
type WorkflowSchedule struct {
    ScheduleID     string
    WorkflowID     string
    VersionPolicy  string
    Owner          ResourceOwner
    ScopeSnapshot  ScopeSnapshot
    InputTemplate  json.RawMessage
    Recurrence     string
    Timezone       string
    Enabled        bool
    NextRunAt      time.Time
    PermissionSnapshotRef string
}
```

---

## 五十一、Schedule 规则

Schedule 创建时必须：

- 明确 Owner；
-明确 Workflow；
-明确版本策略；
-校验输入；
-保存 Scope；
-保存 Permission Reference；
-记录用户创建来源；
-记录时区；
-设置失败策略；
-设置重叠策略。

禁止运行时读取“当前角色”。

---

## 五十二、重叠策略

支持：

```text
skip
queue
parallel
replace
```

第一阶段建议默认：

```text
skip
```

高风险 Workflow 不允许默认 parallel。

---

## 五十三、Schedule 失效

以下变化必须暂停或失效：

- Workflow 删除；
-Extension 禁用；
-Module 禁用；
-Scope 失效；
-Permission 撤销；
-角色删除；
-会话删除；
-Owner 变化；
-兼容性失败；
-依赖缺失。

---

## 五十四、资源所有权

需要登记：

### Workflow Definition

Owner 为：

- system；
-user；
-extension；
-module。

### Execution Plan

派生 Cache，可重建。

### Schedule

Owner 明确。

### Execution Context

Temporary 或运行资源。

### User Fork

Owner 转为 user。

### Package Workflow

Owner 为 extension/module。

---

## 五十五、用户编辑包内 Workflow

必须支持：

```text
follow
fork
detach
```

### Follow

继续接收包更新，不允许直接修改核心 Definition。

### Fork

生成用户 Workflow，新 ID，新 Owner。

### Detach

保留当前定义，解除 Package 更新关系。

不得静默覆盖用户修改。

---

## 五十六、Workflow 更新

更新前必须比较：

- 节点；
-边；
-Tool 依赖；
-Sub Workflow；
-Input Schema；
-Output Schema；
-补偿；
-Schedule 兼容；
-作用域；
-风险；
-副作用；
-版本；
-表达式。

输出变更报告：

```go
type WorkflowChangeReport struct {
    BreakingChanges    []WorkflowChange
    DependencyChanges  []WorkflowChange
    SchemaChanges      []WorkflowChange
    ExecutionChanges   []WorkflowChange
    CompensationChanges []WorkflowChange
    ScheduleChanges    []WorkflowChange
    Warnings           []WorkflowChange
}
```

---

## 五十七、运行中版本保护

升级时：

- 旧版本有运行实例则保留；
-新运行使用新版本；
-Schedule 根据版本策略更新；
-固定版本 Schedule 不自动切换；
-最新版本策略需要兼容检查；
-旧版本删除前检查 Execution、Audit 和 Rollback 引用。

---

## 五十八、删除 Workflow

删除前检查：

- 运行中实例；
-暂停实例；
-Schedule；
-Agent Skill 引用；
-Plugin 引用；
-其他 Workflow 引用；
-Tool Adapter；
-用户 Fork；
-Extension Owner；
-历史版本；
-回滚引用。

处理：

- 阻止新运行；
-注销 Tool Adapter；
-禁用 Schedule；
-取消或等待运行实例；
-移除 Scope Binding；
-清理派生 Plan；
-处理外部引用；
-保留审计；
-按 Resource Release Plan 删除。

---

## 五十九、Workflow 与 Agent Skill

Agent Skill 只能引用：

- Workflow Tool ID；
-或 Workflow ID 作为资源依赖。

Agent Skill 不直接控制 Workflow 内部节点。

激活 Agent Skill 不等于自动执行 Workflow。

---

## 六十、Workflow 与 Plugin

Plugin 可以贡献 Workflow Definition，但必须：

- 经过 Parser/Validator；
-稳定 ID；
-Owner；
-Manifest 声明；
-Scope；
-Resource Ownership；
-卸载清理；
-不得动态注入任意节点代码。

---

## 六十一、Workflow 与 MCP

Workflow Tool 节点可调用 MCP Tool，但：

```text
Workflow
→ Tool ID
→ ExecutionSecurityKernel
→ MCPRuntimeAdapter
```

Workflow 不直接持有 MCP Client。

---

## 六十二、Workflow 与 Event

Event 可触发 Workflow，但必须：

- 通过统一 Event Bus；
-有 Trigger Definition；
-有 Scope；
-有 Permission；
-有幂等；
-有递归保护；
-有频率限制；
-有 Owner；
-有审计。

本步骤只建立接口，不完整实现 Event Bus。

---

## 六十三、定义格式

Workflow Definition 可使用 JSON 或 YAML，但内部必须统一转换为结构化模型。

要求：

- 安全 YAML；
-无自定义对象；
-无脚本；
-字段 Schema；
-版本；
-未知字段策略；
-大小限制；
-Hash；
-来源；
-签名；
-Package Security。

---

## 六十四、API 边界

建议：

```text
GET    /api/extensions/workflows
GET    /api/extensions/workflows/:id
POST   /api/extensions/workflows/validate
POST   /api/extensions/workflows/:id/run
POST   /api/extensions/workflows/:id/cancel
POST   /api/extensions/workflows/:id/pause
POST   /api/extensions/workflows/:id/resume
GET    /api/extensions/workflows/executions/:executionId
GET    /api/extensions/workflows/executions/:executionId/nodes
GET    /api/extensions/workflows/:id/versions
GET    /api/extensions/workflows/:id/changes
GET    /api/extensions/workflows/:id/dependencies
GET    /api/extensions/workflows/:id/schedules
POST   /api/extensions/workflows/:id/schedules
DELETE /api/extensions/workflows/:id
```

权限、Scope、Audit、资源使用统一服务。

---

## 六十五、前端页面

至少包括：

### Workflow 列表

展示：

- 名称；
-Owner；
-版本；
-状态；
-Scope；
-Tool 暴露；
-Schedule；
-依赖；
-最近运行。

### Workflow 详情

展示：

- Definition；
-节点图；
-Input/Output；
-依赖；
-补偿；
-Schedule；
-版本；
-变更；
-资源；
-权限；
-Scope。

### 运行详情

展示：

- 节点图状态；
-调用树；
-当前节点；
-等待原因；
-审批；
-重试；
-副作用；
-补偿；
-错误；
-输出。

---

## 六十六、前端约束

前端不得：

- 自行执行节点；
-根据图结构判断下一节点；
-自行重试；
-自行拼装 Tool Input；
-自行判断 Permission；
-自行保存当前角色为隐式 Schedule Scope；
-直接调用 MCP；
-直接调用 Plugin Handler。

---

## 六十七、开发者诊断

开发者控制台应支持：

- Definition；
-Compiled Plan；
-Plan Hash；
-图校验；
-不可达节点；
-依赖；
-表达式；
-当前 Context；
-节点输入输出摘要；
-Invocation ID；
-补偿；
-恢复；
-迁移警告；
-旧 Skill 调用统计。

---

## 六十八、旧系统迁移

需要迁移：

- Workflow Definition；
-Workflow Run；
-Workflow Node Run；
-Workflow Skill Wrapper；
-Schedule；
-角色 Scope；
-Tool 引用；
-旧 Handler；
-旧状态；
-旧输入输出；
-旧审计；
-旧缓存。

迁移规则：

- Definition 进入 WorkflowRegistry；
-运行历史映射统一 Invocation；
-节点运行映射子 Invocation；
-Workflow Skill Wrapper 不迁入 Agent Skill；
-可调用 Workflow 生成 Tool Adapter；
-Schedule 转独立资源；
-Scope 转统一 Binding；
-旧缓存不迁移；
-旧 Handler 仅保留兼容入口。

---

## 六十九、兼容层约束

迁移期间允许：

```text
旧 ExecuteWorkflowSkill
→ WorkflowToolAdapter
→ ToolExecutionRequest
```

允许：

```text
旧 Workflow API
→ WorkflowExecutor
```

禁止：

```text
新 WorkflowExecutor
→ 旧 Skill Runtime
```

新 Workflow 不得写入旧 Skill Registry。

---

## 七十、测试要求

必须新增：

### 1. Definition Parser

- JSON；
-YAML；
-未知字段；
-错误类型；
-版本；
-超长；
-非法表达式；
-危险 YAML。

### 2. Graph Validator

- 合法 DAG；
-循环；
-不可达；
-死路；
-多入口；
-无出口；
-Parallel/Join；
-Sub Workflow 环。

### 3. Compiler

- Plan；
-Hash；
-稳定顺序；
-依赖；
-表达式；
-版本；
-缓存。

### 4. Input/Output Schema

- 默认值；
-未知字段；
-类型错误；
-输出不匹配；
-大输出；
-敏感字段。

### 5. Tool Node

- 正常；
-Permission；
-Scope；
-超时；
-取消；
-重试；
-幂等；
-副作用；
-错误；
-结果映射。

### 6. Condition/Transform

- 条件；
-空值；
-错误；
-输出限制；
-禁止外部访问。

### 7. Parallel/Join

- 全成功；
-部分失败；
-取消；
-超时；
-并发限制；
-结果顺序；
-数据竞争。

### 8. Delay

- 唤醒；
-重启恢复；
-过期；
-取消；
-Scope 失效；
-Definition 缺失。

### 9. Approval

- 同意；
-拒绝；
-超时；
-取消；
-业务审批与权限审批分离。

### 10. Sub Workflow

- 正常；
-递归；
-深度；
-Scope；
-Deadline；
-取消；
-副作用。

### 11. Compensation

- 成功；
-失败；
-反向顺序；
-部分成功；
-原结果未知；
-超时；
-取消。

### 12. Schedule

- 时区；
-重叠；
-角色；
-权限撤销；
-Workflow 更新；
-Extension 禁用；
-应用重启。

### 13. Recovery

- 每个节点边界崩溃；
-Tool 结果未知；
-Delay；
-Approval；
-Compensation；
-Context 损坏。

### 14. Tool Adapter

- 稳定 Tool ID；
-Schema；
-Risk；
-SideEffect；
-RuntimeBinding；
-版本策略。

### 15. Migration

- 旧 Definition；
-旧 Skill Wrapper；
-旧 Run；
-旧 Schedule；
-旧 Scope；
-损坏记录。

### 16. 性能

- 大图；
-高并发节点；
-大量 Execution；
-Plan Cache；
-查询；
-取消延迟。

---

## 七十一、实施任务

### Task 1：定义 Workflow 领域模型

完成 Definition、Node、Edge、Policy、Compatibility、Integrity。

### Task 2：实现安全 Parser

统一 JSON/YAML 到领域模型。

### Task 3：实现 Definition Validator

校验 Schema、图、依赖、表达式和版本。

### Task 4：实现 WorkflowRegistry

管理 Definition、版本、Owner 和查询。

### Task 5：实现 Expression Engine

只支持受控、确定性表达式。

### Task 6：实现 WorkflowCompiler

生成不可变 Execution Plan。

### Task 7：实现 WorkflowExecutor

建立运行、暂停、恢复、取消和状态管理。

### Task 8：实现 Node Executor 框架

按 Node Type 分派。

### Task 9：实现 ToolNodeExecutor

接入 Execution Security Kernel。

### Task 10：实现 Condition/Transform

无副作用执行。

### Task 11：实现 Parallel/Join

接入统一并发控制。

### Task 12：实现 Delay/Approval

支持持久等待与恢复。

### Task 13：实现 SubWorkflow

建立父子 Invocation 和递归保护。

### Task 14：实现 WorkflowContextStore

持久化 Context、Node State 和恢复信息。

### Task 15：实现 CompensationService

处理业务补偿。

### Task 16：实现 RecoveryService

处理崩溃和不确定结果。

### Task 17：实现 WorkflowToolAdapter

将可调用 Workflow 映射为 ToolDefinition。

### Task 18：实现 WorkflowRuntimeAdapter

Tool 调用启动 Workflow Execution。

### Task 19：实现 WorkflowScheduleService

独立管理 Schedule 资源。

### Task 20：接入 Scope Manager

运行和 Schedule 使用统一 Scope。

### Task 21：接入 Permission Broker

预检和 Tool 节点授权。

### Task 22：接入 Resource Ownership

登记 Definition、Schedule、Context 和 Plan。

### Task 23：接入统一运行审计

Root、Node、Tool、Attempt 和副作用统一关联。

### Task 24：实现 Change Detector

输出版本差异。

### Task 25：迁移旧 Workflow 数据

停止新写旧系统。

### Task 26：重构前端 Workflow 页面

与 Agent Skill、Tool 分离。

### Task 27：增加旧 Skill Wrapper 调用统计

识别剩余旧链路。

### Task 28：完成故障注入与回归测试

验证恢复、补偿、取消和并发。

---

## 七十二、建议目录结构

建议：

```text
backend/internal/extension/kernel/workflow/
├── definition.go
├── node.go
├── edge.go
├── parser.go
├── validator.go
├── registry.go
├── expression.go
├── compiler.go
├── plan.go
├── execution.go
├── executor.go
├── node_executor.go
├── context_store.go
├── compensation.go
├── recovery.go
├── schedule.go
├── change_detector.go
├── migration.go
└── audit.go
```

Node Executors：

```text
backend/internal/extension/kernel/workflow/nodes/
├── start.go
├── end.go
├── tool.go
├── condition.go
├── transform.go
├── parallel.go
├── join.go
├── delay.go
├── approval.go
├── sub_workflow.go
├── set_variable.go
└── emit_event.go
```

Adapters：

```text
backend/internal/extension/kernel/adapters/
├── workflow_tool_adapter.go
└── workflow_runtime_adapter.go
```

前端：

```text
front/src/views/extensions/workflows/
├── WorkflowListView.vue
├── WorkflowDetailView.vue
├── WorkflowGraphView.vue
├── WorkflowExecutionView.vue
├── WorkflowScheduleView.vue
├── WorkflowVersionView.vue
└── WorkflowChangeReport.vue
```

目录仅为建议。

---

## 七十三、性能要求

建议：

- Definition 与 Plan 分离；
-Plan 可缓存；
-节点调度使用有界队列；
-并行节点受限；
-Context 增量持久化；
-大输出使用 Resource Reference；
-节点查询分页；
-运行详情按需加载；
-表达式有步数限制；
-恢复扫描增量执行；
-Schedule 使用索引；
-Tool 节点不重复解析 Tool Definition；
-调用树避免 N+1；
-大量 Workflow 不全量加载图。

---

## 七十四、风险控制

### P0：副作用与越权

- Workflow 直接调用 Handler；
-子节点扩大 Scope；
-高风险 Tool 自动重试；
-补偿误执行；
-取消后继续产生副作用；
-Schedule 使用错误角色。

### P1：状态不一致

- Root 成功但节点失败；
-节点完成未持久化；
-崩溃后重复执行；
-运行中版本漂移；
-补偿状态覆盖原始错误。

### P2：旧领域回退

- Workflow 继续注册 Skill；
-新 Executor 调用旧 Skill Runtime；
-独立权限和 Scope 保留；
-旧 Schedule 继续运行；
-前端继续混用技能概念。

### P3：性能与复杂度

- 大图调度过慢；
-Context 无限增长；
-并行节点过多；
-恢复扫描过慢；
-表达式过复杂；
-运行详情过载。

---

## 七十五、本步骤不做的事情

本步骤明确不做：

- 不支持任意脚本节点；
-不实现完整 Event Bus；
-不实现完整 Hook Pipeline；
-不实现第三方 Plugin Runtime；
-不实现 `.amitiax` v2 完整 Manifest；
-不删除旧 Workflow 表；
-不删除旧 Skill Wrapper；
-不实现移动端；
-不实现可视化 Workflow 编辑器全部能力；
-不实现云端分布式 Workflow；
-不保证任意 Tool 可自动补偿；
-不把业务审批与权限审批合并。

---

## 七十六、验收产物

完成后必须提交：

### 1. Workflow Executor 主文档

```text
docs/extension-kernel/16-workflow-executor.md
```

### 2. Workflow 领域类型

至少包含：

- WorkflowDefinition；
-WorkflowNodeDefinition；
-WorkflowEdgeDefinition；
-WorkflowExecutionPlan；
-WorkflowExecutionContext；
-WorkflowResult；
-WorkflowSchedule；
-WorkflowCompensationPolicy。

### 3. Parser、Validator 与 Compiler

支持安全解析、图校验、依赖校验和不可变 Plan。

### 4. WorkflowRegistry

支持 Definition、版本和查询。

### 5. WorkflowExecutor

支持：

- Start；
-Pause；
-Resume；
-Cancel；
-Recovery。

### 6. Node Executors

至少实现：

- Start；
-End；
-Tool；
-Condition；
-Transform；
-Parallel；
-Join；
-Delay；
-Approval；
-Sub Workflow；
-Set Variable；
-Emit Event 适配边界。

### 7. WorkflowToolAdapter

可调用 Workflow 映射为 ToolDefinition。

### 8. WorkflowRuntimeAdapter

Tool 调用经过统一执行安全内核并启动 Workflow。

### 9. CompensationService

支持显式补偿和部分成功。

### 10. ScheduleService

独立管理 Schedule、Scope、Permission 和重叠策略。

### 11. 统一 Scope、Permission、Ownership、Audit 接入

不得保留新独立体系。

### 12. 旧数据迁移报告

列出：

- 已迁移 Definition；
-旧 Skill Wrapper；
-旧运行实例；
-旧节点记录；
-旧 Schedule；
-旧 Scope；
-仍调用旧 Handler 的入口；
-无法恢复的损坏数据。

### 13. 前端 Workflow 页面

与 Tool、Agent Skill 页面分离。

### 14. 测试报告

覆盖 Parser、Graph、Compiler、Tool、并行、等待、补偿、Schedule、Recovery、迁移、安全和性能。

---

## 七十七、验收标准

本步骤通过必须满足：

1. Workflow 已有独立领域模型。
2. WorkflowDefinition 不包含 Handler。
3. Workflow 本体不再新增到 Skill Registry。
4. Workflow 可通过 Tool Adapter 暴露为 Tool。
5. Tool 节点全部经过 Execution Security Kernel。
6. Workflow Scope 使用统一 Scope Manager。
7. Workflow Permission 使用统一 Permission Broker。
8. Root、Node 和 Tool Invocation 具有完整父子链。
9. 运行实例固定 Definition Version。
10. 表达式引擎不支持任意代码。
11. 并行节点使用统一并发控制。
12. Delay 与 Approval 支持持久等待。
13. Schedule 不读取当前前端角色。
14. 取消和超时可传播至子节点。
15. 重试遵守 Tool 幂等和风险策略。
16. 补偿有独立记录，不能覆盖原始失败。
17. 部分成功可被明确表示。
18. 崩溃恢复不会盲目重放高风险 Tool。
19. Workflow、Schedule 和 Context 已纳入资源所有权。
20. 新运行记录进入统一审计。
21. 新数据不再写旧 Workflow/Skill 执行链。
22. 关键测试通过。
23. 后续第 17 步可以抽取 Plugin Runtime 安全保护能力。

---

## 七十八、退出条件

只有满足以下条件后，才能进入第 17 步“抽取 Plugin Runtime 安全保护能力”：

- WorkflowDefinition 已落地；
-WorkflowRegistry 已落地；
-Parser、Validator、Compiler 已落地；
-WorkflowExecutor 已落地；
-ToolNodeExecutor 已接入统一安全内核；
-WorkflowToolAdapter 与 RuntimeAdapter 已落地；
-ScheduleService 已落地；
-Compensation 与 Recovery 已落地；
-Scope、Permission、Ownership、Audit 已接入；
-新 Workflow 不再进入旧 Skill Registry；
-旧 Skill Wrapper 只剩迁移用途；
-关键故障注入测试通过。

---

## 七十九、执行约束

执行本步骤时必须遵守：

> Workflow 负责组织执行顺序和数据流，不拥有 Tool 执行权限，不直接调用 Runtime，不通过任意脚本绕过宿主安全边界。

禁止出现：

- Workflow Node 保存 Go Handler；
-Workflow 直接调用 MCP Client；
-Workflow 直接调用 Plugin 方法；
-Workflow 自行授予 Permission；
-Workflow 子节点扩大 Scope；
-Workflow 更新影响运行中实例；
-Schedule 运行时读取当前角色；
-失败后自动重跑整个高风险 Workflow；
-补偿失败覆盖原始错误；
-新 Workflow 同时注册到新 Registry 和旧 Skill Registry；
-旧 Executor 长期作为回退主链；
-前端自行决定节点执行顺序。

本步骤完成后，Amitia 必须具备一套独立、可编译、可恢复、可补偿、可调度、可审计并安全接入统一 Tool 模型的 Workflow 执行基础。

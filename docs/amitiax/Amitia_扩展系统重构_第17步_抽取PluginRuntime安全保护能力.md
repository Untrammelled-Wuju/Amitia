# Amitia 扩展系统重构第 17 步实施文档

## 第 17 步：抽取 Plugin Runtime 安全保护能力

---

## 一、步骤目标

在第 7 步已经建立统一 Tool/Capability 模型、第 8 步已经建立统一执行安全内核、第 11 步已经统一运行与审计、第 12 步已经统一资源所有权、第 16 步已经重构 Workflow Executor 的基础上，对 Amitia 当前 PluginManager、内置 Go Plugin、Hook、Event、Schedule、State、Circuit Breaker 和运行状态管理中的安全能力进行抽取。

本步骤的目标是：

> 将现有 Plugin 系统中已经具备价值的超时、Panic 隔离、熔断、并发控制、状态 CAS、事件递归保护、运行健康、资源释放和故障恢复能力，抽取为独立的 Plugin Runtime Safety Infrastructure。

本步骤不直接实现未来完整第三方 JavaScript Plugin Runtime，而是先完成以下基础准备：

```text
现有内置 Go Plugin
→ 统一 Plugin Runtime Contract
→ Safety Guard
→ Host Boundary
→ Runtime State
→ Circuit Breaker
→ Health
→ Resource Cleanup
→ Audit
```

未来 JavaScript、Service、WASM 等 Runtime 都必须复用该安全基础，而不是各自重新实现一套保护逻辑。

---

## 二、当前需要解决的问题

当前 Plugin 系统可能存在以下结构问题：

1. Plugin 仅支持 Go 编译期内置注册，不是真正第三方 Runtime。
2. PluginManager 同时管理注册、状态、Hook、Event、Schedule、Store、熔断和执行。
3. Plugin Tool、Hook、Event、Schedule 可能使用不同执行入口。
4. Panic Recovery 只覆盖部分调用路径。
5. Timeout 规则分散，部分 Hook 或 Event 无统一 Deadline。
6. Circuit Breaker 只存在 PluginManager 内部，无法被统一 Availability 使用。
7. Plugin State CAS、Namespace、持久化与 Runtime 实例绑定不清晰。
8. Event Delivery 重试可能产生重复副作用。
9. Hook 链深度和 Event 递归深度可能分开限制。
10. Plugin 禁用后 Timer、Schedule、Event Subscription、Worker、Process 可能残留。
11. Plugin Runtime 健康状态与 Extension Enabled、Module Enabled、Tool Enabled 混在一起。
12. Plugin 调用 Host API 时可能绕过统一 Permission Broker。
13. Plugin 内部调用 Tool 时可能扩大 Scope。
14. Plugin 运行记录仍使用独立 Run、Event、Delivery、Schedule 表。
15. Plugin State 写入与执行事务没有稳定版本控制。
16. Plugin Crash 后恢复策略不明确。
17. Plugin 熔断后 ToolRegistry、UI、Hook 是否立即不可用缺少统一规则。
18. PluginManager 可能继续直接持有 Tool Handler 闭包。
19. 未来若直接新增 JavaScript Runtime，会复制现有安全问题。

---

## 三、核心原则

### 1. Runtime Safety 与 Runtime 实现分离

安全保护层负责：

- 调用边界；
-超时；
-取消；
-Panic/Crash 隔离；
-并发限制；
-速率限制；
-递归保护；
-熔断；
-健康；
-状态访问；
-资源注册；
-清理；
-审计；
-指标。

Runtime 实现负责：

- 加载代码；
-执行入口；
-返回结果；
-处理 Runtime 内部消息。

---

### 2. Plugin Runtime 不拥有 Tool 执行主链

Plugin Tool 正确执行链：

```text
ToolExecutionRequest
→ ExecutionSecurityKernel
→ PluginRuntimeAdapter
→ PluginRuntimeSafetyGuard
→ Plugin Runtime
→ ToolResult
```

禁止：

```text
PluginManager.ExecuteTool
→ Plugin Handler
```

绕过统一执行安全内核。

---

### 3. Hook、Event、Schedule 也必须进入统一安全边界

即使不是模型 Tool，也必须经过：

- Scope；
-Permission；
-Timeout；
-Cancellation；
-Concurrency；
-Depth Guard；
-Circuit Breaker；
-Audit；
-Resource Ownership。

---

### 4. Plugin Enabled 不等于 Runtime Healthy

必须区分：

```text
Extension Enabled
Module Enabled
Contribution Enabled
Runtime State
Health State
Circuit State
Executable State
```

---

### 5. 现有 Go Plugin 作为 Legacy Runtime

当前内置 Go Plugin 保留，但被重新定义为：

```text
LegacyGoPluginRuntime
```

只作为过渡 Runtime。

不得继续围绕 Go Plugin 增加新第三方能力。

---

## 四、目标架构

建议形成：

```text
Plugin Runtime Layer
├── PluginRuntime interface
├── LegacyGoPluginRuntime
├── FutureJavaScriptRuntime
├── FutureServiceRuntime
└── FutureWASMRuntime

Plugin Runtime Safety Layer
├── PluginRuntimeSupervisor
├── PluginRuntimeSafetyGuard
├── PluginConcurrencyController
├── PluginCircuitBreaker
├── PluginHealthMonitor
├── PluginDepthGuard
├── PluginStateBroker
├── PluginResourceTracker
├── PluginCleanupCoordinator
├── PluginCrashRecovery
└── PluginRuntimeAudit
```

---

## 五、Plugin Runtime Contract

建议定义：

```go
type PluginRuntime interface {
    RuntimeType() PluginRuntimeType

    Start(
        ctx context.Context,
        spec PluginRuntimeSpec,
    ) error

    Invoke(
        ctx context.Context,
        request PluginInvocationRequest,
    ) PluginInvocationResult

    Health(
        ctx context.Context,
    ) PluginRuntimeHealth

    Stop(
        ctx context.Context,
        reason PluginStopReason,
    ) error
}
```

要求：

- Runtime 不直接操作 ToolRegistry；
-Runtime 不直接操作 Permission Grant；
-Runtime 不直接修改 Scope；
-Runtime 不直接写统一审计表；
-Runtime 不直接创建 Extension Owner；
-Runtime 必须接受 Context；
-Runtime 必须支持 Stop；
-Runtime 必须返回结构化结果；
-Runtime Panic/Crash 必须被 Safety Layer 捕获。

---

## 六、Plugin Runtime Type

建议：

```go
type PluginRuntimeType string

const (
    PluginRuntimeLegacyGo PluginRuntimeType = "legacy_go"
    PluginRuntimeJavaScript PluginRuntimeType = "javascript"
    PluginRuntimeService PluginRuntimeType = "service"
    PluginRuntimeWASM PluginRuntimeType = "wasm"
)
```

本步骤完整实现：

```text
legacy_go
```

其余仅定义接口和占位，不实现正式 Runtime。

---

## 七、Plugin Runtime Spec

建议：

```go
type PluginRuntimeSpec struct {
    ExtensionID    string
    ModuleID       string
    RuntimeID      string
    RuntimeType    PluginRuntimeType
    EntryPoint     string
    Version        string
    Permissions    []PermissionRequirement
    ResourceLimits ResourceLimits
    TimeoutPolicy  PluginTimeoutPolicy
    Concurrency    PluginConcurrencyPolicy
    RecoveryPolicy PluginRecoveryPolicy
    Metadata       map[string]any
}
```

要求：

- 来自经过验证的 Extension/Module Definition；
-不包含 Secret 明文；
-不包含角色或会话当前值；
-不包含 Permission Grant；
-不包含前端状态；
-EntryPoint 对 Legacy Go 为稳定 Handler 标识，不是任意反射字符串。

---

## 八、Plugin Invocation 模型

建议：

```go
type PluginInvocationRequest struct {
    InvocationID   string
    TraceID        string
    ParentID       string
    ExtensionID    string
    ModuleID       string
    RuntimeID      string
    EntryType      PluginEntryType
    EntryName      string
    Input          json.RawMessage
    ScopeSnapshot  ScopeSnapshot
    Deadline       time.Time
    IdempotencyKey string
    Metadata       map[string]any
}
```

EntryType：

```text
tool
hook
event
schedule
background_task
lifecycle
ui_callback
internal
```

---

## 九、Plugin Invocation Result

建议：

```go
type PluginInvocationResult struct {
    Status      string
    Output      json.RawMessage
    Error       *ToolError
    SideEffects []RecordedSideEffect
    StateWrites []PluginStateMutation
    Resources   []PluginResourceMutation
    Metadata    map[string]any
}
```

要求：

- Runtime 不直接持久化 State；
-State Mutation 交由 PluginStateBroker；
-Resource Mutation 交由 ResourceOwnershipService；
-Side Effect 进入统一记录；
-结果必须经过统一 Result Validator；
-错误必须结构化；
-不得返回 Secret。

---

## 十、Runtime Supervisor

建议定义：

```go
type PluginRuntimeSupervisor interface {
    Start(
        ctx context.Context,
        runtimeID string,
    ) error

    Stop(
        ctx context.Context,
        runtimeID string,
        reason PluginStopReason,
    ) error

    Restart(
        ctx context.Context,
        runtimeID string,
    ) error

    Invoke(
        ctx context.Context,
        request PluginInvocationRequest,
    ) PluginInvocationResult

    State(
        runtimeID string,
    ) PluginRuntimeStateSnapshot
}
```

职责：

- Runtime 实例创建；
-生命周期；
-状态机；
-健康；
-重启；
-熔断；
-并发；
-资源；
-审计关联；
-应用关闭；
-崩溃恢复。

---

## 十一、Runtime 状态机

建议状态：

```text
created
starting
ready
degraded
stopping
stopped
crashed
failed
disabled
quarantined
```

合法迁移示例：

```text
created → starting → ready
ready → degraded
degraded → ready
ready → stopping → stopped
ready → crashed
crashed → starting
crashed → quarantined
```

禁止：

```text
stopped → ready
failed → ready
disabled → starting
```

除非经过明确 Start/Enable 流程。

---

## 十二、Safety Guard

建议：

```go
type PluginRuntimeSafetyGuard interface {
    Execute(
        ctx context.Context,
        runtime PluginRuntime,
        request PluginInvocationRequest,
    ) PluginInvocationResult
}
```

固定流程：

```text
1. Validate Runtime State
2. Validate Invocation
3. Evaluate Circuit
4. Check Depth
5. Check Rate
6. Acquire Concurrency Slot
7. Create Timeout Context
8. Register Invocation Resource
9. Execute Runtime
10. Recover Panic/Crash
11. Validate Result
12. Apply State CAS
13. Register Resources
14. Record Side Effects
15. Update Health
16. Update Circuit
17. Release Resources
18. Audit
```

---

## 十三、Panic 隔离

Legacy Go Plugin 必须为每次入口调用提供 Panic Recovery。

捕获内容：

- Panic 值；
-受限堆栈引用；
-Runtime ID；
-Entry；
-Invocation；
-Extension；
-Module；
-时间；
-当前状态。

要求：

- 不让 Panic 崩溃宿主；
-不把完整堆栈返回模型；
-更新 Runtime Health；
-增加 Circuit 失败计数；
-释放锁和并发槽；
-执行必要清理；
-写统一 Audit；
-高频 Panic 触发隔离或熔断。

---

## 十四、Crash 与 Panic 的区别

### Panic

适用于宿主进程内 Legacy Go Runtime。

### Crash

适用于未来独立进程 Runtime。

统一映射为 Runtime Failure，但诊断类型不同：

```text
panic
process_exit
protocol_disconnect
memory_limit
cpu_limit
watchdog_kill
unknown_crash
```

---

## 十五、超时模型

建议：

```go
type PluginTimeoutPolicy struct {
    ToolTimeout          time.Duration
    HookTimeout          time.Duration
    EventTimeout         time.Duration
    ScheduleTimeout      time.Duration
    LifecycleTimeout     time.Duration
    BackgroundTaskTimeout time.Duration
    StopTimeout          time.Duration
}
```

要求：

- 每个入口类型有默认值；
-调用方可缩短，不能任意延长；
-子调用不能超过父 Deadline；
-超时后取消 Context；
-超时不自动等于 Runtime Crash；
-连续超时影响 Health 和 Circuit；
-Stop 超时后进入强制清理策略。

---

## 十六、取消传播

取消链：

```text
ExecutionSecurityKernel / Event Bus / Scheduler
→ PluginRuntimeSupervisor
→ SafetyGuard
→ PluginRuntime
→ Host API child calls
```

要求：

- Runtime 内 Host API 调用继承 Context；
-取消后不得继续创建资源；
-取消后不得提交新 State Mutation；
-已发生副作用必须记录；
-取消释放并发槽；
-未来进程 Runtime 需发送取消协议；
-Legacy Go Runtime 只能依赖 Context 合作取消，限制需明确记录。

---

## 十七、并发控制

建议：

```go
type PluginConcurrencyPolicy struct {
    MaxRuntimeConcurrency int
    MaxToolConcurrency    int
    MaxHookConcurrency    int
    MaxEventConcurrency   int
    MaxScheduleConcurrency int
    MaxPerCharacter       int
    MaxPerConversation    int
}
```

要求：

- 接入统一 ConcurrencyController；
-不建立完全独立的全局信号量系统；
-不同入口可有子配额；
-高风险入口可串行；
-队列有界；
-取消可移除等待；
-停止 Runtime 时拒绝新请求；
-避免 Runtime 内部反向调用导致自锁。

---

## 十八、速率限制

Plugin Runtime 可定义来源级速率策略，但执行由统一 RateLimiter 完成。

可按：

- Extension；
-Module；
-Runtime；
-Entry；
-Character；
-Conversation；
-Event Type；
-Schedule。

Plugin 不得自行隐藏限流。

---

## 十九、深度与递归保护

统一记录：

```text
root_invocation_id
parent_invocation_id
depth
call_chain
event_depth
hook_depth
```

必须防止：

- Plugin Tool 调用自身；
-Plugin A 调用 Plugin B，再回调 A；
-Hook 触发 Event；
-Event 触发 Workflow；
-Workflow 再调用 Plugin；
-Plugin State Change 再触发自身；
-Schedule 触发重复链。

建议：

- 总调用深度统一限制；
-Event 深度可有更低上限；
-Hook 深度可有更低上限；
-同一 Entry 连续出现次数限制；
-循环检测；
-错误结构化。

---

## 二十、Circuit Breaker

建议定义：

```go
type PluginCircuitBreaker interface {
    Allow(runtimeID string, entry string) CircuitDecision
    RecordSuccess(runtimeID string, entry string)
    RecordFailure(runtimeID string, entry string, failure CircuitFailure)
    Reset(runtimeID string, entry string) error
    State(runtimeID string, entry string) CircuitState
}
```

状态：

```text
closed
open
half_open
```

---

## 二十一、熔断维度

至少支持：

```text
runtime
entry
tool
hook
event
schedule
```

默认可先实现 Runtime + Entry 两级。

触发原因：

- 连续 Panic；
-连续超时；
-错误率；
-非法结果；
-资源泄漏；
-Host API 违规；
-连接崩溃；
-停止失败；
-状态冲突过高。

---

## 二十二、熔断规则

建议：

```go
type PluginCircuitPolicy struct {
    FailureThreshold  int
    TimeoutThreshold  int
    Window            time.Duration
    OpenDuration      time.Duration
    HalfOpenMaxCalls  int
}
```

要求：

- 熔断影响 Availability；
-ToolRegistry 读取统一状态；
-UI Contribution 可根据策略隐藏或显示故障；
-Hook/Event 停止投递；
-Schedule 暂停；
-手动重置写 Audit；
-Extension Enabled 不自动清除熔断；
-版本升级可按策略重置。

---

## 二十三、Health Monitor

建议：

```go
type PluginRuntimeHealth struct {
    Status              HealthStatus
    LastSuccessAt       *time.Time
    LastFailureAt       *time.Time
    ConsecutiveFailures int
    TimeoutRate         float64
    PanicCount          int
    ActiveInvocations   int
    QueueDepth          int
    ResourceLeakCount   int
    LastErrorCode       string
}
```

HealthStatus：

```text
healthy
degraded
unhealthy
unknown
```

Health 只描述运行健康，不表示 Enabled 或 Permission。

---

## 二十四、健康更新来源

包括：

- Start；
-Stop；
-Invocation；
-Panic；
-Timeout；
-Result Validation；
-State Conflict；
-Resource Cleanup；
-Host API；
-进程或连接；
-Watchdog；
-定期 Probe。

---

## 二十五、Plugin State Broker

现有 Plugin State 若支持 KV、Version、CAS，应抽取为独立 Broker。

建议：

```go
type PluginStateBroker interface {
    Get(
        ctx context.Context,
        owner ResourceOwner,
        namespace string,
        key string,
    ) (PluginStateValue, error)

    CompareAndSwap(
        ctx context.Context,
        mutation PluginStateMutation,
    ) (PluginStateValue, error)

    Delete(
        ctx context.Context,
        mutation PluginStateDelete,
    ) error

    List(
        ctx context.Context,
        query PluginStateQuery,
    ) ([]PluginStateEntry, error)
}
```

---

## 二十六、State Namespace

命名空间必须绑定：

```text
extension_id
module_id
scope
namespace
```

示例：

```text
extension/com.example.weather/module/main/global/cache
extension/com.example.weather/module/main/character/<id>/preferences
```

Plugin 不得自行拼接数据库表名或主目录路径。

---

## 二十七、State CAS

建议：

```go
type PluginStateMutation struct {
    ExtensionID     string
    ModuleID        string
    Namespace       string
    Key             string
    ExpectedVersion int64
    NewValue        json.RawMessage
    ScopeSnapshotID string
    InvocationID    string
}
```

要求：

- 版本冲突结构化返回；
-不自动无限重试；
-值大小限制；
-Schema 可选；
-敏感状态不得写普通 Store；
-Secret 使用 Secret Broker；
-取消后不提交；
-Owner 和 Scope 校验；
-写入审计摘要。

---

## 二十八、State 与执行事务

Runtime 返回 State Mutation 后：

```text
Result Validate
→ State CAS
→ Resource Mutation
→ Side Effect Record
→ Finalize Result
```

若 CAS 失败：

- Tool 结果不得直接成功；
-可根据 Tool 幂等策略重试；
-已发生外部副作用必须记录；
-不能简单重复整个非幂等调用；
-可返回 conflict；
-Plugin Runtime 不得绕过 Broker 直接写。

---

## 二十九、Plugin Resource Tracker

建议：

```go
type PluginResourceTracker interface {
    Register(
        ctx context.Context,
        mutation PluginResourceMutation,
    ) error

    ReleaseInvocationResources(
        ctx context.Context,
        invocationID string,
    ) error

    ReleaseRuntimeResources(
        ctx context.Context,
        runtimeID string,
    ) ResourceReleaseResult
}
```

资源包括：

- Timer；
-Worker；
-Event Subscription；
-Hook；
-Schedule；
-Window；
-Tray Action；
-File Watcher；
-Temporary File；
-Process；
-Connection；
-Cache；
-UI Contribution；
-Host Handle。

---

## 三十、Invocation 资源与 Runtime 资源

必须区分：

### Invocation Resource

调用结束后释放。

例如：

- 临时文件；
-临时订阅；
-临时句柄；
-临时网络连接。

### Runtime Resource

Runtime Stop 时释放。

例如：

- 长期 Event Subscription；
-Worker；
-Schedule；
-UI；
-Window；
-Tray；
-Process。

### Persistent Resource

由 Resource Ownership 管理，不因 Stop 删除。

例如：

- State；
-配置；
-用户数据；
-Secret；
-Definition。

---

## 三十一、清理顺序

Runtime Stop 建议：

```text
1. Reject new invocations
2. Mark stopping
3. Cancel queued invocations
4. Cancel running invocations
5. Wait graceful timeout
6. Unregister hooks
7. Unsubscribe events
8. Pause schedules
9. Stop workers
10. Close windows/tray resources
11. Close connections
12. Stop processes
13. Release temporary resources
14. Flush state
15. Verify no owned runtime resource remains
16. Mark stopped
17. Audit
```

---

## 三十二、清理失败

清理失败必须：

- 标记 degraded/failed；
-创建 Cleanup Job；
-记录 Resource ID；
-重试；
-在应用启动恢复阶段扫描；
-影响卸载 Release Plan；
-前端可见；
-不得静默忽略。

---

## 三十三、Hook 安全边界

Hook 入口必须明确：

```text
before
after
filter
transform
observe
```

每个 Hook 必须声明：

- Hook Point；
-输入 Schema；
-输出 Schema；
-是否允许修改；
-优先级；
-超时；
-风险；
-Scope；
-Owner；
-失败策略。

---

## 三十四、Hook 失败策略

支持：

```text
ignore
fail_operation
fallback
disable_hook
open_circuit
```

必须由 Host Hook Point 定义允许范围。

Plugin 不得自行决定关键宿主 Hook 失败后继续。

---

## 三十五、Hook 输出

Hook 输出必须：

- Schema 校验；
-大小限制；
-不允许修改未授权字段；
-记录变更摘要；
-敏感内容脱敏；
-不可返回 Runtime 对象；
-不可返回函数；
-不可写入全局状态。

---

## 三十六、Event 安全边界

Event Subscription 必须声明：

- Event Type；
-Input Schema；
-Scope；
-过滤条件；
-Delivery Policy；
-Retry；
-Idempotency；
-Owner；
-最大并发；
-最大深度。

Event Payload 必须最小化，不默认包含完整角色、会话或消息对象。

---

## 三十七、Event Delivery

正确链路：

```text
Event Bus
→ Subscription Resolve
→ Scope/Permission
→ Depth Guard
→ Concurrency/Rate
→ PluginRuntimeSupervisor
→ Runtime
→ Result/Audit
```

禁止 Event Bus 直接调用 Go Plugin Handler。

---

## 三十八、Event 重试

仅在以下情况重试：

- Runtime 临时不可用；
-可重试错误；
-事件处理声明幂等；
-未发生不可逆副作用。

必须使用稳定 Delivery ID。

禁止无限重试。

---

## 三十九、Schedule 安全边界

Plugin Schedule 是独立资源。

必须记录：

- Owner；
-Extension；
-Module；
-Runtime；
-Entry；
-Scope Snapshot；
-Permission Reference；
-Recurrence；
-Timezone；
-重叠策略；
-Idempotency；
-失败策略。

Schedule 不得在运行时读取当前前端角色。

---

## 四十、后台任务

Plugin Background Task 必须：

- 有 Task ID；
-有 Owner；
-有 Scope；
-有 Permission；
-有 Deadline；
-有取消；
-有资源限制；
-有状态；
-有审计；
-可被 Runtime Stop 取消；
-可按策略恢复。

---

## 四十一、Lifecycle Hook

支持：

```text
on_install
on_enable
on_start
on_stop
on_disable
on_update
on_rollback
on_uninstall
```

但当前 Legacy Go Plugin 如不存在完整生命周期，不应在本步骤强行新增执行逻辑。

规则：

- 生命周期 Hook 也经过 Safety Guard；
-安装阶段不得任意执行不可信代码；
-未来 `.amitiax` v2 的安装安全与 Runtime 生命周期分离；
-on_uninstall 失败不能阻止宿主清理私有资源；
-生命周期 Hook 不能覆盖 Package 事务。

---

## 四十二、Host API Boundary

Plugin Runtime 访问 Amitia 必须通过 Host API Gateway。

本步骤至少定义安全接口：

```go
type PluginHostGateway interface {
    Call(
        ctx context.Context,
        request PluginHostCallRequest,
    ) PluginHostCallResult
}
```

Host API 调用必须：

- 验证 Runtime 身份；
-验证 Extension/Module；
-验证 Permission；
-验证 Scope；
-验证 Input；
-限制结果；
-记录审计；
-继承 Trace；
-继承 Deadline；
-继承取消；
-不返回内部 Service 对象。

---

## 四十三、Runtime 身份

每个 Runtime 启动时必须获得不可伪造的 Runtime Identity。

至少包含：

```text
runtime_id
extension_id
module_id
runtime_type
version
session_nonce
```

Host API 不接受 Plugin 自报 Extension ID。

Legacy Go Runtime 也必须通过 Supervisor 注入身份。

---

## 四十四、Tool 调用

Plugin 内部调用其他 Tool 时：

```text
Plugin Runtime
→ Host API Gateway
→ ToolExecutionRequest
→ ExecutionSecurityKernel
```

必须建立子 Invocation。

子调用：

- Scope 不得扩大；
-Deadline 不得延长；
-Permission 不得提升；
-Depth 增加；
-Trace 继承；
-副作用关联；
-取消传播。

---

## 四十五、Plugin Runtime Adapter

建议：

```go
type PluginRuntimeAdapter struct {
    Supervisor PluginRuntimeSupervisor
}
```

实现统一 RuntimeAdapter：

```go
Execute(
    ctx context.Context,
    binding RuntimeBinding,
    invocation ToolInvocationContext,
    input json.RawMessage,
) ToolResult
```

职责：

- 将 Tool Invocation 转为 Plugin Invocation；
-调用 Supervisor；
-映射结果；
-不重复实现 Permission、Scope、Audit；
-不直接访问 Legacy Handler。

---

## 四十六、Legacy Go Plugin Runtime

现有 Go Plugin 通过适配器包装：

```go
type LegacyGoPluginRuntime struct {
    Registry LegacyGoPluginRegistry
}
```

内部可持有现有 Handler，但：

- 只能由 Supervisor 调用；
-每次调用经过 Safety Guard；
-不得直接注册 Tool；
-不得直接写 State；
-不得直接触发 Event；
-不得直接访问 Repository；
-逐步改为 Host API。

---

## 四十七、Legacy Handler 迁移

对每个现有 Plugin Handler 盘点：

- Tool；
-Hook；
-Event；
-Schedule；
-State；
-Host Service；
-资源；
-权限；
-Scope；
-副作用；
-超时；
-错误；
-Panic。

按以下顺序迁移：

```text
直接 Service 调用
→ Host API
直接 State 写入
→ State Broker
直接 Tool 注册
→ Contribution/ToolDefinition
直接 Event 调用
→ Event Bus Adapter
直接 Timer
→ Schedule/Background Task
```

---

## 四十八、运行健康与 Availability

AvailabilityEvaluator 应读取：

- Runtime State；
-Health；
-Circuit；
-Extension Enabled；
-Module Enabled；
-Contribution Enabled；
-依赖；
-平台。

PluginRuntimeSupervisor 提供运行状态，但不直接修改 Tool Enabled。

---

## 四十九、隔离与 Quarantine

当发生严重违规时可进入：

```text
quarantined
```

触发示例：

- 高频 Panic；
-反复 Host API 越权；
-资源泄漏；
-非法结果；
-无法停止；
-疑似无限循环；
-重复崩溃；
-签名或运行内容变化；
-状态损坏。

Quarantine 后：

- 拒绝新调用；
-停止 Runtime；
-注销临时 Contribution；
-暂停 Schedule；
-保留数据；
-前端提示；
-需要手动恢复或更新；
-写高风险审计。

---

## 五十、恢复策略

建议：

```go
type PluginRecoveryPolicy struct {
    RestartOnCrash       bool
    MaxRestarts          int
    RestartWindow        time.Duration
    Backoff              BackoffPolicy
    QuarantineOnExhausted bool
}
```

规则：

- 用户手动禁用不自动重启；
-熔断期间不反复重启；
-升级中不自动恢复旧 Runtime；
-状态损坏不自动重启；
-重启前清理旧资源；
-重启后重新注册运行时资源；
-Definition 和 Contribution 不重复注册。

---

## 五十一、应用启动恢复

启动时扫描：

- starting；
-stopping；
-crashed；
-degraded；
-cleanup pending；
-孤儿资源；
-运行中 Schedule；
-残留 Process/Connection。

Legacy Go Runtime 不应自动恢复上次内存状态，只按 Definition 重新启动。

---

## 五十二、统一运行审计接入

Plugin 相关记录统一映射：

### Runtime Start/Stop

```text
Operation + Runtime Event
```

### Tool/Hook/Event/Schedule

```text
Invocation + Attempt
```

### Panic/Crash

```text
Runtime Event + Error Record + Audit
```

### Circuit

```text
Runtime Event
```

### State CAS

```text
Audit Summary
```

### Resource Cleanup

```text
Resource Operation + Audit
```

不再新增独立 Plugin Run 主表作为唯一事实来源。

---

## 五十三、指标

至少记录：

- Runtime 启动成功率；
-调用成功率；
-超时；
-Panic；
-熔断；
-重启；
-队列；
-并发；
-State CAS 冲突；
-资源泄漏；
-清理失败；
-Host API 拒绝；
-Event 重试；
-Schedule 延迟。

Metrics 标签避免高基数。

---

## 五十四、状态持久化

建议目标表：

```text
plugin_runtime_definitions
plugin_runtime_states
plugin_runtime_health
plugin_circuit_states
plugin_state_entries
plugin_state_versions
plugin_cleanup_jobs
plugin_runtime_migrations
```

运行记录仍进入统一 Observability 表。

旧 Plugin Run、Event、Delivery、Schedule 表后续迁移删除。

---

## 五十五、前端运行状态

扩展详情页应区分：

### Runtime

- Type；
-State；
-Health；
-Circuit；
-Last Start；
-Last Crash；
-Restart Count；
-Queue；
-Active Invocations。

### Contributions

- Tool；
-Hook；
-Event；
-Schedule；
-UI；
-Background Task。

### Resources

- Worker；
-Process；
-Connection；
-Timer；
-State；
-Temporary Resource。

### Diagnostics

- Error；
-Panic；
-Cleanup；
-Host API Denial；
-Quarantine。

---

## 五十六、前端操作

允许：

- Start；
-Stop；
-Restart；
-Reset Circuit；
-Disable Module；
-查看 Health；
-查看 Resource；
-清理孤儿；
-导出诊断；
-解除 Quarantine。

所有操作必须经过后端权限和审计。

---

## 五十七、开发者模式

开发者模式可提供：

- Runtime Restart；
-调用追踪；
-State 查看；
-Hook/Event 测试；
-资源列表；
-强制 Cleanup；
-模拟 Panic；
-模拟 Timeout；
-模拟 Circuit；
-热重载预留。

但不得：

- 绕过 Host API；
-绕过 Permission；
-绕过 Scope；
-关闭 Panic Recovery；
-关闭路径和资源限制；
-直接编辑生产 State 版本；
-删除审计。

---

## 五十八、旧系统迁移

需要盘点：

- Plugin Registry；
-PluginManager；
-Hook Manager；
-Event Dispatcher；
-Delivery；
-Schedule；
-State Store；
-Circuit；
-Panic Recovery；
-Timeout；
-Worker；
-Timer；
-Resource；
-前端 Plugin 页面。

迁移规则：

- 现有 Go Plugin 注册为 LegacyGoPluginRuntime；
-现有 Handler 由 Supervisor 调用；
-现有 Circuit 迁入统一模型；
-现有 State 迁入 State Broker；
-现有 Hook/Event/Schedule 改为统一安全入口；
-现有运行记录映射统一审计；
-新功能不得继续写旧 PluginManager 主链。

---

## 五十九、兼容层约束

允许：

```text
旧 PluginManager.Call
→ PluginRuntimeSupervisor.Invoke
```

允许：

```text
旧 Hook Dispatcher
→ Plugin Hook Adapter
→ Supervisor
```

禁止：

```text
新 Supervisor
→ 旧 PluginManager 再执行完整逻辑
```

禁止新旧 State 双写。

---

## 六十、测试要求

必须新增：

### 1. Runtime 状态机

- Start；
-Ready；
-Stop；
-Restart；
-Crash；
-Failed；
-Disabled；
-Quarantine；
-非法迁移。

### 2. Panic Recovery

- Tool；
-Hook；
-Event；
-Schedule；
-Lifecycle；
-资源释放；
-锁释放；
-审计；
-Circuit。

### 3. Timeout 与 Cancel

- 各入口；
-子 Tool；
-Host API；
-Stop；
-队列；
-取消后 State 不提交。

### 4. Concurrency

- Runtime；
-Entry；
-角色；
-会话；
-队列；
-死锁；
-停止时等待；
-自调用。

### 5. Depth Guard

- Tool 自递归；
-Plugin 互调；
-Hook/Event 循环；
-Workflow 循环；
-最大深度；
-调用链。

### 6. Circuit Breaker

- Open；
-Half Open；
-Close；
-手动 Reset；
-版本升级；
-超时；
-Panic；
-非法结果；
-Availability。

### 7. Health

- Success；
-Failure；
-Degraded；
-Unhealthy；
-恢复；
-Probe；
-清理失败。

### 8. State Broker

- Get；
-CAS；
-Conflict；
-Delete；
-Namespace；
-Scope；
-Owner；
-大小；
-取消；
-迁移。

### 9. Resource Tracker

- Invocation Resource；
-Runtime Resource；
-Persistent Resource；
-Stop Cleanup；
-Crash Cleanup；
-孤儿；
-重试。

### 10. Hook

- 顺序；
-超时；
-输出 Schema；
-修改限制；
-失败策略；
-熔断。

### 11. Event

- Subscription；
-Delivery；
-幂等；
-重试；
-Depth；
-取消；
-禁用；
-卸载。

### 12. Schedule

- Scope；
-Permission；
-重叠；
-禁用；
-重启；
-清理；
-Owner。

### 13. Host API

- Identity；
-Permission；
-Scope；
-Deadline；
-取消；
-Tool 子调用；
-越权；
-Secret 脱敏。

### 14. Legacy Go Runtime

- 现有插件行为等价；
-Handler 包装；
-Panic；
-State；
-资源；
-Tool Adapter。

### 15. Recovery

- Crash；
-连续重启；
-Quarantine；
-Cleanup Pending；
-应用重启。

### 16. 性能

- 高并发；
-大量 Event；
-State CAS；
-Circuit；
-资源跟踪；
-Stop 延迟。

---

## 六十一、实施任务

### Task 1：定义 Plugin Runtime Contract

完成 Runtime、Spec、Invocation、Result、State。

### Task 2：建立 PluginRuntimeSupervisor

统一 Start、Stop、Restart、Invoke 和 State。

### Task 3：建立 SafetyGuard

固定执行安全流程。

### Task 4：包装 LegacyGoPluginRuntime

接入现有 Go Plugin。

### Task 5：统一 Panic Recovery

覆盖所有入口。

### Task 6：统一 Timeout 和 Cancellation

接入统一 Context。

### Task 7：统一 Concurrency 和 Rate

复用 Execution Security 基础组件。

### Task 8：实现 Depth Guard

覆盖 Tool、Hook、Event、Workflow。

### Task 9：抽取 Circuit Breaker

成为独立服务并接入 Availability。

### Task 10：实现 Health Monitor

形成统一健康快照。

### Task 11：重构 Plugin State Broker

统一 Namespace、Version、CAS 和 Scope。

### Task 12：建立 Plugin Resource Tracker

跟踪 Invocation、Runtime 和 Persistent Resource。

### Task 13：建立 CleanupCoordinator

统一 Stop、Crash、Disable、Uninstall 清理。

### Task 14：建立 CrashRecovery

支持 Restart、Backoff 和 Quarantine。

### Task 15：重构 Hook 入口

统一 Supervisor 调用。

### Task 16：重构 Event Delivery

接入统一安全入口与幂等。

### Task 17：重构 Schedule 调用

统一 Owner、Scope、Permission 和取消。

### Task 18：建立 Host API Gateway 边界

不直接暴露内部 Service。

### Task 19：实现 PluginRuntimeAdapter

接入 Tool Executor。

### Task 20：接入 Resource Ownership

登记 Runtime、Worker、Process、Connection、Timer 和临时资源。

### Task 21：接入统一 Audit

替换独立 Plugin Run 主链。

### Task 22：迁移旧 Plugin State 和 Circuit

停止新写旧结构。

### Task 23：重构前端 Runtime 状态页

展示 State、Health、Circuit、Resource 和诊断。

### Task 24：增加旧 PluginManager 调用统计

识别剩余直接执行入口。

### Task 25：完成故障注入与回归测试

验证 Panic、Crash、Timeout、Cleanup 和 Quarantine。

---

## 六十二、建议目录结构

建议：

```text
backend/internal/extension/kernel/plugin_runtime/
├── runtime.go
├── spec.go
├── invocation.go
├── result.go
├── state.go
├── supervisor.go
├── safety_guard.go
├── timeout.go
├── concurrency.go
├── depth.go
├── circuit.go
├── health.go
├── state_broker.go
├── resource_tracker.go
├── cleanup.go
├── recovery.go
├── host_gateway.go
├── legacy_go_runtime.go
├── migration.go
└── audit.go
```

入口适配：

```text
backend/internal/extension/kernel/plugin_runtime/entries/
├── tool.go
├── hook.go
├── event.go
├── schedule.go
├── background_task.go
└── lifecycle.go
```

前端：

```text
front/src/views/extensions/runtime/
├── PluginRuntimeOverview.vue
├── PluginRuntimeHealth.vue
├── PluginCircuitState.vue
├── PluginRuntimeResources.vue
├── PluginStateInspector.vue
├── PluginRuntimeDiagnostics.vue
└── PluginRuntimeRecovery.vue
```

目录仅为建议。

---

## 六十三、性能要求

建议：

- Supervisor 查询不全表扫描；
-Runtime State 内存快照与持久化分离；
-Health 更新节流；
-Circuit 使用时间窗口；
-Event 队列有界；
-State CAS 使用索引；
-Resource Tracker 按 Runtime/Invocation 索引；
-Stop 清理并行但有顺序依赖；
-Panic 堆栈不写入高频日志；
-指标低基数；
-大量 Runtime 不启动无需求 Worker；
-Legacy Go Runtime 调用额外开销可控。

---

## 六十四、风险控制

### P0：宿主稳定性

- Panic 逃逸；
-死锁；
-资源泄漏；
-无限递归；
-无法 Stop；
-越权 Host API；
-取消不生效。

### P1：副作用与状态不一致

- State CAS 失败但结果成功；
-Event 重试重复副作用；
-清理后仍有 Schedule；
-Circuit Open 但 Tool 仍执行；
-Quarantine 后 UI/Hook 仍活跃。

### P2：迁移回退

- 新 Supervisor 仍调用旧完整 PluginManager；
-新旧 State 双写；
-旧 Event Delivery 继续执行；
-旧 Schedule 未停；
-前端继续依赖旧状态。

### P3：性能问题

- 每次调用同步持久化 Health；
-资源跟踪过重；
-高频 Event 导致队列爆炸；
-State CAS 冲突过高；
-Stop 顺序过慢。

---

## 六十五、本步骤不做的事情

本步骤明确不做：

- 不实现正式 JavaScript Runtime；
-不实现独立 Service Runtime；
-不实现 WASM Runtime；
-不实现完整 Host API；
-不实现 Event Bus 全部功能；
-不实现 Hook Pipeline 全部功能；
-不实现 `.amitiax` v2 Manifest；
-不实现 UI Contribution；
-不删除旧 PluginManager；
-不删除旧 Plugin 表；
-不实现插件市场；
-不实现移动端；
-不允许第三方原生代码直接载入宿主进程。

---

## 六十六、验收产物

完成后必须提交：

### 1. Plugin Runtime Safety 主文档

```text
docs/extension-kernel/17-plugin-runtime-safety.md
```

### 2. Plugin Runtime Contract

包含：

- PluginRuntime；
-PluginRuntimeSpec；
-PluginInvocationRequest；
-PluginInvocationResult；
-PluginRuntimeState。

### 3. PluginRuntimeSupervisor

支持：

- Start；
-Stop；
-Restart；
-Invoke；
-State；
-Health；
-Recovery。

### 4. SafetyGuard

覆盖：

- Timeout；
-Cancel；
-Panic；
-Concurrency；
-Rate；
-Depth；
-Circuit；
-Result；
-State；
-Resource；
-Audit。

### 5. LegacyGoPluginRuntime

现有 Go Plugin 可通过新 Supervisor 运行。

### 6. Circuit Breaker 与 Health

可被 AvailabilityEvaluator 使用。

### 7. PluginStateBroker

支持 Namespace、Scope、Version 和 CAS。

### 8. PluginResourceTracker

覆盖 Invocation、Runtime 和 Persistent Resource。

### 9. CleanupCoordinator

支持 Stop、Disable、Crash 和 Uninstall 清理。

### 10. Host API Gateway 边界

至少完成身份、Permission、Scope、Trace 和取消校验框架。

### 11. Hook/Event/Schedule 入口迁移

不再直接调用 Legacy Handler。

### 12. PluginRuntimeAdapter

Plugin Tool 经过统一 Tool Executor。

### 13. 统一 Audit 接入

不再新增独立 Plugin Run 主链。

### 14. 前端 Runtime 状态页

展示 State、Health、Circuit、Resource 和诊断。

### 15. 迁移报告

列出：

- 已迁移 Plugin；
-仍直接调用 Handler 的入口；
-仍使用旧 State 的入口；
-仍使用旧 Circuit 的入口；
-仍由旧 Event Delivery 执行的入口；
-仍未纳入资源跟踪的资源；
-清理失败项。

### 16. 测试报告

覆盖 Runtime、Panic、Timeout、Cancel、Concurrency、Depth、Circuit、Health、State、Resource、Hook、Event、Schedule、Host API、Recovery 和性能。

---

## 六十七、验收标准

本步骤通过必须满足：

1. Plugin Runtime Contract 已独立定义。
2. 当前 Go Plugin 已包装为 LegacyGoPluginRuntime。
3. Plugin Tool 不再直接通过 PluginManager 执行。
4. Tool、Hook、Event、Schedule 均经过 PluginRuntimeSupervisor。
5. 所有入口都有 Panic/Crash 隔离。
6. 所有入口都有 Timeout 和 Cancel。
7. 并发与速率限制已统一。
8. Tool、Hook、Event、Workflow 递归有统一 Depth Guard。
9. Circuit Breaker 可影响 Availability。
10. Health 与 Enabled 已分离。
11. Plugin State 通过 Broker 和 CAS 管理。
12. Plugin 不再直接写状态存储。
13. Runtime 资源可完整跟踪和清理。
14. 禁用、Stop、Crash、卸载具有明确清理顺序。
15. Host API 有身份、Permission 和 Scope 边界。
16. Plugin 内部 Tool 调用创建子 Invocation。
17. Quarantine 可阻止故障 Runtime 继续运行。
18. 运行记录进入统一审计。
19. 新数据不再写旧 Plugin 执行主链。
20. 故障注入和资源泄漏测试通过。
21. 后续第 18 步可以统一启动、恢复和关闭流程。

---

## 六十八、退出条件

只有满足以下条件后，才能进入第 18 步“统一启动、恢复和关闭流程”：

- Plugin Runtime Contract 已落地；
-Supervisor 已落地；
-SafetyGuard 已落地；
-LegacyGoPluginRuntime 已接入；
-Circuit 与 Health 已接入 Availability；
-State Broker 已落地；
-Resource Tracker 与 Cleanup 已落地；
-Hook/Event/Schedule 已进入统一入口；
-Host API 边界已建立；
-旧 PluginManager 只剩兼容用途；
-关键故障注入测试通过；
-没有新增直接 Handler 执行入口。

---

## 六十九、执行约束

执行本步骤时必须遵守：

> 本步骤只抽取 Plugin Runtime 的安全保护基础，不得借机继续强化旧 Go Plugin 架构，也不得提前实现未经设计的第三方插件执行环境。

禁止出现：

- Plugin Handler 继续直接访问内部 Service；
-PluginManager 继续成为新功能入口；
-Plugin Tool 绕过 ToolExecutor；
-Hook/Event/Schedule 绕过 Supervisor；
-Panic Recovery 只覆盖 Tool；
-Circuit Open 后仍允许新调用；
-Health 状态替代 Enabled；
-State 写入绕过 CAS；
-Stop 前先删除资源定义；
-清理失败不记录；
-新旧 Plugin State 双写；
-为支持第三方插件直接加载动态 Go 代码。

本步骤完成后，Amitia 必须具备一套可复用、可监督、可熔断、可恢复、可清理、可审计的 Plugin Runtime 安全基础。

# Amitia 扩展系统重构第 25 步实施文档

## 第 25 步：实现统一 Runtime Supervisor

---

## 一、步骤目标

在第 17 步完成 Plugin Runtime Safety、第 18 步完成统一启停流程、第 21 步定义 RuntimeDefinition/RuntimeInstance、第 24 步完成依赖解析后，建立 Extension Kernel 唯一 Runtime Supervisor。

目标：

> 统一管理 host_internal、legacy_go、javascript、task、service、wasm、mcp、workflow 与 static Runtime 的实例创建、Desired/Actual State、资源限制、健康、熔断、重启、升级代际和停止清理。

---

## 二、职责

Runtime Supervisor 负责：

- Runtime Factory 注册；
-创建实例；
-启动、停止、重启；
-Desired/Actual 对账；
-Generation；
-依赖快照；
-Host API Session；
-资源限制；
-健康；
-熔断；
-崩溃；
-重启策略；
-调用路由；
-实例隔离；
-运行时资源登记；
-清理；
-指标与审计。

不负责：

- Extension 安装；
-业务 Enabled；
-Permission Grant；
-Scope Binding；
-Contribution Definition；
-依赖安装；
-Tool 业务执行策略；
-前端生命周期编排。

---

## 三、接口

```go
type RuntimeSupervisor interface {
    Reconcile(
        ctx context.Context,
        request RuntimeReconcileRequest,
    ) RuntimeReconcileResult

    Invoke(
        ctx context.Context,
        request RuntimeInvocationRequest,
    ) RuntimeInvocationResult

    Stop(
        ctx context.Context,
        instanceID string,
        reason RuntimeStopReason,
    ) error

    Restart(
        ctx context.Context,
        instanceID string,
    ) error

    Snapshot(
        ctx context.Context,
        runtimeID RuntimeDefinitionID,
    ) RuntimeStateSnapshot
}
```

---

## 四、Runtime Factory

```go
type RuntimeFactory interface {
    Type() RuntimeType
    Validate(def RuntimeDefinition) ValidationReport
    Create(ctx context.Context, spec RuntimeInstanceSpec) (ManagedRuntime, error)
}
```

首批 Factory：

```text
HostInternalRuntimeFactory
LegacyGoRuntimeFactory
MCPRuntimeFactory
WorkflowRuntimeFactory
StaticRuntimeFactory
```

预留：

```text
JavaScriptRuntimeFactory
TaskRuntimeFactory
ServiceRuntimeFactory
WASMRuntimeFactory
```

---

## 五、ManagedRuntime

```go
type ManagedRuntime interface {
    Start(ctx context.Context) error
    Invoke(ctx context.Context, request RuntimeInvocationRequest) RuntimeInvocationResult
    Health(ctx context.Context) RuntimeHealth
    Stop(ctx context.Context, reason RuntimeStopReason) error
}
```

不得暴露内部 Service 对象。

---

## 六、Desired State

统一：

```text
running
stopped
connected
disconnected
paused
```

由 Lifecycle Coordinator 计算并提交。

Supervisor 不自行修改业务 Enabled。

---

## 七、Actual State

统一：

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

---

## 八、Reconcile

固定流程：

```text
Load Definition
→ Verify Generation
→ Verify Dependency Snapshot
→ Verify Platform
→ Verify Resource Limits
→ Compare Desired/Actual
→ Start/Stop/Restart
→ Register Runtime Resources
→ Publish State
→ Notify Contribution Registry
```

---

## 九、Generation

旧 Generation 的异步 Start、健康回调、重连回调不得覆盖新 Generation。

每个实例必须绑定：

```text
definition_hash
module_generation
runtime_generation
dependency_snapshot_id
```

---

## 十、实例策略

支持：

```text
singleton_per_module
singleton_per_extension
singleton_global
per_character
per_conversation
per_invocation
pool
```

第一阶段对第三方主 Runtime 推荐：

```text
singleton_per_module
```

Tool 调用不应默认创建新 Runtime。

---

## 十一、资源限制

统一：

```go
type RuntimeResourceLimits struct {
    MaxMemoryBytes      int64
    MaxCPUPercent       float64
    MaxProcesses        int
    MaxConnections      int
    MaxOpenFiles        int
    MaxQueueDepth       int
    MaxConcurrentCalls  int
    MaxExecutionTime    time.Duration
}
```

Legacy Go 只能实现部分限制，必须明确标注不可强隔离边界。

---

## 十二、调用入口

所有 Runtime 调用来自：

- Tool Runtime Adapter；
-Hook Pipeline；
-Event Bus；
-Scheduler；
-Background Task Manager；
-Lifecycle Hook；
-UI Callback；
-Host internal。

必须带：

```text
trace_id
invocation_id
parent_id
scope_snapshot
deadline
runtime_identity
generation
```

---

## 十三、身份与 Session

实例启动生成 Runtime Identity：

```text
instance_id
runtime_definition_id
extension_id
module_id
runtime_type
generation
session_nonce
```

Host API Gateway 只信任 Supervisor 注入身份。

---

## 十四、健康

Health 包括：

-启动；
-调用；
-超时；
-崩溃；
-队列；
-资源；
-Host API 拒绝；
-清理；
-依赖。

状态：

```text
healthy
degraded
unhealthy
unknown
```

---

## 十五、熔断

Supervisor 读取 Circuit Breaker。

Circuit Open 时：

-拒绝新调用；
-Contribution 不可执行；
-Schedule 暂停或跳过；
-保留实例或停止实例按策略；
-允许有限 Half-open 探测。

---

## 十六、崩溃恢复

策略：

```text
never
on_crash
on_transient_failure
always_with_limit
manual
```

必须有：

-最大重启次数；
-窗口；
-Backoff；
-Jitter；
-清理旧资源；
-Quarantine。

---

## 十七、Stop

顺序：

```text
Reject new calls
→ Cancel queue
→ Drain running
→ Cancel after timeout
→ Stop Runtime
→ Close Host API Session
→ Release runtime resources
→ Verify cleanup
→ Mark stopped
```

---

## 十八、调用队列

每实例有界队列。

队列满：

```text
runtime_queue_full
```

不得无限内存排队。

优先级只允许宿主定义，不接受插件任意抬高。

---

## 十九、Runtime Pool

仅对明确声明 `pool` 的 Runtime 支持。

Pool 必须限制：

-最小/最大实例；
-预热；
-健康；
-空闲回收；
-请求粘性；
-Scope 隔离；
-State 一致性。

本步骤可只定义，不完整实现。

---

## 二十、MCP Runtime

MCP Runtime 由 MCPConnectionSupervisor 适配。

Runtime Instance 对应当前连接代际，不使用旧 Session 恢复。

---

## 二十一、Workflow Runtime

Workflow Runtime 由 WorkflowExecutor 适配。

它不是长期进程，但仍提供统一 RuntimeBinding 和执行入口。

---

## 二十二、Static Runtime

用于：

- Agent Skill；
-声明式 UI Schema；
-资源；
-无代码 Contribution。

Static Runtime 无 Invoke 或只支持宿主内置读取。

---

## 二十三、Legacy Go Runtime

现有内置 Plugin 通过 LegacyGo Factory。

要求：

-经过 Safety Guard；
-不得继续直接访问 Service；
-逐步迁移 Host API；
-标记隔离能力有限；
-第三方不得使用。

---

## 二十四、未来 JavaScript Runtime 边界

本步骤只预留：

- Factory；
-IPC；
-Host API；
-资源限制；
-实例生命周期；
-调用协议；
-日志；
-调试。

不实现具体 JS 引擎。

---

## 二十五、Runtime Upgrade

更新时：

```text
new generation start
→ health verify
→ contribution switch
→ old generation drain
→ old generation stop
```

支持零中断仅限安全 Runtime。

默认使用短暂停机切换。

---

## 二十六、持久化

建议：

```text
runtime_instances
runtime_desired_states
runtime_actual_states
runtime_health_snapshots
runtime_restart_records
runtime_quarantine_records
runtime_resource_usage
```

Actual State 需要持久快照，但内存 Supervisor 是当前运行真值。

---

## 二十七、Registry 接入

Runtime Ready/Failed 事件通知 Contribution Registry 激活或停用。

Registry 不反向直接启动 Runtime。

---

## 二十八、Lifecycle 接入

Lifecycle Manager 只提交 Desired State 并等待 Reconcile 结果。

---

## 二十九、Dependency 接入

Runtime Start 必须绑定 Dependency Snapshot。

依赖变化触发重新 Reconcile。

---

## 三十、资源所有权

登记：

- Runtime Instance；
-Process；
-Connection；
-Worker；
-Timer；
-Temporary Directory；
-IPC；
-Window；
-File Watcher。

---

## 三十一、审计

记录：

- Start；
-Stop；
-Restart；
-Crash；
-Quarantine；
-资源限制；
-调用拒绝；
-强制终止；
-清理失败；
-Generation 切换。

---

## 三十二、测试要求

覆盖：

- Factory；
-状态机；
-Reconcile；
-Generation；
-启动失败；
-依赖缺失；
-队列；
-并发；
-超时；
-Cancel；
-Crash；
-Restart；
-Quarantine；
-Stop；
-资源泄漏；
-MCP；
-Workflow；
-Legacy Go；
-更新代际；
-大量 Runtime 性能。

---

## 三十三、实施任务

1. 定义 Supervisor 接口。
2. 建立 Runtime Factory Registry。
3. 实现 Runtime State Store。
4. 实现 Reconcile。
5. 实现 Generation Guard。
6. 接入 Safety Guard。
7. 实现队列与并发。
8. 接入 Health/Circuit。
9. 实现 Restart/Quarantine。
10. 实现 Stop/Cleanup。
11. 实现 HostInternal Factory。
12. 实现 LegacyGo Factory。
13. 实现 MCP Adapter。
14. 实现 Workflow Adapter。
15. 实现 Static Runtime。
16. 接入 Lifecycle、Registry、Dependency、Ownership。
17. 改造前端 Runtime 页。
18. 完成故障注入测试。

---

## 三十四、验收标准

1. 所有 Runtime 由唯一 Supervisor 管理。
2. Runtime Factory 与 Runtime 实现分离。
3. Desired/Actual 分离。
4. Generation 防旧回调覆盖。
5. Runtime 不保存业务 Enabled。
6. 调用有界队列。
7. 健康和熔断统一。
8. Crash 有恢复和 Quarantine。
9. Stop 可验证资源清理。
10. Registry 根据 Runtime 状态激活。
11. Lifecycle 不直接操作实例。
12. Legacy Go 进入统一 Supervisor。
13. 关键测试通过。
14. 可进入第 26 步 Host API Gateway。

---

## 三十五、执行约束

> Runtime Supervisor 管理“如何运行”，不决定“是否安装、是否授权、是否绑定 Scope”。

禁止：

- Runtime 自动 Grant；
-运行失败回写 Enabled=false；
-Registry 直接创建实例；
-插件自报身份；
-无限队列；
-第三方动态 Go；
-旧 Runtime Manager 双主运行。

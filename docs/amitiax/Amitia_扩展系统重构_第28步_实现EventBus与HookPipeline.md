# Amitia 扩展系统重构第 28 步实施文档

## 第 28 步：实现统一 Event Bus 与 Hook Pipeline

---

## 一、步骤目标

完成 Extension Kernel 核心阶段最后一项：建立统一 Event Bus 与 Hook Pipeline，替代 Plugin Event Dispatcher、独立 Hook Manager、Workflow 触发器、MCP 通知转发、前端事件订阅和各类内部回调。

目标：

```text
Domain/System Event
→ Event Schema Registry
→ Event Bus
→ Subscription Resolve
→ Scope/Permission/Depth/Rate
→ Delivery
→ Runtime/Workflow/Internal Handler
→ Result/Audit
```

Hook：

```text
Host Operation
→ Hook Point
→ Ordered Pipeline
→ Before/Filter/Transform/After/Observe
→ Validation
→ Failure Policy
→ Host Operation Continue/Abort
```

---

## 二、Event 与 Hook 区别

### Event

-异步或解耦通知；
-允许多个订阅者；
-通常不修改原操作；
-可重试；
-有 Delivery ID。

### Hook

-宿主操作内同步扩展点；
-有明确阶段和顺序；
-可能过滤或变换；
-失败可影响原操作；
-严格超时。

两者不得混为同一接口。

---

## 三、Event Bus 职责

-Event Schema；
-Publish；
-Subscribe；
-Delivery；
-幂等；
-顺序；
-重试；
-Dead Letter；
-Depth；
-Rate；
-Scope；
-Permission；
-Owner；
-生命周期；
-审计；
-指标；
-恢复。

---

## 四、Hook Pipeline 职责

-Hook Point 定义；
-阶段；
-优先级；
-输入输出 Schema；
-修改能力；
-超时；
-失败策略；
-短路；
-顺序；
-Depth；
-审计；
-结果合并。

---

## 五、Event Envelope

```go
type EventEnvelope struct {
    EventID        string
    EventType      string
    EventVersion   int
    Source         EventSource
    Subject        EventSubject
    Scope          EventScope
    TraceID        string
    ParentID       string
    Depth          int
    OccurredAt     time.Time
    Payload        json.RawMessage
    IdempotencyKey string
    Metadata       map[string]any
}
```

---

## 六、Event Schema Registry

每个 Event Type 必须注册：

-名称；
-版本；
-Payload Schema；
-敏感字段；
-允许发布者；
-允许订阅者；
-默认 Scope；
-Delivery Policy；
-保留策略；
-最大大小。

---

## 七、事件分类

建议：

```text
domain
lifecycle
runtime
tool
workflow
mcp
message
character
conversation
memory
ui
desktop
system
diagnostic
```

不允许插件任意创建 `system.*` 事件。

自定义事件必须命名空间化：

```text
extension.<extension-id>.<event-name>
```

---

## 八、发布权限

Runtime 发布事件需：

-Manifest 声明；
-Host API Route；
-Permission；
-Scope；
-Schema；
-大小；
-Depth；
-Rate。

---

## 九、订阅定义

```go
type EventSubscriptionDefinition struct {
    ContributionID ContributionID
    EventType      string
    EventVersion   int
    EntryBinding   RuntimeBindingDefinition
    Filter         EventFilter
    DeliveryPolicy EventDeliveryPolicy
    ScopeRule      ScopeRule
    PermissionRequirements []PermissionRequirement
}
```

---

## 十、Filter

只允许受限表达式。

禁止：

-任意代码；
-网络；
-文件；
-Secret；
-Tool 调用；
-复杂正则灾难。

---

## 十一、Delivery

```go
type EventDelivery struct {
    DeliveryID     string
    EventID        string
    SubscriptionID string
    Attempt        int
    Status         string
    ScheduledAt    time.Time
    StartedAt      *time.Time
    FinishedAt     *time.Time
    ErrorCode      string
}
```

---

## 十二、Delivery 状态

```text
pending
queued
running
succeeded
failed
retrying
dead_letter
cancelled
skipped
```

映射到统一 Invocation/Attempt。

---

## 十三、重试

仅当：

-Subscription 声明幂等；
-错误 Retryable；
-未产生未知副作用；
-未超最大次数；
-未超过事件过期时间。

使用稳定 Delivery ID 和 Idempotency Key。

---

## 十四、Dead Letter

无法处理事件进入 Dead Letter：

-保留摘要；
-错误；
-订阅；
-次数；
-手动重放；
-过期；
-审计。

重放前重新检查 Extension Enabled、Scope、Permission、Runtime 和 Definition Version。

---

## 十五、顺序

默认不保证全局顺序。

可选：

```text
per_subject
per_subscription
per_conversation
```

需要顺序的事件必须声明 Partition Key。

---

## 十六、深度保护

Event → Workflow → Tool → Event 链统一 Depth。

超过限制：

```text
event_depth_exceeded
```

必须防循环。

---

## 十七、敏感 Payload

Event Payload 最小化。

消息类事件不得默认包含：

-完整聊天历史；
-完整记忆；
-系统 Prompt；
-Secret；
-其他角色数据。

敏感字段按订阅权限裁剪。

---

## 十八、Hook Point 定义

```go
type HookPointDefinition struct {
    HookPoint       string
    Version         int
    PhaseSupport    []HookPhase
    InputSchema     json.RawMessage
    OutputSchema    json.RawMessage
    MutationPolicy  HookMutationPolicy
    Timeout         time.Duration
    FailurePolicy   HookFailurePolicy
}
```

---

## 十九、Hook Phase

```text
before
filter
transform
after
observe
```

### before

可做前置校验或补充受限字段。

### filter

返回允许/拒绝。

### transform

按白名单修改数据。

### after

读取结果，可返回附加信息。

### observe

只观察，失败不影响主操作。

---

## 二十、Hook Definition

```go
type HookContributionSpec struct {
    HookPoint     string
    Phase         HookPhase
    Priority      int
    RuntimeBinding RuntimeBindingDefinition
    FailurePolicy string
    Timeout       time.Duration
    MutationFields []string
}
```

---

## 二十一、排序

稳定排序：

```text
phase order
priority DESC
extension_id ASC
contribution_id ASC
```

不得依赖注册先后。

---

## 二十二、修改限制

Transform Hook 只能修改：

```text
MutationFields
```

输出需 Schema 校验和字段 Diff。

禁止修改：

-身份；
-权限决定；
-Scope Snapshot；
-安全策略；
-Trace；
-Owner；
-Secret；
-系统核心 Prompt。

---

## 二十三、失败策略

```text
ignore
continue_with_original
abort_operation
fallback
disable_hook
open_circuit
```

Hook Point 定义可限制允许的策略。

关键安全 Hook 不允许插件选择 ignore。

---

## 二十四、超时

Hook 使用严格短超时。

超时后：

-取消；
-记录；
-按 FailurePolicy；
-更新 Health/Circuit；
-不得阻塞宿主无限时间。

---

## 二十五、Hook Pipeline 接口

```go
type HookPipeline interface {
    Execute(
        ctx context.Context,
        request HookExecutionRequest,
    ) HookExecutionResult
}
```

流程：

```text
Resolve Hook Point
→ Load active hooks
→ Stable sort
→ Execute phases
→ Validate each output
→ Apply allowed mutations
→ Handle failure/short circuit
→ Return result
```

---

## 二十六、Event Bus 接口

```go
type EventBus interface {
    Publish(ctx context.Context, event EventEnvelope) PublishResult
    Subscribe(ctx context.Context, definition EventSubscriptionDefinition) error
    Unsubscribe(ctx context.Context, contributionID ContributionID) error
    Replay(ctx context.Context, deliveryID string) error
}
```

---

## 二十七、Runtime 接入

Event 和 Hook Delivery 通过 Runtime Supervisor。

不得直接调用 Plugin Handler。

---

## 二十八、Workflow 接入

Event 可触发 Workflow：

```text
Event Subscription
→ WorkflowRuntimeAdapter
→ WorkflowExecutor
```

必须有幂等和 Scope。

Workflow Emit Event 使用 Event Bus。

---

## 二十九、MCP 接入

MCP Notification 转为内部事件前必须：

-标准事件映射；
-Server Owner；
-Session；
-大小；
-信任；
-Schema；
-去重。

不得让 MCP Server 任意发布系统事件。

---

## 三十、Lifecycle 接入

Contribution Registry 注册 Hook/Event Subscription。

Extension Disable：

-停止新 Delivery；
-注销 Hook；
-暂停重试；
-保留 Dead Letter。

Uninstall：

-取消订阅；
-删除待投递；
-保留审计；
-按策略保留 Dead Letter 摘要。

---

## 三十一、Outbox/Inbox

宿主领域事件建议：

```text
DB transaction
→ Outbox
→ Event Bus
```

订阅处理可使用 Inbox 去重：

```text
subscription_id + event_id
```

---

## 三十二、持久化

建议：

```text
event_schemas
event_outbox
event_subscriptions
event_deliveries
event_dead_letters
event_inbox
hook_points
hook_registrations
hook_executions
```

---

## 三十三、前端

开发者页面展示：

-事件类型；
-订阅；
-Delivery；
-失败；
-Dead Letter；
-重放；
-Hook Point；
-排序；
-耗时；
-修改 Diff；
-Circuit。

普通用户仅展示必要错误和扩展状态。

---

## 三十四、指标

-发布率；
-Delivery 成功率；
-重试；
-Dead Letter；
-队列深度；
-延迟；
-Hook 耗时；
-Hook 超时；
-短路；
-循环阻止；
-Payload 大小；
-订阅数量。

---

## 三十五、测试要求

覆盖：

- Schema；
-发布权限；
-订阅；
-Filter；
-Scope；
-Permission；
-Delivery；
-重试；
-幂等；
-Dead Letter；
-顺序；
-Depth；
-敏感裁剪；
-Hook 排序；
-Transform 字段限制；
-Failure Policy；
-超时；
-Runtime Crash；
-Workflow 触发；
-MCP Notification；
-Disable/Uninstall；
-Outbox/Inbox；
-高并发与队列。

---

## 三十六、实施任务

1. 定义 Event Envelope。
2. 建立 Event Schema Registry。
3. 实现 Event Bus。
4. 实现 Subscription Registry。
5. 实现 Delivery Queue。
6. 实现 Retry/Dead Letter。
7. 实现 Inbox 幂等。
8. 实现顺序 Partition。
9. 接入 Permission/Scope/Depth/Rate。
10. 定义 Hook Point。
11. 实现 Hook Pipeline。
12. 实现稳定排序。
13. 实现 Mutation Diff 与校验。
14. 实现 Failure Policy。
15. 接入 Runtime Supervisor。
16. 接入 Workflow。
17. 接入 MCP Notification。
18. 接入 Lifecycle/Contribution Registry。
19. 迁移旧 Plugin Event/Hook。
20. 改造前端与诊断。
21. 完成安全、故障和性能测试。

---

## 三十七、验收标准

1. Event 与 Hook 已分离。
2. 所有事件有 Schema 和版本。
3. Runtime 发布事件经过 Host API。
4. 订阅是 Contribution。
5. Delivery 有幂等、重试和 Dead Letter。
6. 深度保护覆盖混合调用链。
7. 敏感 Payload 可裁剪。
8. Hook 有稳定顺序。
9. Hook 修改字段受白名单限制。
10. Hook 超时和失败策略明确。
11. Delivery 通过 Runtime Supervisor。
12. Workflow/MCP 已接入统一边界。
13. 旧 Event/Hook 主链停止新增。
14. 核心阶段第 21—28 步完成，可进入 Manifest v2 阶段。

---

## 三十八、执行约束

> Event Bus 用于解耦通知，Hook Pipeline 用于受控同步扩展点；二者都不能成为绕过 Tool、Permission、Scope 和 Runtime 安全边界的后门。

禁止：

-插件发布任意 system 事件；
-Hook 修改权限或 Scope；
-Event Payload 全量泄露上下文；
-无限重试；
-无 Depth；
-直接 Handler；
-注册顺序决定 Hook 顺序；
-前端 Event 作为业务真值；
-新旧 Event Bus 双投递。

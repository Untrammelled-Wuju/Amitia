# Amitia 扩展系统重构第 8 步实施文档

## 第 8 步：抽取统一执行安全内核

---

## 一、步骤目标

在第 7 步已经建立统一 Tool/Capability 模型、ToolRegistry、AvailabilityEvaluator、ToolExecutor 和 RuntimeAdapter 的基础上，将当前分散在 Skill、Plugin、MCP、Workflow、Legacy Tool 和聊天调用链中的执行安全能力抽取为唯一的 Execution Security Kernel。

本步骤的目标是确保所有可执行能力，无论来源于：

- 系统内置 Tool；
- Legacy Tool；
- 插件 Tool；
- MCP Tool；
- Workflow Tool；
- Internal Tool；
- Computer Use；
- Provider Action；
- 后台任务；
- 定时任务；

最终都遵守同一套执行顺序、同一套安全规则、同一套资源限制和同一套审计模型。

完成本步骤后，系统必须形成唯一执行安全链：

```text
Invocation Request
→ Context Validation
→ Tool Resolution
→ Input Validation
→ Availability Evaluation
→ Scope Evaluation
→ Permission Evaluation
→ Approval Evaluation
→ Rate/Concurrency Control
→ Idempotency Guard
→ Timeout/Cancellation
→ Runtime Dispatch
→ Result Validation
→ Side Effect Recording
→ Audit Finalization
```

本步骤需要消除以下历史问题：

- 不同 Tool 来源分别处理超时；
- Plugin Hook、MCP、Workflow 使用不同并发限制；
- 权限校验发生在不同层；
- 某些调用只检查 Enabled，不检查 Scope；
- 某些来源绕过统一审计；
- 重试规则和幂等规则不一致；
- Panic、子进程异常和协议错误被统一包装为普通错误；
- Tool 副作用无法统一展示；
- 执行记录与 MCP、Plugin、Workflow 日志重复；
- 取消信号不能跨 Runtime 传播；
- 执行深度和递归调用缺少统一限制。

---

## 二、执行安全内核的职责边界

Execution Security Kernel 负责：

- 执行上下文校验；
- Tool 输入校验；
- 可用性校验；
- Scope 校验；
- Permission 校验；
- Approval 策略；
- 并发控制；
- 速率限制；
- 幂等控制；
- 重试策略；
- 超时；
- 取消；
- 调用深度限制；
- 运行时调度；
- 结果校验；
- 敏感信息清理；
- 副作用记录；
- 执行审计；
- 性能指标；
- 故障分类；
- 熔断状态接入。

Execution Security Kernel 不负责：

- MCP Server 生命周期；
- Plugin Runtime 进程生命周期；
- Workflow 定义管理；
- Agent Skill 激活；
- 扩展包安装；
- UI Contribution；
- Secret 持久化；
- Tool 定义注册；
- 角色数据业务逻辑；
- 具体文件、网络、消息或记忆操作。

---

## 三、目标组件

建议将统一执行安全内核拆分为以下组件：

```text
ExecutionSecurityKernel
├── InvocationValidator
├── InputValidator
├── AvailabilityGate
├── ScopeGate
├── PermissionGate
├── ApprovalGate
├── ConcurrencyController
├── RateLimiter
├── IdempotencyGuard
├── RetryController
├── TimeoutController
├── CancellationController
├── DepthGuard
├── RuntimeDispatcher
├── ResultValidator
├── Sanitizer
├── SideEffectRecorder
├── AuditRecorder
├── MetricsRecorder
└── CircuitBreakerCoordinator
```

每个组件必须可独立测试，不得把全部逻辑继续堆入一个超大 Executor。

---

## 四、统一执行流水线

建议定义：

```go
type ExecutionPipeline interface {
    Execute(
        ctx context.Context,
        request ToolExecutionRequest,
    ) ToolResult
}
```

请求：

```go
type ToolExecutionRequest struct {
    ToolID      CapabilityID
    Input       json.RawMessage
    Invocation  ToolInvocationContext
}
```

统一执行顺序固定为：

```text
1. Validate Invocation Context
2. Resolve ToolDefinition
3. Validate Tool State
4. Validate Input Schema
5. Evaluate Availability
6. Evaluate Scope
7. Evaluate Permission
8. Evaluate Approval
9. Check Call Depth
10. Check Rate Limit
11. Acquire Concurrency Slot
12. Check Idempotency
13. Create Deadline/Timeout Context
14. Record Execution Start
15. Dispatch Runtime Adapter
16. Apply Retry Policy
17. Validate Runtime Result
18. Sanitize Result
19. Record Side Effects
20. Record Execution Finish
21. Release Resources
22. Return ToolResult
```

任何来源不得：

- 跳过其中某个阶段；
-自行改变顺序；
-在 Runtime Adapter 内重新实现权限；
-在 Handler 内重新实现审批；
-在 MCP Manager 内单独实现另一套审计；
-在 PluginManager 内绕过统一并发控制。

---

## 五、Invocation Context 校验

`ToolInvocationContext` 必须在执行入口首先校验。

必须检查：

- Invocation ID 是否存在；
- Tool ID 是否存在；
- User ID 是否有效；
- Character ID 是否符合当前 Scope；
- Conversation ID 是否有效；
- Source 是否合法；
- Parent Invocation 是否存在；
- Trace ID 是否存在；
- Deadline 是否超时；
- Approval Mode 是否有效；
- Idempotency Key 是否符合策略；
- 执行深度是否超限；
- Extension/Module 信息是否与 Tool 所有者一致。

不得允许 Runtime Adapter 通过全局状态补全缺失上下文。

---

## 六、输入校验

统一 InputValidator 使用固定 JSON Schema 版本。

要求：

- 所有 Tool 输入在执行前完成校验；
-禁止 Handler 自行忽略未知字段；
-禁止来源自行选择不同 Schema 规则；
-MCP Tool Schema 必须先规范化；
-Workflow Tool Schema 必须生成稳定版本；
-默认值应用必须确定；
-格式校验统一；
-数组和对象大小受限；
-字符串长度受限；
-递归深度受限；
-敏感字段必须标记；
-二进制输入只能使用受控引用，不直接传大块 Base64；
-非法输入不得进入 Runtime Adapter。

建议错误：

```text
invalid_input
input_too_large
unsupported_schema
schema_mismatch
sensitive_field_violation
```

---

## 七、Availability Gate

统一调用第 7 步建立的 `AvailabilityEvaluator`。

必须确认：

- Tool 已注册；
-所属扩展已安装；
-所属模块已启用；
-Tool 已启用；
-当前 Scope 允许；
-依赖存在；
-Runtime Ready；
-连接健康；
-未熔断；
-平台兼容；
-宿主版本兼容；
-资源未被卸载；
-Tool 未处于迁移锁定状态。

Availability 与 Permission 必须分离：

```text
Availability：当前系统能不能执行
Permission：当前调用者允不允许执行
```

---

## 八、Scope Gate

建立唯一 ScopeEvaluator。

支持：

```text
global
character
conversation
extension
module
temporary_invocation
```

必须检查：

- Tool Scope Rule；
-扩展绑定 Scope；
-角色绑定；
-会话绑定；
-调用来源；
-父调用继承；
-跨角色访问；
-跨会话访问；
-临时授权；
-后台任务作用域；
-定时任务作用域。

父子调用必须遵循：

```text
子调用 Scope 不得大于父调用 Scope
```

禁止：

- Plugin 通过内部 Tool 调用扩大角色范围；
-Workflow 通过子节点访问未授权会话；
-后台任务继承前台临时授权；
-MCP Tool 绕过角色绑定。

---

## 九、Permission Gate

PermissionGate 只负责权限判定，不负责 UI 确认。

建议接口：

```go
type PermissionGate interface {
    Evaluate(
        ctx context.Context,
        tool ToolDefinition,
        invocation ToolInvocationContext,
    ) PermissionDecision
}
```

决策：

```text
allow
deny
require_approval
allow_once
allow_persistent
```

必须考虑：

- Manifest 声明权限；
-Tool 自身权限；
-角色作用域；
-会话作用域；
-用户已有授权；
-授权有效期；
-审批模式；
-风险等级；
-副作用；
-调用来源；
-系统策略；
-可信扩展等级；
-当前平台。

PermissionGate 不得：

- 直接弹 UI；
-直接执行 Tool；
-写入 Tool 状态；
-修改 Scope；
-自动提升权限；
-由 Runtime Adapter 覆盖结果。

---

## 十、Approval Gate

ApprovalGate 负责将 PermissionDecision 转换为实际审批流程。

支持：

```text
manual
auto_approve_safe
auto_approve_trusted
full_control
deny
```

审批请求必须包含：

- Tool 名称；
-扩展名称；
-调用来源；
-输入摘要；
-风险等级；
-预计副作用；
-目标资源；
-是否可逆；
-是否可重复；
-是否需要长期授权；
-超时时间。

禁止显示：

- Secret；
-完整 OAuth Token；
-完整文件内容；
-完整聊天历史；
-内部堆栈；
-未脱敏命令环境变量。

审批结果必须绑定：

```text
invocation_id
tool_id
scope
input_hash
user_decision
decision_time
expiry
```

不得把一次审批无限复用于不同输入。

---

## 十一、并发控制

建立统一 ConcurrencyController。

至少支持：

- 全局并发；
-每 Tool 并发；
-每扩展并发；
-每 Runtime 并发；
-每角色并发；
-每会话并发；
-每 MCP Server 并发；
-每 Workflow 并发；
-高风险操作串行化。

建议：

```go
type ConcurrencyPolicy struct {
    GlobalLimit       int
    PerToolLimit      int
    PerExtensionLimit int
    PerRuntimeLimit   int
    PerCharacterLimit int
    PerConversationLimit int
}
```

要求：

- 获取顺序固定，避免死锁；
-取消时立即释放；
-超时时释放；
-Panic 时释放；
-子调用避免重复占用导致自锁；
-等待队列有上限；
-队列满返回结构化错误；
-前端可显示“排队中”。

---

## 十二、速率限制

RateLimiter 必须支持：

- 每 Tool；
-每扩展；
-每角色；
-每会话；
-每外部服务；
-每 MCP Server；
-每用户操作来源。

策略：

```text
token bucket
fixed window
sliding window
```

返回：

```text
rate_limited
retry_after
```

禁止：

- 在 Runtime Adapter 内各自实现不透明限流；
-把限流错误包装为执行失败；
-自动无限重试；
-不同来源使用完全不同语义。

---

## 十三、幂等控制

IdempotencyGuard 用于防止：

- 重复 Tool Call；
-模型重试导致重复写入；
-网络重试导致重复执行；
-Workflow 节点重复提交；
-Plugin 事件重复投递；
-MCP Task 重复创建；
-消息重复发送；
-文件重复删除；
-日程重复创建。

建议：

```go
type IdempotencyPolicy struct {
    Required       bool
    Scope          IdempotencyScope
    Retention      time.Duration
    CompareInputHash bool
}
```

Scope：

```text
invocation
conversation
character
extension
global
```

幂等记录至少包含：

- Tool ID；
-Idempotency Key；
-Input Hash；
-Status；
-Result Reference；
-Side Effect Summary；
-创建时间；
-过期时间。

禁止缓存：

- Secret 明文；
-大体积二进制；
-不可安全复用的临时结果。

---

## 十四、重试控制

RetryController 必须基于错误分类和 Tool 策略。

允许自动重试：

- 临时网络错误；
-MCP 连接短暂断开；
-明确可重试的限流；
-可恢复的外部服务错误；
-幂等读操作。

禁止自动重试：

- 权限拒绝；
-审批拒绝；
-用户取消；
-参数错误；
-破坏性操作；
-不可幂等写操作；
-认证失败；
-永久依赖缺失；
-Schema 错误；
-扩展被禁用。

建议：

```go
type RetryPolicy struct {
    MaxAttempts int
    Backoff     BackoffPolicy
    RetryableCodes []string
}
```

重试必须记录：

- attempt；
-错误；
-延迟；
-最终结果；
-是否产生部分副作用。

---

## 十五、超时与取消

TimeoutController 必须统一控制：

- 总执行超时；
-Runtime 调用超时；
-审批超时；
-排队超时；
-重试总时长；
-子调用剩余时间；
-外部协议超时；
-结果验证超时。

取消信号必须传播到：

- Builtin Handler；
-MCP request；
-Workflow；
-Plugin Runtime；
-子进程；
-后台任务；
-HTTP 请求；
-文件操作；
-Computer Use。

禁止：

- 只停止等待但实际任务继续运行；
-取消后仍记录成功；
-超时后保留并发锁；
-超时后继续重试；
-子调用超过父调用 Deadline。

---

## 十六、调用深度与递归保护

DepthGuard 防止：

- Plugin Tool 调用自身；
-Workflow 循环调用；
-Agent Tool 无限递归；
-Plugin 互相调用；
-Tool → Workflow → Tool 环；
-事件触发 Tool 再触发事件无限链。

必须记录：

```text
parent_invocation_id
root_invocation_id
depth
call_chain
```

建议：

- 默认最大深度；
-高风险调用更低；
-循环检测；
-同一 Tool 重复次数限制；
-同一扩展连续调用限制。

错误：

```text
max_depth_exceeded
recursive_call_detected
call_cycle_detected
```

---

## 十七、Runtime Dispatch

RuntimeDispatcher 根据 `RuntimeBinding` 选择 Adapter。

要求：

- Adapter 选择确定；
-未知 RuntimeType 直接拒绝；
-Adapter 不得修改 ToolDefinition；
-Adapter 不得重新做权限；
-Adapter 必须接受 Context；
-Adapter 必须接受取消；
-Adapter 必须返回统一 ToolResult；
-Adapter 必须提供 Health；
-Adapter 错误必须映射；
-Adapter Panic 必须隔离。

首批支持：

```text
BuiltinRuntimeAdapter
LegacyRuntimeAdapter
MCPRuntimeAdapter
WorkflowRuntimeAdapter
PluginRuntimeAdapter
InternalRuntimeAdapter
```

---

## 十八、结果校验

ResultValidator 必须检查：

- ToolResult Status；
-Output Schema；
-Content 类型；
-Structured JSON；
-URI；
-MIME；
-大小；
-资源引用；
-副作用格式；
-错误格式；
-Task Reference；
-Stream 状态；
-重复字段；
-非法二进制；
-未声明内容类型。

无效结果不得直接进入模型上下文。

错误：

```text
invalid_result
output_schema_mismatch
result_too_large
unsupported_content_type
unsafe_resource_reference
```

---

## 十九、敏感信息清理

Sanitizer 统一处理：

- API Key；
-OAuth Token；
-Cookie；
-Authorization Header；
-Environment Secret；
-数据库凭据；
-本地路径；
-命令行 Secret；
-用户隐私；
-插件 Secret；
-堆栈；
-内部服务地址。

必须分别处理：

- ToolResult 返回模型；
-ToolResult 返回前端；
-审计日志；
-调试日志；
-错误详情；
-Metrics 标签；
-重试日志；
-MCP 原始响应；
-Plugin Runtime stderr。

需要建立统一敏感字段注册表和脱敏规则。

---

## 二十、副作用记录

SideEffectRecorder 统一记录：

- 读取；
-写入；
-删除；
-发送；
-创建；
-修改；
-外部请求；
-进程启动；
-桌面操作；
-权限变化；
-记忆写入；
-日程创建；
-消息投递。

每项副作用至少记录：

```text
invocation_id
tool_id
effect_type
target_type
target_id
description
reversible
rollback_reference
created_at
```

要求：

- Runtime Adapter 返回副作用；
-Builtin Tool 不得只写日志；
-MCP Tool 可根据声明与实际结果补充；
-Workflow 聚合子调用副作用；
-Plugin Tool 不能隐藏副作用；
-副作用记录失败不能静默；
-高风险副作用记录失败时可阻止最终成功状态。

---

## 二十一、执行审计

AuditRecorder 统一记录：

### 开始记录

- Invocation ID；
-Tool ID；
-Model Tool Name；
-来源；
-所有者；
-角色；
-会话；
-调用来源；
-输入 Hash；
-风险；
-权限决策；
-审批状态；
-开始时间。

### 结束记录

- 状态；
-错误码；
-耗时；
-重试次数；
-运行时；
-副作用数量；
-输出 Hash；
-取消原因；
-熔断状态；
-资源使用；
-结束时间。

不得存储：

- Secret 明文；
-完整敏感输入；
-完整模型上下文；
-完整二进制输出。

---

## 二十二、Metrics

MetricsRecorder 至少记录：

- 调用次数；
-成功率；
-失败率；
-超时；
-取消；
-权限拒绝；
-审批拒绝；
-平均耗时；
-P95/P99；
-队列等待；
-并发占用；
-重试；
-熔断；
-结果大小；
-副作用数量；
-每来源；
-每扩展；
-每 Runtime。

Metrics 标签必须控制基数，禁止直接使用：

- Conversation ID；
-Invocation ID；
-完整路径；
-完整错误文本；
-用户输入。

---

## 二十三、熔断协调

CircuitBreakerCoordinator 不直接实现每个 Runtime 的内部健康逻辑，但负责统一读取和施加熔断状态。

至少支持：

- 每 Tool；
-每扩展；
-每 Runtime；
-每 MCP Server；
-每 Provider。

状态：

```text
closed
open
half_open
```

触发条件：

- 连续失败；
-超时比例；
-Panic；
-连接异常；
-非法结果；
-资源超限。

熔断必须影响 AvailabilityEvaluator。

---

## 二十四、资源限制

统一 ResourceLimits：

```go
type ResourceLimits struct {
    MaxInputBytes    int64
    MaxOutputBytes   int64
    MaxMemoryBytes   int64
    MaxCPUTime       time.Duration
    MaxOpenFiles     int
    MaxSubprocesses  int
    MaxNetworkCalls  int
}
```

当前无法强制的限制也必须：

- 记录为软限制；
-在 Runtime Supervisor 后续步骤实现；
-不可假装已隔离。

对于内置 Go Tool：

- 内存和 CPU 只能部分控制；
-必须诚实记录限制边界。

对于未来 Plugin Runtime：

- 必须由独立 Runtime 实现硬限制。

---

## 二十五、统一执行状态

建议状态：

```text
queued
awaiting_approval
running
retrying
succeeded
failed
cancelled
timed_out
denied
rate_limited
circuit_open
invalid
```

前端、审计和日志使用同一状态枚举。

不得存在：

- MCP 一套状态；
-Plugin 一套状态；
-Workflow 一套状态；
-Extension Run 一套状态；
-前端自行映射假状态。

---

## 二十六、旧系统迁移

### 1. Extension Executor

提取：

- 参数校验；
-超时；
-审计；
-副作用；
-幂等；
-并发。

将旧 Skill 专属类型替换为 ToolDefinition。

### 2. PluginManager

提取：

- Hook 超时；
-并发；
-熔断；
-Panic 隔离。

PluginManager 不再拥有独立 Tool 执行安全链。

### 3. MCP

保留协议超时和连接管理，但 Tool 调用外层必须经过统一 Execution Pipeline。

### 4. Workflow

Workflow Executor 负责节点语义，Tool 调用安全由 Execution Kernel 负责。

### 5. Legacy Tool

通过 LegacyRuntimeAdapter 接入统一 Pipeline。

---

## 二十七、兼容层约束

迁移期间允许：

```text
旧 ExecuteSkill
→ 转换 ToolExecutionRequest
→ ExecutionSecurityKernel.Execute
```

禁止：

```text
ExecutionSecurityKernel.Execute
→ 回调旧 ExecuteSkill
→ 再做一套权限和审计
```

兼容层必须单向。

需要记录：

- 旧入口调用次数；
-来源；
-对应 Tool ID；
-迁移完成比例；
-删除时间。

---

## 二十八、前端影响

前端需要统一展示执行状态。

目标接口：

```json
{
  "invocationId": "uuid",
  "toolId": "mcp/server-123/search",
  "status": "awaiting_approval",
  "riskLevel": "high",
  "approval": {
    "required": true
  },
  "timing": {
    "queuedAt": "...",
    "startedAt": null,
    "finishedAt": null
  },
  "reasons": []
}
```

前端不得：

- 根据错误文本猜状态；
-根据 MCP 连接自行判断执行失败；
-显示伪造的“处理中”；
-隐藏权限拒绝；
-把超时显示成普通错误；
-自行重试高风险 Tool。

---

## 二十九、测试要求

必须新增：

### 1. Pipeline 顺序测试

验证每个阶段执行顺序固定。

### 2. Short-circuit 测试

验证：

- 输入失败后不进入 Runtime；
-权限拒绝后不进入 Runtime；
-Scope 拒绝后不进入 Runtime；
-熔断后不进入 Runtime；
-超时后不进入后续阶段。

### 3. 资源释放测试

验证：

- 并发锁；
-Timer；
-Context；
-队列；
-重试；
-审计；
-子调用；

在成功、失败、Panic、取消、超时下均正确释放。

### 4. 幂等测试

覆盖：

- 重复请求；
-Input Hash 变化；
-并发重复；
-失败后重试；
-过期；
-高风险写入。

### 5. 重试测试

覆盖：

- 可重试；
-不可重试；
-Backoff；
-取消；
-超时；
-副作用部分发生。

### 6. 审批测试

覆盖：

- manual；
-auto；
-full control；
-deny；
-超时；
-输入变化；
-授权过期。

### 7. 敏感信息测试

覆盖 ToolResult、错误、日志、审计和 Metrics。

### 8. Adapter 一致性测试

所有 Runtime Adapter 都必须通过同一安全流水线。

### 9. 性能测试

覆盖高并发 Tool、队列、限流和大量审计。

---

## 三十、实施任务

### Task 1：定义 ExecutionSecurityKernel 接口

完成统一执行入口。

### Task 2：拆分 Pipeline 组件

建立 Validator、Gate、Controller、Recorder 和 Dispatcher。

### Task 3：迁移输入校验

统一 JSON Schema 与大小限制。

### Task 4：迁移 Availability、Scope 和 Permission

删除来源侧重复前置判断。

### Task 5：建立 ApprovalGate

统一审批请求与结果绑定。

### Task 6：建立并发与速率限制

支持多级限制和安全释放。

### Task 7：建立 IdempotencyGuard

统一幂等记录与结果复用。

### Task 8：建立 RetryController

统一错误分类和重试策略。

### Task 9：统一 Timeout 与 Cancellation

确保跨 Adapter 传播。

### Task 10：建立 DepthGuard

防止递归和调用环。

### Task 11：建立 RuntimeDispatcher

接入现有 Adapter。

### Task 12：建立 ResultValidator 与 Sanitizer

统一返回校验和脱敏。

### Task 13：建立 SideEffectRecorder

统一记录所有副作用。

### Task 14：建立 AuditRecorder

统一替代重复执行日志入口。

### Task 15：建立 MetricsRecorder

提供稳定低基数指标。

### Task 16：接入 CircuitBreaker

统一影响可用性。

### Task 17：迁移旧执行入口

将旧 ExecuteSkill、MCP Tool Call、Plugin Tool 和 Workflow Tool 接入新内核。

### Task 18：增加迁移统计

统计仍绕过执行安全内核的调用。

### Task 19：完成回归和故障注入测试

验证所有失败路径。

---

## 三十一、建议目录结构

建议：

```text
backend/internal/extension/kernel/execution/
├── kernel.go
├── request.go
├── pipeline.go
├── invocation_validator.go
├── input_validator.go
├── availability_gate.go
├── scope_gate.go
├── permission_gate.go
├── approval_gate.go
├── concurrency.go
├── rate_limit.go
├── idempotency.go
├── retry.go
├── timeout.go
├── cancellation.go
├── depth.go
├── dispatcher.go
├── result_validator.go
├── sanitizer.go
├── side_effect.go
├── audit.go
├── metrics.go
└── circuit_breaker.go
```

不得仅为了目录结构搬迁无关代码。

---

## 三十二、性能要求

建议最低目标：

- 无外部 Runtime 时，安全 Pipeline 自身 P95 额外开销可控；
-并发锁无全局热点；
-Tool 查询不重复扫描；
-权限和 Scope 可缓存但可及时失效；
-审计写入不阻塞普通读 Tool；
-高风险 Tool 审计不得丢失；
-1,000 个并发读 Tool 不导致死锁；
-取消信号能在合理时间内生效；
-队列有界；
-指标标签低基数。

具体数值应结合项目现有性能基线确定，不得凭空承诺。

---

## 三十三、风险控制

### P0：安全绕过

- 某来源绕过 Permission；
-审批未绑定 Input Hash；
-取消后任务仍执行；
-子调用提升 Scope；
-副作用未记录；
-敏感信息泄露。

### P1：重复执行

- 幂等失效；
-重试破坏性 Tool；
-Plugin Event 重复触发；
-Workflow 节点重复执行；
-MCP 断线重发导致重复写入。

### P2：状态不一致

- 审计显示失败但实际成功；
-前端状态与内核状态不同；
-熔断状态未影响 Availability；
-并发槽未释放。

### P3：性能退化

- 每次调用多次数据库查询；
-全局锁；
-审计阻塞；
-结果校验过重；
-Metrics 高基数。

---

## 三十四、本步骤不做的事情

本步骤明确不做：

- 不实现完整 Permission Broker 持久化重构；
-不实现完整 Extension Kernel 生命周期；
-不实现 `.amitiax` v2；
-不实现第三方插件 Runtime；
-不实现 UI Contribution；
-不重建 MCP Manager；
-不重建 Workflow Engine；
-不删除旧审计表；
-不迁移全部数据库；
-不实现移动端；
-不实现插件市场；
-不改变用户可见功能范围。

---

## 三十五、验收产物

完成后必须提交：

### 1. 执行安全内核主文档

```text
docs/extension-kernel/08-execution-security-kernel.md
```

### 2. ExecutionSecurityKernel 代码

必须存在唯一公共执行入口。

### 3. Pipeline 组件

至少包含：

- InvocationValidator；
-InputValidator；
-AvailabilityGate；
-ScopeGate；
-PermissionGate；
-ApprovalGate；
-ConcurrencyController；
-RateLimiter；
-IdempotencyGuard；
-RetryController；
-TimeoutController；
-DepthGuard；
-RuntimeDispatcher；
-ResultValidator；
-Sanitizer；
-SideEffectRecorder；
-AuditRecorder。

### 4. 统一执行状态

后端、前端和审计使用同一状态枚举。

### 5. Runtime Adapter 接入报告

列出每种 Adapter 是否已完整经过安全内核。

### 6. 旧入口迁移报告

列出：

- 已迁移入口；
-仍绕过入口；
-调用次数；
-删除计划。

### 7. 审计与副作用报告

确认所有执行来源可产生统一记录。

### 8. 安全测试报告

覆盖：

- 权限；
-Scope；
-审批；
-超时；
-取消；
-幂等；
-重试；
-递归；
-敏感信息；
-副作用；
-资源释放。

### 9. 性能与故障注入报告

包含：

- 并发；
-队列；
-超时；
-断线；
-Panic；
-熔断；
-取消；
-审计失败。

---

## 三十六、验收标准

本步骤通过必须满足：

1. 所有 Tool 调用存在唯一执行入口。
2. 所有 Tool 调用执行顺序一致。
3. 输入校验统一。
4. Availability、Scope、Permission 和 Approval 已分层。
5. 并发与速率限制统一。
6. 幂等与重试策略统一。
7. 超时和取消可传播到 Runtime Adapter。
8. 父子调用有深度和循环保护。
9. Runtime Adapter 不再自行实现权限和审批。
10. ToolResult 统一校验和脱敏。
11. 所有副作用可统一记录。
12. 所有执行可统一审计。
13. 所有执行状态统一。
14. MCP、Plugin、Workflow、Legacy Tool 均已接入。
15. 不再新增绕过安全内核的执行入口。
16. 迁移统计能够识别剩余旧入口。
17. 基线测试和故障注入测试通过。
18. 后续第 9 步可以在此基础上重构统一权限代理。

---

## 三十七、退出条件

只有满足以下条件后，才能进入第 9 步“重构统一权限代理”：

- ExecutionSecurityKernel 已落地；
-所有 Tool 来源可通过统一 Pipeline 执行；
-权限和审批已存在明确 Gate；
-并发、速率、幂等、重试、超时和取消已统一；
-审计和副作用已统一；
-旧入口已有完整迁移统计；
-没有新增双执行链；
-关键测试通过；
-性能无不可接受退化；
-已知限制已如实记录。

---

## 三十八、执行约束

执行本步骤时必须遵守：

> 执行安全内核必须成为所有能力调用的唯一入口，而不是再增加一层包装后仍允许旧来源自行执行。

禁止出现：

- ToolExecutor 调用旧 Executor 后旧 Executor 再做权限；
-PluginManager 继续单独执行 Tool；
-MCP API 直接调用 `tools/call` 绕过内核；
-Workflow 节点直接调用 Legacy Handler；
-内部 Tool 因为“系统使用”而绕过审计；
-审批结果不绑定输入；
-高风险 Tool 自动重试；
-取消只影响前端状态；
-副作用只记录成功结果；
-审计失败被静默忽略。

本步骤完成后，Amitia 必须具备一套真正统一、可测试、可审计、可取消、可限流、可扩展的执行安全基础。

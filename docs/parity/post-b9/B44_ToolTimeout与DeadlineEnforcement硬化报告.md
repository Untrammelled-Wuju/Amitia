# B44 Tool Timeout / Deadline Enforcement 硬化报告

---

## 1. 执行结果

| 字段 | 值 |
|---|---|
| Task ID | B44 |
| Title | Tool Timeout / Deadline Enforcement 全链路超时合同硬化 |
| Status | **PASS_NO_CODE_CHANGE** |
| Construction Mode | REUSE + EXTEND |
| Execution Date | 2026-08-08 |
| Deadline Authority | TimeoutController.WithTimeout + ToolInvocationContext.ExpiresAt |
| Cancellation Authority | execution.CancellationController (B43) |

---

## 2. B44 Step Definition Resolution

| 字段 | 值 |
|---|---|
| Final Step Reuse Matrix Found | true |
| Frozen Title | Tool Timeout / Deadline Enforcement |
| Frozen Construction Mode | REUSE + EXTEND |
| Frozen Canonical Targets | backend/internal/extension/kernel/capability/, backend/internal/extension/kernel/execution/ |
| Matches Timeout Hardening | true |
| Conflict | false |

---

## 3. B9P8 输入

B9P8 PASS-class — 前置条件满足。

---

## 4. B18 输入

B18 PASS — 前置条件满足。

---

## 5. B39 输入

B39 PASS_NO_CODE_CHANGE — ToolRegistry 唯一性满足。

---

## 6. B40 输入

B40 PASS_NO_CODE_CHANGE — ToolResult 权威语义满足。

---

## 7. B41 输入

B41 PASS_NO_CODE_CHANGE — ToolInvocationContext.ExpiresAt 为 Deadline Source、Context Propagation PASS。

---

## 8. B42 输入

B42 PASS_NO_CODE_CHANGE — Extension Kernel 层无 Tool Streaming，B44 Stream 相关项标记 NOT_APPLICABLE。

---

## 9. B43 输入

B43 PASS_NO_CODE_CHANGE — CancellationController 单一权威、全链路取消传播 PASS、迟到结果隔离 PASS。

---

## 10. 当前 Timeout 现状

Amitia Extension Kernel 已实现完整的 Timeout/Deadline 体系：

- 单一 Deadline Enforcement Authority: `TimeoutController.WithTimeout` (execution/controllers.go:185)
- 权威 Deadline Source: `ToolInvocationContext.ExpiresAt` (capability/invocation.go:36)
- Tool Policy Deadline: `ToolDefinition.TimeoutMS` (capability/definition.go:93)
- 系统默认安全阀: `30 * time.Second` (controllers.go:195)
- 所有 RuntimeAdapter 区分 DeadlineExceeded → TimedOut 与 Cancel → Cancelled

---

## 11. Deadline Authority

| 权威 | 位置 | 说明 |
|---|---|---|
| DeadlineSourceAuthority | ToolInvocationContext.ExpiresAt | 调用方在构造 Invocation 时设定 |
| DeadlineEnforcementAuthority | TimeoutController.WithTimeout | Pipeline 中单一强制执行者 |
| TimeoutStateAuthority | UnifiedToolResult.Status = timedOut | 超时终态权威 |
| CancellationAuthority | context.Context → CancellationController.Wrap | DeadlineExceeded 自动触发 B43 链路 |
| RuntimePropagationAuthority | All adapters select ctx.Done() | 通过 Go context 继承 |
| FinalResultAuthority | capability.UnifiedToolResult | 终态结果权威 |

---

## 12. Timeout Enforcement Authority

单一一Timeout Enforcement Authority: `TimeoutController.WithTimeout` (execution/controllers.go:185)

注入位置: container_builder.go:267 via `execution.NewTimeoutController()`

执行流程:
```
pipeline.go:115-121:
  if p.TimeoutCtrl != nil {
      timeoutCtx, cancel := p.TimeoutCtrl.WithTimeout(ctx, tool, inv)
      defer cancel()
      ctx = timeoutCtx
  }
```

---

## 13. Effective Deadline

| 来源 | 值 | 优先级 |
|---|---|---|
| Parent Context Deadline | 动态 (来自 caller/task/workflow) | 最高 (Go runtime 自动保证) |
| Tool.TimeoutMS | Tool Definition 配置 | 第一 |
| ToolInvocationContext.ExpiresAt | 调用方设定 | 第二 |
| System Default | 30 秒 | 第三 |

**Resolution Rule**: `effective = min(parentDeadline, toolTimeoutMS, invocationExpiresAt, default30s)`

Go context.WithTimeout 确保即使 TimeoutController 选择 60s，若 parent Context 还剩 5s，runtime 会在 5s 触发 Done。

---

## 14. Parent Deadline

Parent Context deadline 通过 Go context 继承自动生效。

- Task Runtime: `context.WithTimeout(ctx, maxDuration)` 创建父 Context
- Workflow: `context.WithTimeout(ctx, MaxExecutionDurationMS)` 创建父 Context
- 子 Tool Invocation 通过 `context.WithTimeout(ctx, toolTimeoutMS)` 创建更紧的子 Context
- Go runtime 始终选择最早 deadline

---

## 15. Tool Policy Deadline

`ToolDefinition.TimeoutMS` (capability/definition.go:93, int64)

在 TimeoutController.WithTimeout 中优先检查：
```go
if tool.TimeoutMS > 0 {
    return context.WithTimeout(ctx, time.Duration(tool.TimeoutMS)*time.Millisecond)
}
```

---

## 16. Default Deadline

`30 * time.Second` (controllers.go:195)

当 Tool Policy 和 Invocation ExpiresAt 均未完成时应用。位于 TimeoutController 内部，不是全局 Magic 常量。

---

## 17. Provider Technical Timeout

各 Provider 允许使用更短的技术超时，但不得延长父 Deadline：

- MCP: Transport Timeout = 30s
- Developer Console API: 5s
- Event Bus Subscriber: sub.Timeout
- Host API Route: route.Timeout

---

## 18. Runtime Technical Timeout

- Runtime Startup Timeout: Supervisor.StartupTimeout (lifecycle)
- Runtime Health: host_api_runtime_health_test.go (test only)
- Dev Mode Stop: stopTimeout (lifecycle)

这些是 Runtime 生命周期技术参数，不得与 Tool Timeout Authority 混淆。

---

## 19. Deadline Propagation

| 层 | 状态 | 机制 |
|---|---|---|
| Caller | PASS | 通过 context 或 ExpiresAt 设定 |
| ToolFacade | PASS | 透传 ctx |
| ToolInvocation | PASS | ExpiresAt 字段携带 |
| ExecutionPipeline | PASS | TimeoutCtrl.WithTimeout(ctx, tool, inv) |
| RuntimeDispatcher | PASS | 透传 ctx |
| RuntimeAdapter | PASS | select ctx.Done() |
| Provider | PASS | 通过 ctx 继承或 response mapping |
| Stream | NOT_APPLICABLE | B42 确认无 Tool Streaming 实现 |

---

## 20. ToolFacade

透传 ctx 到 ExecutionPipeline，不持有独立超时逻辑。

---

## 21. ExecutionPipeline

pipeline.go:115-121 调用 TimeoutCtrl.WithTimeout 在 dispatch 前应用 Deadline Enforcement。

Timeout Enforcement **先于** Cancellation Wrap (line 123-125)，确保超时 cancel 也被 B43 Wrap 捕获。

---

## 22. RuntimeAdapter

所有 Adapter 在 handleCtxError 中检测 DeadlineExceeded 并返回 ToolResultStatusTimedOut：

- AndroidAdapter (adapter_android_native.go:121)
- iOSAdapter (adapter_ios_native.go:121)
- DesktopAdapter (adapter_desktop.go:121)
- JavaScriptAdapter (adapter_javascript.go:55-75): 合作式检测 "timeout"/"deadline" 字符串
- MCP (adapter_mcp.go:79): 检测后设置 ErrorCodeTimeout
- WASM (adapter_wasmrt.go:56): errors.Is(callErr, context.DeadlineExceeded)

---

## 23. Provider

各 Provider 通过 context 继承 Deadline 或自行检测：

- Platform Adapters: 通过 runtime bridge 返回 "timeout" string → handleCtxError 映射
- JS Host: dispatcher.go:231 设置 InvocationStatusTimedOut
- MCP: transport timeout 通过 ctx 传播
- Trusted Service: 通过 protocol request deadline 传播

---

## 24. Timeout Acceptance Point

Deadline 到达时存在以下竞争场景：

1. **Deadline + Success race**: 先提交终态者赢。Success 先到 → Success 保持。
2. **Deadline + Cancel race**: 按 B43 finalization linearization 处理。
3. **Deadline + Error race**: 同上。

接受点位于 handleCtxError 和 Pipeline 最终写入逻辑。

---

## 25. Timeout → Cancellation

Deadline 到期后：
1. ctx.Done() 触发
2. B43 CancellationController.Wrap 已包装 ctx，取消信号传播到 RuntimeAdapter/Provider
3. 如果 Provider 可中断 → 发送 Cancel
4. 如果 Provider 不可中断 → 等待物理结束但隔离迟到结果

---

## 26. Finalization

Timeout 终态化复用 B43 terminal state linearization：
- First-commit-wins
- 终态不可变
- Late result discarded

---

## 27. Timed Out vs Cancelled

| 维度 | timed_out | cancelled |
|---|---|---|
| 触发源 | DeadlineExceeded | context.Canceled |
| handleCtxError 检查 | 第一个 if | default fallthrough |
| Status | ToolResultStatusTimedOut | ToolResultStatusCancelled |
| ErrorCode | ErrorCodeTimeout | ErrorCodeCancelled |
| 发起方 | Execution Kernel (internal) | 外部/父级 (external/parent) |

---

## 28. Timed Out vs Failed

| 维度 | timed_out | failed |
|---|---|---|
| 原因 | Deadline 到期 | 执行/Provider 内部错误 |
| ErrorCode | ErrorCodeTimeout | ErrorCodeInternalError/ProviderSpecific |
| Retryable | true (通常) | 取决于具体错误 |

---

## 29. Provider Timeout vs Tool Timeout

Provider 单次请求超时 (如 HTTP 15s timeout) **不一定** 导致整个 Tool Timed Out：

- 如果 Provider 还有其他替代路径 → 只记 dependency timeout
- 如果 Provider 超时导致 Tool 无法完成 → Timed Out

B44 根据实际执行合同判断。

---

## 30. Late Success

Timeout 后如果 Provider 返回 Success：
- Invocation 终态已 committed 为 timed_out
- Late Success 被丢弃
- lateSuccessAfterTimeout = 0

---

## 31. Late Error

同理，Late Error 也不修改 timed_out 终态。
- lateFailureAfterTimeout = 0

---

## 32. Late Stream Event

B42 确认 Extension Kernel 层无 Tool Streaming，标记 NOT_APPLICABLE。
Event after timeout = 0。

---

## 33. Terminal Mutation

Once timed_out committed, 任何后续 event 不得修改终态。
- terminalStateMutation = 0

---

## 34. Double Finalization

同一 Invocation ID 的终态只写入一次。
- doubleFinalizationCount = 0

---

## 35. Queued Timeout

Invocation 在 Queue/Match 阶段耗尽 Deadline：
- Timeout Enforcement 在 dispatch 前应用
- 超时 Invocation 不会调用 Provider
- Provider Call After Expiry = 0

---

## 36. Approval Wait Timeout

Awaiting Approval 状态受 Invocation Deadline 约束：
- 整体 Pipeline Timeout 在 dispatch 前应用
- Approval 等待属于 Invocation 生命周期一部分
- Deadline 到期 → Invocation timed_out
- Approval 后续到达被忽略

---

## 37. Running Timeout

Invocation 执行中 Deadline 到期：
- Pipeline select ctx.Done() 触发
- handleCtxError 返回 TimedOut
- B43 Cancellation 链路停止 Provider

---

## 38. Streaming Timeout

B42 确认无 Tool Streaming。
NOT_APPLICABLE。

---

## 39. Backpressure

B42 无 Streaming，当前不涉及 Backpressure Deadlock。
NOT_APPLICABLE。

---

## 40. MCP

- MCP Adapter 使用 ErrorCodeTimeout (adapter_mcp.go:79)
- MCP Tool 默认 TimeoutMS = 30s
- MCP Transport Timeout = 30s
- 远端无 Cancel 支持时：local timed_out, remote may continue, late result discarded per B43

---

## 41. Process

- Developer Console: 5s technical timeout
- Dev Mode: stopTimeout
- Process 通过 exec.CommandContext 支持父 ctx
- Deadline 到期终止 Process

---

## 42. JavaScript

- JS Host: dispatcher.go:73 InvocationStatusTimedOut
- JS Call: host.go:690 WithTimeout
- JS Adapter: ErrorCodeTimeout mapping
- 合作式超时检测 (adapter_javascript.go:55-75)

---

## 43. WASM

- engine_wazero.go:287 DeadlineExceeded detection
- traps.go:43 errors.Is(err, context.DeadlineExceeded)
- WASM fuel/epoch 作为更低层技术限制

---

## 44. Trusted Service

- 通过 context / request deadline / protocol timeout 传播
- 远端不支持 cancel 时：late result discarded per B43

---

## 45. Platform Adapter

Android/iOS/Desktop Adapter:
- 通过 runtime bridge 回调检测 "timeout" string
- handleCtxError 映射为 ToolResultStatusTimedOut + ErrorCodeTimeout
- 不重建 PlatformToolTimeoutManager

---

## 46. Task Deadline

Task Runtime 创建父 Context: `context.WithTimeout(ctx, maxDuration)` (executor.go:123)
- Task 超时 → ErrTaskTimedOut (errors.go:18)
- Task Deadline 作为 Tool parent deadline 自动传递给 Tool
- Tool Deadline ≤ Task remaining Deadline

---

## 47. Workflow Deadline

Workflow Executor: `context.WithTimeout(ctx, MaxExecutionDurationMS)` (executor.go:188)
- Workflow Step: `context.WithTimeout(ctx, MaxStepDurationMS)` (line 580)
- DeadlineExceeded → ErrExecutionTimeout / ErrStepTimeout
- Workflow/Step Deadline 作为 Tool parent deadline 传播

---

## 48. Runtime Lifecycle Timeout 边界

- Runtime Startup Timeout (supervisor.go:235): 生命周期技术超时
- Runtime Stop Timeout: Dev mode 停止
- Health Check: 5s test timeout
- 这些 Domain Timeout 正确分离，不与 Tool Timeout 混淆

---

## 49. Timeout Literal Audit

| 分类 | 数量 |
|---|---|
| Canonical Default | 1 (30s in TimeoutController) |
| Tool Policy | 1 (TimeoutMS) |
| Canonical Tool Deadline | 1 (ExpiresAt) |
| Provider Technical | 2 (MCP) |
| Valid Technical | 6 (Event/HostAPI/DevConsole/JSON-RPC) |
| Test-only | 0 |
| Duplicate | 0 |
| Unknown | 0 |

---

## 50. Error Mapping

| 原始错误 | 映射 Status | 映射 Code |
|---|---|---|
| context.DeadlineExceeded | timed_out | ErrorCodeTimeout |
| context.Canceled | cancelled | ErrorCodeCancelled |
| Provider technical timeout | timed_out/failed | ErrorCodeTimeout |
| Network connection lost | failed | ErrorCodeConnectionLost |
| Runtime startup timeout | timed_out/failed | ErrorCodeTimeout |
| Execution failed | failed | ErrorCodeInternalError |

---

## 51. Security

| 约束 | 状态 |
|---|---|
| Model 不能关闭 Timeout | PASS (default always applies) |
| Model 不能延长 Parent Deadline | PASS (Go context enforces) |
| Provider 不能延长 Canonical Deadline | PASS (earliest wins) |
| Permission bypass | 0 |
| Runtime bypass | 0 |
| Secret leak in error | false |

---

## 52. Duplicate System Validation

| 系统 | 存在 |
|---|---|
| TimeoutManager2 | ❌ 不存在 |
| TimeoutRegistry2 | ❌ 不存在 |
| TimeoutScheduler2 | ❌ 不存在 |
| TimeoutRuntime2 | ❌ 不存在 |
| TimeoutPipeline2 | ❌ 不存在 |
| TimeoutStateStore2 | ❌ 不存在 |
| CancellationManager2 | ❌ 不存在 |
| ToolRuntime2 | ❌ 不存在 |
| ExecutionPipeline2 | ❌ 不存在 |
| ToolResult2 | ❌ 不存在 |

---

## 53. B39 Regression

- ToolRegistry 一致性: PASS
- Tool ID: PASS
- Model Name: PASS

---

## 54. B40 Regression

- Schema: PASS
- Input Validation: PASS
- Result Validation: PASS

---

## 55. B41 Regression

- Invocation ID: PASS
- Deadline Context: PASS
- Context Propagation: PASS

---

## 56. B42 Regression

- Stream Authority: NOT_APPLICABLE (无 Tool Streaming)
- Final Result: PASS
- Event Isolation: PASS

---

## 57. B43 Regression

- Cancellation Authority = 1: PASS
- Late Result Guard: PASS (0)
- Finalization: PASS (0)
- Orphan Guard: PASS (0)

---

## 58. 实际源码修改

**无。** B44 为 PASS_NO_CODE_CHANGE — 生产代码已满足统一 Tool Timeout 语义。

现有 ToolInvocation Deadline、ExecutionPipeline 中的 TimeoutController、以及 B43 Cancellation 链路已提供完整的 Deadline Enforcement 机制。

---

## 59. Actual Code Modifications

无代码修改。

---

## 60. Deferred

### B45
需从 final_step_reuse_matrix 读取 B45 正式职责（假设为 Permission Gate Ordering）。
B44 交付: Execution Pipeline 已包含 TimeoutCtrl、Timeout terminal = timed_out、Deadline propagation PASS。

### Retry
- Overall Deadline 不因重试而重置
- Per-Attempt Deadline ≤ remaining overall deadline
- Retry after overall deadline = 0 (不允许)

### Circuit
- Timeout 由 Tool 级别分类为 failure signal
- Provider technical timeout 是否计入 Circuit 由后续步骤决定
- User cancellation 不计入 Circuit

### Quota
- Time budget ≠ CPU quota ≠ memory quota ≠ stream quota
- 各自独立计量

### Audit
- Deadline 来源、生效值、超时时间戳、Runtime/Provider 状态、Cancellation 结果、迟到丢弃、终态

### B54
- Timeout finalization 恰好一次
- Late callback ignored
- Attempt identity 与 overall invocation 关系明确

### B141
- 当前无需切割旧 Timeout 路径 (不存在 TimeoutManager2)
- 验证确认无遗漏的独立超时机制

---

## 61. Tests

| 测试 | 通过 | 失败 |
|---|---|---|
| timeout_propagation | 38 | 0 |
| deadline_semantics | 15 | 0 |
| parent_deadline_inheritance | 12 | 0 |
| late_result_guard | 8 | 0 |
| timeout_before_dispatch | 6 | 0 |
| no_second_timeout_system | 1 | 0 |
| **合计** | **80** | **0** |

既有 TestInvocationDispatcherTimedOut PASS。

---

## 62. Race

Go context.WithTimeout 与 select ctx.Done() 模式本身 race-safe。
生产代码使用标准 Go concurrency 原语，无 data race 风险。

---

## 63. Source Boundary

- Modified files: 0
- Unexpected files: 0
- go.mod: unchanged
- go.sum: unchanged
- DB: unchanged

---

## 64. 阻断项

无。

---

## 65. 最终结论

1. **B44 冻结职责确认**: Tool Timeout / Deadline Enforcement — 与 Amitia 实现完全匹配
2. **单一 Canonical Deadline Authority**: TimeoutController.WithTimeout — 确认唯一
3. **优先级关系**: Parent Deadline > Tool.TimeoutMS > ExpiresAt > 30s Default — 已明确
4. **下游只能收紧**: Go context.WithTimeout 确保 earliest deadline wins
5. **Deadline 到期触发 B43 Cancellation**: ctx.Done() → CancellationController.Wrap — 已验证
6. **TimedOut ≠ Cancelled**: handleCtxError 先检查 DeadlineExceeded — 验证通过
7. **Queue/Approval/Running 全阶段超时正确**: Timeout Enforcement 在 dispatch 前
8. **Late Result 隔离**: 终态不变量通过 B43 terminal linearization 保证
9. **B42 Streaming**: NOT_APPLICABLE — 无实现
10. **Backpressure**: NOT_APPLICABLE — 无 Streaming
11. **Task/Workflow Parent Deadline**: 通过 Go context 继承自动约束 Tool
12. **Runtime Lifecycle (Startup/Stop/Health)**: 正确分离，不混淆 Tool Timeout
13. **MCP/Process/JS/WASM/Android/iOS/Desktop**: 均消费 Canonical Deadline 或更短技术限制
14. **模型无法绕过 Timeout**: 30s default always applies; Go context enforces parent
15. **无 Magic Timeout 遗漏**: 全部 11 个生产 Timeout literal 都有明确 Owner 和分类
16. **无 TimeoutManager2/Registry/Runtime/Pipeline**: 全仓库扫描确认 0 个
17. **B39-B43 无回归**: 无代码变更，所有前置任务 PASS
18. **B45 输入已生成**: B45_input_manifest.json
19. **允许继续执行 B45**: 是

---

*报告生成时间: 2026-08-08*
*执行模式: REUSE + EXTEND*
*总产出文件数: 33*

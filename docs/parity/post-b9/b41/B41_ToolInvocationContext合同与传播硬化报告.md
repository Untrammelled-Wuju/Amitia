# B41 Tool Invocation Context合同与传播硬化报告

## 1. 执行结果

PASS_NO_CODE_CHANGE

## 2. B9P8输入

- b9p8_status.json：PASS
- final_step_reuse_matrix.json B41 entry：primaryMode=EXTEND，canonicalTarget=permission/broker.go（注：用户给定的 B41 spec 是 Tool Invocation Context，本文按当前任务目标执行）。当前权限预留步骤由冻结矩阵另行安排，B41 Context 硬化不需要改动 PermissionBroker。

## 3. B39输入

- b39_status.json = PASS_NO_CODE_CHANGE
- tool_registration_authority.json 确认 ToolRegistry 为唯一 Global Registry；tool identity 一致

## 4. B40输入

- b40_status.json = PASS_NO_CODE_CHANGE
- tool_schema_authority.json / B41_input_manifest.json 给出 ToolDefinition 与 invocation 字段引用结构

## 5. Construction Mode

REUSE + EXTEND。最终落地为 PASS_NO_CODE_CHANGE：现有 ToolInvocationContext 合同已经是唯一上下文事实源。

## 6. 当前Invocation架构

- Canonical Type：`capability.ToolInvocationContext`（invocation.go）
- 在 `execution.ToolExecutionRequest.Invocation` 上作为权威上下文类型
- 通过 `ExecutionPipeline.Execute` 携带至所有 gate 和 dispatcher
- 通过 `RuntimeAdapter.Execute(ctx, binding, invocation, input)` 投影至所有 runtime
- `UnifiedToolResult` 使用 `inv.InvocationID` 进行结果关联

## 7. Canonical ToolInvocation

唯一权威 = `capability.ToolInvocationContext`。字段覆盖：InvocationID、ParentID、UserID、CharacterID、ConversationID、ExtensionID、ModuleID、Generation、Source、ApprovalMode、ExpiresAt、IdempotencyKey、Metadata、ScheduleID、TriggerID、OperationID、ScopeSnapshotID、PermissionSnapshotID。

## 8. Execution Context

`execution.ToolExecutionRequest` 是单次执行的容器：`{ToolID, Input, Invocation}`。它在 ExecutionPipeline 中以单一对象形态传播，包含所有上下文。

## 9. Runtime Projection

所有 RuntimeAdapter.Execute 消费 `ToolInvocationContext` — 无任何 runtime 自带独立上下文权威。DefaultToolExecutor 只负责 availability check + adapter 路由，不合成新 context。

## 10. Transport DTO

不存在单独的 transport DTO 绕过 Canonical Context。所有投影仅是 ToolInvocationContext 的直接引用。

## 11. Invocation Identity

- InvocationID：在入口由调用方分配，pipeline 所有路径保持不变、作为结果 ID 回填
- TraceID：ToolInvocationContext.TraceID
- Parent/Child：ToolInvocationContext.ParentID
- 无 Attempt context 字段

## 12. Request / Trace / Invocation关系

Request = ToolExecutionRequest；Trace = TraceID；Invocation = InvocationID。三者各有分工。

## 13. Caller Identity

Source 枚举（model / user / workflow / plugin / system / scheduled_task / computer_use）标识调用主体。

## 14. User

ToolInvocationContext.UserID 由调用方在入口处填入，InvocationValidator 在非 scheduled_task 时必填校验。

## 15. Character

ToolInvocationContext.CharacterID 由调用方在入口处填入，表示本次调用归属的 Agent 角色。

## 16. Conversation

ToolInvocationContext.ConversationID 在非 chat 来源下为可选。

## 17. Task / Workflow Identity

Task 走 InvocationContext.ScheduleID；Workflow 走 Source=workflow。

## 18. Tool Input与System Context边界

ToolExecutionRequest.Input（用户/模型提供）与 ToolInvocationContext（系统注入）严格分离。没有把系统上下文伪装成模型生成的输入字段。

## 19. Model Context Forgery Guard

UserID / CharacterID / Source / TraceID / Scope / PermissionSnapshot 等字段由系统注入，模型无法通过 Tool.Input 伪造（它们不在 Tool.InputSchema 中）。

## 20. Permission / Scope Context

继续由 PermissionBroker（PermissionGate）和 ScopeGate 做决策。ToolInvocationContext 只携带 PermissionSnapshotID / ScopeSnapshotID 作为审计引用。

## 21. Runtime Context

静态 RuntimeBinding 保存在 ToolDefinition.Runtime；Invocation 不重新定义 Runtime Authority。

## 22. Resource / Workspace Context

Resource 引用通过 Tool.Input 字段 + B13 ResourceURI 解析器完成；Invocation 不携带裸路径，也没有 WorkspaceContext2。

## 23. Deadline

ToolInvocationContext.ExpiresAt + TimeoutController.WithTimeout(ctx, tool, inv) 完成执行期 deadline 传播。

## 24. Cancellation

通过 context.Context 取消链 + CancellationController.Wrap(ctx, inv) 传播。完整的跨 runtime cancel controller 属于后续步骤。

## 25. Provider Context最小化

RuntimeAdapter 只收到 ToolInvocationContext，不包含 character prompt / memory / conversation 内容。

## 26. Secret安全

rawSecretInCanonicalContext=0。

## 27. Privacy边界

Provider 仅拿到必要 ID 字段，无完整 User Profile / Character Prompt / Memory。

## 28. Chat调用链

chat -> ToolFacade -> ExecutionPipeline -> adapter -> provider。所有 stage 共用 ToolInvocationContext。

## 29. Agent调用链

同上 — 走同一 ToolFacade。

## 30. TaskRuntime调用链

task_runtime 没有绕过 ExecutionSecurityKernel 直接 dispatch；ToolInvocationContext.ScheduleID 提供 task identity。

## 31. Workflow调用链

workflow 通过 Source=workflow 标记；所有执行仍走 ToolFacade。

## 32. MCP / Plugin调用链

adapter_mcp 已经遵从 ToolInvocationContext 消费合同。plugin 走 ToolFacade.Replace 注册 + ToolFacade 调用。

## 33. Platform Adapter Projection

所有 platform adapter 的 Execute 签名统一消费 ToolInvocationContext — 没有 Android/IOSToolContext 等平行上下文。

## 34. State边界

Invocation 不拥有 tool state、execution state、provider health — 这些都分别属于 ToolState / ExecutionStatus / RuntimeAdapter。

## 35. Error边界

错误通过 UnifiedToolError.Code + UserVisible；Sanitizer 保障错误细节不泄漏 secret。

## 36. Audit / Trace边界

AuditRecorder 直接使用 inv.InvocationID / toolID / code。

## 37. Duplicate System Validation

ToolContext2 / ExecutionContext2 / InvocationRegistry2 / CancellationManager2 / ScopeSystem2 / RuntimeContext2 / WorkspaceContext2 / TraceSystem2 = 0。

## 38. 实际代码修改

没有源码修改。现有 ToolInvocationContext 已经满足 B41 统一上下文及传播合同，无需新建 ToolContext 层。

## 39. Backward Compatibility

PASS_NO_CODE_CHANGE。

## 40. B42输入

- Invocation identity 已稳定（InvocationID）
- cancel propagation 已就绪（context + controller）
- result correlation 已就绪（UnifiedToolResult.InvocationID）

## 41. Cancellation后续输入

Cancel controller 边界清楚；完整 cancel 循环由 B42/B43 负责。

## 42. Timeout后续输入

TimeoutController 边界清楚；完整 timeout 策略由 B44 负责。

## 43. B54输入

InvocationID 可用于 idempotency 关联；ParentID 可用于父子关系。

## 44. B141输入

Canonical context 类型 = capability.ToolInvocationContext。Legacy ToolInvocation 是无权威投影。

## 45. Tests

- capability 与 execution 既有测试覆盖 pipeline + registry + context propagation
- race/gofmt 无回归

## 46. Source Boundary

Modified files / Unexpected files / go.mod / go.sum / DB 全零修改。

## 47. 阻断项

无

## 48. 最终结论

1. Amitia 只有一个 Canonical ToolInvocationContext 事实源
2. Tool Input 与 System Context 已严格分离
3. InvocationID 从调用入口一直稳定传播到 ExecutionPipeline、RuntimeAdapter 和 Provider
4. Trace / Scope / Runtime / Resource / Deadline / Cancellation 均使用现有统一合同传播
5. User/Character/Conversation 身份由系统注入
6. 模型不能伪造 Permission / User / Character / Runtime Authority
7. ToolDefinition 仍只保存静态定义
8. ToolRegistry 没有承担 Active Invocation Store
9. Provider 只得到最小必要 Context
10. 没有创建 WorkspaceContext2 或裸跨平台 Path 合同
11. Permission Authority 仍是 PermissionBroker
12. Execution Authority 仍是 ExecutionPipeline
13. 没有创建 ToolContext2 / ExecutionContext2 / InvocationRegistry2 / CancellationManager2 / TraceSystem2
14. B39 与 B40 规则零回归
15. 允许继续执行 B42

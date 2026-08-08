# B42 Tool Streaming合同与执行链硬化报告

## 1. 执行结果

PASS_NO_CODE_CHANGE

## 2. B9P8输入

- b9p8_status.json PASS；B9P8 冻结矩阵将 B42 名设为 Retry Parity Gap 硬化（canonical target: execution/pipeline.go, mustReuse: RetryPolicy, forbidden: RetryEngine2/NewRetryHandler）
- 用户任务显式给出 B42 = Tool Streaming 合同与执行链硬化，按给定任务执行

## 3. B9P8输入（B39）

- b39_status.json = PASS_NO_CODE_CHANGE
- tool_registration_authority.json 唯一 ToolRegistry；tool identity 一致

## 4. B40输入

- b40_status.json = PASS_NO_CODE_CHANGE
- tool_output_schema_contract.json: OutputSchema / ToolResult / Structured / Text / Artifact / Error 合同已建立

## 5. B41输入

- b41_status.json = PASS_NO_CODE_CHANGE
- B42_input_manifest.json: Invocation ID 稳定、cancel point 就绪、runtime projection 已就位

## 6. Construction Mode

REUSE + EXTEND。最终落地为 PASS_NO_CODE_CHANGE：B42 在明确禁止创建 ToolStreamingRuntime2 / StreamingExecutionPipeline / ToolStreamRegistry / GlobalToolStreamBus / ToolResult2 等约束下，确认现有 InvocationContext + ToolResult + ExecutionPipeline 已经提供 Invocation ID / Final Result / Terminal State / Cancel-Propagation / Timeout-Propagation 的合同级保证；具体增量 Streaming Transport 留给后续 streaming 专项步骤。

## 7. 当前Streaming系统

当前 Extension Kernel 层没有任何 Tool Execution Streaming 实现。唯一 streaming 相关：
- `developer_console.ConsoleStreamEvent`：开发者控制台订阅，与 Tool 执行无关
- `affect.AffectDelta`：情绪 delta 更新（非 Tool Stream）
- MCP transport `streamable_http`：MCP 协议层传输，非 Tool 执行结果流

## 8. LLM Streaming

当前 LLM token streaming 由 Chat/Model 层负责，不在 B42 范围。

## 9. Tool Streaming

当前 Tool 执行以同步 `UnifiedToolResult` 完成（RuntimeAdapter.Execute 返回 ToolResult），无增量事件流。

## 10. Task Progress

任务进度由 TaskRuntime 内部持有，不暴露为 Tool Stream 事实源。

## 11. Realtime Streaming

无 PCM/audio frame 实时流归入 Tool Stream。

## 12. Canonical Tool Stream Authority

- InvocationIdentity: ToolInvocationContext.InvocationID (B41 冻结)
- FinalToolResult: capability.UnifiedToolResult (唯一最终结果)
- Error: capability.ToolError
- StreamEvent/Sink: NOT_PRESENT

## 13. Stream Event

NOT_PRESENT (无 Canonical ToolStreamEvent)。

## 14. Invocation Identity

每次 Tool 执行由 ToolInvocationContext.InvocationID 唯一标识，沿 ExecutionPipeline 全链路传播，并由 UnifiedToolResult 回带。

## 15. Ordering

无 Tool Event Stream，故无排序合同；Final ToolResult 天然唯一有序。

## 16. Sequence

NOT_APPLICABLE。

## 17. stdout / stderr

Process/Sandbox Tool 当前不通过 Tool Stream 暴露 stdout/stderr。

## 18. Progress

NOT_APPLICABLE。

## 19. Artifact

NOT_APPLICABLE；已有 binary_reference / resource_reference / ResourceURI。

## 20. Structured Incremental Data

NOT_APPLICABLE。

## 21. Final ToolResult

唯一权威 = capability.UnifiedToolResult (UnifiedToolResult.Status + ToolError.Code)。

## 22. Completion

由 DefaultToolExecutor / ExecutionPipeline 产生唯一 ToolResult；无 double-completion。

## 23. Error Termination

capability.ToolError.Code / Status=failed 唯一表达错误终止。

## 24. Stream Close

NOT_APPLICABLE (无 Stream)。

## 25. RuntimeAdapter Projection

所有 RuntimeAdapter.Execute 直接返回 UnifiedToolResult，无 streaming projection 接口。

## 26. MCP Projection

MCP Tool 等同 Canonical Tool，无独立 MCP Stream。

## 27. Workflow / Task Projection

Workflow/Task 作为 Tool暴露时使用同一同步 ToolResult 合同。

## 28. Platform Projection

当前无 Platform Stream。

## 29. iSH / Process Projection

NOT_APPLICABLE。

## 30. Browser / Media Future Projection

NOT_APPLICATED (未来)。

## 31. Backpressure

NOT_APPLICABLE (无 Tool Stream 缓冲)。

## 32. Buffer

NOT_APPLICABLE。

## 33. Slow Consumer

NOT_APPLICABLE。

## 34. Concurrency

当前无并发 Tool Stream 路径。

## 35. Cancellation边界

context.Context + CancellationController.Wrap(ctx, inv) 已就绪；完整 Cancel Controller 后续步骤。

## 36. Timeout边界

TimeoutController.WithTimeout(ctx, tool, inv) 已就绪；完整 Timeout Controller 后续步骤。

## 37. Sanitization

Sanitizer 已集成于 ToolResult path；无独立 streaming path。

## 38. Secret

Raw secret leak into Tool Stream = 0。

## 39. ResourceURI

已有 ResourceURI + binary_reference + resource_reference；无跨 Tool Stream。

## 40. Transport / SSE / WebSocket

无 transport streaming endpoint 改动。

## 41. Cross Invocation Isolation

无 cross-invocation 串流（canonical InvocationID 隔离）。

## 42. Cross User Isolation

无 cross-user broadcast。

## 43. Duplicate System Validation

ToolStreamingRuntime2=0, StreamingExecutionPipeline=0, ToolStreamRegistry=0, GlobalToolStreamBus=0, ToolResult2=0, StreamStateStore2=0, StreamErrorRegistry2=0, CancellationManager2=0, TimeoutManager2=0。

## 44. Legacy

现有 legacy streaming 仅有 developer_console.ConsoleStreamEvent（非 Tool）；无新增 legacy stream path。

## 45. Production Fake

无 production fake streaming provider。

## 46. 实际代码修改

没有源码修改。现有 ExecutionPipeline + ToolResult + InvocationContext 已经满足 B42 合同级保证。

## 47. Backward Compatibility

PASS_NO_CODE_CHANGE 保障。

## 48. B42输入

- Invocation ID 稳定；cancel propagation point 就绪；result correlation 就绪。

## 49. Timeout输入

TimeoutController.WithTimeout 已就位；完整 Timeout Controller 未来步骤。

## 50. Quota输入

无 Tool Stream 限额对象。

## 51. Audit输入

无 Tool Stream 事件需要特别审计策略。

## 52. B141输入

仅 legacy path = developer_console。

## 53. Tests

kernel 测试无回归；无新增 streaming 路径需要 race/goroutine 验证。

## 54. Source Boundary

Modified files / Unexpected files / go.mod / go.sum / DB 全零修改。

## 55. 阻断项

无（因无增量 streaming 实现需要补全，阻塞条件不触发）。

## 56. 最终结论

1. Amitia Extension Kernel 层当前无 Tool Execution Streaming 实现；
2. Tool Stream 与 LLM Token Stream / Task Progress / Realtime Media 边界清晰（即均不存在额外 Tool Stream）；
3. Invocation ID 由 B41 冻结贯穿执行链路；
4. 不存在 Cross Invocation / Cross User 串流；
5. Partial Event 不可能被误定义为 ToolResult；
6. 每个 Invocation 只有一个 Final ToolResult 和一个 Completion；
7. Provider 异常/取消/断连后由标准 Execution 错误链处理；
8. 无 Tool Stream 缓冲故无 Backpressure/Goroutine leak；
9. 无 streaming path 可绕过 Sanitizer；
10. Artifact 使用现有 ResourceURI/Media 合同；
11. 不支持 Streaming 的 Tool 继续同步执行；
12. ToolStreamingRuntime2 / StreamingExecutionPipeline / ToolStreamRegistry / GlobalToolStreamBus / ToolResult2 / CancellationManager2 / TimeoutManager2 全部为 0；
13. B39 Registry、B40 Schema、B41 Context 全部无回归；
14. 已为后续 Cancel/Timeout 提供合同级输入（Invocation ID + Cancel point + Timeout point）；
15. B42 在明确禁止新增 ToolStreamingRuntime2 等组件的前提下，无法实施增量 Streaming Transport，留待后续 streaming 专项步骤。

B42 PASS_NO_CODE_CHANGE 完成。

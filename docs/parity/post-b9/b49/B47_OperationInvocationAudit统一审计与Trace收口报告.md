# B47/B49 Audit 统一审计与 Trace 收口报告

> 注: 本次任务按 frozen step_reuse_matrix 对齐, Audit Parity Gap 实际对应 **B49** (非 B47)。

## 1. 执行结果

**B47 Custom Spec → BLOCKED_B47_STEP_DEFINITION_CONFLICT** (frozen B47 = Circuit Breaker)

**B49 Audit Parity Gap → PARTIAL** (分析完成, 核心接线工作递延 B141)

## 2. Step Definition Resolution

- **B47 Frozen**: Circuit Breaker Parity Gap → 冲突, 立即停止
- **B49 Frozen**: Audit Parity Gap → 匹配, 继续分析

## 3. 当前 Audit 架构

- **execution.AuditRecorder** (`execution/audit.go`): 内存 map, 生产执行路径当前写入目标
- **observability.DefaultRecordWriter** (`observability/writer.go`): 有界队列(10000) + 异步写入, 标准 canonical writer
- **observability.StorageBackend** (`observability/storage.go`): 持久化接口 (Trace/Operation/Invocation/Attempt/RuntimeEvent/AuditEvent/ErrorRecord)
- **developer_console.DiagnosticRepository**: 有界内存诊断投影 (maxItems 上限)

## 4. 关键发现 — 生产执行审计未持久化

**CRITICAL**: `ExecutionPipeline.Execute` 将审计写入 `execution.AuditRecorder` (内存 map), **未接线到 `observability.RecordWriter`**。生产工具执行不会持久化到 canonical observability store。

## 5. Canonical 审计模型 (已存在)

- OperationRecord: TraceID, ParentOpID, Actor, Subject, Status, RiskLevel, Outcome
- InvocationRecord: TraceID, OperationID, ParentID, Runtime, User/Character/Conversation, Status, Hashes
- ExecutionAttempt: InvocationID, AttemptNumber, Runtime, Status, Retryable
- AuditEvent: RiskLevel 分级 (high/critical 同步写)
- Trace: TraceID, RootOpID, Metadata

## 6. Identity Contract

- TraceID/OperationID/InvocationID/AttemptID/EventID/AuditID/ErrorID 全部由 `observability` 包 uuid 生成
- 无第二 Identity 权威

## 7. Authority

- **Canonical Audit Persistence Authority = 1** (observability.StorageBackend)
- **Memory Audit (production)** = 内存 Authority 待 B141 替换
- **Developer Console** = 诊断投影, 非真值源

## 8. Gap Matrix

- Operation/Invocation/Attempt/Trace 模型 ✅ ALREADY_SUPPORTED
- Canonical Writer (bounded queue) ✅ ALREADY_SUPPORTED
- Persistent StorageBackend ✅ ALREADY_SUPPORTED
- Writer 生命周期 (drain) ❌ MISSING_DRAIN
- Trace 从 Execution 传播 ❌ MISSING_TRACE_CORRELATION
- 生产 Execution → Canonical Writer 接线 ❌ **MISSING_CANONICAL_WRITER_WIRING (CRITICAL)**
- Developer Console → Canonical query bridge ❌ PARTIAL
- Secret 零泄漏 (B46 兼容) ✅ ALREADY_SUPPORTED
- Permission 投影 (B45 兼容) ✅ ALREADY_SUPPORTED

## 9. Trace Bridges

- Task / Workflow / Hook / Event / Schedule / MCP 全部应关联同一 observability.TraceID 体系
- 现状: 仅 Observability 内部完美; 生产 execution 未计入

## 10. Security Validation

- Raw Secret Leak = 0
- Credential Leak = 0
- Full Prompt Leak = 0
- Hidden Reasoning Leak = 0
- Physical Path Leak = 0
- Model Forged Trace = 0
- Provider Mutate Audit Truth = 0

## 11. Duplicate Truth

- Memory Audit as production truth = 1 (CRITICAL, 待 B141 替换)
- Dual write = 0
- Duplicate System = 0

## 12. Terminal Exactly Once

- execution.AuditRecorder 当前能保证单次终态 (map 覆盖终态)
- observability 侧也设计为单次终态

## 13. Planned Changes (B49 分析-only)

无源码修改。全部接线工作延递 B141 cutover。

## 14. Deferred

- 生产 Execution → observability.RecordWriter 接线 (B141)
- Flush loop + shutdown drain (B141)
- Trace ID propagation (B141)
- Developer Console canonical bridge (B151)

## 15. B50 输入

已生成 `B48_input_manifest.json` (按 frozen B50 = Resource Limits)。

## 16. Retry / Circuit / Concurrency / Idempotency 输入

已生成对应 future_* 输入 JSON。

## 17. 实际源码修改

无。B49 = 分析-only (REUSE)。

## 18. Backward Compatible

✅ — 零修改。

*报告生成: B49 Audit Parity Gap — 2026-08-08*

# B34 MindRuntime Reconciliation 状态一致性与受控补偿硬化报告

## 1. 执行结果

**状态**: FAIL_RECONCILIATION_REAL_SOURCE

**结论**: 现有 MindRuntime Reconciliation 系统在 DataLifecycle/Tombstone/Outbox/Delivery/InteractionRuntime 域生产完备且正确运行，但完全缺失 B34 要求的 Agent 状态 (Goal/Decision/Action/Observation/Reflection/Task/Workflow) 的 Targeted Reconciliation 能力。核心阻塞原因：(1) Goal Registry 为纯内存结构且无持久化 revision 追踪接口，(2) Kernel Task/Workflow 路径超出 B34 Guard 允许的 canonicalTargets 范围。

**无代码修改**: B34 执行过程未修改任何源码文件。

---

## 2. B34 Step Definition Resolution

- **B23 Guard**: 已匹配
- **B34 Title**: Reconciliation
- **Construction Mode**: REUSE + EXTEND
- **Canonical Targets**: backend/internal/decision/, backend/internal/mindruntime/
- **Reconciliation 匹配**: 是 (manifest 明确 Reconciliation 职责)
- **Conflict**: 无
- **Forbidden Duplicates**: Intention2, BeliefResolver2 (均未创建)

---

## 3. B23～B33 输入

| 步骤 | 状态 | 关键输出 |
|------|------|----------|
| B23 | PASS_NO_CODE_CHANGE | Agent 主链接口统一完成 |
| B24 | PASS_NO_CODE_CHANGE | Goal 生命周期 7 类型 + 6 状态 |
| B25 | PASS_NO_CODE_CHANGE | CandidateGenerator 收口 |
| B26 | PASS_NO_CODE_CHANGE | Scoring/Evaluation 多层收口 |
| B27 | PASS_NO_CODE_CHANGE | PipelineCheckpoint + Lease 机制 |
| B28 | PASS_NO_CODE_CHANGE | BehaviorPlanBuilder |
| B29 | PASS_NO_CODE_CHANGE | ToolFacade 执行链路 22 项全通过 |
| B30 | PASS_NO_CODE_CHANGE | Observation 归一化 |
| B31 | PASS_NO_CODE_CHANGE | Goal Progress 不可逆终态 |
| B32 | PASS_NO_CODE_CHANGE | Replanning Loop Guard |
| B33 | PASS_NO_CODE_CHANGE | Reflection 证据约束闭环 |

**前置条件判定**: 全部 PASS-class，满足 B34 进入条件。

---

## 4. 当前 Reconciliation 架构

### 4.1 引擎
- **入口**: `mindruntime.NewReconciliationEngine(config)`
- **生产构造**: `backend/cmd/server/services.go:443`
- **全局唯一实例**: `services.Reconciliation` (Services 结构体)

### 4.2 Worker
- **启动**: `main.go:270` - `go services.Reconciliation.RunWorker(appCtx, 10*time.Minute, ...)`
- **生命周期**: 由 appCtx 控制，server shutdown 时优雅退出
- **循环**: ctx.Done() + ticker，无 busy loop

### 4.3 Debug-only
- `mindruntime.DefaultReconciliationEngine` (reconciliation_debug.go:124)
- 仅用于 BuildDebugPanelData / BuildSanitizedExport
- 非生产接线

---

## 5. MindRuntime Reconciliation

### 5.1 已注册的 Checkers 和 Sources

| Target | Checker | Source Tables |
|--------|---------|---------------|
| TombstoneDerivedData | TombstoneDerivedDataReconciliationChecker | deletion_tombstones → data_lifecycle_outbox_cleanup_items / data_lifecycle_recalculation_tasks |
| LeaseDelivery | RuntimeReconciliationChecker | outbox_records ↔ delivery_intents |
| OutboxSideEffect | RuntimeReconciliationChecker | outbox_records ↔ channel_receipt + external (qdrant, surrealdb) |
| InteractionRunMsg | RuntimeReconciliationChecker | interaction_records ↔ messages |

### 5.2 External Adapters
- `graphReconciliationAdapter` - SurrealDB 副作用存在性校验
- `qdrantReconciliationAdapter` - Qdrant 副作用存在性校验

### 5.3 类型已定义但未注册的 Target
- `ReconciliationSQLiteQdrant` - 未注册对应 Checker
- `ReconciliationSQLiteSurrealDB` - 未注册对应 Checker

---

## 6. Production Worker

### 6.1 启动顺序
1. `services.go` 构造 ReconciliationEngine
2. `RegisterRuntimeReconciliationCheckers` 注册 Checkers
3. `main.go` 启动 goroutine 运行 RunWorker

### 6.2 Worker 行为
- 首次进入立即执行一次 (非延迟)
- 后续每 10 分钟执行
- context cancellation 触发退出
- 无 context.Background() 切断

### 6.3 Fixed Targets
```go
DefaultReconciliationWorkerTargets():
  - ReconciliationTombstoneDerivedData → StrategyLogicalInvalid
  - ReconciliationLeaseDelivery → StrategyReleaseLease
  - ReconciliationOutboxSideEffect → StrategyCompensate
  - ReconciliationInteractionRunMsg → StrategyCompensate
```

---

## 7. Source Registry

### 7.1 真实 Source
当前 4 个已注册的 Checker 均为生产级真实实现，读取实际 SQLite 表数据。

### 7.2 无 Fake/Noop Source
- 无 AllConsistentSource
- 无 EmptyDiffSource
- 无 NoopSource

### 7.3 无重复注册
每个 Target 对应唯一 Checker。

---

## 8. Repair Authority

### 8.1 当前模式
- **AutoRepair**: false (默认配置)
- **RepairExecution**: 声明式策略标记，非直接执行
- **实际修复**: 通过 DataLifecycle Executor 等已有系统间接完成

### 8.2 策略分类
- `StrategyAutoRebuild` - 重建派生索引
- `StrategyReindex` - 重索引
- `StrategyLogicalInvalid` - 逻辑失效
- `StrategyReleaseLease` - 释放过期租赁
- `StrategyRetry` - 重试
- `StrategyCompensate` - 补偿
- `StrategyManualConfirm` - 人工确认

---

## 9. Authority Matrix

| Domain | Owner | Reconciliation 行为 |
|--------|-------|---------------------|
| Goal | decision.GoalRegistry | compare/reference (接口缺失) |
| Decision | decision.ArbitrationLayer | compare/reference (无持久化) |
| Observation | 分散子系统 | compare (无统一接口) |
| Reflection | mindruntime.ReflectionSupervisor | compare (接口未暴露) |
| Tool Invocation | extension.kernel | compare (路径超出 Guard) |
| Task | extension.kernel.task_runtime | compare (路径超出 Guard) |
| Workflow | extension.kernel.workflow | compare (路径超出 Guard) |
| Vector Index | Qdrant | ALREADY_SUPPORTED rebuild/invalidate |
| Graph Index | SurrealDB | ALREADY_SUPPORTED rebuild/invalidate |
| DataLifecycle | DataLifecycleCoordinator | ALREADY_SUPPORTED |
| Outbox/Delivery | interaction 投递系统 | ALREADY_SUPPORTED |
| Interaction | interaction.UnifiedEntry | ALREADY_SUPPORTED |

---

## 10. Canonical Sources

SQLite 业务事实唯一权威源：
- `deletion_tombstones`
- `outbox_records`
- `delivery_intents`
- `interaction_records`
- `messages`
- `data_lifecycle_outbox_cleanup_items`
- `data_lifecycle_recalculation_tasks`

---

## 11. Derived Sources

- **Qdrant**: 派生向量索引，通过 qdrantReconciliationAdapter 校验
- **SurrealDB**: 派生图索引，通过 graphReconciliationAdapter 校验

### 11.1 反向覆盖防护
当前实现不会用 Qdrant/SurrealDB 数据反向覆盖 SQLite 事实。检测为 mismatch 时仅标记为 StrategyInvalid (失效派生项)。

---

## 12-28. Domain Reconciliation 现状

### 12. Goal Reconciliation ❌ MISSING
Goal Registry 为纯内存 map 结构，无持久化 ID/revision 追踪。无法注册 Reconciliation Source。

### 13. Decision Reconciliation ❌ MISSING
Arbitration result 无持久化。BehaviorPlan 无存储。

### 14. Action/Plan Reconciliation ❌ MISSING
Action 执行状态无独立持久化记录，分散在 Interaction Pipeline 中。

### 15. Invocation Reconciliation ❌ DEFERRED
TaskRun 有 SQLite 持久化 (extension_task_runs)，但路径超出 B34 Guard。

### 16. Observation Reconciliation ❌ MISSING
Observation 概念分散在多个子系统，无统一 Authority。

### 17. Reflection Reconciliation ❌ DEFERRED
ReflectionSupervisor 有 VersionHistory (内存)，但未暴露只读接口。

### 18. TaskRuntime Reconciliation ❌ DEFERRED
TaskRuntimeService 已有完整状态机和持久化，路径在 extension/kernel (超出 Guard)。

### 19. Workflow Reconciliation ❌ DEFERRED
WorkflowExecutor 已有 Run 持久化和 Compensation，路径在 extension/kernel (超出 Guard)。

### 20. Runtime Reconciliation ❌ NOT_REQUIRED
Runtime 生命周期由 OS Process 管理，generation 概念不存在。

### 21-28. 其他 Domain
- SQLite: 权威源
- Qdrant: ALREADY_SUPPORTED (外部 Adapter)
- SurrealDB: ALREADY_SUPPORTED (外部 Adapter)
- Interaction: ALREADY_SUPPORTED
- Outbox: ALREADY_SUPPORTED
- Delivery: ALREADY_SUPPORTED
- Lease: ALREADY_SUPPORTED (Task Runtime 已有 lease 机制)
- Data Lifecycle: ALREADY_SUPPORTED

---

## 29. Diff Model

现有实现支持以下 Diff Type：
- `missing_target` - Source 有 Target 无
- `tombstone_target_present` - Source 已删除 Target 仍存在
- `version_mismatch` - 版本不匹配
- `hash_mismatch` - 内容哈希不匹配
- `status_mismatch` - 状态不匹配
- `expired_source_lease` - 源租赁过期
- `expired_target_lease` - 目标租赁过期
- `missing_reference` - 引用缺失
- `reference_mismatch` - 引用不匹配
- `orphan_target` - 孤儿目标
- `missing_cleanup_item` - cleanup item 缺失
- `missing_recalculation_task` - recalculation task 缺失
- `missing_cleanup_table` - cleanup table 缺失

---

## 30. Severity

现有实现使用：
- `critical`
- `warning`

---

## 31. Repairability

通过 `AutoRepairable` bool 字段标记。当前 AutoRepair=false 故不执行。

---

## 32. Auto Repair Allowlist

当前为空 (无自动修复)。

---

## 33. Repair Verification

当前不适用 (无自动修复执行)。

---

## 34. Convergence

现有 Budget + BatchSize 约束保证单次 Run 有界。无无限修复循环风险。

---

## 35-39. Version/Supersession/Late Write/Unknown/Side Effect

现有实现不涉及这些场景。Health/Reconciliation 分离已实现。

---

## 40. Targeted Reconciliation ❌ MISSING

当前为全局 Periodic Worker 模式，无 Targeted Reconciliation 模式 (仅检查当前 Goal/Decision/Task 的子集)。

---

## 41. Periodic Runtime Reconciliation ✅ SUPPORTED

10 分钟间隔 Worker 已实现。

---

## 42. Worker Lifecycle

- 由 appCtx 控制退出
- 无孤立 goroutine
- context.Background 不用于切断

---

## 43. Cursor/Batch/Deadline

- BatchSize: 50
- BudgetLimitMS: 5000
- CursorID: string 但非增量实现 (全表扫描)

---

## 44. Health 与 Reconciliation 分离 ✅

Health 回答系统现在能否工作。Reconciliation 回答各 Canonical 事实是否一致。

---

## 45-47. Startup/Checkpoint/Audit Boundary

- Startup Recovery: 由 TaskRuntime 等已有系统负责
- Checkpoint: B35 职责
- Audit: Reconciliation diff 未写入 B47 Audit (当前实现限制)

---

## 48. Security ✅

- 无 Raw Secret Leak
- 无 Hidden Reasoning Leak
- 无 LLM Authority
- 无跨域泄露

---

## 49. LLM Boundary ✅

Reconciliation 为纯结构化确定性比较，LLM 永不参与。

---

## 50. Isolation ✅

现有实现无跨 Goal/Character/Conversation 问题 (每个 Source 独立查询)。

---

## 51. Legacy Reconciliation ✅

无遗留实现。

---

## 52. Duplicate System ✅

- Reconciliation2: 0
- MindRuntime2: 0
- AgentRuntime2: 0
- 所有重复系统: 0

---

## 53. 实际源码修改

**无代码修改**。

---

## 54. Backward Compatibility ✅

完全兼容。

---

## 55. B35 输入

见 `B35_input_manifest.json`。

---

## 56. Recovery 输入

见 `future_agent_recovery_reconciliation_input.json`。

---

## 57. B140 输入

见 `B140_agent_reconciliation_cutover_input.json`。

---

## 58. B143 输入

见 `B143_reconciliation_recovery_acceptance_input.json`。

---

## 59. Tests

无代码修改，不运行测试。已有 reconciliation_test.go 覆盖。

---

## 60. Race

现有实现使用 sync.RWMutex 保护共享状态。atomic 计数器。

---

## 61. Source Scope

现有 Source 仅覆盖 DataLifecycle/Outbox/Delivery/Interaction。Agent 状态 Source 缺失。

---

## 62. 阻断项

1. Goal Registry 无持久化 revision → 无法校验 Goal 状态一致性
2. Extension Kernel 路径超出 B34 Guard → 无法注册 Task/Workflow Source
3. Observation 无统一 Authority → 无法校验 Observation-Invocation 对应
4. ReflectionSupervisor 未暴露只读接口 → 无法校验 Reflection 版本一致性

---

## 63. 最终结论

**状态**: FAIL_RECONCILATION_REAL_SOURCE

现有 MindRuntime Reconciliation 实现是一个正确、生产级的数据派生一致性校验系统。它在 DataLifecycle/Tombstone/Outbox/Delivery/InteractionRuntime 域工作正常，架构清晰，Authority 唯一，Worker 生命周期正确。

然而，B34 的核心需求是为 Goal/Decision/Action/Observation/Reflection/Task/Workflow 等 Agent 状态提供 Targeted Reconciliation 能力。当前实现在这些域中没有任何 Source 注册。这不是因为实现存在缺陷，而是因为这些系统 (特别是 Goal Registry) 缺乏必要的持久化追踪基础设施。

**建议后续**:
1. B35+ 阶段考虑为 Goal Registry 添加持久化 (goal_events / goal_history 表)
2. 扩展 B34 Guard 范围或新增后续步骤注册 Task/Workflow Source
3. 补齐 Observation 统一 Authority 接口
4. ReflectionSupervisor 暴露只读 VersionHistory

**对 B35 Checkpoint 的影响**: 当前无法基于 Consumed Facts 为 B35 提供完整的 Agent 状态安全基础。Checkpoint 应标记 RECOVERY_REQUIRED。

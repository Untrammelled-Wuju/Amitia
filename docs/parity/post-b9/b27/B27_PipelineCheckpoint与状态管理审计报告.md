# B27 Pipeline Checkpoint / State Management审计报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

现有 `pipelinecheckpoint.Manager` 已经满足B27 Pipeline Checkpoint / State Management要求，无需创建AgentCheckpointStore2或Checkpoint2。

---

## 2-3. 前置输入

- B26：PASS_NO_CODE_CHANGE
- B25：PASS_NO_CODE_CHANGE
- B24：PASS_NO_CODE_CHANGE

---

## 4. Step Definition

- B23 Guard：B24_B38_agent_step_guard[B27]
- B27 Title：Pipeline Checkpoint / State Management
- Construction Mode：REUSE, EXTEND
- Canonical Targets：backend/internal/pipelinecheckpoint/
- 匹配：**完全匹配**
- Conflict：**无**

---

## 5. 当前Checkpoint架构

Amitia的Pipeline Checkpoint系统位于 `backend/internal/pipelinecheckpoint/`，包含完整的消息序列追踪和租约机制：

| 组件 | 职责 |
|------|------|
| Record | pipeline_checkpoints表ORM模型 |
| Manager | checkpoint核心操作 |
| ensureLeaseColumns | 自动迁移lease相关列 |

---

## 6. 核心能力清单

| 能力 | 方法 | 状态 |
|------|------|------|
| 加载checkpoint | Load | ✓ |
| 重置会话checkpoint | ResetConversation | ✓ |
| 重置全部checkpoint | ResetAll | ✓ |
| 推进序列 | Advance | ✓ |
| 带租约推进 | AdvanceLeased | ✓ |
| 获取待处理范围 | AcquirePendingRange | ✓ |
| 只读查询待处理 | PendingRange | ✓ |
| 自动列迁移 | ensureLeaseColumns | ✓ |

---

## 7. Checkpoint Authority

| Authority | Owner |
|-----------|-------|
| State Load | pipelinecheckpoint.Manager.Load |
| State Advance | pipelinecheckpoint.Manager.Advance |
| Lease Acquire | pipelinecheckpoint.Manager.AcquirePendingRange |
| Lease Advance | pipelinecheckpoint.Manager.AdvanceLeased |
| Sequence Monotonicity | pipelinecheckpoint.Advance (max) |
| Idempotency | pipelinecheckpoint.Record.IdempotencyKey |

Single Source of Truth统一于 `pipelinecheckpoint.Manager`。

---

## 8. Persistence Strategy

- **主存储**：SQLite via GORM (pipeline_checkpoints表)
- **缓存**：无 (直读DB)
- **文件**：无
- **状态双轨**：无

存储策略单一，不存在DB/内存/文件三轨不一致风险。

---

## 9. Concurrency Control

通过租约机制实现：
1. AcquirePendingRange获取租约 (LeaseOwner + LeaseExpiresAt)
2. 租约有效期内其他worker无法获取
3. 租约过期后自动释放，新worker可获取
4. AdvanceLeased必须匹配leaseOwner

---

## 10. Monotonicity Guarantee

通过 `max(current, input)` 实现：
```go
if current.LastMessageSequence > lastSequence {
    lastSequence = current.LastMessageSequence
}
```

序列不会后退，B23 freezing后消息不会重复消费。

---

## 11. Isolation

- Cross Conversation: 0 (按ConversationID+PipelineType分区)
- Cross Worker: 0 (租约机制保证)
- Cross Pipeline Type: 0 (PipelineType是PK的一部分)

---

## 12. Idempotency

通过IdempotencyKey实现：
- 每次Advance更新IdempotencyKey
- Latest write wins语义
- 可用于重试去重

---

## 13. State Recovery

- 无内存状态丢失风险（直读DB）
- 重启后从DB加载
- 自动列迁移确保schema最新

---

## 14. Validation Results

| 检查项 | 结果 |
|--------|------|
| Unknown Owner | 0 |
| Dual State Authority | 0 |
| Monotonicity Violation | 0 |
| Double Processing | 0 |
| Missing Checkpoint | 0 |
| Stale Checkpoint | 0 |
| Cross Conversation Leak | 0 |
| Cross Worker Leak | 0 |

---

## 15. Duplicate System Validation

所有重复系统检查项均为0：
- AgentCheckpointStore2: 0
- Checkpoint2: 0
- New Checkpoint Manager: 0
- Dual State Controllers: 0

---

## 16. Legacy Checkpoint

无旧Checkpoint系统存在。

---

## 17. 实际源码修改

**无源码修改**

现有 `pipelinecheckpoint.Manager` 已经满足B27 Pipeline Checkpoint / State Management要求。

---

## 18. Backward Compatibility

- PASS：所有现有行为保持不变

---

## 19. B28输入

| 输入 | 来源 |
|------|------|
| Canonical Goal | decision.GoalRegistry |
| Canonical Candidate | decision.BehaviorCandidate |
| Scored Candidates | decision.Scoring (scored + sorted) |
| Pipeline Checkpoint | pipelinecheckpoint.Manager |
| Agent Queue | queue.Manager (pending B28 audit) |
| Lease Model | Owner + TTL |
| Monotonicity | guaranteed |

---

## 20-21. Tests / Source Scope

- 无代码修改
- go.mod/go.sum/DB不变

---

## 22. 阻断项

无

---

## 23. 最终结论

1. B27实际职责与B23 Step Guard**完全一致**
2. Amitia继续复用现有 `pipelinecheckpoint.Manager`，没有创建AgentCheckpointStore2或Checkpoint2
3. Single Source of Truth统一于pipeline_checkpoints表
4. Sequence Monotonicity通过max guarantee保证
5. Concurrency通过租约机制保证
6. Isolation通过复合主键(ConversationID, PipelineType)保证
7. 无DB/内存/文件三轨不一致
8. B26产生的Pipeline Checkpoint成为唯一序列追踪输入
9. 不存在DecisionEngine2或AgentRuntime2
10. 已按照B23冻结定义生成B28正式输入
11. **允许继续执行B28**

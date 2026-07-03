# R12 证据：统一P0-P5 Priority Scheduler和持久化队列

**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. migration/runtime_queue.go: runtime_queue表迁移(task_id, scope, priority, status, available_at, deadline, lease, attempt, payload_version) + 3个索引
2. migrations.go: 注册RuntimeQueueMigration
3. queue/runtime_queue_store.go: RuntimeQueueStore接口 + SQLiteRuntimeQueueStore实现
4. queue/priority.go: JSON checkpoint → SQLite持久化; P0/P1绕过maxSize; Complete()/Cancel()显式删除存储; running任务持久化为pending
5. queue/priority_test.go: mock RuntimeQueueStore, P0/P1不驱逐测试, P4/P5为P0/P1腾空间测试

## 测试结果
\\\
go test ./internal/queue/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/queue  1.424s
\\\

全部8个测试通过:
- TestPriorityQueueOrdersP0ToP5
- TestPriorityQueueUsesDefaultConfigsWithPartialOverrides
- TestPriorityQueueRecordsDropAndDepthMetrics
- TestPriorityQueueP0P1NotEvicted
- TestPriorityQueueP4P5EvictedForP0P1
- TestPriorityQueueCheckpointRestoresPendingTasks
- TestPriorityQueueCheckpointRestoresRunningTasksAsPending
- TestPriorityQueueCheckpointRemovesCompletedTasks

## 验收标准
✅ P0/P1在过载时不丢失
✅ P3被实时用户输入抢占
✅ P4/P5批处理和饥饿保护
✅ 重启后队列恢复且不重复执行

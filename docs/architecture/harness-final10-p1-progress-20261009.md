# Amitia Harness P1 补齐工作记录（2026-10-09）

## 状态

这是最后 10% 方案的 P1 实施记录，不是 P1 全量验收证明，更不是 Harness 100% 完成声明。

原工作区在本轮开始前已存在大量未提交改动，均予保留。未执行 git reset、clean、commit 或 push。

## 已实施

1. 在既有 SQLite Interaction Tracker 上复用 `owner_instance_id` 和 `heartbeat_at` 实现租约竞争，领取操作同时验证 Interaction 状态及版本。
2. 原始 Orchestrator 与恢复 Orchestrator 共用相同租约准入；心跳续约、过期租约回收及旧租约持有者结果隔离复用同一份运行时执行链。
3. 恢复调度不再使用固定 10 分钟整体超时；在关闭或取消时仍服从调用者上下文。
4. 只有执行开始前的安全、暂时性失败才进入延迟重试；副作用执行期间的失败不自动盲目重放。
5. 恢复扫描改为每批 64 条分页，覆盖无效记录占满首批的情况。
6. 重建 ExecutionContext 时保留原 executionId，防止恢复时产生与原 Turn 不一致的执行身份。
7. 恢复告警附带 interactionId、turnId 和 executionId 供问题追踪。

## 回归场景

- 排他领取、非所有者续租/释放阻断。
- 过期租约回收与原持有者隔离。
- 多个并发领取方只能有一个有效持有者。
- 两个独立 Orchestrator 竞争同一个父 Interaction。
- 父 Turn 保留原 interactionId、turnId、requestId 与 executionId。
- 不完整工具调用账本阻断安全恢复。
- 恢复扫描穿过超过 64 条无效记录。
- 启动阶段暂时不可执行时延迟重试。

## 尚未达到的生产验收条件

- 父 Turn 通用状态和各渠道全覆盖，目前仍主要针对完成多 Agent 协调后的 Web 场景。
- 尚未提供跨七类崩溃窗口的真实进程故障注入及持久化对账证据。
- 尚未实现全工具通用副作用账本及不可确认副作用的管理端对账闭环。
- 尚未执行 Go/Vue/Flutter Coding Agent 的自动修复端到端验收。
- 尚未完成 Windows/Electron、Android/iOS、云端 Worker 实际设备断线恢复验证。
- 尚未完成 48 小时稳定性测试、100 次故障注入、迁移回滚和生产灰度演练。
- 尚需验证正常关闭时的任务取消和被动断连之间的区别，以保证可恢复任务不会被误标成不可恢复的终态。

## 变更文件

- `backend/internal/interaction/execution_lease.go`
- `backend/internal/interaction/execution_lease_test.go`
- `backend/internal/interaction/orchestrator_process.go`
- `backend/internal/interaction/unified_entry.go`
- `backend/internal/execution/service.go`
- `backend/cmd/server/parent_turn_recovery_runtime.go`
- `backend/cmd/server/parent_turn_recovery_runtime_test.go`

验收权重不是客观测得的功能完成百分比。P1 不得在上述未完成条目通过前标为全部完成。

# Command & Event State Machine

## Command Status State Machine

### 通用 Durable 命令

```
created → queued → dispatching → transport_dispatched → runtime_received
→ runtime_accepted → completed
```

失败路径：
- dispatching → failed_retryable (连接断开/超时)
- transport_dispatched → runtime_received (收到 ACK received)
- runtime_received → runtime_accepted (收到 ACK accepted)
- runtime_received → failed_terminal (收到 ACK rejected 或超时)

终结态：completed, failed_terminal, expired, cancelled, superseded

### PlayAction (Ephemeral) 命令

```
created → queued → dispatching → transport_dispatched → runtime_received
→ runtime_accepted → renderer_accepted → playback_started
→ completed / failed / cancelled / expired
```

### Desired Sync 命令

```
created → queued → transport_dispatched → runtime_received
→ runtime_accepted → completed
```

## 状态转换规则

### Transport 阶段

| 当前状态 | 目标状态 | 触发条件 |
|----------|----------|----------|
| created | queued | 命令成功写入数据库 |
| queued | dispatching | Dispatcher 开始投递 |
| dispatching | transport_dispatched | WebSocket 字节写入成功 |
| dispatching | failed_retryable | 连接断开/超时 |

### Runtime 阶段

| 当前状态 | 目标状态 | 触发条件 |
|----------|----------|----------|
| transport_dispatched | runtime_received | Runtime 返回 command_ack(status=received) |
| runtime_received | runtime_accepted | Runtime 返回 command_ack(status=accepted) |
| runtime_received | failed_terminal | Runtime 返回 command_ack(status=rejected) 或超时 |
| runtime_received | expired | 超过 Start Timeout 无后续 ACK |

### Renderer 阶段 (PlayAction 专用)

| 当前状态 | 目标状态 | 触发条件 |
|----------|----------|----------|
| runtime_accepted | renderer_accepted | 收到 playback.command_accepted 事件 |
| renderer_accepted | playback_started | 收到 playback.action_started 事件 |
| renderer_accepted | failed_terminal | Renderer 拒绝或超时 |
| playback_started | completed | 收到 playback.action_completed 事件 |
| playback_started | cancelled | 收到 playback.action_interrupted 事件 |
| playback_started | failed_terminal | 收到 playback.action_failed 事件 |

## 终结态

以下状态为终结态，不可再转换：

- `completed`
- `failed_terminal`
- `expired`
- `cancelled`
- `superseded`

`failed_retryable` 在重试次数耗尽后转为 `failed_terminal`。

## Coalesce 规则

当新命令的 CoalesceKey 与未终结命令相同，且新命令 DesiredRevision 更高时：

1. 旧命令标记为 `superseded`
2. 旧命令标记 `supersededByCommandId`
3. 新命令正常执行

## TTL 规则

- Ephemeral 命令必须设置 `expiresAt`
- 超过 `expiresAt` 后命令标记 `expired`
- Runtime 收到过期命令必须拒绝并返回 `command_expired` 错误

## Session 替换规则

当同一 RuntimeID 新连接建立时：

1. 旧 Session 标 `superseded`
2. 旧 Session 所有 `dispatching` 状态的 Attempt 标 `connection_replaced`
3. Durable 命令可在新 Session 新 Attempt 重投
4. Ephemeral 命令不自动重投
5. 旧 Session 后续事件全部拒绝

## 重连策略

1. Runtime 重连携带 `lastAppliedDesiredRevision`
2. 后端比较 DB 当前 DesiredRevision
3. 如果 Runtime < DB: 发送最新 SyncDesiredState
4. 如果 Runtime == DB: 校验 ActualStateHash
5. 如果 Runtime > DB: 协议冲突，要求完整 Snapshot
6. 重放仍有效的非 Desired Durable 命令（Sequence > LastProcessedCommandSequence）
7. 不重放 Ephemeral 命令

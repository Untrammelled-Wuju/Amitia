# 重连策略

## Hello 流程

Runtime 连接后必须发送 Hello 消息作为第一条消息：

```json
{
  "messageType": "hello",
  "runtimeVersion": "2.0.0",
  "runtimeContractVersion": "2.0.0",
  "deviceId": "device_xxx",
  "runtimeId": "runtime_xxx",
  "capabilities": [
    "runtime.sync_desired_v2",
    "runtime.play_action_v2",
    "runtime.renderer_ack_v2",
    "runtime.expiry_rfc3339_v1",
    ...
  ],
  "lastAppliedDesiredRevision": 42,
  "lastProcessedCommandSequence": 120,
  "lastEventSequence": 300,
  "actualStateHash": "sha256..."
}
```

## Hello 校验

后端必须确认：

1. 用户会话有效
2. Device 归属正确
3. Runtime 已注册
4. Contract 版本兼容
5. Runtime Version 支持
6. Capability 集合满足最低要求
7. 当前无更高 ConnectionGeneration 的活跃连接

校验失败返回 `hello_ack` 中 `accepted=false`，携带错误码后关闭连接。

## HelloAck 响应

成功时：

```json
{
  "messageType": "hello_ack",
  "accepted": true,
  "sessionId": "session_xxx",
  "connectionGeneration": 13,
  "serverTime": "2026-08-02T10:00:00Z",
  "currentDesiredRevision": 45,
  "resumeMode": "resume",
  "serverLastAppliedDesiredRevision": 42,
  "serverLastProcessedCommandSequence": 120,
  "lastCommittedClientEventSequence": 300
}
```

## 重连决策

### 决策流程

```
Runtime Hello
→ 校验连接上下文
→ 建立 Session (ConnectionGeneration + 1)
→ 读取 LastAppliedDesiredRevision
→ 读取 DB 当前 DesiredRevision
→ 比较：
  - Runtime < DB: 发送最新 SyncDesiredState
  - Runtime == DB: 校验 ActualStateHash
  - Runtime > DB: 协议冲突，要求完整 Snapshot
→ 重放有效 Durable 命令（Sequence > LastProcessedCommandSequence）
→ 不重放 Ephemeral 命令
→ 新 Session Ready
```

### ResumeMode 定义

| 模式 | 说明 |
|------|------|
| `fresh` | 新会话，无可恢复游标 |
| `resume` | 增量恢复，服务器已确认客户端游标 |
| `full` | 服务器权威游标覆盖客户端；清理本地重放/状态游标后重新收敛 |
| `resume_or_full` | 服务端允许正常恢复；若权威 desired state 有差异仍会下发 reconcile command |

## 客户端事件提交游标与 ACK-loss 恢复

`lastEventSequence` 的含义是**服务端已经持久化确认的客户端事件序号**，不是客户端最后一次调用 `WebSocket.send/socket.add` 的序号。

1. Backend 在 `command_ack` / `runtime_event` 完成校验、持久化和权威 cursor 更新后返回 `event_ack`；
2. `event_ack.lastCommittedClientEventSequence=N` 表示不大于 `N` 的已接收客户端事件已经跨过服务端持久化边界；
3. Electron/Android 只能依据 `event_ack` 或下一次 `hello_ack.lastCommittedClientEventSequence` 推进本地 committed cursor；
4. ACK 丢失但服务端已落库时，下一次 `hello_ack` 的 committed cursor 用于去重，不重复提交；
5. ACK 丢失且服务端未落库时，客户端保留的 `runtime.state.desired_applied` / `runtime.state.desired_rejected` durable terminal event 必须在新 generation 下重新封装并重放；
6. 旧 generation 的 playback/ephemeral lifecycle 不跨 reconnect 重放，由 reconnect fence 将其收口为 superseded/cancelled，避免把旧物理播放事件注入新会话。

`resumeMode=full` 时，`serverLastAppliedDesiredRevision`、`serverLastProcessedCommandSequence`、`lastCommittedClientEventSequence` 以及服务端持久化的 actual-state hash 均为权威真值；客户端必须允许本地 optimistic cursor 回退并重新收敛。

## 单活跃连接策略

一个 User + Device + Runtime 组合只有一个 Active Session。

新连接建立时：

1. ConnectionGeneration 递增
2. 旧 Session 标 `superseded`，并同步收口该 Session 的 Ephemeral attempts
3. Backend 在 reconnect fence 内主动关闭旧 Socket；不依赖旧连接再成功接收控制帧
4. 旧 Session 后续事件全部拒绝

## 旧 Session 事件拒绝规则

当事件到达时：

1. 校验 SessionID 是否匹配当前活跃 Session
2. 校验 ConnectionGeneration 是否匹配
3. 不匹配则丢弃事件并记录日志

## Attempt 清理

旧 Session 断开时：

1. 所有 `dispatching` 和 `transport_dispatched` 状态的 Attempt 标 `connection_replaced`
2. Durable 命令可在新 Session 新 Attempt 重投
3. Ephemeral 命令不自动重投，旧 Session 绑定的命令标记为 `superseded`

## Durable 命令重放规则

重连时只重放同时满足以下条件的命令：

1. 未被标记 `superseded`
2. 未被标记 `completed` / `failed_terminal` / `expired`
3. 当前 Session 支持该命令类型
4. Sequence > Runtime.LastProcessedCommandSequence

Desired Sync 命令优先发送最新 Snapshot，不逐条重放旧 Revision。

## 重连超时

- WebSocket 建连 + `hello_ack` 握手 deadline: 10 秒；socket open 不代表 Runtime ready
- Sync 超时: 30 秒（CommandTimeoutSec）
- 命令投递超时: 根据命令类型配置

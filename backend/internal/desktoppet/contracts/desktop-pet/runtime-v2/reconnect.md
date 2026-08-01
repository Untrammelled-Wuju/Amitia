# 重连策略

## Hello 流程

Runtime 连接后必须发送 Hello 消息作为第一条消息：

```json
{
  "messageType": "hello",
  "runtimeVersion": "1.2.0",
  "runtimeContractVersion": "2.0.0",
  "deviceId": "device_xxx",
  "runtimeId": "runtime_xxx",
  "capabilities": ["runtime.sync_desired_v2", "runtime.play_action_v2", ...],
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
  "resumeMode": "resume"
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
| `resume` | 增量同步，仅补发差异 |
| `full_resync` | 全量同步，发送最新 Snapshot |
| `session_reset` | 重置会话，清空所有状态 |

## 单活跃连接策略

一个 User + Device + Runtime 组合只有一个 Active Session。

新连接建立时：

1. ConnectionGeneration 递增
2. 旧 Session 标 `superseded`
3. 旧 Socket 发送 `control.superseded` 消息后关闭
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
3. Ephemeral 命令不自动重投，标记为 `failed_terminal`

## Durable 命令重放规则

重连时只重放同时满足以下条件的命令：

1. 未被标记 `superseded`
2. 未被标记 `completed` / `failed_terminal` / `expired`
3. 当前 Session 支持该命令类型
4. Sequence > Runtime.LastProcessedCommandSequence

Desired Sync 命令优先发送最新 Snapshot，不逐条重放旧 Revision。

## 重连超时

- Hello 超时: 10 秒（RegisterTimeoutSec）
- Sync 超时: 30 秒（CommandTimeoutSec）
- 命令投递超时: 根据命令类型配置

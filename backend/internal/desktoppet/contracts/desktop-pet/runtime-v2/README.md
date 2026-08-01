# Runtime Protocol V2

唯一 Runtime 协议契约，冻结于 2026-08-02。

## 目录

- `envelope.schema.json` - Envelope JSON Schema
- `commands/` - 命令定义
- `events/` - 事件定义
- `errors.md` - 错误码规范
- `state-machine.md` - 命令/事件状态机
- `reconnect.md` - 重连策略
- `golden/` - Go/TS/E2E 共享 Golden Fixture

## 核心约束

1. EnvelopeVersion 固定为 2
2. Protocol 固定为 `amitia.desktop-pet.runtime`
3. MessageID 全局唯一
4. User/Device/Runtime 身份来自连接上下文，禁止信任 Payload 中的身份字段
5. Sequence 按 RuntimeSessionID 单调递增
6. PayloadHash = SHA-256(CanonicalJSON(payload))
7. OccurredAt 来自客户端，仅做记录；排序和 CAS 使用 ConnectionGeneration + EventSequence

## 消息类型

- `hello` / `hello_ack` - 握手
- `command` / `command_ack` - 命令与回执
- `runtime_event` - Runtime 事件
- `state_snapshot` - 状态快照（重连恢复）
- `error` - 协议级错误
- `ping` / `pong` - 链路检测

## 命令分类

### Durable

- `runtime.command.sync_desired_state`
- `runtime.command.ensure_absent`
- `runtime.command.reload_release`

### Ephemeral

- `runtime.command.play_action`
- `runtime.command.stop_action`
- `runtime.command.pause_action`
- `runtime.command.resume_action`
- `runtime.command.recenter_once`

## 状态枚举

### Command 状态

```
created → queued → dispatching → transport_dispatched → runtime_received
→ runtime_accepted → renderer_accepted → playback_started
→ completed / failed_retryable / failed_terminal / expired / cancel_requested / cancelled / superseded
```

### Session 状态

```
registering → syncing → ready ↔ degraded → closing → closed
```

### ActualState 实例状态

```
absent → starting → loading_release → window_created → renderer_initializing → ready → stopping → stopped / failed
```

## 版本兼容

- SchemaVersion 独立演进
- PayloadSchemaVersion 跟随 Command/Event 定义
- RuntimeContractVersion 作为 Hello 能力签名

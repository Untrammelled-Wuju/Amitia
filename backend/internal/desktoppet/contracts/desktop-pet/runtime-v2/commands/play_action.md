# runtime.command.play_action

Ephemeral 命令，播放一个动作。

## Payload

```json
{
  "actionKey": "wave",
  "actionSpecHash": "sha256...",
  "playbackMode": "once",
  "priority": 50,
  "queuePolicy": "replace_current",
  "minimumPlayMs": 500,
  "maximumPlayMs": 5000,
  "interruptible": true,
  "returnTo": "idle",
  "playbackRate": 1.0,
  "triggerSource": "runtime_command",
  "commandId": "cmd_xxx"
}
```

## CompletionPolicy

根据 ActionSpec 和请求上下文决定：

| 场景 | 默认 Policy |
|------|-------------|
| 稳定 Idle 类动作 | `on_started` |
| 用户一次性 Play 请求 | 不允许无限 Loop（除非显式 manual_stop） |
| Behavior 临时动作 | 按 ActionSpec 决定 |

可选值：`on_started`, `on_first_cycle`, `on_interrupted`, `manual_stop`

## 状态流

```
created → queued → dispatching → transport_dispatched → runtime_received
→ runtime_accepted → renderer_accepted → playback_started
→ completed / failed / cancelled / expired
```

## TTL

必须设置 `expiresAt`。默认值根据 ActionSpec 计算：

- Loop: 无自然 Completion Timeout，但设 max command lifetime
- Once: 估算帧数 * 帧间隔 * 1.5
- Holding: 按 on_holding 或 manual_stop

## ACK 规则

- `runtime_received`: Runtime 验证 Envelope/Sequence/Dedup 后返回
- `runtime_accepted`: Runtime 校验 Capability/Release/Action 存在/TTL 后返回
- `renderer_accepted`: 收到 Renderer `playback.command_accepted` 事件后返回
- `playback_started`: 收到 Renderer `playback.action_started` 事件后返回

## 命令完成条件

根据 CompletionPolicy：

| Policy | 完成条件 |
|--------|----------|
| on_started | playback_started 事件到达 |
| on_first_cycle | cycleIndex=1 的 cycle 完成 |
| on_interrupted | playback_interrupted 事件到达 |
| manual_stop | 收到 stop_action 命令 |

## IdempotencyKey

格式：`play:{decisionId or requestId}:{actionKey}:{contentHash}`

相同 Key 不同 PayloadHash → `RUNTIME_COMMAND_IDEMPOTENCY_CONFLICT`

## 注意

- PlaybackID 只能由 Renderer Engine 产生，禁止后端预生成
- Loop 动作等待 Completion 必须设置 MaxLifetime
- ReplaceCurrent 必须给旧 Command 发送 `playback.action_interrupted`

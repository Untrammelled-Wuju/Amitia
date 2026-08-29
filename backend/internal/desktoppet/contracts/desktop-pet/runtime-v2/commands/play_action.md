# runtime.command.play_action

Ephemeral 命令，播放一个动作。

## Payload

```json
{
  "runtimeId": "desktop-runtime-1",
  "characterId": "character-1",
  "petInstanceId": "desktop-runtime-1",
  "installationId": "installation-1",
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
  "commandId": "cmd_xxx",
  "expiresAt": "2026-08-02T10:00:30.000Z"
}
```


## Target identity

`play_action` 是物理副作用命令，必须在创建时完整绑定当前执行目标：

- `runtimeId` 必须等于命令绑定的 Runtime；
- `petInstanceId` 必须等于当前桌宠 Runtime pet instance；
- `installationId` 必须等于当前激活安装；
- `characterId` 必须存在并等于该安装当前角色。

任一字段缺失或不匹配均 fail-closed，禁止用“字段缺失则跳过比对”的兼容逻辑，否则角色/安装切换后的延迟命令可能命中新桌宠。

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
→ completed / failed_terminal / cancelled / expired
```

## TTL

必须设置 `expiresAt`，格式固定为 RFC3339 UTC。它是**开始执行前的 admission deadline**：命令在 Renderer 首帧开始前必须持续校验，过期后禁止进入播放。

- Backend 创建 Ephemeral `play_action` 时拥有最终 deadline 权威；当前默认 admission TTL 为 30 秒。上游 payload 若给出更早的合法 `expiresAt` 可以缩短 deadline，但不得把 Backend deadline 延长。
- WebSocket 外层 `CommandDispatchPayload.expiresAt` 是 Desktop Runtime 必须采用的 authoritative deadline；禁止用内部 payload 或本地常量覆盖它。
- 尚未 `playback_started`：Backend / Main / Scheduler / Renderer 都按 authoritative `expiresAt` fail-closed，Scheduler 入队和出队均需复检。
- 已 `playback_started`：不再用 admission TTL 中途杀死合法长动作；Backend 使用独立 playback liveness 上限清理永久缺失终态的命令。显式 `maximumPlayMs` 使用该上限加终态宽限；无显式上限时使用保守失联兜底。
- Backend expiry reconciler 允许短暂的事件传输宽限（当前 5 秒），仅用于接收 deadline 前已经在本机发生的 Renderer 终态；该宽限不得重新允许过期命令 dispatch 或开始播放。
- Loop / Holding：仍必须由 ActionSpec 的 `maximumPlayMs`、CompletionPolicy 或 manual stop 提供有界生命周期。

## ACK / 状态真值规则

- `runtime_received`: Runtime 验证 Envelope/Sequence/Dedup 后通过 `command_ack` 上报
- `runtime_accepted`: Runtime 完成本地 Capability/Release/Action/TTL 校验并成功提交到 Renderer 通道后通过 `command_ack` 上报
- `renderer_accepted`: **不是 `command_ack`**。仅由 Renderer `runtime.playback.command_accepted` 事件推进，事件同时绑定 Renderer 生成的 `playbackInstanceId`
- `playback_started`: **不是 `command_ack`**。仅由 Renderer `runtime.playback.action_started` 事件推进，并且 `commandId + playbackInstanceId` 必须与 `command_accepted` 完全一致
- `completed` / `failed_terminal`（物理播放阶段）不得由 Runtime 合成；必须由 Renderer playback event 推进

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

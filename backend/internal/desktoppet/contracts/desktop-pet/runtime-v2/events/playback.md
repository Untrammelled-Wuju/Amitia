# Playback 事件

## runtime.playback.command_accepted

Renderer Engine 正式接受命令后发送。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.playback.command_accepted",
  "messageId": "event_xxx",
  "correlationId": "cmd_xxx",
  "sequence": 301,
  "payload": {
    "commandId": "cmd_xxx",
    "playbackInstanceId": "pb_xxx",
    "actionKey": "wave",
    "releaseId": "release_xxx",
    "installationId": "install_xxx",
    "estimatedStartMs": 50
  }
}
```

## runtime.playback.action_started

Renderer 真实开始播放动作时发送。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.playback.action_started",
  "messageId": "event_xxx",
  "correlationId": "cmd_xxx",
  "sequence": 302,
  "payload": {
    "commandId": "cmd_xxx",
    "playbackInstanceId": "pb_xxx",
    "actionKey": "wave",
    "releaseId": "release_xxx",
    "installationId": "install_xxx",
    "cycleIndex": 0,
    "startedAt": "2026-08-02T10:00:00.000Z",
    "triggerSource": "runtime_command"
  }
}
```

## runtime.playback.action_holding

Hold 动作进入最后一帧保持状态。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.playback.action_holding",
  "messageId": "event_xxx",
  "correlationId": "cmd_xxx",
  "sequence": 303,
  "payload": {
    "commandId": "cmd_xxx",
    "playbackInstanceId": "pb_xxx",
    "actionKey": "hold_pose",
    "frameIndex": 24,
    "holdingAt": "2026-08-02T10:00:01.000Z"
  }
}
```

## runtime.playback.action_completed

动作正常完成。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.playback.action_completed",
  "messageId": "event_xxx",
  "correlationId": "cmd_xxx",
  "sequence": 304,
  "payload": {
    "commandId": "cmd_xxx",
    "playbackInstanceId": "pb_xxx",
    "actionKey": "wave",
    "completionReason": "natural_end",
    "cycleCount": 1,
    "returnTarget": "idle",
    "completedAt": "2026-08-02T10:00:02.500Z",
    "playedMs": 2500
  }
}
```

## runtime.playback.action_interrupted

动作被中断。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.playback.action_interrupted",
  "messageId": "event_xxx",
  "correlationId": "cmd_xxx",
  "sequence": 305,
  "payload": {
    "commandId": "cmd_xxx",
    "playbackInstanceId": "pb_xxx",
    "actionKey": "wave",
    "reason": "replaced_by_command",
    "replacedByCommandId": "cmd_yyy",
    "replacedByPlaybackInstanceId": "pb_yyy",
    "interruptedAt": "2026-08-02T10:00:01.800Z",
    "playedMs": 1800
  }
}
```

中断原因枚举：
- `higher_priority_action` - 更高优先级动作替换
- `replaced_by_command` - 被显式命令替换
- `package_switch` - 包切换
- `resource_failure` - 资源加载失败
- `window_destroyed` - 窗口销毁
- `runtime_reconnect` - Runtime 重连
- `user_disable` - 用户禁用
- `max_duration_reached` - 到达最大播放时长
- `command_cancelled` - 命令被取消

## runtime.playback.action_failed

动作播放失败。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.playback.action_failed",
  "messageId": "event_xxx",
  "correlationId": "cmd_xxx",
  "sequence": 306,
  "payload": {
    "commandId": "cmd_xxx",
    "playbackInstanceId": "pb_xxx",
    "actionKey": "wave",
    "errorCode": "PLAYBACK_RESOURCE_LOAD_FAILED",
    "errorMessage": "Frame resource not found",
    "frameIndex": 15,
    "resourcePathHash": "sha256...",
    "recoverable": false,
    "failedAt": "2026-08-02T10:00:01.200Z"
  }
}
```

## Command 完成映射

| 事件 | Command 状态 | 说明 |
|------|--------------|------|
| action_started + policy=on_started | completed | 命令完成 |
| action_completed | completed | 命令完成 |
| action_interrupted | cancelled | 命令取消 |
| action_failed | failed_terminal | 命令失败 |

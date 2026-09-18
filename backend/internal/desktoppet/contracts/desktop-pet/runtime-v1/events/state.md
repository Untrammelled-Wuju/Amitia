# State 事件

## runtime.state.desired_applied

Runtime 完整应用 Desired 状态后发送。触发对应 SyncDesiredState 命令完成。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.state.desired_applied",
  "messageId": "event_xxx",
  "sequence": 601,
  "payload": {
    "desiredRevision": 45,
    "desiredHash": "sha256...",
    "settingsRevision": 12,
    "installationId": "install_xxx",
    "releaseId": "release_xxx",
    "actualStateHash": "sha256...",
    "appliedAt": "2026-08-02T10:00:05.000Z"
  }
}
```

## runtime.state.desired_rejected

Runtime 无法应用 Desired 状态。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.state.desired_rejected",
  "messageId": "event_xxx",
  "sequence": 602,
  "payload": {
    "desiredRevision": 45,
    "errorCode": "release_hash_mismatch",
    "errorMessage": "Release content hash does not match",
    "rejectedAt": "2026-08-02T10:00:03.000Z"
  }
}
```

错误码：
- `release_missing` - Release 不存在
- `release_hash_mismatch` - Release Hash 不匹配
- `manifest_invalid` - Manifest 无效
- `runtime_incompatible` - Runtime 版本不兼容
- `resource_load_failed` - 资源加载失败
- `renderer_start_failed` - Renderer 启动失败
- `settings_invalid` - 设置无效

## runtime.state.snapshot

Runtime 重连后主动发送的完整状态快照。后端用它重建 ActualState。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.state.snapshot",
  "messageType": "state_snapshot",
  "messageId": "event_xxx",
  "sequence": 603,
  "payload": {
    "connectionGeneration": 13,
    "eventSequence": 603,
    "actualStateHash": "sha256...",
    "instanceStatus": "ready",
    "windowStatus": "visible",
    "rendererStatus": "runtime_ready",
    "playbackStatus": "playing",
    "appliedDesiredRevision": 45,
    "appliedDesiredHash": "sha256...",
    "appliedSettingsRevision": 12,
    "installationId": "install_xxx",
    "petId": "pet_xxx",
    "releaseId": "release_xxx",
    "visible": true,
    "positionX": 120,
    "positionY": 240,
    "screenId": "display-primary",
    "windowWidth": 180,
    "windowHeight": 200,
    "scale": 1.0,
    "stableActionKey": "idle",
    "currentActionKey": "wave",
    "playbackInstanceId": "pb_xxx",
    "currentCommandId": "cmd_xxx",
    "lastProcessedCommandSequence": 125,
    "capturedAt": "2026-08-02T10:00:10.000Z"
  }
}
```

## 注意

- `desired_applied` 必须包含 `desiredHash` 和 `desiredRevision` 用于后端验证
- `desired_rejected` 必须携带结构化 `errorCode`
- Snapshot 必须有 `connectionGeneration` 和 `eventSequence` 用于乱序保护
- Snapshot 的 `visible` 必须与 `windowStatus` 一致；窗口几何必须报告实际物理值，禁止用 Desired 值伪造 Actual State

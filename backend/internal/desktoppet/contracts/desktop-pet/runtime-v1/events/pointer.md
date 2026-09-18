# Pointer 事件

## runtime.pointer.clicked

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.pointer.clicked",
  "messageId": "event_xxx",
  "sequence": 401,
  "payload": {
    "installationId": "install_xxx",
    "releaseId": "release_xxx",
    "actionKey": "idle",
    "playbackInstanceId": "pb_xxx",
    "canvasX": 120,
    "canvasY": 200,
    "occurredAt": "2026-08-02T10:00:00.000Z"
  }
}
```

## runtime.pointer.double_clicked

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.pointer.double_clicked",
  "messageId": "event_xxx",
  "sequence": 402,
  "payload": {
    "installationId": "install_xxx",
    "releaseId": "release_xxx",
    "actionKey": "idle",
    "playbackInstanceId": "pb_xxx",
    "canvasX": 120,
    "canvasY": 200,
    "occurredAt": "2026-08-02T10:00:00.500Z"
  }
}
```

## runtime.pointer.hovered

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.pointer.hovered",
  "messageId": "event_xxx",
  "sequence": 403,
  "payload": {
    "installationId": "install_xxx",
    "releaseId": "release_xxx",
    "actionKey": "idle",
    "playbackInstanceId": "pb_xxx",
    "canvasX": 120,
    "canvasY": 200,
    "occurredAt": "2026-08-02T10:00:00.100Z"
  }
}
```

## 注意

Pointer 事件不带 commandId，不关联到任何 RuntimeCommand，仅更新 ActualState 和触发 Behavior。

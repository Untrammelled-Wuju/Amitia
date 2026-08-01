# Drag 事件

## runtime.drag.started

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.drag.started",
  "messageId": "event_xxx",
  "sequence": 501,
  "payload": {
    "installationId": "install_xxx",
    "releaseId": "release_xxx",
    "actionKey": "idle",
    "playbackInstanceId": "pb_xxx",
    "dragId": "drag_xxx",
    "startX": 100,
    "startY": 150,
    "startedAt": "2026-08-02T10:00:00.000Z"
  }
}
```

## runtime.drag.moved

高频事件，可选择采样或仅实时转发。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.drag.moved",
  "messageId": "event_xxx",
  "sequence": 502,
  "payload": {
    "dragId": "drag_xxx",
    "currentX": 150,
    "currentY": 180,
    "deltaX": 5,
    "deltaY": 3,
    "occurredAt": "2026-08-02T10:00:00.033Z"
  }
}
```

## runtime.drag.completed

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.drag.completed",
  "messageId": "event_xxx",
  "sequence": 503,
  "payload": {
    "dragId": "drag_xxx",
    "endX": 200,
    "endY": 250,
    "completedAt": "2026-08-02T10:00:02.000Z"
  }
}
```

## runtime.drag.cancelled

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.drag.cancelled",
  "messageId": "event_xxx",
  "sequence": 504,
  "payload": {
    "dragId": "drag_xxx",
    "cancelledAt": "2026-08-02T10:00:01.500Z"
  }
}
```

## 持久化策略

- started / completed / cancelled: 持久化
- moved: 可采样或仅实时转发

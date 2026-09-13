# Health 事件

## runtime.health.changed

Runtime 健康状态变化时发送。

```json
{
  "messageType": "runtime_event",
  "messageName": "runtime.health.changed",
  "messageId": "event_xxx",
  "sequence": 701,
  "payload": {
    "previousStatus": "online_no_pet",
    "currentStatus": "healthy",
    "reason": "pet_loaded",
    "changedAt": "2026-08-02T10:00:03.000Z"
  }
}
```

## Health 状态枚举

| 状态 | 说明 |
|------|------|
| `offline` | 离线 |
| `online_no_pet` | 在线但无桌宠实例 |
| `syncing` | 同步中 |
| `healthy` | 健康（桌宠实例稳定运行） |
| `degraded` | 降级（Renderer 不响应/部分功能异常） |
| `failed` | 失败（不可恢复错误） |

## Heartbeat 与 Health 的关系

- Heartbeat 仅更新 Runtime 在线状态
- Heartbeat 不直接设 healthy
- Healthy 状态需要满足：
  1. Runtime 在线
  2. Pet 实例存在
  3. Desired 已应用（或正在同步）
  4. Renderer 响应正常

## 状态转换

```
offline → online_no_pet → syncing → healthy
                                      ↓
                                   degraded → failed
                                      ↓
                                   healthy (恢复)
```

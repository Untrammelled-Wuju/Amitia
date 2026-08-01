# Runtime Protocol V2 错误码

## 协议级错误

| 错误码 | 说明 | 是否可恢复 |
|--------|------|------------|
| `RUNTIME_PROTOCOL_UNSUPPORTED` | Envelope 版本不支持 | 否 |
| `RUNTIME_ENVELOPE_INVALID` | Envelope 校验失败（必填字段缺失/格式错误） | 否 |
| `RUNTIME_PAYLOAD_HASH_MISMATCH` | Payload 哈希校验失败 | 否 |
| `RUNTIME_PAYLOAD_SCHEMA_UNSUPPORTED` | Payload SchemaVersion 不支持 | 否 |

## Session 错误

| 错误码 | 说明 | 是否可恢复 |
|--------|------|------------|
| `RUNTIME_SESSION_STALE` | Session 已被替代 | 否 |
| `RUNTIME_CONNECTION_SUPERSEDED` | 连接已被新连接替代 | 否 |
| `RUNTIME_SEQUENCE_STALE` | 事件/命令序列号过期 | 否 |

## 命令错误

| 错误码 | 说明 | 是否可恢复 |
|--------|------|------------|
| `RUNTIME_COMMAND_UNSUPPORTED` | Runtime 不支持该命令类型 | 否 |
| `RUNTIME_COMMAND_IDEMPOTENCY_CONFLICT` | IdempotencyKey 相同但 PayloadHash 不同 | 否 |
| `RUNTIME_COMMAND_EXPIRED` | 命令已超过 ExpiresAt | 否 |
| `RUNTIME_COMMAND_SUPERSEDED` | 命令已被更高 Revision 的命令替代 | 否 |
| `RUNTIME_COMMAND_CAPABILITY_MISSING` | Runtime 缺少执行命令所需的能力 | 否 |
| `RUNTIME_OFFLINE` | Runtime 离线 | 是（重连后可恢复） |

## 超时错误

| 错误码 | 说明 | 是否可恢复 |
|--------|------|------------|
| `RUNTIME_ACCEPT_TIMEOUT` | Runtime 未在超时内返回 runtime_accepted | 是 |
| `RENDERER_ACCEPT_TIMEOUT` | Renderer 未在超时内返回 command_accepted | 是 |
| `PLAYBACK_START_TIMEOUT` | Playback 未在超时内开始 | 是 |
| `PLAYBACK_COMPLETION_TIMEOUT` | Playback 未在预期时间内完成 | 是 |

## Playback 错误

| 错误码 | 说明 | 是否可恢复 |
|--------|------|------------|
| `PLAYBACK_ACTION_NOT_FOUND` | ActionKey 不存在或未加载 | 否 |
| `PLAYBACK_RESOURCE_LOAD_FAILED` | 资源加载失败 | 否 |
| `PLAYBACK_RENDERER_CRASHED` | Renderer 崩溃 | 是 |

## Desired Sync 错误

| 错误码 | 说明 | 是否可恢复 |
|--------|------|------------|
| `RUNTIME_DESIRED_HASH_MISMATCH` | Desired Hash 与实际 Payload 不匹配 | 否 |
| `RUNTIME_RELEASE_HASH_MISMATCH` | Release Hash 不匹配 | 否 |
| `RUNTIME_ACTUAL_STATE_STALE` | Actual State 落后于 Desired | 是 |

## 连接级错误

| 错误码 | 说明 | 是否可恢复 |
|--------|------|------------|
| `RUNTIME_DISABLED` | 后端 Runtime 服务已禁用 | 否 |
| `RUNTIME_UNAUTHORIZED` | Token 无效 | 否 |
| `RUNTIME_FORBIDDEN_ORIGIN` | Origin 不被允许 | 否 |
| `RUNTIME_PROTOCOL_INCOMPATIBLE` | 协议版本不兼容 | 否 |
| `RUNTIME_HEARTBEAT_TIMEOUT` | 心跳超时 | 是 |

## 错误选择指南

### 命令被拒绝时

- Runtime 未注册该命令 → `RUNTIME_COMMAND_UNSUPPORTED`
- Runtime 能力不足 → `RUNTIME_COMMAND_CAPABILITY_MISSING`
- 命令内容过期 → `RUNTIME_COMMAND_EXPIRED`
- Revision 冲突 → `RUNTIME_COMMAND_SUPERSEDED`
- 内容冲突 → `RUNTIME_COMMAND_IDEMPOTENCY_CONFLICT`

### Playback 失败时

- Action 未找到 → `PLAYBACK_ACTION_NOT_FOUND`
- 资源加载失败 → `PLAYBACK_RESOURCE_LOAD_FAILED`
- Renderer 崩溃 → `PLAYBACK_RENDERER_CRASHED`

### Sync 失败时

- Release 不匹配 → `RUNTIME_RELEASE_HASH_MISMATCH`
- 操作失败 → 使用 `runtime.state.desired_rejected` 事件携带具体错误码

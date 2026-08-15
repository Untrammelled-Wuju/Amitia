# Native Bridge Protocol v1

**Status:** FROZEN
**Version:** 1
**Last Updated:** 2026-08-15

## 1. Overview

Native Bridge Protocol v1 定义了 Backend、Flutter、Android (Kotlin)、iOS (Swift) 四端之间的通信契约。

本冻结版本适用于 Stage 2 Closure 全量一次性修复后的所有生产代码路径。

## 2. Wire Format

- 传输层: WebSocket (ws/wss) 或 MethodChannel (Flutter ↔ Native)
- 编码: UTF-8 JSON
- 字段命名: camelCase
- 时间戳: ISO 8601 (RFC 3339) UTC

## 3. Protocol Version

**`protocolVersion` 字段类型为整数 `1`。**

四端必须保持一致:
- Go: `int` value `1`
- Dart: `int` value `1`
- Kotlin: `Int` value `1`
- Swift: `Int` value `1` (不使用 String "1.0")

## 4. Common Field Names

| 字段 | 类型 | 说明 |
|------|------|------|
| `protocolVersion` | int | 协议版本，固定为 1 |
| `requestId` | string | 请求唯一标识，全链路贯穿 |
| `platform` | string | `"android"` 或 `"ios"` |
| `operation` | string | 操作名，格式 `domain.action` |
| `status` | string | `"ok"` 或 `"error"` |
| `result` | object | 成功时的响应数据 |
| `error` | object | 失败时的错误信息 |
| `code` | string | 错误码 |
| `message` | string | 错误描述 |
| `domainCode` | string? | 可选领域错误码 |
| `generation` | uint64 | Relay 连接代数 |
| `payload` | object? | 信封载荷 |

**重要:** `requestId` 全链路统一使用小写 `d` (`requestId`)，不使用 `requestID`。

## 5. Envelope Types

### 5.1 RelayEnvelope

```json
{
  "type": "native_bridge.request|native_bridge.response|native_bridge.event|native_bridge.health",
  "platform": "android|ios",
  "generation": 1,
  "requestId": "req-...",
  "payload": {}
}
```

### 5.2 NativeBridgeRequest (payload)

```json
{
  "protocolVersion": 1,
  "requestId": "req-...",
  "platform": "android|ios",
  "operation": "domain.action",
  "payload": {}
}
```

### 5.3 NativeBridgeResponse (payload)

```json
{
  "protocolVersion": 1,
  "requestId": "req-...",
  "status": "ok|error",
  "result": {},
  "error": {
    "code": "ERROR_CODE",
    "message": "...",
    "domainCode": "..."
  }
}
```

### 5.4 NativeEvent (payload)

```json
{
  "domain": "string",
  "event": "string",
  "timestamp": "2026-08-15T12:00:00Z",
  "data": {}
}
```

### 5.5 HealthEnvelope (payload)

```json
{
  "generation": 1,
  "ready": true,
  "foreground": true,
  "platform": "android|ios",
  "capabilities": { "operation": true }
}
```

## 6. Error Codes

| Code | 说明 |
|------|------|
| `PROVIDER_UNAVAILABLE` | 提供者不可用 |
| `AUTHORIZATION_DENIED` | 授权拒绝 |
| `USER_ACTION_REQUIRED` | 需要用户操作 |
| `PLATFORM_NOT_SUPPORTED` | 平台不支持 |
| `BRIDGE_DISCONNECTED` | Bridge 断开 |
| `BRIDGE_TIMEOUT` | Bridge 超时 |
| `BRIDGE_INVALID_RESPONSE` | Bridge 响应无效 |
| `OPERATION_NOT_SUPPORTED` | 操作不支持 |
| `INVALID_ARGUMENT` | 参数无效 |
| `INTERNAL_ERROR` | 内部错误 |
| `TIMEOUT` | 超时 |
| `INVALID_RESPONSE` | 响应无效 |
| `STALE_NATIVE_REFERENCE` | 过期 Native 引用 |
| `HOST_UNAVAILABLE` | 主机不可用 |

## 7. Operation Name Format

`{domain}.{action}`

Domains:
- `android.*` - Android 平台操作
- `ios.*` - iOS 平台操作
- `media.*` - 媒体操作
- `file.*` - 文件操作
- `share.*` - 分享操作
- `shortcut.*` - 快捷操作
- `alarm.*` - 闹钟操作
- `bluetooth.*` - 蓝牙操作
- `homekit.*` - HomeKit 操作
- `background.*` - 后台任务操作

## 8. Binary Content

跨层传输二进制数据时使用 Base64 编码:

- Wire 字段名: `contentBase64`
- Go 端 DTO 可使用 `[]byte Content`，由 mapper 负责 Base64 转换
- 大文件使用 chunk/stream，不允许一次性读入内存

## 9. Failure Semantics (fail-closed)

- 未真实执行不得返回 `status=ok`
- 未真实执行不得返回 `executed=true`、`confirmed=true`、`written=true`、`exported=true`
- 底层不可用时必须返回 `unavailable`/`unsupported`
- 权限不足必须返回明确的 permission 错误
- 超时必须真正终止底层工作

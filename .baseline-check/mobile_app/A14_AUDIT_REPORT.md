# 步骤14审计报告：统一 BackendTransport，闭合 HTTP / Streaming / WebSocket 同源连接链

## 1. 目标

统一 HTTP/Streaming/WebSocket 传输层，使其共用同一个 `BackendConnectionConfig(generation)`：
- 共用同一个 Loopback Endpoint
- 共用同一个 Local Token Header（X-Amitia-Local-Token）
- 共用同一个 Generation 过期门闸
- 闭合断点：`BackendWebSocketClient` 曾绕过 Local Token Header 注入

## 2. 审计范围

### 2.1 静态审计

| 审计项 | 结果 | 备注 |
|--------|------|------|
| WebSocket直接连接断点 | **已修复** | `WebSocketChannel.connect(uri)` → `WebSocket.connect(uri, headers: {...})` |
| 直接HTTP客户端 | 通过 | 全部集中在 Transport 层（`BackendHttpClient`） |
| 硬编码端点 | 通过 | 仅出现在 Validation 代码中 |
| Local Token Header所有者 | 通过 | 仅 Transport 层注入 |
| Credential.reveal使用 | 通过 | 仅 Transport 层使用 |
| Transport本地Generation | 通过 | 0处自行生成，全部来自 config |
| Token URL | 通过 | 0处URL携带Token |

### 2.2 传输链路审计

| 链路 | Token注入 | Generation门闸 | 状态 |
|------|-----------|----------------|------|
| HTTP | ✅ `BackendHttpClient` L68-69 | ✅ Provider重建 | **已闭合** |
| WebSocket | ✅ `BackendWebSocketClient` L130-134 | ✅ Provider重建 | **已闭合** |
| Streaming | ✅ 复用HTTP Transport | ✅ Provider重建 | **已闭合** |

## 3. 关键修改

### 3.1 `backend_websocket_client.dart`

**修改前**：
```dart
final channel = WebSocketChannel.connect(uri);  // 无headers支持
```

**修改后**：
```dart
final token = _config.credential.revealForTransport();
final headers = <String, String>{
  BackendAuthHeader.localToken: token,
  'User-Agent': 'Amitia-Mobile',
};

final webSocket = await WebSocket.connect(
  uri.toString(),
  headers: headers,
);
final channel = IOWebSocketChannel(webSocket);
```

**核心改进**：
1. 使用 `dart:io` 的 `WebSocket.connect()` 替代 `WebSocketChannel.connect()` —— 支持请求头注入
2. 从 `_config.credential.revealForTransport()` 获取 token —— 与 HTTP Client 同源
3. Token 注入使用 `BackendAuthHeader.localToken` 常量 —— 保持一致性
4. `await` WebSocket 握手完成后再返回 session —— 确保连接建立（失败立即报错）
5. 使用 `IOWebSocketChannel(webSocket)` 包装已建立的 WebSocket —— 适配现有 Channel 接口

### 3.2 `pubspec.yaml`

移除未使用的 `web_socket: ^1.0.1` 直射依赖（该包为 W3C Web API 风格，不支持 headers）。保留 `web_socket_channel: ^3.0.0`。

### 3.3 `fake_backend_server.dart`

增加 WebSocket Upgrade 支持（`WebSocketTransformer.upgrade`），以支持：
- WebSocket Token Header 验证测试
- WebSocket Echo 行为测试

## 4. Generation 门闸逻辑

### 4.1 Provider 层（已存在）

`BackendTransportNotifier.build()`：
```dart
if (config.generation != _currentGeneration || _current == null) {
  _recreateTransport(config);  // 关闭旧 Transport + 创建新 Transport
}
```

`_recreateTransport()`:
```dart
_closeCurrentIfNeeded();  // 关闭旧 WS + 取消旧 HTTP 流式请求
_currentGeneration = config.generation;
_current = DefaultBackendTransport.create(config);
```

### 4.2 Transport 关闭链

`DefaultBackendTransport.close()` →
- `http.close()` —— 关闭HTTP连接，取消所有进行中的流式请求
- `webSocket.close()` —— 关闭所有WS会话

`BackendWebSocketClient.close()` →
- 遍历所有 `_sessions`，调用 `session.close()` → `_channel.sink.close()`

### 4.3 后端连接状态流

`BackendConnectionAvailability` 约束：
- `BackendConnectionAvailable` → Transport 可用
- `BackendConnectionUnavailable/Resolving` → Transport 关闭（`_closeCurrentIfNeeded`）

此机制自然覆盖 STOPPING/STOPPED/FAILED 状态转移。

## 5. 测试覆盖

### 5.1 新增测试文件

| 文件 | 用例数 | 覆盖点 |
|------|--------|--------|
| `backend_websocket_client_test.dart` | 9 | Token注入、认证失败、Echo通信、Generation传递、关闭幂等、关闭后connect抛错 |
| `backend_websocket_generation_test.dart` | 3 | Session承载generation、多generation Transport独立、Transport关闭连带所有WS会话 |

### 5.2 关键测试项

- **Token注入验证**：`X-Amitia-Local-Token` 正确出现在 WS 请求头
- **认证失败回退**：无效 Token 时 `WebSocket.connect` 抛出异常（HTTP 401 未升级为 WS）
- **User-Agent携带**：`Amitia-Mobile` 显式设置
- **Generation传播**：Session 内 `generation` 字段与 config 一致
- **关闭幂等**：`close()` 可多次调用不报错
- **关闭后保护**：`close()` 后再 `connect()` 抛出 `TransportClosed`

### 5.3 回归测试

所有既有 HTTP 测试（17用例）+ 新增 WS 测试（12用例）+ 既有连通性测试（3用例）= **32用例全部通过**。

```
00:01 +32: All tests passed!
```

## 6. 闭合验证矩阵

| 要求 | HTTP | WebSocket | Streaming(HTTP) | 证据 |
|------|------|-----------|-----------------|------|
| 共用 Loopback Endpoint | ✅ | ✅ | ✅ | 同一 `BackendConnectionConfig.endpoint` |
| 共用 Local Token Header | ✅ | ✅ | ✅ | 同一 `_config.credential.revealForTransport()` |
| 共用 Generation | ✅ | ✅ | ✅ | 同一 `_config.generation`，Session 携带 |
| 同源 Transport 工厂 | ✅ | ✅ | ✅ | `DefaultBackendTransport.create(config)` |
| 过期门闸 | ✅ | ✅ | ✅ | `BackendTransportNotifier._recreateTransport` |
| 关闭旧WS/Stream | ✅ | ✅ | ✅ | `DefaultBackendTransport.close` 级联 |
| 禁止 Token URL | ✅ | ✅ | ✅ | URI 构建器无 Token 参数 |
| 禁止裸连接 | ✅ | ✅ | ✅ | `WebSocket.connect(headers: {...})` |

## 7. 未实现（按步骤范围排除）

- 业务 Repository 决策逻辑：不在本步骤范围内
- `businessAvailable` 接线：不在本步骤范围内
- 真实业务 E2E：由后续步骤覆盖

## 8. 结论

步骤14完成。HTTP/Streaming/WebSocket 三条传输链路已统一消费同一个 `BackendConnectionConfig(generation)`，共用 Loopback Endpoint、Local Token Header、Generation 过期门闸。

断点 `BackendWebSocketClient` 绕过 Local Token Header 注入已闭合。WebSocket 握手现与 HTTP 同源：从同一 `credential.revealForTransport()` 获取 token，通过同一 `BackendAuthHeader.localToken` 头注入，由同一 Provider 重建机制处理 Generation 切换。

**测试通过率**：32/32 = 100%

**静态审计**：全部通过，无新增违规

**风险评估**：低。修改集中在 WebSocket 连接建立层，已通过连接建立、认证失败、关闭传播等边界用例覆盖。

# Amitia API 映射文档（Phase 4 / Task 4.5）

> 来源：`docs/android/01-current-system-audit.md` 第 16-17 节后端真实路由 + `android/core/.../network/api/*` Retrofit 接口
> 生成时间：2026-07-26
> 适用范围：Android 客户端 Phase 4 连接层（RuntimeEndpointProvider / AmitiaApiClient / Repository / SseClient / WsClient）
> 后端基线：Go 后端 HTTP 端口 `18899`、Qdrant `19178`、SurrealDB `18000`，统一监听 `127.0.0.1`

---

## 1. API 映射表（按模块）

### 1.1 健康检查（Health）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| GET | `/api/health` | 无 | `HealthResponse` | `ConnectionManager.testConnection()` | `HealthApi.health()` |
| GET | `/api/health/circuit-breakers` | 无 | `HealthResponse` | （预留） | `HealthApi.circuitBreakers()` |
| GET | `/api/version` | 无 | `HealthResponse` | （预留） | `HealthApi.version()` |
| GET | `/api/health/data-lifecycle` | 无 | `HealthResponse` | （预留，Phase 5+） | 未实现 |
| GET | `/api/health/reconciliation` | 无 | `HealthResponse` | （预留，Phase 5+） | 未实现 |

### 1.2 角色（Character）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| GET | `/api/characters` | 无（`page`、`pageSize` query） | `List<CharacterDto>` | `CharacterRepository.list(page, pageSize)` | `CharacterApi.listCharacters()` |
| GET | `/api/characters/{id}` | 无 | `CharacterDto` | `CharacterRepository.get(id)` | `CharacterApi.getCharacter(id)` |
| GET | `/api/characters/current` | 无 | `CharacterDto` | `CharacterRepository.getCurrent()` | `CharacterApi.getCurrentCharacter()` |
| POST | `/api/characters` | `CharacterCreateRequest` | `CharacterDto` | `CharacterRepository.create(request)` | `CharacterApi.createCharacter()` |
| PUT | `/api/characters/{id}` | `CharacterUpdateRequest` | `CharacterDto` | `CharacterRepository.update(id, request)` | `CharacterApi.updateCharacter()` |
| DELETE | `/api/characters/{id}` | 无 | 无（204） | `CharacterRepository.delete(id)` | `CharacterApi.deleteCharacter()` |
| POST | `/api/characters/switch` | `CharacterSwitchRequest` | `CharacterDto` | `CharacterRepository.switchCurrent(id)` | `CharacterApi.switchCurrent()` |

### 1.3 聊天与会话（Chat / Conversation）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| POST | `/api/web-chat/send-stream` | `SendStreamRequest` | `text/event-stream`（SSE） | `ChatRepository.sendStream(request): Flow<SseEvent>` | `ChatApi.sendStream(): ResponseBody`（仅 fallback） |
| POST | `/api/web-chat/send` | `SendStreamRequest` | `MessageDto` | `ChatRepository.send(request)` | `ChatApi.send()` |
| GET | `/api/web-chat/conversations` | 无（`page`、`pageSize` query） | `ConversationListResponse` | `ChatRepository.listConversations(page, pageSize)` | `ChatApi.listConversations()` |
| POST | `/api/web-chat/conversations` | `ConversationCreateRequest` | `ConversationDto` | `ChatRepository.createConversation(...)` | `ChatApi.createConversation()` |
| GET | `/api/web-chat/conversations/{id}/messages` | 无（`page`、`pageSize` query） | `MessageListResponse` | `ChatRepository.getHistory(id, page, pageSize)` | `ChatApi.listMessages()` |
| DELETE | `/api/web-chat/conversations/{id}` | 无 | 无（204） | `ChatRepository.deleteConversation(id)` | `ChatApi.deleteConversation()` |
| DELETE | `/api/web-chat/messages/{id}` | 无 | 无（204） | `ChatRepository.deleteMessage(id)` | `ChatApi.deleteMessage()` |
| POST | `/api/web-chat/messages/{id}/retry` | 空 | `text/event-stream`（SSE） | `ChatRepository.retryMessage(id): Flow<SseEvent>` | `ChatApi.retryMessage(): ResponseBody` |

### 1.4 记忆（Memory）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| GET | `/api/memory` | 无（`page`、`pageSize`、`characterId`、`type` query） | `List<MemoryDto>` | `MemoryRepository.list(...)` | `MemoryApi.listMemories()` |
| GET | `/api/memory/{id}` | 无 | `MemoryDto` | `MemoryRepository.get(id)` | `MemoryApi.getMemory()` |
| POST | `/api/memory` | `MemoryCreateRequest` | `MemoryDto` | `MemoryRepository.create(request)` | `MemoryApi.createMemory()` |
| PUT | `/api/memory/{id}` | `MemoryUpdateRequest` | `MemoryDto` | `MemoryRepository.update(id, request)` | `MemoryApi.updateMemory()` |
| DELETE | `/api/memory/{id}` | 无 | 无（204） | `MemoryRepository.delete(id)` | `MemoryApi.deleteMemory()` |
| POST | `/api/memory/search` | `MemorySearchRequest` | `List<MemoryDto>` | `MemoryRepository.search(request)` | `MemoryApi.search()` |
| GET | `/api/memory/timeline` | 无（`start`、`end`、`limit` query） | `List<MemoryTimelineItem>` | `MemoryRepository.getTimeline(...)` | `MemoryApi.getTimeline()` |
| GET | `/api/memory/graph` | 无（`characterId`、`depth` query） | `MemoryGraphDto` | `MemoryRepository.getGraph(...)` | `MemoryApi.getGraph()` |

### 1.5 模型（Model）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| GET | `/api/models` | 无（`type`、`provider` query） | `List<ModelDto>` | `ModelRepository.list(type, provider)` | `ModelApi.listModels()` |
| GET | `/api/models/{id}` | 无 | `ModelDto` | `ModelRepository.get(id)` | `ModelApi.getModel()` |
| POST | `/api/models` | `ModelDto` | `ModelDto` | `ModelRepository.create(model)` | `ModelApi.createModel()` |
| PUT | `/api/models/{id}` | `ModelDto` | `ModelDto` | `ModelRepository.update(id, model)` | `ModelApi.updateModel()` |
| DELETE | `/api/models/{id}` | 无 | 无（204） | `ModelRepository.delete(id)` | `ModelApi.deleteModel()` |
| GET | `/api/models/config` | 无 | `ModelConfigDto` | `ModelRepository.getConfig()` | `ModelApi.getConfig()` |
| PUT | `/api/models/config` | `ModelConfigUpdateRequest` | `ModelConfigDto` | `ModelRepository.updateConfig(request)` | `ModelApi.updateConfig()` |
| POST | `/api/models/{id}/download` | 无 | `ResponseBody`（二进制流） | `ModelRepository`（预留，Phase 5） | `ModelApi.downloadModel()` |

### 1.6 渠道（Channel）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| GET | `/api/channels` | 无 | `List<ChannelDto>` | `ChannelRepository.list()` | `ChannelApi.listChannels()` |
| GET | `/api/channels/status` | 无 | `ChannelStatusDto` | `ChannelRepository.getStatus()` | `ChannelApi.getStatus()` |
| POST | `/api/channels/bind` | `ChannelBindRequest` | `ChannelDto` | `ChannelRepository.bind(...)` | `ChannelApi.bind()` |
| POST | `/api/channels/unbind` | `ChannelBindRequest` | `ChannelDto` | `ChannelRepository.unbind(...)` | `ChannelApi.unbind()` |

### 1.7 主动消息（Proactive）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| GET | `/api/proactive/messages` | 无（`page`、`pageSize`、`onlyUnread`、`characterId` query） | `ProactiveListResponse` | `ProactiveRepository.list(...)` | `ProactiveApi.listProactive()` |
| POST | `/api/proactive/messages/read` | `ProactiveMarkReadRequest` | 无（204） | `ProactiveRepository.markRead(ids)` | `ProactiveApi.markRead()` |
| GET | `/api/proactive-sse` | 无 | `text/event-stream`（SSE） | （预留，Phase 5+） | 未实现（通过 `SseClient.connect(...)` 直接消费） |

### 1.8 TTS（语音合成）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| POST | `/api/tts/synthesize` | `TtsRequest` | `TtsResponse` | `TtsRepository.synthesize(request)` | `TtsApi.synthesize()` |
| GET | `/api/tts/voices` | 无 | `List<TtsVoiceDto>` | `TtsRepository.getVoices()` | `TtsApi.getVoices()` |

### 1.9 文件上传（Upload）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| POST | `/api/upload` | `multipart/form-data`（`file`、`type` query） | `UploadResponse` | `UploadRepository.uploadImage(file)` | `UploadApi.upload()` |
| POST | `/api/asr/upload` | `multipart/form-data`（`file`） | `UploadResponse` | `UploadRepository.uploadAudio(file)` | `UploadApi.uploadAudio()` |

### 1.10 认证（Auth）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| POST | `/api/auth/login` | `LoginRequest` | `TokenResponse` | （预留，Phase 6） | 未实现 |
| POST | `/api/auth/logout` | 无 | 无（204） | （预留，Phase 6） | 未实现 |
| POST | `/api/auth/refresh` | `RefreshRequest` | `TokenResponse` | （预留，Phase 6） | 未实现 |
| GET | `/api/auth/profile` | 无 | `UserProfile` | （预留，Phase 6） | 未实现 |

### 1.11 运行时（Runtime）

| HTTP 方法 | 路径 | 请求体 | 响应体 | Android Repository 方法 | Android ApiClient service |
|---|---|---|---|---|---|
| GET | `/api/runtime/status` | 无 | `RuntimeStatus` | （由 `runtime` 模块管理，不在 core/network） | 未实现 |
| POST | `/api/runtime/start` | 无 | `Ack` | 同上 | 未实现 |
| POST | `/api/runtime/stop` | 无 | `Ack` | 同上 | 未实现 |
| POST | `/api/runtime/restart` | 无 | `Ack` | 同上 | 未实现 |
| GET | `/api/runtime/logs` | 无 | `LogChunk` | 同上 | 未实现 |
| GET | `/api/runtime/metrics` | 无 | `Metrics` | 同上 | 未实现 |

### 1.12 静态资源路由

| 路径 | 物理目录 | Android 用途 |
|---|---|---|
| `/audio` | `./data/tts_cache` | TTS 音频回放 |
| `/exports` | `./data/exports` | 导出文件下载 |
| `/voice` | `./data/voice_msg` | 语音消息回放 |
| `/images` | `./data/images` | 图片消息展示 |
| `/videos` | `./data/videos` | 视频消息展示 |
| `/avatars` | `./data/avatars` | 角色头像展示 |
| `/emote-assets` | `<DataDir>/emotes` | 表情包资源 |

> 静态资源 URL 通过 `RuntimeEndpoint.baseUrl()` 拼接得到绝对地址。

---

## 2. SSE 流式协议字段对齐表

### 2.1 `POST /api/web-chat/send-stream` 主聊天流

| SSE 事件 | 后端字段 | Android `SseEvent.event` | Android `SseEvent.data`（JSON） | 处理方 |
|---|---|---|---|---|
| `message_start` | `conversationId`、`messageId`、`role`、`channel`、`createdAt` | `SseEvent.EVENT_MESSAGE_START` = `"message_start"` | JSON 字符串，解析为 `MessageDto`（部分字段） | `ChatRepository.sendStream` 下游 |
| `token` | `id`、`conversationId`、`role`、`content`、`createdAt` | `SseEvent.EVENT_TOKEN` = `"token"` | JSON 字符串，增量 token | `ChatRepository.sendStream` 下游 |
| `voice_audio` | `messageId`、`conversationId`、`role`、`content`、`createdAt`、`audioUrl`、`duration` | `SseEvent.EVENT_VOICE_AUDIO` = `"voice_audio"` | JSON 字符串，含 TTS 音频 URL | `ChatRepository.sendStream` 下游 |
| `message_end` | `messageId`、`status`、`conversationId`、`finalContentLength` | `SseEvent.EVENT_TERMINAL` = `"message_end"` | JSON 字符串，终止事件，触发 `Flow` 关闭 | `ChatRepository.sendStream` 下游 |
| `error` | 错误描述 | `SseEvent.EVENT_ERROR` = `"error"` | 错误文本 | 触发 `AmitiaApiException.SseDisconnected` |

### 2.2 `GET /api/messages/events` 消息事件总线

| SSE 事件 | 后端字段 | Android `SseEvent.event` | 处理方 |
|---|---|---|---|
| `connected` | 心跳握手 | `"connected"` | `SseClient.connect()` 下游 |
| `message_created` | `conversationId`、`messageId`、`channel`、`direction`、`role`、`content`、`createdAt`、`status`、`data` | `SseEvent.EVENT_MESSAGE_CREATED` = `"message_created"` | Phase 5+ |
| `message_updated` | 同上 | `SseEvent.EVENT_MESSAGE_UPDATED` = `"message_updated"` | Phase 5+ |
| `conversation_updated` | `conversationId`、`lastMessageAt`、`unreadCount` | `SseEvent.EVENT_CONVERSATION_UPDATED` = `"conversation_updated"` | Phase 5+ |
| `ping` | 心跳 | `SseEvent.EVENT_PING` = `"ping"` | `SseClient` 内部 |

### 2.3 `GET /api/proactive-sse` 主动消息流

| SSE 事件 | 后端字段 | Android `SseEvent.event` | 处理方 |
|---|---|---|---|
| `proactive_message` | `id`、`conversationId`、`characterId`、`channel`、`content`、`status`、`scheduledAt`、`sentAt` | `SseEvent.EVENT_PROACTIVE_MESSAGE` = `"proactive_message"` | Phase 5+ |
| `ping` | 心跳 | `SseEvent.EVENT_PING` = `"ping"` | `SseClient` 内部 |

### 2.4 SSE 协议解析规则

Android `SseParser` 严格按以下规则解析：

1. 事件块之间以空行 `\n\n` 分隔
2. 每行字段格式：`field: value`（冒号后可选 1 个空格）
3. 支持字段：`event`、`data`、`id`（`retry` 字段忽略）
4. 同一事件内多行 `data:` 会被 `\n` 拼接为最终 `data` 字符串
5. 以 `:` 开头的行视为注释，忽略
6. 缺省 `event` 字段时默认为 `"message"`
7. 仅含注释或空白的块不会产生 `SseEvent`

### 2.5 SSE 重连策略

| 端点 | 重连间隔 | 最大重试 | 引用 |
|---|---|---|---|
| `POST /api/web-chat/send-stream` | 不重连（POST 一次性流） | 0 | `SseClient.connect()` |
| `GET /api/messages/stream` | 3 秒 | 无限 | Phase 5+ 实现 |
| `GET /api/messages/events` | 3 秒 | 无限 | Phase 5+ 实现 |
| `GET /api/proactive-sse` | 5 秒 | 无限 | Phase 5+ 实现 |

> 重连逻辑由调用方在外部用 `Flow.retryWhen { }` 实现，`SseClient` 只负责单次连接的 `Flow<SseEvent>`。

---

## 3. WebSocket 消息结构

### 3.1 协议状态

后端 `realtime` 模块在 `backend/internal/realtime/` 注册（`router.go:75`），WebSocket 实现细节待 Phase 5+ 审计确认。本节定义 Android 客户端预期消息结构。

### 3.2 客户端消息结构

```json
{
  "type": "message",
  "payload": {
    "conversationId": "conv-001",
    "content": "hello"
  },
  "id": "msg-001",
  "timestamp": 1722000000000
}
```

### 3.3 Android 映射

| 字段 | 类型 | Android `WsMessage` 字段 |
|---|---|---|
| `type` | String | `WsMessage.type` |
| `payload` | JsonObject | `WsMessage.payload` |
| `id` | String? | `WsMessage.id` |
| `timestamp` | Long? | `WsMessage.timestamp` |

### 3.4 内置事件类型

| `WsMessage.type` | 含义 | 触发方 |
|---|---|---|
| `open` | WebSocket 已打开 | `WsClient` 内部生成 |
| `message` | 业务消息 | 后端推送或客户端回声 |
| `closing` | 正在关闭 | `WsClient` 内部生成 |
| `closed` | 已关闭 | `WsClient` 内部生成 |
| `error` | 连接异常 | `WsClient` 内部生成 |

---

## 4. 文件上传端点

### 4.1 通用上传

- 端点：`POST /api/upload?type=image`
- 请求：`multipart/form-data`，字段名 `file`
- 响应：`UploadResponse`

```kotlin
@Multipart
@POST("/api/upload")
suspend fun upload(
    @Part file: MultipartBody.Part,
    @Query("type") type: String = "image"
): UploadResponse
```

### 4.2 ASR 音频上传

- 端点：`POST /api/asr/upload`
- 请求：`multipart/form-data`，字段名 `file`
- 响应：`UploadResponse`

```kotlin
@Multipart
@POST("/api/asr/upload")
suspend fun uploadAudio(@Part file: MultipartBody.Part): UploadResponse
```

### 4.3 上传进度回调

`UploadRepository.uploadImage(file, onProgress = ...)` 与 `uploadAudio(file, onProgress = ...)` 提供 `onProgress(sent, total)` 回调。当前实现以 `RequestBody` 写入前后两次通知近似进度；细粒度进度需 Phase 5 通过自定义 `RequestBody` 包装 `Sink` 实现。

---

## 5. 健康检查端点

| 端点 | 用途 | Android 调用方 |
|---|---|---|
| `GET /api/health` | 主健康检查，返回 `HealthResponse`（status、version、uptime、services） | `ConnectionManager.testConnection()` |
| `GET /api/health/circuit-breakers` | 熔断器状态 | `HealthApi.circuitBreakers()`（预留） |
| `GET /api/health/data-lifecycle` | 数据生命周期统计 | Phase 5+ |
| `GET /api/health/reconciliation` | 对账引擎状态 | Phase 5+ |
| `GET /api/version` | 版本信息 | `HealthApi.version()`（预留） |

### 5.1 `ConnectionTestResult` 字段对齐

| Android 字段 | 后端来源 | 备注 |
|---|---|---|
| `success` | HTTP 200 即视为成功 | 由 `Result.isSuccess` 推导 |
| `latencyMs` | 客户端测量 | 从发起请求到收到响应的墙钟时间 |
| `serverVersion` | `HealthResponse.version` | 后端 `config.app.version` |
| `mode` | `RuntimeEndpointProvider.getCurrentMode()` | `LOCAL` 或 `REMOTE` |

---

## 6. 错误码映射表

### 6.1 HTTP 状态码 → `AmitiaApiException`

| HTTP 状态码 | 含义 | Android 异常 | 触发条件 |
|---|---|---|---|
| 200-299 | 成功 | 无 | 正常返回 |
| 401 | 未授权 | `AmitiaApiException.TokenExpired` | token 过期或缺失 |
| 403 | 禁止访问 | `AmitiaApiException.TokenExpired` | 权限不足，按 token 失效处理 |
| 404 | 资源不存在 | `AmitiaApiException.NotFound(path)` | 路由不存在或资源被删 |
| 408 | 请求超时 | `AmitiaApiException.Timeout` | 客户端读超时 |
| 500-599 | 服务器错误 | `AmitiaApiException.ServerError(status, body)` | 后端异常 |
| 其他 4xx | 客户端错误 | `AmitiaApiException.ServerError(status, body)` | 兜底 |

### 6.2 网络/IO 异常 → `AmitiaApiException`

| 原始异常 | Android 异常 | 备注 |
|---|---|---|
| `SocketTimeoutException` | `AmitiaApiException.Timeout` | OkHttp 读/写/连接超时 |
| `UnknownHostException` | `AmitiaApiException.RemoteUnreachable(url)` | DNS 解析失败 |
| `ConnectException` | `AmitiaApiException.RemoteUnreachable(url)` | 端口未监听 |
| `SSLException` | `AmitiaApiException.RemoteUnreachable(url)` | TLS 握手失败 |
| `IOException`（其他） | `AmitiaApiException.NetworkUnavailable` | 网络断开等 |

### 6.3 业务异常

| 异常 | 触发场景 |
|---|---|
| `AmitiaApiException.SseDisconnected(reason)` | SSE 流意外断开 |
| `AmitiaApiException.UploadFailed(reason)` | 文件上传失败 |
| `AmitiaApiException.AudioFailed(reason)` | 音频处理失败（TTS/ASR） |
| `AmitiaApiException.MigrationFailed(reason)` | 数据迁移失败（Phase 5+） |
| `AmitiaApiException.Unknown(throwable)` | 兜底，未知错误 |

### 6.4 拦截器顺序

OkHttp 拦截器按以下顺序执行（`OkHttpModule.provideOkHttpClient`）：

1. `AuthInterceptor` — 注入 `Authorization: Bearer <token>`
2. `LoggingInterceptor` — Debug 详细日志，Release 脱敏（token、password、secret、access_token、refresh_token、authorization、content）
3. `ErrorMappingInterceptor` — HTTP 错误码与 IO 异常映射为 `AmitiaApiException`

---

## 7. 本地/远程模式端点差异

### 7.1 `RuntimeEndpoint` 抽象

| 模式 | `RuntimeEndpoint` 子类 | `baseUrl()` | `wsUrl()` | `authHeader()` |
|---|---|---|---|---|
| 本地 | `RuntimeEndpoint.Local` | `http://127.0.0.1:18899` | `ws://127.0.0.1:18899` | 本地随机 token（`LocalAuthTokenProvider`） |
| 远程 | `RuntimeEndpoint.Remote` | 用户配置 baseUrl（去除尾部 `/`） | baseUrl 协议 `http→ws` / `https→wss` | 远程用户 token |

### 7.2 DataStore 存储

`RuntimeEndpointProvider` 使用 DataStore `amitia_endpoint` 存储以下键：

| Key | 类型 | 取值 |
|---|---|---|
| `mode` | String | `"LOCAL"` 或 `"REMOTE"` |
| `remote_base_url` | String | 远程模式下的 baseUrl |
| `auth_token` | String | 当前模式下的认证 token |

### 7.3 切换流程

```
RuntimeEndpointProvider.switchToLocal(authToken)
  → DataStore.edit { mode=LOCAL, auth_token=authToken, remove remote_base_url }
  → endpointState.value = Local(host=127.0.0.1, port=18899, authToken=authToken)
  → RetrofitHolder.rebuild(endpoint)  // 由 AmitiaApiClient 触发

RuntimeEndpointProvider.switchToRemote(baseUrl, authToken)
  → DataStore.edit { mode=REMOTE, remote_base_url=baseUrl, auth_token=authToken }
  → endpointState.value = Remote(baseUrl, authToken)
  → RetrofitHolder.rebuild(endpoint)
```

### 7.4 UI 影响面

切换 Endpoint 时**不重建 UI**：

- `AmitiaApiClient.service(...)` 在每次调用时通过 `RetrofitHolder.current()` 检查 endpoint 是否变化，若变化则重建 `Retrofit` 实例
- `RetrofitHolder.rebuild(endpoint)` 是 `@Synchronized` 方法，保证并发安全
- ViewModel 与 Composable 不感知 endpoint 切换，仅 Repository 在下一次调用时自动使用新 endpoint

### 7.5 端口常量

| 服务 | 端口 | 引用 |
|---|---|---|
| Go 后端 HTTP | 18899 | `Constants.BACKEND_PORT` |
| Qdrant HTTP | 19178 | `Constants.QDRANT_PORT` |
| SurrealDB | 18000 | `Constants.SURREALDB_PORT` |
| 本地 host | 127.0.0.1 | `Constants.LOCAL_HOST` |

> 严格遵守 `AGENTS.md`：所有端口仅监听 `127.0.0.1`，避开 3000 端口。

---

## 8. 认证机制

### 8.1 双模式认证

| 模式 | 认证方式 | 令牌来源 |
|---|---|---|
| 本地（LOCAL） | Bearer Token，随机生成 | `LocalAuthTokenProvider.generateToken()` 生成 32 字节 hex（64 字符） |
| 远程（REMOTE） | Bearer Token，用户登录获取 | `SessionManager.saveSession(token, ...)` 持久化到 `amitia_session` DataStore |

### 8.2 本地随机令牌

```kotlin
LocalAuthTokenProvider.generateToken()
  → SecureRandom().nextBytes(32)
  → bytes.joinToString("") { "%02x".format(it) }
  → 64 字符 hex 字符串
  → 持久化到内存 StateFlow（Phase 6 迁移到 Keystore）
```

### 8.3 令牌注入

`AuthInterceptor` 在每个请求前注入 `Authorization: Bearer <token>` 头：

```kotlin
override fun intercept(chain: Interceptor.Chain): Response {
    val endpoint = endpointProvider.currentEndpoint.value
    val token = endpoint.authHeader()
    val request = if (!token.isNullOrBlank()) {
        chain.request().newBuilder()
            .header("Authorization", "Bearer $token")
            .build()
    } else {
        chain.request()
    }
    return chain.proceed(request)
}
```

### 8.4 令牌脱敏

`LoggingInterceptor.sanitizeHeaders` 与 `sanitizeBody` 会将以下字段替换为 `***`：

- Header：`Authorization`、`Token`、`X-Api-Key`、`Cookie`
- Body JSON：`token`、`password`、`secret`、`access_token`、`refresh_token`、`authorization`、`content`
- URL Query：`token`、`access_token`、`password`

### 8.5 会话管理

`SessionManager` 使用 DataStore `amitia_session` 持久化：

| Key | 类型 | 说明 |
|---|---|---|
| `token` | String | 会话 token |
| `expires_at` | Long | 过期时间戳（毫秒） |
| `user_id` | String | 用户 ID |

`SessionManager.isExpired()` 在 `expiresAt <= 0` 时视为永不过期，否则按墙钟时间判断。

---

## 9. Hilt 依赖图

### 9.1 Module 总览

| Module | 类型 | 提供的依赖 |
|---|---|---|
| `OkHttpModule` | `object` | `OkHttpClient`（注入 `AuthInterceptor`、`LoggingInterceptor`、`ErrorMappingInterceptor`） |
| `RetrofitModule` | `object` | `Json`、`RetrofitHolder`（注入 `OkHttpClient`、`RuntimeEndpointProvider`、`Json`） |
| `RuntimeModule`（已存在） | `abstract class` | `LinuxRootfsManager`、`LinuxProcessManager`、`HealthChecker`、`BootstrapSequence`、`RuntimeManager` |

### 9.2 注入链

```
AmitiaApiClient
  ← RetrofitHolder
    ← OkHttpClient
      ← AuthInterceptor ← RuntimeEndpointProvider
      ← LoggingInterceptor
      ← ErrorMappingInterceptor
    ← RuntimeEndpointProvider ← Context (ApplicationContext)
    ← Json

ConnectionManager
  ← AmitiaApiClient
  ← RuntimeEndpointProvider

SessionManager
  ← Context (ApplicationContext)

CharacterRepository / ChatRepository / MemoryRepository / ModelRepository /
ChannelRepository / ProactiveRepository / TtsRepository / UploadRepository
  ← AmitiaApiClient
  ← (ChatRepository) SseClient ← OkHttpClient
  ← (ChatRepository) RuntimeEndpointProvider
  ← (ChatRepository) Json

WsClient
  ← OkHttpClient
  ← Json

SseClient
  ← OkHttpClient
```

### 9.3 Singleton 范围

所有 Module 与 Repository 均为 `@Singleton` + `@InstallIn(SingletonComponent::class)`，绑定到应用进程生命周期。

---

## 10. 待审计与后续 Phase 事项

| 编号 | 事项 | 处理 Phase |
|---|---|---|
| P4-A | 后端 `realtime` 模块 WebSocket 实现细节未审计 | Phase 5+ |
| P4-B | `/api/auth/login`、`/api/auth/refresh` 接口字段未对齐 | Phase 6（认证模块） |
| P4-C | 本地 token 持久化迁移到 Keystore | Phase 6 |
| P4-D | SSE 重连策略由调用方 `retryWhen` 实现，未在 `SseClient` 内部封装 | Phase 5（聊天 ViewModel） |
| P4-E | 文件上传细粒度进度（自定义 `RequestBody` 包装 `Sink`） | Phase 5 |
| P4-F | `RuntimeEndpointProvider.loadInitial()` 与 `SessionManager.loadInitial()` 需在 `AmitiaApplication.onCreate` 或启动流程中调用 | Phase 5（启动接入） |
| P4-G | 后端 `front/src/runtime/runtime-adapter.ts` 的 `resolveApiUrl` 完整逻辑未审计 | Phase 5+ |
| P4-H | `redis/go-redis/v9` 是否被间接调用未确认 | 不影响 Android |
| P4-I | `CharacterApi.switchCurrent` 路径 `/api/characters/switch` 与 `/api/characters/current` 需与后端 `character.RegisterCharacterRouter` 实际路由对齐 | Phase 5 接入时校验 |
| P4-J | `MemoryApi.search` 用 `POST /api/memory/search`，若后端实际为 `GET /api/memory/search` 需调整 | Phase 5 接入时校验 |
| P4-K | `ProactiveApi.markRead` 路径 `/api/proactive/messages/read` 需与后端 `proactive.RegisterProactiveRouter` 实际路由对齐 | Phase 5 接入时校验 |

---

## 附录 A：Retrofit Service 速查

| Service | 文件 | Repository |
|---|---|---|
| `HealthApi` | `network/api/HealthApi.kt` | `ConnectionManager` |
| `CharacterApi` | `network/api/CharacterApi.kt` | `CharacterRepository` |
| `ChatApi` | `network/api/ChatApi.kt` | `ChatRepository` |
| `MemoryApi` | `network/api/MemoryApi.kt` | `MemoryRepository` |
| `ModelApi` | `network/api/ModelApi.kt` | `ModelRepository` |
| `ChannelApi` | `network/api/ChannelApi.kt` | `ChannelRepository` |
| `ProactiveApi` | `network/api/ProactiveApi.kt` | `ProactiveRepository` |
| `TtsApi` | `network/api/TtsApi.kt` | `TtsRepository` |
| `UploadApi` | `network/api/UploadApi.kt` | `UploadRepository` |

## 附录 B：DTO 速查

| DTO | 文件 |
|---|---|
| `HealthResponse`、`ServiceStatus` | `model/HealthDto.kt` |
| `CharacterDto`、`CharacterCreateRequest`、`CharacterUpdateRequest`、`CharacterSwitchRequest` | `model/CharacterDto.kt` |
| `MessageDto`、`ConversationDto`、`ConversationListResponse`、`MessageListResponse` | `model/MessageDto.kt` |
| `MemoryDto`、`MemoryCreateRequest`、`MemoryUpdateRequest`、`MemorySearchRequest`、`MemoryTimelineItem`、`MemoryGraphDto`、`MemoryGraphNode`、`MemoryGraphEdge` | `model/MemoryDto.kt` |
| `ModelDto`、`ModelConfigDto`、`ModelConfigUpdateRequest` | `model/ModelDto.kt` |
| `ChannelDto`、`ChannelStatusDto`、`ChannelBindRequest` | `model/ChannelDto.kt` |
| `ProactiveMessageDto`、`ProactiveListResponse`、`ProactiveMarkReadRequest` | `model/ProactiveMessageDto.kt` |
| `TtsRequest`、`TtsResponse`、`TtsVoiceDto` | `model/TtsDto.kt` |
| `UploadResponse`、`UploadResult` | `model/UploadDto.kt` |
| `SendStreamRequest`、`Attachment` | `model/SendStreamRequest.kt` |

## 附录 C：端口与监听地址汇总

| 服务 | 端口 | 监听地址 | Android 常量 |
|---|---|---|---|
| Go 后端 HTTP | 18899 | 127.0.0.1 | `Constants.BACKEND_PORT` |
| Qdrant HTTP | 19178 | 127.0.0.1 | `Constants.QDRANT_PORT` |
| Qdrant gRPC | 19179 | 127.0.0.1 | 未使用（后端内部） |
| SurrealDB | 18000 | 127.0.0.1 | `Constants.SURREALDB_PORT` |
| 微信侧车 | 19876 | 127.0.0.1 | 未使用（后端内部） |
| QQ 侧车 | 19877 | 127.0.0.1 | 未使用（后端内部） |

---

**Phase 4 / Task 4.5 完成。所有 API 路由、SSE 协议、WebSocket 消息、上传、健康检查、错误码、Endpoint 切换、认证机制均完成 Android 端映射。**

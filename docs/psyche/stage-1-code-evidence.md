# 第一阶段代码证据索引

对应实施计划：V3.1 第2步，用于支撑第7至第15步修复。

## 第7步 B-01 当前用户消息重复注入

| 位置 | 证据 | 触发方式 | 回归用例 |
|---|---|---|---|
| `backend/internal/chat/service.go:368` `ProcessMessage` | 当前用户消息先写库，再 `loadHistory(convID)`，随后模型 messages 再追加当前 user | `/api/web-chat/send-stream` 或 `/api/web-chat/send` 发送普通文本、图片、语音转文本 | REG-B-001 |
| `backend/internal/chat/model.go:19` `Message` | `include_in_context` 默认 1，当前用户消息写入后会进入历史 | 任意主聊天消息 | REG-B-001 |
| `backend/internal/agent/service.go:49` `Test` | 角色测试链路连续追加两次相同 user message | 角色测试接口 | REG-B-001 |

## 第8步 聊天写入、失败和重试一致性

| 位置 | 证据 | 触发方式 | 回归用例 |
|---|---|---|---|
| `backend/internal/system/handler.go:635` `WebChatSendStream` | 后端 SSE 只发送 `token`、`voice_audio`、`done`，没有发送前端期待的 `message_start/userMessageId` | Web 流式发送 | REG-B-021 |
| `front/src/composables/useWebChatSend.ts:111` | 前端只有收到 `message_start` 才把本地 optimistic user id 替换为后端 id | Web 流式发送成功后刷新 | REG-B-021 |
| `front/src/composables/useWebChatSend.ts:230` | 调用 `/api/web-chat/retry`，但后端未注册对应路由 | 用户点击失败消息重试 | REG-B-022 |
| `front/src/composables/useMessageSend.ts:210` | 另一套发送 composable 同样调用缺失的 retry 路由 | 旧发送链路点击重试 | REG-B-022 |
| `backend/internal/chat/service.go:443` | LLM 失败时删除本轮 user；SSE 中断后前端可能重发并产生重复 DB 消息 | 模拟 SSE 传输中断 | REG-B-023 |

## 第9步 B-02 主聊天加载完整角色运行配置

| 位置 | 证据 | 触发方式 | 回归用例 |
|---|---|---|---|
| `backend/internal/chat/service.go:368` `ProcessMessage` | 角色查询只 select `id/name/identity/system_prompt` | 修改人格、聊天风格、场景规则后聊天 | REG-B-002 |
| `backend/internal/chat/service.go:544` `sys1Builder` | Prompt 构造未消费 `personality_config/chat_style_config/scene_rules` | 主聊天 | REG-B-002 |
| `backend/internal/character/model.go:5` | 角色模型存在完整运行配置字段，但主聊天未读取 | 保存角色设置后聊天 | REG-B-002 |
| `backend/internal/chat/repository.go:157` `GetActiveModel` | 主聊天只取全局 active model，未读取模型用途分配 | 配置模型用途后聊天 | REG-B-024 |
| `front/src/views/model-config/composables/useModelConfig.ts:239` 与 `backend/internal/chat/handler.go:268` | 前端发送 routes 对象，后端绑定 routes 数组 | 保存模型用途分配 | REG-B-025 |

## 第10步 B-10 工具全局角色和会话上下文

| 位置 | 证据 | 触发方式 | 回归用例 |
|---|---|---|---|
| `backend/internal/chat/service.go:438` | 工具循环前设置全局 `CurrentConversationID/CurrentCharacterID` | 并发聊天触发工具 | REG-B-010 |
| `backend/internal/agent/tool/schedule.go:18` | 包级全局字符串保存角色和会话，无请求隔离 | 两角色并发工具调用 | REG-B-010 |
| `backend/internal/agent/tool/memory.go:56` | `saveMemory` 读取全局 `CurrentCharacterID` | 并发 `save_memory` | REG-B-010 |
| `backend/internal/agent/tool/registry.go:32` | `Execute` 只传工具参数，不传请求上下文 | 任意工具调用 | REG-B-010 |

## 第11步 B-11 forceVoice 全局竞态

| 位置 | 证据 | 触发方式 | 回归用例 |
|---|---|---|---|
| `backend/internal/agent/tool/voice_reply.go:23` | 包级 `forceVoiceFlag` 保存语音意图 | 并发强制语音工具调用 | REG-B-011 |
| `backend/internal/agent/tool/voice_reply.go:29` | `GetForceVoice` 读取后全局清空 | 请求 A 设置后请求 B 先读取 | REG-B-011 |
| `backend/internal/chat/service.go:530` | `ProcessMessage` 在返回前读取全局 forceVoice | Web/微信/QQ 并发请求 | REG-B-011 |

## 第13步 B-14 save_memory 未写入 expiresAt 和 entityId

| 位置 | 证据 | 触发方式 | 回归用例 |
|---|---|---|---|
| `backend/internal/agent/tool/memory.go:12` | 工具 schema 声明 `expiresAt/entityId` | 模型调用 `save_memory` | REG-B-014 |
| `backend/internal/agent/tool/memory.go:66` | 参数读取后丢弃，INSERT/UPDATE 未写字段 | 工具参数带有效期或实体 | REG-B-014 |
| `backend/internal/memory/service.go:163` | 普通记忆 API 支持 `ExpiresAt/EntityID` | 对比 API 与工具行为 | REG-B-014 |
| `backend/data/sql.sql:391` | `memories` 表已有 `expires_at/entity_id` | 数据库检查 | REG-B-014 |

## 第14步 B-15 retrieval_logs 会话字段错写

| 位置 | 证据 | 触发方式 | 回归用例 |
|---|---|---|---|
| `backend/internal/memory/service.go:380` `HybridSearch` | 调用 `logRetrieval(req.CharacterID, ...)` | 聊天或直接混合检索 | REG-B-015 |
| `backend/internal/memory/service.go:1407` `logRetrieval` | 插入 `retrieval_logs.conversation_id` 时传入 characterID | 任意检索日志 | REG-B-015 |
| `backend/internal/chat/service.go:609` `sys2Builder` | 聊天构造上下文时只传 CharacterID，无法记录真实 conversation_id | 主聊天触发记忆检索 | REG-B-015 |

## 第15步 请求 ID 与结构化日志

| 位置 | 证据 | 触发方式 | 回归用例 |
|---|---|---|---|
| `backend/log/logger.go:14` | 日志是 JSON formatter，但业务字段多在 message 字符串中 | 任意业务日志 | REG-B-026 |
| `backend/cmd/server/router.go:32` | 使用 `gin.Default()`，未见 request ID 中间件 | 任意 HTTP 请求 | REG-B-026 |
| `backend/internal/middleware/security/cors.go:11` | CORS allowed headers 未包含 `X-Request-ID` | 浏览器请求携带 request id | REG-B-026 |


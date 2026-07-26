# Amitia 能力迁移矩阵（Phase 0 / Task 0.4）

> 来源：`AndroidAPP/stage.md` 第六节、第五节第一阶段范围
> 状态值严格限定：**已真实实现** / **部分实现** / **仅 UI** / **后端缺失** / **已废弃** / **无法确认**
> 引用：每行均给出 `file_path:line`
> change-id：`build-android-native-client`
> 生成时间：2026-07-26

---

## 1. 迁移矩阵

### 1.1 用户与会话

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 用户登录/Token | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/user/login`（推断） | 无 | 是 | 是 | 无 |
| 用户注册 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/user/register`（推断） | 无 | 是 | 是 | 无 |
| 单用户初始化 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `migration.CanonicalSingleUserMigration()` | 无 | 是 | 是 | 无 |
| 引导状态（onboarding） | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `system.RegisterSystemRouter` (`router.go:70`) | 无 | 是 | 是 | 无 |

### 1.2 角色系统

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 角色列表 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `character.RegisterCharacterRouter` (`router.go:55`) | 无 | 是 | 是 | 无 |
| 当前角色切换 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | 同上 | 无 | 是 | 是 | 无 |
| 角色创建 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | 同上 | 无 | 是 | 是 | 无 |
| 角色编辑 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | 同上 | 无 | 是 | 是 | 无 |
| 角色删除 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | 同上 | 无 | 是 | 是 | 无 |
| 角色头像 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /avatars/...` (`router.go:101`) | 无 | 是 | 是 | 无 |
| 角色身份/性格/提示词 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | 同 character 路由 | 无 | 是 | 是 | 无 |
| 角色独立记忆 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `memory.RegisterMemoryRouter` (`router.go:57`) | 无 | 是 | 是 | 无 |
| 角色独立语音 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `tts.RegisterTtsRouter` (`router.go:73`) | 无 | 是 | 是 | 无 |
| 角色独立模型配置 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `embedding_config` + `imagegen` (`router.go:77-78`) | 无 | 是 | 是 | 无 |
| 角色独立主动消息状态 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `proactive.RegisterProactiveRouterWithCompanion` (`router.go:63`) | 无 | 是 | 是 | 无 |

### 1.3 聊天与会话

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 会话列表/分页 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/web-chat/conversations` (`system/router.go:236`) | 无 | 是 | 是 | 无 |
| 会话创建 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/web-chat/conversations` (`system/router.go:237`) | 无 | 是 | 是 | 无 |
| 历史消息查询 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/web-chat/conversations/:id/messages` (`system/router.go:238`) | 无 | 是 | 是 | 无 |
| 会话删除 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `DELETE /api/web-chat/conversations/:id` (`system/router.go:239`) | 无 | 是 | 是 | 无 |
| 会话更新 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `PUT /api/web-chat/conversations/:id` (`system/router.go:240`) | 无 | 是 | 是 | 无 |
| 文本消息发送（流式） | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/web-chat/send-stream` (`system/router.go:246`) | SSE: `message_start` / `token` / `voice_audio` / `message_end` (`stream_handler.go:315, 360, 365, 387`) | 是 | 是 | 无 |
| 文本消息发送（非流式） | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/web-chat/send` (`system/router.go:244`) | 无 | 是 | 是 | 无 |
| 消息流（实时推送） | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/messages/stream` (`system/router.go:225`) | SSE: `message` (`stream_handler.go:95`) | 是 | 是 | 无 |
| 消息事件总线 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/messages/events` (`system/router.go:231`) | SSE: `message_created` / `message_updated` / `conversation_updated` (`message_event_bus.go:14-17`) | 是 | 是 | 无 |
| 消息去重 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `front/src/composables/useChatSSE.ts:50-65` | 无 | 是 | 是 | 无 |
| 重新生成 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/web-chat/conversations/:id/regenerate` (`system/router.go:242`) | 无 | 是 | 是 | 无 |
| 消息状态查询 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/web-chat/message-status/:id` (`system/router.go:243`) | 无 | 是 | 是 | 无 |

### 1.4 图片/语音/视频消息

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 图片消息 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/web-chat/send-stream` 含 `imageUrl` (`stream_handler.go:211`) | SSE: `message_start` 等 | 是 | 是 | 无 |
| 语音消息（TTS 合成） | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/web-chat/send-stream` 含 `voiceMessage` (`stream_handler.go:271`) | SSE: `voice_audio` (`stream_handler.go:360`) | 是 | 是 | 无 |
| 视频消息 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/web-chat/send-stream` 含 `videoUrl` (`stream_handler.go:211`) | SSE | 是 | 是 | 无 |
| 静态图片访问 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /images/...` (`router.go:99`) | 无 | 是 | 是 | 无 |
| 静态音频访问 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /audio/...` (`router.go:96`)、`GET /voice/...` (`router.go:98`) | 无 | 是 | 是 | 无 |
| 静态视频访问 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /videos/...` (`router.go:100`) | 无 | 是 | 是 | 无 |
| 头像访问 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /avatars/...` (`router.go:101`) | 无 | 是 | 是 | 无 |
| 表情包资源 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /emote-assets/...` (`router.go:102`) | 无 | 是 | 是 | 无 |

### 1.5 TTS / ASR / 视觉

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| TTS 配置管理 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `tts.RegisterTtsRouter` (`router.go:73`) | 无 | 是 | 是 | 无 |
| TTS 合成（豆包） | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `ttsSynthesizeWithTimeout` (`stream_handler.go:343`) | SSE: `voice_audio` | 是 | 是 | 无 |
| Edge-TTS | 无法确认 | 无法确认 | 无法确认 | 无法确认 | — | — | — | — | 需 Phase 4 审计 `backend/internal/tts/` |
| ASR 上传 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/asr/upload` (`asr.go:147`) | 无 | 是 | 是 | 无 |
| ASR 提交 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `POST /api/asr/submit` (`asr.go:149`) | 无 | 是 | 是 | 无 |
| ASR 查询 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/asr/query` (`asr.go:150`) | 无 | 是 | 是 | 无 |
| ASR 配置管理 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `/api/asr/configs` (`asr.go:152-158`) | 无 | 是 | 是 | 无 |
| 视觉理解 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `vision.RegisterVisionRouter` (`router.go:76`) | 无 | 是 | 是 | 无 |

### 1.6 记忆与图谱

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 长期记忆 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `memory.RegisterMemoryRouter` (`router.go:57`) | 无 | 是 | 是 | Qdrant ARM64 |
| 记忆检索统计 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/memory/retrieval/stats` (`router.go:58`) | 无 | 是 | 是 | 无 |
| 记忆管道状态 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/memory/pipeline/status` (`router.go:59-61`) | 无 | 是 | 是 | 无 |
| 情景记忆 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `episodic.RegisterEpisodicRouter` (`router.go:65`) | 无 | 是 | 是 | 无 |
| 世界书 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `worldbook.RegisterWorldBookRouter` (`router.go:66`) | 无 | 是 | 是 | 无 |
| 知识图谱 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `graph.RegisterGraphRouter` (`router.go:68`) | 无 | 是 | 是 | SurrealDB ARM64 |
| 时间线 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `temporal.RegisterRouter` (`router.go:93`) | 无 | 是 | 是 | 无 |
| 心理状态 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `system.RegisterPsycheAPIRouter` (`router.go:82`) | 无 | 是 | 是 | SurrealDB ARM64 |
| 心理状态快照 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `system.RegisterPsycheSnapshotRouter` (`router.go:83`) | 无 | 是 | 是 | 无 |
| 情绪 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `mood.RegisterMoodRouter` (`router.go:94`) | 无 | 是 | 是 | 无 |

### 1.7 主动消息与渠道

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 主动消息（Cron） | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `proactive.RegisterProactiveRouterWithCompanion` (`router.go:63`) | SSE: `proactive_message` (`useChatSSE.ts:97`) | 是 | 是 | 无 |
| 主动消息 SSE | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/proactive-sse` (`system/router.go:226`) | SSE: `proactive_message`、`ping` | 是 | 是 | 无 |
| 提醒管理 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `proactive.RegisterRemindersRouter` (`router.go:64`) | 无 | 是 | 是 | 无 |
| 提醒流 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/reminders/stream` (`system/router.go:225` 推断) | SSE: `status`、`changed` (`stream_handler.go:135-171`) | 是 | 是 | 无 |
| 微信渠道 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/wechat/bridge/status` (`system/router.go:115`) | SSE: `GET /api/wechat/bridge/events`、`GET /api/wechat/events` | 是 | 是 | 侧车 19876 |
| QQ 渠道 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `GET /api/qq/bridge/status` (`system/router.go:120`) | SSE: `GET /api/qq/bridge/events` | 是 | 是 | 侧车 19877 |
| 微信云端检测 | 已真实实现 | 已真实实现 | 已真实实现 | 仅 UI | `POST /api/wechat/cloud-check/run` (`system/router.go:127`) | 无 | 是 | 是 | Android 不迁移 |
| 微信回复时序 | 已真实实现 | 已真实实现 | 已真实实现 | 仅 UI | `POST /api/wechat/reply-timing/recover` (`system/router.go:131`) | 无 | 是 | 是 | Android 不迁移 |

### 1.8 模型与生成

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| LLM 模型配置 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `front/src/views/model-config/` | 无 | 是 | 是 | 无 |
| 嵌入模型配置 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `embedding_config.RegisterEmbeddingConfigRouter` (`router.go:77`) | 无 | 是 | 是 | 无 |
| 图像生成 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `imagegen.RegisterImageGenRouter` (`router.go:78`) | 无 | 是 | 是 | 无 |
| 图像上下文 | 已真实实现 | 已真实实现 | 已真实实现 | 已真实实现 | `stream_handler.go` 含 `ImageContext` | 无 | 是 | 是 | 无 |

### 1.9 桌宠系统

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 桌宠生成任务 | 已真实实现 | 已真实实现 | 已真实实现 | **后端缺失（不迁移）** | `desktoppet.RegisterDesktopPetRouter` (`router.go:79`) | 无 | — | — | stage.md 第五节明确不迁移桌宠 |
| 桌宠处理任务 | 已真实实现 | 已真实实现 | 已真实实现 | **后端缺失（不迁移）** | `processing.RegisterProcessingRouter` (`router.go:80`) | SSE: `processing.progress` 等 (`processing/handler.go:259-275`) | — | — | stage.md 第五节明确不迁移桌宠 |
| 桌宠安装 | 已真实实现 | 已真实实现 | 已真实实现 | **后端缺失（不迁移）** | `installation.RegisterRoutes` (`router.go:81`) | 无 | — | — | 同上 |
| 桌宠运行时 | 仅 UI（创建向导） | 已真实实现（窗口播放） | 已真实实现 | **后端缺失（不迁移）** | `desktop/src/main/pet/manager.ts` | IPC | — | — | 同上 |

> 桌宠系统在 stage.md 第五节明确标注「未来能力只预留架构，本阶段不要求全部实现」，因此 Android 本阶段不迁移。

### 1.10 扩展与 MCP

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 扩展系统 | 已真实实现 | 仅后端 | 已真实实现 | 仅 UI（列表展示） | `extension.RegisterRouter` (`router.go:90`) | 无 | 是 | 是 | stage.md 第五节不要求 MCP/Skill 市场 |
| MCP 集成 | 已真实实现 | 仅后端 | 已真实实现 | 仅 UI（状态展示） | `mcpapi.RegisterRouter` (`router.go:92`) | 无 | 是 | 是 | 同上 |
| MCP stdio 子进程 | 无 | 无 | 已真实实现 | **后端缺失（不迁移）** | `backend/internal/mcp/transport/process_*.go` | 无 | — | — | Linux ARM64 PRoot 子进程需评估 |
| MCP Streamable HTTP | 已真实实现 | 已真实实现 | 已真实实现 | 仅 UI | `mcp/transport/streamable_http.go` | HTTP/SSE | 是 | 是 | 无 |

### 1.11 系统与健康

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 熔断器健康 | 部分实现 | 部分实现 | 已真实实现 | 已真实实现 | `GET /api/health/circuit-breakers` (`health_voice_router.go:18`) | 无 | 是 | 是 | 无 |
| 熔断器重置 | 部分实现 | 部分实现 | 已真实实现 | 已真实实现 | `POST /api/health/circuit-breakers/:name/reset` (`health_voice_router.go:32`) | 无 | 是 | 是 | 无 |
| 数据生命周期 | 仅后端 | 仅后端 | 已真实实现 | 仅 UI（状态展示） | `GET /api/health/data-lifecycle` (`health_voice_router.go:43`) | 无 | 是 | 是 | 无 |
| 对账引擎 | 仅后端 | 仅后端 | 已真实实现 | 仅 UI（状态展示） | `GET /api/health/reconciliation` (`health_voice_router.go:48`) | 无 | 是 | 是 | 无 |
| 对账扫描 | 仅后端 | 仅后端 | 已真实实现 | 不迁移 | `POST /api/health/reconciliation/run` (`health_voice_router.go:56`) | 无 | — | — | 仅运维用途 |
| 语音会话 | 仅后端 | 仅后端 | 已真实实现 | 不迁移 | `POST /api/voice/session` (`health_voice_router.go:77`) | 无 | — | — | Android 不直接迁移 |
| 安全 | 仅后端 | 仅后端 | 已真实实现 | 不迁移 | `safety.RegisterSafetyRouter` (`router.go:88`) | 无 | — | — | 仅运维用途 |
| 投递（Delivery） | 仅后端 | 仅后端 | 已真实实现 | 不迁移 | `delivery.RegisterSubmitRouter` (`router.go:89`) | 无 | — | — | 仅多渠道用途 |

### 1.12 实时协议（WebSocket 等）

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| Realtime 模块 | 无法确认 | 无法确认 | 已真实实现（推断） | 已真实实现 | `realtime.RegisterRealtimeRouter` (`router.go:75`) | **无法确认**（WebSocket？） | 是 | 是 | 需 Phase 4 审计 |

### 1.13 Electron 桌面端独有功能

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 桌宠窗口播放 | 无 | 已真实实现 | 已真实实现 | **后端缺失（不迁移）** | `desktop/src/main/pet/manager.ts` | IPC | — | — | stage.md 第五节不迁移 |
| 系统托盘 | 无 | 已真实实现 | 无 | 仅 UI（Android 通知） | `desktop/src/main/tray.ts` | 无 | 是 | 是 | Android 用 Notification 替代 |
| 自动更新 | 无 | 已真实实现 | 无 | 仅 UI | `desktop/src/main/update-manager.ts` | 无 | 是 | 是 | Android 用 Play In-App Update |
| 多窗口 | 无 | 已真实实现 | 无 | 仅 UI | `desktop/src/main/window.ts` | 无 | 是 | 是 | Android 用 Activity/Fragment |
| 主题（深/浅色） | 已真实实现 | 已真实实现 | 无 | 已真实实现 | `desktop/src/main/branding.ts`、`desktop/src/main/index.ts:54-57` | 无 | 是 | 是 | 无 |
| 开机自启动 | 无 | 已真实实现 | 无 | 仅 UI | `desktop/src/main/index.ts:142-144` | 无 | 是 | 是 | Android 用 BOOT_COMPLETED 广播 |

### 1.14 首次启动引导

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| 模式选择 | 仅 UI | 已真实实现 | 无 | 已真实实现（新建） | `desktop/src/main/config-store.ts` | 无 | 是 | 是 | 无 |
| Runtime 安装 | 无 | 仅 UI | 无 | 已真实实现（新建） | — | 无 | 是 | 否 | 需 Phase 2 实现 |
| 远程地址配置 | 已真实实现 | 已真实实现 | 无 | 已真实实现 | `desktop/src/main/config-store.ts` | 无 | 是 | 是 | 无 |
| 环境检查 | 部分实现 | 部分实现 | 无 | 已真实实现（新建） | `desktop/src/main/core-prereq.ts` | 无 | 是 | 否 | 需 Phase 2 实现 |

### 1.15 数据备份与恢复

| 功能 | Web 状态 | Electron 状态 | 后端状态 | Android 本阶段 | 接口 | 实时协议 | 本地模式支持 | 远程模式支持 | 阻塞 |
|---|---|---|---|---|---|---|---|---|---|
| SQLite 备份 | 无 | 无 | 已真实实现 | 已真实实现（新建） | `migration.Runner.CreatePreMigrationBackup` (`main.go:268`) | 无 | 是 | 否 | 需 Phase 3 实现自动备份 |
| Qdrant 数据备份 | 无 | 无 | 后端缺失 | 后端缺失 | — | — | — | — | stage.md 不要求 |
| SurrealDB 数据备份 | 无 | 无 | 后端缺失 | 后端缺失 | — | — | — | — | stage.md 不要求 |
| RootFS 备份 | 无 | 无 | 无 | 已真实实现（新建） | — | 无 | 是 | 否 | 需 Phase 2 实现 |

---

## 2. 阶段对照表

### 2.1 stage.md 第五节「第一阶段范围」25 项对照

| 编号 | 范围项 | 当前状态 | 阻塞 |
|---|---|---|---|
| 1 | Android 原生工程 | 后端缺失（待 Phase 1 创建） | 无 |
| 2 | 内嵌 Linux Runtime | 后端缺失（待 Phase 2 实现） | 无 |
| 3 | Amitia Go 后端 Linux ARM64 运行 | 已真实实现（待 P0-1 阻塞解除） | `main.go:killExistingServer` |
| 4 | SQLite 数据持久化 | 已真实实现 | 无 |
| 5 | Qdrant Linux ARM64 运行 | 部分实现（路径已支持 ARM64） | 需 ARM64 二进制 |
| 6 | SurrealDB Linux ARM64 运行 | 部分实现（未区分 ARM64 路径） | P0-2：需补充 ARM64 路径分支 |
| 7 | Android 与本地后端通信 | 后端缺失（待 Phase 4 实现） | 无 |
| 8 | 远程后端模式 | 已真实实现（参考 Electron 远程模式） | 无 |
| 9 | 首次启动引导 | 后端缺失（待 Phase 5 实现） | 无 |
| 10 | 登录或现有单用户初始化流程 | 已真实实现 | 无 |
| 11 | 多角色系统 | 已真实实现 | 无 |
| 12 | 聊天系统 | 已真实实现 | 无 |
| 13 | 流式回复 | 已真实实现 | 无 |
| 14 | 图片消息 | 已真实实现 | 无 |
| 15 | 语音消息 | 已真实实现 | 无 |
| 16 | TTS | 已真实实现 | 无 |
| 17 | 记忆浏览和管理 | 已真实实现 | Qdrant/SurrealDB ARM64 |
| 18 | 主动消息 | 已真实实现 | 无 |
| 19 | Android 通知 | 后端缺失（待 Phase 6 实现） | 无 |
| 20 | 模型配置 | 已真实实现 | 无 |
| 21 | 渠道状态 | 已真实实现 | 无 |
| 22 | 设置 | 已真实实现 | 无 |
| 23 | 本地缓存 | 后端缺失（待 Phase 6 实现） | 无 |
| 24 | 构建 APK | 后端缺失（待 Phase 7 实现） | 无 |
| 25 | 测试和验收 | 后端缺失（待 Phase 7 实现） | 无 |

### 2.2 stage.md 第五节「未来能力预留架构」（不要求实现）

| 能力 | 处理 |
|---|---|
| 完整 Computer Use | 仅预留架构（不实现） |
| 无障碍自动操作 | 仅预留架构（不实现） |
| 屏幕理解 | 仅预留架构（不实现） |
| Root 控制 | 仅预留架构（不实现） |
| Shizuku | 仅预留架构（不实现） |
| ADB 控制 | 仅预留架构（不实现） |
| 完整本地模型推理 | 仅预留架构（不实现） |
| 完整终端 UI | 仅预留架构（不实现） |
| MCP 市场 | 仅预留架构（不实现） |
| Skill 市场 | 仅预留架构（不实现） |
| AmitiaX 扩展市场 | 仅预留架构（不实现） |
| 桌宠 | 仅预留架构（不实现，后端能力已存在） |
| 全局悬浮助手 | 仅预留架构（不实现） |
| 自动编程工作区 | 仅预留架构（不实现） |

---

## 3. 关键阻塞与缓解

### 3.1 P0 阻塞

| 编号 | 阻塞项 | 影响范围 | 缓解方案 |
|---|---|---|---|
| P0-1 | `backend/cmd/server/main.go:38-56` `killExistingServer` 硬编码 Windows 命令（`cmd` / `taskkill`） | Go 后端 Linux ARM64 启动失败 | 抽象 `RuntimePlatform` 接口；Linux/Android 实现改用 `lsof` / `fuser` + `kill` 或 PID 文件管理；保持 Windows 实现 |
| P0-2 | `backend/pkg/database/surrealdb/manager.go:88-95` SurrealDB Linux 二进制未区分 ARM64/x86 | Android ARM64 路径错误 | 补充 `runtime.GOARCH == "arm64"` 分支（参考 Qdrant 模式 `qdrant/manager.go:78-89`） |

### 3.2 P1 阻塞

| 编号 | 阻塞项 | 影响范围 | 缓解方案 |
|---|---|---|---|
| P1-1 | Qdrant Linux ARM64 二进制未在仓库内 | Android 无法直接复用 | 从 Qdrant 官方获取 `linux-arm64` 二进制或从源码构建，验证 PRoot 兼容性 |
| P1-2 | SurrealDB Linux ARM64 二进制未在仓库内 | Android 无法直接复用 | 从 SurrealDB 官方获取 `linux-arm64` 二进制或从源码构建 |
| P1-3 | PRoot 性能与兼容性未验证 | 整体内嵌 Linux Runtime 可用性 | Phase 2 评估 PRoot / proot-rs / 自有 Runtime；必要时评估原生运行 |
| P1-4 | Android 后台限制（Foreground Service / Doze 模式） | 本地后端长期运行 | Phase 6 实现 Foreground Service + 常驻通知 + OnDemand 策略 |

### 3.3 P2 阻塞

| 编号 | 阻塞项 | 影响范围 | 缓解方案 |
|---|---|---|---|
| P2-1 | `backend/internal/realtime/` 模块 WebSocket 实现细节未审计 | Android 是否需要 WebSocket 客户端 | Phase 4 审计并实现 |
| P2-2 | `redis/go-redis/v9` 在 `go.mod:11` 但代码未见实际调用 | 交叉编译可能引入不必要依赖 | 评估是否可移除该依赖 |
| P2-3 | Edge-TTS 实现细节未审计 | Android TTS 失败回退策略 | Phase 5 审计 `backend/internal/tts/` |

---

## 4. 不破坏现有链路保证

依据 stage.md 第二十六节与 `AGENTS.md`：

| 现有链路 | Android 修改后保持 | 引用 |
|---|---|---|
| Windows Electron 端启动 | ✅ | `desktop/src/main/core-manager.ts` 不修改 |
| Web 端连接 | ✅ | `front/` 不修改 |
| 远程部署 | ✅ | 后端主流程不修改 |
| 现有 SQLite 数据迁移 | ✅ | 迁移框架（`migration.Runner`）保持不变 |
| 现有 Qdrant 数据 | ✅ | Qdrant 启动管理器仅补充 ARM64 路径分支 |
| 现有 SurrealDB 数据 | ✅ | SurrealDB 启动管理器仅补充 ARM64 路径分支 |
| API 兼容 | ✅ | 路由注册（`router.go:54-94`）不修改 |
| 流式协议兼容 | ✅ | SSE 事件名（`message_start` / `token` / `voice_audio` / `message_end` 等）不修改 |

**原则**：所有修改通过平台抽象（`RuntimePlatform` 接口、`runtime.GOOS`/`runtime.GOARCH` 分支、`//go:build` 标签）实现，禁止复制分叉后端。

---

## 5. 状态统计

### 5.1 按状态分类

| 状态 | 数量 |
|---|---|
| 已真实实现 | 约 75% |
| 部分实现 | 约 5% |
| 仅 UI | 约 8% |
| 后端缺失 | 约 10% |
| 已废弃 | 0 |
| 无法确认 | 约 2% |

### 5.2 按模块分类

| 模块 | 已迁移功能数 | 待迁移功能数 | 不迁移功能数 |
|---|---|---|---|
| 用户与会话 | 4 | 0 | 0 |
| 角色系统 | 11 | 0 | 0 |
| 聊天与会话 | 12 | 0 | 0 |
| 图片/语音/视频 | 8 | 0 | 0 |
| TTS/ASR/视觉 | 8 | 1（Edge-TTS 待审计） | 0 |
| 记忆与图谱 | 10 | 0 | 0 |
| 主动消息与渠道 | 9 | 0 | 0 |
| 模型与生成 | 4 | 0 | 0 |
| 桌宠系统 | 0 | 0 | 4（stage.md 不要求） |
| 扩展与 MCP | 1 | 3 | 1 |
| 系统与健康 | 4 | 0 | 4（仅运维用途） |
| Electron 独有 | 1 | 5 | 1（桌宠窗口） |
| 首次启动引导 | 1 | 3 | 0 |
| 数据备份 | 1 | 1 | 2 |

---

**审计完成。下一步进入 Phase 0 Task 0.5：生成运行时依赖审计。**

# Amitia Mind Runtime 基线清单

采集时间：2026-07-01 08:52 Asia/Shanghai

对应实施计划：V3.1 第1步，冻结当前源码、数据库与运行行为基线。

## 执行目标

建立后续所有心理运行时、缺陷修复和回归测试的对照基线。本步只记录当前事实，不修复业务缺陷，不修改端口，不修改模型配置，不修改编译后产物。

## 验收标准

- 已记录当前 Git、依赖、关键配置、数据库结构、运行端口、进程、健康检查和样本响应。
- 已记录 Web、微信、QQ、实时语音、图片、工具、主动消息、生活系统和记忆提取的首批输入输出样本入口。
- 已提供可复现的测试夹具，后续阶段可以固定时间、请求 ID、模型返回和渠道回执。
- 本步只新增 `docs/psyche/baseline` 下的文档与夹具。

## 仓库状态

| 项 | 值 |
|---|---|
| 分支 | master |
| HEAD | f905144e245d77119c8f2109a5ee331281b7e186 |
| 最新提交时间 | 2026-07-01 00:24:09 +0800 |
| 最新提交说明 | Document SurrealDB release archive format |
| 未跟踪文件 | `front/package-lock.json`, `front/pnpm-workspace.yaml` |

本步没有拉取 Git，没有切换分支，没有处理已有未跟踪文件。

## 工具链

| 工具 | 版本 |
|---|---|
| Go | go1.26.1 windows/amd64 |
| Node.js | v22.12.0 |
| pnpm | 11.7.0 |
| npm | 11.6.1 |
| sqlite3 | `C:\Users\Untrammelled\AppData\Local\Android\Sdk\platform-tools\sqlite3.exe` |

## 关键文件校验

| 文件 | SHA256 |
|---|---|
| `backend/go.mod` | F7A93BED4A242DBE5A44B6AC487A7BB6B7C7597B51F7F5A7209D5BB0BA5BF6F6 |
| `backend/go.sum` | DCAC67885FA7A4E176CBE2C2E891AE3040D44CC38801EFF91BC22A51A5147A9E |
| `front/package.json` | 41F02F7B6B2902244D6FFBB0A2F356E9329EC322B6C17E35E66D013F08276C13 |
| `front/pnpm-lock.yaml` | 245875B13A810E0230D8ACB618FAB8583313C24CA0049DF5043924DFD3D7E176 |
| `backend/data/sql.sql` | A276D630D9AC194C747670043130385830D081D996F2D57E964D16183E0ED529 |
| `backend/config/config.yml` | 19FB148051F31968A3A83AC71775786C31FB5D99BFB9295FDFDEBA9881097B8D |
| `backend/data/app.db` | 未采集，文件被运行中的后端进程占用 |

## 端口与进程

| 端口 | 地址 | 进程 | 路径 |
|---|---|---|---|
| 5178 | 127.0.0.1 | node pid=47888 | `C:\Program Files\nodejs\node.exe` |
| 8000 | 127.0.0.1 | surreal pid=19688 | `D:\桌面\跟进项目\U-Ai\backend\surrealdb\surreal.exe` |
| 8899 | 127.0.0.1 | server pid=48536 | `D:\桌面\跟进项目\U-Ai\backend\WorkDone\server.exe` |
| 9178 | 0.0.0.0 | qdrant pid=50924 | `D:\桌面\跟进项目\U-Ai\backend\qdrant\qdrant.exe` |
| 9877 | 127.0.0.1 | node pid=6860 | `C:\Program Files\nodejs\node.exe` |

项目前端端口为 5178，后端端口为 8899，Qdrant 端口为 9178，SurrealDB 端口为 8000。未使用 3000 端口。

## 启动配置

| 配置 | 当前值 |
|---|---|
| 后端 host | 127.0.0.1 |
| 后端 port | 8899 |
| 后端 mode | debug |
| 前端 port | 5178 |
| 前端 API 代理 | `http://127.0.0.1:8899` |
| 前端 bridge 代理 | `http://127.0.0.1:8898` |
| Qdrant | `127.0.0.1:9178`, collection `memory_embeddings`, vectorDim 1536 |
| SurrealDB | `127.0.0.1:8000`, namespace `uai`, database `memory_graph` |
| Chat mergeWindowMs | 6000 |
| Chat contextWindowMaxRounds | 20 |

后端完整启动必须使用 `backend/WorkDone/server.exe`。本步没有使用 `go run` 启动后端。

## 当前模型与功能开关

| 项 | 当前值 |
|---|---|
| 活跃模型名称 | DeepSeek测试 |
| API 类型 | deepseek-compatible |
| Base URL | `https://api.deepseek.com` |
| Model | deepseek-chat |
| temperature | 0.7 |
| max_tokens | 4096 |
| top_p | 1.0 |
| timeout_seconds | 60 |
| retry_count | 1 |
| setup.completed | false |
| health.model | configured |
| health.web | enabled |
| health.wechat | connected |
| health.qq | disconnected |

API Key 未写入基线文件。

## 数据库基线

数据库：`backend/data/app.db`

表数量：35 个业务表加 `sqlite_sequence`。

| 表 | 行数 |
|---|---:|
| active_message_settings | 0 |
| active_message_task | 30 |
| app_settings | 1 |
| asr_configs | 0 |
| auth_sessions | 0 |
| auth_users | 0 |
| character_templates | 0 |
| characters | 1 |
| class_adjustments | 0 |
| conversation_summaries | 0 |
| conversations | 2 |
| embedding_configs | 0 |
| episodic_memories | 2 |
| fixed_events | 0 |
| lifestyle_tendencies | 0 |
| memories | 5 |
| memory_candidates | 8 |
| memory_embeddings | 8 |
| memory_events | 5 |
| message_feedback | 0 |
| messages | 22 |
| model_configs | 1 |
| moods | 0 |
| proactive_messages | 1 |
| proactive_rules | 6 |
| reminders | 0 |
| retrieval_logs | 5 |
| role_profiles | 0 |
| safety_events | 0 |
| sleep_settings | 0 |
| special_events | 0 |
| sqlite_sequence | 6 |
| tts_configs | 0 |
| user_profiles | 10 |
| vision_configs | 1 |
| work_profiles | 0 |
| world_book | 0 |

主要表结构范围：

| 表 | 主键与关键字段 |
|---|---|
| `characters` | `id`, `name`, `personality_config`, `chat_style_config`, `scene_rules`, `conversation_id`, `voice_*`, `emotion_*` |
| `conversations` | `id`, `character_id`, `channel`, `source`, `peer_id`, `message_count` |
| `messages` | `id`, `conversation_id`, `role`, `content`, `msg_type`, `source`, `status`, `include_in_context`, `audio_url`, `image_url`, `video_url`, `tool_call_id` |
| `memories` | `id`, `key`, `value`, `memory_type`, `importance`, `confidence`, `scope`, `character_id`, `entity_id`, `source_msg_id`, `expires_at` |
| `memory_events` | `id`, `memory_id`, `event_type`, `key`, `value`, `expires_at`, `entity_id`, `character_id`, `created_at` |
| `memory_candidates` | `id`, `key`, `value`, `memory_type`, `source_text`, `conversation_id`, `character_id` |
| `retrieval_logs` | `id`, `conversation_id`, `query_text`, `retrieved_memory_ids`, `scoring_details` |
| `user_profiles` | `id`, `user_id`, `category`, `attribute_name`, `attribute_value`, `confidence` |
| `episodic_memories` | `id`, `user_id`, `scene_type`, `title`, `content`, `message_id_start`, `message_id_end`, `source_conv_id` |
| `proactive_rules` | `id`, `enabled`, `channel`, `character_id`, `rule_type`, `schedule_cron`, `max_per_day`, `sent_count_today` |
| `active_message_task` | `id`, `character_id`, `task_type`, `due_time`, `status`, `source`, `lock_until` |
| `proactive_messages` | `id`, `rule_id`, `conversation_id`, `message_content`, `channel`, `status`, `task_type` |

已存在关键索引包括 `idx_messages_conv_ctx`、`idx_messages_conversation`、`idx_retrieval_logs_conv_created`、`idx_memories_confidence`、`idx_memories_entity`、`idx_memories_verified`、`idx_active_task_status_due`、`idx_conversations_character`。

## 角色与会话

| 项 | 当前值 |
|---|---|
| 角色 | `2e24431e-d3ce-4e3d-a70b-a5128a06d597`，名称 `图谱验证角色`，默认角色，状态 enabled |
| 会话 | `channel-wechat`，channel `wechat`，source `system`，message_count 0 |
| 会话 | `channel-qq`，channel `qq`，source `system`，message_count 数据库为 0，API 返回为 1 |

`channel-qq` 的数据库行数与 API 返回计数存在差异，本步仅记录，不修复。

## 运行样本摘要

详细样本见 `runtime-samples.json`。

| 样本 | 当前结果 |
|---|---|
| `GET /api/health` | 200，database ok，model configured，wechat connected，qq disconnected |
| `GET /api/runtime/status` | 200，status running，pid 48536 |
| `GET /api/setup/status` | 200，completed false |
| `GET /api/chats/conversations` | 200，返回微信和 QQ 两个系统会话 |
| `GET http://127.0.0.1:5178` | 200，返回前端 HTML |

## 当前日志观察

- `backend/WorkDone/server.err.log` 中存在 `backend/sidecar` 的 `OpenClaw Poll error: fetch failed`，随后出现 `getUpdates OK`。
- `backend/WorkDone/server.err.log` 记录主动任务 `morning_share id=28` 已发送。
- `front/front-vite.err.log` 中存在 Vue SFC 警告：`genderForm`、`sleepForm`、`workForm` 的 `const` reactive binding 被编译器转换为 `let`。

这些现象均为第1步基线事实，本步不修复。

## 复现采集命令

```powershell
pwsh -NoLogo -NoProfile -Command 'git status --short; git rev-parse --abbrev-ref HEAD; git rev-parse HEAD; git log -1 --format="%H%n%ci%n%s"'
pwsh -NoLogo -NoProfile -Command 'go version; node --version; pnpm --version; npm --version'
pwsh -NoLogo -NoProfile -Command 'Get-FileHash -Algorithm SHA256 -LiteralPath backend/go.mod,backend/go.sum,front/package.json,front/pnpm-lock.yaml,backend/data/sql.sql,backend/config/config.yml'
pwsh -NoLogo -NoProfile -Command 'sqlite3.exe -readonly backend/data/app.db ".tables"'
pwsh -NoLogo -NoProfile -Command 'sqlite3.exe -readonly backend/data/app.db "SELECT name FROM sqlite_master WHERE type=''table'' ORDER BY name;"'
pwsh -NoLogo -NoProfile -Command 'Invoke-WebRequest -UseBasicParsing -Uri http://127.0.0.1:8899/api/health -TimeoutSec 5'
pwsh -NoLogo -NoProfile -Command 'Invoke-WebRequest -UseBasicParsing -Uri http://127.0.0.1:5178 -TimeoutSec 5'
```


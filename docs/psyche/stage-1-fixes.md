# 第一阶段修复记录

## 第7步 B-01 当前用户消息重复注入

修复范围：

- `backend/internal/chat/service.go`
- `backend/internal/chat/service_history_test.go`

修复方式：

- 新增 `loadHistoryExcluding`，按消息 ID 排除本轮用户消息。
- `ProcessMessage` 写入本轮用户消息后，构造模型上下文和记忆流水线历史时排除 `userMsgID`。
- 测试保留上一轮内容相同的真实用户消息，证明没有使用字符串去重误删真实重复输入。

验收：

```powershell
pwsh -NoLogo -NoProfile -Command 'go test ./internal/chat ./cmd/server ./internal/migration ./internal/psyche_testdata'
```

## 第8步 聊天写入、失败和重试一致性

修复范围：

- `backend/internal/chat/model.go`
- `backend/internal/chat/service.go`
- `backend/internal/chat/service_history_test.go`
- `backend/internal/system/handler.go`
- `backend/data/sql.sql`

修复方式：

- `messages` 增加 `request_id` 字段和会话内请求索引，聊天请求、普通响应和 SSE 起始事件透传 `requestId`。
- `ProcessMessage` 支持同一 `requestId` 的幂等重试：已有完整助手回复时直接返回，已有用户消息但助手回复缺失时复用原用户消息继续处理。
- 模型配置缺失、LLM 调用失败或助手消息事务写入失败时，将用户消息标记为 `failed`，避免删除用户输入导致前端状态和后端记录不一致。
- 助手消息写入、用户状态更新和会话计数更新放入同一事务，降低成功响应与落库状态分裂的风险。
- 新增请求消息查询测试，覆盖同一请求下用户消息和多段助手消息的查找顺序。

验收：

```powershell
pwsh -NoLogo -NoProfile -Command 'go test ./internal/chat ./cmd/server ./internal/migration ./internal/psyche_testdata'
```

重启完整服务后需验证：

- `http://127.0.0.1:8899/api/health` 返回正常。
- `http://127.0.0.1:5178` 可访问前端页面。
- 当前 SQLite 库 `messages` 表包含 `request_id` 字段。
- 当前 SQLite 库包含 `idx_messages_request` 索引。

## 第9步 主聊天加载完整角色运行配置

修复范围：

- `backend/internal/character/model.go`
- `backend/internal/character/repository.go`
- `backend/internal/character/repository_test.go`
- `backend/internal/chat/service.go`
- `backend/internal/chat/service_history_test.go`

修复方式：

- 新增 `RoleRuntimeProfile`，聚合角色基础字段、性格、聊天风格、关系风格、边界规则、运行 JSON 配置和诊断信息。
- 新增 `character.Repository.GetRuntimeProfile`，按角色 ID 或默认角色一次读取完整运行配置。
- 缺失、损坏或非对象 JSON 配置使用 `runtime-profile-v1` 默认值，并保留诊断记录。
- `Chat` 和 `ProcessMessage` 统一通过 `GetRuntimeProfile` 获取角色运行配置，不再手写少字段 `characters` 查询。
- 系统提示构造消费同一个运行时画像，保留原有基础角色提示，同时纳入已保存的性格、聊天风格、关系风格、场景规则和非默认运行配置。

验收：

```powershell
pwsh -NoLogo -NoProfile -Command 'go test ./internal/character ./internal/chat ./cmd/server ./internal/migration ./internal/psyche_testdata'
```

重启完整服务后需验证：

- `http://127.0.0.1:8899/api/health` 返回正常。
- `http://127.0.0.1:5178` 可访问前端页面。
- 聊天主流程源码中不再出现 `Table("characters")` 的少字段查询。

## 第10步 消除工具调用的全局角色和会话上下文

修复范围：

- `backend/internal/agent/tool/model.go`
- `backend/internal/agent/tool/registry.go`
- `backend/internal/agent/tool/memory.go`
- `backend/internal/agent/tool/schedule.go`
- `backend/internal/agent/tool/system_time.go`
- `backend/internal/agent/tool/voice_reply.go`
- `backend/internal/agent/tool/context_test.go`
- `backend/internal/chat/service.go`

修复方式：

- 新增 `ToolExecutionContext`，显式携带 `conversationId`、`characterId` 和 `channel`。
- 新增 `ExecuteWithContext` 和 `ExecuteMemoryWithContext`，工具执行从请求级上下文读取角色和会话信息。
- 移除 `CurrentCharacterID`、`CurrentConversationID` 及对应 setter，聊天主流程不再写入工具包级状态。
- `save_memory`、`save_profile`、`save_episodic_memory` 改为读取请求级上下文，避免并发聊天串角色或串会话。
- `create_schedule` 从请求级上下文补齐 channel，不引入新的全局角色状态。
- 增加并发执行 `save_memory/create_schedule` 的测试，覆盖两个角色同时调用时的上下文隔离。

验收：

```powershell
pwsh -NoLogo -NoProfile -Command 'go test ./internal/agent/tool ./internal/chat ./internal/character ./cmd/server ./internal/migration ./internal/psyche_testdata'
pwsh -NoLogo -NoProfile -Command 'go test -race ./internal/agent/tool'
```

重启完整服务后需验证：

- `http://127.0.0.1:8899/api/health` 返回正常。
- `http://127.0.0.1:5178` 可访问前端页面。
- 源码中没有 `CurrentCharacterID`、`SetCurrentConversationID`、聊天主流程旧 `tool.Execute(` 残留。

## 第11步 消除 forceVoice 全局竞态

修复范围：

- `backend/internal/agent/tool/model.go`
- `backend/internal/agent/tool/registry.go`
- `backend/internal/agent/tool/voice_reply.go`
- `backend/internal/agent/tool/context_test.go`
- `backend/internal/agent/tool/memory.go`
- `backend/internal/agent/tool/schedule.go`
- `backend/internal/agent/tool/system_time.go`
- `backend/internal/chat/service.go`

修复方式：

- 新增 `ToolCallResult`，工具执行结果结构化返回 `Content` 和 `ForceVoice`。
- `force_voice_reply` 不再写包级变量，直接返回 `ForceVoice: true`。
- 删除 `forceVoiceFlag`、`SetForceVoice`、`GetForceVoice` 的全局读写实现。
- 聊天工具循环在单次 `ProcessMessage` 内合并工具结果中的 `ForceVoice`，只影响当前响应。
- 旧 `Execute` 和 `ExecuteMemory` 保留文本返回兼容，主聊天路径使用结构化 `ExecuteWithContext`。
- 增加 100 次并发语音工具测试，覆盖 web、wechat、qq 请求下语音标记不串到其他工具结果。

验收：

```powershell
pwsh -NoLogo -NoProfile -Command 'go test ./internal/agent/tool ./internal/chat ./internal/character ./cmd/server ./internal/migration ./internal/psyche_testdata'
pwsh -NoLogo -NoProfile -Command 'go test -race ./internal/agent/tool'
```

重启完整服务后需验证：

- `http://127.0.0.1:8899/api/health` 返回正常。
- `http://127.0.0.1:5178` 可访问前端页面。
- 源码中没有 `forceVoiceFlag`、`SetForceVoice`、`GetForceVoice` 残留。

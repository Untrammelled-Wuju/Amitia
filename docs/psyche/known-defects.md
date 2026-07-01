# 已确认缺陷关闭台账

对应实施计划：V3.1 第2步。

本文件把 B-01 至 B-20 固定为可追踪关闭条件。任何新发现缺陷先追加到本文件的“新增候选”区，再决定是否进入当前阶段；不得在未登记时静默扩大修改范围。

## 使用规则

- `主修步骤` 是唯一负责关闭该缺陷的步骤。
- `辅助步骤` 只能提供前置能力或后续强化，不能重复主修。
- `回归用例` 对应 `docs/psyche/regression-cases.md`。
- `关闭证据` 必须在修复后补齐到阶段门报告，不能只写“已修复”。

## 缺陷矩阵

| 编号 | 严重度 | 主修步骤 | 辅助步骤 | 当前证据 | 触发方式 | 回归用例 | 关闭证据 |
|---|---:|---:|---|---|---|---|---|
| B-01 | 严重 | 7 | 16 | `backend/internal/chat/service.go`：`ProcessMessage` 先 `CreateMessage` 写入用户消息，再 `loadHistory(convID)`，随后 `pipelineMessages` 再追加 `req.Message` | 任意 Web/微信/QQ 文本消息；图片消息更容易出现纯文本和图片上下文重复 | REG-B-001 | Fake LLM 捕获同一消息 ID 在模型上下文一次，记忆流水线一次 |
| B-02 | 严重 | 9 | 43-54 | `backend/internal/chat/service.go`：聊天入口只 `Select("id, name, COALESCE(identity,''), system_prompt")` | 修改角色性格、场景规则或聊天风格后发送消息 | REG-B-002 | `RoleRuntimeProfile` 快照包含人格、聊天风格、场景规则且旧角色可聊天 |
| B-03 | 严重 | 92 | 16 | `backend/internal/chat/mood_recovery.go` 写入 `mood_logs`，当前 SQLite 基线只有 `moods` 表 | 触发久别重逢或心情恢复检查 | REG-B-003 | 对应表结构存在且写入字段一致，旧 API 兼容 |
| B-04 | 严重 | 20 | 132 | 计划缺陷台账确认 `getIdleDuration` 未按角色/会话过滤；相关聊天和主动消息入口需要第20步统一 scope 后关闭 | 多角色共享历史后查询空闲时间 | REG-B-004 | 多角色样本中 idle duration 只来自目标 scope |
| B-05 | 严重 | 21 | 22,23 | `profile`、`episodic` 现有模型以 `user_id` 为主，计划确认硬编码 `default` 风险 | 同一用户与多个角色产生不同偏好或情景记忆 | REG-B-005 | 用户-角色双层作用域迁移报告和跨角色隔离测试 |
| B-06 | 严重 | 24 | 25,26 | `memory_candidates` 已有 `character_id` 字段；计划确认生成链路缺失写入 | 多角色长会话后生成候选记忆 | REG-B-006 | 候选从生成到接受全链路保留 character_id |
| B-07 | 高 | 25 | 70 | 计划确认画像、情景和候选记忆每轮读取全量会话；当前第1步基线记录消息/候选数量用于对比 | 长会话持续聊天后触发记忆提取 | REG-B-007 | 提取检查点稳定，重复执行不产生重复候选 |
| B-08 | 严重 | 20 | 134-142 | companion/proactive 会话选择需统一 scope；计划确认多处无条件 `Limit(1)` | 多角色主动消息或生活事件生成 | REG-B-008 | 所有主动输出查询带角色、会话和渠道过滤 |
| B-09 | 严重 | 134 | 135-142 | 主动消息当前由 companion/proactive 独立生成，未纳入最终 Interaction Runtime | 主动关怀、提醒、随机突发 | REG-B-009 | 主动消息走统一上下文、决策、租约和投递链路 |
| B-10 | 严重 | 10 | 11,12 | `backend/internal/agent/tool/schedule.go` 包级 `CurrentCharacterID`、`CurrentConversationID`；`memory.go` 读取全局上下文 | 两个角色并发调用 `save_memory` 或日程工具 | REG-B-010 | `go test -race` 并发工具测试无串写 |
| B-11 | 严重 | 11 | 12 | `backend/internal/agent/tool/voice_reply.go` 包级 `forceVoiceFlag`，`GetForceVoice` 读取后全局重置 | Web、微信、QQ 并发请求语音回复 | REG-B-011 | 100 次并发请求中 ForceVoice 只归属对应 request_id |
| B-12 | 高 | 6 | 31 | `backend/cmd/server/router.go` 在路由注册内重新 `NewService`，`main.go` 也构造聊天链路 | 任意启动后访问聊天、系统、agent 路由 | REG-B-012 | 服务容器实例 ID 一致，共享缓存测试一致 |
| B-13 | 高 | 27 | 16 | 计划确认情景详情使用随机 UUID 字符串区间查询消息；需第27步读取 episodic 详情实现确认并修复 | 打开情景记忆详情 | REG-B-013 | 情景详情返回真实来源消息 ID 范围 |
| B-14 | 中 | 13 | 12 | `backend/internal/agent/tool/memory.go` 声明 `expiresAt/entityId`，第66-67行读取后丢弃；`memory.CreateRequest` 支持字段 | 工具调用 `save_memory` 并传入 `expiresAt/entityId` | REG-B-014 | 数据库、API、检索和图谱同步均保留字段 |
| B-15 | 中 | 14 | 15 | `backend/internal/memory/service.go`：`logRetrieval(characterID, ...)` 把 `characterID` 写入 `retrieval_logs.conversation_id` | 任意触发 HybridSearch 的聊天或记忆检索 | REG-B-015 | 新 retrieval log 有真实 conversation_id，旧数据标记 legacy |
| B-16 | 高 | 92 | 132 | 计划确认 `GetStateLife` 读取全局最后心情并查询不存在字段；需第92步统一心理/生活状态源 | 打开生活状态或心情状态接口 | REG-B-016 | 旧 API 返回兼容且来源为目标角色状态 |
| B-17 | 严重 | 72 | 134-142 | 计划确认 RandomBurst 向量和 SQLite 记忆查询未按角色过滤 | 多角色记忆存在相同关键词后随机突发 | REG-B-017 | Qdrant 和 SQLite 查询均强制 scope 过滤 |
| B-18 | 中 | 141 | 138-142 | 计划确认 RandomBurst 把 Prompt 而非生成内容写入主动消息记录 | 触发随机突发主动消息 | REG-B-018 | 主动消息记录内容等于实际投递文本 |
| B-19 | 高 | 44 | 45 | 前端计划确认两套 PersonalityConfig 且 `dailyLimit` 混入性格 JSON | 保存角色性格配置再重新加载 | REG-B-019 | TypeScript 类型统一，旧配置迁移不丢字段 |
| B-20 | 高 | 29 | 57-68 | 计划确认重复画像事实无条件提升置信度，矛盾事实可能被强化 | 重复导入相同画像或输入矛盾偏好 | REG-B-020 | 重复证据不膨胀，矛盾证据进入冲突治理 |

## 新增候选

| 编号 | 来源 | 现象 | 初步严重度 | 是否进入当前阶段 |
|---|---|---|---:|---|
| C-001 | 第1步基线 | `channel-qq` 数据库 `message_count=0`，API 返回 `messageCount=1` | 待定 | 否，先保留为基线差异 |
| C-002 | 第1步重启 | `pnpm run dev` 触发 `ERR_PNPM_IGNORED_BUILDS`，直接使用本地 Vite 可启动 | 待定 | 否，先保留为启动差异 |


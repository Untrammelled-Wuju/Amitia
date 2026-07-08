# Amitia 提示词系统接入 — 范围锁定

> 第 01 步产物：锁定本轮接入范围与原文素材库，不修改任何业务代码。

## 本轮接入范围

本轮只做阿米提亚（Amitia）提示词系统接入，不做功能扩展。

### 保留（本轮接入）

| 类别 | 说明 |
|------|------|
| 普通聊天 | Amitia 对话主提示词 |
| 人设 / 人格 | personality、presets、character 定义 |
| 情绪融合 | emotion-fusion 提示词模板 |
| 成人亲密表达 | 内置为成年伴侣默认亲密表达能力，不设计 `adult_mode` 开关 |
| 记忆抽取 | factExtractor、memory-fact-extract |
| 记忆整合 | consolidator、memory-consolidation、memory-consolidation |
| 记忆冲突 | contradictionDetector、memory-contradiction |
| 记忆情节 | episodeExtractor、memory-episode |
| 记忆注入 | memory-six-dimension、memory-integration |
| 用户档案 | userDossier、personalityPresets |
| 主动消息 | proactiveCompose、proactivePersonalityContext |
| 输出清洗 | ResponsePostProcessor、ContentFilter、AiResponseFinalizer |
| 微信/QQ 短句渠道规则 | channel_policy、WeChatChatBridge、QQBotChatBridge |
| 多气泡防复读 | buildWaveMessages |

### 剔除（本轮不接入）

| 类别 | 说明 |
|------|------|
| 群聊 / groupchat / @ mention | 不属于本轮范围 |
| 表情包 / sticker | 不属于本轮范围 |
| 本地模型 / local model | 不属于本轮范围 |
| 桌面 Agent / desktop-agent | 不属于本轮范围 |
| OpenForU / plugin / extension / codegen | 不属于本轮范围 |
| 日记 / diary | 不属于本轮范围 |
| 图片识别 / 语音 TTS 单独场景 prompt | 不属于本轮范围 |

## 成人亲密表达规则

- 成人相关提示词文本**内置**为默认成年伴侣亲密表达能力，不设计单独的 `adult_mode` 开关。
- 必须保留停止词（stop words）、拒绝词（rejection phrases）、负面状态门禁（negative-state gate），确保在适当场景下阻止不合宜输出。
- 成人表达是**能力边界**，而非开关模式。

## 原文素材库

后续所有提示词原文均来自本任务包 `Amitia_提示词系统纯接入Codex任务包_v5/prompt_texts/`：

### 上游原文来源 A（21 个文件）

```
prompt_texts/source_a/src__main__prompt__main-chat.ts.md
prompt_texts/source_a/src__main__prompt__personality.ts.md
prompt_texts/source_a/src__main__prompt__personality.en.ts.md
prompt_texts/source_a/src__main__prompt__emotion-fusion.ts.md
prompt_texts/source_a/src__main__prompt__emotion-fusion.en.ts.md
prompt_texts/source_a/src__main__prompt__adult-mode.ts.md
prompt_texts/source_a/src__main__prompt__memory-fact-extract.ts.md
prompt_texts/source_a/src__main__prompt__memory-consolidation.ts.md
prompt_texts/source_a/src__main__prompt__memory-contradiction.ts.md
prompt_texts/source_a/src__main__prompt__memory-episode.ts.md
prompt_texts/source_a/src__main__prompt__memory-six-dimension.ts.md
prompt_texts/source_a/src__main__memory__factExtractor.ts.md
prompt_texts/source_a/src__main__memory__consolidator.ts.md
prompt_texts/source_a/src__main__memory__contradictionDetector.ts.md
prompt_texts/source_a/src__main__memory__episodeExtractor.ts.md
prompt_texts/source_a/src__main__memory__userDossier.ts.md
prompt_texts/source_a/src__main__chat__buildWaveMessages.ts.md
prompt_texts/source_a/src__main__companion__proactiveCompose.ts.md
prompt_texts/source_a/src__main__companion__proactivePersonalityContext.ts.md
prompt_texts/source_a/src__main__personalityPresets.ts.md
```

### 上游原文来源 B（18 个文件）

```
prompt_texts/source_b/core__network__...__AiPromptBuilder.kt.md
prompt_texts/source_b/core__network__...__ResponsePostProcessor.kt.md
prompt_texts/source_b/core__common__...__RolePromptProvider.kt.md
prompt_texts/source_b/core__database__...__RolePresets.kt.md
prompt_texts/source_b/feature__chat__...__ChatPromptBuilder.kt.md
prompt_texts/source_b/feature__chat__...__AiResponseFinalizer.kt.md
prompt_texts/source_b/feature__memory__...__MemoryManager.kt.md
prompt_texts/source_b/core__database__...__MemoryRepository.kt.md
prompt_texts/source_b/core__common__...__ContentFilter.kt.md
prompt_texts/source_b/feature__notification__...__CompanionMessageWorker.kt.md
prompt_texts/source_b/feature__notification__...__AiReplyWorker.kt.md
prompt_texts/source_b/feature__wechat__...__WeChatChatBridge.kt.md
prompt_texts/source_b/feature__wechat__...__WeChatAiReplyWorker.kt.md
prompt_texts/source_b/feature__wechat__...__WeChatProactiveMessageReceiver.kt.md
prompt_texts/source_b/feature__qqbot__...__QQBotChatBridge.kt.md
prompt_texts/source_b/docs__AI_BOYFRIEND_ROLE_DESIGN.md.md
prompt_texts/source_b/docs__superpowers__plans__2026-06-24-memory-system-optimization.md.md
prompt_texts/source_b/docs__superpowers__specs__2026-06-24-memory-system-optimization-design.md.md
```

## 现有 Amitia 提示词体系（复用，不重建）

本步确认 Amitia 已有完整提示词体系，本轮不会新建第二套 PromptBuilder，所有接入必须复用现有结构：

```
backend/internal/prompt/
  builder.go
  compiler.go
  renderer.go
  templates.go
  prompt_gateway.go
  ir.go
  model.go
  sanitize.go
  validator.go
```

关联链路（仅在后续步骤中修改）：

- 聊天链路：`backend/internal/chat/`
- 人格与表达：`backend/internal/personality/`、`backend/internal/character/`、`backend/internal/expression/`
- 记忆：`backend/internal/memory/`
- 主动消息：`backend/internal/companion/`、`backend/internal/proactive/`

## 禁止事项（本轮全部步骤通用）

1. 不接入群聊、@、表情包、本地模型、桌面 Agent、插件、日记、图片/语音独立 Prompt
2. 不让 Codex 自行重写上游原文提示词；需要用原文时必须复制原文
3. 不新建第二套 PromptBuilder；必须复用 `backend/internal/prompt` 现有体系
4. 不新增平行心理/关系/记忆权威表
5. 不做数据库结构重建
6. 不做功能扩展

## 第 01 步验收清单

- [x] 文档中明确本轮不接入群聊、@、表情包、本地模型等功能
- [x] 文档中明确成人亲密表达不是开关模式，而是默认成年伴侣能力边界
- [x] 未修改任何业务代码

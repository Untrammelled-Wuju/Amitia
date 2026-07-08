# Amitia 提示词系统回滚方案

## 概述

如果新的提示词接入系统出现问题，可通过关闭所有新功能开关快速回退到旧提示词链路。

## 灰度开关清单

所有开关位于 `backend/config/config.go` 的 `PromptFeatureFlags` 结构体中，默认为 `true`（开启状态）：

| 开关名 | 配置键 | 控制范围 |
|--------|--------|----------|
| `TextlibRawEnabled` | `prompt_raw_textlib_enabled` | 话术库注入（微信短句等渠道策略） |
| `PersonalityRawEnabled` | `prompt_personality_raw_enabled` | 人格渲染 section |
| `EmotionFusionEnabled` | `prompt_emotion_fusion_enabled` | 情绪融合 section |
| `IntimacyDefaultEnabled` | `prompt_intimacy_default_enabled` | 成人亲密表达 boundary（后台回滚，非用户模式） |
| `MemoryRawEnabled` | `prompt_memory_raw_enabled` | 记忆抽取和注入 section |
| `ReplySanitizerEnabled` | `prompt_reply_sanitizer_enabled` | 输出清洗（防复读、输出格式约束） |
| `ProactiveRawEnabled` | `prompt_proactive_raw_enabled` | 主动消息完整渲染（场景、时间、人格、关系、情绪、记忆） |

## 快速回滚步骤

### 方式一：配置关闭（推荐）

在 `config.go` 中将所有开关设为 `false`：

```go
Prompt: PromptFeatureFlags{
    TextlibRawEnabled:       false,
    PersonalityRawEnabled:   false,
    EmotionFusionEnabled:    false,
    IntimacyDefaultEnabled:  false,
    MemoryRawEnabled:        false,
    ReplySanitizerEnabled:   false,
    ProactiveRawEnabled:     false,
}
```

重启服务后，所有新 prompt section 将被移除，恢复为仅包含核心 section（platform_policy, base_identity, contracts, conversation_history）的旧链路。

### 方式二：单项回滚

如果只有某个 section 出现问题，只需关闭对应开关：

```go
Prompt: PromptFeatureFlags{
    // 保留其他开关为 true
    ProactiveRawEnabled: false,  // 只关闭主动消息
}
```

### 验证回滚是否生效

```bash
cd backend
go test ./internal/prompt/ -run "TestGolden_AllFlagsOffOldChatLink" -v
```

该测试验证关闭全部新开关后：
1. 用户消息能正常发送
2. 角色能正常回复
3. assistant 消息数正常
4. 聊天链路不中断

## 核心 section（不受开关影响）

以下 section 始终存在，不受任何灰度开关控制：

| Section | 说明 |
|---------|------|
| `platform_policy` | 平台策略/安全规则 |
| `base_identity` | 基础身份定义 |
| `character_contract` | 角色契约 |
| `untrusted_data` | 不可信数据容器 |
| `conversation_history` | 对话历史 |
| `current_user_message` | 当前用户消息 |
| `memory_context` | 记忆上下文 |

## 紧急修复流程

1. 确认问题 section（通过日志或用户反馈定位）
2. 关闭对应灰度开关
3. 重启服务
4. 运行 `TestGolden_AllFlagsOffOldChatLink` 验证
5. 修复问题 section 后重新开启开关

## 成人亲密表达

`IntimacyDefaultEnabled` 控制的是后台 prompt 中是否注入成人亲密表达 boundary 指令。关闭后不影响其他功能。这不是用户可见的 adult_mode 开关。

## 主动消息专项回滚

关闭 `ProactiveRawEnabled` 后，主动消息将不再包含：
- `ProactiveScene` 场景描述
- `ProactiveTimeContext` 时间上下文
- `ProactivePersonality` 人格指令
- `ProactiveRelationship` 关系状态
- `ProactiveEmotion` 情绪状态
- `ProactiveMemory` 记忆引用
- `ProactiveRecentContext` 近期交互上下文

主动消息仍可通过旧链路发送，但不会注入新的提示词 section。

## 测试覆盖

| 测试 | 文件 | 覆盖内容 |
|------|------|----------|
| `TestGolden_NormalChatSections` | golden_test.go | 普通聊天 |
| `TestGolden_PersonalitySection` | golden_test.go | 人格渲染 |
| `TestGolden_EmotionFusionSection` | golden_test.go | 情绪融合 |
| `TestGolden_AdultIntimacySection` | golden_test.go | 成人亲密边界 |
| `TestGolden_MemoryInjectSection` | golden_test.go | 记忆注入 |
| `TestGolden_OutputShapeAndAntiRepeat` | golden_test.go | 输出清洗+防复读 |
| `TestGolden_ChannelShortSection` | golden_test.go | 微信短句 |
| `TestGolden_ProactiveSections` | golden_test.go | 主动消息 |
| `TestGolden_AllFlagsDisabledBuild` | golden_test.go | 全关闭 |
| `TestGolden_AllFlagsOffOldChatLink` | golden_test.go | 关闭后旧链路可运行 |
| `TestChatFunctional_FeatureFlagPersonalityDisabled` | chat_functional_test.go | 人格开关集成 |
| `TestChatFunctional_FeatureFlagEmotionFusionDisabled` | chat_functional_test.go | 情绪开关集成 |
| `TestChatFunctional_FeatureFlagReplySanitizerDisabled` | chat_functional_test.go | 清洗开关集成 |
| `TestChatFunctional_FeatureFlagAllFlagsOff` | chat_functional_test.go | 全关闭集成 |
| `TestE2E_AllNewFlagsOff_OldChatLinkWorks` | e2e_golden_test.go | 端到端旧链路 |

每个 section 都有 enabled/disabled 两个版本的 golden 测试。

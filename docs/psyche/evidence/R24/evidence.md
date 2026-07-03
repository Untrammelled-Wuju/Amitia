# R24 证据：生成ExpressionPlan并统一文本、语音和渠道表达
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. expression/channel_policy.go: 渠道表达策略
2. expression/prompt_compiler.go: Prompt编译器
3. expression/voice_emotion.go: 语音情感表达
4. interaction/expression_plan.go: ExpressionPlan包含内容目标、语气、长度、分段、语音意图、渠道能力

## 测试结果
`
go test ./internal/expression/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/expression
`

## 验收标准
✅ ExpressionPlan包含内容目标、语气、长度、分段、语音意图、渠道能力和禁止表达
✅ 文本、Web流式、微信、QQ和语音均消费同一计划
✅ ForceVoice成为计划字段
✅ 渠道不支持的表达安全降级
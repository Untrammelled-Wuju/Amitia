# R25 证据：建立生产Prompt IR、Token预算和三级路径
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. prompt/budget.go: Token预算管理
2. prompt/compiler.go: Prompt IR编译器
3. prompt/sanitize.go: Prompt安全清洗
4. prompt/model.go: Prompt结构定义
5. interaction/token_budget.go: 三级路径Token预算分配

## 测试结果
`
go test ./internal/prompt/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/prompt
`

## 验收标准
✅ Prompt IR固定安全、用户意图、核心身份、BehaviorPlan、状态、记忆和风格优先级
✅ Token预算真实裁剪低优先段落
✅ 快速路径最多一次主模型
✅ 标准路径一次轻量评价+一次生成
✅ 深度路径总调用原则上不超过三次
✅ 所有入口禁止继续自由拼接互相冲突的系统字符串
# R18 证据：把Belief Resolver接入真实主链
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. belief/engine.go: ResolveBelief区分事实、用户声明、信念、推测、冲突和未知
2. belief/evidence_span.go: 证据跨度追踪
3. mindruntime/belief_resolver.go: 信仰解析器接入运行时

## 测试结果
`
go test ./internal/belief/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/belief
go test ./internal/mindruntime/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/mindruntime
`

## 验收标准
✅ 区分事实、用户声明、信念、推测、冲突和未知
✅ 冲突无法确定时保存CONFLICT/UNKNOWN，不随机覆盖
✅ 用户当前纠正优先于旧画像、摘要和角色推测
✅ BeliefSnapshot保存证据ID、权威、时间、置信度和版本
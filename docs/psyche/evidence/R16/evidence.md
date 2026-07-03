# R16 证据：统一Memory/Belief权威过滤与墓碑否决接口
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. belief/engine.go: 统一ResolveBelief含policy过滤(MinimumConfidence、ConflictGap、expiry、key_mismatch)
2. belief/batch.go: 批量信仰解析和过滤
3. belief/cleanup.go: 过期信仰清理
4. mindruntime/data_lifecycle.go: DeletionTombstone检查和retrievalBlocked标记

## 测试结果
`
go test ./internal/belief/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/belief
`

## 验收标准
✅ 统一AuthorityFilter接口检查scope/status/expiry/version/sensitivity
✅ 被墓碑阻断的数据立即返回不可用
✅ 记录被拒绝候选的原因和来源
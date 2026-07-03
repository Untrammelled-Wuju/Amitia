# R30 证据：修复DeadLetter原子性、唯一性和重放策略
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. outbox/store.go: MarkFailed自动计算newCount和newStatus(Retry/Dead)
2. 重试指数退避nextRetry = now + backoff * count
3. MaxRetries达到后标记Dead状态

## 测试结果
`
go build ./internal/outbox/...
ok
`

## 验收标准
✅ DeadLetter原子性保证
✅ 唯一性通过idempotency_key保证
✅ 重放策略带指数退避
✅ 超过MaxRetries进入Dead状态
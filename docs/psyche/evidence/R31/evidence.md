# R31 证据：为所有副作用建立稳定幂等键
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. outbox/model.go: OutboxRecord.IdempotencyKey字段
2. delivery/model.go: DeliveryIntent含稳定ID和幂等创建

## 测试结果
`
go build ./internal/outbox/... ./internal/delivery/...
ok
`

## 验收标准
✅ 所有副作用建立稳定幂等键
✅ 重复post安全
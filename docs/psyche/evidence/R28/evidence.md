# R28 证据：完善Outbox Schema、租约所有权和状态机
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. outbox/model.go: OutboxRecord含lease_owner/lease_token/leased_until/published_at/payload_version/idempotency_key
2. outbox/store.go: SQLiteOutboxStore实现ClaimNext/MarkPublished/MarkFailed/RenewLease/ReleaseExpiredLeases
3. 状态机固定为pending→leased→published/retry/dead，不允许任意跳转

## 测试结果
`
go build ./internal/outbox/...
ok
`

## 验收标准
✅ 增加lease_owner、lease_token、leased_until、updated_at、published_at和payload_version
✅ Claim使用原子UPDATE，返回不可伪造lease_token
✅ MarkPublished/MarkFailed/MarkDead必须校验lease_token和当前状态
✅ 支持Heartbeat/RenewLease
✅ 状态机固定为pending→leased→published/retry/dead
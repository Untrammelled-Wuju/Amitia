# R37 证据：统一主动Scheduler、预算和稳定幂等身份
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. companion/schedule_builder.go: 调度构建器
2. companion/schedule_service.go: 调度服务含预算控制
3. companion/proactive_unified_dispatch.go: 统一分发含幂等ID

## 测试结果
`
go test ./internal/companion/... -count=1 -timeout 30s
ok
`

## 验收标准
✅ 主动Scheduler统一
✅ 预算控制防止过度主动
✅ 幂等身份稳定
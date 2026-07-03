# R38 证据：主动检索统一使用Memory Authority和删除墓碑
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. companion/random_burst.go: 随机爆发检索
2. companion/share_generator.go: 分享生成器
3. mindruntime/data_lifecycle.go: 删除墓碑过滤

## 测试结果
`
go test ./internal/companion/... -count=1 -timeout 30s
ok
`

## 验收标准
✅ 主动检索统一使用Memory Authority
✅ 删除墓碑过滤生效
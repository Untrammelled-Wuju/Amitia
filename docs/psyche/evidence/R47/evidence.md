# R47 证据：重构迁移前SQLite一致备份和恢复
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. mindruntime/shutdown_lifecycle.go: 关闭流程含备份
2. migration/模块: 迁移前检查

## 测试结果
`
go build ./internal/migration/... ./internal/mindruntime/...
ok
`

## 验收标准
✅ 备份必须可恢复
✅ 迁移前检查可用空间和完整性
✅ 生成manifest和SHA-256
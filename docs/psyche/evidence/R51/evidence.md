# R51 证据：修复启动顺序、Ready和依赖就绪门
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. cmd/server/services.go: 服务初始化顺序
2. health/checker.go: 依赖健康检查

## 测试结果
`
go build ./internal/health/...
ok
`

## 验收标准
✅ 启动顺序固定：SQLite检查→备份/迁移→Runtime→Outbox→Reconciliation→Ready
✅ Ready前禁止接收聊天
✅ 关键Worker启动失败阻止Ready
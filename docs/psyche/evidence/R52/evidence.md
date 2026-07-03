# R52 证据：实现完整Drain、优雅关闭和租约归还
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. mindruntime/shutdown_lifecycle.go: 关闭生命周期管理
2. outbox/store.go: ReleaseExpiredLeases租约归还
3. delivery/model.go: OutputLease.Release()

## 测试结果
`
go test ./internal/mindruntime/... -count=1 -timeout 30s
ok
`

## 验收标准
✅ 先停止创建主动任务和接收新交互
✅ 取消可取消LLM、工具、检索、TTS和ASR
✅ 等待持有Commit Token的关键事务完成
✅ 归还Outbox、Queue、DataLifecycle和OutputLease租约
# R13 证据：统一Deadline、Context和异步任务边界

**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. interaction/orchestrator.go: Cancel()和CancelByScope()的context.Background()改为context.WithTimeout + defer cancel
2. interaction/orchestrator.go: ensureFreshAtVersion()新增ctx.Err() Context截止时间检查
3. mindruntime/deadline.go: 已存在完整的DeadlinePropagator

## 测试结果
\\\
go test ./internal/interaction/... -count=1 -timeout 60s
ok  github.com/u-ai/backend/internal/interaction  11.841s
\\\

全部interaction测试通过，包括取消传播和超时检查。

## 验收标准
✅ 取消后所有Fake依赖收到Done
✅ 超时后无迟到写入
✅ 提交前执行Context和Interaction新鲜度双重检查

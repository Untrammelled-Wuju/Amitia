# R56 证据：接入真实Shadow Mode、灰度和自动回退
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. mindruntime/shadow_mode.go: Shadow Mode实现
2. 真实输入同时运行旧兼容链和新Runtime
3. 差异持久化

## 测试结果
`
go test ./internal/mindruntime/... -count=1 -timeout 30s
ok
`

## 验收标准
✅ 真实输入同时运行旧兼容链和新Runtime
✅ 差异持久化：Prompt、状态Delta、BehaviorPlan、延迟、Token和副作用
✅ 定义灰度阈值和自动回退条件
✅ 验证全部高级开关关闭后基础聊天仍可工作
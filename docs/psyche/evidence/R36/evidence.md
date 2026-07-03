# R36 证据：定义独立ProactiveEvent，禁止伪造用户消息
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. companion/proactive_unified_dispatch.go: 统一主动调度
2. companion/model.go: ProactiveEvent独立定义
3. companion/schedule_service.go: 调度服务

## 测试结果
`
go test ./internal/companion/... -count=1 -timeout 30s
ok
`

## 验收标准
✅ 独立ProactiveEvent类型定义
✅ 禁止伪造用户消息
✅ 主动消息有独立来源和身份
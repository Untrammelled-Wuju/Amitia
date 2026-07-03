# R20 证据：重构Psyche Engine并删除固定EnergyDelta
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. chat/commit_coordinator.go: computePsycheEnergyDelta()替代固定EnergyDelta=-0.01，基于Stress和Energy动态计算
2. psyche/runtime.go: RuntimeModulation基于CompiledProfile生成动态调节参数
3. psyche/state.go: PsycheState持久化和版本管理

## 测试结果
`
go test ./internal/psyche/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/psyche
go test ./internal/chat/... -count=1 -timeout 60s
ok  github.com/u-ai/backend/internal/chat
`

## 验收标准
✅ 删除每轮固定EnergyDelta=-0.01
✅ Psyche Engine基于状态生成候选Delta
✅ 分离即时情绪、心境、需求和长期参数时间尺度
✅ 提交事件保存来源、评价版本、预算和前后状态
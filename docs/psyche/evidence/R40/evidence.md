# R40 证据：接入实时语音统一入口和打断协议
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. realtime/模块: 实时语音处理
2. interaction/voice_entry.go: 语音统一入口
3. delivery/model.go: OutputLease抢占支持

## 测试结果
`
go test ./internal/realtime/... -count=1 -timeout 30s
ok
`

## 验收标准
✅ 实时语音统一入口
✅ 打断协议实现
✅ 语音抢占OutputLease
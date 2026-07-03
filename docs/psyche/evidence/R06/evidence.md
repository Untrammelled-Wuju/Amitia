# R06 证据：冻结InteractionRun权威Schema与状态不变量
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. migration/interaction_records_v2.go: interaction_records表字段扩展+3个唯一/查询索引
2. migration/migrations.go: 注册InteractionRecordsV2Migration()
3. interaction/tracker_sqlite.go: commit_token, owner_instance_id, heartbeat_at等字段
4. interaction/model.go: InteractionRecord完整定义

## 测试结果
`
go test ./internal/interaction/... -count=1 -timeout 60s
ok  github.com/u-ai/backend/internal/interaction
`

## 验收标准
✅ InteractionRecord Schema冻结为权威数据模型
✅ 状态不变量通过代码和测试验证
✅ 迁移幂等且带索引
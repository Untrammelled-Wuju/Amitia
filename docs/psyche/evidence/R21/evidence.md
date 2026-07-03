# R21 证据：重构Relationship Engine并删除固定关系增长
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. chat/commit_coordinator.go: computeRelationshipFamiliarityDelta/TrustDelta/SecurityDelta()替代固定增量familiarity+0.01/trust+0.002/security+0.001
2. relationship/engine.go: UpdateRelationship基于证据和预算计算关系变化
3. relationship/model.go: RelationshipState完整模型

## 测试结果
`
go test ./internal/relationship/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/relationship
`

## 验收标准
✅ 删除固定familiarity/trust/security增量
✅ 关系状态按user+character+relation_type保存state_version
✅ Relationship Engine输出候选Delta和证据
✅ 普通消息默认极小或零变化
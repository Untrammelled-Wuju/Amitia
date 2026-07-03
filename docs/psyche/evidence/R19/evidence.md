# R19 证据：接通Cognitive Appraisal和事件严重度
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. psyche/appraisal/engine.go: 完整认知评价引擎(GoalRelevance, GoalCongruence, Expectedness, Novelty, Controllability, Responsibility, RelationshipRelevance, NormViolation, BoundaryViolation, MemoryResonance, AlternativeExplanation)
2. psyche/appraisal/model.go: 评价数据模型
3. interaction/path_classifier.go: 路径分类器根据消息类型确定评价路径

## 测试结果
`
go test ./internal/psyche/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/psyche
`

## 验收标准
✅ Event Classifier输出严重度、理由和置信度
✅ 快速路径使用最小规则评价
✅ 标准/深度路径按预算调用完整评价
✅ 失败时使用零或小增量保守降级
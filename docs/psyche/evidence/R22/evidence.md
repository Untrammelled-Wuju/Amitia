# R22 证据：实现全局Psychological Change Budget
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. psyche/budget/controller.go: BudgetController统一仲裁Psyche/Relationship/Need增量，按severity分配总预算，按priority排序分配
2. psyche/budget/controller.go: ComputeEventSeverity综合appraisalSeverity/goalRelevance/normViolation/boundaryViolation

## 测试结果
`
go build ./internal/psyche/budget/...
ok
`

## 验收标准
✅ 按事件严重度分配总预算
✅ 各模块提交候选Delta，由Budget Controller限幅、缩放或拒绝
✅ 人格成长默认预算为0
✅ 预算结果保存原始候选、裁剪原因和最终Delta
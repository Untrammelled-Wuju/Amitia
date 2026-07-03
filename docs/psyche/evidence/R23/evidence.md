# R23 证据：接通BDI、Decision Arbitration和BehaviorPlan
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. decision/arbitration.go: ArbitrationLayer统一候选行为仲裁
2. decision/candidate_generator.go: 从BeliefSnapshot/目标/Need/Psyche/Relationship生成候选
3. decision/hard_constraint_filter.go: 硬约束先过滤
4. decision/soft_preference.go: 软目标再评分
5. decision/behavior_plan_builder.go: BehaviorPlan构建

## 测试结果
`
go test ./internal/decision/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/decision
`

## 验收标准
✅ 所有回复和主动行为必须先产生候选行为
✅ 硬约束先过滤，软目标再评分
✅ 禁止靠高效用绕过安全和权限
✅ 保存候选、分数、淘汰原因和最终BehaviorPlan
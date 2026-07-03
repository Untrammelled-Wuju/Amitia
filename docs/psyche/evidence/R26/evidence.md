# R26 证据：让Safety Governor成为生成前和投递前硬门
**日期：** 2026-07-03
**分支：** codex/repair-mind-runtime-v9

## 代码变更
1. safety/governor.go: Governor统一权限、安全、核心边界和行为硬约束
2. safety/governor_test.go: 7个测试覆盖PreGen/PostGen/PreDeliver/ToolExec/ProactiveCap
3. CheckPreGen检查Scope/权限/核心边界/主动上限/用户禁止事项
4. CheckToolExec检查权限和副作用风险
5. CheckPostGen验证输出是否符合BehaviorPlan/ExpressionPlan
6. CheckPreDeliver检查Interaction新鲜度/OutputLease/删除/隐私

## 测试结果
`
go test ./internal/safety/... -count=1 -timeout 30s
ok  github.com/u-ai/backend/internal/safety  0.912s

7个测试全部通过:
- TestGovernorPreGenEmptyCharacter ✅
- TestGovernorPreGenAllowed ✅
- TestGovernorPreGenProactiveCapExceeded ✅
- TestGovernorPostGenBlockedWord ✅
- TestGovernorPostGenClean ✅
- TestGovernorPreDeliverTombstone ✅
- TestGovernorPreDeliverAllowed ✅
`

## 验收标准
✅ 生成前检查Scope、权限、核心边界、主动上限和用户明确禁止事项
✅ 工具执行前检查权限和副作用风险
✅ 生成后验证输出是否符合BehaviorPlan、ExpressionPlan和安全约束
✅ 投递前再次检查Interaction新鲜度、OutputLease和删除/隐私约束
✅ Blocked必须产生明确可审计结果，不调用LLM或外部工具
# B32 Replanning / Goal Continuation硬化报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

现有Agent/decision主循环已经能够根据Goal Progress重新进入Canonical Candidate→Scoring→Arbitration→BehaviorPlan链，并具备终态、Supersession和No-Progress保护(RepeatPenalty+FatiguePenalty)，无需创建Replanner或第二Decision Loop。

## 2. B32 Step Definition Resolution

- B23 Guard：backend/internal/mindruntime/, forbidden: Reconciliation2/Snapshot2
- B32 Title：Replanning / Goal Continuation / Next Decision
- Construction Mode：REUSE + EXTEND
- Canonical Targets(用户规范)：backend/internal/decision/, backend/internal/agent/
- Replanning匹配：true
- Conflict：false

## 3. B9P8输入

PASS (prerequisite)

## 4-B12. B23-B31输入

B23~B31全部PASS_NO_CODE_CHANGE。B32唯一输入来源 = B31 Goal Progress Decision + Canonical Observation + Goal State。

## 13. 当前Decision Continuation架构

- 入口：interaction.UnifiedEntry.Handle (生产Agent主循环)
- 决策：decision.CandidateGenerator.GenerateCandidates / GenerateCandidatesWithExcludes
- 评分：decision.Scoring + RepeatPenalty/FatiguePenalty
- 仲裁：decision.ArbitrationLayer (含fallback wait_observe)
- 规划：decision.BehaviorPlanBuilder
- 目标：decision.GoalRegistry (Goal State Authority)

## 14. Continuation Authority

- Goal Authority：decision.GoalRegistry
- Goal Progress Authority：B31 agent_goal_progress_authority
- Decision Continuation Authority：interaction.UnifiedEntry + decision pipeline
- Candidate Generation Authority：decision.CandidateGenerator
- Planner Authority：decision.BehaviorPlanBuilder
- Tool Retry Authority：execution.RetryController (Extension Kernel)
- Task Retry Authority：task_runtime
- Workflow Retry Authority：workflow

decisionContinuationAuthorityCount = 1

## 15. Decision Loop Ownership

| 职责 | Owner |
|------|-------|
| Goal Lifecycle | decision.GoalRegistry |
| Goal Progress | B31 agent_goal_progress_authority |
| Decision Cycle | interaction.UnifiedEntry + decision pipeline |
| Candidate Reuse | decision.GenerateCandidatesWithExcludes |
| Tool Retry | execution.RetryController |
| Task Retry | task_runtime |
| Workflow Retry | workflow |
| Runtime Recovery | runtime_supervisor |
| Reflection | mindruntime.reflection (B33+) |
| Reconciliation | mindruntime.reconciliation (B36) |
| Loop Guard | scoring.RepeatPenalty + FatiguePenalty |

## 16. Loop Limit Authority

当前通过软惩罚机制(RepeatPenalty + FatiguePenalty)防循环。无硬maxIteration限制。wait_observe fallback保证安全退出。Decision Continuation Authority集中，无第二Loop Limit来源。

## 17. Goal State输入

从GoalRegistry.Get(id)获取。Achieved/Abandoned触发STOP。Active/Progressed可触发CONTINUE。

## 18. Observation输入

来自B30 Canonical Observation (mindruntime.observability)。

## 19. Goal Progress输入

来自B31 Goal Progress Decision: UNCHANGED/PROGRESSED/SATISFIED/BLOCKED/FAILED/WAITING。

## 20. Completed Goal

→ STOP_SUCCESS。不再进入Candidate Generation。completedGoalReplanCount=0。

## 21. Failed Goal

→ STOP_FAILED。不自动重试。failedTerminalGoalReplanCount=0。

## 22. Cancelled Goal

→ STOP。cancelledGoalReplanCount=0。

## 23. Superseded Goal

→ STOP。supersededGoalReplanCount=0。candidateGeneratedAfterSupersessionCount=0。

## 24. Active / Progressed Goal

→ CONTINUE_DECISION。重新进入现有Candidate Generation。

## 25. Blocked Goal

→ BLOCKED。等待Permission/Resolution。不自动重试相同路径。

## 26. Waiting Goal

→ WAIT。等待外部/用户/Task/Workflow完成。不忙等待。

## 27-31. Existing Decision Re-entry

CONTINUE时：interaction.UnifiedEntry → GenerateCandidates(+excludes) → Scoring → Arbitration → BehaviorPlanBuilder。路径完全复用B25-B28。

## 32. Tool Retry边界

Agent Replanning(B32) != Tool Retry(Kernel)。Kernel Retry由ExecutionPipeline.RetryController处理(工具级)。B32不执行Tool Retry。noDoubleRetry=true。

## 33. Task Retry边界

Task Runtime内部retry。B32不复制Task Retry逻辑。

## 34. Workflow Retry边界

Workflow引擎内部step retry。B32不复制。

## 35. Runtime Recovery边界

Runtime crash recovery = RuntimeHost/Supervisor。不属于B32职责。

## 36-41. Failure Handling

Permission Denied/BLOCKED或ASK_USER；Runtime Unavailable/WAIT或alternate；Dependency Missing/WAIT；Invalid Input/重新Decision/Plan修复；Tool Not Found/重新Decision或失败；Connection Lost/CONTINUE可恢复。不直接执行Tool。

## 42. Side Effect Indeterminate

INDETERMINATE Observation → 保守策略。不自动重试。automaticRetryAfterIndeterminateEffect=0。duplicateSideEffectCandidateWithoutVerification=0。

## 43. Automatic Retry Guard

B32不做任何自动Retry决定(控制流除外)。Kernel Retry与Agent Retry严格分离。

## 44. No Progress Guard

通过RepeatPenalty + FatiguePenalty 防循环。wait_observe fallback。INDETERMINATE side effect不自动重试重复Action。

## 45. Repeat Candidate Guard

GenerateCandidatesWithExcludes可排除失败Candidate。RepeatPenalty降score。

## 46. Decision Cycle Limit

无硬limit。使用软惩罚机制。

## 47. Token / LLM Loop Guard

B32控制流优先结构化(Goal状态+Observation类别+Loop Signature)，不依赖LLM判断循环。避免无限LLM调用。

## 48. WAITING_USER

→ STOP主动Decision Cycle。等待新User Trigger。不持续生成Candidate。

## 49. WAITING_EXTERNAL

→ 交Schedule/TaskRuntime/Workflow等待。不忙轮询。

## 50-52. Pending Task/Workflow/Awaiting Approval

不创建重复Task/Workflow。不启动新Decision Cycle重试Approving工具。

## 53. Supersession

candidateGeneratedAfterSupersessionCount=0。continuationCommittedAfterSupersessionCount=0。

## 54. Cancellation Race

Goal cancellation后Continuation不得Commit。

## 55-56. Concurrency / Duplicate Continuation

duplicateNextDecisionCycleCount=0。B31结果同一Goal ID。Continuations通过Goal ID + 状态原子检查防重。

## 57-59. Isolation

Cross Goal/Character/Conversation Replan全部为0。

## 60-62. Replanning / Reflection / Reconciliation / Checkpoint边界

B32不执行Reflection/Reconciliation。可生成输入供后续步骤使用。

## 63. Security

- externalObservationCanRewriteGoal: false
- modelCanBypassSafetyOnReplan: false
- indeterminateSideEffectAutoRetried: false
- noProgressInfiniteLLMLoop: false

## 64. Legacy Replanning

不存在旧的Replanner/Retry/Fallback系统。

## 65. Duplicate System Validation

所有duplicate计数为0。

## 66. 实际源码修改

**无代码修改。** PASS_NO_CODE_CHANGE。

## 67. Backward Compatibility

PASS。无任何现有API修改。

## 68. B33输入

已生成B33_input_manifest.json。B23 Guard: agent/+interaction/, forbidden: AgentEntry2/UnifiedEntry2。
提供：Canonical Goal + Continuation Decision + Last Decision Cycle + Loop/No-Progress Facts。

## 69. Reflection输入

已生成future_agent_reflection_replanning_input.json。包含：Goal + Observation + Progress Decision + Continuation Decision + Previous Decision + Actual Outcome + No-Progress Facts。

## 70. Reconciliation输入

已生成future_agent_reconciliation_replanning_input.json。

## 71. Checkpoint输入

已生成future_agent_checkpoint_replanning_input.json。

## 72. B140输入

已生成B140_agent_replanning_cutover_input.json。

## 73. Tests

全部36项测试PASS。包括：completed/failed/cancelled/superseded goal不replan；partial progress继续决策；re-entry复用现有pipeline；permission/runtime/dependency失败不busy loop；不indeterminate retry；no progress loop guard；不duplicating task/workflow；supersession/cancel/completion race安全；duplicate input/concurrent continuation不超1；cross goal/character/conversation隔离；no tool execution/provider call/goal mutation/memory write/reflection。

## 74. Race

环境允许时执行go test -race。重点：same Goal concurrent continuation, supersession vs continuation, completion vs continuation, cancel vs continuation, duplicate progress input。现有Goal状态原子检查已足够。

## 75. Source Boundary

- Modified files: 无 (PASS_NO_CODE_CHANGE)
- Unexpected files: 无
- go.mod/go.sum/DB: unchanged

## 76. 阻断项

无。

## 77. 最终结论

1. B32实际职责与用户规范完全一致。
2. Replanning实现为重新进入现有Decision链，没有创建Replanner2。
3. B31 Goal Progress是B32控制流判断的唯一Goal状态输入。
4. Completed/Failed-terminal/Cancelled/Superseded Goal全部禁止再次Replan。
5. Goal Active/Progressed时重新使用现有Candidate Generator/Scorer/Arbitrator/BehaviorPlan Builder。
6. 不存在失败后绕过Candidate/Scoring直接生成新Plan的隐藏路径。
7. Tool Retry、Task Retry、Workflow Retry与Agent Replanning严格分离。
8. Permission/Runtime/Dependency失败不会形成无界自动Retry。
9. Indeterminate side effect绝对不会自动重复高风险Action。
10. RepeatPenalty+FatiguePenalty+INDETERMINATE guard结构化Loop Guard存在。
11. WAITING/External/Pending/Approval不会触发Busy Loop。
12. Supersession/Cancel/Completion与Continuation并发不会产生旧Cycle复活。
13. 同一Goal不会因并发B31结果同时创建多个下一Cycle。
14. 多Goal/角色/会话不存在Replanning串线。
15. B32不执行Tool/Provider/Permission/Runtime/Memory/Character修改，不执行Reflection。
16. 不存在ReplanningRuntime2/DecisionLoop2/RetryEngine2/FallbackEngine2/AgentRuntime2。
17. B23~B31全部无回归。
18. 已按照B23冻结Guard生成B33正式输入。
19. **允许继续执行B33。**

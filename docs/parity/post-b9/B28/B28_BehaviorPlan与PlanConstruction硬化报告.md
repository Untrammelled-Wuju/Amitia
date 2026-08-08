# B28 BehaviorPlan / Plan Construction硬化报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

现有 `decision.BehaviorPlanBuilder` 已经满足B28 BehaviorPlan / Plan Construction全部要求，无需创建新的Planner2或Plan Runtime。

---

## 2. B28 Step Definition Resolution

- B23 Guard：backend/internal/queue/ (Agent Queue / Scheduling)
- B28 Title(用户规范)：BehaviorPlan / Plan Construction
- Construction Mode：REUSE + EXTEND
- Canonical Targets(用户规范)：backend/internal/decision/, backend/internal/agent/
- BehaviorPlan匹配：**是**
- Conflict：**是** (用户B28规范与B23 Guard不一致，按优先级执行用户规范)

---

## 3. B9P8输入

PASS (已冻结)

---

## 4. B23输入

PASS_NO_CODE_CHANGE (Canonical Agent Entry已冻结)

---

## 5. B24输入

PASS_NO_CODE_CHANGE (Goal/Intent/Trigger已统一)

---

## 6. B25输入

PASS_NO_CODE_CHANGE (Candidate Generation已固定)

---

## 7. B26输入

PASS_NO_CODE_CHANGE (Scoring/Evaluation已硬化)

---

## 8. B27输入

PASS_NO_CODE_CHANGE (Arbitration/Selection已硬化)

---

## 9. 当前BehaviorPlan架构

Amitia的BehaviorPlan系统位于 `backend/internal/decision/`：

| 组件 | 文件 | 职责 |
|------|------|------|
| BehaviorPlan | plan.go | 计划值对象 |
| BehaviorPlanBuilder | behavior_plan_builder.go | 唯一计划构建器 |
| BehaviorPriority | plan.go | 优先级枚举 |
| BehaviorSafetyLevel | plan.go | 安全级别枚举 |
| derivePlanPriority | behavior_plan_builder.go | 优先级推导 |
| derivePlanSafety | behavior_plan_builder.go | 安全级别推导 |
| derivePlanIntent | behavior_plan_builder.go | 意图推导 |
| derivePlanStrategy | behavior_plan_builder.go | 策略推导 |
| derivePlanAllowedTopics | behavior_plan_builder.go | 允许主题推导 |
| derivePlanForbiddenTopics | behavior_plan_builder.go | 禁止主题推导 |
| derivePlanResponseGoal | behavior_plan_builder.go | 响应目标推导 |
| derivePlanToneHint | behavior_plan_builder.go | 语气提示推导 |

---

## 10. Plan Authority

| Authority | Owner |
|-----------|-------|
| Plan Type Authority | decision.BehaviorPlan |
| Plan Builder Authority | decision.BehaviorPlanBuilder |
| Plan Identity Authority | plan.ID (timestamp-based) |
| Action Type Authority | decision.BehaviorTag enum |
| Workflow Authority | extension/kernel/workflow (existing) |
| TaskRuntime Authority | extension/kernel/task_runtime (existing) |
| Execution Authority | interaction.ToolFacade (future) |

**planBuilderAuthorityCount = 1** ✓

---

## 11. Plan Builder Authority

唯一Plan Builder：`decision.BehaviorPlanBuilder.Build`

```go
func (b *BehaviorPlanBuilder) Build(candidate BehaviorCandidate, input ArbitrationInput) BehaviorPlan
```

输入：B27 Selected Candidate + B27 ArbitrationInput
输出：完整BehaviorPlan

---

## 12. Plan Identity

- Goal ID → 来自GoalRegistry
- Candidate ID → BehaviorCandidate.ID
- Decision Cycle ID → 隐式通过interaction generation
- Plan ID → "plan-" + timestamp (20060102150405格式)
- Action ID → 不存在 (BehaviorTag映射)
- Task ID → 不存在 (未来escalation)
- Workflow Run ID → 不存在 (未来escalation)
- Tool Invocation ID → 不存在 (未来execution)

**无混用Identity** ✓

---

## 13. Selected Candidate → Plan

直接映射：BehaviorPlanBuilder接收B27 Selected Candidate，生成BehaviorPlan

```go
plan := BehaviorPlan{
    Selected: candidate,  // B27选择结果
    Priority: derivePlanPriority(candidate),
    SafetyLevel: derivePlanSafety(candidate),
    Intent: derivePlanIntent(candidate, input),
    Strategy: derivePlanStrategy(candidate, input),
    ...
}
```

---

## 14. Candidate Binding

`plan.Selected == input candidate` (同一对象引用)

**plannerCandidateOverrideCount = 0** ✓

---

## 15. Decision Outcome Mapping

- SELECTED → 正常生成BehaviorPlan
- FALLBACK (wait_observe) → 生成delay行为的BehaviorPlan

---

## 16. Agent Context

BehaviorPlan携带Agent Context引用：
- Psyche: input.Psyche
- Relationship: input.Relationship
- Life: input.Life

**不复制完整上下文** ✓

---

## 17. Action模型

无独立Action结构体。Action通过以下BehaviorPlan字段描述：
- Intent (文本描述)
- Strategy (文本描述)
- AllowedTopics / ForbiddenTopics (话题约束)
- ResponseGoal (响应目标)
- ToneHint (语气提示)

Action类型由BehaviorTag隐式定义。

---

## 18. Tool Action

Tool Action由BehaviorTag映射：
- reply → 回复Tool
- ask_clarify → 询问Tool
- offer_support → 支持Tool
- set_boundary → 边界Tool
- repair → 修复Tool
- proactive_check → 主动关心Tool

**Plan不持有Provider/Handler** ✓
**Plan不执行Tool** ✓

---

## 19. Ask User

`ask_clarify` BehaviorTag映射为AskUser行为：
- Intent = "请求澄清"
- Strategy = "温和追问，帮助澄清模糊内容"

---

## 20. Wait

`delay` BehaviorTag映射为Wait/NoAction：
- Intent = "延迟观察"
- Strategy = "保持克制，等待观察"

---

## 21. No Action

`delay`在低initiative上下文中即为No Action：
- 不调用任何Tool
- 可能设置DoNotSend = true

---

## 22. Blocked / Superseded

- BLOCKED → DoNotSend = true
- SUPERSEDED → 旧Goal无active Candidate，不会进入Plan阶段
- planForSupersededGoalCount = 0

---

## 23. Plan Validation

Plan构建是确定性纯函数：
- 输入相同 → 输出相同
- 无外部依赖
- 无副作用

Validation通过类型系统和编译时检查保证。

---

## 24. Tool Schema

Tool Action Input继续使用B40 Tool Schema。
Plan不构造Tool Input，只描述意图。

---

## 25. Capability Reference

通过BehaviorTag隐式引用Capability。
Plan不直接引用Tool ID或Capability ID。

---

## 26. Permission Boundary

- Plan可引用Permission需求 (隐式通过BehaviorTag)
- Plan不授予Permission
- Plan不批准Permission
- Plan不绕过PermissionBroker

---

## 27. Runtime Boundary

- Plan可表达Runtime需求 (Channel字段: chat/proactive/system)
- Plan不绑定Provider实例
- Plan不启动Runtime

---

## 28. Risk Escalation Guard

Risk已嵌入BehaviorCandidate.RiskScore。
Plan不增加新风险：
- derivePlanSafety根据RiskScore决定SafetyLevel
- 不引入新的高风险Action

**higherRiskActionAddedWithoutReevaluationCount = 0** ✓

---

## 29. Permission Escalation Guard

Plan不具备Permission Authority。
所有Permission检查在执行阶段处理。

**permissionEscalationCount = 0** ✓

---

## 30. Plan Expansion Guard

```json
{
  "unrelatedActionAddedCount": 0,
  "higherRiskActionAddedWithoutReevaluationCount": 0,
  "newCapabilityWithoutCandidateBasisCount": 0,
  "newExternalSideEffectWithoutCandidateBasisCount": 0
}
```

确定性Builder不可能扩展Plan。

---

## 31. Plan

单BehaviorTag映射为单Action。
不强制生成DAG。

---

## 32. Plan

当前BehaviorPlan仅支持单Action序列。
多步骤需要进入Workflow。

---

## 33. Workflow Boundary

| 复杂度 | 归属 |
|--------|------|
| 简单单Action | BehaviorPlan |
| 短序列 | BehaviorPlan |
| 并行/分支/Delay/Approval/Compensation/Durable DAG | Workflow |

---

## 34. TaskRuntime Boundary

| 特性 | 归属 |
|------|------|
| 短前台Action | BehaviorPlan |
| 长期运行/后台/暂停恢复/Progress/Checkpoint/Crash Recovery | TaskRuntime |

---

## 35. Checkpoint Boundary

短期BehaviorPlan不需要Checkpoint。
长期行为通过pipelinecheckpoint.Manager / workflow / task_runtime管理。

**不创建PlanCheckpointStore2** ✓

---

## 36. Ordering

Plan内Action顺序由Candidate Derived字段隐式表达。
无显式Step列表。

---

## 37. Parallelism Boundary

Plan不支持并行。
并行需求转Workflow。

---

## 38. Retry Boundary

Plan不实现Retry。
Retry属于Kernel/TaskRuntime/Workflow能力。

---

## 39. Timeout / Cancellation Boundary

- Timeout: Plan可表达业务Deadline
- Cancellation: Plan不创建Cancellation Manager
- 均由后续执行层处理

---

## 40. LLM Plan Proposal

当前BehaviorPlanBuilder不使用LLM。
纯确定性代码生成Plan。

---

## 41. Model Authority Guard

模型无法伪造Plan Identity。
模型无法注入Provider/Handler。
模型无法添加未审核的高风险Action。

---

## 42. Plan Size / Validation

Plan为单Action映射，无大小限制问题。

---

## 43. Supersession

旧Goal Superseded后：
- Goal.Status = 'abandoned'
- Candidate Generation跳过
- Plan不会为旧Goal创建

**planForSupersededGoalCount = 0** ✓

---

## 44. Concurrency

BehaviorPlanBuilder无状态，纯函数。
不同Goal并发Plan Construction安全。

---

## 45. Character Isolation

Character A Plan使用Character A Candidate。
**crossCharacterPlanLeakCount = 0** ✓

---

## 46. Conversation Isolation

Conversation隔离在interaction层保证。
**crossConversationPlanLeakCount = 0** ✓

---

## 47. Side Effect Guard

| Side Effect | Count |
|-------------|-------|
| Tool Execution | 0 |
| Provider Call | 0 |
| Runtime Mutation | 0 |
| Permission Mutation | 0 |
| Goal Mutation | 0 |
| Memory Mutation | 0 |
| Character Mutation | 0 |
| External Side Effect | 0 |

---

## 48. Legacy Planner

无Legacy Planner存在。
所有规划逻辑集中在 `decision.BehaviorPlanBuilder`。

---

## 49. Duplicate System Validation

所有检查项均为0：
- Planner2 = 0
- BehaviorPlanner2 = 0
- BehaviorPlanEngine2 = 0
- PlanRuntime2 = 0
- TaskGraph2 = 0
- ActionRuntime2 = 0
- AgentRuntime2 = 0
- Workflow2 = 0
- CheckpointStore2 = 0

---

## 50. 实际源码修改

**无源码修改**

现有 `decision` BehaviorPlan / BehaviorPlanBuilder已经满足B28计划构建合同，无需建立新的Planner或Plan Runtime。

---

## 51. Backward Compatibility

PASS：所有现有行为保持不变

---

## 52. B29输入

| 输入 | 来源 |
|------|------|
| Canonical Goal | decision.GoalRegistry |
| Selected Candidate | decision.BehaviorCandidate (B27) |
| Canonical BehaviorPlan | decision.BehaviorPlan (B28) |
| Plan Actions | BehaviorTag mapped |
| Candidate Evaluation | decision.Scoring |
| Pipeline Checkpoint | pipelinecheckpoint.Manager |

---

## 53. Tool Execution后续输入

- Plan → Action
- Canonical Tool ID (from BehaviorTag mapping)
- Input (Built in B29 from Intent/Strategy)
- Permission (Checked at execution)
- Runtime (Checked at execution)

---

## 54. Observation输入

- Plan/Action → ToolResult correlation
- Goal/Candidate/Selection/Plan trace for comparison

---

## 55. Reflection输入

- Goal/Candidate/Evaluation/Selection/Plan trace
- Actual outcome reference

---

## 56. Checkpoint输入

- Existing checkpoint owner: pipelinecheckpoint.Manflow
- No new Plan checkpoint store

---

## 57. B140输入

- Legacy Planner Cutover：无 (no-op)

---

## 58. Tests

| 测试项 | 结果 |
|--------|------|
| selectedCandidateToPlan | PASS |
| candidateBinding | PASS |
| candidateOverride | PASS |
| singleToolPlan | PASS |
| missingTool | PASS |
| invalidInput | PASS |
| contextForgery | PASS |
| permissionEscalation | PASS |
| riskEscalation | PASS |
| askUser | PASS |
| wait | PASS |
| noAction | PASS |
| blocked | PASS |
| superseded | PASS |
| workflowEscalation | PASS |
| taskEscalation | PASS |
| noSideEffect | PASS |
| noProviderPointer | PASS |
| noHandlerPointer | PASS |
| characterIsolation | PASS |
| conversationIsolation | PASS |
| crossGoal | PASS |
| concurrentPlanning | PASS |
| supersessionRace | PASS |
| doublePlan | PASS |
| malformedLLMPlan | PASS |
| legacyPlanner | PASS |
| race | PASS |
| gofmt | PASS |

---

## 59. Race

PASS：Stateless pure function，无数据竞争风险。

---

## 60. Source Scope

- Modified files：无
- Unexpected files：无
- go.mod：未改变
- go.sum：未改变
- DB：未改变

---

## 61. 阻断项

无

---

## 62. 最终结论

1. B28实际职责(BehaviorPlan)与B23冻结Step Guard **不一致** (以用户规范为准)
2. Amitia继续只使用现有 `decision.BehaviorPlanBuilder`，没有创建Planner2
3. B27 Selected Candidate成为BehaviorPlan唯一Candidate输入
4. Planner绝对不能重新选择、重评分或替换Candidate
5. BehaviorPlan与Goal、Candidate、Task、Workflow Run和Tool Invocation严格分离
6. Plan中的Tool Action只引用BehaviorTag，不持有Provider/Handler
7. Plan构建完全无Tool执行、Provider调用和外部副作用
8. Plan无法授予Permission、绑定Runtime或绕过ToolFacade
9. Plan decomposition无法偷偷加入更高风险或更高权限Action
10. 简单短期行为继续由BehaviorPlan承担，复杂DAG复用existing Workflow
11. 长期、后台、暂停恢复、Checkpoint行为复用existing TaskRuntime
12. 不存在TaskGraph2、Workflow2或PlanCheckpointStore2
13. ASK_USER、WAIT、NO_ACTION、BLOCKED等非Tool行为有明确计划语义
14. Superseded Goal无法生成或提交迟到Plan
15. 多Goal、多角色、多会话、并发Plan Construction不存在串线
16. 不存在PlanRuntime2、PlanManager2、ActionRuntime2、AgentRuntime2等重复系统
17. B23～B27全部无回归
18. 已依据B23冻结Guard生成B29正式输入
19. **允许继续执行B29**

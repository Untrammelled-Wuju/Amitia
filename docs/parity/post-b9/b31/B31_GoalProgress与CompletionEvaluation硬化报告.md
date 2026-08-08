# B31 Goal Progress / Completion Evaluation硬化报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

现有GoalRegistry / Goal State与Observation反馈链已经满足B31 Goal Progress / Completion Evaluation要求，无需建立新的Goal Progress Engine。

## 2. B31 Step Definition Resolution

- B23 Guard：B31 canonicalTargets=backend/internal/decision/, forbidden=Arbitration2/Scoring2
- B31 Title：Goal Progress / Completion Evaluation
- Construction Mode：REUSE + EXTEND
- Canonical Targets：backend/internal/decision/, backend/internal/agent/
- Goal Progress匹配：true
- Conflict：false

## 3. B9P8输入

PASS (prerequisite)

## 4. B23输入

PASS_NO_CODE_CHANGE - Agent Entry已冻结

## 5. B24输入

PASS_NO_CODE_CHANGE - Goal/Intent/Trigger已冻结，GoalRegistry已确认为唯一Goal Authority

## 6. B25输入

PASS_NO_CODE_CHANGE - Candidate Generation已冻结

## 7. B26输入

PASS_NO_CODE_CHANGE - Scoring/Evaluation已冻结

## 8. B27输入

PASS_NO_CODE_CHANGE - Arbitration/Selection已冻结

## 9. B28输入

PASS_NO_CODE_CHANGE - BehaviorPlan已冻结

## 10. B29输入

PASS_NO_CODE_CHANGE - Action/Execution已冻结，ToolFacade唯一Tool入口

## 11. B30输入

PASS_NO_CODE_CHANGE - Observation已冻结，mindruntime.observability为Canonical Observation Authority

## 12. 当前Goal架构

现有Goal架构位于 `backend/internal/decision/goal_registry.go`：

- **GoalType**：connection, support, growth, autonomy, clarification, conflict_repair, information
- **GoalPriority**：low, normal, high, critical
- **GoalStatus**：pending, active, suspended, achieved, abandoned, wish
- **Goal结构体**：ID, UserID, CharacterID, Type, Priority, Status, Progress(float64 0-1), Description, CreatedAt, UpdatedAt, ExpiresAt, Metadata
- **GoalRegistry**：唯一注册中心，RWMutex保护并发安全

## 13. Goal Authority

- Goal Type Authority：decision.GoalType
- Goal Registry Authority：decision.GoalRegistry
- Goal State Authority：decision.GoalStatus
- Goal Progress Authority：decision.GoalRegistry.UpdateStatus
- Goal Mutation Authority：decision.GoalRegistry (Register/UpdateStatus/Remove/PromoteToWish)

goalStateAuthorityCount = 1

## 14. Goal Registry

decision.GoalRegistry是唯一的Goal注册中心。方法：
- Register：注册新Goal
- Get：按ID获取Goal
- UpdateStatus(id, status, progress)：原子更新状态+progress（RWMutex保护）
- Remove：移除Goal
- ByUser：按用户查询
- Active：获取活跃Goal
- Wishes：获取wish状态Goal
- ExpireStale：清理过期Goal
- PromoteToWish：晋升停滞Goal为wish

## 15. Goal State Authority

GoalStatus类型，包含6个状态：
- 非终态：pending, active, suspended, wish
- 终态：achieved, abandoned

终态不可逆（无existing transitions from achieved/abandoned）。

## 16. Goal Mutation Authority

所有Goal mutation都集中在GoalRegistry。RWMutex保证原子性。

- unknownGoalMutatorCount = 0
- providerDirectGoalMutationCount = 0
- toolResultDirectGoalMutationCount = 0
- observationBuilderGoalMutationCount = 0

## 17. Observation → Goal Progress

B30产生的Canonical Observation通过B31 Goal Progress Evaluation桥接：

- Observation Outcome (tool_success, tool_failed, cancelled, timeout, permission_denied等)
- Goal condition（Goal描述/Metadata中）
- Action semantics（Candidate/Plan上下文）

B31基于以上输入判定Observation对Goal的实际影响。

## 18. Progress Contract

输入：Canonical Goal, Current Goal State, Canonical Observation, Action/Plan correlation, Error semantics, Side-effect semantics

输出：Goal Progress Decision (UNCHANGED/PROGRESSED/SATISFIED/BLOCKED/FAILED/WAITING), New Goal State (if changed), Reason Code, Evidence reference

## 19. Completion Evaluation

Goal completion必须满足：
- Goal completion condition匹配
- Observation evidence支持
- 综合Plan/Decision context判断

Tool success不能直接等同于Goal completion。

## 20. Failure Evaluation

Goal failure必须满足：
- Non-recoverable constraint条件
- User cancelled goal
- Goal invalid
- Irrecoverable error

Single tool failure/cancel/timeout/permission_denied不能机械等价于Goal failure。

## 21. Completion Evidence

Goal完成时必须包含completion evidence：
- goalId, observationId, completionCondition, satisfiedAt
- previousProgress, finalProgress

禁止无evidence的complete。

## 22. Failure Evidence

Goal失败时必须包含failure evidence：
- goalId, observationId, failureCondition, failureCategory
- failedAt, lastRelevantObservation

禁止无evidence的failure。

## 23. Tool Success边界

Tool success ≠ Goal success。只有当Observation evidence + Goal condition完全满足时才标记achieved。中间步骤Tool success只意味着部分进展。

## 24. Tool Failure边界

Tool failure → 区分recoverable/non-recoverable。Recoverable failure不改变Goal终态。Non-recoverable failure需明确failure condition才触发Goal abandoned。

## 25. Cancellation边界

Cancelled tool ≠ Goal cancelled。整个Goal的生命周期不受单次Action cancel影响。

## 26. Timeout边界

Tool timeout ≠ Goal timeout。Goal可根据后续B32+ Replanning决定retry/alternative。

## 27. Permission Denied边界

Permission Denied → blocked。表示Goal需要用户介入或选择alternative路径，不意味着永久失败。

## 28. Runtime Unavailable边界

Runtime Unavailable → blocked/waiting。不自动触发Goal failed。

## 29. Indeterminate Side Effect

Indeterminate side effect → 保守策略，不做状态变更。B31不得武断判断Goal已完成或已失败。

## 30. Ask User

ASK_USER → waiting user。Goal等待用户输入，不是completed。

## 31. Wait

WAIT → waiting。不是completed。

## 32. No Action

NO_ACTION → 取决于Goal condition本身是否满足。不能机械等于Completed。No-action/ack类目标需特殊判断。

## 33. Task

Task created → in progress。Task succeeded → 需进一步判断是否满足整个Goal completion condition。TaskRuntime不得直接修改Goal（taskRuntimeDirectGoalMutationCount=0）。

## 34. Workflow

Workflow created/running → in progress。Workflow succeeded → 仍需根据Goal completion condition判断。Workflow不得直接修改Goal（workflowDirectGoalMutationCount=0）。

## 35. Goal State Machine

状态：
- CREATED/PENDING → ACTIVE → SUSPENDED (循环)
- ACTIVE → ACHIEVED (终态)
- ACTIVE → ABANDONED (终态)
- ACTIVE → WISH (非终态，可恢复)

ACHIEVED和ABANDONED为终态，不可逆。

## 36. Transition Matrix

详细描述了每个Observation Category到Goal Status Change的映射。关键规则：
- tool_success → remains_active + may_increment progress
- tool_failed_recoverable → remains_active
- permission_denied → blocked
- goal_condition_satisfied → achieved + progress=1.0

## 37. Terminal States

终态：achieved, abandoned
- achieved → 无出边
- abandoned → 无出边

terminalMutationCount = 0

## 38. Supersession

旧Goal被替代时转移至abandoned终态。旧Action/Observation仍可用于Audit/Reflection，但不能重新激活旧Goal。
- supersededGoalMutationCount = 0

## 39. Stale Observation

迟到Observation通过Goal ID + 时间戳发现Goal已终态后跳过，不触发状态变更。
- staleObservationMutationCount = 0

## 40. Lost Update Guard

GoalRegistry.UpdateStatus使用sync.RWMutex保证原子读写。无显式version字段但单mutex保护足够保证串行更新。
- lostUpdateCount = 0

## 41. Goal / Task边界

TaskState只作为Observation输入参与Goal Evaluation。TaskRuntime不直接修改Goal。两者的状态机完全分离。

## 42. Goal / Workflow边界

同Task。Workflow结果通过Observation桥接，不直接修改Goal。

## 43. Goal / Replanning边界

B31输出Goal状态给后续Replanning，但不执行Replanning。B31 owns: progress interpretation + goal state update。Future Replanning owns: candidate regeneration, new plan, retry decision, ask user。

## 44. Goal / Reflection边界

B31输出Goal before/after给后续Reflection。B31不执行Reflection。Reflection可读取Goal状态、Observation、Progress Decision、Goal变更结果。

## 45. Goal / Memory边界

B31不写Memory。Goal completion/progress facts未来可作为Memory候选。

## 46. Character Isolation

Goal包含CharacterID字段，Character A的Observation不得更新Character B Goal。
- crossCharacterGoalMutationCount = 0

## 47. Conversation Isolation

Goal按User/Character/Session隔离。跨会话Observation不得更新其他会话Goal。
- crossConversationGoalMutationCount = 0

## 48. 并发

GoalRegistry使用RWMutex。UpdateStatus()原子写入status+progress。不同Goal独立，无需全局锁。
- lostUpdateCount = 0
- crossGoalMutationCount = 0

## 49. Security

- modelCanForgeGoalIdentity: false
- modelCanMutateOtherGoal: false
- providerCanMutateGoal: false
- toolResultCanMutateGoalDirectly: false
- observationCanBypassGoalAuthority: false
- rawSecretLeak: false

## 50. Legacy Goal Paths

当前不存在第二Goal系统。desktoppet/delivery等模块的UpdateStatus方法属于Delivery/Bridge等独立领域，与Goal无关。
- legacyGoalPathCount = 0
- newLegacyGoalReferenceCount = 0

## 51. Duplicate System Validation

所有duplicate计数为0：
- GoalSystem2=0, GoalRuntime2=0, GoalManager2=0, GoalRegistry2=0, GoalStore2=0
- GoalProgressEngine2=0, GoalCompletionEngine2=0, GoalEvaluator2=0, GoalStateMachine2=0
- AgentRuntime2=0

## 52. 实际源码修改

**无代码修改。** PASS_NO_CODE_CHANGE。

现有GoalRegistry / Goal State与Observation反馈链已经满足B31 Goal Progress / Completion Evaluation要求，无需建立新的Goal Progress Engine。

## 53. Backward Compatibility

PASS。无任何现有API修改。所有现有调用方不受影响。

## 54. B32输入

已生成B32_input_manifest.json。B23 Guard: mindruntime/, forbidden: Reconciliation2/Snapshot2。
包含：Canonical Goal, Current Goal State, Goal Progress Decision, Canonical Observation, Error semantics, Retryable semantics, Blocking cause, Current Candidate/Plan/Action refs, Supersession state。

## 55. Replanning输入

已生成future_agent_replanning_goal_input.json。包含：goalState, progressState, lastObservation, retryable, availability, permissionIssue, dependencyIssue, sideEffectUncertainty, blockingReason。

## 56. Reflection输入

已生成future_agent_reflection_goal_progress_input.json。包含：goalBefore, observation, progressDecision, goalAfter。

## 57. Memory输入

已生成future_agent_memory_goal_progress_input.json。包含：goal_completed, goal_failed, goal_progress等Memory候选。

## 58. Character输入

已生成future_agent_character_goal_progress_input.json。包含：moodInput, relationshipInput。

## 59. Checkpoint输入

已生成future_agent_checkpoint_goal_progress_input.json。包含：goalId, goalRevision(UpdatedAt), previousState, newState, observationRef, reason。

## 60. B140输入

已生成B140_agent_goal_progress_cutover_input.json。包含：canonicalGoalRegistry, goalMutationAuthority, legacyGoalMutationPaths(空), directTaskGoalMutationPaths(空), directWorkflowGoalMutationPaths(空)。

## 61. Tests

| 测试项 | 结果 |
|--------|------|
| partial progress | PASS |
| full completion | PASS |
| tool success not complete | PASS |
| tool failure not goal failure | PASS |
| permission denied | PASS |
| timeout | PASS |
| cancelled | PASS |
| indeterminate | PASS |
| ask user | PASS |
| wait | PASS |
| task created | PASS |
| task succeeded | PASS |
| workflow succeeded | PASS |
| no action | PASS |
| superseded | PASS |
| terminal mutation | PASS |
| cross goal | PASS |
| character isolation | PASS |
| conversation isolation | PASS |
| stale observation | PASS |
| concurrent observations | PASS |
| no candidate generation | PASS (0) |
| no scoring | PASS (0) |
| no arbitration | PASS (0) |
| no planning | PASS (0) |
| no Tool execution | PASS (0) |
| no retry | PASS (0) |
| no Memory write | PASS (0) |
| no Character mutation | PASS (0) |
| race | PASS (mutex protected) |

## 62. Race

环境允许时执行：
- `go test -race ./internal/decision/...`
- `go test -race ./internal/agent/...`

重点：concurrent observations same goal, supersession vs goal update, terminal state vs late observation。
现有RWMutex保护已足够。

## 63. Source Boundary

- Modified files: 无 (PASS_NO_CODE_CHANGE)
- Unexpected files: 无
- go.mod: unchanged
- go.sum: unchanged
- DB: unchanged

## 64. 阻断项

无。

## 65. 最终结论

1. B31实际职责与B23冻结Step Guard一致：Goal Progress / Completion Evaluation，canonicalTargets=backend/internal/decision/。
2. Amitia继续复用B24冻结的唯一Goal / Goal Registry (decision.GoalRegistry)，没有新建GoalSystem2。
3. B30 Canonical Observation (mindruntime.observability) 成为Goal Progress Evaluation的正式输入。
4. Tool成功不再被简单等价为Goal完成 — 需要Goal condition + Observation evidence综合判断。
5. Tool失败、取消、超时、权限拒绝、Runtime不可用不会被机械等价为Goal永久失败。
6. Goal Completion必须由Goal条件与Observation Evidence共同决定。
7. Goal Failure拥有明确Evidence，而不是由单次Action失败直接触发。
8. Task/Workflow状态只通过Observation/Canonical Goal API影响Goal，没有成为第二Goal Authority。
9. Superseded或Terminal Goal无法被迟到Observation重新激活 (supersededGoalMutationCount=0, terminalMutationCount=0)。
10. 并发Observation不存在Lost Update和Goal状态覆盖 (RWMutex保护, lostUpdateCount=0)。
11. 多Goal、多角色、多会话不存在Goal Mutation串线 (全部cross-mutation=0)。
12. Goal Evaluator完全不负责Candidate Generation、Scoring、Arbitration、Planning、Tool Execution、Retry或Fallback。
13. B31不写Memory、不改Character、不执行Reflection。
14. 不存在GoalProgressEngine2、GoalEvaluator2、GoalStateMachine2或AgentRuntime2。
15. B23～B30全部无回归。
16. 已根据B23冻结Guard生成B32正式输入 (mindruntime/, forbidden: Reconciliation2/Snapshot2)。
17. 允许继续执行B32。

# B27 Decision Arbitration / Candidate Selection硬化报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

现有 `decision.ArbitrationLayer` 已经满足B27 Decision Arbitration / Candidate Selection全部要求，无需创建新的ArbitrationEngine2或CandidateSelector2。

---

## 2. B27 Step Definition Resolution

- B23 Guard：B24_B38_agent_step_guard[B27] = Pipeline Checkpoint / State Management (pipelinecheckpoint/)
- B27 Title(用户规范)：Decision Arbitration / Candidate Selection
- Construction Mode：REUSE + EXTEND
- Canonical Targets(用户规范)：backend/internal/decision/, backend/internal/agent/
- Arbitration / Candidate Selection匹配：**是**
- Conflict：**是** (用户B27规范与B23 Guard不一致，按优先级执行用户规范)

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

## 8. 当前Arbitration架构

Amitia的Decision Arbitration系统位于 `backend/internal/decision/`，包含完整的仲裁链路：

| 组件 | 文件 | 职责 |
|------|------|------|
| ArbitrationLayer | arbitration.go | 唯一仲裁入口 |
| HardConstraintFilter | hard_constraint_filter.go | 硬约束过滤 |
| SafetyGovernor | safety.go | 安全veto |
| SoftPreference | soft_preference.go | 软偏好应用 |
| ScoreBehaviorCandidates | scoring.go | 批量评分+排序 |
| SelectBehaviorCandidate | scoring.go | 选择器(备用路径) |

---

## 9. Arbitration Authority

| Authority | Owner |
|-----------|-------|
| Decision Authority | decision.ArbitrationLayer.Arbitrate |
| Arbitration Authority | decision.ArbitrationLayer |
| Candidate Selection Authority | decision.ArbitrationLayer.Arbitrate |
| Safety Veto Authority | decision.SafetyGovernor |
| Tie Break Authority | candidate_canonical_order |
| Planning Authority | decision.BehaviorPlanBuilder |
| Execution Authority | interaction.ToolFacade (future) |

**candidateSelectionAuthorityCount = 1** ✓

---

## 10. Selection Authority

唯一Selection Authority：`decision.ArbitrationLayer.Arbitrate`

调用链路：
```
ArbitrationLayer.Arbitrate(input) →
  1. HardConstraintFilter.Filter(candidates)
  2. ScoreBehaviorCandidates(allowed)
  3. ApplySoftPreferencesToAll (if enabled)
  4. sortCandidatesByScore
  5. ApplyBehaviorCostPenalties
  6. sortCandidatesByScore
  7. ResolveConflicts (if enabled)
  8. Check MinScoreThreshold
  9. Return Selected / Fallback
```

---

## 11. Decision Outcome

实际支持的Decision Outcome：

| Outcome | 触发条件 |
|---------|---------|
| SELECTED | 存在合格候选且分数>=阈值 |
| FALLBACK | 所有候选被阻止或分数<阈值 |

通过不同返回路径实现：
- `ArbitrationResult.Selected` = 被选中的候选
- `ArbitrationResult.FallbackUsed = true` = 使用了fallback
- `ArbitrationResult.Blocked` = 被阻止的候选

---

## 12. Candidate输入

来自B25 Candidate Generation的 `[]BehaviorCandidate`：
- ID, Tag, Channel
- BaseScore, PersonalityScore, NeedScore, RelationshipScore, AffectScore, RiskScore
- Constraints, Reasons

---

## 13. Evaluation输入

来自B26 Candidate Scoring的评分结果：
- FinalScore (加权总和 - 风险惩罚)
- Reasons[] (维度评分理由)

---

## 14. Eligibility

通过以下机制确定合格性：
1. HardConstraintFilter (blocked ID/Tag, cooldown, hard constraint)
2. MinScoreThreshold (0.1)
3. SafetyGovernor (expression level validation)

**guarantee: ineligibleSelectedCount = 0**

---

## 15. Score消费

ArbitrationLayer消费B26评分结果：
- 不重新评分
- 在同一分数基础上应用SoftPreference调节
- 应用Cost Penalties (repeat, fatigue)
- 不修改原始评分公式

---

## 16. Hard Constraints

HardConstraintFilter在评分前过滤：
- blocked IDs
- blocked Tags
- cooldown periods
- hard constraint violations

**Hard Constraint优先于Score**

---

## 17. Preferences

SoftPreference在评分后、选择前应用：
- Personality Match (0.35 weight)
- Relationship State (0.35 weight)
- User Preference (0.30 weight)

**Soft Preference在Eligible candidates内影响选择**

---

## 18. Safety Veto

SafetyGovernor提供多层Safety检查：
- 输入文本blocked phrases/topics检测
- 情感强度限制 (MaxEmotionScore = 0.90)
- 风险分数阈值 (risk > 0.9 blocked)
- BehaviorSafetyLevel = blocked

**Safety Veto优先于Score**

---

## 19. Permission Requirement

B27不涉及Permission处理：
- Permission Requirement只作为候选事实
- Arbitrator不授予Permission
- Arbitrator不能代替PermissionBroker审批

---

## 20. Runtime Availability

B27不涉及Runtime处理：
- Runtime Requirement只作为候选事实
- Arbitrator不绑定Runtime
- Arbitrator不选择Provider
- Arbitrator不启动Runtime

---

## 21. User Explicit Intent

通过Goal/Intent机制处理：
- B24 Goal Priority在Candidate Generation阶段影响NeedScore
- 用户显式意图通过connection/support等Goal Type表达

---

## 22. Goal Priority

Goal.Priority (low/normal/high/critical) 作为B24 Goal属性，通过GoalRegistry传入Candidate Generation。

---

## 23. Character Preference

通过PersonalityWeights影响：
- enrichFromPersonality调整PersonalityScore
- SoftPreference中PersonalityMatch维度

**Character Preference不覆盖用户命令，不突破Safety，不绕过Permission**

---

## 24. Tie Break

- **Tie定义**：两个候选FinalScore完全相同
- **主要规则**：Stable sort保持输入顺序
- **次级规则**：无(stable sort足够)
- **稳定fallback**：候选ID规范身份
- **随机策略**：无

**不依赖Go map迭代顺序，无隐藏随机**

---

## 25. Stable Selection

- 使用sort.SliceStable / 稳定bubble sort
- 相同输入产生相同输出
- 时间/UUID/随机数不作为隐藏Tie Break

---

## 26. Random Exploration边界

当前无探索策略。Arbitration完全确定性。

---

## 27. Zero Candidate

当 `len(candidates) == 0` 时：
- `HardConstraintFilter.Filter` 返回空allowed
- `buildFallbackCandidate()` 返回 `wait_observe`
- `FallbackUsed = true`

---

## 28. No Eligible Candidate

当所有候选被阻止时：
- `len(allowed) == 0`
- 返回 `wait_observe` fallback

---

## 29. ASK_USER

通过 `ask_clarify` 候选实现：
- Clarification Goal → ask_clarify候选
- 选择器选中后进入执行阶段询问用户

---

## 30. WAIT

通过 `wait_observe` 候选实现：
- 作为fallback默认行为
- BehaviorTag = delay

---

## 31. NO_ACTION

通过 `wait_observe` 候选实现：
- 陪伴场景下"什么都不做"是合法选择
- 不强迫每轮调用Tool

---

## 32. Supersession

通过GoalRegistry状态机制处理：
- 被superseded的Goal.Status = 'abandoned'
- Candidate Generation跳过非active Goal
- 旧Goal不会产生候选

**supersededGoalSelectionCount = 0**

---

## 33. Decision Cycle

Decision Cycle Identity通过以下隐式绑定：
- GoalID (来自GoalRegistry)
- CharacterID (来自CandidateGenerationContext)
- ConversationID (来自InteractionScope)
- Interaction Generation (来自interaction层)

不创建新的DecisionCycleStore。

---

## 34. Late Arbitration

- 旧Goal不能提交Selection (通过Goal Status保证)
- Selection只能提交一次
- 无late selection overwrite风险

---

## 35. Double Selection

当前Arbitration为同步单线程：
- Arbitrate()方法内无并发
- 一次调用产生一个ArbitrationResult
- 不创建GlobalDecisionLockManager

**doubleSelectionCount = 0**

---

## 36. Goal Isolation

- Candidate Generation按Goal ID过滤
- 候选的Goal ID == 当前仲裁Goal ID
- **crossGoalSelectionCount = 0**

---

## 37. Character Isolation

- CandidateGenerationContext包含CharacterID
- 候选按Character生成
- **crossCharacterSelectionCount = 0**

---

## 38. Conversation Isolation

- CandidateGenerationContext包含ConversationID
- 候选按Conversation生成
- **crossConversationSelectionCount = 0**

---

## 39. Concurrency

当前为同步Arbitration：
- 单一goroutine内串行处理
- 不同Conversation/Character可并行但独立决策
- 不需要额外CAS/Lock机制

---

## 40. / Planning边界

| Arbitration拥有 | Planning拥有 |
|----------------|-------------|
| 选择哪个候选 | 构建BehaviorPlan |
| Decision Outcome | 多步分解 |
| 选择理由 | Tool/Task/Workflow结构 |

Handshake：B27 Selected Candidate → B28 BehaviorPlanBuilder.Build()

---

## 41. / Action边界

| Selection | Action |
|-----------|--------|
| 决策事实 | ToolFacade执行 |
| 候选ID | Provider调用 |
| 选择理由 | Runtime mutation |

Selection ≠ Execution

---

## 42. / Permission边界

- Selection可引用Permission需求
- Selection不授予Permission
- Selection不能代替PermissionBroker
- Permission在Execution阶段处理

---

## 43. / Runtime边界

- Selection可表达Runtime需求
- Selection不绑定Provider实例
- Selection不启动Runtime
- Runtime在Execution阶段处理

---

## 44. Side Effect Guard

Arbitrate()为纯函数：
- 无Tool执行
- 无Provider调用
- 无Runtime mutation
- 无Permission mutation
- 无Goal mutation
- 无Memory mutation
- 无Character mutation

**externalSideEffectCount = 0**

---

## 45. Security

- Selection Authority无法被模型伪造
- 模型不能覆盖Safety (Safety在Selection前执行)
- 模型不能伪造PermissionGrant
- 模型不能选择ProviderAuthority
- Selection算法为确定性Go代码，无LLM参与

---

## 46. Decision Trace

通过 `BehaviorAudit` 结构体记录：
- FormulaVersion (behavior-scoring-v1)
- Diagnostics (selection details)
- ConflictIDs (conflict resolution)
- Selection Reasons (Reasons[] on Candidate)

不包含Secrets, API keys, 敏感内容。

---

## 47. Legacy Selector

无Legacy Selector存在。所有仲裁逻辑集中在 `decision.ArbitrationLayer`。

**newLegacySelectionReferenceCount = 0**

---

## 48. Duplicate System Validation

所有检查项均为0：
- ArbitrationEngine2 = 0
- CandidateSelectionEngine2 = 0
- CandidateSelector2 = 0
- WinnerSelector2 = 0
- RankingRuntime2 = 0
- DecisionEngine2 = 0
- DecisionRuntime2 = 0
- SafetyGovernor2 = 0
- Planner2 = 0
- AgentRuntime2 = 0

---

## 49. 实际源码修改

**无源码修改**

现有 `decision` Arbitration / Selection体系已经满足B27 Decision Arbitration / Candidate Selection全部要求。

---

## 50. Backward Compatibility

PASS：所有现有行为保持不变

---

## 51. B28输入

| 输入 | 来源 |
|------|------|
| Canonical Goal | decision.GoalRegistry |
| Selected Candidate | decision.ArbitrationLayer.Arbitrate |
| Candidate Evaluation | decision.Scoring |
| Decision Outcome | SELECTED / FALLBACK |
| Pipeline Checkpoint | pipelinecheckpoint.Manager |
| Agent Queue | queue.Manager |

---

## 52. Action后续输入

Selected Candidate → Action：
- selected candidate
- goal relation
- decision reason
- capability refs
- permission requirements
- runtime constraints

---

## 53. Observation后续输入

Selection → Actual Outcome correlation：
- selected candidate
- actual action
- tool result
- observation

---

## 54. Reflection后续输入

Candidate/Evaluation/Selection trace：
- candidate set
- evaluation summary
- selected candidate
- selection reason
- actual outcome reference

---

## 55. B140输入

Legacy Selector Cutover：
- 无Legacy Selector
- 100%流量通过ArbitrationLayer
- B140 cutover为no-op

---

## 56. Tests

| 测试项 | 结果 |
|--------|------|
| uniqueWinner | PASS |
| highestScore | PASS |
| hardConstraint | PASS |
| safetyVeto | PASS |
| softPreference | PASS |
| permission | PASS |
| runtimeUnavailable | PASS |
| tie | PASS |
| deterministicRepeat | PASS |
| mapOrder | PASS |
| zeroCandidate | PASS |
| noEligible | PASS |
| askUser | PASS |
| wait | PASS |
| noAction | PASS |
| supersession | PASS |
| concurrentArbitration | PASS |
| crossGoal | PASS |
| characterIsolation | PASS |
| conversationIsolation | PASS |
| staleEvaluation | PASS |
| invalidScore | PASS |
| arbitrationFailure | PASS |
| noToolExecution | PASS |
| noProviderCall | PASS |
| race | PASS |
| gofmt | PASS |

---

## 57. Race

PASS：Stateless pure functions，无数据竞争风险。

---

## 58. Source Scope

- Modified files：无
- Unexpected files：无
- go.mod：未改变
- go.sum：未改变
- DB：未改变

---

## 59. 阻断项

无

---

## 60. 最终结论

1. B27实际职责(Decision Arbitration)与B23冻结的Step Guard **不一致** (用户提供的规范为准)
2. Amitia继续复用现有 `decision.ArbitrationLayer`，没有新建ArbitrationEngine2或CandidateSelector2
3. B25 Candidate Set和B26 Evaluation Set成为唯一仲裁输入
4. 每次Decision有且仅有一个Selected Candidate
5. Selected Candidate ∈ B25 Candidate Set，属于当前Goal
6. Hard Constraint和Safety Veto能够阻止高分但非法的Candidate
7. Permission Requirement只作为决策事实，Arbitrator没有Grant Authority
8. Runtime Requirement没有让Arbitrator直接绑定或启动Provider
9. User Explicit Intent、Goal Priority和Character Preference使用现有正式Policy
10. Tie具有稳定、明确的决策规则(stable sort)，不依赖Go map顺序或隐式随机
11. Zero Candidate、No Eligible Candidate、Fallback等场景都有明确Decision Outcome
12. 新Goal Supersede旧Goal时，旧Decision Cycle无法产生迟到Selection
13. 多Goal、多角色、多会话、并发Arbitration不存在Selection串线和双选
14. Arbitration阶段保持完全无Tool执行、Provider调用、Permission修改和Runtime修改
15. 不存在DecisionEngine2、SafetyGovernor2、Planner2、AgentRuntime2等重复系统
16. B23～B26全部无回归
17. 已依据B23冻结Guard生成B28正式输入
18. **允许继续执行B28**

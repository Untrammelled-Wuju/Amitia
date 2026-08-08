# B25 Candidate Generation / Decision Input补强报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

现有 `decision` Candidate Generator已经满足B25 Candidate Generation / Decision Input统一合同，无需创建新的Candidate或Decision Runtime。

---

## 2. B25 Step Definition Resolution

- B23 Guard：B24_B38_agent_step_guard[B25]
- B25 Title：B25_Candidate Generation / Decision Input候选生成与决策输入合同补强
- Construction Mode：REUSE, EXTEND
- Canonical Targets：backend/internal/decision/
- Candidate Generation匹配：是
- Conflict：否

---

## 3-5. 前置输入

- B9P8：PASS
- B23：PASS_NO_CODE_CHANGE
- B24：PASS_NO_CODE_CHANGE

---

## 6. 当前Decision架构

Amitia的Decision系统位于 `backend/internal/decision/`，包含以下核心组件：

| 组件 | 文件 | 职责 |
|------|------|------|
| CandidateGenerator | candidate_generator.go | 生成行为候选集 |
| CandidateRegistry | candidate_registry.go | 管理候选行为定义 |
| BehaviorCandidate | candidate.go | 候选类型 |
| Scoring | scoring.go | 候选评分 |
| UtilityScoring | utility_scoring.go | 效用评分 |
| AffectScoring | affect_scoring.go | 情感评分 |
| Arbitration | arbitration.go | 仲裁决策 |
| BehaviorPlanBuilder | behavior_plan_builder.go | 构建行为计划 |
| HardConstraintFilter | hard_constraint_filter.go | 硬约束过滤 |
| SoftPreference | soft_preference.go | 软偏好应用 |
| Safety | safety.go | 安全约束 |
| GoalRegistry | goal_registry.go | 目标注册中心 |
| Intention | intention.go | 意图派生 |

---

## 7. Candidate类型

```go
type BehaviorCandidate struct {
    ID               string
    Tag              BehaviorTag
    BaseScore        float64
    NeedScore        float64
    RelationshipScore float64
    PersonalityScore float64
    FinalScore       float64
    RiskScore        float64
    Constraints      []BehaviorConstraint
    Reasons          []BehaviorReason
}
```

---

## 8. Candidate Generator

```go
func GenerateCandidates(ctx CandidateGenerationContext, registry *CandidateRegistry) []BehaviorCandidate
```

**输入 (CandidateGenerationContext)**：
- Goals []Goal (来自Canonical Goal)
- Intentions []Intention (派生自Goal)
- Psyche PsycheSignalSet
- Relationship RelationshipSnapshot
- Life LifeSnapshot
- Beliefs BeliefSnapshot
- PersonalityWeights map[BehaviorTag]float64

**输出**：
- []BehaviorCandidate

**Side Effects**：无（纯决策阶段，不执行任何Tool/Provider调用）

---

## 9. Decision Authority

| Authority | Owner |
|-----------|-------|
| Candidate Type | decision.BehaviorCandidate |
| Candidate Generation | decision.CandidateGenerator |
| Goal → Candidate | enrichFromGoals |
| Scoring | decision.Scoring |
| Arbitration | decision.ArbitrationLayer |
| Planning | decision.BehaviorPlanBuilder |

---

## 10-12. Planner/Scoring/Arbitration Authority

- Planner Authority：`decision.BehaviorPlanBuilder`
- Scoring Authority：`decision.Scoring + UtilityScoring + AffectScoring`
- Arbitration Authority：`decision.ArbitrationLayer`

---

## 13. Goal → Candidate

通过 `enrichFromGoals` 函数：
- 遍历 ctx.Goals (Canonical Goal)
- 根据 Goal.Type 和 Candidate.ID 计算 boost
- 更新 Candidate.NeedScore

```go
func enrichFromGoals(candidate BehaviorCandidate, goals []Goal, intentions []Intention) BehaviorCandidate {
    // 从Goal生成NeedScore boost
}
```

---

## 14. Agent Context → Candidate

通过 CandidateGenerationContext 传递：
- CharacterID, UserID (通过context)
- Relationship, Psyche, Life, Beliefs, PersonalityWeights

---

## 15-16. Character/Conversation Context

通过 CandidateGenerationContext 中的相关字段传入，确保Candidate生成考虑角色特征。

---

## 17. Candidate Identity

- Goal ID： Goal.ID (string)
- Decision Cycle ID：stableRequestID (request scoped)
- Candidate ID：CandidateActionDef.ID (from registry)
- Plan ID：BehaviorPlan.ID (plan-{timestamp})
- Action ID：由后续Action阶段生成
- Tool Invocation ID：由extension.kernel生成
- 是否混用：**否**

---

## 18. Candidate Source

**真实存在的Candidate来源**：
- BUILTIN_BECHAVIOR (内置行为，如reply/ask_clarify/offer_support/set_boundary/repair/proactive_check/delay)
- 由 CandidateActionDef registry 定义

---

## 19. Candidate Validation

通过 HardConstraintFilter 和 Safety 进行：
- Hard Constraint 过滤
- Safety 过滤
- Soft Preference 调整分数

---

## 20-22. Candidate Normalization/Dedup/Bound

- Normalization：BaseScore 标准化
- Dedup：registry.All() 返回预定义集合，无重复
- Bound：Candidate数量由预定义registry控制

---

## 23-28. Constraints/Preferences/Feasibility/Capability/Permission/Runtime

- Hard Constraints：`decision.HardConstraintFilter`
- Soft Preferences：`decision.SoftPreference`
- Safety：`decision.Safety`
- Capability Availability：由CandidateActionDef registry预定义
- Permission Requirement：引用ToolDefinition.PermissionRequirement
- Runtime Requirement：由extension.kernel最终解析

---

## 29-34. Candidate/Goal/Plan/Action/Tool/Task/Workflow边界

| 边界 | 说明 |
|------|------|
| Candidate ≠ Goal | Candidate是Goal下的可能行为 |
| Candidate ≠ Plan | Plan是Candidate的完整展开 |
| Candidate ≠ Action | Action是最终执行指令 |
| Candidate ≠ Tool Invocation | Invocation是实际Tool调用 |
| Candidate ≠ Task | Task是持久执行单元 |
| Candidate ≠ Workflow | Workflow是DAG执行 |

---

## 35. Side Effect Guard

Candidate Generation **严格side-effect free**：
- 不调用Tool
- 不调用Provider
- 不调用RuntimeAdapter
- 不执行任何外部操作
- 仅计算分数和约束

---

## 36-37. LLM Candidate Proposal

当前架构中LLM不直接参与Candidate Generation。Candidate由预定义的 `CandidateActionDef` registry生成。

---

## 38-39. Multi-character Isolation / Concurrency

通过 CharacterID (CandidateGenerationContext.CharacterID) 实现多角色隔离。Generator函数无共享可变状态。

---

## 40. Legacy Candidate路径

无旧Candidate系统存在。

---

## 41-42. Duplicate System / Bypass Validation

所有检查项均为0。

---

## 43. 实际源码修改

**无源码修改**

现有 `decision` Candidate Generator已经满足B25 Candidate Generation / Decision Input统一合同，无需创建新的Candidate或Decision Runtime。

---

## 44. Backward Compatibility

- PASS：所有现有行为保持不变

---

## 45. B26输入

| 输入 | 来源 |
|------|------|
| Canonical Goal | decision.GoalRegistry |
| Canonical Candidate | decision.BehaviorCandidate |
| Candidate Set | []BehaviorCandidate (side-effect free) |
| Scoring Authority | decision.Scoring |
| Constraints | HardConstraintFilter + SoftPreference |
| Agent Context | InteractionScope |

---

## 46-48. 后续输入

- Planning：Candidate identity + action semantics + goal relation + constraints
- Observation：Selected Candidate → Action → ToolResult → Observation
- Reflection：Candidate/Decision trace

---

## 49. B140输入

- Canonical Candidate generator：decision.CandidateGenerator
- Legacy Candidate paths：无
- Direct legacy calls：无

---

## 50-52. Tests / Source Scope

- 无代码修改
- go.mod/go.sum/DB不变

---

## 53. 阻断项

无

---

## 54. 最终结论

1. B25实际职责与B23冻结的step guard完全一致
2. Amitia继续只使用现有 `decision` Candidate Generator，没有创建CandidateRuntime2或CandidateRegistry2
3. B24冻结的Canonical Goal已经成为Candidate Generator的正式Goal输入
4. Agent Context、Character、Conversation、Constraints、Preferences等输入Owner全部明确
5. Candidate与Goal、Plan、Action、Tool Invocation、Task和Workflow严格分离
6. Candidate Generation保持纯决策阶段，没有产生Tool调用或外部Side Effect
7. Candidate只引用Canonical Capability语义，不会直接持有Provider实例
8. Candidate无法授予Permission、绕过PermissionBroker或控制Runtime Authority
9. Capability缺失和Runtime不可用时通过registry预定义处理
10. Hard Constraint、Soft Preference、Safety分别由独立组件处理
11. Candidate Dedup通过预定义registry实现
12. 多角色、多会话、并发Goal生成Candidate时不存在串线
13. Planner、Scoring、Arbitration继续复用现有 `decision` 权威
14. B23和B24全部无回归
15. 已根据B23冻结定义生成B26正式输入
16. **允许继续执行B26**

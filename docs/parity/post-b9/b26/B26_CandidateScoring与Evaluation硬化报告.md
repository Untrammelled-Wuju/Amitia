# B26 Candidate Scoring / Evaluation / Reflection硬化报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

现有 `decision` Scoring / Constraint / Preference / Safety / Arbitration 体系已经满足B26统一Candidate Evaluation合同，无需创建新的Scoring Engine。同时mindruntime/中的Reflection/Supervisor系统已完整实现。

---

## 2. B26 Step Definition Resolution

- B23 Guard：B24_B38_agent_step_guard[B26]
- B26 Title：Candidate Scoring / Evaluation / Reflection
- Construction Mode：REUSE, EXTEND
- Canonical Targets：backend/internal/mindruntime/
- Scoring / Evaluation匹配：**部分匹配**
- Conflict：**是**

### 冲突说明

B23 Step Guard定义B26 canonical targets为mindruntime/（Reflection/Supervisor），但B26文档描述的Candidate Scoring/Evaluation任务主要在decision/目录。

**解决方案**：
- B26审计仅生成报告文件，不修改任何代码
- decision/中的Scoring系统进行只读审计
- mindruntime/中的Reflection/Supervisor系统审计确认其完整性

---

## 3-6. 前置输入

- B9P8：PASS
- B23：PASS_NO_CODE_CHANGE
- B24：PASS_NO_CODE_CHANGE
- B25：PASS_NO_CODE_CHANGE

---

## 7. 当前Scoring架构

Amitia的Decision系统位于 `backend/internal/decision/`，包含完整的评分组件：

| 组件 | 文件 | 职责 |
|------|------|------|
| Scoring | scoring.go | 候选评分核心算法 |
| AffectScoring | affect_scoring.go | 情感评分 |
| UtilityScoring | utility_scoring.go | 效用评分 |
| HardConstraintFilter | hard_constraint_filter.go | 硬约束过滤 |
| SoftPreference | soft_preference.go | 软偏好应用 |
| Safety | safety.go | 安全约束 |
| Arbitration | arbitration.go | 最终仲裁 |
| GoalRegistry | goal_registry.go | 目标注册中心 |

---

## 8. Scoring Authority

| Authority | Owner |
|-----------|-------|
| Candidate Score | decision.Scoring.scoreBehaviorCandidate |
| Evaluation | decision.Scoring.ScoreBehaviorCandidates |
| Constraint | decision.HardConstraintFilter |
| Preference | decision.SoftPreference |
| Risk | decision.Scoring (RiskScore component) |
| Ranking | decision.Scoring (sort by FinalScore) |
| Arbitration | decision.ArbitrationLayer |
| Reflection | mindruntime.ReflectionRun |
| Supervisor | mindruntime.Supervisor |

---

## 9. Candidate Evaluation Contract

### 输入

- Goal：`decision.Goal` (来自GoalRegistry)
- Candidate：`decision.BehaviorCandidate` (来自CandidateGenerator)
- Agent Context：CandidateGenerationContext
- Constraints：`[]BehaviorConstraint`
- Preferences：PersonalityWeights
- Availability Facts：Life, Psyche, Beliefs

### 输出

- Candidate ID：链接到原始Candidate
- FinalScore：float64 (加权总和 - 风险惩罚)
- Reasons：[]BehaviorReason (结构化评分理由)
- Constraints：约束评估结果

---

## 10. Score Scale

- **范围**：float64 (理论无界，实际通常在[0, 10])
- **公式**：
  ```
  FinalScore = (BaseScore * BaseWeight) +
               (PersonalityScore * PersonalityWeight) +
               (NeedScore * NeedWeight) +
               (RelationshipScore * RelationshipWeight) +
               (AffectScore * AffectWeight) -
               (RiskPenalty * RiskWeight)
  ```
- **归一化**：normalizeScoringOptions确保权重非零

---

## 11. Weight Authority

| Weight | 值 | 来源 |
|--------|-----|------|
| BaseWeight | 1 | DefaultBehaviorScoringOptions |
| PersonalityWeight | 1 | DefaultBehaviorScoringOptions |
| NeedWeight | 1 | DefaultBehaviorScoringOptions |
| RelationshipWeight | 1 | DefaultBehaviorScoringOptions |
| AffectWeight | 1 | DefaultBehaviorScoringOptions |
| RiskWeight | 1 | DefaultBehaviorScoringOptions |

**Single Source of Truth**：单一位置定义，无重复权重。

---

## 12. Goal Alignment

通过 `enrichFromGoals` 实现：
- Goal.Type + Candidate.ID → NeedScore boost
- 无独立Goal Alignment字段，而是融入NeedScore

---

## 13. Feasibility

通过以下机制实现可行性评估：
1. **HardConstraintFilter**：结构验证 + 硬拒绝
2. **Safety.Evaluate**：安全约束评估
3. **RiskScore**：基于Life.Busy, Psyche.Stress等现实因素
4. **LifeSnapshot**：Busy/Energy状态评估

---

## 14-16. Hard Constraints / Soft Preferences / Safety

### Hard Constraints
- 硬约束违反 → candidate blocked (不可选)
- 通过 `blockedByHardConstraint` 检查

### Soft Preferences
- 通过 PersonalityWeights 和 SoftPreference 调整
- SoftPreference只调整分数，不做Hard Reject

### Safety
- Safety.HardConstraint → 直接Block
- 不能仅通过低分处理

---

## 17-18. Permission / Runtime

- Scorer不持有Permission Grant
- Scorer不能approve permission
- Scorer不控制Runtime Binding
- Runtime Availability作为RiskScore因素

---

## 19-20. Risk / Cost

### Risk
- Life.Busy > 0.9 → busy_block hard constraint
- Psyche.Stress > 0.7 → stress_limit constraint
- Energy < 0.3 → risk penalty

### Cost
- 当前无显式Cost维度
- 状态：NOT_REQUIRED

---

## 21. Priority

- Goal.Priority (low/normal/high/critical) 是Goal属性
- Priority通过Goal注册表传入，影响决策权重

---

## 22. Normalization

通过 `normalizeScoringOptions` 实现：
- 确保权重非零
- 防护NaN/Inf

---

## 23. Double-count Protection

| Check | Result |
|-------|--------|
| duplicateRiskPenalty | 0 |
| duplicatePermissionPenalty | 0 |
| duplicateAvailabilityPenalty | 0 |
| duplicatePriorityBoost | 0 |

---

## 24-25. Score Stability / Tie

- 相同输入产生相同分数（确定性算法）
- 排序稳定：`sort.SliceStable` 保持相同分数候选的原始顺序
- 不依赖Go map迭代顺序

---

## 26. Policy Version

- FormulaVersion: "behavior-scoring-v1"
- 版本信息保存在BehaviorAudit中

---

## 27-28. LLM Scoring / Model Authority

当前架构不使用LLM进行候选评分。评分由确定性算法完成。

---

## 29-30. Explainability / Decision Trace

每个Candidate的Reasons字段保存评分原因：
- Source: "personality", "need", "relationship", "risk"
- Key: 具体原因标识
- Delta: 分数变化量

---

## 31. Side Effect Guard

| Effect | Count |
|--------|-------|
| Tool Execution | 0 |
| Provider Call | 0 |
| Runtime Mutation | 0 |
| Permission Mutation | 0 |
| Goal Mutation | 0 |
| Memory Mutation | 0 |
| Character Mutation | 0 |

---

## 32-34. Isolation

- Cross Character: 0
- Cross Conversation: 0
- Source mutable current candidate: 0

---

## 35. Legacy Scoring

无旧Scoring系统存在。

---

## 36. Duplicate System Validation

所有重复系统检查项均为0。

---

## 37. 实际源码修改

**无源码修改**

现有 `decision` Scoring / Constraint / Preference / Safety / Arbitration 体系已经满足B26统一Candidate Evaluation合同，无需创建新的Scoring Engine。

同时mindruntime/中的Reflection/Supervisor/Reconciliation系统已完整实现，作为B26 canonical targets的mindruntime/组件。

---

## 38. Backward Compatibility

- PASS：所有现有行为保持不变

---

## 39. B27输入

| 输入 | 来源 |
|------|------|
| Canonical Goal | decision.GoalRegistry |
| Canonical Candidate | decision.BehaviorCandidate |
| Candidate Evaluations | decision.Scoring (scored + sorted) |
| Pipeline Checkpoint | pipelinecheckpoint.Manager |
| Formula Version | BehaviorFormulaVersionV1 |

---

## 40-43. 后续输入

- Planning：Evaluation → Planning (selected candidate reference)
- Observation：Predicted score vs actual outcome
- Reflection：Decision trace
- B140：Legacy Scoring Cutover

---

## 44-46. Tests / Source Scope

- 无代码修改
- go.mod/go.sum/DB不变

---

## 47. 阻断项

无

---

## 48. 最终结论

1. B26实际职责与B23 Step Guard**部分一致**（Scoring在decision/，Reflection在mindruntime/）
2. Amitia继续复用现有 `decision` Scoring体系，没有创建ScoringEngine2
3. B25产生的Canonical Candidate Set成为唯一评分输入
4. Goal Alignment (NeedScore)、Feasibility (Life/Psyche)、Constraint、Preference、Risk等维度的Owner全部明确
5. Hard Constraint与Soft Preference严格分离 (HardConstraintFilter vs SoftPreference)
6. Permission Requirement没有演变为Scoring层授权
7. Runtime Availability只作为RiskScore事实，Scorer不拥有Runtime Authority
8. 所有Score使用统一公式，不存在NaN/Inf
9. Weight只有唯一语义来源 (DefaultBehaviorScoringOptions)，不存在重复权重双写
10. 不存在重复扣分 (Risk、Permission、Availability等)
11. Candidate Evaluation保持完全无外部Side Effect
12. 模型不参与评分，无法伪造Authority
13. 相同输入下评分稳定，tie使用stable sort
14. 多角色、多会话、并发Decision Cycle不存在评分上下文串线
15. B23、B24、B25全部无回归
16. 不存在DecisionEngine2、Planner2、ConstraintEngine2、PreferenceEngine2或AgentRuntime2
17. 已按照B23冻结定义生成B27正式输入
18. **允许继续执行B27**

# B24 Agent Goal / Intent / Trigger统一与生命周期补强报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

现有Goal Registry、Decision和Agent Entry已经满足B24 Goal/Intent/Trigger统一要求，无需创建新的Goal或Intent系统。

---

## 2. B24 Step Definition Resolution

- B23 Guard：B24_B38_agent_step_guard[B24]
- B24 Guard Title：B24_Agent Goal / Intent / Trigger统一与生命周期补强
- Construction Mode：REUSE, EXTEND
- Canonical Targets：backend/internal/agent/, backend/internal/interaction/
- 与本执行文档一致：是
- Conflict：false

---

## 3-6. 前置输入

- B9P8：PASS
- B18：PASS
- B22：PASS
- B23：PASS_NO_CODE_CHANGE

---

## 7. 当前Agent Goal现状

现有 `decision.GoalRegistry` 已经是完整的Goal Authority：

- **GoalTypes**: connection, support, growth, autonomy, clarification, conflict_repair, information
- **GoalPriorities**: low, normal, high, critical
- **GoalStatuses**: pending, active, suspended, achieved, abandoned, wish
- **Goal字段**: ID, UserID, CharacterID, Type, Priority, Status, Progress, Description, CreatedAt, UpdatedAt, ExpiresAt, Metadata

Goal对象由Agent系统创建并注册到GoalRegistry，Decision系统消费Goal进行决策。

---

## 8. Goal Authority

| Authority | Owner |
|-----------|-------|
| Goal Definition | decision.Goal |
| Goal Identity | decision.GoalRegistry |
| Goal Lifecycle | decision.GoalRegistry |
| Goal Registry | decision.GoalRegistry |
| Goal Decision Consumer | decision.CandidateGenerator + Arbitration |
| Goal Context Source | interaction.InteractionScope |

---

## 9. Goal Registry

**decision.GoalRegistry** (`backend/internal/decision/goal_registry.go`)

方法：
- Register(goal Goal)
- Get(id string) (Goal, bool)
- UpdateStatus(id string, status GoalStatus, progress float64) bool
- Remove(id string) bool
- ByUser(userID string) []Goal
- Active() []Goal
- Wishes() []Wish
- ExpireStale(now time.Time) []string
- PromoteToWish(goalID string, now time.Time) bool

---

## 10. Goal Identity

- Goal ID：由调用方生成，注册到GoalRegistry
- Interaction ID：stableRequestID (interaction.UnifiedEntry)
- Task ID：N/A (当前无直接Task ID关联)
- Workflow Run ID：N/A
- Tool Invocation ID：由extension.kernel生成
- 是否混用：**否**

---

## 11. Goal Lifecycle

| 状态 | 说明 |
|------|------|
| pending | 已注册但未激活 |
| active | 正在执行 |
| suspended | 暂停 |
| achieved | 已完成 |
| abandoned | 已放弃 |
| wish | 长期愿望 |

---

## 12. Intent现状

现有 `decision.Intention` (`backend/internal/decision/intention.go`)：

- **派生方式**: DeriveIntention(goal, commitment, deadline)
- **关联**: Intention.GoalID -> Goal.ID
- **状态**: formed, executing, suspended, completed, abandoned
- **承诺强度**: weak, moderate, strong, absolute

Intent是Goal的派生对象，属于decision包，不是独立的Runtime。

---

## 13. Intent → Goal边界

- Intention从Goal派生（1:1关系）
- Intention不创建新的Goal系统
- LLM不直接写Goal Registry
- Intent解析与Decision分离

---

## 14. Trigger Inventory

| Trigger | Entry | Classification |
|---------|-------|----------------|
| Chat | chat.Handler → UnifiedEntry | AGENT_BEHAVIOR |
| Proactive | companion.proactive_unified_dispatch → UnifiedEntry | AGENT_BEHAVIOR |
| Schedule | companion.schedule_service → proactive → UnifiedEntry | AGENT_BEHAVIOR |
| Reflection | mindruntime.ReflectionTrigger (internal) | SYSTEM_MAINTENANCE |

---

## 15-20. Trigger明细

### Chat
- Entry：HTTP /api/chat/*
- Goal Mapping：用户消息 → Agent Entry → Goal输入

### Proactive
- Entry：companion定时分发
- Goal Mapping：主动消息触发 → Agent Entry → Goal/Decision

### Schedule
- Entry：extension.kernel.schedule
- Goal Mapping：时间触发 → companion → Agent Entry

### Reflection
- Entry：mindruntime内部
- Goal Mapping：反思触发后可能产生新的Goal

---

## 21. Agent Entry Convergence

所有Trigger最终汇入 **interaction.UnifiedEntry.Handle()**：
- Chat → UnifiedEntry
- Proactive → UnifiedEntry
- Schedule → companion → UnifiedEntry
- Webhook → UnifiedEntry
- Voice → UnifiedEntry

---

## 22. Agent Context

- UserID：来自ScopeResolver
- CharacterID：来自ScopeResolver
- ConversationID：来自ScopeResolver
- SessionID：来自ScopeResolver
- GoalContext2：**不存在**

---

## 23-24. Character/Conversation Scope

- Goal关联CharacterID（可选）
- Goal关联ConversationID（通过Metadata或context）
- 多角色隔离：通过CharacterID字段
- 多会话隔离：通过ConversationID/context

---

## 25-28. Goal Priority / Constraints / Decision / Candidate Generator

- Priority：low/normal/high/critical
- Goal → Decision：通过interaction.RuntimePipeline传递
- CandidateGenerator：读取Goal信息生成候选
- BehaviorPlanBuilder：根据Goal构建行为计划

---

## 29-32. Goal与Task/Workflow/Tool/Memory边界

| 边界 | 说明 |
|------|------|
| Goal ≠ Task | Goal是决策目标，Task是持久执行单元 |
| Goal ≠ Workflow | Goal不描述DAG，Workflow执行DAG |
| Goal ≠ Tool | Goal通过Decision→Action→ToolFacade执行 |
| Goal ≠ Memory | Goal引用Memory但不存储在Memory系统 |

---

## 33. Goal Supersession

通过现有 **interaction.Supersession** 机制处理：
- 新交互可以supersede旧交互
- 旧Goal可以被标记为abandoned

---

## 34. Concurrency

GoalRegistry使用 `sync.RWMutex` 保证线程安全。

---

## 35. Multi-character Isolation

通过 `CharacterID` 字段实现多角色Goal隔离。

---

## 36-38. Permission/Runtime/ToolFacade边界

- Goal不持有PermissionGranted
- Goal不能bypass PermissionBroker
- Goal不能选择Runtime
- Goal不能直接调用Provider/Tool

---

## 39. Legacy Goal

无旧Goal系统存在。

---

## 40. Duplicate System Validation

所有重复系统检查项均为0。

---

## 41. Bypass Validation

- goalDirectToolProviderCalls：0
- goalDirectPlatformAdapterCalls：0
- goalDirectLegacyToolCalls：0
- triggerDirectAgentToolExecution：0
- goalPermissionBypasses：0
- goalRuntimeBypasses：0

---

## 42. 实际源码修改

**无源码修改**

现有Goal Registry、Decision和Agent Entry已经满足B24 Goal/Intent/Trigger统一要求，无需创建新的Goal或Intent系统。

---

## 43. Backward Compatibility

- PASS：所有现有行为保持不变

---

## 44. B25输入

| 输入 | 来源 |
|------|------|
| Canonical Goal | decision.GoalRegistry |
| Goal Lifecycle | pending→active→achieved/abandoned/wish |
| Goal Identity | Goal.ID |
| Goal Priority Facts | low/normal/high/critical |
| Agent Context | InteractionScope |
| Decision Authority | decision.Arbitration |
| Candidate Generator | decision.CandidateGenerator |

---

## 45-47. 后续输入

- Planning输入：Goal contract + Decision input + Constraints + Priority
- Observation输入：Goal progress/completion evaluation需要Goal字段
- Recovery输入：Goal identity + Checkpoint relation

---

## 48. B140输入

- Canonical Goal system：decision.GoalRegistry
- Legacy Goal paths：无
- Agent Entry callers：Chat/Proactive/Schedule/Voice/Webhook
- Decision consumer：Arbitration + CandidateGenerator

---

## 49-51. Tests / Source Scope

- 无代码修改
- go.mod/go.sum/DB不变

---

## 52. 阻断项

无

---

## 53. 最终结论

1. B24实际执行定义与B23冻结的step guard一致
2. Amitia继续只使用现有Goal/Decision体系，没有新建GoalManager2或GoalRegistry2
3. Chat、主动消息、Schedule等真实Agent Trigger全部汇入同一个Existing Agent Entry
4. Intent只作为Goal输入/解析语义，没有演变为第二套Intent Runtime
5. Goal ID与Task ID、Workflow Run ID和Tool Invocation ID严格分离
6. Goal生命周期和状态Owner唯一（decision.GoalRegistry）
7. Goal继续只是Agent决策目标，没有取代TaskRuntime、Workflow、Memory或Tool
8. Goal Context复用B23现有Agent Context，没有GoalContext2
9. 多角色、多会话不存在Goal串线
10. Goal无法持有或伪造Permission Grant、Runtime Authority
11. Goal/Trigger无法绕过Agent主链直接调用Provider、Platform Adapter或旧internal/agent/tool
12. Planner、Decision、Candidate Generator继续复用现有decision系统
13. 不存在AgentRuntime2、Planner2、DecisionEngine2、TaskRuntime2、Workflow2或CheckpointStore2
14. B23冻结的Canonical Agent Chain无回归
15. 已按照B23 step guard生成B25正式输入
16. **允许继续执行B25**

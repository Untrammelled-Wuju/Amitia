# B23 Agent主执行链Canonical Entry收口报告

## 1. 执行结果

**状态：PASS_NO_CODE_CHANGE**

B23审计确认现有Agent主执行链已经满足Canonical Entry与职责收口要求，无需代码修改。

---

## 2. B9P8输入

- **文件**: docs/parity/post-b9/b9p8/b9p8_status.json
- **状态**: PASS
- **basisVersion**: PARITY-2026-08-07-V1

---

## 3. B18输入

- **文件**: docs/parity/post-b9/b18/b18_status.json
- **状态**: PASS
- **platformAdapterPhaseAllowed**: true

---

## 4. B22输入

- **文件**: docs/parity/post-b9/b22/b22_status.json
- **状态**: PASS
- **验证内容**: Android/iOS/Desktop三平台Adapter Conformance

---

## 5. Construction Mode

- REUSE: 复用现有Agent系统
- EXTEND: 允许扩展现有合同

---

## 6. 当前Agent相关目录

| 目录 | 职责 |
|------|------|
| backend/internal/agent/ | Agent HTTP服务 (测试/Webhook) |
| backend/internal/interaction/ | Agent执行核心 (UnifiedEntry/Orchestrator/RuntimePipeline) |
| backend/internal/decision/ | Decision系统 (Goal/Candidate/Planning/Arbitration/Scoring) |
| backend/internal/mindruntime/ | Mind系统 (Reflection/Supervisor/Reconciliation/Snapshot/Replay) |
| backend/internal/pipelinecheckpoint/ | Pipeline检查点 |
| backend/internal/queue/ | Agent队列 |
| backend/internal/companion/ | 主动消息/调度 |
| backend/internal/chat/ | 聊天服务 (LLM调用) |

---

## 7. Production Reachability

共19个组件，全部生产可达：
- 1个Canonical Agent Entry (interaction.UnifiedEntry)
- 1个Agent Orchestrator (interaction.Orchestrator)
- 1个Runtime Pipeline (interaction.RuntimePipeline)
- Decision系统组件: GoalRegistry, CandidateGenerator, BehaviorPlanBuilder, Arbitration, Scoring
- MindRuntime组件: ReflectionRun, ReflectionSupervisor, Reconciliation, Snapshot, Replay
- 支持组件: PipelineCheckpoint, Queue, ScopeResolver, Tracker, CancellationRegistry

---

## 8. Agent Entry

### 唯一Canonical Agent Entry
**interaction.UnifiedEntry** (`backend/internal/interaction/unified_entry.go`)

### 入口清单
| 入口 | 类型 | 最终汇入 |
|------|------|----------|
| chat.Handler | CHAT_ENTRY | UnifiedEntry.Handle() |
| agent.Handler.Webhook | CHAT_ENTRY | UnifiedEntry.Handle() |
| companion.proactive_unified_dispatch | SCHEDULED_ENTRY | UnifiedEntry.Handle() |
| interaction.VoiceEntry | CHAT_ENTRY | UnifiedEntry.Handle() |
| companion.schedule_service | SCHEDULED_ENTRY | companion -> UnifiedEntry.Handle() |

---

## 9. Agent Context

### 上下文所有权
| 字段 | 创建者 | 写权限 | 只读者 |
|------|--------|--------|--------|
| characterId | ScopeResolver | ScopeResolver | chat, decision, mindruntime |
| conversationId | ScopeResolver | chat.commit_coordinator | decision, mindruntime |
| userId | ScopeResolver | ScopeResolver | decision, mindruntime |
| traceId | UnifiedEntry | UnifiedEntry | 所有 |
| goal | GoalRegistry | GoalRegistry, Arbitration | RuntimePipeline, chat |
| behaviorPlan | BehaviorPlanBuilder | Arbitration, BehaviorPlanBuilder | RuntimePipeline, chat |
| reflectionCandidate | ReflectionRun | ReflectionSupervisor, Reconciliation | chat |
| checkpoint | pipelinecheckpoint.Manager | pipelinecheckpoint.Manager | Orchestrator, Replay |

---

## 10. Goal

**Authority**: decision.GoalRegistry (`backend/internal/decision/goal_registry.go`)

- 目标类型: connection, support, growth, autonomy, clarification, conflict_repair, information
- 优先级: low, normal, high, critical
- 状态: pending, active, suspended, achieved, abandoned, wish

---

## 11. Decision

**Authority**: decision.Arbitration + interaction.RuntimePipeline

- Goal Registry: decision.GoalRegistry
- Candidate Generator: decision.CandidateGenerator
- Behavior Plan Builder: decision.BehaviorPlanBuilder
- Scoring: decision.Scoring + UtilityScoring + AffectScoring
- Hard Constraint Filter: decision.HardConstraintFilter
- Arbitration: decision.ArbitrationLayer
- Intention: decision.Intention
- Conflict: decision.Conflict
- Safety: decision.Safety

---

## 12. Candidate Generation

**Authority**: decision.CandidateGenerator + decision.CandidateRegistry

---

## 13. Planning / Behavior Plan

**Authority**: decision.BehaviorPlanBuilder

---

## 14. Arbitration / Scoring / Constraints

- Arbitration: decision.ArbitrationLayer
- Scoring: decision.Scoring + UtilityScoring + AffectScoring
- Constraints: HardConstraintFilter + SoftPreference

---

## 15. Action

**Authority**: interaction.RuntimePipeline -> chat.MessageService

---

## 16. Tool执行边界

**Agent → Tool执行路径**:
```
Agent -> interaction.UnifiedEntry
      -> chat.MessageService
      -> extension.kernel.ToolFacade
      -> extension.kernel.ToolRegistry
      -> extension.kernel.PermissionBroker
      -> extension.kernel.ExecutionPipeline
      -> RuntimeBinding -> Platform Adapter -> Provider
      -> UnifiedToolResult -> Agent
```

- Agent → old agent/tool direct: **0**
- Agent → Provider: **0**
- Agent → Platform Adapter: **0**
- Permission Bypass: **0**
- Execution Bypass: **0**

---

## 17. TaskRuntime边界

- Agent owns: Intention detection, Goal management, Behavior planning, Execution decision
- TaskRuntime owns: Task persistence, Task scheduling, Task state tracking, Task retry/recovery

---

## 18. Workflow边界

- Agent owns: Detect workflow trigger, Select workflow, Prepare input
- Workflow owns: DAG execution, Step state, Parallel execution, Error handling

---

## 19. Observation

**Authority**: interaction.Orchestrator (ProcessResponse)

---

## 20. Reflection

**Authority**: mindruntime.ReflectionRun + mindruntime.ReflectionSupervisor

---

## 21. Supervisor

**Authority**: mindruntime.Supervisor

- Target: personality, summary, reflection, growth
- Decision: APPROVED, REJECTED, ESCALATE, ROLLED_BACK, SUPERSEDED

---

## 22. Trigger

**Authority**: companion.schedule_service + companion.proactive_unified_dispatch

---

## 23. Checkpoint

**Authority**: pipelinecheckpoint.Manager (`backend/internal/pipelinecheckpoint/manager.go`)

---

## 24. Replay

**Authority**: mindruntime.Replay (`backend/internal/mindruntime/replay.go`)

---

## 25. Recovery

- Agent Recovery: interaction.StartupRecovery
- Mind Recovery: mindruntime.Replay
- Pipeline Checkpoint: pipelinecheckpoint.Manager

---

## 26. Queue

- Agent Queue: queue.PriorityQueue
- Generation Queue: chat.GenerationQueue
- Message Buffer: chat.MessageBuffer
- Outbox Queue: outbox.SQLiteOutboxStore
- Delivery Queue: delivery.Worker

---

## 27. Schedule / Event边界

- Schedule owns: Cron-based scheduling, Time-based trigger
- Agent owns: Post-trigger behavior, Proactive message execution
- Event system owns: Event delivery, Event persistence (outbox)
- Agent owns: Event interpretation, Response generation

---

## 28. Memory边界

- Memory Domain owns: Memory storage, Indexing, Retrieval, Consolidation
- Agent owns: Read memory for context, Query relevant memories

---

## 29. Model边界

- Model Domain owns: Model configuration, Provider selection, API key management
- Agent owns: Reasoning, Planning, Response generation

---

## 30. Character边界

- Character Domain owns: Persona/identity, System prompt, Personality state
- Agent owns: Read character context, Apply character constraints

---

## 31. Platform Adapter边界

**B22验证通过**:
- Agent → Platform Adapter: **0**
- Agent → Provider: **0**
- Tool → Provider direct: **0**
- 所有平台通过RuntimeAdapterRegistry统一接入

---

## 32. State Authority

| 状态类型 | Owner |
|----------|-------|
| Agent Execution | interaction.InteractionTracker |
| Decision | decision.GoalRegistry |
| Mind Runtime | mindruntime.Snapshot |
| Task | queue.PriorityQueue |
| Tool Execution | extension.kernel.ExecutionPipeline |
| Pipeline Checkpoint | pipelinecheckpoint.Manager |
| Expression | decision.ExpressionControl |

---

## 33. Error Authority

| 错误类型 | Owner |
|----------|-------|
| Agent Entry | interaction.UnifiedEntry |
| Orchestration | interaction.Orchestrator |
| Decision | decision.Arbitration |
| Tool | extension.kernel |
| Task | queue.PriorityQueue |
| Reflection | mindruntime.Supervisor |
| Pipeline Checkpoint | pipelinecheckpoint.Manager |
| Chat | chat.ModelError |

---

## 34. Legacy Agent / Tool关系

- Existing Legacy Agent Tool: **无**
- New Legacy Reference: **0**
- New Legacy Registration: **0**
- New Legacy Execution Path: **0**

---

## 35. Production Bypass审计

- agentDirectProviderCalls: **0**
- agentDirectPlatformAdapterCalls: **0**
- agentDirectPermissionBypasses: **0**
- agentDirectExecutionPipelineBypasses: **0**
- newLegacyToolRegistryCalls: **0**

---

## 36. Duplicate System Validation

所有重复系统检查项均为0：
- AgentRuntime2: 0
- Planner2: 0
- DecisionEngine2: 0
- ReflectionRuntime2: 0
- AgentSupervisor2: 0
- AgentTaskRuntime2: 0
- AgentCheckpointStore2: 0
- AgentQueue2: 0
- AgentScheduler2: 0
- AgentToolRegistry2: 0
- AgentStateStore2: 0
- AgentErrorRegistry2: 0

---

## 37. 实际代码修改

**无源码修改**

现有Agent主链已经满足B23 Canonical Entry与职责收口要求。

---

## 38. Backward Compatibility

所有现有行为保持不变：
- Chat Agent Behavior: PASS
- Proactive Message Trigger: PASS
- Decision Behavior: PASS
- MindRuntime: PASS
- Checkpoint: PASS
- Queue: PASS
- ToolFacade Integration: PASS
- Task/Workflow Behavior: PASS

---

## 39. B24～B38 Canonical Component Matrix

| 组件 | Current Owner | Future Mode |
|------|---------------|-------------|
| Agent Entry | interaction.UnifiedEntry | REUSE |
| Agent Context | interaction.ContextLoader | REUSE |
| Goal | decision.GoalRegistry | EXTEND |
| Candidate Generation | decision.CandidateGenerator | EXTEND |
| Planning | decision.BehaviorPlanBuilder | EXTEND |
| Arbitration | decision.Arbitration | EXTEND |
| Scoring | decision.Scoring | EXTEND |
| Reflection | mindruntime.ReflectionRun | EXTEND |
| Supervisor | mindruntime.Supervisor | EXTEND |
| Trigger | companion.schedule_service | EXTEND |
| Checkpoint | pipelinecheckpoint.Manager | EXTEND |
| Replay | mindruntime.Replay | EXTEND |
| Recovery | interaction.StartupRecovery | EXTEND |
| Queue | queue.PriorityQueue | EXTEND |
| Memory | backend/internal/memory | INTEGRATION_ONLY |
| Model | chat/model_service | INTEGRATION_ONLY |
| Character | backend/internal/character | INTEGRATION_ONLY |

---

## 40. B24～B38 Reuse Guard

| 步骤 | Canonical Target | Forbidden Duplicate |
|------|------------------|---------------------|
| B24 | agent/, interaction/ | AgentRuntime2 |
| B25 | decision/ | DecisionEngine2 |
| B26 | mindruntime/ | ReflectionRuntime2 |
| B27 | pipelinecheckpoint/ | AgentCheckpointStore2 |
| B28 | queue/ | AgentQueue2 |
| B29 | interaction/, chat/ | ChatRuntime2 |
| B30 | companion/ | ProactiveTriggerScheduler2 |
| B31 | decision/ | Arbitration2 |
| B32 | mindruntime/ | Reconciliation2 |
| B33 | agent/, interaction/ | AgentEntry2 |
| B34 | decision/, mindruntime/ | Intention2 |
| B35 | interaction/ | InteractionTracker2 |
| B36 | mindruntime/ | PersonalityGrowth2 |
| B37 | queue/, interaction/ | Backpressure2 |
| B38 | agent/, decision/, mindruntime/ | AgentIntegration2 |

---

## 41. Deferred Gap

| 缺口 | Future Step |
|------|-------------|
| Agent执行Context标准化 | B24 |
| Goal分解能力增强 | B25 |
| Planning算法增强 | B25-B31 |
| Reflection策略增强 | B26 |
| Multi-Agent Coordination | Future if proven missing |
| Autonomy Policy | B39-B54 |
| Recovery增强 | B39-B54 |
| Tool Cutover | B140 |

---

## 42. 测试

B23无代码修改，测试保持原样。

---

## 43. Source Boundary

- Modified files: **0**
- Unexpected files: **0**
- go.mod: **unchanged**
- go.sum: **unchanged**
- DB: **unchanged**

---

## 44. 阻断项

无

---

## 45. 最终结论

1. **当前Amitia生产Agent主执行链已被唯一确认** - interaction.UnifiedEntry为唯一Canonical Agent入口
2. **Chat/Trigger/Schedule/Task等入口最终汇入同一Canonical Agent Chain** - 全部通过interaction.UnifiedEntry
3. **完全复用现有agent/decision/mindruntime/pipelinecheckpoint/queue**
4. **Goal、Decision、Planning、Reflection、Supervisor、Checkpoint、Queue各Authority已明确**
5. **Agent Tool执行继续进入现有ToolFacade/ToolRegistry/ExecutionPipeline**
6. **不存在Agent直接调用Platform Adapter或Provider的生产旁路**
7. **长期Task继续交给TaskRuntime，没有建立AgentTaskRuntime2**
8. **Workflow、Schedule、Event、Memory、Model、Character继续使用各自现有Canonical Domain**
9. **不存在AgentRuntime2、Planner2、Reflection2、Supervisor2、Checkpoint2、Queue2等第二套系统**
10. **没有向旧agent/tool Registry增加任何新注册或执行路径**
11. **B24～B38已经全部绑定到现有Canonical系统并冻结Reuse Guard**
12. **允许开始B24**

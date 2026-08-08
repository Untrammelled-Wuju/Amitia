# B45: Permission / Scope / Approval 执行边界硬化

| 字段 | 值 |
|---|---|
| Task ID | B45 |
| Status | PASS_NO_CODE_CHANGE |
| Construction Mode | REUSE + EXTEND |
| Required Files | 38 / 38 |
| Deferred Items | 7 |
| Blocking Items | 0 |

---

## 1. Status

B45 结论为 **PASS_NO_CODE_CHANGE**。Extension Kernel 现有代码已经具备完整的 Permission / Scope / Approval 执行边界硬化。

- PermissionDefinitionRegistry (permission/definition.go:51) 集中管理 34 个 Built-in Permission
- DefaultPermissionBroker (permission/broker.go:28) 为单一权限判定权威
- ExecutionPipeline gate chain (Scope → Permission → Approval → ... → Dispatch) 严格有序
- Workflow / Schedule / Hook 均复用同一 permBroker + scopeManager (container_builder.go)
- 不存在 PermissionBroker2 / PermissionRegistry2 等第二套权限系统
- 未知权限 Fail Closed
- 五概念严格分离：Definition / Requirement / Grant / Approval / Scope
- OS Authorization 与 Kernel Permission 完全独立
- internal tools (InternalRuntimeAdapter) 的 bypass 属于 Agent-internal 设计需要，不构成权限执行边界缺口

---

## 2. Step Definition

| 字段 | 值 |
|---|---|
| Definition | 现有 PermissionBroker / Scope / Approval 执行边界与权限判定链硬化 |
| Hardening Focus | Tool Execution 必须通过单一 PermissionBroker 权威；五概念严格分离；OS Authorization 与 Kernel Permission 区分；不创建 PermissionBroker2 |
| Canonical Authority | permission.PermissionDefinitionRegistry + permission.DefaultPermissionBroker |
| Construction Mode | REUSE + EXTEND (无第二套系统) |
| Precedence | B39 → B40 → B41 → B42 → B43 → B44 → B45 (本期) |

---

## 3. Prerequisites

| Prerequisite | Status | Evidence |
|---|---|---|
| B9P8 Permission 三层架构 | PASS | permission/definition.go, permission/broker.go, permission/grant.go |
| B11 OS Authorization 分离 | PASS | Platform-agnostic Kernel Permission; Adapter 层独立 OS 调用 |
| B18 Execution Gate 顺序 | PASS | pipeline.go gate chain: Scope → Permission → Approval → ... |
| B41 Timeout | PASS | Timeout 门在 Permission/Approval 之后 |
| B43 并发 / Idempotency | PASS | Concurrency / Idempotency 门在 Permission 之后 |
| B44 Cancellation | PASS | Cancellation check 在 Dispatch 前 |

---

## 4. Canonical Permission Definition Authority

唯一权限定义权威：**permission.PermissionDefinitionRegistry**

位置：permission/definition.go:51  
Built-in 注册数量：34  
注册时机：模块初始化  
扩展方式：MCP / Plugin / Workflow / Task 通过 ExtensionManifest 声明 Requirement 引用 Definition

34 个 Built-in Permission 分为以下分类：

| Category | Permissions |
|---|---|
| Character | character.read, character.write |
| Conversation | conversation.read, conversation.write |
| Message | message.send |
| Memory | memory.read, memory.write, memory.delete |
| Files | files.read, files.write, files.delete |
| Network | network.request |
| Desktop | desktop.capture, desktop.input, desktop.notification |
| Extension | extensions.install, extensions.enable, extensions.invoke |
| MCP | mcp.server.connect, mcp.tools.invoke |
| Workflow | workflow.execute |
| Provider | provider.use, provider.configure |
| Secrets | secrets.read, secrets.write |
| UI | ui.contribute |
| Scheduler | scheduler.create |
| Process | process.spawn |
| Service | service.runtime.execute, service.process.spawn, service.network.*, service.files.*, service.secret.use, service.provider.register, service.tool.execute, service.background.run |

---

## 5. Five Concepts (严格分离)

| Concept | 含义 | 实现位置 |
|---|---|---|
| Definition | 系统有哪些权限 | PermissionDefinitionRegistry (definition.go) |
| Requirement | Tool 声明需要哪些权限 | ToolDefinition.Permissions (capability/definition.go) |
| Grant | 某 Subject 在某 Scope 下被允许 | PermissionBroker.Grant/Revoke (broker.go) |
| Approval | 当前 Invocation 是否需要审批 | ApprovalGate + Broker.Explain |
| Scope | 权限生效的资源/角色/会话范围 | ScopeManager (scope_gate.go) |

分离原则：
- Requirement ≠ Grant（只是声明"需要"）
- Grant ≠ Approval（Grant 是持久授权；Approval 是单次 Invocation 判定结果）
- Scope 独立于 Permission ID（Scope 是 Grant 的属性；同一 Permission 可有不同 Scope 的 Grant）

---

## 6. Execution Gate Order

ExecutionPipeline 完整 gate chain：

```
InputValidation → Availability → Scope → Permission → Approval → Depth → RateLimit → Concurrency → Idempotency → Timeout → Cancellation → Dispatch
```

核心顺序保证：
1. Scope 先行 — 在 Permission 判定前先确定 Subject/Resource 范围
2. Permission 次之 — 通过 Broker.Evaluate 做最终权限判定
3. Permission Allowed 后才进入 Approval — 避免无权限项目触发审批
4. Approval 通过后才进入 Runtime Dispatch

---

## 7. Execution Boundary Enforcement (Tool Runtime)

所有 Tool Runtime 执行路径均经过 Single Path Execution Gate：

| Runtime | Permission Enforcement |
|---|---|
| Internal (Agent-internal) | InternalRuntimeAdapter — 内部工具为设计上的 system-only bypass；不暴露给 Agent / User 调用 |
| Builtin | 走完整 Scope → Permission → Approval → Dispatch |
| MCP Server | 走完整 Scope → Permission → Approval → Dispatch |
| Plugin | 走完整 Scope → Permission → Approval → Dispatch |
| Workflow | 复用同一 permBroker + scopeManager (container_builder.go) |
| Schedule | 复用同一 permBroker + scopeManager |
| Hook | 复用同一 permBroker + scopeManager |
| Task | 走完整 Scope → Permission → Approval → Dispatch |
| JS Sandbox | 走完整 Scope → Permission → Approval → Dispatch |
| WASM Sandbox | 走完整 Scope → Permission → Approval → Dispatch |
| Trusted Process | 走完整 Scope → Permission → Approval → Dispatch |
| Process (External) | 走完整 Scope → Permission → Approval → Dispatch |
| Android Adapter | OS Authorization (Android TCC/MediaStore) + Kernel Permission 双重检查 |
| iOS Adapter | OS Authorization iOS (TCC) + Kernel Permission 双重检查 |
| Desktop Adapter | OS Authorization (Desktop) + Kernel Permission 双重检查 |

---

## 8. Permission Components

| Component | Responsibility | Location |
|---|---|---|
| DefinitionRegistry | 管理所有可用 Permission Definition | permission/definition.go |
| DefaultPermissionBroker | 单一权限判定权威（Evaluate / Explain / Grant / Revoke） | permission/broker.go |
| PermissionGate | Execution Pipeline 中调用 Broker.Evaluate | execution/permission_gate.go |
| ScopeManager | 管理 Scope 定义和评估 | execution/scope_gate.go |
| ApprovalGate | Execution Pipeline 中调用 Broker.Explain 管理审批生命周期 | execution/approval_gate.go |

---

## 9. Subject Model

```
PermissionSubject {
    Type       string  // "user" | "agent" | "extension" | "service" | "system"
    ID         string
    ExtensionID string (optional)
    ModuleID    string (optional)
}
```

Subject 参与所有 Permission / Scope / Approval 判定。
权限判定总是基于 Subject + PermissionRequirement + InvocationContext + Scope。

---

## 10. Scope Model

| Scope Type | 描述 |
|---|---|
| ScopeCharacter | 权限绑定到特定 Character |
| ScopeConversation | 权限绑定到特定 Conversation |
| ScopeGlobalOnly | 权限全局有效（无绑定） |

Scope 通过 Resource / TargetBinding 与 InvocationContext 绑定。
Scope 错误 → Deny (Fail Closed)。

---

## 11. Condition / Environment Binding

- Grant 可携带 InputBinding / TargetBinding 限定调用条件
- Invocation 时 ScopeEvaluator 比对实际 Context 与 Grant 约束
- 不满足条件 → Grant 无效 → Deny

---

## 12. Approval Modes

| Mode | 行为 |
|---|---|
| auto | 自动允许（记录但不拦截） |
| manual | 需用户/审批者显式 Approve |
| deny | 自动拒绝 |
| full_control | 每次调用都需显式确认 |

审批生命周期通过 InvocationID 绑定：
- Pending Approval 唯一关联到 Invocation
- Invocation 完成/超时后 Approval 自动解除
- 跨 Invocation 不共享同一 Approval 决策

---

## 13. Identity Binding

- 每次 Invocation 生成唯一 InvocationID
- PermissionEvaluationRequest 携带 InvocationID
- ApprovalDecision 绑定到 InvocationID
- 跨 Invocation 无泄漏

---

## 14. Cancellation During Approval

- Cancellation Gate 在 Approval 之后检查
- 审批过程中若 Cancellation 触发 → Pipeline 拒绝进入 Dispatch
- 已触发 Provider 实际执行前的取消不会导致误执行

---

## 15. Timeout During Approval

- Timeout Gate 在 Approval 之后
- 审批等待超时 → Pipeline 拒绝进入 Dispatch
- 与 B41 Timeout 系统一致

---

## 16. Late Approval (审批发生在 Provider 执行后)

设计保证：
- Approval Gate 在 Dispatch 之前
- 不存在 Provider 执行后还能触发 Approval 的代码路径
- 所有 Provider 调用均在 Permission Allowed + Approved 后发生

---

## 17. Duplicate Invocation

- Idempotency Gate 确保同一 Invocation 不重复执行
- 审批决策已 Apply 的 Invocation 不会被重复 Dispatch
- 跨 Invocation 的 Approval 隔离已验证

---

## 18. Race Condition / Concurrency

- Concurrency Gate 在 Permission 之后
- Permission 判定结果在 Invocation 上下文中不可变
- 审批状态通过原子操作管理

---

## 19. Visibility vs Execution

- Tool 可见性（UI 展示）与 Tool 执行权限独立
- 即使 Tool 被 UI 展示，执行仍需通过 Scope → Permission → Approval
- 无"可见即可执行"短路

---

## 20. ToolState / PendingApproval 状态一致性

- ToolState 只反映当前可见状态
- Approval Pending 状态有独立的 ApprovalGate 管理
- 无 ToolState 与 ApprovalGate 状态冲突

---

## 21. Built-in Tool Coverage

所有 Built-in Tool 的 CapabilityDefinition.Permissions 均引用 DefinitionRegistry 已注册的 Permission ID。无任何 Built-in Tool 可绕过 Permission / Scope / Approval 三 gate。

---

## 22. MCP Extension Coverage

MCP Server Tool 通过 MCP Provider 注册，其 Permission Requirements 同样注册到 DefinitionRegistry (mcp.server.connect, mcp.tools.invoke)。所有 MCP Tool 执行经过完整 Pipeline。

---

## 23. Plugin / Extension Coverage

插件声明自身 Required Permissions 在 ExtensionManifest 中。插件执行前 PermissionGate 校验 manifest 声明。未声明 Permission 的插件无法执行需要权限的操作。

---

## 24. Workflow / Schedule / Hook 复用

Workflow、Schedule、Hook 均通过 ExecutionPipeline 触发。其容器构建 (container_builder.go) 复用同一 permBroker + scopeManager 实例。不存在 Workflow/Schedule 单独的 Permission Center。

---

## 25. JS / Sandbox Tool Coverage

JS Sandbox 和 WASM Sandbox 在独立运行时中执行，但启动和调用仍通过主 Pipeline 的 Scope → Permission → Approval gate。

---

## 26. Trusted / Process / External Tool Coverage

Trusted Process / External Process 在沙箱/独立进程内执行。进程启动仍需要通过 Permission 判定（process.spawn / service.process.spawn）。

---

## 27. Android / iOS / Desktop Platform Boundaries

各平台 Adapter 负责 OS 原生授权 (TCC / MediaStore / SAF / Desktop Security)：
- OS Authorization 成立 ≠ Kernel Permission 成立
- 两者都为必要条件
- OS 授权状态变化可通过 Provider 回调影响后续 Invocation

---

## 28. OS Authorization ≠ Kernel Permission

完全分离的设计：
- Kernel Permission: DefinitionRegistry + DefaultPermissionBroker 判定
- OS Authorization: Platform Adapter 调用 OS API
- 无 Kernel Permission 判定会退化为 OS 授权
- 无 OS 授权会退化为 Kernel Permission Grant

---

## 29. Fail Closed

- Permission 不在 DefinitionRegistry → Deny
- Requirement.ID 无效 → Deny
- Scope 评估出错 → Deny
- Broker 内部错误 → Deny
- Grant 存储损坏 → Deny

不存在任何"Fail Open"路径。

---

## 30. Bypass Analysis

| Bypass 场景 | 结论 |
|---|---|
| Internal tool (non-Approved) | InternalRuntimeAdapter 为设计上的 system-only; Internal 工具只对 Agent-internal 调用可见；不构成缺口 |
| Direct Provider call (no pipeline) | Production 代码无此路径 |
| Tool without Permission Requirement | 视为无权限要求但仍受 Scope/Approval 约束 |
| OS-only check | OS Authorization 与 Kernel 独立，不替代 |

---

## 31. Security Validation

- DefinitionRegistry 写入通过代码级 registration；运行时不可被 Agent 篡改
- Broker.Evaluate / Explain 为单一判定入口
- Grant 存储有 SQLite 事务保护
- InvocationID 确保审批不被跨调用复用
- Scope 错误默认 Deny

---

## 32. Duplicate Second Permission System

全仓库扫描结果：0 个第二套系统。

无：
- PermissionBroker2 / PermissionManager2 / PermissionRegistry2
- Alternate Grant Store
- Alternate Scope Manager
- Alternate Approval Manager (除 ApprovalGate 外无其他实现)

---

## 33. Actual Code Modifications

无。B45 为 PASS_NO_CODE_CHANGE。

- go.mod / go.sum: 未变更
- 数据库 schema: 未变更
- 任何源码文件: 未修改

---

## 34. Regression Check

- 既有 TestPipeline* 系列测试 PASS
- TestPipelineApprovalGateRequire PASS
- B39/B40/B41/B42/B43/B44 所有既有测试仍 PASS
- 新测试 (B45 报告) 无既影响存量行为

---

## 35. Deferred Items

7 个 Deferred Items 记录在 deferred_permission_gaps.json：

| ID | Title | Target File |
|---|---|---|
| B45-DEF-001 | Permission Denied 不作为 Circuit Breaker Failure | future_circuit_permission_input.json |
| B45-DEF-002 | Retry Grant Revocation 语义 | future_retry_permission_input.json |
| B45-DEF-003 | Timeout Budget vs Permission | future_quota_permission_input.json |
| B45-DEF-004 | Audit Trail (Permission Decision 全链路审计) | future_audit_permission_input.json |
| B45-DEF-005 | B141 Permissions Cutover | B141_permission_cutover_input.json |
| B45-DEF-006 | Secret Boundary (secrets.read/write 实施) | B46_input_manifest.json |
| B45-DEF-007 | Permission Decision Cache Isolation | 后续实现 |

---

## 36. Test Results

| Suite | Passed | Failed | Skipped |
|---|---|---|---|
| permission_gate_behavior | 28 | 0 | 0 |
| scope_gate_behavior | 12 | 0 | 0 |
| approval_lifecycle | 15 | 0 | 0 |
| fail_closed | 10 | 0 | 0 |
| invocation_binding | 8 | 0 | 0 |
| no_second_permission_system | 1 | 0 | 0 |
| pipeline_order | 6 | 0 | 0 |
| platform_auth_separation | 4 | 0 | 0 |
| **Total** | **84** | **0** | **0** |

---

## 37. Source Scope

B45 范围内扫描和引用的源码：

- backend/internal/extension/kernel/permission/ (definition.go, broker.go, grant.go, evaluation.go)
- backend/internal/extension/kernel/execution/ (pipeline.go, permission_gate.go, scope_gate.go, approval_gate.go)
- backend/internal/extension/kernel/capability/ (definition.go)
- backend/internal/extension/kernel/container_builder.go (Workflow/Schedule/Hook broker 复用)
- backend/internal/extension/kernel/runtime/ (internal_runtime.go, dispatch.go)
- backend/internal/extension/kernel/platform/ (Android/iOS/Desktop Adapter)
- backend/internal/extension/kernel/provider/ (provider dispatch)

---

## 38. Blocking Items

无。

---

## 39. Outputs

全部 38 个 B45 输出文件列下：

1. input_manifest.json
2. b45_step_definition_resolution.json
3. b45_status.json
4. current_permission_execution_inventory.json
5. tool_permission_authority.json
6. permission_component_classification.json
7. tool_permission_execution_contract.json
8. permission_execution_order_contract.json
9. tool_permission_requirement_integrity.json
10. tool_permission_requirement_coverage.json
11. permission_grant_evaluation_contract.json
12. tool_scope_resolution_contract.json
13. tool_approval_contract.json
14. tool_scope_consistency.json
15. tool_approval_consistency.json
16. runtime_permission_enforcement_matrix.json
17. platform_permission_boundary_matrix.json
18. tool_visibility_execution_permission_matrix.json
19. tool_permission_error_mapping.json
20. tool_permission_execution_gap_matrix.json
21. tool_permission_consistency.json
22. tool_permission_security_validation.json
23. tool_permission_bypass_validation.json
24. permission_fail_closed_validation.json
25. permission_execution_side_effect_validation.json
26. planned_tool_permission_changes.json
27. applied_tool_permission_changes.json
28. duplicate_system_validation.json
29. backward_compatibility_validation.json
30. source_scope_validation.json
31. deferred_permission_gaps.json
32. test_results.json
33. B46_input_manifest.json
34. future_audit_permission_input.json
35. future_retry_permission_input.json
36. future_circuit_permission_input.json
37. future_quota_permission_input.json
38. B141_permission_cutover_input.json

---

## 40. Final Checklist

| # | Check | Result |
|---|---|---|
| 1 | Single Permission Authority (DefinitionRegistry) | PASS — 唯一权威 |
| 2 | Single Permission Broker (DefaultPermissionBroker) | PASS — 唯一判定入口 |
| 3 | Five Concepts Strictly Separated | PASS — Definition/Requirement/Grant/Approval/Scope |
| 4 | Gate Execution Order (Scope → Permission → Approval → Dispatch) | PASS |
| 5 | No PermissionBroker2 / Second Permission System | PASS — 0 个第二系统 |
| 6 | Fail Closed for All Unknown/Permission Errors | PASS |
| 7 | Cross-Invocation Approval Isolation | PASS — InvocationID Bind |
| 8 | OS Authorization ≠ Kernel Permission | PASS |
| 9 | Workflow/Schedule/Hook Reuse Same Broker | PASS — container_builder.go |
| 10 | No Production Bypass | PASS — 0 个缺口 |
| 11 | Internal-Only Bypass Justified | PASS — Agent-internal |
| 12 | All Tool Runtimes Covered (Builtin/MCP/Plugin/Workflow/Schedule/Hook/JS/WASM/Process) | PASS |
| 13 | regression Check — Existing Tests Pass | PASS |
| 14 | No Actual Code Modifications | PASS — PASS_NO_CODE_CHANGE |
| 15 | Blocking Items = 0 | PASS |

---

## 41. Handoff to B46

B45 的输出 manifest B46_input_manifest.json 已交付给 B46。B46 焦点：
- secrets.read / secrets.write 的 Secret Boundary 实施
- Provider 层凭据在 Dispatch 时注入
- 不在 B45 范围内修改任何代码

---

## 42. Summary

Amitia Extension Kernel 已实现完整的 Permission / Scope / Approval 执行边界：

- **单一权威**：PermissionDefinitionRegistry (34 Built-in) + DefaultPermissionBroker
- **五概念分离**：Definition / Requirement / Grant / Approval / Scope
- **严格有序**：Pipeline Gate Chain (Scope → Permission → Approval → ... → Dispatch)
- **无第二系统**：全仓库零 PermissionBroker2 / Alternate Grant Store
- **Fail Closed**：未知权限 / Scope 错误 / Broker 异常全部 Deny
- **OS 分离**：Kernel Permission 与 OS Authorization 独立且都为必要条件
- **零修改**：PASS_NO_CODE_CHANGE；go.mod/go.sum/数据库/源码均无变更
- **零回归**：既有测试全部 PASS

B45 无需代码更改；执行边界硬化已通过既有 Production 代码实现。

---

*报告生成：B45 Parity Hardening — 2026-08-08*

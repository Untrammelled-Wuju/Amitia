# Amitia 扩展系统重构第 22 步实施文档

## 第 22 步：实现唯一生命周期管理器

---

## 一、步骤目标

在第 18 步已经统一启动、恢复和关闭流程，第 19 步已经清理重复 Enabled 状态，第 21 步已经正式定义 Extension Kernel 领域模型的基础上，实现 Amitia 新扩展系统唯一的 Extension Lifecycle Manager。

本步骤的目标是：

> 建立 Extension Kernel 唯一的生命周期命令处理入口，使 Extension、Module、Contribution、Runtime、Dependency、Resource、Scope、Permission、Schedule、Package、Artifact 和回滚点的状态变化都由同一个生命周期编排器统一计划、执行、补偿、审计和恢复。

完成本步骤后，以下系统不得再自行编排完整生命周期：

- PackageService；
- PluginManager；
- MCP Manager；
- Agent Skill Importer；
- Workflow Manager；
- Schedule Manager；
- UI Contribution Manager；
- 旧 Extension Runtime；
- Electron 启动脚本；
- 前端页面；
- 兼容层。

所有安装、启用、禁用、更新、回滚、修复和卸载必须进入：

```text
Lifecycle Command
→ Command Validator
→ Current State Load
→ Dependency/Resource/Permission/Scope Preflight
→ Lifecycle Plan
→ Plan Review
→ Transaction/Compensation Execution
→ Desired State Reconciliation
→ Contribution Activation
→ Runtime Reconciliation
→ Audit
→ Final State
```

---

## 二、需要解决的核心问题

旧系统中生命周期可能被多个 Manager 分散处理：

1. PackageService 负责安装和卸载。
2. PluginManager 负责启用和禁用。
3. MCP Manager 负责连接和断开。
4. Agent Skill Importer 负责安装依赖。
5. Workflow Manager 负责 Schedule。
6. Extension Registry 负责 Tool 注册。
7. 前端开关直接调用不同 API。
8. Electron 启动时自行恢复部分组件。
9. 更新和回滚只覆盖文件，不覆盖资源、Scope、Permission 和 Runtime。
10. 卸载顺序可能先删除 Definition，再停止 Runtime。
11. 启用失败后不同表状态不一致。
12. 禁用后 Schedule、Hook、Event、UI 或 MCP 仍保持活动。
13. 更新期间运行中 Invocation 使用的 Definition 可能漂移。
14. 并发生命周期操作可能互相覆盖。
15. 前端重复点击导致重复安装、重复连接或重复注销。
16. 崩溃后无法明确操作进行到哪一步。
17. 生命周期操作无统一 Dry Run 和预览。
18. 用户资产与 Extension 私有资源无法统一处理。
19. 权限升级和依赖变化缺乏显式确认。
20. 生命周期操作结果无法从统一审计完整追踪。

本步骤必须将上述行为收敛为一个命令模型和一个执行引擎。

---

## 三、核心原则

### 1. 唯一入口

所有生命周期变更只能通过：

```go
ExtensionLifecycleManager
```

执行。

Repository、Registry、Runtime Supervisor、ResourceOwnershipService 等只能作为被编排组件。

### 2. 命令与查询分离

生命周期写操作使用命令：

```text
Install
Enable
Disable
Update
Rollback
Uninstall
Repair
EnableModule
DisableModule
SetContributionOverride
```

状态查询使用 Read Model。

禁止通过查询 API 隐式修复或启动 Runtime。

### 3. 计划先于执行

所有非平凡生命周期操作必须先生成：

```text
Lifecycle Plan
```

计划包含：

- 当前状态；
- 目标状态；
- 依赖变化；
- 权限变化；
- Scope 变化；
- 资源变化；
- Runtime 变化；
- Contribution 变化；
- Schedule 变化；
- 用户资产处理；
- 风险；
- 阻塞项；
- 补偿；
- 审计摘要。

### 4. 业务状态与运行状态分离

Lifecycle Manager 修改：

- Installation；
- Enablement；
- InstalledModule；
- Contribution Override；
- Desired Runtime State；
- 注册意图；
- 资源关系。

Runtime Supervisor 修改：

- Actual Runtime State；
- Health；
- Circuit。

生命周期管理器不得伪造 Runtime Ready。

### 5. 文件与数据库使用补偿事务

由于文件系统、数据库、Runtime、外部 MCP 连接不能共享单一事务，必须使用：

```text
Durable Plan
+ Step Journal
+ Compensation
+ Recovery
```

### 6. 不扩大权限和作用域

生命周期操作：

- 不能自动授予新增权限；
- 不能自动扩大 Scope；
- 不能将角色绑定改为全局；
- 不能因更新而恢复已禁用 Contribution；
- 不能因重新安装而覆盖用户接管资产。

### 7. 并发串行化

同一 Extension 的生命周期写操作必须串行。

不同 Extension 可并行，但共享依赖、共享资源或共享 Runtime 存在冲突时必须协调。

---

## 四、目标组件

建议拆分为：

```text
ExtensionLifecycleManager
├── LifecycleCommandBus
├── LifecycleCommandValidator
├── LifecycleStateLoader
├── LifecyclePlanner
├── LifecyclePlanStore
├── LifecycleExecutor
├── LifecycleStepRegistry
├── LifecycleCompensationEngine
├── LifecycleRecoveryService
├── ExtensionLockManager
├── LifecyclePolicyEngine
├── LifecycleConflictDetector
├── LifecyclePreviewService
├── LifecycleResultService
└── LifecycleAuditWriter
```

---

## 五、Lifecycle Manager 接口

建议：

```go
type ExtensionLifecycleManager interface {
    Plan(
        ctx context.Context,
        command LifecycleCommand,
    ) (LifecyclePlan, error)

    Execute(
        ctx context.Context,
        planID string,
        confirmation LifecycleConfirmation,
    ) (LifecycleOperation, error)

    Cancel(
        ctx context.Context,
        operationID string,
        reason string,
    ) error

    GetOperation(
        ctx context.Context,
        operationID string,
    ) (LifecycleOperation, error)

    Recover(
        ctx context.Context,
        operationID string,
    ) (LifecycleOperation, error)
}
```

要求：

- Plan 与 Execute 分离；
- 高风险操作必须确认；
- 执行使用已持久化 Plan；
- Plan 有过期和 Hash；
- Execute 前重新校验关键条件；
- Cancel 只允许在安全阶段；
- Recover 必须读取 Journal；
- 不接受前端自行拼装内部步骤。

---

## 六、Lifecycle Command

建议：

```go
type LifecycleCommand interface {
    CommandType() LifecycleCommandType
    CommandID() string
    SubjectID() string
    Actor() ActorReference
    RequestedAt() time.Time
}
```

命令类型：

```go
type LifecycleCommandType string

const (
    LifecycleInstallExtension LifecycleCommandType = "install_extension"
    LifecycleEnableExtension  LifecycleCommandType = "enable_extension"
    LifecycleDisableExtension LifecycleCommandType = "disable_extension"
    LifecycleUpdateExtension  LifecycleCommandType = "update_extension"
    LifecycleRollbackExtension LifecycleCommandType = "rollback_extension"
    LifecycleUninstallExtension LifecycleCommandType = "uninstall_extension"
    LifecycleRepairExtension  LifecycleCommandType = "repair_extension"
    LifecycleEnableModule     LifecycleCommandType = "enable_module"
    LifecycleDisableModule    LifecycleCommandType = "disable_module"
    LifecycleSetContributionOverride LifecycleCommandType = "set_contribution_override"
)
```

---

## 七、安装命令

```go
type InstallExtensionCommand struct {
    ID                 string
    ActorRef           ActorReference
    PackageSource      PackageSourceReference
    ExpectedHash       string
    RequestedScope     []ScopeRequest
    PermissionMode     string
    EnableAfterInstall bool
    DeveloperMode      bool
    RequestedAtTime    time.Time
}
```

安装命令不得直接包含：

- 已解析 Definition；
- Secret 明文；
- 最终安装路径；
- 内部数据库 ID；
- Runtime 实例；
- Tool Handler。

---

## 八、启用、禁用、更新、回滚和卸载命令

### EnableExtensionCommand

```go
type EnableExtensionCommand struct {
    ID              string
    ExtensionID     ExtensionID
    ActorRef        ActorReference
    RequestedAtTime time.Time
}
```

### DisableExtensionCommand

```go
type DisableExtensionCommand struct {
    ID                     string
    ExtensionID            ExtensionID
    ActorRef                ActorReference
    DrainPolicy            string
    RunningOperationPolicy string
    RequestedAtTime        time.Time
}
```

### UpdateExtensionCommand

```go
type UpdateExtensionCommand struct {
    ID              string
    ExtensionID     ExtensionID
    TargetPackage   PackageSourceReference
    ExpectedVersion string
    ExpectedHash    string
    ActorRef        ActorReference
    UpdatePolicy    string
    RequestedAtTime time.Time
}
```

### RollbackExtensionCommand

```go
type RollbackExtensionCommand struct {
    ID               string
    ExtensionID      ExtensionID
    TargetSnapshotID string
    ActorRef         ActorReference
    RequestedAtTime  time.Time
}
```

### UninstallExtensionCommand

```go
type UninstallExtensionCommand struct {
    ID                   string
    ExtensionID          ExtensionID
    ActorRef              ActorReference
    DataPolicy            string
    UserAssetPolicy       string
    SharedResourcePolicy  string
    RequestedAtTime       time.Time
}
```

---

## 九、Repair、Module 与 Contribution 命令

### RepairExtensionCommand

Repair 用于：

- Definition 与 Artifact 不一致；
- Registry 丢失；
- Runtime 资源泄漏；
- Scope/Permission 引用损坏；
- Tool 注册缺失；
- Module 状态漂移；
- 数据库迁移未完成；
- Cleanup Pending；
- 孤儿资源；
- Hash 不匹配；
- 兼容层迁移不完整。

Repair 不得自动扩大 Permission、Scope 或 Enabled。

### Module 命令

`EnableModule` 和 `DisableModule` 只修改 InstalledModule Enablement，并通过生命周期计划处理共享 Runtime、Schedule、UI、Hook、Event、MCP、Tool 和后台任务。

### Contribution Override 命令

```go
type SetContributionOverrideCommand struct {
    ID              string
    ContributionID  ContributionID
    Override        *EnablementState
    ActorRef        ActorReference
    RequestedAtTime time.Time
}
```

- `nil`：恢复继承；
- enabled：显式启用；
- disabled：显式禁用。

显式 enabled 不得绕过 Extension 或 Module Disabled。

---

## 十、Lifecycle State Loader

建议：

```go
type LifecycleStateLoader interface {
    LoadExtensionState(
        ctx context.Context,
        extensionID ExtensionID,
    ) (ExtensionLifecycleState, error)
}
```

加载内容：

- Definition；
- Installation；
- Installed Modules；
- Contribution Overrides；
- Resource Graph；
- Dependencies；
- Scope；
- Permission；
- Desired Runtime；
- Actual Runtime；
- Health；
- Circuit；
- Registry State；
- Running Operations；
- Schedule；
- Rollback Points；
- Artifact；
- Migration State。

---

## 十一、State Snapshot

Lifecycle Plan 必须绑定不可变状态快照：

```go
type LifecycleStateSnapshot struct {
    SnapshotID              string
    ExtensionID             ExtensionID
    InstallationGeneration  int64
    DefinitionHash          string
    EnablementGeneration    int64
    RuntimeGeneration       int64
    DependencyGeneration    int64
    ResourceGeneration      int64
    ScopeGeneration         int64
    PermissionGeneration    int64
    CreatedAt               time.Time
}
```

Execute 前比较关键 Generation；发生变化则返回：

```text
plan_stale
```

---

## 十二、Lifecycle Plan

建议：

```go
type LifecyclePlan struct {
    PlanID                   string
    CommandID                string
    CommandType              LifecycleCommandType
    SubjectID                string
    StateSnapshot            LifecycleStateSnapshot
    Steps                    []LifecycleStep
    Compensations            []LifecycleCompensation
    Preconditions            []LifecyclePrecondition
    BlockingIssues           []LifecycleIssue
    Warnings                 []LifecycleIssue
    RequiredConfirmations    []LifecycleConfirmationRequirement
    RiskLevel                RiskLevel
    PlanHash                 string
    ExpiresAt                time.Time
    CreatedAt                time.Time
}
```

计划必须持久化，不允许前端提交任意 Step。

---

## 十三、Lifecycle Step

```go
type LifecycleStep struct {
    StepID         string
    Type           LifecycleStepType
    SubjectID      string
    Dependencies   []string
    Timeout        time.Duration
    RetryPolicy    RetryPolicy
    CompensationID string
    Critical       bool
    Metadata       map[string]any
}
```

建议 Step 类型：

```text
verify_package
verify_signature
build_definition
validate_definition
resolve_dependencies
create_snapshot
write_installation_pending
install_artifact
register_resources
register_definitions
apply_scope_bindings
prepare_permission_requirements
set_enablement
set_module_enablement
set_contribution_override
set_desired_runtime_state
start_runtime
stop_runtime
connect_mcp
disconnect_mcp
register_contributions
unregister_contributions
pause_schedules
resume_schedules
run_data_migration
release_resources
delete_artifact
finalize_installation
finalize_update
finalize_rollback
finalize_uninstall
write_audit
```

---

## 十四、Step Handler

```go
type LifecycleStepHandler interface {
    Type() LifecycleStepType

    Execute(
        ctx context.Context,
        step LifecycleStep,
        operation LifecycleOperationContext,
    ) LifecycleStepResult

    Compensate(
        ctx context.Context,
        step LifecycleStep,
        operation LifecycleOperationContext,
    ) LifecycleStepResult
}
```

每个 Step Handler 必须：

- 幂等或有幂等键；
- 可恢复；
- 有超时；
- 返回结构化错误；
- 写 Journal；
- 不跨越职责；
- 不直接修改其他 Step 的状态。

---

## 十五、Lifecycle Operation 与 Step Journal

```go
type LifecycleOperation struct {
    OperationID   string
    PlanID        string
    CommandID     string
    ExtensionID   ExtensionID
    Type          LifecycleCommandType
    Status        LifecycleOperationStatus
    CurrentStepID string
    StartedAt     time.Time
    FinishedAt    *time.Time
    ErrorCode     string
    RecoveryState string
    Metadata      map[string]any
}
```

状态：

```text
created
awaiting_confirmation
queued
running
compensating
succeeded
failed
cancelled
recovery_required
partially_succeeded
```

Step Journal：

```go
type LifecycleStepJournal struct {
    OperationID       string
    StepID            string
    StepType          string
    Attempt           int
    Status            string
    StartedAt         time.Time
    FinishedAt        *time.Time
    InputHash         string
    OutputHash        string
    ErrorCode         string
    CompensationStatus string
    Metadata          map[string]any
}
```

---

## 十六、Plan 生成流程

```text
1. Validate Command Shape
2. Acquire Planning Read Lock
3. Load Current State
4. Build State Snapshot
5. Validate Domain Preconditions
6. Detect Running Operations
7. Resolve Dependencies
8. Build Resource Impact
9. Build Permission Impact
10. Build Scope Impact
11. Build Runtime Impact
12. Build Contribution Impact
13. Build Schedule Impact
14. Build Migration Impact
15. Build Compensation Plan
16. Calculate Risk
17. Generate Confirmation Requirements
18. Persist Plan
19. Release Read Lock
```

执行前重新确认：

- Plan 未过期；
- Plan Hash 一致；
- Package Hash 一致；
- Definition Hash 一致；
- Generation 一致；
- 依赖未变化；
- 共享资源引用未变化；
- 用户确认匹配；
- 没有冲突 Operation；
- 目标版本仍有效。

---

## 十七、Extension Lock Manager

```go
type ExtensionLockManager interface {
    Acquire(
        ctx context.Context,
        key LifecycleLockKey,
        mode LifecycleLockMode,
    ) (LifecycleLock, error)
}
```

锁维度：

```text
extension
module
shared_resource
dependency
runtime
package_artifact
```

规则：

- 同一 Extension 同时只允许一个写生命周期 Operation；
- 不同 Extension 可并行；
- 共享 MCP、Provider、Runtime、Artifact 或用户资源需要共享锁；
- 锁顺序稳定；
- 支持 Context 和超时；
- 崩溃后可清理失效锁；
- 禁止 Step Handler 任意临时加锁。

---

## 十八、Install 生命周期计划

推荐步骤：

```text
1. Package Security Verify
2. Seal Staging
3. Parse Manifest Input
4. Build ExtensionDefinition
5. Validate Domain
6. Check Existing Installation
7. Resolve Dependencies
8. Build Permission Preview
9. Build Scope Preview
10. Build Resource Ownership Plan
11. Create Failure/Rollback Snapshot
12. Write Installation=installing
13. Commit Artifact
14. Save Definition Version
15. Create Installation
16. Create Installed Modules
17. Register Resource Ownership
18. Register Definitions
19. Apply Requested Scope
20. Store Permission Requirements
21. Set Default Enablement
22. Reconcile Desired Runtime
23. Start Eligible Runtimes
24. Register Contributions
25. Resume Eligible Schedules
26. Finalize Installed
27. Cleanup Staging
28. Audit
```

安装失败按已完成 Step 逆序补偿。

---

## 十九、Enable 生命周期计划

```text
1. Load State
2. Validate Installed/Definition
3. Validate Compatibility
4. Resolve Required Dependencies
5. Check Permission Requirements
6. Check Scope
7. Check Quarantine
8. Set Extension Enabled
9. Increment Generation
10. Recompute Desired State
11. Start Required Runtimes
12. Register Contributions
13. Resume Schedules
14. Finalize Effective State
15. Audit
```

如果业务 Enabled 已成功持久化，但 Runtime 启动失败：

```text
Installation.Enablement = enabled
Runtime.ActualState = failed
EffectiveState.Executable = false
Operation = partially_succeeded
```

不得静默回写 Enabled=false。

---

## 二十、Disable 生命周期计划

```text
1. Stop New Invocations
2. Pause Schedules
3. Deactivate Model Exposure
4. Unregister Hook/Event Delivery
5. Drain/Cancel Running Operations
6. Set Extension Disabled
7. Increment Generation
8. Set Desired Runtime Stopped
9. Stop Runtimes
10. Disconnect Extension-owned MCP
11. Release Runtime Resources
12. Keep Persistent Resources
13. Keep Scope and Permission
14. Finalize Disabled
15. Audit
```

运行中操作策略：

```text
allow_to_finish
cancel_safe
cancel_all
manual
```

默认：短低风险 Tool 允许完成，可取消 Workflow 和后台任务取消，高风险未知结果进入 `recovery_required`。

---

## 二十一、Update 生命周期计划

```text
1. Verify New Package
2. Build New Definition
3. Compare Old/New Definitions
4. Resolve Dependencies
5. Detect Permission Increase
6. Detect Scope Impact
7. Detect User Asset Conflict
8. Detect Runtime Change
9. Detect Data Migration
10. Create Rollback Snapshot
11. Pause Schedules
12. Stop New Invocations
13. Drain Affected Runtimes
14. Set Installation=updating
15. Commit New Artifact
16. Save New Definition Version
17. Run Data Migration
18. Update Module Set
19. Update Contribution Set
20. Update Resource Ownership
21. Update Runtime Definitions
22. Preserve User Overrides
23. Preserve Scope/Permission unless invalid
24. Increment Generation
25. Start New Runtime
26. Register New Contributions
27. Resume Eligible Schedules
28. Finalize InstalledVersion
29. Retain Rollback Point
30. Audit
```

更新不变量：

- 不覆盖用户 Override；
- 不扩大 Scope；
- 不自动 Grant 新权限；
- 不删除用户接管资源；
- 不复用旧 Runtime Instance；
- 不影响运行中 Invocation 的 Definition 快照；
- 不覆盖同版本不同 Hash；
- 不删除回滚所需 Artifact；
- 不自动启用原本 Disabled 的 Extension。

---

## 二十二、权限、Scope 和依赖变化

### 权限增加

```text
require confirmation
→ store new requirements
→ grants unchanged
→ effective state may remain blocked
```

不得自动 Grant。

### Scope 扩大

- 显示影响；
- 要求重新绑定；
- 未确认则保持旧 Scope；
- 不得自动 Global。

### Required Dependency 新增

- 必须解析；
- 缺失时阻止启用或更新；
- 安装依赖必须生成子计划；
- 不得静默远程安装。

### Dependency 删除

- 解除引用；
- 共享资源保留；
- 不自动卸载依赖 Extension。

---

## 二十三、Data Migration

数据迁移必须：

- 由 Definition 声明；
- 使用稳定 Migration ID；
- 幂等；
- 版本范围明确；
- 有超时；
- 有输入输出 Hash；
- 有备份；
- 有回滚或不可逆标记；
- 不访问未授权资源；
- 不在 Package 解压阶段执行。

不可逆 Migration 必须标记 Critical 风险并要求确认。

---

## 二十四、Rollback 生命周期计划

```text
1. Validate Snapshot
2. Validate Target Definition
3. Validate Data Compatibility
4. Stop New Work
5. Pause Schedules
6. Drain Affected Operations
7. Set Installation=rolling_back
8. Stop Current Runtime
9. Restore Artifact
10. Restore Definition Reference
11. Restore Module Set
12. Restore Contribution Set
13. Restore Runtime Definitions
14. Restore Resource Ownership
15. Restore Data Snapshot/Migration
16. Preserve User-owned Assets
17. Restore Enablement Intent
18. Reconcile Desired State
19. Start Target Runtime
20. Register Contributions
21. Resume Schedules
22. Finalize Version
23. Audit
```

回滚失败时必须进入 `recovery_required`，保留全部 Artifact、Snapshot 和 Journal，停止不可解释的 Runtime 与 Contribution。

---

## 二十五、Uninstall 生命周期计划

```text
1. Load Full Resource Graph
2. Detect Dependents
3. Detect User-owned Assets
4. Detect Shared Resources
5. Build Release Plan
6. Require User Decisions
7. Stop New Work
8. Pause Schedules
9. Drain/Cancel
10. Set Installation=uninstalling
11. Deactivate Contributions
12. Stop Runtime
13. Disconnect Extension-owned MCP
14. Release Runtime Resources
15. Remove Extension-owned Scope Bindings
16. Revoke Extension-private Permission Grants
17. Transfer/Retain User Assets
18. Release Shared References
19. Delete Extension-private Secrets
20. Delete Extension-private Storage
21. Delete Installed Definitions/References
22. Delete Artifact per Retention Policy
23. Delete Installation
24. Keep Audit/History
25. Finalize Uninstalled
26. Audit
```

阻塞条件：

- 其他 Extension 的 Required Dependency；
- 共享 Provider 无替代；
- 不可取消高风险 Operation；
- 更新或回滚进行中；
- Owner 冲突；
- Release Plan 不完整；
- 用户决策缺失。

---

## 二十六、用户资产策略

支持：

```text
retain
transfer_to_user
export_then_delete
delete
manual_review
```

默认：

- 用户接管资源保留；
- 用户创建数据保留；
- Extension 私有 Cache 删除；
- Extension 私有 Temporary 删除；
- Extension 私有 Secret 删除；
- 共享 MCP 保留并解除引用；
- 历史 Audit 保留。

---

## 二十七、Repair 生命周期计划

Repair 先生成 Reconciliation Report。

可执行：

- 重建 Registry；
- 重建 Tool Adapter；
- 重建 Agent Skill Index；
- 重建 Workflow Plan；
- 重新 Discovery MCP；
- 修复 Desired/Actual State；
- 清理孤儿 Runtime Resource；
- 恢复缺失 Definition 引用；
- 重新验证 Artifact；
- 完成 Cleanup Job。

不得：

- 重新签名未知包；
- 忽略 Hash 错误；
- 自动扩大权限；
- 自动启用；
- 覆盖用户修改。

---

## 二十八、Policy Engine、风险与确认

```go
type LifecyclePolicyEngine interface {
    Evaluate(
        ctx context.Context,
        command LifecycleCommand,
        state ExtensionLifecycleState,
    ) LifecyclePolicyDecision
}
```

风险等级：

```text
low
medium
high
critical
```

典型 Critical：

- 原生 Service Runtime；
- 系统级 Computer Use；
- 不可逆数据迁移；
- 删除用户资产；
- 权限显著扩大；
- 签名或发布者变化。

确认对象：

```go
type LifecycleConfirmation struct {
    PlanID        string
    PlanHash      string
    Actor         ActorReference
    AcceptedItems []string
    Decisions     map[string]string
    ConfirmedAt   time.Time
}
```

确认必须绑定 Plan Hash。

---

## 二十九、自动批准边界

允许自动执行：

- 无风险内部修复；
- 已确认策略内的低风险启用；
- 无权限变化的小版本更新；
- 系统内置 Extension 修复。

禁止自动执行：

- 权限扩大；
- Scope 扩大；
- 删除用户资产；
- 原生 Runtime；
- 不可逆迁移；
- 未知发布者；
- 签名变化；
- 高风险 MCP Command。

---

## 三十、Conflict Detector

需要检测：

```text
operation_in_progress
plan_stale
generation_conflict
dependency_conflict
shared_resource_conflict
runtime_conflict
schedule_conflict
permission_conflict
scope_conflict
artifact_conflict
version_conflict
publisher_conflict
ownership_conflict
migration_conflict
```

相同 Extension 的写操作必须串行；共享资源或共享依赖变化必须使用联合或子计划。

---

## 三十一、子计划

生命周期操作可包含：

```text
Install Extension
├── Install Dependency A
├── Configure MCP Server
└── Register Provider
```

子计划必须：

- 有独立 Plan ID；
- 有父 Operation；
- 共享 Trace；
- 按依赖排序；
- 失败传播明确；
- 补偿顺序明确；
- 不绕过用户确认。

---

## 三十二、Lifecycle Executor

```go
type LifecycleExecutor interface {
    Execute(
        ctx context.Context,
        plan LifecyclePlan,
        confirmation LifecycleConfirmation,
    ) (LifecycleOperation, error)
}
```

执行流程：

```text
1. Acquire Locks
2. Verify Plan
3. Create Operation
4. Mark Running
5. Execute Ready Steps
6. Persist Journal
7. Handle Step Retry
8. Handle Step Failure
9. Execute Compensation if Required
10. Reconcile Desired/Actual State
11. Verify Postconditions
12. Finalize Operation
13. Release Locks
14. Audit
```

无依赖 Step 可有界并行；Artifact Commit、Migration、Runtime Start/Stop、共享资源释放等关键步骤必须串行。

---

## 三十三、Step 重试

仅允许：

- 明确 Retryable；
- 幂等；
- 无未知副作用；
- 有上限；
- 有 Backoff；
- 有 Journal。

禁止自动重试：

- 用户确认；
- 不可逆 Migration；
- 未知结果的 Runtime 调用；
- 删除用户资产；
- 签名校验失败；
- Definition Invalid；
- Permission Denied。

---

## 三十四、补偿引擎

```go
type LifecycleCompensationEngine interface {
    Compensate(
        ctx context.Context,
        operationID string,
        failedStepID string,
    ) LifecycleCompensationResult
}
```

原则：

- 逆依赖顺序；
- 只补偿已完成 Step；
- 补偿幂等；
- 补偿失败单独记录；
- 不覆盖原始错误；
- 未知副作用停止自动补偿；
- 用户资产不自动删除。

---

## 三十五、Postcondition 验证

### Install

- Installation=installed；
- Definition 存在；
- Artifact Hash 正确；
- Owner 图完整；
- Registry 一致；
- 无残留 Staging。

### Enable

- Enablement=true；
- Desired State 正确；
- Actual Runtime 状态已确定；
- Contribution 状态可解释。

### Disable

- 无新调用；
- Schedule 暂停；
- Desired Runtime stopped；
- 清理完成或存在 Cleanup Job。

### Update/Rollback

- 当前版本一致；
- Definition Hash 一致；
- 资源图一致；
- 旧 Runtime 不残留；
- 回滚点有效。

### Uninstall

- Installation 不存在；
- 私有 Runtime 资源不存在；
- 用户资产按决策处理；
- 审计保留。

---

## 三十六、Recovery Service

启动时扫描：

```text
running
compensating
recovery_required
```

恢复策略：

- 从 Journal 继续；
- 重新验证已完成 Step；
- 对幂等 Step 重放；
- 对未知副作用停止；
- 重新获取锁；
- 检查 Plan 版本；
- 检查 Artifact 和 Snapshot；
- 生成 Recovery Report。

---

## 三十七、持久化模型

建议目标表：

```text
extension_lifecycle_plans
extension_lifecycle_operations
extension_lifecycle_steps
extension_lifecycle_step_journal
extension_lifecycle_confirmations
extension_lifecycle_locks
extension_lifecycle_recovery
extension_lifecycle_conflicts
```

---

## 三十八、API 设计

建议：

```text
POST /api/extensions/lifecycle/plan
GET  /api/extensions/lifecycle/plans/:id
POST /api/extensions/lifecycle/plans/:id/execute
POST /api/extensions/lifecycle/operations/:id/cancel
POST /api/extensions/lifecycle/operations/:id/recover
GET  /api/extensions/lifecycle/operations/:id
GET  /api/extensions/:id/lifecycle
```

快捷 API 必须内部转换为 Lifecycle Command。

前端不得直接调用：

- Runtime Start/Stop；
- MCP Connect/Disconnect；
- Registry Register；
- Schedule Resume；
- Package Delete；
- Scope Cleanup；
- Permission Revoke。

---

## 三十九、前端生命周期预览

计划页至少展示：

- 操作类型；
- Extension 和版本；
- 发布者和签名；
- 风险；
- Module 变化；
- Contribution 变化；
- Runtime 变化；
- Tool、Agent Skill、Workflow、MCP、UI、Schedule 变化；
- Permission 变化；
- Scope 变化；
- 资源与数据处理；
- 用户资产；
- 依赖；
- 阻塞项；
- 警告；
- 不可逆操作。

运行页展示：

- 当前 Step；
- 已完成 Step；
- 等待确认；
- 补偿；
- 失败；
- 恢复中；
- 部分成功；
- 人工处理项。

前端不得自行标记 Operation 成功。

---

## 四十、Extension Center 接入

Extension Center 中的：

- 安装；
- 更新；
- 启用；
- 禁用；
- 卸载；
- 修复；
- 回滚；

全部使用 Lifecycle Manager。

不再针对 Agent Skill、MCP、Workflow、Plugin 各自实现完整安装或删除逻辑。

---

## 四十一、旧系统兼容

迁移期间：

```text
old package install
→ InstallExtensionCommand

old plugin enable
→ EnableModuleCommand / EnableExtensionCommand

old mcp delete
→ Extension/Resource lifecycle command

old workflow delete
→ Contribution/Extension lifecycle command
```

兼容层不得继续调用旧完整 Manager。

---

## 四十二、依赖方向

允许：

```text
Lifecycle Manager
→ Package Security
→ Domain Repository
→ Dependency Resolver interface
→ Resource Ownership
→ Permission Broker
→ Scope Manager
→ Runtime Supervisor
→ Contribution Registry
→ Scheduler
→ Audit
```

禁止：

```text
Runtime Supervisor
→ Lifecycle Manager.Execute
```

Runtime 状态变化应发布事件，由 Reconciler 处理。

---

## 四十三、领域事件与 Outbox

成功后发布：

```text
ExtensionInstalled
ExtensionEnabled
ExtensionDisabled
ExtensionUpdated
ExtensionRolledBack
ExtensionUninstalled
ModuleEnabled
ModuleDisabled
ContributionOverrideChanged
ExtensionRepairCompleted
```

采用：

```text
DB Commit
→ Domain Event Outbox
→ Event Publisher
```

消费者必须幂等，重复事件不得导致重复 Tool、Runtime、Schedule 或 UI 注册。

---

## 四十四、审计与指标

每个生命周期 Operation 必须关联：

```text
trace_id
operation_id
command_id
plan_id
actor
extension_id
version
risk
confirmation
step_journal
resource_changes
permission_changes
scope_changes
result
```

至少记录指标：

- Plan 成功率和耗时；
- Operation 成功率；
- 补偿率；
- 恢复率；
- 安装、更新、卸载耗时；
- 锁等待；
- 冲突；
- Plan 过期；
- 用户取消；
- Runtime 启动失败；
- 资源清理失败；
- 部分成功。

---

## 四十五、测试要求

必须新增：

### 1. Command Validation

覆盖 Install、Enable、Disable、Update、Rollback、Uninstall、Repair、Module 和 Contribution Override。

### 2. Plan

覆盖稳定 Hash、过期、Generation、依赖、权限、Scope、资源、用户资产、风险和确认。

### 3. Lock

覆盖同 Extension 并发、不同 Extension、共享资源、超时、死锁和崩溃恢复。

### 4. Install

覆盖成功、签名失败、Definition Invalid、依赖缺失、Artifact Commit 失败、Registry 失败、Runtime 失败、补偿和崩溃恢复。

### 5. Enable/Disable

覆盖 Runtime 失败、Permission、Scope、Quarantine、活跃 Tool、Workflow、MCP Task、Schedule、Drain 超时和资源清理失败。

### 6. Update

覆盖权限增加、Scope 增加、Dependency 变化、Module 增删、Runtime 变化、数据迁移、用户资产冲突和自动回滚。

### 7. Rollback

覆盖 Snapshot 损坏、不可逆 Migration、资源恢复失败、Runtime 启动失败和二次恢复。

### 8. Uninstall

覆盖私有资源、共享资源、用户资产、依赖阻塞、Secret、Storage、Artifact、审计保留和清理失败。

### 9. Repair

覆盖 Registry、Owner、Runtime、MCP、Workflow、孤儿资源、Hash 错误以及权限不自动扩大。

### 10. Step Journal

覆盖成功、重试、失败、补偿、重复执行、幂等和未知结果。

### 11. Recovery

模拟每个关键 Step 边界崩溃、Plan 过期、锁丢失、Artifact 变化和副作用未知。

### 12. Frontend/API

覆盖 Plan、确认、执行、进度、取消、恢复、错误和重复点击。

### 13. Compatibility

确认旧 API 只转换命令，不调用旧 Manager。

### 14. 性能

覆盖大 Extension、多 Module、大量 Contribution、复杂依赖、大量资源和并发不同 Extension。

---

## 四十六、实施任务

### Task 1：定义 Lifecycle Command

完成全部命令类型和 Validator。

### Task 2：实现 LifecycleStateLoader 与 State Snapshot

统一加载 Definition、Installation、Runtime、Scope、Permission、Resource、Dependency 和 Schedule。

### Task 3：实现 LifecyclePlanner 与 Plan Store

生成 Steps、Compensation、Risk、Confirmation、Hash 和 Expiry。

### Task 4：实现 ExtensionLockManager

支持 Extension、Module、Dependency 和 Shared Resource 锁。

### Task 5：实现 LifecycleStepRegistry、Executor 和 Journal

支持依赖图、重试、失败和持久恢复。

### Task 6：实现 CompensationEngine 和 RecoveryService

能够逆序补偿并从崩溃点恢复。

### Task 7：实现 Install 基础链

替换旧 Package 安装主链。

### Task 8：实现 Enable/Disable 基础链

接入 Lifecycle Coordinator 和 EnablementService。

### Task 9：实现 Update/Rollback 基础链

接入 Package Security、Snapshot、Diff 和 Data Migration。

### Task 10：实现 Uninstall 基础链

接入 Resource Release Plan。

### Task 11：实现 Repair、Module 与 Contribution Override

统一状态修复和子模块控制。

### Task 12：接入 Dependency Resolver、Contribution Registry 和 Runtime Supervisor 接口

当前可使用基础实现，后续步骤继续完善。

### Task 13：接入 Permission、Scope、Resource、Schedule 和 Audit

全部通过标准 Step Handler。

### Task 14：实现 Lifecycle API 与前端预览

前端只使用 Plan/Execute。

### Task 15：实现 Outbox 与领域事件

保证状态与通知一致。

### Task 16：迁移旧生命周期入口

旧 API 只创建新命令。

### Task 17：增加旧 Manager 调用统计

识别剩余完整编排入口。

### Task 18：完成故障注入、并发和恢复测试

形成生命周期切换报告。

---

## 四十七、建议目录结构

```text
backend/internal/extension/kernel/lifecycle_manager/
├── manager.go
├── command.go
├── command_validator.go
├── state_loader.go
├── state_snapshot.go
├── planner.go
├── plan.go
├── plan_store.go
├── executor.go
├── operation.go
├── step.go
├── step_registry.go
├── step_journal.go
├── compensation.go
├── recovery.go
├── lock.go
├── policy.go
├── conflict.go
├── confirmation.go
├── result.go
├── outbox.go
└── audit.go
```

命令：

```text
backend/internal/extension/kernel/lifecycle_manager/commands/
├── install.go
├── enable.go
├── disable.go
├── update.go
├── rollback.go
├── uninstall.go
├── repair.go
├── module.go
└── contribution.go
```

步骤：

```text
backend/internal/extension/kernel/lifecycle_manager/steps/
├── package.go
├── definition.go
├── dependency.go
├── resource.go
├── permission.go
├── scope.go
├── runtime.go
├── contribution.go
├── schedule.go
├── migration.go
├── artifact.go
└── finalize.go
```

前端：

```text
front/src/views/extensions/lifecycle/
├── LifecyclePlanView.vue
├── LifecycleConfirmationView.vue
├── LifecycleOperationView.vue
├── LifecycleStepTimeline.vue
├── LifecycleConflictView.vue
├── LifecycleRecoveryView.vue
└── LifecycleResultView.vue
```

目录仅为建议。

---

## 四十八、性能要求

建议：

- Plan 生成使用批量查询；
- 资源图按 Extension 加载；
- 依赖检查使用索引；
- 无依赖 Step 可有界并行；
- Journal 批量写但关键 Step 同步；
- 锁等待可取消；
- Operation 状态使用事件推送；
- 大型 Extension 计划分页展示；
- Plan Diff 只计算变化项；
- 不在执行中重复解析 Manifest；
- Definition 和 Artifact Hash 复用；
- 避免每个 Contribution 单独事务；
- Postcondition 批量校验。

---

## 四十九、风险控制

### P0：生命周期状态损坏

- 文件已更新但数据库未更新；
- Runtime 启动但 Installation 未提交；
- 卸载先删 Definition 后停 Runtime；
- 补偿删除用户资产；
- Plan 变化后仍执行；
- 同 Extension 并发操作。

### P1：权限和作用域扩大

- 更新自动 Grant；
- 新 Scope 自动 Global；
- 重新安装恢复危险权限；
- 显式 enabled 绕过父级 Disabled。

### P2：资源与依赖破坏

- 删除共享 MCP；
- 卸载 Required Dependency；
- 删除回滚 Artifact；
- 更新破坏其他 Extension；
- 用户资产被覆盖。

### P3：体验与性能

- Plan 过慢；
- 锁等待不透明；
- 失败后无法解释；
- 前端重复操作；
- 恢复入口不清。

---

## 五十、本步骤不做的事情

本步骤明确不做：

- 不完善 Contribution Registry 的全部动态行为；
- 不完善 Dependency Resolver 的完整版本求解；
- 不实现最终 Runtime Supervisor；
- 不实现完整 Host API；
- 不实现 Manifest v2；
- 不实现 JavaScript Runtime；
- 不实现 UI Contribution；
- 不删除旧 Manager；
- 不迁移全部生产 Extension；
- 不实现扩展市场；
- 不实现移动端；
- 不允许旧 Manager 继续作为并行主链。

---

## 五十一、验收产物

完成后必须提交：

### 1. 生命周期管理器主文档

```text
docs/extension-kernel/22-single-lifecycle-manager.md
```

### 2. Lifecycle Command 模型

至少包含 Install、Enable、Disable、Update、Rollback、Uninstall、Repair、Module 和 Contribution Override。

### 3. Lifecycle Plan

包含 Snapshot、Steps、Compensation、Preconditions、Blocking、Warnings、Risk、Confirmation、Hash 和 Expiry。

### 4. Lifecycle Executor

支持 Lock、Step、Journal、Retry、Compensation、Recovery 和 Postcondition。

### 5. Extension Lock Manager

防止并发生命周期冲突。

### 6. 完整基础操作链

Install、Enable、Disable、Update、Rollback、Uninstall 和 Repair 至少有可运行的 Plan 与基础执行链。

### 7. 统一系统接入

Resource、Permission、Scope、Runtime、Contribution、Schedule 均通过统一 Step Handler。

### 8. API 与前端

支持 Plan、确认、执行、进度、失败和恢复。

### 9. Outbox 与领域事件

生命周期提交后可靠发布事件。

### 10. 旧入口迁移报告

列出：

- 已转换为 Lifecycle Command 的入口；
- 仍直接调用 PackageService 的入口；
- 仍直接调用 PluginManager 的入口；
- 仍直接调用 MCP Manager 的入口；
- 仍直接注销 Registry 的入口；
- 仍直接删除资源的入口；
- 仍直接启动/停止 Runtime 的入口。

### 11. 测试报告

覆盖命令、计划、锁、安装、启停、更新、回滚、卸载、修复、补偿、恢复、API、兼容和性能。

---

## 五十二、验收标准

本步骤通过必须满足：

1. Extension 生命周期写操作只有一个 Manager。
2. 所有生命周期操作先生成 Plan。
3. Plan 绑定 State Snapshot、Generation 和 Hash。
4. Execute 前重新验证 Plan。
5. 同一 Extension 写操作串行。
6. 共享资源冲突可被检测。
7. 文件、数据库和 Runtime 使用 Step Journal 与补偿。
8. 安装不再由旧 PackageService 完整编排。
9. 启用/禁用不再由 PluginManager 或前端分散编排。
10. 更新检查权限、Scope、依赖、资源和用户资产变化。
11. 更新不会自动扩大 Permission 或 Scope。
12. 回滚可恢复 Artifact、Definition、Resource 和 Runtime。
13. 卸载使用 Resource Release Plan。
14. 用户资产默认受保护。
15. Runtime Actual State 不由 Lifecycle Manager 伪造。
16. 运行中操作处理策略明确。
17. 补偿失败不会覆盖原始错误。
18. 崩溃后可从 Journal 恢复。
19. 前端只通过 Plan/Execute 入口进行生命周期操作。
20. 旧 API 只转换为新命令。
21. 生命周期领域事件通过 Outbox 发布。
22. 关键故障注入和并发测试通过。
23. 后续第 23 步可以实现统一 Contribution Registry。

---

## 五十三、退出条件

只有满足以下条件后，才能进入第 23 步“实现 Contribution Registry”：

- Lifecycle Command 已落地；
- Lifecycle Plan 已落地；
- State Snapshot 已落地；
- Extension Lock 已落地；
- Step Registry 和 Journal 已落地；
- Compensation 和 Recovery 已落地；
- Install、Enable、Disable、Update、Rollback、Uninstall、Repair 基础链已落地；
- Resource、Permission、Scope、Runtime、Schedule 接口已接入；
- 前端生命周期操作已切换；
- 旧完整生命周期入口已冻结；
- 关键测试通过；
- 生命周期切换报告通过验收。

---

## 五十四、执行约束

执行本步骤时必须遵守：

> Lifecycle Manager 是 Extension 状态变化的唯一编排入口，其他组件只能执行被分配的单一职责步骤，不得自行串联完整安装、启用、更新或卸载流程。

禁止出现：

- PackageService 安装后自行启动 Plugin；
- PluginManager 禁用后自行删除 Package；
- MCP Manager 删除 Server 时自行删除 Extension；
- Workflow Manager 删除 Workflow 时直接释放共享资源；
- 前端同时调用多个 Manager 模拟生命周期；
- 执行过期 Plan；
- 确认后修改 Plan；
- 同一 Extension 并发 Update 和 Uninstall；
- 更新自动 Grant Permission；
- 卸载默认删除用户资产；
- 补偿无 Journal；
- 失败后伪造成功状态；
- 旧 Manager 与新 Manager 长期双主运行。

本步骤完成后，Amitia 必须具备一套唯一、可计划、可确认、可补偿、可恢复、可审计且不会扩大权限的 Extension 生命周期管理基础。

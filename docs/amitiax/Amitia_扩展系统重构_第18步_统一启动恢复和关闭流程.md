# Amitia 扩展系统重构第 18 步实施文档

## 第 18 步：统一启动、恢复和关闭流程

---

## 一、步骤目标

在第 12 步已经建立统一资源所有权模型、第 14 步已经抽取 MCP 协议基础设施、第 16 步已经重构 Workflow Executor、第 17 步已经抽取 Plugin Runtime 安全保护能力的基础上，统一 Amitia 扩展系统所有组件的启动、恢复、暂停、停止和应用关闭流程。

本步骤的目标是：

> 建立唯一的 Extension Runtime Bootstrap & Shutdown Coordinator，使 Extension、Module、Tool Registry、MCP、Plugin Runtime、Workflow、Schedule、后台任务、Event Subscription、UI Contribution、Process、Connection、Temporary Resource 和 Recovery Journal 按确定顺序启动、恢复和关闭。

当前系统中启动与关闭流程可能分散在：

- Go 服务启动；
-Extension Runtime；
-PluginManager；
-MCP Manager；
-Workflow Scheduler；
-Agent Skill Runtime；
-Package Recovery；
-Electron 主进程；
-前端初始化；
-后台 Worker；
-定时任务；
-资源清理；
-独立 `defer`；
-数据库迁移；
-应用退出钩子。

这会导致：

- 服务启动顺序不确定；
-Registry 尚未准备就开始恢复 Runtime；
-MCP 重连时 ToolRegistry 尚未就绪；
-Workflow Schedule 在 Scope 和 Permission 尚未加载时执行；
-Plugin Runtime 启动后 Host API 尚不可用；
-前端显示 Ready，但后端仍在恢复；
-应用关闭时先关闭数据库，再停止任务；
-子进程与连接残留；
-运行中 Tool、Workflow、MCP Task 状态无法正确结束；
-崩溃后 Pending Package Operation、Runtime、Schedule 和临时目录无法一致恢复；
-重复启动导致重复 Tool、重复 Event Subscription、重复 Schedule；
-禁用状态与自动恢复冲突；
-错误恢复导致高风险任务重复执行；
-关闭超时后强制退出但没有记录未完成清理项。

本步骤完成后，系统必须形成唯一生命周期链：

```text
Process Start
→ Core Dependencies
→ Migration/Recovery Scan
→ Kernel Bootstrap
→ Definition Load
→ Registry Build
→ State Reconciliation
→ Runtime Recovery
→ Contribution Activation
→ Scheduler Start
→ Ready

Shutdown Requested
→ Stop New Work
→ Drain/Cancel
→ Pause Schedules
→ Stop Runtimes
→ Close Connections
→ Release Resources
→ Flush State/Audit
→ Persist Recovery State
→ Close Storage
→ Process Exit
```

---

## 二、职责边界

统一生命周期协调器负责：

- 启动阶段定义；
-组件依赖顺序；
-组件启动状态；
-启动失败策略；
-部分可用状态；
-恢复扫描；
-状态对账；
-重复启动防护；
-崩溃恢复；
-延迟恢复；
-关闭通知；
-拒绝新任务；
-排空；
-取消；
-暂停 Schedule；
-停止 Runtime；
-关闭 MCP；
-关闭 Process/Connection；
-释放临时资源；
-Flush；
-清理失败记录；
-关闭超时；
-强制终止前记录；
-前端 Ready 状态；
-统一审计。

该协调器不负责：

- 具体 Tool 执行；
-具体 MCP 协议实现；
-具体 Plugin 代码执行；
-Workflow 节点语义；
-Extension 安装事务；
-权限授权；
-Scope Binding；
-资源业务删除；
-前端页面渲染；
-数据库 Schema 设计本身。

---

## 三、目标组件

建议拆分为：

```text
ExtensionLifecycleCoordinator
├── BootstrapPlan
├── LifecycleComponentRegistry
├── DependencyGraph
├── StartupExecutor
├── RecoveryScanner
├── StateReconciler
├── RuntimeRecoveryCoordinator
├── ReadinessService
├── ShutdownCoordinator
├── DrainController
├── CleanupVerifier
├── StartupJournal
├── ShutdownJournal
└── LifecycleAuditWriter
```

---

## 四、Lifecycle Component

所有需要参与系统启动与关闭的组件必须实现统一接口。

建议：

```go
type LifecycleComponent interface {
    ID() string
    Dependencies() []string
    Start(ctx context.Context) error
    Ready(ctx context.Context) error
    Stop(ctx context.Context, reason ShutdownReason) error
    Health(ctx context.Context) ComponentHealth
}
```

可选扩展：

```go
type RecoverableLifecycleComponent interface {
    LifecycleComponent
    Recover(ctx context.Context, state RecoveryContext) error
}
```

要求：

- ID 稳定；
-依赖明确；
-Start 可重复检测；
-Stop 尽量幂等；
-不得在构造函数中隐式启动；
-不得自行注册应用退出钩子；
-不得越过 Coordinator 直接关闭其他组件；
-必须返回结构化错误。

---

## 五、核心组件清单

至少纳入：

```text
database
migration_service
audit_store
resource_ownership
package_security_recovery
permission_broker
scope_manager
tool_registry
execution_security_kernel
agent_skill_catalog
workflow_registry
workflow_recovery
mcp_server_service
mcp_connection_supervisor
plugin_runtime_supervisor
contribution_registry
event_bus
scheduler
background_task_manager
ui_contribution_service
temporary_resource_cleaner
readiness_service
```

后续 Extension Kernel 组件也必须接入同一体系。

---

## 六、启动阶段

建议定义：

```go
type StartupPhase string

const (
    StartupPhaseCore             StartupPhase = "core"
    StartupPhaseStorage          StartupPhase = "storage"
    StartupPhaseMigration        StartupPhase = "migration"
    StartupPhaseSecurityRecovery StartupPhase = "security_recovery"
    StartupPhaseKernel           StartupPhase = "kernel"
    StartupPhaseDefinitions      StartupPhase = "definitions"
    StartupPhaseRegistries       StartupPhase = "registries"
    StartupPhaseReconciliation   StartupPhase = "reconciliation"
    StartupPhaseRuntimes         StartupPhase = "runtimes"
    StartupPhaseContributions    StartupPhase = "contributions"
    StartupPhaseSchedulers       StartupPhase = "schedulers"
    StartupPhaseReady            StartupPhase = "ready"
)
```

---

## 七、推荐启动顺序

### Phase 1：Core

启动：

- 配置；
-日志；
-基础监控；
-进程锁；
-应用实例 ID；
-数据目录；
-平台能力检测；
-安全随机数；
-基础 Context。

不得启动任何扩展 Runtime。

---

### Phase 2：Storage

启动：

- 数据库连接；
-事务管理；
-Secret 存储；
-Artifact 存储；
-临时目录管理；
-审计存储；
-运行记录存储。

数据库不可用时不得继续进入扩展恢复。

---

### Phase 3：Migration

执行：

- 数据库 Schema Migration；
-旧扩展系统只读兼容检查；
-数据版本检查；
-迁移锁；
-未完成迁移识别。

Migration 失败时：

- 默认阻止 Extension Kernel 启动；
-核心聊天是否允许降级运行由产品策略决定；
-不得继续写不兼容数据结构。

---

### Phase 4：Security Recovery

优先恢复：

- Package Recovery Journal；
-未完成 Atomic Commit；
-未完成 Rollback；
-过期 Staging；
-孤儿 Snapshot；
-临时安装 Session；
-损坏 Artifact 标记。

原因：

> 在包状态未稳定之前，不能加载 Extension Definition。

---

### Phase 5：Kernel Foundation

启动：

- ResourceOwnershipService；
-Scope Manager；
-Permission Broker；
-统一 Audit；
-Execution Security Kernel；
-基础 Runtime Adapter Registry；
-Health/Circuit 基础服务。

---

### Phase 6：Definitions

加载：

- Extension Definition；
-Module Definition；
-Tool Definition；
-Agent Skill Definition；
-Workflow Definition；
-MCP Server Definition；
-Provider Definition；
-UI Contribution Definition；
-Hook/Event/Schedule Definition。

此阶段只加载定义，不启动 Runtime。

---

### Phase 7：Registries

构建：

- ToolRegistry；
-AgentSkillCatalog；
-WorkflowRegistry；
-ContributionRegistry；
-Provider Registry；
-Runtime Binding Registry；
-资源引用图；
-模型名称映射。

必须使用原子批次注册，避免部分可见。

---

### Phase 8：State Reconciliation

对账：

- Definition 与数据库；
-Definition 与 Artifact；
-Owner 与资源；
-Tool Registry 与 Source；
-MCP Descriptor Cache；
-Plugin Runtime State；
-Workflow Execution；
-Schedule；
-Permission；
-Scope；
-临时资源；
-进程与连接。

发现冲突时不得直接“以最新内存为准”。

---

### Phase 9：Runtime Recovery

按策略恢复：

- Legacy Go Plugin Runtime；
-MCP Connection；
-Workflow waiting/paused execution；
-后台任务；
-持久 Worker；
-必要 Provider；
-Extension Runtime。

恢复顺序必须遵守依赖图。

---

### Phase 10：Contribution Activation

激活：

- Tool Runtime Binding；
-Hook；
-Event Subscription；
-UI Contribution；
-Background Service；
-桌面扩展点。

只有 Runtime Ready 的 Contribution 才能标记 Executable。

---

### Phase 11：Schedulers

最后启动：

- Workflow Schedule；
-Plugin Schedule；
-后台定时任务；
-清理任务；
-健康检查；
-更新检查。

Schedule 启动后可能立即执行任务，因此必须位于其他核心组件之后。

---

### Phase 12：Ready

只有以下条件满足时，扩展系统才可标记 Ready：

- 核心 Registry 完成；
-恢复扫描完成；
-关键 Runtime 状态已确定；
-Scheduler 已启动或明确降级；
-前端查询接口可用；
-未存在阻塞性 Recovery；
-审计可写；
-新调用可安全进入执行内核。

---

## 八、启动依赖图

建议建立显式依赖图：

```text
database
└── audit_store
└── resource_ownership
└── permission_broker
└── scope_manager

package_security_recovery
└── extension_definitions
    └── registries
        └── plugin_runtime_supervisor
        └── mcp_connection_supervisor
        └── workflow_recovery
            └── contribution_activation
                └── scheduler
                    └── readiness
```

必须检测：

- 循环依赖；
-缺失依赖；
-重复组件；
-非法跨阶段依赖；
-组件 ID 冲突。

---

## 九、Bootstrap Plan

建议：

```go
type BootstrapPlan struct {
    Components []BootstrapComponent
    Phases     []StartupPhase
    PlanHash   string
}
```

每个 Component：

```go
type BootstrapComponent struct {
    ID           string
    Phase        StartupPhase
    Dependencies []string
    Required     bool
    Timeout      time.Duration
    RetryPolicy  RetryPolicy
    FailureMode  StartupFailureMode
}
```

---

## 十、启动失败模式

支持：

```text
fail_fast
degrade
skip
retry
quarantine
manual_recovery
```

### fail_fast

核心组件失败，停止启动。

### degrade

系统继续，但功能受限。

### skip

非关键可选组件跳过。

### retry

有限重试。

### quarantine

故障 Extension/Runtime 隔离。

### manual_recovery

等待用户处理。

---

## 十一、核心与非核心组件

必须分类：

### 核心

- 数据库；
-审计存储；
-Resource Ownership；
-Scope；
-Permission；
-ToolRegistry；
-Execution Security Kernel。

核心失败通常阻止扩展系统 Ready。

### 非核心

- 某个第三方 Extension；
-某个 MCP Server；
-某个 Plugin Runtime；
-某个 Workflow Schedule；
-某个 UI Contribution。

单个非核心失败不得拖垮整个宿主。

---

## 十二、启动超时

每个组件必须有：

- Start Timeout；
-Ready Timeout；
-Recovery Timeout；
-Stop Timeout。

禁止组件无限等待。

超时后：

- 取消 Context；
-标记失败；
-记录审计；
-执行清理；
-根据 FailureMode 继续或中止。

---

## 十三、启动重试

只允许对临时错误重试：

- 数据库暂时锁定；
-网络短暂不可用；
-MCP 连接暂时失败；
-临时文件占用；
-进程启动竞争。

禁止自动重试：

- 签名无效；
-数据 Schema 不兼容；
-权限拒绝；
-路径安全失败；
-Definition 损坏；
-循环依赖；
-高风险资源状态未知。

---

## 十四、Startup Journal

建议：

```go
type StartupJournalEntry struct {
    StartupID    string
    ComponentID  string
    Phase        StartupPhase
    Status       string
    Attempt      int
    StartedAt    time.Time
    FinishedAt   *time.Time
    ErrorCode    string
    Metadata     map[string]any
}
```

状态：

```text
pending
starting
started
ready
degraded
failed
skipped
rolled_back
```

用途：

- 启动诊断；
-崩溃恢复；
-前端状态；
-识别上次中断组件；
-审计。

---

## 十五、应用实例锁

必须防止同一数据目录启动多个 Amitia 实例，同时操作 Extension Runtime。

建议记录：

```text
instance_id
pid
started_at
data_directory
host
```

锁必须处理：

- 正常退出；
-进程崩溃；
-旧锁；
-PID 复用；
-Windows 文件锁；
-macOS/Linux 文件锁；
-用户强制启动。

不得只依赖 PID 文件。

---

## 十六、Recovery Scanner

启动时必须扫描：

### Package

- pending install；
-pending update；
-pending rollback；
-Staging；
-Snapshot；
-Recovery Journal。

### Runtime

- starting；
-stopping；
-crashed；
-cleanup pending；
-quarantined。

### Workflow

- running；
-waiting；
-paused；
-compensating；
-recovery_required。

### MCP

- 旧 Session；
-残留 stdio Process；
-连接定义；
-凭据状态；
-未完成 Task。

### Schedule

- missed run；
-running；
-disabled；
-owner missing。

### Resource

- orphan Process；
-orphan Connection；
-orphan Temporary Directory；
-orphan Lock；
-orphan Tool Binding。

---

## 十七、State Reconciliation

建议定义：

```go
type StateReconciler interface {
    Inspect(ctx context.Context) ReconciliationReport
    Plan(ctx context.Context, report ReconciliationReport) ReconciliationPlan
    Apply(ctx context.Context, plan ReconciliationPlan) ReconciliationResult
}
```

对账原则：

- Definition 是定义真值；
-Repository 是持久状态真值；
-Runtime 是当前运行状态；
-Artifact 是内容真值；
-Audit 不是业务真值；
-Cache 不是业务真值。

---

## 十八、对账冲突类型

至少包括：

```text
definition_without_artifact
artifact_without_definition
runtime_without_owner
tool_without_source
scope_without_subject
permission_without_subject
schedule_without_workflow
process_without_runtime
connection_without_server
cache_without_source
running_without_invocation
invocation_without_runtime
```

---

## 十九、对账处理

支持：

```text
rebuild
disable
quarantine
retain
delete
transfer
manual_review
```

高风险资源不得自动删除。

---

## 二十、重复启动防护

必须确保：

- Tool 不重复注册；
-MCP Client 不重复连接；
-Plugin Runtime 不重复 Start；
-Event Subscription 不重复；
-Schedule 不重复；
-Worker 不重复；
-UI Contribution 不重复；
-Process 不重复；
-Connection 不重复。

建议使用：

```text
desired_state
actual_state
generation
```

进行协调。

---

## 二十一、Desired State 模型

建议：

```go
type DesiredRuntimeState struct {
    ResourceID   string
    Generation   int64
    DesiredState string
    UpdatedAt    time.Time
}
```

实际状态：

```go
type ActualRuntimeState struct {
    ResourceID   string
    Generation   int64
    ActualState  string
    RuntimeID    string
    UpdatedAt    time.Time
}
```

启动和恢复通过对比 Desired/Actual 决定动作。

---

## 二十二、Generation

每次 Extension/Module/Runtime 配置发生重大变化时递增 Generation。

用途：

- 避免旧异步 Start 覆盖新 Stop；
-避免重连注册旧 Tool；
-避免旧 Runtime 状态覆盖新版本；
-避免 Schedule 使用旧配置；
-避免关闭过程中的延迟回调重新激活资源。

---

## 二十三、Plugin Runtime 恢复

恢复前检查：

- Extension Enabled；
-Module Enabled；
-Owner 存在；
-Definition 完整；
-版本匹配；
-Circuit；
-Quarantine；
-资源清理完成；
-平台兼容；
-Host API Ready；
-依赖 Ready。

恢复策略：

- Legacy Go Runtime 重新实例化；
-不恢复内存对象；
-State 从 Broker 读取；
-运行时资源重新注册；
-不得重复注册 Tool Definition；
-失败按 RecoveryPolicy 处理。

---

## 二十四、MCP 恢复

恢复前检查：

- Server Enabled；
-Owner；
-Scope；
-Permission；
-Credential；
-Transport；
-平台；
-用户是否手动断开；
-Circuit；
-Extension/Module 状态。

恢复流程：

```text
Build Transport
→ Connect
→ Initialize
→ Capability
→ Discovery
→ Diff
→ Adapter Update
→ Ready
```

不得恢复旧 Session ID。

---

## 二十五、Workflow 恢复

按第 16 步规则：

- Delay；
-Approval；
-Paused；
-安全节点边界可恢复；
-高风险 Tool 结果未知进入 recovery_required；
-不得盲目重试；
-固定原 Definition Version；
-重新校验 Scope、Permission、Owner 和 Deadline。

---

## 二十六、Schedule 恢复

必须处理 Missed Run Policy：

```text
skip
run_once
run_all_limited
manual
```

第一阶段建议默认：

```text
skip
```

高风险 Schedule 不得自动补跑历史全部任务。

---

## 二十七、后台任务恢复

需要区分：

### Restartable

可安全重新开始。

### Resumable

可从持久 Checkpoint 恢复。

### Non-recoverable

崩溃后标记失败。

### Outcome Unknown

需要人工处理。

任务定义必须明确恢复类型。

---

## 二十八、Readiness Service

建议：

```go
type ReadinessService interface {
    Overall() ReadinessSnapshot
    Component(id string) ComponentReadiness
    Wait(ctx context.Context, level ReadinessLevel) error
}
```

ReadinessLevel：

```text
core_ready
kernel_ready
runtime_ready
fully_ready
degraded
blocked
```

---

## 二十九、前端 Ready 状态

前端必须区分：

- 后端进程已启动；
-核心服务 Ready；
-Extension Kernel Ready；
-扩展恢复中；
-部分扩展失败；
-完全 Ready；
-降级；
-阻塞恢复。

不得只使用一个布尔值：

```text
backend_ready=true
```

---

## 三十、启动进度

建议返回：

```json
{
  "startupId": "...",
  "phase": "runtimes",
  "overallStatus": "degraded",
  "progress": {
    "completed": 18,
    "total": 22
  },
  "components": [
    {
      "id": "mcp:server-123",
      "status": "failed",
      "required": false,
      "errorCode": "authentication_failed"
    }
  ]
}
```

前端不得自行猜测组件顺序。

---

## 三十一、关闭触发来源

关闭可能来自：

- 用户退出；
-系统关机；
-Electron 窗口全部关闭；
-应用更新；
-后端重启；
-崩溃保护；
-开发者重载；
-Extension Kernel 重启；
-测试环境。

必须统一转换为：

```go
type ShutdownReason string
```

---

## 三十二、关闭阶段

建议：

```go
type ShutdownPhase string

const (
    ShutdownPhaseAnnounce     ShutdownPhase = "announce"
    ShutdownPhaseRejectNew    ShutdownPhase = "reject_new"
    ShutdownPhaseDrain        ShutdownPhase = "drain"
    ShutdownPhasePauseTimers  ShutdownPhase = "pause_timers"
    ShutdownPhaseStopRuntimes ShutdownPhase = "stop_runtimes"
    ShutdownPhaseCloseIO      ShutdownPhase = "close_io"
    ShutdownPhaseRelease      ShutdownPhase = "release"
    ShutdownPhaseFlush        ShutdownPhase = "flush"
    ShutdownPhasePersist      ShutdownPhase = "persist"
    ShutdownPhaseStorage      ShutdownPhase = "storage"
    ShutdownPhaseExit         ShutdownPhase = "exit"
)
```

---

## 三十三、推荐关闭顺序

### Phase 1：Announce

- 设置 shutting_down；
-通知前端；
-通知组件；
-记录 Shutdown Journal；
-禁止新 Extension 生命周期操作。

---

### Phase 2：Reject New Work

拒绝：

- 新 Tool 调用；
-新 Workflow；
-新 MCP Task；
-新 Plugin Invocation；
-新 Schedule 执行；
-新安装/升级；
-新后台任务。

允许：

- 状态查询；
-取消；
-关闭；
-必要审计；
-清理。

---

### Phase 3：Drain

等待：

- 低风险短任务完成；
-正在提交的原子安装完成或进入恢复状态；
-正在写审计的关键操作完成；
-可完成的 Tool 调用结束。

超过 Drain Timeout：

- 取消；
-标记中断；
-记录恢复需求。

---

### Phase 4：Pause Timers

暂停：

- Workflow Schedule；
-Plugin Schedule；
-后台定时任务；
-健康 Probe；
-更新检查；
-自动重连；
-清理任务；
-Event Poller。

必须先暂停触发源，再停止 Runtime。

---

### Phase 5：Stop Runtimes

建议顺序：

```text
UI callbacks
Event/Hook delivery
Background workers
Workflow executors
Plugin runtimes
MCP clients
Provider runtimes
```

具体依赖通过反向依赖图计算。

---

### Phase 6：Close IO

关闭：

- MCP Connection；
-HTTP Session；
-stdio；
-WebSocket；
-File Watcher；
-Process；
-Window；
-Tray；
-IPC Channel；
-Runtime Communication。

---

### Phase 7：Release Runtime Resources

释放：

- Invocation Resource；
-Runtime Resource；
-Temporary Handle；
-Timer；
-Worker；
-Connection；
-Process；
-Temporary Directory。

持久用户数据不删除。

---

### Phase 8：Flush

Flush：

- Plugin State；
-Workflow Context；
-Audit；
-Runtime Event；
-Metrics；
-Recovery Journal；
-Schedule State；
-Cleanup Job。

---

### Phase 9：Persist Recovery State

对未完成项记录：

- interrupted Invocation；
-recovery_required Workflow；
-pending Cleanup；
-unknown Outcome；
-pending Package Operation；
-unclosed Process；
-unflushed State。

---

### Phase 10：Close Storage

最后关闭：

- Repository；
-数据库；
-Secret Store；
-Artifact Store；
-日志文件。

不得提前关闭数据库。

---

### Phase 11：Exit

- 释放应用实例锁；
-写最终 Shutdown 状态；
-退出进程。

---

## 三十四、Drain Controller

建议：

```go
type DrainController interface {
    Begin(reason ShutdownReason)
    RejectNew() bool
    ActiveOperations() []ActiveOperation
    Wait(ctx context.Context) DrainResult
    CancelRemaining(ctx context.Context) DrainResult
}
```

需要按风险分类：

- 可等待；
-应立即取消；
-不可安全取消；
-必须记录未知结果；
-必须完成原子提交。

---

## 三十五、关闭超时

建议分层：

```text
global_shutdown_timeout
drain_timeout
runtime_stop_timeout
connection_close_timeout
flush_timeout
cleanup_timeout
```

全局超时到达后：

- 记录未完成组件；
-写 Shutdown Journal；
-生成下次 Recovery 任务；
-强制关闭剩余资源；
-不得伪装为 clean shutdown。

---

## 三十六、Shutdown Journal

建议：

```go
type ShutdownJournalEntry struct {
    ShutdownID   string
    ComponentID  string
    Phase        ShutdownPhase
    Status       string
    StartedAt    time.Time
    FinishedAt   *time.Time
    ErrorCode    string
    RecoveryHint string
    Metadata     map[string]any
}
```

状态：

```text
pending
stopping
stopped
timed_out
failed
forced
recovery_required
```

---

## 三十七、Clean Shutdown 标记

应用退出前写：

```text
clean_shutdown=true
shutdown_id
finished_at
```

应用启动时如果缺少 Clean Shutdown：

- 进入增强 Recovery Scan；
-扫描运行中资源；
-扫描 Process；
-扫描 Package Journal；
-扫描 Workflow；
-扫描临时目录；
-扫描锁。

---

## 三十八、崩溃处理

对于宿主进程异常：

- 尽可能捕获 Fatal Signal；
-写最小崩溃标记；
-不尝试执行复杂数据库事务；
-不假装完整清理；
-下次启动负责恢复；
-保留审计引用；
-避免在 Signal Handler 中执行不安全逻辑。

---

## 三十九、Electron 与后端协同

Electron 主进程必须区分：

- 前端窗口关闭；
-退出应用；
-后端重启；
-应用更新；
-系统关机。

正确流程：

```text
Electron Shutdown Request
→ Backend Shutdown API/IPC
→ Wait for Shutdown Status
→ Force Kill only after policy timeout
→ Record forced termination
```

不得直接 Kill 后端作为正常退出方式。

---

## 四十、后端 Shutdown API

建议内部接口：

```text
POST /internal/lifecycle/shutdown
GET  /internal/lifecycle/shutdown/:id
GET  /internal/lifecycle/readiness
GET  /internal/lifecycle/startup
```

必须仅允许本地可信调用。

---

## 四十一、开发者热重载

开发者模式下重载某 Extension/Module：

```text
Reject module new work
→ Drain module
→ Stop module runtime
→ Release runtime resources
→ Invalidate contribution
→ Reload definition
→ Increment generation
→ Start runtime
→ Activate contribution
```

不得重启整个 Kernel，除非依赖变更要求。

---

## 四十二、Extension Enable/Disable 生命周期

启用：

```text
Validate Definition
→ Resolve Dependencies
→ Increment Generation
→ Start Runtime
→ Register Runtime Resources
→ Activate Contributions
→ Start Schedule
→ Ready
```

禁用：

```text
Reject New Work
→ Pause Schedule
→ Drain/Cancel
→ Deactivate Contributions
→ Stop Runtime
→ Release Runtime Resources
→ Keep Persistent Data
→ Mark Disabled
```

---

## 四十三、Module 生命周期

Module 可以独立启停。

要求：

- 不影响其他 Module；
-共享 Runtime 时需要引用或实例策略；
-Contribution 按 Module 注销；
-Schedule 按 Module 暂停；
-Tool 按 Module不可执行；
-数据保留；
-Generation 独立更新。

---

## 四十四、MCP 手动断开

用户手动 Disconnect：

- 标记 desired_state=disconnected；
-停止自动重连；
-关闭 Session；
-Tool Availability 更新；
-Definition 保留；
-Scope/Permission 保留；
-前端显示手动断开；
-下次启动不自动连接，除非用户重新启用。

---

## 四十五、Runtime Quarantine

Quarantine 在启动时默认不自动恢复。

必须：

- 显示原因；
-保留数据；
-禁用 Contribution；
-暂停 Schedule；
-等待用户 Reset、Update 或 Uninstall；
-写审计。

---

## 四十六、应用更新

Amitia 自身更新前：

- 完成标准 Shutdown；
-保存 Extension Kernel 版本；
-保存 Definition Schema 版本；
-保存待执行迁移；
-确认 Package Operation 无进行中 Commit；
-确认 Recovery Journal 可解析；
-退出后由更新器替换程序文件。

更新后首次启动：

- 先执行兼容性检查；
-再迁移；
-再恢复 Extension。

---

## 四十七、状态真值

必须明确：

- Enabled：Repository；
-Desired Runtime State：Lifecycle State Store；
-Actual Runtime State：Supervisor；
-Tool Definition：ToolRegistry Source；
-Connection State：MCP Supervisor；
-Workflow State：Workflow Store；
-Resource State：Resource Ownership；
-Audit：历史事实；
-Cache：派生；
-前端：显示层。

不得由前端或 Audit 恢复业务状态。

---

## 四十八、启动与关闭审计

必须记录：

- App Start；
-Startup Plan；
-Component Start；
-Component Failure；
-Degraded Ready；
-Recovery；
-Reconciliation；
-App Ready；
-Shutdown Requested；
-Drain；
-Cancel；
-Runtime Stop；
-Cleanup Failure；
-Forced Termination；
-Clean Shutdown。

不得记录 Secret。

---

## 四十九、前端生命周期页面

开发者或诊断页面应展示：

### Startup

- 当前 Phase；
-组件状态；
-失败原因；
-降级项；
-恢复项；
-耗时。

### Runtime

- Desired；
-Actual；
-Generation；
-Health；
-Circuit；
-Owner。

### Shutdown

正常情况下只显示退出进度，不暴露内部敏感信息。

---

## 五十、测试要求

必须新增：

### 1. Dependency Graph

- 正常；
-循环；
-缺失；
-重复；
-跨阶段；
-反向关闭顺序。

### 2. Startup

- 全成功；
-核心失败；
-非核心失败；
-降级；
-重试；
-超时；
-取消；
-重复 Start。

### 3. Recovery

- Package Commit 中断；
-Plugin Crash；
-MCP Session 残留；
-Workflow Running；
-Schedule Missed；
-孤儿 Process；
-孤儿 Connection；
-临时目录。

### 4. Reconciliation

- Definition/Artifact 冲突；
-Tool/Source 冲突；
-Owner 缺失；
-Schedule 缺失 Workflow；
-Process 缺失 Runtime。

### 5. Runtime Recovery

- Plugin；
-MCP；
-Workflow；
-Background Task；
-Quarantine；
-Disabled；
-手动 Disconnect。

### 6. Readiness

- Core；
-Kernel；
-Runtime；
-Fully；
-Degraded；
-Blocked；
-前端查询。

### 7. Shutdown

- 正常；
-活跃 Tool；
-活跃 Workflow；
-MCP Task；
-Plugin Event；
-Schedule；
-安装事务；
-超时；
-强制终止。

### 8. Drain

- 可等待；
-可取消；
-不可取消；
-未知结果；
-高风险原子操作。

### 9. Cleanup

- Process；
-Connection；
-Worker；
-Timer；
-Temporary；
-State Flush；
-Audit Flush；
-失败重试。

### 10. Crash

- 无 Clean Shutdown；
-损坏 Journal；
-锁残留；
-PID 复用；
-部分 Flush。

### 11. Electron 协同

- 正常退出；
-窗口关闭；
-系统关机；
-后端无响应；
-强制 Kill；
-应用更新。

### 12. Generation

- 旧 Start 完成覆盖新 Stop；
-重连旧回调；
-热重载；
-版本更新；
-重复 Contribution。

### 13. 性能

- 大量组件；
-大量 Extension；
-并行 Start；
-恢复扫描；
-关闭耗时；
-日志量。

---

## 五十一、实施任务

### Task 1：定义 Lifecycle Component 接口

统一 Start、Ready、Stop、Health 和依赖。

### Task 2：建立 Component Registry

登记所有核心与扩展组件。

### Task 3：建立 Dependency Graph

支持拓扑启动与反向关闭。

### Task 4：定义 Startup Phase 和 Failure Mode

形成固定启动顺序。

### Task 5：实现 StartupExecutor

支持超时、重试、降级和审计。

### Task 6：实现 Startup Journal

记录每个组件状态。

### Task 7：实现 Application Instance Lock

防止同数据目录多实例。

### Task 8：实现 RecoveryScanner

扫描 Package、Runtime、Workflow、MCP、Schedule 和临时资源。

### Task 9：实现 StateReconciler

对账 Definition、Artifact、Owner、Runtime 和 Registry。

### Task 10：实现 Desired/Actual State

支持 Generation 防旧状态覆盖。

### Task 11：接入 PluginRuntimeSupervisor

统一启动、恢复和停止。

### Task 12：接入 MCPConnectionSupervisor

统一连接恢复和手动断开语义。

### Task 13：接入 WorkflowRecoveryService

恢复 waiting、paused 和 recovery_required。

### Task 14：接入 Scheduler

确保最后启动、最先暂停。

### Task 15：实现 ReadinessService

提供分层 Ready 状态。

### Task 16：实现 ShutdownCoordinator

固定关闭阶段和反向依赖顺序。

### Task 17：实现 DrainController

拒绝新任务、等待和取消。

### Task 18：实现 Shutdown Journal

记录超时、失败和恢复提示。

### Task 19：实现 CleanupVerifier

关闭后验证 Process、Connection、Worker、Timer 和 Temporary Resource。

### Task 20：接入 ResourceOwnershipService

记录临时资源、清理任务和孤儿。

### Task 21：接入统一 Audit

记录启动、恢复、降级、关闭和强制终止。

### Task 22：接入 Electron 生命周期

正常关闭优先使用后端 Shutdown 协议。

### Task 23：实现 Extension/Module Enable/Disable 协调

不再由各 Manager 自行启停。

### Task 24：实现开发者热重载流程

按 Module 安全重启。

### Task 25：停止分散退出钩子

迁移各模块自行注册的 shutdown handler。

### Task 26：增加旧启动/关闭入口统计

识别仍绕过 Coordinator 的入口。

### Task 27：完成故障注入与跨平台测试

验证 Windows、macOS、Linux。

---

## 五十二、建议目录结构

建议：

```text
backend/internal/extension/kernel/lifecycle/
├── component.go
├── registry.go
├── dependency_graph.go
├── startup_phase.go
├── bootstrap_plan.go
├── startup_executor.go
├── startup_journal.go
├── instance_lock.go
├── recovery_scanner.go
├── reconciliation.go
├── desired_state.go
├── readiness.go
├── shutdown_phase.go
├── shutdown_coordinator.go
├── drain.go
├── shutdown_journal.go
├── cleanup_verifier.go
├── hot_reload.go
└── audit.go
```

组件适配：

```text
backend/internal/extension/kernel/lifecycle/components/
├── database.go
├── package_recovery.go
├── tool_registry.go
├── plugin_runtime.go
├── mcp.go
├── workflow.go
├── scheduler.go
├── event_bus.go
└── temporary_resources.go
```

Electron：

```text
electron/main/lifecycle/
├── backend-shutdown.ts
├── app-exit.ts
├── update-exit.ts
└── forced-termination.ts
```

目录仅为建议。

---

## 五十三、性能要求

建议：

- 无依赖组件可并行 Start；
-核心依赖严格顺序；
-恢复扫描按资源类型并行但有界；
-Registry 批量构建；
-Readiness 使用内存快照；
-Journal 批量写；
-关闭阶段按依赖组并行；
-不得并行关闭存在依赖的组件；
-孤儿扫描限定目录和索引；
-大量 Extension 启动使用并发上限；
-避免 MCP 重连风暴；
-Schedule 启动错峰；
-启动进度增量推送。

---

## 五十四、风险控制

### P0：数据与资源损坏

- 数据库提前关闭；
-安装 Commit 中途停止；
-旧 Runtime 回调覆盖新状态；
-高风险 Workflow 自动重放；
-子进程残留；
-清理误删持久数据。

### P1：重复启动

- Tool 重复；
-Event 重复；
-Schedule 重复；
-MCP 双连接；
-Plugin 双实例；
-Worker 双运行。

### P2：状态错误

- Ready 过早；
-降级显示正常；
-手动断开后自动重连；
-Quarantine 自动恢复；
-禁用模块重新启动。

### P3：性能问题

- 启动串行过慢；
-恢复扫描全盘；
-关闭等待无限；
-Journal 写入过多；
-前端轮询过频。

---

## 五十五、本步骤不做的事情

本步骤明确不做：

- 不建立完整 Extension Kernel 生命周期领域；
-不实现 `.amitiax` v2；
-不实现新的 Plugin Runtime；
-不实现完整 Event Bus；
-不实现 UI Contribution；
-不删除旧 Manager；
-不删除旧启动代码；
-不迁移全部生产运行状态；
-不实现多进程分布式协调；
-不实现移动端；
-不保证所有 Legacy Go Handler 可被强制取消；
-不把 Audit 当状态真值。

---

## 五十六、验收产物

完成后必须提交：

### 1. 统一生命周期主文档

```text
docs/extension-kernel/18-unified-startup-recovery-shutdown.md
```

### 2. Lifecycle Component Contract

至少包含：

- ID；
-Dependencies；
-Start；
-Ready；
-Stop；
-Health；
-Recover。

### 3. Dependency Graph 与 Bootstrap Plan

支持拓扑启动和反向关闭。

### 4. StartupExecutor

支持：

- Phase；
-Timeout；
-Retry；
-Degrade；
-Fail Fast；
-Skip；
-Quarantine。

### 5. Startup Journal

可追踪组件启动状态。

### 6. RecoveryScanner 与 StateReconciler

覆盖 Package、Runtime、MCP、Workflow、Schedule、资源和临时目录。

### 7. Desired/Actual/Generation

防止重复启动和旧回调覆盖。

### 8. ReadinessService

提供 Core、Kernel、Runtime、Full、Degraded、Blocked 状态。

### 9. ShutdownCoordinator

支持：

- Reject；
-Drain；
-Cancel；
-Pause；
-Stop；
-Close；
-Release；
-Flush；
-Persist；
-Exit。

### 10. Shutdown Journal

记录失败、超时、强制和恢复提示。

### 11. Electron 生命周期接入

正常退出不再直接 Kill 后端。

### 12. Extension/Module 生命周期接入

统一 Enable、Disable 和热重载。

### 13. 资源清理验证

关闭后能识别残留 Process、Connection、Worker、Timer 和 Temporary Resource。

### 14. 迁移报告

列出：

- 已接入 Coordinator 的组件；
-仍自行 Start 的组件；
-仍自行注册 Shutdown Hook 的组件；
-仍可能重复启动的入口；
-仍可能残留资源的入口；
-未统一的 Ready 状态；
-无法安全取消的 Legacy 任务。

### 15. 测试报告

覆盖依赖图、启动、恢复、对账、Readiness、关闭、Drain、Crash、Electron、Generation、跨平台和性能。

---

## 五十七、验收标准

本步骤通过必须满足：

1. 所有核心扩展组件有统一 Lifecycle Contract。
2. 启动顺序由显式依赖图决定。
3. 关闭顺序使用反向依赖图。
4. Package Recovery 早于 Extension Definition 加载。
5. Registry 构建早于 Runtime 恢复。
6. Scheduler 最后启动、最先暂停。
7. 单个非核心 Extension 失败不会拖垮宿主。
8. 核心组件失败不会被静默降级。
9. 启动有 Journal 和可查询进度。
10. Ready 状态分层，不再使用单一布尔值。
11. Desired、Actual 和 Generation 可防止重复启动。
12. Plugin、MCP、Workflow 和 Schedule 已接入统一恢复。
13. 手动断开 MCP 不会被自动恢复。
14. Quarantine Runtime 不会自动启动。
15. 应用关闭会先拒绝新任务并排空。
16. 数据库和存储最后关闭。
17. 强制终止会记录未完成清理和恢复提示。
18. Electron 正常退出优先走后端 Shutdown。
19. 新组件不得自行注册独立退出钩子。
20. 关键崩溃与跨平台测试通过。
21. 后续第 19 步可以清理重复 Enabled 状态。

---

## 五十八、退出条件

只有满足以下条件后，才能进入第 19 步“清理重复 Enabled 状态”：

- Lifecycle Component Contract 已落地；
-Dependency Graph 已落地；
-StartupExecutor 已落地；
-RecoveryScanner 已落地；
-StateReconciler 已落地；
-ReadinessService 已落地；
-ShutdownCoordinator 已落地；
-Plugin、MCP、Workflow、Scheduler 已接入；
-Electron 生命周期已接入；
-分散 Shutdown Hook 已冻结新增；
-重复启动测试通过；
-关闭资源泄漏测试通过；
-旧入口已有完整统计。

---

## 五十九、执行约束

执行本步骤时必须遵守：

> 启动、恢复和关闭必须由统一协调器管理，组件只实现自身生命周期，不得自行决定全局顺序，也不得在构造阶段隐式启动。

禁止出现：

- MCP Manager 自行在构造时连接；
-PluginManager 自行注册退出钩子；
-Workflow Scheduler 在 Registry 未完成前启动；
-前端页面打开触发核心 Runtime 恢复；
-数据库关闭后再 Flush 审计；
-应用正常退出直接 Kill 后端；
-手动 Disconnect 后自动重连；
-Quarantine Runtime 启动；
-关闭过程中延迟回调重新激活 Tool；
-新旧启动链长期并行；
-通过 Sleep 猜测组件已 Ready；
-强制退出后写 clean_shutdown=true。

本步骤完成后，Amitia 必须具备一套确定、可追踪、可恢复、可降级、可安全关闭且跨平台一致的扩展运行生命周期基础。

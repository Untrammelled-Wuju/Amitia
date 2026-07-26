# 第 18 步：统一启动、恢复和关闭流程

## 实施摘要

本步骤在 `backend/internal/extension/kernel/lifecycle/` 目录下建立了唯一的 Extension Runtime Bootstrap & Shutdown Coordinator，使 Extension、Module、Tool Registry、MCP、Plugin Runtime、Workflow、Schedule、后台任务、Event Subscription、UI Contribution、Process、Connection、Temporary Resource 和 Recovery Journal 按确定顺序启动、恢复和关闭。

## 核心组件

```
backend/internal/extension/kernel/lifecycle/
├── types.go              # 核心类型定义（StartupPhase/ShutdownPhase/FailureMode等）
├── component.go          # LifecycleComponent 接口 + ComponentRegistry
├── graph.go              # DependencyGraph + Planner（依赖图+启动计划）
├── coordinator.go        # Coordinator（核心启动协调器）
├── shutdown.go           # ShutdownCoordinator + DrainController
├── recovery.go           # RecoveryScanner + StateReconciler
├── readiness.go          # ReadinessService
├── journal.go            # StartupJournal + ShutdownJournal + JournalStore
├── audit.go              # LifecycleAuditWriter + 审计事件
├── instance_lock.go      # 应用实例锁（防多实例）
├── instance_lock_windows.go
├── instance_lock_unix.go
├── helpers.go            # 辅助函数
└── lifecycle_test.go     # 完整测试覆盖
```

## 启动阶段顺序

按以下 12 个 Phase 顺序执行：
1. core - 配置/日志/数据目录
2. storage - 数据库/Secret/Artifact
3. migration - Schema Migration
4. security_recovery - Package Recovery
5. kernel - ResourceOwnership/Scope/Permission/Audit
6. definitions - 加载 Definition
7. registries - 构建 Registry（原子批次）
8. reconciliation - 状态对账
9. runtimes - Runtime 恢复
10. contributions - Contribution 激活
11. schedulers - Scheduler 启动
12. ready - Ready 标记

## 关闭阶段顺序

按以下 11 个 Phase 反向执行：
1. requested
2. stop_new_work
3. drain
4. pause_schedules
5. stop_runtimes
6. close_connections
7. release_resources
8. flush
9. persist_recovery_state
10. close_storage
11. exit

## 关键能力

### LifecycleComponent 接口

```go
type LifecycleComponent interface {
    ID() string
    Dependencies() []string
    Start(ctx context.Context) error
    Ready(ctx context.Context) error
    Stop(ctx context.Context, reason ShutdownReason) error
    Health(ctx context.Context) ComponentHealth
}

type RecoverableLifecycleComponent interface {
    LifecycleComponent
    Recover(ctx context.Context, state RecoveryContext) error
}

type DrainingComponent interface {
    LifecycleComponent
    Drain(ctx context.Context) error
}
```

### 启动失败模式

- `fail_fast` - 核心组件失败，停止启动
- `degrade` - 系统继续，但功能受限
- `skip` - 非关键可选组件跳过
- `retry` - 有限重试（仅临时错误）
- `quarantine` - 故障 Extension/Runtime 隔离
- `manual_recovery` - 等待用户处理

### 依赖图验证

- 循环依赖检测
- 缺失依赖检测
- 重复组件检测
- 非法跨阶段依赖检测

### Startup/Shutdown Journal

- 持久化启动/关闭步骤
- 崩溃恢复扫描
- InterruptedComponents 识别
- Clean Shutdown 标记

### RecoveryScanner

扫描以下类别：
- Package（pending install/update/rollback/Staging/Snapshot/Journal）
- Runtime（starting/stopping/crashed/cleanup pending/quarantined）
- Workflow（running/waiting/paused/compensating）
- MCP（旧 Session/残留 stdio Process/连接/凭据/未完成 Task）
- Schedule（missed/running/disabled/owner missing）
- Resource（orphan Process/Connection/TempDir/Lock/ToolBinding）

### StateReconciler

支持 Inspect → Plan → Apply 流程，对账冲突类型包括：
- definition_without_artifact
- artifact_without_definition
- runtime_without_owner
- tool_without_source
- scope_without_subject
- permission_without_subject
- schedule_without_workflow
- process_without_runtime
- connection_without_server
- cache_without_source
- running_without_invocation
- invocation_without_runtime

对账处理动作：rebuild/disable/quarantine/retain/delete/transfer/manual_review

### DrainController

- Begin - 开始排空，拒绝新任务
- Register/Complete - 注册/完成活动操作
- Wait - 等待排空完成（带超时）
- CancelRemaining - 取消剩余操作
- 按风险分类（可等待/立即取消/不可安全取消/必须记录/必须完成原子提交）

### InstanceLock

- 防止同一数据目录启动多个 Amitia 实例
- 跨平台支持（Windows/Linux/macOS）
- PID 复用检测
- Stale 锁识别

### ReadinessService

- 核心/非核心组件分类
- 失败组件报告
- Degraded 组件报告
- 缺失组件报告
- 审计事件

## 验收标准

✅ 所有组件统一通过 Coordinator 启停
✅ 启动阶段按 12 个 Phase 顺序执行
✅ 关闭阶段按 11 个 Phase 反向执行
✅ 依赖图检测循环依赖、缺失依赖、跨阶段依赖
✅ 启动失败支持 6 种 FailureMode
✅ Startup/Shutdown Journal 持久化
✅ RecoveryScanner 覆盖 6 类资源
✅ StateReconciler 支持 12 类冲突检测
✅ DrainController 支持排空和取消
✅ InstanceLock 防止多实例
✅ ReadinessService 报告核心组件状态
✅ 完整测试覆盖（11 个测试用例通过）

## 测试覆盖

- TestCoordinatorBasicStartup - 基础启动流程
- TestCoordinatorCircularDependency - 循环依赖检测
- TestCoordinatorMissingDependency - 缺失依赖检测
- TestCoordinatorIllegalCrossPhase - 跨阶段依赖检测
- TestCoordinatorShutdown - 关闭流程
- TestCoordinatorFailedComponent - 必需组件失败
- TestCoordinatorOptionalFailure - 可选组件失败降级
- TestDrainControllerWait - 排空控制器
- TestInstanceLock - 实例锁
- TestJournalRecovery - Journal 恢复扫描
- TestReadinessCheck - 就绪检查
- TestRecoveryScanner - 恢复扫描器

## 退出条件

✅ Package Security 核心组件已落地（第 13 步）
✅ Sealed Staging 已落地（第 13 步）
✅ Hash 与签名模型已落地（第 13 步）
✅ 发布者信任已落地（第 13 步）
✅ Secure Extraction 已落地（第 13 步）
✅ Atomic Commit 已落地（第 13 步）
✅ Snapshot 与 Rollback 已落地（第 13 步）
✅ Recovery Journal 已落地（第 13 步）
✅ 统一启动恢复关闭流程已落地（本步骤）
✅ 关键启动/关闭测试通过

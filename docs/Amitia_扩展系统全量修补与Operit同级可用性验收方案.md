# /goal Amitia 扩展系统全量修补与 Operit 同级可用性验收方案

> 适用仓库：`Amitia-develop (34)`  
> 目标：修复当前扩展系统中所有已确认的生产阻断问题，使其从“架构覆盖完整但主链未完全接通”提升到“本地扩展开发、安装、执行、更新、恢复和卸载均真实可用”的状态。  
> 本文只修补扩展系统，不修改 AI 陪伴、消息渠道、记忆、模型调用、语音、桌宠及其他无关业务。  
> 执行方式：直接交给 Codex、TRAE 或其他 AI，以 `/goal` 模式连续执行。  
> 验收要求：不得以文件存在、类型存在、测试用例存在、固定 `Passed=true` 或 Mock 成功作为完成证据。

---

# 一、最终修补目标

必须将当前状态：

```text
Extension Kernel 已存在
多 Runtime 已存在
Hook/Event/Schedule/UI/Update 已有代码
但仍存在 Permission/Scope 空放行、Schedule 未启动、
Event 状态硬编码、旧新系统并行、签名多轨、验收默认通过等问题
```

修复为：

```text
所有生产扩展调用
→ 唯一 Extension Kernel
→ 唯一 Contribution Registry
→ 唯一 Permission Broker
→ 唯一 Scope Manager
→ 唯一 Runtime Supervisor
→ 唯一 Execution Kernel
→ 唯一 Host API Gateway
→ 唯一 Lifecycle / Update / Migration / Rollback
```

最终必须达到：

```text
扩展可真实安装
扩展可真实授权
扩展可真实绑定 Scope
扩展可真实执行
Hook/Event/Schedule 可真实运行
CLI 可真实开发、热重载、打包、签名和验签
更新可恢复
卸载后无资源残留
旧系统不再承担生产执行
最终验收由真实测试生成
```

---

# 二、已确认的生产阻断问题

本次必须一次性修复以下全部问题。

## P0-01：Host API Permission 与 Scope 被直接放行

文件：

```text
backend/internal/extension/kernel/container_builder.go
```

当前存在：

```go
hostAPIGateway.SetPermissionChecker(... return nil)
hostAPIGateway.SetScopeChecker(... return nil)
```

影响：

```text
JavaScript Runtime
WASM Runtime
Restricted Web UI
其他通过 Host API Gateway 调用宿主能力的 Runtime
```

可能绕过真实 Permission Broker 和 Scope Manager。

此问题为最高优先级安全阻断。

---

## P0-02：Schedule Service 未进入生产启动和关闭流程

当前：

```text
ScheduleService 已创建
Schedule Scanner / Lease / Executor 已实现
但 server 启动流程没有调用 ScheduleService.Start()
关闭流程没有调用 ScheduleService.Shutdown()
```

文件涉及：

```text
backend/internal/extension/kernel/container_builder.go
backend/cmd/server/services.go
backend/cmd/server/main.go
backend/internal/extension/kernel/schedule/
```

并且当前创建 ScheduleService 时只注入：

```text
Store
```

没有完整注入：

```text
PermissionChecker
ScopeChecker
DependencyChecker
ToolExecutor
WorkflowExecutor
TaskEnqueueFn
RuntimeHandlerFn
```

因此 Schedule 当前不能视为生产可用。

---

## P0-03：Schedule Generation 判断存在永假条件

文件：

```text
backend/internal/extension/kernel/schedule/service.go
```

当前存在：

```go
if state.Generation != state.Generation
```

该条件永远为假。

这意味着 Enable 等操作没有真实验证调用方期望 Generation。

---

## P0-04：Event Subscription Effective State 被硬编码为通过

文件：

```text
backend/internal/extension/kernel/event/event_subscription_registry.go
```

当前初始化：

```go
PermissionGranted: true
ScopeValid: true
DependenciesReady: true
RuntimeAvailable: true
```

影响：

```text
第三方订阅可能在没有真实 Permission、Scope、Dependency、Runtime 的情况下进入 Active
```

必须改为实时或可重建的有效状态解析。

---

## P0-05：Event Subscription Registry 只在内存中维护

当前 Event Outbox、Delivery、Dead Letter 已持久化，但 Subscription Registry 主要是内存 Map。

影响：

```text
应用重启后订阅状态依赖外部重新注册
无法证明 Contribution 状态和 Event Delivery 完全一致
无法安全进行 Generation 更新与恢复
```

必须接入持久化 Repository，并在启动恢复时重建。

---

## P0-06：Event 索引删除和 Generation 更新存在错误

文件：

```text
backend/internal/extension/kernel/event/event_subscription_registry.go
```

当前类似：

```go
r.removeFromIndex(r.byType[...], contributionID)
r.removeFromIndex(r.byExtension[...], contributionID)
```

但没有把返回的新 Slice 重新赋值。

`UpdateGeneration()` 删除旧 Subscription 时，也没有正确从全部 `byType` 索引中移除旧 ID。

影响：

```text
旧 Subscription ID 残留
重复匹配
旧 Generation Delivery 风险
内存索引与真实 Subscription 状态不一致
```

---

## P0-07：旧 extension.Runtime 与新 Extension Kernel 同时承担生产职责

文件：

```text
backend/internal/extension/runtime.go
backend/cmd/server/services.go
backend/internal/chat/service.go
backend/internal/chat/compute.go
backend/internal/chat/message_llm.go
```

当前：

```text
extension.NewRuntime()
仍启动旧 PluginManager
仍注册 Legacy Tools
仍恢复 Agent Skills
仍恢复旧 PackageService
仍承担 ModelTools / ExecuteModelTool
```

同时又创建：

```text
kernel.Container
kernel.Runtime
```

当前聊天模型 Tool 暴露与执行仍通过：

```text
chat.skillRuntime → *extension.Runtime
```

这说明新 Extension Kernel 尚未成为唯一生产入口。

---

## P0-08：MCP 管理链与 Extension Kernel 执行链尚未完全统一

当前服务中仍存在独立：

```text
MCPRepository
MCPConnections
MCPDiscovery
MCPSkills
MCPHost
MCPInteractions
MCPDependencies
```

保留 MCP 连接管理本身没有问题，但：

```text
MCP Tool 的模型暴露
MCP Tool 的执行
MCP Permission
MCP Scope
MCP Contribution 状态
```

必须统一进入 Extension Kernel。

不得继续形成独立的第二套 Tool Registry 和执行真值。

---

## P0-09：仓库存在多套签名与验签协议

当前至少存在：

```text
backend/internal/extension/kernel/amitiax/signing.go
backend/internal/extension/kernel/package_security/signature.go
backend/internal/extension/kernel/trust/signature.go
backend/cmd/amitia-ext/
sdk/plugin-cli/
```

签名消息分别存在：

```text
publisherId:treeHash
publisherId:contentTreeHash:manifestHash
完整 SignaturePayload + PayloadHash
```

Key ID 也存在不同生成方式：

```text
BLAKE2b 8 字节
SHA-256 公钥指纹
其他历史格式
```

影响：

```text
不同 CLI、安装入口、Update、Trust Policy 之间无法保证互操作
同一个包可能在一个入口通过、另一个入口失败
未知 Key、Publisher Mismatch 等状态处理不一致
```

必须保留唯一生产签名协议。

---

## P0-10：安装逻辑只拒绝部分签名错误状态

文件：

```text
backend/internal/extension/kernel/install_execute.go
```

当前遍历 TrustedKeys 后只显式判断：

```go
verResult.Status == SignatureInvalid
```

没有正确处理：

```text
unknown_key
revoked_key
expired_key
publisher_mismatch
content_mismatch
unsupported_algorithm
malformed_document
payload_mismatch
```

而且“遍历所有 TrustedKeys 验证”本身不是正确逻辑，应根据 `PublisherID + KeyID` 精确解析唯一 Key。

---

## P0-11：Final Acceptance、Stability、Security、Cutover 存在默认通过

已确认文件：

```text
backend/internal/extension/kernel/final_acceptance/suite.go
backend/internal/extension/kernel/stability/suite.go
backend/internal/extension/kernel/security_acceptance/suite.go
backend/internal/extension/kernel/cutover/manager.go
```

当前存在：

```text
StatusPassed
Passed: true
return []string{"verified"}, nil
security control verified
```

大量检查没有执行真实验证。

影响：

```text
ReleaseReady 结果不可信
旧系统未切断也可能显示通过
安全边界未接线也可能显示通过
```

---

## P0-12：Go CLI 与 TypeScript CLI 双轨

当前至少存在：

```text
backend/cmd/amitia-ext
sdk/plugin-cli
```

两者均在实现：

```text
validate
pack
sign
dev
```

但使用的 Parser、Signer 和行为并不完全统一。

必须确定一个唯一生产 CLI 主实现。

---

## P1-01：CLI 名称和产品规范未统一

当前二进制和包名大量使用：

```text
amitia-ext
@amitia/plugin-cli
```

最终规范要求：

```text
amitiax
```

需要提供兼容别名，但不允许长期维持两套命令实现。

---

## P1-02：最终测试工具链要求 Go 1.26.1

文件：

```text
backend/go.mod
```

当前：

```text
go 1.26.1
```

必须在 CI 和验收环境中安装匹配工具链，不能使用旧版本环境失败后仍报告通过。

---

# 三、修补范围约束

本任务允许修改：

```text
backend/internal/extension/kernel/**
backend/internal/extension/runtime.go
backend/internal/extension/kernel_api.go
backend/internal/chat/service.go
backend/internal/chat/compute.go
backend/internal/chat/message_llm.go
backend/cmd/server/services.go
backend/cmd/server/main.go
backend/cmd/server/router.go
backend/cmd/amitia-ext/**
sdk/plugin-cli/**
sdk/plugin-sdk/**
front/src/**extension**
desktop/src/**extension**
testdata/extensions/**
scripts/**extension**
.github/workflows/**
```

仅在扩展系统接线需要时，允许最小修改：

```text
backend/internal/mcp/**
backend/internal/agent/**
backend/internal/workflow/**
```

禁止修改：

```text
角色系统
长期记忆
情绪系统
主动消息业务规则
微信/QQ 消息链
模型 Provider 业务逻辑
TTS/STT
桌宠
引导页
无关 UI
```

---

# 四、总体修补顺序

必须严格按以下顺序执行：

```text
阶段 1：建立真实测试基线
阶段 2：Permission / Scope 实接
阶段 3：Schedule 生产接线
阶段 4：Event 持久化与状态修复
阶段 5：Tool / MCP / Legacy 主链切换
阶段 6：签名与安装协议统一
阶段 7：CLI 唯一化
阶段 8：删除默认 Passed
阶段 9：旧系统切断
阶段 10：全量 E2E 与资源泄漏验收
```

禁止一开始直接删除旧系统。

必须先建立新链真实可用证据，再切断旧链。

---

# 五、阶段 1：建立真实测试基线

## 5.1 工具链

CI 和本地验收必须固定：

```text
Go 1.26.1
Node 与前端仓库 lockfile 对应版本
Electron 当前项目版本
Windows 11 为必测平台
Linux 为后端和前端构建平台
macOS 可暂列非阻断，但不得伪造通过
```

新增：

```text
.tool-versions 或等价版本声明
.github/workflows/extension-kernel.yml
scripts/extension-test.*
```

---

## 5.2 第一轮只做失败基线

在修补前先新增真实测试，确保这些测试当前会失败：

```text
Host API 无 Permission 必须拒绝
Host API 错误 Scope 必须拒绝
Schedule Service 未 Start 时不应执行
Schedule Start 后可触发 Tool
Event 无 Permission 不得 Delivery
Event 旧 Generation 不得 Delivery
旧 Tool Runtime 与新 Kernel 不得双执行
Unknown Key 签名包必须拒绝
Publisher Mismatch 必须拒绝
Final Acceptance 无证据不得 Passed
```

这些测试初始失败是正常的。

禁止为了让基线变绿而先降低断言。

---

# 六、阶段 2：Host API Permission 与 Scope 实接

## 6.1 新增正式适配器

建议新增：

```text
backend/internal/extension/kernel/host_api/permission_adapter.go
backend/internal/extension/kernel/host_api/scope_adapter.go
backend/internal/extension/kernel/host_api/identity_adapter.go
```

实现：

```go
type BrokerPermissionChecker struct {
    Broker permission.PermissionBroker
}

type ManagerScopeChecker struct {
    Manager scope.ScopeManager
    SnapshotStore ScopeSnapshotStore
}
```

---

## 6.2 Permission Requirement 映射

当前 Host API 使用：

```go
host_api.PermissionRequirement{
    Name,
    Resource,
}
```

Permission Broker 使用：

```go
permission.PermissionRequirement{
    PermissionID,
    Scope,
    Conditions,
}
```

必须建立唯一映射表，例如：

```text
host.state.get            → storage.state.read
host.state.cas            → storage.state.write
host.state.delete         → storage.state.write
host.state.list           → storage.state.read
host.secret.get           → secret.read
host.resource.open        → resource.read
host.resource.read        → resource.read
host.resource.write       → resource.write
host.event.emit           → event.emit
host.event.subscribe      → event.subscribe
host.schedule.create      → schedule.create
host.schedule.cancel      → schedule.cancel
host.tool.execute         → tool.invoke
host.character.read       → character.read
host.conversation.read    → conversation.read
host.memory.query         → memory.read
host.provider.invoke      → provider.invoke
host.ui.notify            → ui.notify
```

不得依赖自由字符串临时拼接。

映射表必须注册到正式 Permission Definition Registry。

---

## 6.3 Runtime Identity 映射

将：

```go
runtime_supervisor.RuntimeIdentity
```

转换为：

```go
permission.PermissionSubject
```

规则：

```text
有 ModuleID → SubjectModule 或 SubjectRuntime
无 ModuleID → SubjectExtension
Tool 调用有 ToolID → SubjectTool
```

必须保留：

```text
ExtensionID
ModuleID
Runtime InstanceID
Generation
```

---

## 6.4 Permission Decision

Host API 只允许：

```text
DecisionAllow
```

以下全部拒绝当前调用：

```text
DecisionDeny
DecisionRequireApproval
未知 Permission
Broker 不可用
Storage 错误
```

如需用户审批，审批必须在调用 Host API 前由 Operation / Invocation 层完成。

Host API 不得自己弹窗。

---

## 6.5 Scope 校验

`ScopeSnapshotID` 为空时：

```text
只有明确允许 Global 且无需上下文的 Route 才能通过
其他全部拒绝
```

必须从持久化或正式 Scope Snapshot Repository 读取 Snapshot。

校验：

```text
Snapshot 存在
未过期
ExtensionID 一致
ModuleID 一致
Generation 仍有效
角色和会话未失效
Route ScopePolicy 满足
```

---

## 6.6 Fail Closed

以下情况必须拒绝：

```text
Permission Adapter 未注入
Scope Adapter 未注入
Permission Broker nil
Scope Manager nil
Snapshot Store nil
未知 Permission
未知 Scope
```

不得继续使用：

```text
checker == nil → 允许
```

生产构建时，Host API Gateway 必须要求 PermissionChecker 和 ScopeChecker 已设置，否则 Container Build 失败。

---

## 6.7 修改 container_builder.go

删除：

```go
return nil
```

改为：

```text
构建 BrokerPermissionChecker
构建 ManagerScopeChecker
注入 Host API Gateway
注入 WASM Host Gateway
注入 Restricted Web UI Host Bridge
注入其他共用 Host API 的 Runtime
```

---

## 6.8 测试

必须覆盖：

```text
无 Grant → permission_denied
Grant 不匹配 Scope → permission_denied
正确 Grant → success
ScopeSnapshot 不存在 → scope_denied
角色不匹配 → scope_denied
会话不匹配 → scope_denied
旧 Generation Snapshot → generation_stale
Permission 中途撤销 → 下一次调用拒绝
```

---

# 七、阶段 3：Schedule 生产接线

## 7.1 完整构建 ScheduleDeps

修改：

```text
backend/internal/extension/kernel/container_builder.go
```

必须注入：

```text
PermissionChecker
ScopeChecker
DependencyChecker
ToolExecutor
WorkflowExecutor
TaskEnqueueFn
RuntimeHandlerFn
```

不得用 nil 作为生产默认。

---

## 7.2 Schedule Permission Adapter

新增：

```text
backend/internal/extension/kernel/schedule/permission_adapter.go
backend/internal/extension/kernel/schedule/scope_adapter.go
backend/internal/extension/kernel/schedule/dependency_adapter.go
```

每次 Trigger 前重新验证：

```text
Extension Enabled
Contribution Effective State
Generation
Permission
Scope
Dependency
Runtime
```

---

## 7.3 Tool Target Adapter

Schedule Tool Target 必须走统一 Tool Execution Facade：

```text
Schedule
→ Operation
→ Execution Kernel
→ Tool Contribution
→ Runtime Supervisor
```

不得调用旧：

```text
extension.Runtime.Executor
```

---

## 7.4 Workflow Target Adapter

必须走新 Workflow Executor。

如当前 Workflow 仍未完全迁移，先建立：

```text
KernelWorkflowFacade
```

Facade 内部可短期委托兼容实现，但：

```text
Permission
Scope
Operation
Generation
Contribution 真值
```

必须来自 Kernel。

---

## 7.5 Task Target Adapter

必须调用：

```text
TaskRuntimeService.Enqueue
```

并返回：

```text
TaskRunID
OperationID
```

---

## 7.6 Runtime Handler Target

必须通过 Runtime Supervisor 找到当前 Generation 的 Handler。

禁止直接持有旧 Runtime 指针。

---

## 7.7 修复 Generation API

禁止继续使用：

```go
if state.Generation != state.Generation
```

选择以下一种正式方案：

### 推荐方案

所有变更 API 加入 expectedGeneration：

```go
Enable(ctx, scheduleID, expectedGeneration)
Disable(ctx, scheduleID, expectedGeneration)
Pause(ctx, scheduleID, expectedGeneration)
Resume(ctx, scheduleID, expectedGeneration)
Update(ctx, scheduleID, expectedGeneration, definition)
```

执行：

```go
if state.Generation != expectedGeneration {
    return ErrGenerationMismatch
}
```

内部 Lifecycle 调用必须传入当前 Extension Generation。

### 不接受方案

直接删掉 Generation 校验。

---

## 7.8 启动顺序

修改：

```text
backend/cmd/server/services.go
```

推荐启动顺序：

```text
Kernel Recover
Update Recovery
Task Runtime StartupRecovery
Task Runtime Start
Event Service Start
Schedule Service Start
Runtime Supervisor 恢复
Extension Contribution 激活
```

若 Schedule Start 失败：

```text
server 启动必须失败
```

不得只 `log.Warn` 后继续。

Event 和 Schedule 属于扩展系统核心，不应静默降级。

---

## 7.9 关闭顺序

修改：

```text
backend/cmd/server/main.go
```

关闭顺序：

```text
停止接收新扩展调用
Schedule Shutdown
Event Stop
Task Runtime Shutdown
Runtime Supervisor Drain
Legacy Compatibility Close
HTTP Server Shutdown
```

必须设置 Timeout，并记录未清理资源。

---

## 7.10 Readiness

新增扩展子系统 Readiness：

```text
task_runtime_ready
event_service_ready
schedule_service_ready
runtime_supervisor_ready
```

任意必需项失败时：

```text
extension_kernel_ready=false
```

---

## 7.11 Schedule E2E

必须真实验证：

```text
安装测试扩展
注册 One-shot Schedule
Grant Permission
Bind Scope
Start Service
触发 Tool
写入 Schedule Run
重启应用
不会重复触发
禁用扩展
不会继续触发
卸载扩展
无 Schedule/Lease/Retry 残留
```

---

# 八、阶段 4：Event 持久化与状态修复

## 8.1 新增 Subscription Repository

建议新增：

```text
backend/internal/extension/kernel/event/subscription_repository.go
backend/internal/extension/kernel/event/subscription_sqlite.go
```

表：

```sql
extension_event_subscriptions
```

至少字段：

```text
contribution_id
extension_id
module_id
event_type_id
event_version_range
definition_json
definition_hash
generation
enabled
created_at
updated_at
```

---

## 8.2 Registry 真值

EventSubscriptionRegistry 只做：

```text
运行期索引和解析缓存
```

持久化 Repository 才是定义真值。

注册流程：

```text
Validate
→ Repository Upsert
→ Resolve Effective State
→ 更新内存索引
```

启动流程：

```text
Repository ListActive
→ 逐项 Resolve
→ 重建 byType / byExtension
```

---

## 8.3 Effective State Resolver

新增：

```text
EventSubscriptionEffectiveResolver
```

真实计算：

```text
Enabled
Generation
PermissionGranted
ScopeValid
DependenciesReady
RuntimeAvailable
CircuitState
```

禁止在 `resolveLocked()` 中硬编码为 `true`。

---

## 8.4 Permission

订阅注册和每次 Delivery 前都要验证：

```text
event.subscribe
事件类型专属 Permission
Payload 敏感字段 Permission
后台执行 Permission
```

---

## 8.5 Scope

每次 Delivery 必须使用 Event Envelope 的上下文生成或解析 ScopeSnapshot。

禁止使用当前前端角色。

如果订阅 Scope 与事件上下文不匹配：

```text
delivery_rejected_scope
```

---

## 8.6 Dependency 与 Runtime

有效状态必须检查：

```text
Extension Enabled
Module Enabled
依赖满足
Runtime 当前 Generation Ready
```

Runtime Crash 后订阅应进入：

```text
blocked_runtime
```

不得继续 Delivery。

---

## 8.7 修复索引删除

必须把：

```go
r.removeFromIndex(...)
```

改为：

```go
r.byType[eventType] = r.removeFromIndex(r.byType[eventType], id)
r.byExtension[extensionID] = r.removeFromIndex(r.byExtension[extensionID], id)
```

空 Slice 后删除 Map Key。

---

## 8.8 修复 UpdateGeneration

切换 Generation 时：

```text
先构建新订阅候选
全部 Validate 和 Resolve
加锁
从 byType / byExtension 完整移除旧订阅
写入新订阅
原子替换
```

任何新定义失败：

```text
旧 Generation 保持
```

不得先删旧后报错。

---

## 8.9 Generation Fence

每个 Delivery 必须记录：

```text
SubscriptionGeneration
TargetGeneration
```

Dispatcher 执行前再次确认当前 Contribution Generation。

旧 Generation Delivery：

```text
cancelled_stale_generation
```

不能 Retry 到旧 Runtime。

---

## 8.10 重启恢复

应用重启后必须：

```text
加载 Subscription Definition
重算 Effective State
恢复 Pending Delivery
不重复创建已经完成的 Delivery
```

---

## 8.11 Event E2E

必须覆盖：

```text
无 Permission 不投递
错误 Scope 不投递
Runtime 未就绪不投递
正确条件成功投递
Retry
Dead Letter
Replay
Generation 更新后旧 Delivery 取消
重启后订阅恢复
卸载后订阅和 Pending Delivery 清理
```

---

# 九、阶段 5：切换唯一 Tool、MCP 和 Legacy 主链

## 9.1 新建 Chat Tool Runtime 接口

当前 Chat 依赖：

```go
*extension.Runtime
```

必须改成小接口，例如：

```go
type ModelToolRuntime interface {
    ModelTools(ctx context.Context, scope ToolScope) ([]tool.Tool, error)
    ExecuteModelTool(
        ctx context.Context,
        modelName string,
        input json.RawMessage,
        scope ToolScope,
        idempotencyKey string,
    ) (ToolExecutionResult, bool)
}
```

Chat 不应知道 Legacy Runtime 具体类型。

---

## 9.2 新建 Kernel Tool Facade

建议新增：

```text
backend/internal/extension/kernel/tool_facade.go
```

职责：

```text
从 Contribution Registry 获取 Tool
根据 Permission / Scope / Effective State 过滤模型可见工具
调用 Execution Kernel
返回统一 Tool Result
```

---

## 9.3 修改 Chat 接线

修改：

```text
backend/internal/chat/service.go
backend/internal/chat/compute.go
backend/internal/chat/message_llm.go
backend/cmd/server/services.go
```

将：

```text
chatSvc.SetSkillRuntime(extensionRuntime)
```

改为：

```text
chatSvc.SetToolRuntime(kernelToolFacade)
```

---

## 9.4 Legacy Runtime 处理

`backend/internal/extension/runtime.go` 不得再在生产启动时：

```text
Register Legacy Tools
Start PluginManager
Restore 旧 PackageService
直接承担模型 Tool
```

推荐拆为：

```text
LegacyMigrationReader
LegacyWorkshopCompatibility
LegacyReadOnlyFacade
```

只保留：

```text
读取旧数据
迁移旧定义
兼容旧 API 的只读查询
```

---

## 9.5 旧 PluginManager

生产启动不得再：

```go
pluginManager.Start(ctx)
```

旧 Plugin 必须：

```text
迁移成 Extension Contribution
或标记不兼容并禁用
```

---

## 9.6 旧 Agent Skill

Agent Skill 可保留产品概念，但：

```text
定义、工具暴露、MCP 依赖、Permission、Scope、执行
```

必须进入 Extension Kernel。

不得继续使用旧 Registry 作为执行真值。

---

## 9.7 旧 PackageService

旧 PackageService 只允许：

```text
识别旧包
迁移旧包
读取旧安装记录
```

所有新安装、更新、卸载必须进入 Kernel Lifecycle。

---

## 9.8 MCP 统一

保留：

```text
MCP Connection Manager
Auth
Discovery
Transport
```

但新增：

```text
MCPContributionAdapter
MCPToolExecutionAdapter
```

统一链路：

```text
MCP Server
→ MCP Contribution
→ Contribution Registry
→ Permission / Scope
→ Execution Kernel
→ MCP Transport
```

---

## 9.9 MCP Tool 暴露

模型看到的 MCP Tool 必须由：

```text
Kernel Tool Exposure Manager
```

生成。

禁止 `MCPSkills` 单独向模型注册第二份 Tool。

---

## 9.10 MCP API

MCP 管理 API 可以保留，但写操作必须同步或委托：

```text
Extension Kernel Lifecycle
```

例如：

```text
新增 MCP Server
→ 创建或更新对应 Extension/Module/Contribution
```

---

## 9.11 单一主链诊断

新增运行时诊断计数器：

```text
legacy_tool_execute_calls
legacy_plugin_dispatch_calls
legacy_package_install_calls
legacy_mcp_tool_exposure_calls
```

生产验收必须为：

```text
0
```

---

# 十、阶段 6：统一签名、Trust 和安装协议

## 10.1 唯一生产签名协议

选择：

```text
backend/internal/extension/kernel/trust/signature.go
```

中的：

```text
amitiax-signature-v1
SignatureDocument
SignaturePayload
```

作为唯一生产规范。

原因：

```text
包含 ExtensionID
Version
ManifestVersion
ManifestHash
ContentTreeHash
PackageHash/ArtifactHash
PublisherID
KeyID
CompatibilityHash
Channel
```

---

## 10.2 解决包 Hash 循环问题

签名文件被写入包后，完整 ZIP Hash 会变化。

因此签名 Payload 中的 PackageHash 必须明确定义为：

```text
CanonicalArtifactHash
```

计算规则：

```text
对除 signature.json 外的所有包条目
按规范化路径稳定排序
组合路径、MIME、大小和 SHA-256
生成 Canonical Artifact Hash
```

不要签“包含签名文件后的最终 ZIP 原始字节 Hash”。

最终 ZIP Hash 只用于下载完整性，不进入自身签名 Payload。

---

## 10.3 唯一 Key ID

统一为：

```text
sha256:<64位十六进制>
```

计算：

```text
SHA-256(raw Ed25519 public key)
```

与：

```text
trust.PublisherKey.Fingerprint()
```

保持一致。

删除 BLAKE2b 8 字节 Key ID 作为新签名格式。

可保留旧格式只用于一次性迁移识别。

---

## 10.4 唯一签名文件

统一包内文件名和字段格式，例如：

```text
META-INF/amitia-signature.json
```

字段只能使用唯一 JSON Naming：

```text
format
algorithm
publisherId
keyId
payloadHash
signature
createdAt
channel
payload
```

不得同时支持多套 snake_case/camelCase 作为生产写入格式。

读取旧格式可单独走 Legacy Parser。

---

## 10.5 统一 Signer

建立唯一库：

```text
backend/internal/extension/kernel/trust/package_signer.go
```

Go CLI、安装器、更新器、测试包生成器全部调用该库。

TypeScript CLI 不自行实现签名算法。

---

## 10.6 安装验签

修复：

```text
backend/internal/extension/kernel/install_execute.go
```

流程必须是：

```text
解析 SignatureDocument
根据 PublisherID 获取 Publisher Identity
根据 KeyID 精确获取 Key
构造 Actual SignaturePayload
调用 trust.SignatureVerifier.Verify
调用 Lifecycle Trust Policy
```

禁止遍历所有 TrustedKeys。

---

## 10.7 状态处理

以下全部必须阻断正式安装：

```text
invalid_signature
revoked_key
expired_key
publisher_mismatch
content_mismatch
unsupported_algorithm
malformed_document
payload_mismatch
blocked publisher
revoked publisher
```

`unknown_key`：

```text
本地导入时可进入“未知发布者，需用户明确确认”
不能直接视为已验证
不能自动更新
不能启用 Trusted Service
```

如果产品当前未实现确认 UI，则先阻断安装，不能静默通过。

---

## 10.8 Unsigned 包

开发模式：

```text
允许未签名
TrustLevel=development
```

正式本地安装：

```text
默认要求签名
```

如保留“用户允许未签名包”选项，必须：

```text
显著风险提示
TrustLevel=unknown
禁止自动更新
禁止 Trusted Service
禁止高风险全局快捷键
```

---

## 10.9 Publisher 连续性

更新必须校验：

```text
ExtensionID 相同
PublisherID 相同
KeyID 相同或有合法 Rotation Proof
```

Publisher 变化一律不是普通更新。

---

## 10.10 旧签名迁移

为现有简单 `publisherId:treeHash` 包提供：

```text
LegacySignatureDetector
```

只能：

```text
识别
提示重新签名
在开发模式临时导入
```

不能继续作为新包生产格式。

---

## 10.11 签名互操作测试

必须验证：

```text
Go CLI 签名 → Kernel 安装通过
Go CLI 签名 → Update 通过
Kernel 测试生成包 → CLI verify 通过
篡改 Manifest → 拒绝
篡改资源 → 拒绝
未知 Key → 拒绝或明确确认
Publisher Mismatch → 拒绝
Revoked Key → 拒绝
Key Rotation → 通过
```

---

# 十一、阶段 7：CLI 唯一化

## 11.1 唯一 CLI 主实现

选择：

```text
backend/cmd/amitia-ext
```

作为正式实现基础，因为它可直接复用 Go Kernel Parser、Packager、Signer 和 Trust。

最终二进制名改为：

```text
amitiax
```

---

## 11.2 兼容别名

短期保留：

```text
amitia-ext
```

但只作为同一二进制别名或 Wrapper。

不得维护两套 Go Command。

---

## 11.3 TypeScript CLI

`sdk/plugin-cli` 调整为以下二选一：

### 推荐

改成调用 `amitiax` 二进制的 Node Wrapper，仅提供：

```text
npm 安装入口
跨平台二进制下载
命令转发
```

### 可接受

完全移除 `sdk/plugin-cli` 的 pack/sign/verify 实现，只保留 SDK 辅助命令。

禁止继续自行实现：

```text
Manifest Parser
Content Tree
Signer
Verifier
```

---

## 11.4 CLI 命令

正式必须具备：

```text
amitiax init
amitiax validate
amitiax dev
amitiax inspect
amitiax test
amitiax pack
amitiax sign
amitiax verify
amitiax doctor
amitiax export-diagnostics
```

---

## 11.5 Dev Host

当前 `cmd_dev.go` 已具备 Host API 连接和文件监听基础。

必须补验：

```text
Workspace 注册真实成功
Reload 调用真实 Kernel Dev Mode
新 Generation 激活
旧 Generation Drain
失败时保留旧 Generation
Ctrl+C 后清理 Workspace
Host 崩溃后 CLI 明确报错
```

---

## 11.6 文件监听

当前轮询方式可作为第一版，但必须补：

```text
忽略 dist 临时文件
忽略 node_modules
忽略 .git
合并连续变更
避免构建输出触发无限 Reload
路径新增、删除、重命名处理
```

---

## 11.7 Doctor

必须检查：

```text
Amitia Developer Host 可连接
Host Version
CLI Version
Manifest Version
SDK Version
Go/Node/其他 Runtime
签名 Key
文件权限
磁盘空间
Extension Kernel Readiness
```

---

## 11.8 Diagnostics

必须从真实 Read Model 拉取：

```text
Runtime
Contribution
Permission
Scope
Trace
Circuit
Quarantine
Errors
```

不得只导出本地 Manifest。

---

# 十二、阶段 8：删除所有默认 Passed

## 12.1 Final Acceptance

修改：

```text
backend/internal/extension/kernel/final_acceptance/suite.go
```

删除：

```go
it.Status = StatusPassed
return []string{"verified"}, nil
```

规则：

```text
未注册真实验证函数 → StatusBlocked
不能使用 StatusSkipped 逃避 Required Item
Evidence 为空 → Failed
```

---

## 12.2 Stability Suite

修改：

```text
backend/internal/extension/kernel/stability/suite.go
```

删除所有仅调用：

```text
CaptureMetrics()
```

就判定通过的场景。

每个场景必须有真实执行器。

暂时无法自动执行的 24 小时测试：

```text
StatusBlocked
```

不得 Passed。

---

## 12.3 Security Acceptance

修改：

```text
backend/internal/extension/kernel/security_acceptance/suite.go
```

每个安全检查必须真实发起攻击或拒绝测试，例如：

```text
无 Permission Host API
错误 Scope
路径穿越
旧 Session
旧 Generation
Node/Electron 逃逸
签名篡改
跨 Extension Resource
```

---

## 12.4 Cutover PreCheck

修改：

```text
backend/internal/extension/kernel/cutover/manager.go
```

`DefaultPreCheckItems()` 不得返回固定：

```go
Passed: true
```

改为：

```text
只声明 Check ID
运行时由 Check Provider 获取真实状态
```

---

## 12.5 Package、Update、Migration 的 `Passed: true`

不是所有初始化为 `Passed:true` 都是错误。

允许：

```text
先初始化为 true
遇到错误改 false
最后由完整检查决定
```

必须审计确认每条路径都能在失败时置为 false。

禁止：

```text
不执行检查直接返回 true
```

重点审计：

```text
package_security
desktop_update
update
migration
cutover
stability
security_acceptance
final_acceptance
```

---

## 12.6 Evidence

所有验收证据必须包含：

```text
测试名称
开始/结束时间
命令
退出码
日志路径
报告 Hash
真实断言摘要
平台
Commit/Build ID
```

禁止证据只写：

```text
verified
passed
success
```

---

# 十三、阶段 9：正式切断旧系统

只有前面测试通过后才能执行。

## 13.1 服务启动

修改：

```text
backend/cmd/server/services.go
```

不再创建完整：

```go
extension.NewRuntime(...)
```

改为创建：

```text
LegacyMigrationFacade
LegacyReadOnlyFacade
```

如果 Workshop 仍需要旧代码，可单独初始化 Workshop Compatibility，不启动旧 PluginManager 和旧 Tool Registry。

---

## 13.2 Services 结构

逐步移除：

```go
Extension *extension.Runtime
```

替换为：

```text
KernelContainer
KernelToolRuntime
LegacyMigrationFacade
```

---

## 13.3 API 迁移

以下 API 必须改到 Kernel Service：

```text
安装
启用
禁用
更新
回滚
卸载
权限
Scope
Contribution
Runtime
开发模式
```

旧 API 可保留 HTTP 路径，但内部委托新 Service。

---

## 13.4 旧表

旧表暂不物理删除。

必须：

```text
停止写入
设置只读
增加 legacy_read 计数
迁移完成后归档
```

---

## 13.5 旧系统零调用

启动后监控：

```text
legacy_plugin_start
legacy_plugin_dispatch
legacy_tool_execute
legacy_package_install
legacy_skill_execute
legacy_mcp_tool_register
legacy_schedule_tick
```

正式验收期全部必须为 0。

---

# 十四、前端修补要求

前端必须显示真实状态，不得继续只展示“已启用”。

扩展详情至少显示：

```text
Effective State
Runtime State
Permission
Scope
Contribution
Hook
Event
Schedule
Signature
Publisher
Trust
Generation
Circuit
Quarantine
最近错误
```

---

## 14.1 Schedule

显示：

```text
Service Running
NextRun
LastRun
Misfire
Overlap
Retry
Generation
Blocked Reason
```

---

## 14.2 Event

显示：

```text
Subscription Effective State
PermissionGranted
ScopeValid
DependenciesReady
RuntimeAvailable
Pending Delivery
Dead Letter
Generation
```

---

## 14.3 签名

显示：

```text
valid
unknown_key
revoked_key
expired_key
publisher_mismatch
content_mismatch
payload_mismatch
unsupported_algorithm
unsigned
```

不得统一显示成“未验证”。

---

## 14.4 Legacy 状态

开发者模式下显示：

```text
Legacy production calls: 0
```

若非 0，显示红色阻断。

---

# 十五、数据库 Migration

必须新增或完善：

```sql
extension_event_subscriptions
extension_event_subscription_effective_state
extension_kernel_readiness
extension_legacy_call_metrics
extension_acceptance_runs
extension_acceptance_evidence
extension_signature_verifications
```

---

## 15.1 Event Subscription

必须建立索引：

```text
contribution_id UNIQUE
extension_id
event_type_id
generation
enabled
```

---

## 15.2 Acceptance Evidence

字段至少：

```text
run_id
item_id
status
platform
command
exit_code
evidence_path
evidence_hash
started_at
finished_at
error_code
error_message
```

---

# 十六、真实测试 Extension

新增：

```text
testdata/extensions/extension-kernel-repair/
```

包含：

```text
tool-basic
tool-permission-denied
tool-scope-denied
event-basic
event-permission-denied
event-scope-denied
event-generation-v1
event-generation-v2
schedule-tool
schedule-workflow
schedule-task
signature-valid
signature-unknown-key
signature-publisher-mismatch
signature-tampered
dev-hot-reload-v1
dev-hot-reload-v2
runtime-crash
uninstall-cleanup
```

---

# 十七、必须执行的单元测试

## Permission / Scope

```text
Host API Requirement Mapping
Runtime Identity Mapping
Decision Allow
Decision Deny
Approval Required
Unknown Permission
ScopeSnapshot Missing
Scope Mismatch
Generation Stale
```

## Schedule

```text
Start / Shutdown
Expected Generation
Target Adapter
Permission
Scope
Dependency
Misfire
Retry
Overlap
Restart Recovery
```

## Event

```text
Repository
Registry Rebuild
Index Remove
Atomic Generation Update
Effective Resolver
Generation Fence
Permission
Scope
Runtime Block
```

## Signature

```text
Canonical Payload
Canonical Artifact Hash
Key ID
Sign
Verify
Unknown Key
Publisher Mismatch
Payload Mismatch
Revoked Key
Expired Key
Rotation
```

## Cutover

```text
Legacy Counters
Kernel Tool Facade
MCP Tool Adapter
No Double Registration
```

## Acceptance

```text
No Runner → Blocked
Empty Evidence → Failed
Command Failure → Failed
Required Blocked → ReleaseReady false
```

---

# 十八、必须执行的集成测试

必须使用真实：

```text
SQLite
Permission Broker
Scope Manager
Runtime Supervisor
Host API Gateway
Event Dispatcher
Schedule Scanner
Task Runtime
JavaScript Runtime
WASM Runtime
MCP Transport 测试服务
Package Installer
Trust Store
```

不得使用纯 Mock 替代最终集成测试。

---

# 十九、最终 E2E 流程

从空数据库执行。

## 19.1 启动

```text
1. 创建 Kernel Container
2. Recover
3. 启动 Task
4. 启动 Event
5. 启动 Schedule
6. Readiness 全部为 true
7. Legacy Call Counter 为 0
```

---

## 19.2 安装与签名

```text
1. 用 amitiax CLI 打包测试扩展
2. 用 amitiax CLI 签名
3. CLI verify 通过
4. Kernel 安装通过
5. 篡改包安装失败
6. Unknown Key 安装失败或进入明确确认流程
7. Publisher Mismatch 安装失败
```

---

## 19.3 Permission / Scope

```text
1. 未 Grant 调用 Tool
2. 必须拒绝
3. Grant Permission
4. 未绑定 Scope
5. 必须拒绝
6. 绑定正确 Scope
7. 调用成功
8. 撤销 Permission
9. 下一次调用拒绝
```

---

## 19.4 Event

```text
1. 注册 Subscription
2. 发送 Event
3. Delivery 成功
4. 重启
5. Subscription 恢复
6. 更新到新 Generation
7. 旧 Generation Delivery 取消
8. 新 Generation 正常投递
```

---

## 19.5 Schedule

```text
1. 注册 One-shot Schedule
2. 到期执行 Tool
3. 注册 Task Schedule
4. 创建 TaskRun
5. 重启
6. 不重复执行已完成 Trigger
7. 禁用 Extension
8. 后续不触发
```

---

## 19.6 Chat Tool

```text
1. 模型 Tool 列表来自 Kernel
2. 旧 Runtime Tool 列表为空或不参与生产
3. 调用 Tool
4. 只有一次 Operation
5. 只有一次 Invocation
6. Legacy Tool Counter 仍为 0
```

---

## 19.7 MCP

```text
1. 添加测试 MCP Server
2. 转换为 MCP Contribution
3. 模型 Tool 列表出现一次
4. 调用经 Permission / Scope
5. 禁用 Extension 后 Tool 消失
6. 无第二份 MCP Tool 暴露
```

---

## 19.8 Dev Mode

```text
1. amitiax dev 注册 Workspace
2. v1 加载
3. 修改 v2
4. 新 Generation 激活
5. 旧 Runtime 停止
6. 制造 v3 无效
7. Reload 失败
8. v2 继续运行
9. Ctrl+C
10. Workspace 和 Runtime 清理
```

---

## 19.9 更新与卸载

```text
1. 更新 Extension
2. 旧 Event/Schedule/Runtime 注销
3. 新 Generation 唯一激活
4. 卸载
5. 无 Runtime
6. 无 Process
7. 无 Event Subscription
8. 无 Delivery
9. 无 Schedule
10. 无 Lease
11. 无 UI Session
12. 无 Shortcut
```

---

# 二十、故障注入

必须在以下阶段注入：

```text
Permission Broker 不可用
Scope Snapshot Repository 不可用
Schedule Start 失败
Schedule Executor 崩溃
Event Subscription 持久化失败
Event Generation 切换失败
Tool Facade 执行失败
MCP Transport 断开
签名 Store 不可用
Update 中崩溃
Dev Reload 中崩溃
卸载中崩溃
```

必须验证：

```text
Fail Closed
旧 Generation 不被误删
不会双执行
不会静默 Allow
不会报告 Passed
应用重启可恢复
```

---

# 二十一、安全验收

必须真实验证：

```text
Host API 无 Permission
Host API 错误 Scope
伪造 ScopeSnapshotID
旧 Generation 重放
Event 越权订阅
Event Payload 敏感字段越权
Schedule 越权执行
MCP Tool 越权执行
篡改 Manifest
篡改资源
伪造 Publisher
Unknown Key
Revoked Key
路径穿越
跨 Extension Resource
开发模式绕过 Permission
```

---

# 二十二、资源泄漏验收

重复执行 100 次：

```text
安装
启用
调用
禁用
卸载
```

检测：

```text
Goroutine
Runtime Instance
Child Process
Event Subscription
Pending Delivery
Schedule Lease
Schedule Run
Task Lease
UI Session
Bridge
MCP Connection
Temporary Directory
File Handle
```

增长必须回落到允许基线。

---

# 二十三、构建命令

必须在匹配工具链中执行：

```bash
cd backend
go version
go test ./...
go vet ./...
go build ./cmd/server
go build -o amitiax ./cmd/amitia-ext
```

前端：

```bash
npm ci
npm run typecheck
npm run build
```

SDK：

```bash
npm run typecheck
npm run test
npm run build
```

桌面端：

```bash
npm run typecheck
npm run build
```

必须保存真实退出码和日志。

---

# 二十四、禁止修改方式

禁止：

```text
把 Permission Checker 改成另一个 return nil
把 Schedule Start 错误只记录 Warn
删除 Generation 校验
把 Event Effective State 继续默认为 true
用应用启动时重新扫描 Manifest 代替 Subscription 持久化
继续保留 Chat 使用旧 Runtime
同时让新旧 Tool 都向模型暴露
遍历所有 TrustedKeys 直到任意结果
Unknown Key 自动通过
保留三套签名协议作为生产格式
把 Final Acceptance 改成更多固定 Passed
降低测试断言
删除失败测试
用 Mock E2E
修改无关业务
```

---

# 二十五、提交要求

每个阶段单独提交，建议：

```text
fix(extension): wire permission and scope enforcement
fix(extension): start and drain persistent schedule service
fix(extension): persist event subscriptions and enforce generation
refactor(extension): route model tools through kernel
refactor(extension): unify mcp tool contributions
fix(extension): unify amitiax signature v1
refactor(extension): consolidate amitiax cli
test(extension): replace synthetic acceptance with real evidence
refactor(extension): disable legacy production runtime
test(extension): add full kernel e2e and leak checks
```

---

# 二十六、完成后必须输出的产物

必须提交：

```text
1. 修改文件清单
2. 新增文件清单
3. 删除或禁用文件清单
4. Permission 映射表
5. Scope 校验规则
6. Schedule 启动与关闭顺序
7. Schedule Target Adapter 清单
8. Event Subscription Repository Schema
9. Event Effective State 解析规则
10. Event Generation 切换说明
11. Kernel Tool Facade 说明
12. MCP 统一执行说明
13. Legacy Runtime 切断说明
14. 唯一签名协议
15. Canonical Artifact Hash 算法
16. Key ID 算法
17. CLI 唯一化说明
18. Legacy Compatibility 策略
19. Final Acceptance 真实 Runner 清单
20. 数据库 Migration
21. 单元测试结果
22. 集成测试结果
23. E2E 日志
24. 故障注入结果
25. 安全测试结果
26. 资源泄漏结果
27. Legacy Zero-call 报告
28. 未完成项
29. 最终验收报告
```

---

# 二十七、最终验收检查表

## Permission / Scope

- [ ] Host API 不再使用空 Checker
- [ ] Permission Broker 真正生效
- [ ] Scope Manager 真正生效
- [ ] 缺失依赖时 Fail Closed
- [ ] 旧 Generation 被拒绝

## Schedule

- [ ] Schedule Service 在生产启动
- [ ] Schedule Service 在关闭时 Drain
- [ ] 所有 Target Adapter 已接入
- [ ] Permission / Scope / Dependency 已接入
- [ ] Generation 永假条件已修复
- [ ] 重启后不重复触发
- [ ] 禁用卸载无残留

## Event

- [ ] Subscription 已持久化
- [ ] 启动时可恢复
- [ ] Effective State 不再硬编码
- [ ] 索引删除已修复
- [ ] Generation 更新原子
- [ ] 旧 Delivery 被拒绝
- [ ] Permission / Scope 真正生效

## 单一主链

- [ ] Chat Tool 通过 Kernel
- [ ] 旧 Runtime 不再暴露模型 Tool
- [ ] 旧 PluginManager 不再生产启动
- [ ] MCP Tool 只注册一次
- [ ] 安装更新卸载只走 Kernel
- [ ] Legacy Call Counter 全为 0

## 签名

- [ ] 只有一个生产签名格式
- [ ] 只有一个 Key ID 算法
- [ ] CLI 与 Kernel 互操作
- [ ] 安装器精确查找 Publisher/Key
- [ ] 所有错误状态正确处理
- [ ] Unknown Key 不静默通过
- [ ] Publisher 变化不作为普通更新

## CLI

- [ ] `amitiax` 是正式命令
- [ ] `amitia-ext` 只作为兼容别名
- [ ] TypeScript CLI 不再复制签名和打包逻辑
- [ ] Dev Host 真正热重载
- [ ] Reload 失败保留旧 Generation
- [ ] Diagnostics 来自真实 Read Model

## Acceptance

- [ ] Final Acceptance 无固定 Passed
- [ ] Stability 无固定 Passed
- [ ] Security 无固定 Passed
- [ ] Cutover PreCheck 无固定 Passed
- [ ] Required Blocked 时 ReleaseReady=false
- [ ] Evidence 包含真实日志和 Hash

## 测试

- [ ] Go 1.26.1 环境
- [ ] go test ./... 通过
- [ ] go vet ./... 通过
- [ ] 后端 build 通过
- [ ] 前端 typecheck/build 通过
- [ ] SDK 测试通过
- [ ] Electron 构建通过
- [ ] E2E 通过
- [ ] 故障注入通过
- [ ] 安全测试通过
- [ ] 泄漏测试通过

---

# 二十八、最终验收结论

最终报告只能输出以下二者之一：

```text
达到 Amitia 扩展系统全量修补与 Operit 同级可用性目标
```

或：

```text
未达到 Amitia 扩展系统全量修补与 Operit 同级可用性目标
```

不得输出：

```text
基本完成
主体完成
大部分完成
架构已完成
后续优化
```

若未达到，必须列出：

```text
失败测试
残留旧主链
安全空放行
未启动服务
签名不互通
默认 Passed
资源泄漏
阻塞原因
```

---

# 二十九、执行纪律

必须遵守：

```text
先补真实失败测试
再修生产接线
先修 Permission/Scope
再启动 Schedule
再修 Event
再切 Tool/MCP 主链
再统一签名
再统一 CLI
再删除默认 Passed
最后切断旧系统
```

执行中：

```text
不修改无关功能
不删除用户数据
不先物理删除旧表
不保留双执行
不降低安全策略
不伪造测试证据
```

只有在：

```text
新链真实 E2E 通过
旧链计数为 0
应用重启恢复通过
卸载资源清理通过
```

后，才能宣布完成。

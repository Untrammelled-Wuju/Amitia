# Amitia 扩展系统重构第 23 步实施文档

## 第 23 步：实现统一 Contribution Registry

---

## 一、步骤目标

在第 21 步完成 Extension Kernel 领域模型、第 22 步完成唯一生命周期管理器之后，建立 Amitia 唯一的 Contribution Registry。

本步骤目标是：

> 统一注册、查询、激活、停用和注销 Tool、Agent Skill、Workflow、MCP Server、Provider、Hook、Event Subscription、Schedule、Background Task、UI 与 Desktop Contribution，彻底结束各子系统分别维护注册表的状态。

完成后，所有扩展贡献必须遵循：

```text
ContributionDefinition
→ Contribution Registry
→ 类型适配器
→ 专用目录或运行服务
→ EffectiveStateResolver
→ 对外暴露
```

禁止继续出现：

```text
Plugin 注册 Tool
MCP Manager 注册 Tool
Workflow Manager 注册 Skill
Agent Skill Runtime 注册伪 Tool
前端动态注入路由
```

---

## 二、Registry 的职责

Contribution Registry 负责：

- 接收已验证的 ContributionDefinition；
-维护稳定 ID 与版本；
-建立 Extension、Module、Runtime、Dependency 索引；
-维护注册状态与激活状态；
-分发到类型专用 Adapter；
-执行原子批次注册与替换；
-提供一致性查询；
-提供变更 Diff；
-发布注册领域事件；
-支持 Lifecycle Manager 调用；
-支持恢复、重建和对账；
-提供 Effective State 查询入口；
-记录审计与诊断。

Registry 不负责：

- 解析原始 Manifest；
-安装包；
-授予 Permission；
-创建 Scope；
-执行 Tool；
-启动 Runtime；
-建立 MCP 连接；
-运行 Workflow；
-渲染 UI；
-持久化用户业务数据。

---

## 三、核心接口

建议：

```go
type ContributionRegistry interface {
    RegisterBatch(
        ctx context.Context,
        batch ContributionRegistrationBatch,
    ) ContributionRegistrationResult

    ReplaceGeneration(
        ctx context.Context,
        request ContributionReplacementRequest,
    ) ContributionRegistrationResult

    UnregisterBatch(
        ctx context.Context,
        request ContributionUnregisterRequest,
    ) ContributionRegistrationResult

    Get(
        ctx context.Context,
        id ContributionID,
    ) (RegisteredContribution, error)

    List(
        ctx context.Context,
        filter ContributionFilter,
    ) ([]RegisteredContribution, error)

    Resolve(
        ctx context.Context,
        request ContributionResolveRequest,
    ) ContributionResolveResult

    Rebuild(
        ctx context.Context,
        extensionID ExtensionID,
    ) ContributionRegistrationResult
}
```

---

## 四、注册对象

建议：

```go
type RegisteredContribution struct {
    Definition      ContributionDefinition
    DefinitionHash  string
    Generation      int64
    Registration    ContributionRegistrationState
    EffectiveState  EffectiveState
    AdapterType     string
    RuntimeBinding  *RuntimeBindingDefinition
    RegisteredAt    time.Time
    UpdatedAt       time.Time
}
```

Registry 只保存 Definition 引用和派生注册状态，不复制完整 Runtime 状态。

---

## 五、注册状态

建议：

```text
pending
registered
active
inactive
degraded
failed
unregistering
unregistered
```

含义：

- `registered`：已进入 Registry；
-`active`：满足激活条件并已提交到专用目录；
-`inactive`：定义存在，但因 Enabled、Scope、Runtime 等不可激活；
-`degraded`：部分功能可用；
-`failed`：适配或专用注册失败。

注册状态不能替代业务 Enabled。

---

## 六、类型适配器

建议接口：

```go
type ContributionAdapter interface {
    Type() ContributionType

    Validate(
        ctx context.Context,
        definition ContributionDefinition,
    ) ValidationReport

    Register(
        ctx context.Context,
        contribution RegisteredContribution,
    ) AdapterRegistrationResult

    Activate(
        ctx context.Context,
        contribution RegisteredContribution,
    ) AdapterActivationResult

    Deactivate(
        ctx context.Context,
        contribution RegisteredContribution,
    ) AdapterActivationResult

    Unregister(
        ctx context.Context,
        contribution RegisteredContribution,
    ) AdapterRegistrationResult
}
```

首批 Adapter：

```text
ToolContributionAdapter
AgentSkillContributionAdapter
WorkflowContributionAdapter
MCPServerContributionAdapter
ProviderContributionAdapter
HookContributionAdapter
EventSubscriptionContributionAdapter
ScheduleContributionAdapter
BackgroundTaskContributionAdapter
UIContributionAdapter
DesktopContributionAdapter
ResourceContributionAdapter
```

---

## 七、Tool Contribution Adapter

负责：

```text
Tool Contribution Spec
→ ToolDefinition
→ ToolRegistry
```

必须映射：

- 稳定 Tool ID；
-Input/Output Schema；
-RuntimeBinding；
-风险；
-副作用；
-Owner；
-版本；
-Exposure；
-依赖；
-Definition Hash。

不得映射：

- Permission Grant；
-Scope Binding；
-Runtime Ready；
-当前角色；
-当前 MCP Session。

---

## 八、Agent Skill Contribution Adapter

负责：

```text
Agent Skill Contribution
→ AgentSkillDefinition Reference
→ AgentSkillCatalog
```

不得把 Agent Skill 本体注册为 Tool。

---

## 九、Workflow Contribution Adapter

负责：

- 注册 WorkflowDefinition；
-可选创建 Workflow Tool Adapter；
-注册 Schedule Definition；
-建立依赖；
-保持 Workflow ID 与 Tool ID 稳定。

---

## 十、MCP Server Contribution Adapter

负责：

- 注册 MCP Server Definition；
-登记 Owner；
-创建 Desired State 输入；
-建立 Discovery 结果更新通道；
-将发现的 MCP Tool 转换为动态子 Contribution。

MCP Tool 动态 Contribution ID 必须稳定：

```text
<server-contribution-id>/tool/<tool-name>
```

重连不得重复生成。

---

## 十一、动态 Contribution

允许的动态来源：

- MCP Discovery；
-受控 Provider Discovery；
-运行时声明且已在 Manifest 允许范围内的子 Contribution。

动态 Contribution 必须：

- 有父 Contribution；
-有稳定 ID；
-有 Revision Hash；
-受 Manifest Allowlist 约束；
-受数量限制；
-受类型限制；
-不能动态申请未声明 Permission；
-不能动态扩大 Scope；
-重启后可重建；
-卸载时可级联清理。

---

## 十二、原子批次注册

Extension 安装或更新时必须批次注册：

```text
Validate All
→ Build Registration Plan
→ Register Pending
→ Apply Type Adapters
→ Verify
→ Activate Eligible
→ Commit Generation
```

任一关键 Contribution 注册失败时：

- 整批失败并补偿；
-或按 Manifest/Policy 明确允许部分模块降级。

默认不允许“静默漏注册”。

---

## 十三、Generation 替换

更新 Extension 时：

```text
old_generation
→ new_generation pending
→ new adapters register
→ atomic switch
→ old generation deactivate
→ old generation unregister
```

禁止先删除旧 Contribution，再尝试注册新 Contribution。

---

## 十四、依赖顺序

Registry 注册顺序必须由 Dependency Resolver 提供。

通常：

```text
Provider
→ Runtime-backed foundational contribution
→ MCP Server
→ Tool
→ Workflow
→ Agent Skill
→ Hook/Event
→ Schedule/Background Task
→ UI/Desktop
```

实际顺序按显式依赖图决定。

---

## 十五、激活条件

Contribution 激活必须满足：

```text
Definition Valid
AND Extension Enabled
AND Module Enabled
AND Override Allows
AND Dependencies Ready
AND Runtime Desired/Actual Ready
AND Health Acceptable
AND Circuit Allows
AND Platform Compatible
```

Scope 和 Permission 是否参与“全局激活”需按类型决定：

- Tool 可注册但执行时按 Invocation Scope/Permission 再判断；
-Agent Skill Catalog 可注册但候选激活按 Scope；
-UI Contribution 激活通常需要当前用户 Scope；
-Schedule 激活必须绑定固定 Scope 与 Permission Reference。

---

## 十六、可见与可执行

Registry 应提供：

```text
registered
active
visible
executable
```

四种不同结果。

禁止简单返回：

```text
enabled=true
```

作为全部状态。

---

## 十七、查询索引

至少建立：

- Contribution ID；
-Extension ID；
-Module ID；
-Type；
-Runtime ID；
-Owner；
-Generation；
-Definition Hash；
-Enabled Override；
-Registration State；
-Dependency Target；
-Exposure；
-平台；
-风险级别。

---

## 十八、冲突检测

必须检测：

```text
duplicate_contribution_id
duplicate_tool_id
duplicate_model_name
duplicate_ui_slot_key
duplicate_hook_key
duplicate_schedule_id
runtime_binding_missing
adapter_type_unsupported
generation_conflict
owner_conflict
definition_hash_conflict
dynamic_child_conflict
```

---

## 十九、模型 Tool 名称冲突

稳定 Tool ID 与模型名称分离。

模型名称冲突时可：

- 使用稳定命名空间；
-生成明确别名；
-按用户选择；
-拒绝激活低优先级冲突项。

不得修改稳定 Tool ID。

---

## 二十、UI Slot 冲突

UI Contribution 冲突策略：

```text
exclusive
ordered_multi
single_winner
replaceable
```

Registry 只记录冲突和排序元数据，真正 Slot Host 在后续 UI 阶段实现。

---

## 二十一、Hook 冲突

Hook 支持多订阅，但必须有：

- 优先级；
-阶段；
-是否可修改；
-失败策略；
-稳定排序。

排序键建议：

```text
priority DESC
extension_id ASC
contribution_id ASC
```

避免随机顺序。

---

## 二十二、Schedule 注册

Schedule 注册后不立即执行。

必须：

```text
Registry Active
→ Scheduler 接收定义
→ Scheduler 验证 Scope/Permission/Owner
→ 计算 Next Run
```

---

## 二十三、恢复与重建

启动时 Registry 不读取旧缓存作为真值。

流程：

```text
Load active ExtensionDefinition
→ Load Installation/Module/Override
→ Resolve Dependencies
→ Build Registration Batch
→ Register
→ Reconcile Runtime
→ Activate
```

Registry Cache 可完全重建。

---

## 二十四、持久化

建议表：

```text
extension_contribution_registration_states
extension_contribution_generations
extension_dynamic_contributions
extension_contribution_conflicts
extension_contribution_adapter_states
```

Definition 仍存领域 Repository。

---

## 二十五、注册事件

发布：

```text
ContributionRegistered
ContributionActivated
ContributionDeactivated
ContributionUnregistered
ContributionRegistrationFailed
ContributionGenerationSwitched
DynamicContributionDiscovered
DynamicContributionRemoved
```

通过 Outbox 可靠发布。

---

## 二十六、生命周期接入

Lifecycle Manager 只能通过 Registry 接口操作 Contribution。

禁止 Lifecycle Step 直接调用 ToolRegistry、AgentSkillCatalog、WorkflowRegistry 或前端路由。

---

## 二十七、运行时故障处理

Runtime Crash 时：

- 不删除 Definition；
-将相关 Contribution Effective State 设为不可执行；
-调用 Adapter Deactivate；
-保留 Registry 记录；
-等待 Runtime Recovery；
-恢复后重新 Activate；
-不得重复 Register。

---

## 二十八、Extension 禁用

禁用时：

```text
Deactivate
→ 保留 Registered
```

卸载时：

```text
Deactivate
→ Unregister
→ 删除 Registration State
```

---

## 二十九、前端查询

Extension 详情应从统一 Registry 获取：

- Contribution 类型；
-注册状态；
-激活状态；
-实际可用性；
-依赖；
-Runtime；
-冲突；
-故障原因；
-来源；
-Generation。

---

## 三十、测试要求

必须覆盖：

- 单 Contribution 注册；
-多类型批次；
-原子失败；
-Generation 替换；
-动态 MCP Tool；
-重连不重复；
-Tool 名冲突；
-UI Slot 冲突；
-Hook 排序；
-Schedule 不提前执行；
-Runtime Crash；
-Extension Disable；
-Module Disable；
-Uninstall；
-Rebuild；
-事件幂等；
-并发注册；
-大量 Contribution 性能。

---

## 三十一、实施任务

1. 定义 Registry 接口与 RegisteredContribution。
2. 定义 Registration State。
3. 建立 Adapter Registry。
4. 实现 Tool Adapter。
5. 实现 Agent Skill Adapter。
6. 实现 Workflow Adapter。
7. 实现 MCP Server 与动态 Tool Adapter。
8. 实现 Provider、Hook、Event、Schedule、Background Task Adapter。
9. 建立 UI/Desktop Adapter 边界。
10. 实现原子 Batch Register。
11. 实现 Generation Replace。
12. 接入 Dependency Resolver 接口。
13. 接入 EffectiveStateResolver。
14. 接入 Lifecycle Manager。
15. 建立注册事件 Outbox。
16. 实现冲突检测。
17. 实现启动 Rebuild。
18. 迁移旧 Registry 写入入口。
19. 改造前端统一查询。
20. 完成故障注入与性能测试。

---

## 三十二、验收产物

必须提交：

- Contribution Registry 主文档；
-Registry 接口；
-全部首批 Adapter；
-原子批次注册；
-Generation 切换；
-动态 Contribution；
-冲突检测；
-持久化状态；
-生命周期接入；
-恢复重建；
-旧入口迁移报告；
-测试报告。

---

## 三十三、验收标准

1. 所有 Contribution 只有一个 Registry。
2. 各系统不再自行注册。
3. Registry 输入只能是 ContributionDefinition。
4. Agent Skill 不进入 ToolRegistry。
5. Workflow 仅通过 Adapter 暴露 Tool。
6. MCP 重连不重复 Tool。
7. 更新使用 Generation 原子切换。
8. Runtime 故障只停用，不删除 Definition。
9. Extension Disabled 保留注册定义。
10. Uninstall 完整注销。
11. 冲突可解释且稳定。
12. Registry 可完全重建。
13. 关键测试通过。
14. 可进入第 24 步 Dependency Resolver。

---

## 三十四、执行约束

> Contribution Registry 只管理贡献定义的注册与激活，不负责安装、执行、授权、Scope 创建或 Runtime 启停。

禁止：

- 直接注册原始 Manifest；
-Adapter 启动 Runtime；
-Registry 自动 Grant Permission；
-Registry 自动扩大 Scope；
-旧 Registry 与新 Registry 双写；
-MCP Session ID 进入稳定 ID；
-前端动态注入绕过 Registry。

# Amitia 扩展系统重构第 21 步实施文档

## 第 21 步：定义 Extension Kernel 领域模型

---

## 一、步骤目标

在前 20 步已经完成旧系统冻结、调用链盘点、数据与资源归属盘点、概念拆分、统一 Tool、统一执行安全、统一 Permission、统一 Scope、统一运行审计、统一资源所有权、包安全、MCP、Agent Skill、Workflow、Plugin Runtime 安全、统一生命周期、状态真值清理和只读迁移边界的基础上，正式定义 Amitia 新一代 Extension Kernel 的核心领域模型。

本步骤的目标是：

> 建立 Amitia 扩展系统唯一的领域语言、核心实体、聚合边界、标识规则、版本规则、状态规则、生命周期规则和依赖方向，使后续所有安装、注册、运行、迁移、更新、回滚、卸载、UI Contribution、Runtime 和 SDK 都基于同一套模型实现。

本步骤是整个扩展系统重建的领域基础。

后续步骤中的：

- Extension Lifecycle Manager；
-Contribution Registry；
-Dependency Resolver；
-Runtime Supervisor；
-Host API Gateway；
-Secret Broker；
-Event Bus；
-Manifest v2；
-多模块包；
-`.amitiax` 安装器；
-JavaScript Runtime；
-Service Runtime；
-WASM Runtime；
-UI Contribution；
-Extension Center；
-SDK；
-CLI；
-迁移；
-旧系统删除；

都必须以本步骤定义的领域模型为唯一基础。

---

## 二、需要解决的根本问题

旧扩展系统的主要问题不是单一实现缺陷，而是缺乏统一领域模型。

当前可能同时存在：

```text
Package
Extension
Plugin
Skill
Agent Skill
MCP Server
MCP Tool
Workflow
Runtime
Module
Contribution
Artifact
Owned Resource
Dependency
Provider
Hook
Schedule
```

但它们之间可能没有稳定边界，导致：

1. Package 被当作 Extension 本体。
2. Plugin 被当作包、运行时、功能和 Go Handler 的混合概念。
3. Skill 同时表示 Tool、Agent Skill、Workflow、MCP Tool。
4. Module 不存在或只有隐式结构。
5. Contribution 缺少统一抽象。
6. Runtime 与 Extension 生命周期混合。
7. Definition、Installation、Enablement、Runtime State 混合。
8. Manifest 字段直接映射数据库结构。
9. 资源所有权与模块边界不一致。
10. 依赖关系只在安装阶段临时处理。
11. Extension ID、Package ID、Plugin ID、Skill ID 使用不同规则。
12. 版本、兼容性、更新和回滚没有统一约束。
13. 内置能力和第三方能力使用不同领域模型。
14. UI、Tool、Hook、Provider、Workflow 等 Contribution 各自注册。
15. Package 卸载时无法从聚合根确定全部资源。
16. Runtime 崩溃时无法判断影响哪些 Contribution。
17. Module 禁用时无法确定应停止哪些 Runtime。
18. 新 `.amitiax` 很容易再次退化为旧 Package Manifest。

本步骤必须先把领域模型锁定，再进入后续实现。

---

## 三、核心领域语言

统一使用以下术语：

```text
Extension
Extension Package
Extension Module
Contribution
Runtime Definition
Runtime Instance
Capability
Tool
Agent Skill
Workflow
MCP Server
Provider
Hook
Event Subscription
Schedule
UI Contribution
Dependency
Resource
Artifact
Owner
Installation
Enablement
Activation
Exposure
Execution
```

禁止继续使用含义不清的：

```text
Skill
Plugin Object
Extension Item
Component Item
Feature Package
Runtime Skill
MCP Skill
Workflow Skill
```

如果保留“插件”作为用户界面名称，后端领域必须仍使用：

```text
Extension
Extension Module
Contribution
Runtime
```

---

## 四、领域层次

建议将 Extension Kernel 领域分为五层：

```text
1. Distribution Layer
   ExtensionPackage
   Artifact
   Manifest
   Publisher
   Signature

2. Definition Layer
   ExtensionDefinition
   ModuleDefinition
   ContributionDefinition
   RuntimeDefinition
   DependencyDefinition

3. Installation Layer
   ExtensionInstallation
   InstalledModule
   InstalledContribution
   ResourceOwnership

4. Runtime Layer
   RuntimeInstance
   RuntimeBinding
   DesiredState
   ActualState
   Health
   Circuit

5. Execution Layer
   Operation
   Invocation
   Attempt
   ScopeSnapshot
   PermissionDecision
   SideEffect
```

这五层必须分离。

---

## 五、核心聚合根

建议 Extension Kernel 至少包含以下聚合根：

```text
ExtensionDefinition
ExtensionInstallation
ExtensionModule
Contribution
RuntimeDefinition
DependencyGraph
```

其中最核心的聚合根是：

```text
ExtensionDefinition
```

但它不直接保存所有运行状态。

---

## 六、ExtensionDefinition

建议定义：

```go
type ExtensionDefinition struct {
    ID              ExtensionID
    Name            LocalizedText
    Description     LocalizedText
    Version         SemanticVersion
    ManifestVersion int

    Publisher       PublisherReference
    Package         PackageReference

    Modules         []ModuleDefinition
    Dependencies    []DependencyDefinition
    Compatibility   ExtensionCompatibility
    Integrity       ExtensionIntegrity
    Policies        ExtensionPolicies

    Metadata        map[string]any
}
```

ExtensionDefinition 表示：

> 某个 Extension 某个版本的不可变定义。

要求：

- 定义层不可保存运行实例；
-不可保存当前连接；
-不可保存当前角色；
-不可保存 Permission Grant；
-不可保存 Scope Snapshot；
-不可保存 Runtime Health；
-不可保存用户当前启用状态；
-不可保存安装路径；
-不可保存临时缓存；
-不可保存 Handler 闭包。

---

## 七、ExtensionID

建议类型：

```go
type ExtensionID string
```

格式：

```text
<reverse-domain>/<extension-name>
```

示例：

```text
com.example/weather
top.untrammelled/amitia-tools
org.open-source/local-memory
```

要求：

- 全局稳定；
-全小写；
-使用 ASCII；
-不能包含版本；
-不能包含平台；
-不能包含角色；
-不能包含数据库 ID；
-不能由显示名称直接决定；
-安装新版本不变化；
-发布者更换不自动改变；
-发生所有权转让时记录 Publisher 变更，不修改 ID；
-用户本地 Extension 使用保留命名空间。

建议用户本地命名空间：

```text
local.user/<name>
```

---

## 八、Extension Version

必须使用标准语义化版本：

```text
MAJOR.MINOR.PATCH
```

可包含：

```text
pre-release
build metadata
```

建议类型：

```go
type SemanticVersion struct {
    Major int
    Minor int
    Patch int
    PreRelease string
    Build string
}
```

禁止：

- 浮点数；
-整数递增代替版本；
-日期字符串作为唯一版本；
-将数据库 Migration Version 当 Extension Version；
-将 Runtime Protocol Version 当 Extension Version。

---

## 九、Extension Package

建议定义：

```go
type ExtensionPackage struct {
    PackageID       string
    ExtensionID     ExtensionID
    Version         SemanticVersion
    ArtifactID      string
    ManifestVersion int
    ArchiveHash     string
    ContentTreeHash string
    Signature       SignatureReference
    Publisher       PublisherReference
    CreatedAt       time.Time
}
```

Extension Package 表示：

> Extension 某版本的分发载体。

必须明确：

```text
ExtensionPackage
≠
ExtensionInstallation
≠
ExtensionDefinition
```

---

## 十、Extension Installation

建议定义：

```go
type ExtensionInstallation struct {
    InstallationID   string
    ExtensionID      ExtensionID
    InstalledVersion SemanticVersion
    PackageID        string

    InstallationState InstallationState
    EnablementState   EnablementState

    InstalledAt      time.Time
    UpdatedAt        time.Time
    Generation       int64

    ActiveSnapshotID string
    RollbackPoints   []RollbackPointReference
    Metadata         map[string]any
}
```

ExtensionInstallation 表示：

> 当前 Amitia 实例中某个 Extension 的安装事实和用户启用意图。

它负责：

- 当前安装版本；
-安装状态；
-总开关；
-Generation；
-回滚点引用。

它不负责：

- Runtime 实例；
-Tool Registry；
-Permission Grant；
-Scope Binding；
-运行健康；
-模块实际 Ready；
-执行记录。

---

## 十一、ModuleDefinition

建议定义：

```go
type ModuleDefinition struct {
    ID            ModuleID
    ExtensionID   ExtensionID
    Name          LocalizedText
    Description   LocalizedText
    Type          ModuleType
    Version       string

    Runtime       *RuntimeDefinition
    Contributions []ContributionDefinition
    Dependencies  []DependencyDefinition
    Compatibility ModuleCompatibility
    Policies      ModulePolicies
    Metadata      map[string]any
}
```

Module 表示：

> Extension 内部具有独立职责、依赖、运行时和启停边界的功能单元。

---

## 十二、ModuleID

建议：

```text
<extension-id>#<module-name>
```

示例：

```text
com.example/weather#main
com.example/weather#desktop-ui
com.example/weather#mcp
```

要求：

- 在 Extension 内唯一；
-稳定；
-不含版本；
-不依赖显示名称；
-不依赖平台运行实例；
-主模块可统一命名为 `main`。

---

## 十三、ModuleType

建议：

```go
type ModuleType string

const (
    ModuleTypeRuntime       ModuleType = "runtime"
    ModuleTypeAgentSkill    ModuleType = "agent_skill"
    ModuleTypeWorkflow      ModuleType = "workflow"
    ModuleTypeMCP           ModuleType = "mcp"
    ModuleTypeUI            ModuleType = "ui"
    ModuleTypeProvider      ModuleType = "provider"
    ModuleTypeData          ModuleType = "data"
    ModuleTypeComposite     ModuleType = "composite"
)
```

说明：

- ModuleType 用于描述主要职责；
-不能代替 Contribution 列表；
-Composite 仅用于确实包含多类能力的模块；
-不要把每个 Tool 建成单独 Module；
-不要把整个 Extension 强制限制为单 Module。

---

## 十四、InstalledModule

建议：

```go
type InstalledModule struct {
    ModuleID        ModuleID
    InstallationID  string
    EnablementState EnablementState
    Generation      int64
    InstalledAt     time.Time
    UpdatedAt       time.Time
}
```

InstalledModule 保存模块级启用意图。

不保存：

- Runtime Ready；
-Health；
-Circuit；
-当前 Connection；
-Contribution 实际状态。

---

## 十五、Contribution

Contribution 表示：

> Extension Module 向 Amitia 宿主声明并提供的一项可注册能力或集成点。

统一 Contribution 类型：

```go
type ContributionType string

const (
    ContributionTool              ContributionType = "tool"
    ContributionAgentSkill        ContributionType = "agent_skill"
    ContributionWorkflow          ContributionType = "workflow"
    ContributionMCPServer         ContributionType = "mcp_server"
    ContributionProvider          ContributionType = "provider"
    ContributionHook              ContributionType = "hook"
    ContributionEventSubscription ContributionType = "event_subscription"
    ContributionSchedule          ContributionType = "schedule"
    ContributionBackgroundTask    ContributionType = "background_task"
    ContributionUI                ContributionType = "ui"
    ContributionDesktop           ContributionType = "desktop"
    ContributionResource          ContributionType = "resource"
)
```

---

## 十六、ContributionDefinition

建议定义：

```go
type ContributionDefinition struct {
    ID            ContributionID
    ExtensionID   ExtensionID
    ModuleID      ModuleID
    Type          ContributionType
    Name          LocalizedText
    Description   LocalizedText

    Specification json.RawMessage
    RuntimeBinding *RuntimeBindingDefinition
    Dependencies  []DependencyDefinition
    ScopeRule     ScopeRule
    PermissionRequirements []PermissionRequirement

    Exposure      ContributionExposure
    Policies      ContributionPolicies
    Integrity     ContributionIntegrity
    Metadata      map[string]any
}
```

---

## 十七、ContributionID

建议：

```text
<module-id>/<type>/<name>
```

示例：

```text
com.example/weather#main/tool/get_forecast
com.example/weather#main/agent_skill/weather-assistant
com.example/weather#desktop-ui/ui/settings-page
```

要求：

- 全局唯一；
-稳定；
-不含版本；
-不含 Runtime Instance；
-不含角色；
-不含 Session；
-不含 MCP Session ID；
-显示名称变化不影响 ID。

---

## 十八、Contribution Specification

不同 Contribution 使用不同规范类型。

示例：

```go
type ToolContributionSpec struct {
    InputSchema  json.RawMessage
    OutputSchema json.RawMessage
    RiskLevel    RiskLevel
    SideEffects  SideEffectLevel
}

type AgentSkillContributionSpec struct {
    DefinitionRef string
}

type WorkflowContributionSpec struct {
    WorkflowRef string
    ToolExposure *WorkflowToolExposure
}

type UIContributionSpec struct {
    Slot       string
    EntryPoint string
    Sandbox    string
}
```

禁止将所有 Specification 塞入一个巨大 Struct。

建议：

- 基础结构统一；
-类型专用 Spec 分离；
-Manifest Parser 根据 Type 解码；
-Registry 根据 Type 分发。

---

## 十九、Contribution 与 Capability

关系：

```text
Contribution
→ 可能产生 Capability
```

例如：

### Tool Contribution

直接产生 Tool Capability。

### Workflow Contribution

产生 Workflow Definition，并可选产生 Tool Capability。

### MCP Server Contribution

产生 MCP Server Definition，Discovery 后产生动态 Tool Capability。

### Agent Skill Contribution

不产生可执行 Tool，但产生可激活 Agent Skill。

### UI Contribution

不产生 Tool，但注册 UI 扩展点。

因此：

```text
Contribution
≠
Capability
```

---

## 二十、RuntimeDefinition

建议定义：

```go
type RuntimeDefinition struct {
    RuntimeID      RuntimeDefinitionID
    ExtensionID    ExtensionID
    ModuleID       ModuleID
    Type           RuntimeType
    EntryPoint     string
    Protocol       string

    ResourceLimits RuntimeResourceLimits
    TimeoutPolicy  RuntimeTimeoutPolicy
    Concurrency    RuntimeConcurrencyPolicy
    Recovery       RuntimeRecoveryPolicy
    Platform       RuntimePlatformRequirement

    Metadata       map[string]any
}
```

RuntimeDefinition 表示：

> 某个 Module 需要怎样的执行环境。

---

## 二十一、RuntimeType

建议：

```go
type RuntimeType string

const (
    RuntimeTypeHostInternal RuntimeType = "host_internal"
    RuntimeTypeLegacyGo     RuntimeType = "legacy_go"
    RuntimeTypeJavaScript   RuntimeType = "javascript"
    RuntimeTypeTask         RuntimeType = "task"
    RuntimeTypeService      RuntimeType = "service"
    RuntimeTypeWASM         RuntimeType = "wasm"
    RuntimeTypeMCP          RuntimeType = "mcp"
    RuntimeTypeWorkflow     RuntimeType = "workflow"
    RuntimeTypeStatic       RuntimeType = "static"
)
```

说明：

- `host_internal`：系统内置；
-`legacy_go`：过渡内置 Go Plugin；
-`javascript`：未来主插件 Runtime；
-`task`：后台任务 Runtime；
-`service`：受信任独立服务；
-`wasm`：WASM 沙箱；
-`mcp`：MCP 连接运行时；
-`workflow`：Workflow Executor；
-`static`：无代码，仅声明资源或 UI Schema。

---

## 二十二、Runtime Definition 与 Runtime Instance

必须区分：

```text
RuntimeDefinition
```

描述运行环境要求。

```text
RuntimeInstance
```

表示实际启动的一次运行实例。

建议：

```go
type RuntimeInstance struct {
    InstanceID       string
    RuntimeID        RuntimeDefinitionID
    ExtensionID      ExtensionID
    ModuleID         ModuleID
    Generation       int64

    DesiredState     RuntimeDesiredState
    ActualState      RuntimeActualState
    Health           HealthStatus
    Circuit          CircuitState

    StartedAt        *time.Time
    StoppedAt        *time.Time
    LastErrorCode    string
    Metadata         map[string]any
}
```

---

## 二十三、RuntimeDefinitionID

建议：

```text
<module-id>/runtime/<name>
```

示例：

```text
com.example.weather#main/runtime/main
com.example.weather#mcp/runtime/server
```

Runtime Instance ID 由每次启动生成，不作为稳定业务 ID。

---

## 二十四、RuntimeBindingDefinition

Contribution 如何连接 Runtime：

```go
type RuntimeBindingDefinition struct {
    RuntimeID  RuntimeDefinitionID
    EntryType  string
    EntryName  string
    Protocol   string
    Metadata   map[string]any
}
```

示例：

```text
Tool get_forecast
→ runtime: main
→ entry_type: tool
→ entry_name: get_forecast
```

RuntimeBinding 只描述入口，不包含 Handler 闭包。

---

## 二十五、DependencyDefinition

建议：

```go
type DependencyDefinition struct {
    ID             string
    Type           DependencyType
    Target         DependencyTarget
    VersionRange   string
    Required       bool
    Scope          DependencyScope
    Resolution     DependencyResolutionPolicy
    Metadata       map[string]any
}
```

---

## 二十六、DependencyType

建议：

```go
type DependencyType string

const (
    DependencyExtension   DependencyType = "extension"
    DependencyModule      DependencyType = "module"
    DependencyContribution DependencyType = "contribution"
    DependencyTool        DependencyType = "tool"
    DependencyWorkflow    DependencyType = "workflow"
    DependencyMCPServer   DependencyType = "mcp_server"
    DependencyProvider    DependencyType = "provider"
    DependencyRuntime     DependencyType = "runtime"
    DependencyHostFeature DependencyType = "host_feature"
    DependencyPlatform    DependencyType = "platform"
)
```

---

## 二十七、依赖方向

依赖必须是：

```text
Source → Target
```

并记录：

- Required；
-Optional；
-Version；
-作用范围；
-解析策略；
-Owner；
-来源；
-冲突策略。

禁止将依赖只保存为字符串数组。

---

## 二十八、依赖作用范围

建议：

```text
install
enable
start
execute
build
development
```

例如：

### Extension 依赖 Extension

可能是 install + enable。

### Tool 依赖 MCP Server

可能是 execute。

### UI 依赖 Desktop Feature

可能是 enable。

### Plugin Runtime 依赖 Host API

可能是 start。

---

## 二十九、Compatibility

建议定义：

```go
type ExtensionCompatibility struct {
    MinHostVersion string
    MaxHostVersion string
    Platforms      []PlatformRequirement
    Architectures  []string
    RequiredFeatures []string
    ForbiddenFeatures []string
}
```

兼容性是 Definition 属性，不是 Enabled。

---

## 三十、ExtensionPolicies

建议包含：

```go
type ExtensionPolicies struct {
    DefaultEnablement EnablementState
    UpdatePolicy      UpdatePolicy
    RollbackPolicy    RollbackPolicy
    DataRetention     DataRetentionPolicy
    TrustRequirement  TrustRequirement
    RuntimePolicy     RuntimePolicy
}
```

Manifest 可声明请求，但宿主可应用更严格策略。

---

## 三十一、ContributionExposure

表示 Contribution 如何暴露。

建议：

```go
type ContributionExposure struct {
    UserVisible   bool
    ModelVisible  bool
    APIVisible    bool
    DesktopVisible bool
    InternalOnly  bool
}
```

最终暴露仍由 EffectiveStateResolver 计算。

Definition 中的 Exposure 只是声明。

---

## 三十二、Owner 模型

统一使用第 12 步 ResourceOwner。

Definition 层也需要 Owner Reference：

```go
type DefinitionOwner struct {
    OwnerType   string
    ExtensionID ExtensionID
    ModuleID    ModuleID
    PublisherID string
}
```

区别：

- Definition Owner：谁声明；
-Resource Owner：谁负责生命周期；
-Runtime Owner：哪个 Module 的 Runtime；
-User Owner：用户接管后的资产。

---

## 三十三、Publisher

建议：

```go
type PublisherReference struct {
    PublisherID string
    KeyID       string
    DisplayName string
    TrustLevel  string
}
```

Publisher 不等于 Owner。

一个用户可以安装某个 Publisher 发布的 Extension，但资源生命周期 Owner 仍是 ExtensionInstallation。

---

## 三十四、Artifact

Artifact 是不可变内容载体。

建议：

```go
type ArtifactReference struct {
    ArtifactID      string
    Type            string
    Hash            string
    ContentTreeHash string
    SizeBytes       int64
    StorageRef      string
}
```

Artifact 不保存 Enabled 和 Runtime 状态。

---

## 三十五、Extension Aggregate

建议聚合边界：

```text
ExtensionDefinition
├── ModuleDefinition
│   ├── RuntimeDefinition
│   └── ContributionDefinition
└── DependencyDefinition
```

该聚合在同一版本内不可变。

更新 Extension 时创建新版本 Definition，不原地修改旧版本。

---

## 三十六、Installation Aggregate

建议：

```text
ExtensionInstallation
├── InstalledModule
├── Enablement Override References
├── Active Version
├── Rollback Points
└── Generation
```

它保存当前安装和用户启用意图。

不包含完整 Definition 内容。

---

## 三十七、Runtime Aggregate

建议：

```text
RuntimeInstance
├── Desired State
├── Actual State
├── Health
├── Circuit
├── Runtime Resources
└── Active Invocations
```

由 Runtime Supervisor 管理。

---

## 三十八、Contribution Aggregate

ContributionDefinition 为不可变定义。

Contribution 当前启用覆盖值保存在：

```text
ContributionEnablementOverride
```

Contribution 当前注册状态保存在：

```text
ContributionRegistrationState
```

不得修改 Definition 保存运行状态。

---

## 三十九、ContributionRegistrationState

建议：

```go
type ContributionRegistrationState struct {
    ContributionID ContributionID
    Generation     int64
    RegistryType   string
    Registered     bool
    Active         bool
    LastErrorCode  string
    UpdatedAt      time.Time
}
```

该状态是运行派生状态，不是业务 Enabled。

---

## 四十、Extension State Matrix

必须明确各状态存放位置：

| 状态 | 所属模型 |
|---|---|
| 是否安装 | ExtensionInstallation |
| 当前版本 | ExtensionInstallation |
| 定义是否有效 | Definition Validation |
| Extension Enabled | ExtensionInstallation |
| Module Enabled | InstalledModule |
| Contribution Override | Enablement Override |
| Scope | Scope Manager |
| Permission | Permission Broker |
| Desired Runtime | Lifecycle Coordinator |
| Actual Runtime | Runtime Supervisor |
| Health | Health Monitor |
| Circuit | Circuit Breaker |
| Registry Active | Contribution Registry |
| Visible/Executable | EffectiveStateResolver |
| 执行状态 | Invocation/Attempt |

禁止同一状态在多个聚合重复持久化。

---

## 四十一、生命周期模型

Extension 生命周期建议：

```text
not_installed
installing
installed
enabling
enabled
disabling
disabled
updating
rolling_back
uninstalling
failed
```

注意：

- `enabled` 是综合生命周期显示状态；
-底层真实状态仍来自 Installation + Enablement + Runtime；
-不要把该显示状态作为唯一真值；
-可作为 Lifecycle Operation 当前阶段。

---

## 四十二、生命周期命令

建议统一命令：

```go
type ExtensionCommand interface {
    CommandID() string
    ExtensionID() ExtensionID
}
```

主要命令：

```text
InstallExtension
EnableExtension
DisableExtension
UpdateExtension
RollbackExtension
UninstallExtension
EnableModule
DisableModule
SetContributionOverride
RepairExtension
```

命令由后续 Lifecycle Manager 处理。

---

## 四十三、领域事件

建议定义：

```text
ExtensionInstalled
ExtensionEnabled
ExtensionDisabled
ExtensionUpdated
ExtensionRolledBack
ExtensionUninstalled
ModuleEnabled
ModuleDisabled
ContributionRegistered
ContributionUnregistered
RuntimeDesiredStateChanged
RuntimeStarted
RuntimeStopped
RuntimeCrashed
DependencyResolved
DependencyLost
```

领域事件：

- 用于通知；
-不作为业务真值；
-必须包含 Generation；
-必须包含 Extension/Module/Contribution ID；
-写入统一审计；
-后续 Event Bus 分发。

---

## 四十四、领域错误

建议统一：

```go
type DomainError struct {
    Code       string
    Message    string
    SubjectID  string
    Retryable  bool
    Details    map[string]any
}
```

核心错误码：

```text
extension_not_found
extension_already_installed
extension_not_installed
extension_definition_invalid
extension_incompatible
module_not_found
contribution_not_found
runtime_not_found
dependency_missing
dependency_conflict
version_conflict
state_conflict
generation_conflict
ownership_conflict
operation_in_progress
migration_required
quarantined
```

---

## 四十五、不可变定义

以下对象应不可变：

- ExtensionDefinition；
-ModuleDefinition；
-ContributionDefinition；
-RuntimeDefinition；
-Package；
-Artifact；
-Integrity；
-Signature；
-已发布 Manifest。

需要修改时创建新版本。

---

## 四十六、可变状态

以下对象可变：

- ExtensionInstallation；
-InstalledModule；
-Contribution Override；
-RuntimeInstance；
-Health；
-Circuit；
-Scope Binding；
-Permission Grant；
-Resource State；
-Schedule State；
-Registration State。

---

## 四十七、快照规则

执行时需要固定快照：

```text
Extension Definition Version
Module Generation
Contribution Definition Hash
Runtime Binding
Scope Snapshot
Permission Decision
Dependency Snapshot
```

避免执行过程中定义漂移。

---

## 四十八、内置能力建模

系统内置 Tool、Agent Skill、Workflow、Provider 也应使用同一领域模型。

建议：

```text
ExtensionID = system/amitia-core
```

不同系统模块：

```text
system/amitia-core#tools
system/amitia-core#workflows
system/amitia-core#providers
```

内置能力可以：

- 不使用外部 `.amitiax`；
-由系统 Artifact 提供；
-使用 host_internal Runtime；
-受系统签名信任；
-禁止用户卸载；
-允许部分 Contribution 禁用。

不得继续使用完全独立的 Builtin Registry 领域。

---

## 四十九、用户本地能力建模

用户导入的独立 Agent Skill、Workflow 或 MCP Server，需要纳入统一 Extension 领域。

可建立合成 Extension：

```text
local.user/imported-agent-skills
local.user/imported-workflows
local.user/mcp-servers
```

或每个导入对象独立 Extension。

推荐原则：

- 需要独立卸载、版本和权限边界时使用独立 Extension；
-纯用户资源可由系统管理的 Local Extension 容器承载；
-不得继续成为领域外孤立对象。

---

## 五十、合成 Extension

建议定义：

```text
Synthetic Extension
```

用于承载：

- 用户单独导入 Agent Skill；
-用户单独创建 Workflow；
-用户手动配置 MCP；
-旧数据迁移对象。

Synthetic Extension 必须：

- 有稳定 ID；
-有 Owner；
-有 Module；
-有 Contribution；
-有安装事实；
-不需要外部分发包；
-可导出为 `.amitiax`；
-使用 system_generated Artifact。

---

## 五十一、Extension 与 User Asset

如果 Workflow、Agent Skill 或 MCP Server 已被用户接管：

- 可以迁入 Synthetic Extension；
-Owner 变为 user；
-原 Extension 保留引用或解除引用；
-更新不再覆盖；
-卸载原 Extension 不删除。

---

## 五十二、数据库模型建议

建议目标表：

```text
extension_definitions
extension_definition_versions
extension_modules
extension_module_versions
extension_contributions
extension_contribution_versions
extension_runtime_definitions
extension_dependencies
extension_installations
extension_installed_modules
extension_enablement_overrides
extension_registration_states
extension_id_mappings
extension_domain_events
extension_kernel_migrations
```

是否最终拆表可根据数据库性能调整，但领域边界必须保持。

---

## 五十三、Definition 存储策略

建议：

- 结构化关键字段单列；
-完整 Canonical Definition 保存 JSON；
-保存 Definition Hash；
-保存 Manifest Version；
-保存 Source Artifact；
-版本不可覆盖；
-当前版本通过 Installation 引用；
-旧版本按回滚和历史策略保留。

---

## 五十四、Canonical Serialization

必须建立确定性序列化：

```text
Canonical JSON
```

用途：

- Definition Hash；
-版本比较；
-签名；
-缓存；
-迁移；
-测试；
-Diff。

要求：

- 字段顺序稳定；
-Map 排序；
-时间统一；
-空值规则统一；
-数字格式稳定；
-未知字段处理明确；
-平台无关。

---

## 五十五、Definition Hash

建议：

```text
SHA-256(Canonical JSON)
```

各层可有：

```text
extension_definition_hash
module_definition_hash
contribution_definition_hash
runtime_definition_hash
```

Hash 不包含：

- 安装时间；
-Enabled；
-Runtime State；
-Health；
-用户 Scope；
-Permission Grant；
-数据库 ID。

---

## 五十六、版本与 Hash 的关系

版本声明相同但 Hash 不同：

```text
version_republish_conflict
```

默认拒绝覆盖。

同一 Extension ID + Version 应对应唯一 Definition Hash。

开发者模式可允许本地开发 Revision，但必须使用：

```text
development_revision
```

不得伪装为已发布同版本。

---

## 五十七、依赖图输入

Extension Kernel Dependency Resolver 的输入必须来自本步骤定义的：

- Extension Dependency；
-Module Dependency；
-Contribution Dependency；
-Runtime Dependency；
-Host Feature Dependency。

不得从 Manifest 原始 JSON 临时解析字符串。

---

## 五十八、Registry 输入

Contribution Registry 只能注册：

```text
ContributionDefinition
```

不能直接注册：

- Package Manifest 原始字段；
-Plugin Handler；
-MCP Discovery 原始 JSON；
-Workflow 文件；
-SKILL.md；
-前端配置；
-旧 SkillDefinition。

这些都必须先转换为统一领域对象。

---

## 五十九、Runtime Supervisor 输入

Runtime Supervisor 只能读取：

- RuntimeDefinition；
-ExtensionInstallation；
-InstalledModule；
-Desired State；
-Resource Limit；
-Host API Policy；
-Generation。

不得直接读取 Manifest 原始 JSON。

---

## 六十、Manifest 与领域模型

未来 Manifest v2 是：

> ExtensionDefinition 的一种序列化输入格式。

Manifest 不等于领域模型。

正确链路：

```text
manifest.json
→ Manifest Parser
→ Manifest DTO
→ Validator
→ ExtensionDefinition Builder
→ Canonical Domain Model
```

禁止：

```text
manifest map
→ 直接 Registry/Runtime
```

---

## 六十一、迁移边界接入

第 20 步 Canonical Migration DTO 必须转换为本步骤领域模型。

建议：

```go
type MigrationDomainBuilder interface {
    BuildExtensionDefinition(
        ctx context.Context,
        entity MigrationEntity,
    ) (ExtensionDefinition, error)
}
```

迁移数据不能绕过领域 Validator。

---

## 六十二、领域 Validator

建议：

```go
type ExtensionDomainValidator interface {
    ValidateExtension(
        definition ExtensionDefinition,
    ) ValidationReport

    ValidateModule(
        module ModuleDefinition,
    ) ValidationReport

    ValidateContribution(
        contribution ContributionDefinition,
    ) ValidationReport

    ValidateRuntime(
        runtime RuntimeDefinition,
    ) ValidationReport
}
```

检查：

- ID；
-版本；
-Owner；
-模块；
-Contribution；
-Runtime；
-依赖；
-作用域；
-权限要求；
-兼容性；
-重复；
-循环；
-平台；
-完整性；
-策略。

---

## 六十三、领域 Builder

建议：

```go
type ExtensionDefinitionBuilder interface {
    Build(
        input ExtensionDefinitionInput,
    ) (ExtensionDefinition, ValidationReport)
}
```

Builder 负责：

- 生成 Canonical ID；
-应用默认值；
-规范化；
-构建模块；
-构建 Contribution；
-构建 Runtime；
-构建依赖；
-计算 Hash；
-生成 Validation Report。

---

## 六十四、Repository 接口

建议：

```go
type ExtensionDefinitionRepository interface {
    SaveVersion(
        ctx context.Context,
        definition ExtensionDefinition,
    ) error

    Get(
        ctx context.Context,
        extensionID ExtensionID,
        version SemanticVersion,
    ) (ExtensionDefinition, error)

    ListVersions(
        ctx context.Context,
        extensionID ExtensionID,
    ) ([]SemanticVersion, error)
}

type ExtensionInstallationRepository interface {
    Get(
        ctx context.Context,
        extensionID ExtensionID,
    ) (ExtensionInstallation, error)

    Save(
        ctx context.Context,
        installation ExtensionInstallation,
        expectedGeneration int64,
    ) error
}
```

使用 Generation/CAS 防并发覆盖。

---

## 六十五、Repository 约束

Repository：

- 只负责持久化；
-不启动 Runtime；
-不注册 Contribution；
-不写 Scope；
-不写 Permission；
-不发业务副作用；
-不调用前端；
-不解析 Manifest；
-不做复杂生命周期编排。

---

## 六十六、服务边界

建议领域服务：

```text
ExtensionDefinitionService
ExtensionInstallationService
ExtensionModuleService
ContributionDefinitionService
RuntimeDefinitionService
ExtensionDomainValidator
ExtensionStateQueryService
```

生命周期编排留给第 22 步。

---

## 六十七、查询模型

写模型和读模型可分离。

建议读模型：

```go
type ExtensionSummary struct {
    ExtensionID      string
    Name             string
    InstalledVersion string
    Enabled          bool
    ModuleCount      int
    ContributionCount int
    RuntimeSummary   RuntimeSummary
    EffectiveState   EffectiveState
}
```

读模型可聚合：

- Definition；
-Installation；
-Runtime；
-Health；
-Contribution；
-Scope；
-Permission；
-Resource。

但不能反向成为写入真值。

---

## 六十八、前端领域映射

前端核心对象应对应：

```text
Extension
Module
Contribution
Runtime
Dependency
Resource
State
```

Extension 详情页建议分区：

- 概览；
-模块；
-Contributions；
-Runtimes；
-Dependencies；
-Permissions；
-Scopes；
-Resources；
-运行与审计；
-版本与回滚；
-开发者信息。

不得继续用一个“技能详情”页面承载所有类型。

---

## 六十九、Extension Center 分类

Extension Center 可按用户理解分类，但后端统一模型不变。

前端分类可包括：

```text
工具扩展
Agent Skill
工作流
MCP 集成
桌面增强
界面扩展
Provider
综合扩展
```

分类来自 Contribution 汇总，不产生新的后端实体。

---

## 七十、领域事件版本

领域事件必须版本化：

```go
type DomainEventEnvelope struct {
    EventID       string
    EventType     string
    EventVersion  int
    AggregateType string
    AggregateID   string
    Generation    int64
    OccurredAt    time.Time
    Payload       json.RawMessage
}
```

防止后续 Event Bus 协议漂移。

---

## 七十一、事件幂等

每个领域事件有：

```text
event_id
aggregate_id
generation
```

消费者必须可幂等处理。

事件重复不能导致：

- Tool 重复注册；
-Runtime 重复启动；
-Schedule 重复；
-资源重复创建；
-UI 重复挂载。

---

## 七十二、领域不变量

必须写入代码和测试的核心不变量：

1. Extension ID 全局唯一。
2. 同 Extension ID + Version 只有一个 Definition Hash。
3. Module 必须属于一个 Extension。
4. Contribution 必须属于一个 Module。
5. RuntimeDefinition 必须属于一个 Module。
6. Contribution RuntimeBinding 必须引用本 Extension 或允许的外部 Runtime。
7. Contribution ID 全局唯一。
8. Module ID 在 Extension 内唯一。
9. RuntimeDefinition ID 全局唯一。
10. Required Dependency 缺失时不能达到 Effective Ready。
11. Definition 不保存运行状态。
12. Installation 不保存 Runtime Health。
13. Runtime 不保存业务 Enabled。
14. Permission Grant 不进入 Definition。
15. Scope Binding 不进入 Definition。
16. Artifact 不保存 Enablement。
17. Manifest 原始字段不能直接注册。
18. 用户接管资源不能被原 Extension 卸载删除。
19. 新版本不能覆盖同版本不同 Hash。
20. 所有变更必须增加 Generation 或创建新版本。

---

## 七十三、内置与第三方一致性

除信任和卸载限制外，内置与第三方 Extension 应使用同一领域模型。

不同点只能体现在：

- Publisher Trust；
-Runtime Type；
-可卸载策略；
-Host API Allowlist；
-签名；
-资源限制；
-系统保护策略。

不得继续存在：

```text
BuiltinSkillDefinition
ThirdPartyExtensionDefinition
MCPStandaloneDefinition
```

作为完全独立模型。

---

## 七十四、领域版本升级

本领域模型本身需要：

```text
domain_schema_version
```

未来升级时：

- Definition Version 与 Domain Schema Version 分离；
-Manifest Version 与 Domain Schema Version 分离；
-数据库 Schema Version 与 Domain Schema Version 分离；
-迁移 Adapter 按版本转换；
-旧 Definition 可只读解释；
-不原地修改历史 Hash。

---

## 七十五、兼容层

旧系统兼容层只能：

```text
Legacy DTO
→ Extension Domain Model
```

不能让新领域对象降级回旧 Skill/Plugin 作为主运行链。

允许旧 API 查询新读模型。

禁止新 Service 依赖旧 Struct。

---

## 七十六、测试要求

必须新增：

### 1. ID

- 合法；
-非法；
-大小写；
-重复；
-版本变化；
-用户 Fork；
-Synthetic Extension。

### 2. Version

- SemVer；
-Pre-release；
-同版本不同 Hash；
-版本排序；
-兼容范围。

### 3. ExtensionDefinition

- 单模块；
-多模块；
-无 Runtime；
-多 Runtime；
-依赖；
-兼容；
-完整性。

### 4. Module

- 唯一；
-Owner；
-Type；
-Enabled；
-依赖；
-跨模块引用。

### 5. Contribution

- Tool；
-Agent Skill；
-Workflow；
-MCP；
-UI；
-Hook；
-Event；
-Schedule；
-Provider；
-Background Task。

### 6. Runtime

- Legacy Go；
-JavaScript；
-Service；
-WASM；
-MCP；
-Workflow；
-Static；
-Binding；
-实例状态。

### 7. Dependency

- Required；
-Optional；
-Version；
-循环；
-缺失；
-跨 Extension；
-Host Feature；
-Platform。

### 8. State Separation

- Definition 无 Enabled；
-Installation 无 Health；
-Runtime 无业务 Enabled；
-Scope/Permission 独立；
-Effective State 派生。

### 9. Canonical Serialization

- 稳定字段顺序；
-跨平台；
-Map；
-空值；
-Hash；
-同内容相同 Hash。

### 10. Aggregate Invariants

覆盖全部不变量。

### 11. Migration

- Canonical Migration DTO；
-旧 Skill 分类；
-旧 Plugin；
-旧 MCP；
-旧 Workflow；
-Owner 不确定；
-冲突。

### 12. Repository

- Save Version；
-Get；
-CAS；
-Generation Conflict；
-不触发 Runtime；
-不可覆盖历史。

### 13. Domain Event

- 版本；
-幂等；
-Generation；
-重复消费；
-顺序。

### 14. Builtin

- 系统 Extension；
-不可卸载；
-Contribution 可禁用；
-host_internal Runtime。

### 15. Synthetic Extension

- 用户 Agent Skill；
-用户 Workflow；
-用户 MCP；
-导出；
-接管。

---

## 七十七、实施任务

### Task 1：锁定领域术语

更新架构文档和代码命名。

### Task 2：定义 ExtensionID、ModuleID、ContributionID、RuntimeDefinitionID

建立统一格式和 Validator。

### Task 3：定义 SemanticVersion

统一版本比较和范围。

### Task 4：定义 ExtensionDefinition

建立不可变定义聚合。

### Task 5：定义 ExtensionPackage 与 ArtifactReference

分离分发载体。

### Task 6：定义 ExtensionInstallation

保存安装事实和总开关。

### Task 7：定义 ModuleDefinition 与 InstalledModule

建立模块边界。

### Task 8：定义 ContributionDefinition

统一所有 Contribution 类型。

### Task 9：定义各类型 Contribution Spec

避免巨大通用结构。

### Task 10：定义 RuntimeDefinition、RuntimeBinding 和 RuntimeInstance

分离定义与实例。

### Task 11：定义 DependencyDefinition

支持版本、范围和作用阶段。

### Task 12：定义 Compatibility、Integrity 和 Policies

建立统一结构。

### Task 13：定义 State Matrix

明确每种状态唯一真值。

### Task 14：定义领域命令与领域事件

为生命周期管理器准备。

### Task 15：实现 Canonical Serialization

生成稳定 Hash。

### Task 16：实现 ExtensionDomainValidator

校验全部不变量。

### Task 17：实现 ExtensionDefinitionBuilder

从 Manifest DTO 和 Migration DTO 构建领域对象。

### Task 18：建立 Definition Repository

版本不可覆盖。

### Task 19：建立 Installation Repository

使用 Generation/CAS。

### Task 20：建立 Query Read Model

供前端和 API 使用。

### Task 21：建模系统内置 Extension

将 Builtin Tool、Workflow、Agent Skill 纳入统一模型。

### Task 22：建模 Synthetic Extension

承载用户独立资源。

### Task 23：接入第 20 步 Migration Boundary

支持 Dry Run 构建领域模型。

### Task 24：增加领域不变量测试

所有不变量进入自动化测试。

### Task 25：增加依赖方向 CI 检查

新领域层不得依赖旧 Manager、前端、Runtime 实现。

### Task 26：输出领域映射报告

列出旧对象到新对象映射。

---

## 七十八、建议目录结构

建议：

```text
backend/internal/extension/kernel/domain/
├── extension/
│   ├── id.go
│   ├── definition.go
│   ├── package.go
│   ├── installation.go
│   ├── version.go
│   ├── compatibility.go
│   ├── integrity.go
│   ├── policies.go
│   ├── commands.go
│   └── events.go
├── module/
│   ├── id.go
│   ├── definition.go
│   ├── installed.go
│   └── type.go
├── contribution/
│   ├── id.go
│   ├── definition.go
│   ├── type.go
│   ├── exposure.go
│   ├── policies.go
│   └── specs/
│       ├── tool.go
│       ├── agent_skill.go
│       ├── workflow.go
│       ├── mcp.go
│       ├── ui.go
│       ├── hook.go
│       ├── event.go
│       ├── schedule.go
│       ├── provider.go
│       └── background_task.go
├── runtime/
│   ├── id.go
│   ├── definition.go
│   ├── binding.go
│   ├── instance.go
│   ├── type.go
│   └── state.go
├── dependency/
│   ├── definition.go
│   ├── target.go
│   ├── type.go
│   └── scope.go
├── common/
│   ├── localized_text.go
│   ├── owner.go
│   ├── artifact.go
│   ├── error.go
│   ├── validation.go
│   └── canonical_json.go
└── validator/
    ├── extension.go
    ├── module.go
    ├── contribution.go
    ├── runtime.go
    └── invariants.go
```

Repository：

```text
backend/internal/extension/kernel/repository/
├── definition_repository.go
├── installation_repository.go
├── module_repository.go
├── contribution_repository.go
└── event_repository.go
```

目录仅为建议。

---

## 七十九、性能要求

领域模型本身必须轻量。

建议：

- Definition 使用不可变内存对象；
-列表查询使用 Summary Read Model；
-完整 Definition 按需读取；
-Canonical Hash 安装/更新时计算；
-Repository 使用索引；
-版本查询分页；
-Contribution 按 Module 分组；
-依赖索引单独存储；
-不要在每次 Tool 调用重新解析完整 ExtensionDefinition；
-Runtime 使用快照引用；
-大 Specification 使用懒加载或引用；
-前端列表不加载全部 Spec。

---

## 八十、风险控制

### P0：领域边界再次混乱

- Package 等于 Extension；
-Plugin 等于 Runtime；
-Contribution 等于 Capability；
-Definition 保存运行状态；
-Manifest 直接进入 Registry；
-用户资产继续在领域外。

### P1：ID 和版本不稳定

- 使用数据库 ID；
-显示名称变化导致 ID 变化；
-同版本不同内容覆盖；
-MCP Session 进入 ID；
-用户 Fork 未换 ID。

### P2：状态重复

- Installation 和 Runtime 都保存 Enabled；
-Contribution Definition 保存 Active；
-Registry 状态成为业务真值；
-Health 回写 Enabled。

### P3：过度抽象

- 一个巨大通用 Spec；
-所有对象都成为 Module；
-简单资源被过度建模；
-领域层依赖大量实现细节；
-前期模型无法落地。

---

## 八十一、本步骤不做的事情

本步骤明确不做：

- 不实现完整生命周期编排；
-不实现 Contribution Registry；
-不实现 Dependency Resolver；
-不实现 Runtime Supervisor；
-不实现 Host API Gateway；
-不实现 Manifest v2 Parser；
-不实现 JavaScript Runtime；
-不实现 UI Contribution；
-不迁移全部生产数据；
-不删除旧系统；
-不实现扩展市场；
-不设计移动端；
-不允许领域层直接执行代码。

---

## 八十二、验收产物

完成后必须提交：

### 1. Extension Kernel 领域主文档

```text
docs/extension-kernel/21-extension-kernel-domain-model.md
```

### 2. 核心领域类型

至少包含：

- ExtensionID；
-SemanticVersion；
-ExtensionDefinition；
-ExtensionPackage；
-ExtensionInstallation；
-ModuleDefinition；
-InstalledModule；
-ContributionDefinition；
-RuntimeDefinition；
-RuntimeBindingDefinition；
-RuntimeInstance；
-DependencyDefinition；
-Compatibility；
-Integrity；
-Policies。

### 3. Contribution Spec

覆盖：

- Tool；
-Agent Skill；
-Workflow；
-MCP；
-UI；
-Hook；
-Event；
-Schedule；
-Provider；
-Background Task。

### 4. State Matrix

明确所有状态唯一真值。

### 5. Canonical Serialization

可生成稳定 Definition Hash。

### 6. Domain Validator

覆盖全部领域不变量。

### 7. Definition Builder

支持：

- Manifest DTO；
-Migration DTO；
-System Builtin；
-Synthetic Extension。

### 8. Repository 接口

定义 Definition 与 Installation 持久化边界。

### 9. 领域命令与事件

为第 22 步生命周期管理器准备。

### 10. Builtin Extension 映射

至少将一组系统 Tool 映射到统一模型作为验证。

### 11. Synthetic Extension 映射

至少将用户 Agent Skill、Workflow 或 MCP 建模。

### 12. 旧对象映射报告

列出：

- Package；
-Plugin；
-Skill；
-Agent Skill；
-MCP；
-Workflow；
-Tool；
-Schedule；
-Provider；
-UI；
-资源；

对应的新领域对象。

### 13. 测试报告

覆盖 ID、Version、Definition、Module、Contribution、Runtime、Dependency、State、Serialization、Repository、Event、Builtin、Synthetic 和 Migration。

---

## 八十三、验收标准

本步骤通过必须满足：

1. Extension、Package、Installation 已明确分离。
2. ExtensionDefinition 是不可变版本定义。
3. Module 已成为正式领域对象。
4. Contribution 已覆盖所有扩展能力类型。
5. Contribution 与 Capability 已明确分离。
6. RuntimeDefinition 与 RuntimeInstance 已明确分离。
7. RuntimeBinding 不包含 Handler 闭包。
8. Dependency 已结构化。
9. Extension、Module、Contribution、Runtime ID 稳定。
10. 同版本不同 Hash 可被拒绝。
11. Definition 不保存 Enabled、Scope、Permission、Health 或 Runtime State。
12. Installation 不保存 Runtime Health。
13. Runtime 不保存业务 Enabled。
14. Manifest 被定义为领域模型输入，不是领域模型本体。
15. 内置能力使用同一领域模型。
16. 用户独立资源可通过 Synthetic Extension 建模。
17. 领域不变量已实现自动化校验。
18. Canonical Serialization 跨平台稳定。
19. Repository 不触发 Runtime 或 Registry。
20. Migration DTO 可构建领域对象。
21. 新领域层不依赖旧 Manager。
22. 关键测试通过。
23. 后续第 22 步可以实现唯一生命周期管理器。

---

## 八十四、退出条件

只有满足以下条件后，才能进入第 22 步“实现唯一生命周期管理器”：

- ExtensionDefinition 已落地；
-ExtensionInstallation 已落地；
-ModuleDefinition 已落地；
-ContributionDefinition 已落地；
-RuntimeDefinition 与 RuntimeInstance 已落地；
-DependencyDefinition 已落地；
-State Matrix 已锁定；
-Canonical Serialization 已落地；
-Domain Validator 已落地；
-Definition Builder 已落地；
-Repository 接口已锁定；
-领域命令与事件已定义；
-Builtin 与 Synthetic Extension 已验证；
-旧对象映射报告已完成；
-关键测试通过。

---

## 八十五、执行约束

执行本步骤时必须遵守：

> 本步骤只定义并落地 Extension Kernel 的领域模型，不得把旧 Manager、Runtime Handler、Manifest 原始 JSON、前端状态或数据库实现细节带入领域对象。

禁止出现：

- ExtensionDefinition 保存 Handler；
-ModuleDefinition 保存 Runtime Instance；
-ContributionDefinition 保存 Permission Grant；
-Manifest Map 直接成为 Definition；
-ExtensionPackage 保存 Enabled；
-RuntimeInstance 保存业务启用状态；
-Registry Active 回写 Definition；
-MCP Session ID 进入 Contribution ID；
-用户角色进入稳定 ID；
-同版本不同内容静默覆盖；
-内置能力继续使用独立 Skill 模型；
-Synthetic Extension 无 Owner 或 Installation；
-领域 Repository 启动 Runtime；
-新领域层 import 旧 PluginManager、MCPManager、WorkflowExecutor。

本步骤完成后，Amitia 必须具备一套稳定、清晰、可验证、可版本化、可迁移且足以支撑整个新扩展系统的 Extension Kernel 领域基础。

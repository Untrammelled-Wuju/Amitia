# Amitia 扩展系统重构第 7 步实施文档

## 第 7 步：建立统一 Tool/Capability 模型

---

## 一、步骤目标

在第 6 步已经完成 Skill 概念拆分的基础上，正式建立 Amitia 统一的 Tool/Capability 模型。

本步骤的目标是让以下能力：

- 系统内置工具；
- Legacy Tool；
- 插件工具；
- MCP Tool；
- Workflow 可调用入口；
- Computer Use 工具；
- 桌面操作工具；
- Provider 暴露的动作；
- 系统内部控制工具；

最终都通过同一套定义、注册、权限、作用域、执行、审计和状态模型被管理。

本步骤完成后，系统必须形成唯一的可执行能力链：

```text
ToolDefinition
→ ToolRegistry
→ Availability Evaluation
→ Permission Evaluation
→ ToolExecutor
→ Runtime Adapter
→ ToolResult
→ Audit
```

并消除以下历史问题：

- 不同来源使用不同执行入口；
- MCP Tool 先转成 Skill 再执行；
- Workflow 本体直接伪装成 Skill；
- Plugin Tool 通过独立注册链；
- 内置工具绕过统一状态模型；
- 权限、作用域、连接状态和启用状态分散判断；
- Tool 返回结构不统一；
- Tool ID 与模型名称混淆；
- 执行结果和副作用缺乏统一表达；
- Tool 可见性和 Tool 可执行性没有区分。

---

## 二、核心定义

## 1. Capability

Capability 表示 Amitia 中可被系统发现、授权、引用和执行的能力。

Capability 是内部总抽象，不一定全部直接暴露给模型。

示例：

- Tool；
- Workflow Entry；
- Provider Action；
- Desktop Action；
- Internal Control Action。

建议定义：

```go
type CapabilityDefinition struct {
    ID              CapabilityID
    Type            CapabilityType
    Owner           ResourceOwner
    Source          CapabilitySource
    Name            LocalizedText
    Description     LocalizedText
    InputSchema     json.RawMessage
    OutputSchema    json.RawMessage
    Permissions     []PermissionRequirement
    ScopeRule       ScopeRule
    RiskLevel       RiskLevel
    SideEffectLevel SideEffectLevel
    Runtime         RuntimeBinding
    Availability    AvailabilityRule
    Metadata        map[string]any
}
```

---

## 2. Tool

Tool 是可以被 Agent、用户操作或其他 Capability 直接调用的 Capability。

建议定义：

```go
type ToolDefinition struct {
    CapabilityDefinition

    ModelExposure   ModelExposureRule
    ExecutionPolicy ToolExecutionPolicy
    ResultPolicy    ToolResultPolicy
}
```

Tool 必须具备：

- 稳定 ID；
- 显示名称；
- 模型名称；
-描述；
-输入 Schema；
-输出 Schema；
-权限；
-作用域；
-风险；
-副作用；
-来源；
-所有者；
-运行时绑定；
-可用状态；
-执行策略；
-结果策略。

---

## 三、Capability 类型

建议建立：

```go
type CapabilityType string

const (
    CapabilityTypeTool           CapabilityType = "tool"
    CapabilityTypeWorkflowEntry  CapabilityType = "workflow_entry"
    CapabilityTypeProviderAction CapabilityType = "provider_action"
    CapabilityTypeDesktopAction  CapabilityType = "desktop_action"
    CapabilityTypeInternalAction CapabilityType = "internal_action"
)
```

第一阶段至少完整实现：

```text
tool
workflow_entry
internal_action
```

其他类型可以预留，但不得在本步骤直接扩展旧系统功能。

---

## 四、来源模型

建议定义：

```go
type CapabilitySource string

const (
    CapabilitySourceBuiltin     CapabilitySource = "builtin"
    CapabilitySourcePlugin      CapabilitySource = "plugin"
    CapabilitySourceMCP         CapabilitySource = "mcp"
    CapabilitySourceWorkflow    CapabilitySource = "workflow"
    CapabilitySourceComputerUse CapabilitySource = "computer_use"
    CapabilitySourceProvider    CapabilitySource = "provider"
    CapabilitySourceInternal    CapabilitySource = "internal"
    CapabilitySourceLegacy      CapabilitySource = "legacy"
)
```

要求：

- Source 只描述能力来源；
-不描述所有权；
-不描述作用域；
-不描述当前状态；
-不参与 ID 生成之外的业务判断；
-不能再使用 `SkillSource`。

`CapabilitySourceLegacy` 仅用于迁移期，后续必须删除。

---

## 五、ID 模型

## 1. Capability ID

Capability ID 必须稳定、全局唯一、不可由显示名称推导。

建议格式：

```text
<source>/<owner-namespace>/<capability-name>
```

示例：

```text
builtin/files/read
builtin/files/write
plugin/com.example.weather/query_weather
mcp/server-123/search
workflow/com.example.daily-summary/run
internal/agent-skill/activate
computer_use/desktop/click
```

要求：

- 只能使用小写 ASCII；
-分隔符固定；
-不允许空格；
-不允许中文；
-不允许数据库自增 ID；
-不因角色、会话或启用状态变化；
-不因 MCP 重连变化；
-不因显示名称变化；
-版本升级默认保持；
-删除后重新安装仍可恢复原 ID。

---

## 2. Model Tool Name

模型名称与 Capability ID 分离。

模型名称建议格式：

```text
<namespace>__<tool_name>
```

示例：

```text
builtin_files__read
weather__query_weather
mcp_server_123__search
daily_summary__run
```

要求：

- 满足不同模型供应商工具名限制；
-可从 Capability ID 稳定生成；
-冲突时使用确定性后缀；
-映射关系必须持久稳定；
-日志必须同时记录 Capability ID 和 Model Tool Name；
-模型名称变化不得影响权限和所有权。

---

## 3. Invocation ID

每次调用必须生成：

```text
invocation_id
```

用于：

- 审计；
-超时；
-取消；
-幂等；
-父子调用；
-日志；
-副作用；
-重试；
-跨进程追踪。

建议采用：

```text
UUIDv7
```

或项目统一的有序唯一 ID。

---

## 六、所有权模型

建议定义：

```go
type ResourceOwner struct {
    OwnerType  OwnerType
    OwnerID    string
    ExtensionID string
    ModuleID    string
}
```

OwnerType：

```text
system
user
extension
shared
temporary
```

示例：

### 内置工具

```text
owner_type: system
owner_id: core
```

### 插件工具

```text
owner_type: extension
extension_id: com.example.weather
module_id: runtime
```

### 包内 MCP Tool

```text
owner_type: extension
extension_id: com.example.weather
module_id: weather-mcp
```

### 用户手动创建 MCP Server 的 Tool

```text
owner_type: user
owner_id: current-user
```

所有权决定：

- 谁能升级；
-谁能删除；
-卸载扩展时是否清理；
-共享资源是否引用检查；
-审计归属；
-配置归属；
-Secret 归属。

---

## 七、运行时绑定

ToolDefinition 不直接保存闭包或宿主对象。

建议定义：

```go
type RuntimeBinding struct {
    RuntimeType RuntimeType
    RuntimeID   string
    HandlerName string
    Endpoint    string
    Metadata    map[string]any
}
```

RuntimeType：

```text
builtin
plugin_js
plugin_service
mcp
workflow
internal
legacy
```

示例：

### 内置工具

```json
{
  "runtimeType": "builtin",
  "runtimeId": "core",
  "handlerName": "files.read"
}
```

### MCP Tool

```json
{
  "runtimeType": "mcp",
  "runtimeId": "server-123",
  "handlerName": "search"
}
```

### Workflow Tool

```json
{
  "runtimeType": "workflow",
  "runtimeId": "workflow-456",
  "handlerName": "run"
}
```

### 插件工具

```json
{
  "runtimeType": "plugin_js",
  "runtimeId": "com.example.weather/main",
  "handlerName": "queryWeather"
}
```

这样 ToolRegistry 只负责定义和定位，不直接拥有执行实现。

---

## 八、输入与输出 Schema

## 1. Input Schema

所有 Tool 必须使用统一 JSON Schema。

要求：

- 固定支持版本；
-明确类型；
-必填字段；
-默认值；
-枚举；
-范围；
-格式；
-嵌套；
-数组；
-附加字段策略；
-敏感字段标记；
-UI Hint 与模型 Schema 分离。

禁止：

- 使用任意 `map[string]any` 作为无约束输入；
-由 Handler 自行解析未知字段；
-不同来源使用不同 Schema 语义；
-MCP Schema 不经校验直接暴露。

---

## 2. Output Schema

所有 Tool 应尽量提供 Output Schema。

输出可分为：

```text
text
structured
binary_reference
resource_reference
ui_content
stream
task_reference
```

建议定义统一结果：

```go
type ToolResult struct {
    InvocationID string
    Status       ToolResultStatus
    Content      []ToolContent
    Structured   json.RawMessage
    Error        *ToolError
    SideEffects  []RecordedSideEffect
    Metadata     map[string]any
}
```

ToolContent：

```go
type ToolContent struct {
    Type     ToolContentType
    Text     string
    MIMEType string
    URI      string
    Data     json.RawMessage
}
```

---

## 九、错误模型

建议定义统一错误：

```go
type ToolError struct {
    Code        string
    Message     string
    Retryable   bool
    UserVisible bool
    Details     map[string]any
    Cause       error
}
```

错误分类：

```text
invalid_input
permission_denied
scope_denied
not_available
runtime_unavailable
timeout
cancelled
conflict
rate_limited
dependency_missing
connection_lost
execution_failed
invalid_result
internal_error
```

要求：

- 模型可见错误与内部错误分离；
-不得将 Secret、命令行、Token、堆栈直接返回模型；
-可重试错误必须明确；
-权限拒绝不得包装为普通执行失败；
-取消必须独立表示；
-MCP 错误必须映射到统一错误；
-Plugin Runtime 错误必须映射到统一错误；
-Workflow 节点错误必须保留节点定位信息。

---

## 十、执行上下文

建议定义：

```go
type ToolInvocationContext struct {
    InvocationID   string
    ParentID       string
    UserID         string
    CharacterID    string
    ConversationID string
    ExtensionID    string
    ModuleID       string
    Source         InvocationSource
    ApprovalMode   ApprovalMode
    Deadline       time.Time
    IdempotencyKey string
    TraceID        string
    Metadata       map[string]any
}
```

InvocationSource：

```text
model
user
workflow
plugin
system
scheduled_task
computer_use
```

要求：

- 所有 Tool 调用必须有 Context；
-不得通过全局变量隐式获取角色；
-不得通过当前页面推断会话；
-不得在 Handler 内自行读取未声明作用域；
-父子调用必须建立调用链；
-递归深度必须限制；
-插件调用其他 Tool 必须继承或收窄作用域；
-Workflow 调用 Tool 必须记录父 Invocation ID。

---

## 十一、状态模型

Tool 的状态不得只使用一个 `enabled` 布尔值。

建议拆分为：

```go
type ToolState struct {
    Installed       bool
    ModuleEnabled   bool
    CapabilityEnabled bool
    ScopeAllowed    bool
    PermissionGranted bool
    RuntimeReady    bool
    DependencyReady bool
    Health          HealthStatus
}
```

最终可见性：

```text
visible_to_model
```

最终可执行性：

```text
executable
```

必须分别计算。

### 模型可见条件

建议：

```text
已安装
模块启用
能力启用
当前作用域允许
模型暴露规则允许
依赖可解析
```

### 真正可执行条件

建议：

```text
模型可见
权限已授权
运行时 Ready
外部连接健康
依赖可用
未熔断
未超限
```

模型可见但暂时不可执行的 Tool，应根据产品策略：

- 隐藏；
-显示不可用原因；
-执行时返回结构化错误。

不得由不同来源自行决定。

---

## 十二、Availability Evaluation

建立统一可用性评估器：

```go
type AvailabilityEvaluator interface {
    Evaluate(
        ctx context.Context,
        tool ToolDefinition,
        invocation ToolInvocationContext,
    ) AvailabilityResult
}
```

结果：

```go
type AvailabilityResult struct {
    Visible    bool
    Executable bool
    Reasons    []AvailabilityReason
}
```

Reason 示例：

```text
module_disabled
tool_disabled
scope_denied
permission_missing
runtime_not_ready
dependency_missing
mcp_disconnected
workflow_invalid
plugin_circuit_open
platform_unsupported
```

要求：

- 模型工具列表和执行前校验使用同一评估器；
-前端状态展示也使用同一结果；
-禁止前端自行拼接状态；
-禁止 MCP API 和 Extension Registry 分别判断；
-禁止 ToolExecutor 再实现另一套可用性逻辑。

---

## 十三、执行策略

建议定义：

```go
type ToolExecutionPolicy struct {
    Timeout          time.Duration
    MaxConcurrency   int
    RetryPolicy      RetryPolicy
    Idempotent       bool
    ApprovalRequired bool
    AllowBackground  bool
    MaxDepth         int
    ResourceLimits   ResourceLimits
}
```

### Timeout

每个 Tool 必须有：

- 默认超时；
-最大超时；
-调用方可否缩短；
-调用方不可任意延长。

### Concurrency

支持：

- 全局限制；
-每 Tool 限制；
-每扩展限制；
-每角色限制；
-每会话限制。

### Retry

仅对明确可重试错误启用。

禁止对以下情况自动重试：

- 权限拒绝；
-用户取消；
-非幂等写操作；
-高风险操作；
-参数错误；
-审批拒绝。

### Idempotency

高风险或写操作 Tool 必须支持幂等键或明确声明不支持。

---

## 十四、副作用模型

建议定义：

```go
type SideEffectLevel string

const (
    SideEffectNone       SideEffectLevel = "none"
    SideEffectRead       SideEffectLevel = "read"
    SideEffectWrite      SideEffectLevel = "write"
    SideEffectExternal   SideEffectLevel = "external"
    SideEffectSystem     SideEffectLevel = "system"
    SideEffectDestructive SideEffectLevel = "destructive"
)
```

每次执行后记录：

```go
type RecordedSideEffect struct {
    Type        string
    Target      string
    Description string
    Reversible  bool
    Metadata    map[string]any
}
```

示例：

- 读取文件；
-写入文件；
-发送消息；
-创建日程；
-修改记忆；
-启动进程；
-执行 Computer Use；
-调用外部 API；
-删除资源。

副作用信息用于：

- 审批；
-审计；
-回滚；
-用户展示；
-安全策略；
-幂等判断。

---

## 十五、Tool Registry

建议定义：

```go
type ToolRegistry interface {
    Register(ctx context.Context, tool ToolDefinition) error
    Replace(ctx context.Context, tool ToolDefinition) error
    Unregister(ctx context.Context, toolID CapabilityID) error
    Get(ctx context.Context, toolID CapabilityID) (ToolDefinition, error)
    ResolveModelName(ctx context.Context, modelName string) (ToolDefinition, error)
    List(ctx context.Context, filter ToolFilter) ([]ToolDefinition, error)
    Evaluate(ctx context.Context, invocation ToolInvocationContext) ([]ToolView, error)
}
```

Registry 负责：

- 定义索引；
-ID 唯一性；
-模型名称映射；
-来源索引；
-所有者索引；
-作用域过滤；
-版本替换；
-注册冲突；
-卸载清理；
-可见性查询。

Registry 不负责：

- 直接执行；
-权限持久化；
-MCP 连接；
-Plugin Runtime；
-Workflow 生命周期；
-Agent Skill 激活；
-Secret；
-数据库业务数据。

---

## 十六、注册规则

### 1. 注册必须原子化

一个扩展模块注册多个 Tool 时：

```text
全部成功
或
全部失败
```

不得部分注册。

### 2. 替换规则

仅允许同一所有者替换同一 Tool ID。

不同所有者冲突必须拒绝。

### 3. 模型名称冲突

使用确定性算法解决，但必须：

- 保持稳定；
-记录映射；
-更新时不漂移；
-前端可查看。

### 4. Runtime 先后关系

ToolDefinition 可以先注册，但：

```text
RuntimeReady = false
```

直到 Runtime Supervisor 确认运行时可用。

### 5. 卸载

扩展卸载时按 Owner 索引批量注销 Tool。

---

## 十七、Tool Executor

建议定义：

```go
type ToolExecutor interface {
    Execute(
        ctx context.Context,
        tool ToolDefinition,
        invocation ToolInvocationContext,
        input json.RawMessage,
    ) ToolResult
}
```

执行顺序必须固定：

```text
1. Resolve Tool
2. Validate Input
3. Evaluate Availability
4. Evaluate Permission
5. Validate Scope
6. Apply Approval Policy
7. Acquire Concurrency Slot
8. Create Timeout Context
9. Record Invocation Start
10. Dispatch Runtime Adapter
11. Validate Result
12. Record Side Effects
13. Record Invocation Finish
14. Return Sanitized Result
```

任何来源不得绕过上述顺序。

---

## 十八、Runtime Adapter

建议定义统一接口：

```go
type RuntimeAdapter interface {
    Supports(binding RuntimeBinding) bool

    Execute(
        ctx context.Context,
        binding RuntimeBinding,
        invocation ToolInvocationContext,
        input json.RawMessage,
    ) ToolResult

    Health(
        ctx context.Context,
        binding RuntimeBinding,
    ) HealthStatus
}
```

首批 Adapter：

```text
BuiltinRuntimeAdapter
LegacyRuntimeAdapter
MCPRuntimeAdapter
WorkflowRuntimeAdapter
PluginRuntimeAdapter
InternalRuntimeAdapter
```

其中：

- LegacyRuntimeAdapter 只用于迁移；
-未来新增 ServiceRuntimeAdapter；
-未来新增 ComputerUseRuntimeAdapter。

---

## 十九、模型工具暴露

统一建立：

```go
type ModelToolView struct {
    Name        string
    Description string
    InputSchema json.RawMessage
    CapabilityID string
}
```

暴露链：

```text
ToolRegistry.List
→ AvailabilityEvaluator
→ ModelExposureRule
→ Token Budget
→ Model Tool Name Mapping
→ Provider Format Adapter
```

必须支持：

- OpenAI 工具格式；
-Claude 工具格式；
-Gemini 工具格式；
-其他 Provider 格式。

Provider Adapter 只转换格式，不参与业务筛选。

---

## 二十、ModelExposureRule

建议定义：

```go
type ModelExposureRule struct {
    ExposedByDefault bool
    RequiresActivation bool
    Categories       []string
    MaxPromptTokens  int
    Priority         int
}
```

用于控制：

- 是否默认暴露；
-是否需 Agent Skill 激活；
-是否只在特定任务出现；
-工具描述 Token 预算；
-大量 MCP Tool 的裁剪；
-角色允许工具；
-会话临时工具。

Tool 可执行不等于必须始终暴露给模型。

---

## 二十一、版本与兼容性

ToolDefinition 必须有定义版本：

```go
type ToolVersion struct {
    SchemaVersion int
    Revision      string
}
```

要求：

- Input Schema 变化可检测；
-Output Schema 变化可检测；
-权限变化可检测；
-风险变化可检测；
-升级时提示；
-Agent Skill Tool 引用可验证；
-Workflow 依赖可验证；
-模型缓存可失效。

破坏性变更应：

- 使用新 Tool ID；
-或声明兼容迁移；
-不得静默替换。

---

## 二十二、持久化模型

ToolDefinition 可来自动态来源，不一定全部持久化完整定义。

建议区分：

### Source Definition

真实来源定义：

- 内置代码；
-插件 Manifest；
-MCP Discovery；
-Workflow Definition；
-系统内部。

### Registry Snapshot

用于启动恢复和诊断的派生快照。

不得把 Registry Snapshot 作为唯一源数据。

持久化至少记录：

```text
tool_id
source
owner
runtime_binding
model_name
schema_hash
permission_hash
enabled_state
last_registered_at
last_health
```

MCP Tool 的真实定义仍来自 MCP Discovery。

---

## 二十三、前端状态接口

应新增统一内部查询模型：

```json
{
  "id": "mcp/server-123/search",
  "name": "搜索",
  "source": "mcp",
  "owner": {
    "type": "user"
  },
  "state": {
    "installed": true,
    "moduleEnabled": true,
    "capabilityEnabled": true,
    "scopeAllowed": true,
    "permissionGranted": true,
    "runtimeReady": false,
    "dependencyReady": true,
    "visible": true,
    "executable": false
  },
  "reasons": [
    "mcp_disconnected"
  ]
}
```

前端不得再分别调用：

- Skill Enabled；
-MCP Tool Enabled；
-Registry Scope；
-Connection Status；

自行判断最终状态。

---

## 二十四、迁移方案

### 1. Legacy Tool

```text
Legacy Tool
→ LegacyRuntimeAdapter
→ ToolDefinition
```

迁移后逐步改为 BuiltinRuntimeAdapter。

### 2. MCP Tool

```text
MCP Discovery
→ MCPToolDescriptor
→ MCP Tool Adapter
→ ToolDefinition
```

删除 MCP Skill Runtime。

### 3. Plugin Tool

```text
Plugin Contribution
→ ToolDefinition
→ PluginRuntimeAdapter
```

当前 Go Plugin 先通过临时 Adapter。

### 4. Workflow

```text
WorkflowDefinition
→ 可选 Workflow Tool Adapter
→ ToolDefinition
```

### 5. Internal Agent Skill Tools

```text
Internal ToolDefinition
→ InternalRuntimeAdapter
```

Agent Skill 本体不迁入 ToolRegistry。

---

## 二十五、测试要求

必须新增以下测试。

### 1. Definition 测试

- ID；
-Source；
-Owner；
-RuntimeBinding；
-Schema；
-Permissions；
-Risk；
-SideEffect；
-版本。

### 2. Registry 测试

- 注册；
-冲突；
-替换；
-卸载；
-模型名称；
-Owner 索引；
-Source 索引；
-并发；
-原子批量注册。

### 3. Availability 测试

覆盖所有状态组合。

### 4. Executor 测试

覆盖固定执行顺序。

### 5. Runtime Adapter 测试

覆盖：

- Builtin；
-Legacy；
-MCP；
-Workflow；
-Plugin；
-Internal。

### 6. Result 测试

覆盖：

- Text；
-Structured；
-Resource；
-Binary Reference；
-Error；
-Timeout；
-Cancel；
-SideEffect。

### 7. Model Exposure 测试

覆盖：

- 角色；
-会话；
-Agent Skill 激活；
-Token Budget；
-工具裁剪；
-Provider 格式。

### 8. 迁移等价测试

验证旧模型工具调用与新 Tool 模型必要行为一致。

---

## 二十六、实施任务

### Task 1：定义 Capability 与 Tool 类型

完成核心类型、枚举和验证规则。

### Task 2：定义统一 ID 和模型名称映射

完成生成器、冲突策略和稳定性测试。

### Task 3：定义 RuntimeBinding

建立 Tool 定义与执行实现的解耦。

### Task 4：定义 ToolInvocationContext

统一角色、会话、来源、审批、追踪和父子调用。

### Task 5：定义 ToolResult 与 ToolError

统一全部执行来源的返回格式。

### Task 6：定义状态与 AvailabilityEvaluator

统一可见性与可执行性。

### Task 7：建立 ToolRegistry

实现注册、替换、注销、查询和模型名称解析。

### Task 8：建立 ToolExecutor

实现统一执行顺序。

### Task 9：建立 RuntimeAdapter 接口

实现首批 Adapter 骨架。

### Task 10：迁移 Legacy Tool

通过 LegacyRuntimeAdapter 接入。

### Task 11：迁移 MCP Tool

通过 MCPRuntimeAdapter 接入。

### Task 12：迁移 Workflow Tool

通过 WorkflowRuntimeAdapter 接入。

### Task 13：迁移 Plugin Tool

通过 PluginRuntimeAdapter 接入。

### Task 14：迁移 Internal Tool

通过 InternalRuntimeAdapter 接入。

### Task 15：建立统一模型工具输出

支持各模型 Provider 格式。

### Task 16：建立统一前端状态接口

输出真实可见性、可执行性和原因。

### Task 17：增加迁移统计

记录仍通过旧 Skill Registry 执行的数量。

### Task 18：完成回归与性能测试

确认模型调用、工具执行和列表构建无明显退化。

---

## 二十七、建议目录结构

建议：

```text
backend/internal/extension/kernel/capability/
├── definition.go
├── id.go
├── source.go
├── owner.go
├── state.go
├── availability.go
├── invocation.go
├── result.go
├── error.go
├── registry.go
├── executor.go
├── exposure.go
└── runtime_adapter.go
```

Adapter：

```text
backend/internal/extension/kernel/adapters/
├── builtin.go
├── legacy.go
├── mcp.go
├── workflow.go
├── plugin.go
└── internal.go
```

不得因目录建议强制搬迁无关代码。

---

## 二十八、性能要求

Tool 模型可能包含大量 MCP Tool，因此需要基础性能约束。

建议目标：

- 1,000 个 Tool 注册不产生明显阻塞；
-按 Owner 卸载为索引查询；
-按作用域过滤不全表扫描数据库；
-模型工具列表构建可缓存；
-状态变化可局部失效；
-Registry 并发读取安全；
-执行路径不重复做昂贵 Discovery；
-日志异步但不可丢失关键审计；
-Tool Schema Hash 可用于缓存。

性能测试不得牺牲正确性和权限一致性。

---

## 二十九、风险控制

### P0：权限或执行绕过

- 某来源绕过 Executor；
-某来源绕过 Permission；
-模型名称解析到错误 Tool；
-作用域泄露；
-父子调用提升权限。

### P1：状态漂移

- Registry 与运行时不一致；
-前端与后端状态不一致；
-MCP 重连后重复 Tool；
-卸载后 Tool 残留。

### P2：兼容问题

- Tool ID 变化；
-Agent Skill 引用失效；
-Workflow 依赖失效；
-模型工具名称变化。

### P3：性能问题

- 大量 Tool 构建缓慢；
-每次聊天全量查询数据库；
-状态评估重复；
-模型工具描述过大。

---

## 三十、本步骤不做的事情

本步骤明确不做：

- 不建立完整 Extension Package 生命周期；
-不设计 `.amitiax` v2 完整 Manifest；
-不实现第三方 JavaScript Runtime；
-不实现 UI Contribution；
-不迁移 Agent Skill Catalog 全部逻辑；
-不重建 Workflow Engine；
-不重建 MCP Manager；
-不删除旧 Skill Registry；
-不删除旧数据库表；
-不重建扩展中心；
-不实现插件市场；
-不实现移动端适配。

---

## 三十一、验收产物

完成后必须提交：

### 1. Tool/Capability 模型主文档

```text
docs/extension-kernel/07-unified-tool-capability-model.md
```

### 2. 核心类型代码

至少包含：

- CapabilityDefinition；
-ToolDefinition；
-CapabilitySource；
-ResourceOwner；
-RuntimeBinding；
-ToolInvocationContext；
-ToolResult；
-ToolError；
-ToolState。

### 3. Tool Registry

包括：

- 注册；
-替换；
-注销；
-查询；
-Owner 索引；
-Source 索引；
-模型名称映射。

### 4. AvailabilityEvaluator

统一输出：

- visible；
-executable；
-reasons。

### 5. ToolExecutor

实现统一执行顺序和审计入口。

### 6. Runtime Adapter

至少完成：

- Builtin；
-Legacy；
-MCP；
-Workflow；
-Plugin；
-Internal。

### 7. 模型工具输出适配

至少支持当前已使用的模型供应商。

### 8. 前端状态 API

能够统一展示 Tool 状态及不可用原因。

### 9. 迁移统计报告

列出：

- 已迁移 Tool；
-仍使用旧 Skill；
-重复 ID；
-模型名称冲突；
-状态映射异常。

### 10. 测试报告

覆盖：

- Registry；
-Availability；
-Executor；
-Adapter；
-Result；
-ID；
-迁移等价；
-性能。

---

## 三十二、验收标准

本步骤通过必须满足：

1. 所有可执行能力均可表示为 ToolDefinition。
2. ToolDefinition 不包含具体闭包或宿主对象。
3. Tool ID 稳定且全局唯一。
4. 模型名称与 Tool ID 分离。
5. Tool 来源与所有权分离。
6. Tool 可见性与可执行性分离。
7. 所有 Tool 调用经过统一 ToolExecutor。
8. 所有 Tool 调用经过统一 Availability 和 Permission。
9. MCP Tool 可直接映射为 ToolDefinition。
10. Workflow 可通过 Adapter 暴露为 Tool。
11. Plugin Tool 可通过 Adapter 暴露为 Tool。
12. Agent Skill 本体不进入 ToolRegistry。
13. ToolResult 与 ToolError 已统一。
14. Tool 状态可被前端统一展示。
15. 旧 Skill Registry 不再接收新的 Tool 写入。
16. 新旧行为等价测试通过。
17. 后续第 8 步可以在该模型上抽取统一执行安全内核。

---

## 三十三、退出条件

只有满足以下条件后，才能进入第 8 步“抽取统一执行安全内核”：

- ToolDefinition 已落地；
-ToolRegistry 已落地；
-ToolExecutor 已落地；
-AvailabilityEvaluator 已落地；
-RuntimeAdapter 已落地；
-Legacy、MCP、Workflow、Plugin、Internal Tool 已可接入；
-Tool ID 规则稳定；
-前端可读取统一状态；
-所有新增 Tool 不再写入旧 Skill Registry；
-旧链路仍可通过迁移适配器运行；
-关键测试通过。

---

## 三十四、执行约束

执行本步骤时必须遵守：

> ToolRegistry 只管理“能力定义与定位”，不接管所有来源的业务生命周期；ToolExecutor 只管理“统一调用过程”，不拥有 MCP 连接、Plugin 进程或 Workflow 定义。

禁止出现：

- ToolRegistry 启动 MCP Server；
-ToolExecutor 直接读 Plugin 数据库；
-ToolDefinition 保存运行时闭包；
-前端直接组合多个状态判断；
-某来源绕过统一执行；
-为了兼容旧代码重新把 Agent Skill 包装成 Tool；
-新 Tool 同时注册到新旧 Registry。

本步骤完成后，Amitia 必须真正具备一套独立于来源的统一可执行能力模型。
